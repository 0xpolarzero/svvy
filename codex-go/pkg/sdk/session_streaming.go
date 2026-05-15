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
// This is shared between Submit() and SubmitStream().
func (s *Session) submitInternal(ctx context.Context, message string) (<-chan StreamEvent, error) {
	// Create event channel for streaming events back to caller
	eventCh := make(chan StreamEvent, 10)

	// Start async submission processing
	go func() {
		defer close(eventCh)

		// Build the operation
		op, err := s.buildUserTurnOp(message)
		if err != nil {
			eventCh <- StreamEvent{
				Type:  "error",
				Error: fmt.Errorf("failed to build operation: %w", err),
				Done:  true,
			}
			return
		}

		// Create a result collector to accumulate response data
		collector := &eventCollector{
			content:      "",
			tokenUsage:   TokenUsage{},
			finishReason: "stop",
		}

		// Create a channel to receive events from our handler
		handlerEventCh := make(chan *protocol.Event, 10)

		// Create event handler that forwards events to our channel
		eventHandler := func(ctx context.Context, event *protocol.Event) error {
			select {
			case handlerEventCh <- event:
			case <-ctx.Done():
				return ctx.Err()
			default:
				// Don't block if channel is full
			}
			return nil
		}

		// Create temporary session with event handler for this specific turn
		tempSessionID := fmt.Sprintf("%s_turn_%d", s.conversationID, time.Now().UnixNano())
		tempSession, err := s.sdk.manager.CreateSession(ctx, manager.SessionConfig{
			ID:            tempSessionID,
			Client:        s.sdk.client.Internal(),
			TurnContext:   op.TurnContext,
			EventHandlers: []manager.EventHandler{eventHandler},
			Orchestrator:  s.sdk.orchestrator,
		})
		if err != nil {
			eventCh <- StreamEvent{
				Type:  "error",
				Error: fmt.Errorf("failed to create manager session: %w", err),
				Done:  true,
			}
			return
		}

		// Clean up temporary session when done
		defer func() {
			_ = s.sdk.manager.CloseSession(tempSessionID)
		}()

		// Start event processor
		processorDone := make(chan struct{})
		go func() {
			defer close(processorDone)
			for {
				select {
				case event := <-handlerEventCh:
					if event == nil {
						return
					}
					if done := s.processEvent(event, collector, eventCh, ctx); done {
						return
					}
				case <-ctx.Done():
					return
				case <-tempSession.Context().Done():
					return
				}
			}
		}()

		// Submit the operation to the manager
		err = s.sdk.manager.SubmitOp(ctx, tempSessionID, op.Op)
		if err != nil {
			eventCh <- StreamEvent{
				Type:  "error",
				Error: fmt.Errorf("failed to submit operation: %w", err),
				Done:  true,
			}
			return
		}

		// Wait for completion with timeout
		timeout := time.After(5 * time.Minute)
		select {
		case <-processorDone:
			// Event processor finished
		case <-timeout:
			eventCh <- StreamEvent{
				Type:  "error",
				Error: fmt.Errorf("operation timeout after 5 minutes"),
				Done:  true,
			}
		case <-ctx.Done():
			eventCh <- StreamEvent{
				Type:  "error",
				Error: ctx.Err(),
				Done:  true,
			}
		}
	}()

	return eventCh, nil
}

// userTurnOperation wraps the operation and turn context
type userTurnOperation struct {
	Op          *protocol.OpUserTurn
	TurnContext *manager.TurnContext
}

// buildUserTurnOp builds a protocol operation from the user message.
func (s *Session) buildUserTurnOp(message string) (*userTurnOperation, error) {
	// Get working directory
	cwd := s.workingDirectory
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			cwd = "."
		}
	}

	// Apply default values if not set
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
		model = "claude-sonnet-4-5" // Default model
	}

	// Build message with system prompt if provided
	fullMessage := message
	if s.systemPrompt != "" {
		fullMessage = fmt.Sprintf("System: %s\n\nUser: %s", s.systemPrompt, message)
	}

	op := &protocol.OpUserTurn{
		Items: []protocol.UserInput{
			{
				Type: "text",
				Text: &fullMessage,
			},
		},
		Cwd:            cwd,
		ApprovalPolicy: approvalPolicy,
		SandboxPolicy: protocol.SandboxPolicy{
			Mode: sandboxMode,
		},
		Model: model,
	}

	turnCtx := &manager.TurnContext{
		Cwd:            cwd,
		ApprovalPolicy: approvalPolicy,
		SandboxPolicy: protocol.SandboxPolicy{
			Mode: sandboxMode,
		},
		Model:    model,
		MaxTurns: 10,
	}

	return &userTurnOperation{
		Op:          op,
		TurnContext: turnCtx,
	}, nil
}

