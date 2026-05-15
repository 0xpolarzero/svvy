package manager

import (
	"fmt"
	"strings"

	"github.com/evmts/codex/codex-go/internal/client"
	"github.com/evmts/codex/codex-go/internal/protocol"
)

// ConversationMessage represents a reconstructed message in the conversation history.
type ConversationMessage struct {
	Role    string // "user", "assistant", "tool", "tool_result"
	Content string
	// For tool calls
	ToolCallID   string
	ToolName     string
	ToolArgs     string
	// For tool results
	ToolResultID string
	ExitCode     *int
}

// SessionReconstructedState contains the reconstructed state from history.
type SessionReconstructedState struct {
	// Conversation history
	Messages []ConversationMessage

	// Session state
	LastAgentMessage string
	TokenUsage       *protocol.TokenUsage
	TurnContext      *TurnContext

	// State validation
	HasIncompleteTurn    bool
	HasPendingApproval   bool
	LastSubmissionID     string
	LastSubmissionType   string
	InterruptedTurnID    string

	// Statistics
	TotalTurns           int
	CompletedTurns       int
	TotalToolExecutions  int
}

// ReconstructStateFromHistory analyzes history events and submissions to rebuild session state.
func ReconstructStateFromHistory(submissions []*protocol.Submission, events []*protocol.Event) (*SessionReconstructedState, error) {
	state := &SessionReconstructedState{
		Messages: make([]ConversationMessage, 0),
		TokenUsage: &protocol.TokenUsage{
			InputTokens:  0,
			OutputTokens: 0,
			TotalTokens:  0,
		},
		TurnContext: &TurnContext{
			Cwd:            ".",
			ApprovalPolicy: "auto",
			SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
			Model:          "",
		},
	}

	// Track active turns and their state
	activeTurns := make(map[string]*turnState)

	// Track tool calls within each turn
	toolCalls := make(map[string]*toolCallInfo)

	// Process submissions first to understand turn structure
	for _, sub := range submissions {
		state.LastSubmissionID = sub.ID
		state.LastSubmissionType = sub.Op.OpType()

		switch op := sub.Op.(type) {
		case *protocol.OpUserTurn, *protocol.OpUserInput:
			state.TotalTurns++
			activeTurns[sub.ID] = &turnState{
				submissionID:  sub.ID,
				isComplete:    false,
				isInterrupted: false,
			}

			// Extract user message
			var items []protocol.UserInput
			if userTurn, ok := op.(*protocol.OpUserTurn); ok {
				items = userTurn.Items
				// Update turn context from user turn
				state.TurnContext.Cwd = userTurn.Cwd
				state.TurnContext.ApprovalPolicy = userTurn.ApprovalPolicy
				state.TurnContext.SandboxPolicy = userTurn.SandboxPolicy
				state.TurnContext.Model = userTurn.Model
				state.TurnContext.Effort = userTurn.Effort
				state.TurnContext.Summary = userTurn.Summary
			} else if userInput, ok := op.(*protocol.OpUserInput); ok {
				items = userInput.Items
			}

			// Build user message content
			var contentParts []string
			for _, item := range items {
				if item.Type == "text" && item.Text != nil {
					contentParts = append(contentParts, *item.Text)
				} else if item.Type == "image_url" && item.ImageURL != nil {
					contentParts = append(contentParts, fmt.Sprintf("[Image: %s]", *item.ImageURL))
				}
			}
			if len(contentParts) > 0 {
				state.Messages = append(state.Messages, ConversationMessage{
					Role:    "user",
					Content: strings.Join(contentParts, "\n"),
				})
			}

		case *protocol.OpInterrupt:
			// Mark the last active turn as interrupted
			if state.LastSubmissionID != "" {
				if ts, ok := activeTurns[state.LastSubmissionID]; ok {
					ts.isInterrupted = true
					state.InterruptedTurnID = state.LastSubmissionID
				}
			}

		case *protocol.OpOverrideTurnContext:
			// Update turn context
			if op.Cwd != nil {
				state.TurnContext.Cwd = *op.Cwd
			}
			if op.ApprovalPolicy != nil {
				state.TurnContext.ApprovalPolicy = *op.ApprovalPolicy
			}
			if op.SandboxPolicy != nil {
				state.TurnContext.SandboxPolicy = *op.SandboxPolicy
			}
			if op.Model != nil {
				state.TurnContext.Model = *op.Model
			}
			if op.Effort != nil {
				state.TurnContext.Effort = op.Effort
			}
			if op.Summary != nil {
				state.TurnContext.Summary = *op.Summary
			}
		}
	}

	// Process events to extract assistant messages, tool calls, and state updates
	agentMessageDeltas := make(map[string]*strings.Builder) // submissionID -> accumulated text

	for _, evt := range events {
		switch msg := evt.Msg.(type) {
		case *protocol.EventAgentMessage:
			// Complete agent message (rare in streaming)
			state.Messages = append(state.Messages, ConversationMessage{
				Role:    "assistant",
				Content: msg.Message,
			})
			state.LastAgentMessage = msg.Message

		case *protocol.EventAgentMessageDelta:
			// Accumulate deltas for this submission
			if _, ok := agentMessageDeltas[evt.ID]; !ok {
				agentMessageDeltas[evt.ID] = &strings.Builder{}
			}
			agentMessageDeltas[evt.ID].WriteString(msg.Delta)

		case *protocol.EventUserMessage:
			// Synthetic user message event (uncommon but handle it)
			state.Messages = append(state.Messages, ConversationMessage{
				Role:    "user",
				Content: msg.Message,
			})

		case *protocol.EventExecCommandBegin:
			// Tool call started
			state.TotalToolExecutions++
			toolCalls[msg.CallID] = &toolCallInfo{
				callID:   msg.CallID,
				command:  msg.Command,
				cwd:      msg.Cwd,
				hasBegun: true,
			}

		case *protocol.EventExecCommandEnd:
			// Tool call completed
			if tc, ok := toolCalls[msg.CallID]; ok {
				tc.hasEnded = true
				tc.exitCode = msg.ExitCode
				tc.output = msg.AggregatedOutput
			}

			// Add tool result message
			exitCode := msg.ExitCode
			state.Messages = append(state.Messages, ConversationMessage{
				Role:         "tool_result",
				Content:      msg.AggregatedOutput,
				ToolResultID: msg.CallID,
				ExitCode:     &exitCode,
			})

		case *protocol.EventTokenCount:
			// Update token usage (use TotalTokenUsage for session cumulative, copy to avoid pointer aliasing)
			if msg.Info != nil {
				usage := msg.Info.TotalTokenUsage // Copy the value
				state.TokenUsage = &usage
			}

		case *protocol.EventTaskComplete:
			// Turn completed successfully
			if ts, ok := activeTurns[evt.ID]; ok {
				ts.isComplete = true
				state.CompletedTurns++
			}
			if msg.LastAgentMessage != nil {
				state.LastAgentMessage = *msg.LastAgentMessage
			}

		case *protocol.EventTaskStarted:
			// Turn started (no specific action needed for reconstruction)

		case *protocol.EventError:
			// Error in turn processing
			if ts, ok := activeTurns[evt.ID]; ok {
				ts.hasError = true
				ts.errorMessage = msg.Message
			}
		}
	}

	// Finalize agent messages from accumulated deltas
	for submissionID, builder := range agentMessageDeltas {
		content := builder.String()
		if content != "" {
			state.Messages = append(state.Messages, ConversationMessage{
				Role:    "assistant",
				Content: content,
			})
			state.LastAgentMessage = content
		}
		// Check if this turn completed
		if ts, ok := activeTurns[submissionID]; ok && ts.isComplete {
			// Already marked complete above
		}
	}

	// Add tool call messages (before their results)
	// We need to insert these in the right order, but for simplicity we'll add them at the end
	// In a production system, we'd maintain proper ordering
	for _, tc := range toolCalls {
		if tc.hasBegun {
			cmdStr := strings.Join(tc.command, " ")
			state.Messages = append(state.Messages, ConversationMessage{
				Role:       "tool",
				ToolCallID: tc.callID,
				ToolName:   extractToolName(tc.command),
				Content:    cmdStr,
			})
		}
	}

	// Validate session state
	// Check for incomplete turns
	for _, ts := range activeTurns {
		if !ts.isComplete && !ts.isInterrupted {
			state.HasIncompleteTurn = true
			break
		}
	}

	// Check for pending approvals (would need to track approval requests in events)
	// This is a simplified check - in production, you'd track EventApprovalRequest events

	return state, nil
}

