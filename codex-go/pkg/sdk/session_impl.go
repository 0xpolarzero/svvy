package sdk

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/evmts/codex/codex-go/internal/conversation/manager"
	"github.com/evmts/codex/codex-go/internal/protocol"
)

// submitInternal handles the actual submission to the manager and returns a channel of events.
// This method creates a protocol.OpUserTurn and submits it to the manager, streams events
// back through a channel, and handles response collection asynchronously.
func (s *Session) submitInternal(ctx context.Context, message string) (<-chan StreamEvent, error) {
	// Create working directory
	cwd := s.workingDirectory
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			cwd = "."
		}
	}

	// Apply defaults
	approvalPolicy := s.approvalPolicy
	if approvalPolicy == "" {
		approvalPolicy = "auto"
	}

	sandboxMode := s.sandboxPolicy
	if sandboxMode == "" {
		sandboxMode = "native"
	}

	model := s.model
	if model == "" {
		model = "claude-sonnet-4-5"
	}

	// Create the user turn operation
	op := &protocol.OpUserTurn{
		Items: []protocol.UserInput{
			{
				Type: "text",
				Text: &message,
			},
		},
		Cwd:            cwd,
		ApprovalPolicy: approvalPolicy,
		SandboxPolicy: protocol.SandboxPolicy{
			Mode: sandboxMode,
		},
		Model: model,
	}

	// Create event channel for streaming events back to caller
	eventCh := make(chan StreamEvent, 10)

	// Create a response collector
	collector := &responseCollector{
		done:     make(chan struct{}),
		errors:   make(chan error, 1),
		eventCh:  eventCh,
		ctx:      ctx,
	}

	// Create event handler to collect response data and forward events
	eventHandler := func(ctx context.Context, event *protocol.Event) error {
		return collector.handleEvent(ctx, event)
	}

	// Get or create manager session with event handler
	_, err := s.getOrCreateManagerSession(ctx, eventHandler)
	if err != nil {
		close(eventCh)
		return nil, fmt.Errorf("failed to get manager session: %w", err)
	}

	// Start async processing
	go func() {
		defer close(eventCh)

		// Submit the operation to the manager
		err := s.sdk.manager.SubmitOp(ctx, s.conversationID, op)
		if err != nil {
			select {
			case eventCh <- StreamEvent{
				Type:  "error",
				Error: fmt.Errorf("failed to submit operation: %w", err),
				Done:  true,
			}:
			case <-ctx.Done():
			}
			return
		}

		// Wait for completion with timeout
		timeout := time.After(5 * time.Minute)

		select {
		case <-collector.done:
			// Success - send final event with response
			response := collector.getResponse()
			select {
			case eventCh <- StreamEvent{
				Type:     "done",
				Done:     true,
				Response: response,
			}:
			case <-ctx.Done():
			}

		case err := <-collector.errors:
			// Error during processing
			select {
			case eventCh <- StreamEvent{
				Type:  "error",
				Error: err,
				Done:  true,
			}:
			case <-ctx.Done():
			}

		case <-ctx.Done():
			// Context cancelled
			select {
			case eventCh <- StreamEvent{
				Type:  "error",
				Error: ctx.Err(),
				Done:  true,
			}:
			default:
			}

		case <-timeout:
			// Timeout
			select {
			case eventCh <- StreamEvent{
				Type:  "error",
				Error: fmt.Errorf("operation timeout after 5 minutes"),
				Done:  true,
			}:
			case <-ctx.Done():
			}
		}
	}()

	return eventCh, nil
}

// getOrCreateManagerSession gets or creates the manager session for this SDK session
func (s *Session) getOrCreateManagerSession(ctx context.Context, eventHandler manager.EventHandler) (*manager.Session, error) {
	// Try to get existing session first
	mgrSession, err := s.sdk.manager.GetSession(s.conversationID)
	if err == nil {
		// Session exists, return it
		return mgrSession, nil
	}

	// Session doesn't exist, create it
	cwd := s.workingDirectory
	if cwd == "" {
		cwd, _ = os.Getwd()
		if cwd == "" {
			cwd = "."
		}
	}

	approvalPolicy := s.approvalPolicy
	if approvalPolicy == "" {
		approvalPolicy = "auto"
	}

	sandboxMode := s.sandboxPolicy
	if sandboxMode == "" {
		sandboxMode = "native"
	}

	model := s.model
	if model == "" {
		model = "claude-sonnet-4-5"
	}

	turnCtx := &manager.TurnContext{
		Cwd:            cwd,
		ApprovalPolicy: approvalPolicy,
		SandboxPolicy: protocol.SandboxPolicy{
			Mode: sandboxMode,
		},
		Model: model,
	}

	cfg := manager.SessionConfig{
		ID:            s.conversationID,
		Client:        s.sdk.client.Internal(),
		TurnContext:   turnCtx,
		EventHandlers: []manager.EventHandler{eventHandler},
		Orchestrator:  s.sdk.orchestrator,
	}

	return s.sdk.manager.CreateSession(ctx, cfg)
}

// responseCollector accumulates response data from protocol events
type responseCollector struct {
	mu           sync.Mutex
	content      string
	tokenUsage   TokenUsage
	finishReason string
	done         chan struct{}
	errors       chan error
	eventCh      chan StreamEvent
	ctx          context.Context
	completed    bool
}

// handleEvent processes protocol events and accumulates response data
func (rc *responseCollector) handleEvent(ctx context.Context, event *protocol.Event) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Don't process events after completion
	if rc.completed {
		return nil
	}

	switch msg := event.Msg.(type) {
	case *protocol.EventAgentMessageDelta:
		// Accumulate content deltas
		rc.content += msg.Delta
		// Forward delta to caller
		select {
		case rc.eventCh <- StreamEvent{
			Type:  "agent_message_delta",
			Delta: msg.Delta,
		}:
		case <-rc.ctx.Done():
			return rc.ctx.Err()
		}

	case *protocol.EventTokenCount:
		// Update token usage
		if msg.Info != nil {
			rc.tokenUsage = TokenUsage{
				InputTokens:  msg.Info.TotalTokenUsage.InputTokens,
				OutputTokens: msg.Info.TotalTokenUsage.OutputTokens,
				TotalTokens:  msg.Info.TotalTokenUsage.TotalTokens,
			}
		}

	case *protocol.EventTaskComplete:
		// Task completed successfully
		if msg.LastAgentMessage != nil && *msg.LastAgentMessage != "" {
			rc.content = *msg.LastAgentMessage
		}
		rc.finishReason = "stop"
		rc.completed = true
		close(rc.done)

	case *protocol.EventError:
		// Error occurred
		rc.completed = true
		select {
		case rc.errors <- fmt.Errorf("turn processing error: %s", msg.Message):
		default:
		}
		close(rc.done)
	}

	return nil
}

// getResponse returns the accumulated response
func (rc *responseCollector) getResponse() *Response {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	return &Response{
		Content:      rc.content,
		FinishReason: rc.finishReason,
		TokenUsage:   rc.tokenUsage,
	}
}