// processEvent processes a protocol event and forwards it to the caller.
// Returns true if processing should stop (done or error).
func (s *Session) processEvent(event *protocol.Event, collector *eventCollector, eventCh chan<- StreamEvent, ctx context.Context) bool {
	// Process different event types
	switch msg := event.Msg.(type) {
	case *protocol.EventAgentMessageDelta:
		// Accumulate deltas
		collector.mu.Lock()
		collector.content += msg.Delta
		collector.mu.Unlock()

		// Forward delta to caller
		select {
		case eventCh <- StreamEvent{
			Type:  "content_delta",
			Delta: msg.Delta,
		}:
		case <-ctx.Done():
			return true
		}

	case *protocol.EventAgentReasoningDelta:
		// Forward reasoning delta to caller
		select {
		case eventCh <- StreamEvent{
			Type:  "reasoning_delta",
			Delta: msg.Delta,
		}:
		case <-ctx.Done():
			return true
		}

	case *protocol.EventTokenCount:
		// Extract token usage
		if msg.Info != nil {
			collector.mu.Lock()
			collector.tokenUsage = TokenUsage{
				InputTokens:  msg.Info.TotalTokenUsage.InputTokens,
				OutputTokens: msg.Info.TotalTokenUsage.OutputTokens,
				TotalTokens:  msg.Info.TotalTokenUsage.TotalTokens,
			}
			collector.mu.Unlock()
		}

	case *protocol.EventTaskComplete:
		// Mark as complete
		collector.mu.Lock()
		collector.done = true
		if msg.LastAgentMessage != nil && *msg.LastAgentMessage != "" {
			collector.content = *msg.LastAgentMessage
		}

		// Send final event
		response := &Response{
			Content:      collector.content,
			FinishReason: collector.finishReason,
			TokenUsage:   collector.tokenUsage,
		}

		// Add assistant message to history
		assistantMsg := &Message{
			Role:    "assistant",
			Content: collector.content,
		}
		s.addMessage(assistantMsg)

		collector.mu.Unlock()

		select {
		case eventCh <- StreamEvent{
			Type:     "done",
			Done:     true,
			Response: response,
		}:
		case <-ctx.Done():
		}

		return true // Done processing

	case *protocol.EventError:
		// Forward error
		collector.mu.Lock()
		collector.err = fmt.Errorf("%s", msg.Message)
		collector.done = true
		collector.mu.Unlock()

		select {
		case eventCh <- StreamEvent{
			Type:  "error",
			Error: collector.err,
			Done:  true,
		}:
		case <-ctx.Done():
		}

		return true // Done processing

	case *protocol.EventExecCommandBegin:
		// Tool call started - create a minimal tool call representation
		toolCall := ToolCall{
			ID:   msg.CallID,
			Name: "exec",
			Arguments: fmt.Sprintf(`{"command": %v, "cwd": "%s"}`,
				msg.Command, msg.Cwd),
		}
		select {
		case eventCh <- StreamEvent{
			Type:     "tool_call_delta",
			ToolCall: &toolCall,
		}:
		case <-ctx.Done():
			return true
		}

	case *protocol.EventExecCommandEnd:
		// Tool call completed
		toolCall := ToolCall{
			ID:     msg.CallID,
			Name:   "exec",
			Result: fmt.Sprintf("Exit code: %d", msg.ExitCode),
		}
		if msg.Error != "" {
			toolCall.Error = msg.Error
		}
		select {
		case eventCh <- StreamEvent{
			Type:     "tool_call_delta",
			ToolCall: &toolCall,
		}:
		case <-ctx.Done():
			return true
		}
	}

	return false // Continue processing
}

// eventCollector accumulates data from protocol events
type eventCollector struct {
	content      string
	tokenUsage   TokenUsage
	finishReason string
	done         bool
	err          error
	mu           sync.Mutex
}