// Helper types

type turnState struct {
	submissionID  string
	isComplete    bool
	isInterrupted bool
	hasError      bool
	errorMessage  string
}

type toolCallInfo struct {
	callID   string
	command  []string
	cwd      string
	hasBegun bool
	hasEnded bool
	exitCode int
	output   string
}

func extractToolName(command []string) string {
	if len(command) == 0 {
		return "unknown"
	}
	// For shell commands like ["sh", "-c", "..."], return "shell"
	if command[0] == "sh" || command[0] == "bash" {
		return "shell"
	}
	return command[0]
}

// ValidateResumedState checks if the reconstructed state is safe to resume from.
func ValidateResumedState(state *SessionReconstructedState) error {
	if state == nil {
		return fmt.Errorf("state is nil")
	}

	// Check for incomplete turns
	if state.HasIncompleteTurn {
		// This is actually OK - we can resume from an incomplete turn
		// The session should start in idle state
	}

	// Check for pending approvals
	if state.HasPendingApproval {
		return fmt.Errorf("cannot resume session with pending approval - approval state cannot be reconstructed")
	}

	// Validate turn context
	if state.TurnContext == nil {
		return fmt.Errorf("turn context is nil")
	}

	if state.TurnContext.Cwd == "" {
		return fmt.Errorf("turn context cwd is empty")
	}

	// Token usage should be non-nil but can be zero
	if state.TokenUsage == nil {
		return fmt.Errorf("token usage is nil")
	}

	return nil
}

// ConvertToClientMessages converts reconstructed conversation messages to client messages.
// This is used when feeding history to the model on resume.
func ConvertToClientMessages(convMessages []ConversationMessage) []client.Message {
	if len(convMessages) == 0 {
		return nil
	}

	var messages []client.Message
	for _, msg := range convMessages {
		// Skip tool messages as they're typically not included in conversation history for the model
		// (tool results are, but tool invocations are already in assistant messages)
		if msg.Role == "tool" {
			continue
		}

		// Convert based on role
		switch msg.Role {
		case "user":
			messages = append(messages, client.NewUserMessage(msg.Content))

		case "assistant":
			messages = append(messages, client.Message{
				Role:    "assistant",
				Content: msg.Content,
			})

		case "tool_result":
			// Tool results should be formatted as tool messages
			messages = append(messages, client.NewToolMessage(msg.ToolResultID, msg.Content))
		}
	}

	return messages
}
