package manager

import (
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/evmts/codex/codex-go/internal/client"
    "github.com/evmts/codex/codex-go/internal/protocol"
    "github.com/evmts/codex/codex-go/internal/tools/orchestrator"
    "github.com/evmts/codex/codex-go/internal/tools/runtime"
    "github.com/evmts/codex/codex-go/internal/tools/schema"
)

// TurnProcessor handles the execution of a turn including:
// - Building the completion request
// - Streaming responses
// - Event emission
// - State updates
// - Approval workflow coordination
type TurnProcessor struct {
	session         *Session
	approvalHandler *SessionApprovalHandler
	submissionID    string
}

// NewTurnProcessor creates a new turn processor for a session.
func NewTurnProcessor(session *Session) *TurnProcessor {
	return &TurnProcessor{
		session: session,
	}
}

// NewTurnProcessorWithApprovalHandler creates a turn processor with approval handler.
func NewTurnProcessorWithApprovalHandler(session *Session, submissionID string) *TurnProcessor {
	approvalHandler := NewSessionApprovalHandler(session, submissionID)
	// Register approval handler with session
	session.SetApprovalHandler(approvalHandler)

	return &TurnProcessor{
		session:         session,
		approvalHandler: approvalHandler,
		submissionID:    submissionID,
	}
}

// ProcessTurn executes a user turn and streams events back.
// This is the main entry point for turn execution.
func (tp *TurnProcessor) ProcessTurn(ctx context.Context, submissionID string, op *protocol.OpUserTurn) error {
	// Emit task started event
	if err := tp.emitTaskStarted(ctx, submissionID); err != nil {
		return fmt.Errorf("failed to emit task started: %w", err)
	}

	// Build the completion request
	req, err := tp.buildRequest(op)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}

	// Stream the response
	eventChan, err := tp.session.client.Stream(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to start stream: %w", err)
	}

	// Process stream events (handles multi-turn automatically)
	if err := tp.processStream(ctx, submissionID, eventChan, req.Messages); err != nil {
		return fmt.Errorf("stream processing failed: %w", err)
	}

	// Emit task complete event
	if err := tp.emitTaskComplete(ctx, submissionID); err != nil {
		return fmt.Errorf("failed to emit task complete: %w", err)
	}

	return nil
}

// buildRequest constructs a chat completion request from the user turn.
func (tp *TurnProcessor) buildRequest(op *protocol.OpUserTurn) (*client.ChatCompletionRequest, error) {
	// Build messages from user input
	var messages []client.Message

	// Prepend reconstructed history if available (for resume scenarios)
	reconstructedHistory := tp.session.GetReconstructedHistory()
	if len(reconstructedHistory) > 0 {
		messages = append(messages, reconstructedHistory...)
	}

	// Add user message from current turn
	// Collect all items and combine them into a single message
	var userContent strings.Builder
	var hasContent bool

	for _, item := range op.Items {
		if item.Type == "text" && item.Text != nil {
			if hasContent {
				userContent.WriteString("\n\n")
			}
			userContent.WriteString(*item.Text)
			hasContent = true
		} else if item.Type == "path" && item.Path != nil {
			// Read and include file content
			content, err := os.ReadFile(*item.Path)
			if err != nil {
				// Include error message in context
				if hasContent {
					userContent.WriteString("\n\n")
				}
				userContent.WriteString(fmt.Sprintf("[Error reading file %s: %v]", filepath.Base(*item.Path), err))
				hasContent = true
				continue
			}

			// Format file content for LLM
			if hasContent {
				userContent.WriteString("\n\n")
			}
			userContent.WriteString(fmt.Sprintf("[File: %s]\n%s\n[End File: %s]",
				filepath.Base(*item.Path), string(content), filepath.Base(*item.Path)))
			hasContent = true
		}
	}

	// Add the combined user message
	if hasContent {
		messages = append(messages, client.NewUserMessage(userContent.String()))
	}

	// Build request
	req := &client.ChatCompletionRequest{
		Model:    op.Model,
		Messages: messages,
		Stream:   true,
	}

	// Add tools if orchestrator is available
	if tp.session.Orchestrator() != nil {
		orch := tp.session.Orchestrator()
		registry := orch.GetRegistry()
		if registry != nil {
			// Generate tool schemas from registry
			toolSchemas := schema.GenerateToolSchemas(registry)
			if len(toolSchemas) > 0 {
				req.Tools = toolSchemas
				req.ParallelToolCalls = true
			}
		}
	}

	// Add reasoning configuration if specified
	if op.Effort != nil || op.Summary != "" {
		req.Reasoning = &client.Reasoning{}
		if op.Effort != nil {
			req.Reasoning.Effort = op.Effort
		}
		if op.Summary != "" {
			req.Reasoning.Summary = &op.Summary
		}
	}

	return req, nil
}

// buildRequestWithMessages constructs a chat completion request with the given messages.
// This is used for follow-up requests that include tool results.
func (tp *TurnProcessor) buildRequestWithMessages(messages []client.Message) (*client.ChatCompletionRequest, error) {
	turnCtx := tp.session.GetTurnContext()

	// Build request with existing messages
	req := &client.ChatCompletionRequest{
		Model:    turnCtx.Model,
		Messages: messages,
		Stream:   true,
	}

	// Add tools if orchestrator is available (needed for multi-turn tool use)
	if tp.session.Orchestrator() != nil {
		orch := tp.session.Orchestrator()
		registry := orch.GetRegistry()
		if registry != nil {
			// Generate tool schemas from registry
			toolSchemas := schema.GenerateToolSchemas(registry)
			if len(toolSchemas) > 0 {
				req.Tools = toolSchemas
				req.ParallelToolCalls = true
			}
		}
	}

	// Add reasoning configuration if specified in turn context
	if turnCtx.Effort != nil || turnCtx.Summary != "" {
		req.Reasoning = &client.Reasoning{}
		if turnCtx.Effort != nil {
			req.Reasoning.Effort = turnCtx.Effort
		}
		if turnCtx.Summary != "" {
			req.Reasoning.Summary = &turnCtx.Summary
		}
	}

	return req, nil
}

// processStream processes events from the client stream and emits protocol events.
// This is the main orchestration function that handles multi-turn streaming with tool execution.
func (tp *TurnProcessor) processStream(ctx context.Context, submissionID string, eventChan <-chan client.StreamEvent, initialMessages []client.Message) error {
	// Track conversation history across turns (start with initial messages)
	conversationMessages := make([]client.Message, len(initialMessages))
	copy(conversationMessages, initialMessages)

	// Track accumulated token usage across all turns
	var totalUsage client.TokenUsage

	// Process the initial stream
	result, err := tp.processSingleStream(ctx, submissionID, eventChan, &totalUsage)
	if err != nil {
		return err
	}

	// Update session state with results
	if result.lastMessage != "" {
		tp.session.SetLastAgentMessage(result.lastMessage)
	}

	// If no tool calls were made, we're done
	if len(result.toolCalls) == 0 {
		return nil
	}

    // Multi-turn loop: continue as long as the model requests tools
    // Add a safety guard to avoid infinite loops
    turnCtx := tp.session.GetTurnContext()
    maxTurns := turnCtx.MaxTurns
    if maxTurns <= 0 {
        maxTurns = 10 // Default to 10 if not configured
    }
    turns := 0
    for len(result.toolCalls) > 0 {
        turns++
        if turns > maxTurns {
            return fmt.Errorf("maximum multi-turn iterations exceeded: %d", maxTurns)
        }
		// Build assistant message with tool calls to add to conversation
		assistantMsg := client.Message{
			Role:      "assistant",
			Content:   result.lastMessage,
			Reasoning: result.reasoningContent,
			ToolCalls: result.toolCalls,
		}
		conversationMessages = append(conversationMessages, assistantMsg)

		// Compact conversation if nearing context window limit
		conversationMessages = tp.compactConversationIfNeeded(conversationMessages)

		// Execute tool calls and collect results
		toolMessages, err := tp.executeToolCalls(ctx, submissionID, result.toolCalls)
		if err != nil {
			return fmt.Errorf("tool execution failed: %w", err)
		}

		// Append tool result messages to conversation
		conversationMessages = append(conversationMessages, toolMessages...)

		// Build new request with updated messages including tool results
		req, err := tp.buildRequestWithMessages(conversationMessages)
		if err != nil {
			return fmt.Errorf("failed to build follow-up request: %w", err)
		}

		// Start a new stream with tool results
		newEventChan, err := tp.session.client.Stream(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to start follow-up stream: %w", err)
		}

		// Process the follow-up stream
		result, err = tp.processSingleStream(ctx, submissionID, newEventChan, &totalUsage)
		if err != nil {
			return err
		}

		// Update session state with new message
		if result.lastMessage != "" {
			tp.session.SetLastAgentMessage(result.lastMessage)
		}
	}

	return nil
}

// streamResult holds the result of processing a single stream.
type streamResult struct {
	lastMessage     string
	reasoningContent string
	toolCalls       []client.ToolCall
}

// processSingleStream processes a single stream and returns the result.
// It accumulates token usage into the provided totalUsage parameter.
func (tp *TurnProcessor) processSingleStream(ctx context.Context, submissionID string, eventChan <-chan client.StreamEvent, totalUsage *client.TokenUsage) (*streamResult, error) {
	result := &streamResult{}
	var streamUsage *client.TokenUsage

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case evt, ok := <-eventChan:
			if !ok {
				// Stream closed - accumulate token usage if available
				if streamUsage != nil {
					totalUsage.InputTokens += streamUsage.InputTokens
					totalUsage.CachedInputTokens += streamUsage.CachedInputTokens
					totalUsage.OutputTokens += streamUsage.OutputTokens
					totalUsage.ReasoningOutputTokens += streamUsage.ReasoningOutputTokens
					totalUsage.TotalTokens += streamUsage.TotalTokens

					// Update session state with cumulative usage
					tp.session.UpdateTokenUsage(&protocol.TokenUsage{
						InputTokens:           totalUsage.InputTokens,
						CachedInputTokens:     totalUsage.CachedInputTokens,
						OutputTokens:          totalUsage.OutputTokens,
						ReasoningOutputTokens: totalUsage.ReasoningOutputTokens,
						TotalTokens:           totalUsage.TotalTokens,
					})
				}
				return result, nil
			}

			// Handle errors
			if evt.Error != nil {
				return nil, fmt.Errorf("stream error: %w", evt.Error)
			}

			// Process based on event type
			switch evt.Type {
			case client.EventTypeOutputTextDelta:
				// Extract delta text
				if delta, ok := evt.Data.(string); ok {
					result.lastMessage += delta
					if err := tp.emitAgentMessageDelta(ctx, submissionID, delta); err != nil {
						return nil, err
					}
				}

			case client.EventTypeReasoningContentDelta:
				// Handle reasoning deltas
				if delta, ok := evt.Data.(string); ok {
					result.reasoningContent += delta
					if err := tp.emitAgentReasoningDelta(ctx, submissionID, delta); err != nil {
						return nil, err
					}
				}

			case client.EventTypeCompleted:
				// Handle completion
				if completedEvt, ok := evt.Data.(*client.CompletedEvent); ok {
					if completedEvt.TokenUsage != nil {
						streamUsage = completedEvt.TokenUsage
						// Emit token count event immediately with cumulative usage
						cumulativeUsage := client.TokenUsage{
							InputTokens:           totalUsage.InputTokens + streamUsage.InputTokens,
							CachedInputTokens:     totalUsage.CachedInputTokens + streamUsage.CachedInputTokens,
							OutputTokens:          totalUsage.OutputTokens + streamUsage.OutputTokens,
							ReasoningOutputTokens: totalUsage.ReasoningOutputTokens + streamUsage.ReasoningOutputTokens,
							TotalTokens:           totalUsage.TotalTokens + streamUsage.TotalTokens,
						}
						if err := tp.emitTokenCount(ctx, submissionID, &cumulativeUsage); err != nil {
							return nil, err
						}
					}
				}

			case client.EventTypeError:
				// Handle error event
				if errMsg, ok := evt.Data.(string); ok {
					return nil, fmt.Errorf("stream error event: %s", errMsg)
				}

			case client.EventTypeOutputItemDone:
				// Extract tool calls but don't execute them yet
				if toolCalls := tp.extractToolCalls(evt.Data); len(toolCalls) > 0 {
					result.toolCalls = append(result.toolCalls, toolCalls...)
				}
			}
		}
	}
}

// Event emission helpers

func (tp *TurnProcessor) emitTaskStarted(ctx context.Context, submissionID string) error {
	contextWindow := tp.session.client.GetModelContextWindow()

	event := &protocol.Event{
		ID: submissionID,
		Msg: &protocol.EventTaskStarted{
			ModelContextWindow: &contextWindow,
		},
	}

	return tp.session.EmitEvent(ctx, event)
}

func (tp *TurnProcessor) emitTaskComplete(ctx context.Context, submissionID string) error {
	lastMsg := tp.session.GetLastAgentMessage()

	event := &protocol.Event{
		ID: submissionID,
		Msg: &protocol.EventTaskComplete{
			LastAgentMessage: &lastMsg,
		},
	}

	return tp.session.EmitEvent(ctx, event)
}

func (tp *TurnProcessor) emitAgentMessageDelta(ctx context.Context, submissionID string, delta string) error {
	event := &protocol.Event{
		ID: submissionID,
		Msg: &protocol.EventAgentMessageDelta{
			Delta: delta,
		},
	}

	return tp.session.EmitEvent(ctx, event)
}

func (tp *TurnProcessor) emitAgentReasoningDelta(ctx context.Context, submissionID string, delta string) error {
	// Emit the reasoning delta event
	event := &protocol.Event{
		ID: submissionID,
		Msg: &protocol.EventAgentReasoningDelta{
			Delta: delta,
		},
	}

	if err := tp.session.EmitEvent(ctx, event); err != nil {
		return err
	}

	// Also emit raw content delta for clients that want raw reasoning
	rawEvent := &protocol.Event{
		ID: submissionID,
		Msg: &protocol.EventAgentReasoningRawContentDelta{
			Delta: delta,
		},
	}

	return tp.session.EmitEvent(ctx, rawEvent)
}

func (tp *TurnProcessor) emitError(ctx context.Context, submissionID string, errMsg string) error {
	event := &protocol.Event{
		ID: submissionID,
		Msg: &protocol.EventError{
			Message: errMsg,
		},
	}

	return tp.session.EmitEvent(ctx, event)
}

// emitTokenCount emits a token usage update.
func (tp *TurnProcessor) emitTokenCount(ctx context.Context, submissionID string, usage *client.TokenUsage) error {
    if usage == nil {
        return nil
    }
    // Update session state
    tp.session.UpdateTokenUsage(&protocol.TokenUsage{
        InputTokens:           usage.InputTokens,
        CachedInputTokens:     usage.CachedInputTokens,
        OutputTokens:          usage.OutputTokens,
        ReasoningOutputTokens: usage.ReasoningOutputTokens,
        TotalTokens:           usage.TotalTokens,
    })

    // Emit event
    contextWindow := tp.session.client.GetModelContextWindow()
    evt := &protocol.Event{
        ID: submissionID,
        Msg: &protocol.EventTokenCount{
            Info: &protocol.TokenUsageInfo{
                TotalTokenUsage: tp.session.GetTokenUsage(),
                LastTokenUsage: protocol.TokenUsage{
                    InputTokens:           usage.InputTokens,
                    CachedInputTokens:     usage.CachedInputTokens,
                    OutputTokens:          usage.OutputTokens,
                    ReasoningOutputTokens: usage.ReasoningOutputTokens,
                    TotalTokens:           usage.TotalTokens,
                },
                ModelContextWindow: &contextWindow,
            },
        },
    }
    return tp.session.EmitEvent(ctx, evt)
}

// extractToolCalls parses tool calls from the event data.
func (tp *TurnProcessor) extractToolCalls(data interface{}) []client.ToolCall {
	if tp.session.Orchestrator() == nil {
		// No orchestrator configured; ignore tool calls
		return nil
	}

	// Data is expected to be map[string]interface{}{"tool_calls": []client.ToolCall}
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}

	toolCallsVal, ok := dataMap["tool_calls"]
	if !ok || toolCallsVal == nil {
		return nil
	}

	// Assert slice of client.ToolCall
	toolCalls, ok := toolCallsVal.([]client.ToolCall)
	if !ok {
		// Try []interface{} -> convert
		list, ok2 := toolCallsVal.([]interface{})
		if !ok2 {
			return nil
		}
		toolCalls = make([]client.ToolCall, 0, len(list))
		for _, it := range list {
			if c, ok3 := it.(client.ToolCall); ok3 {
				toolCalls = append(toolCalls, c)
			}
		}
	}

	return toolCalls
}

// executeToolCalls executes tool calls and returns tool result messages.
func (tp *TurnProcessor) executeToolCalls(ctx context.Context, submissionID string, toolCalls []client.ToolCall) ([]client.Message, error) {
	if tp.session.Orchestrator() == nil {
		return nil, fmt.Errorf("no orchestrator configured")
	}

	var toolMessages []client.Message

	// Build and execute requests sequentially for now
	for _, call := range toolCalls {
        // Only handle function tool calls for now
        toolName := ""
        args := ""
        if call.Type == "function" && call.Function != nil {
            toolName = call.Function.Name
            args = call.Function.Arguments
        } else if call.Custom != nil {
            toolName = call.Custom.Name
            // For custom, map input to arguments JSON string
            args = call.Custom.Input
        } else {
            // Unsupported type for now
            continue
        }

        req := &runtime.ToolRequest{
            CallID:           call.ID,
            ToolName:         toolName,
            Arguments:        args,
            WorkingDirectory: tp.session.GetTurnContext().Cwd,
        }

        // Prepare streaming writer to emit protocol deltas
        writer := runtime.NewStreamWriter(call.ID, func(delta runtime.OutputDelta) {
            switch delta.Type {
            case runtime.DeltaStdout:
                // Encode binary data as base64 to prevent corruption
                encoded := base64.StdEncoding.EncodeToString([]byte(delta.Content))
                _ = tp.session.EmitEvent(ctx, &protocol.Event{ // nolint:errcheck
                    ID: submissionID,
                    Msg: &protocol.EventExecCommandOutputDelta{CallID: call.ID, Stream: "stdout", Chunk: encoded},
                })
            case runtime.DeltaStderr:
                // Encode binary data as base64 to prevent corruption
                encoded := base64.StdEncoding.EncodeToString([]byte(delta.Content))
                _ = tp.session.EmitEvent(ctx, &protocol.Event{ // nolint:errcheck
                    ID: submissionID,
                    Msg: &protocol.EventExecCommandOutputDelta{CallID: call.ID, Stream: "stderr", Chunk: encoded},
                })
            default:
                // Ignore status/complete here
            }
        })

        // Emit begin event with best-effort command details for shell
        begin := &protocol.EventExecCommandBegin{CallID: call.ID, Cwd: req.WorkingDirectory}
        if toolName == "shell" {
            // Try to parse {"command":"..."}
            cmd := parseShellCommandFromArgs(args)
            if cmd != "" {
                begin.Command = []string{"sh", "-c", cmd}
                begin.ParsedCmd = []interface{}{"sh", "-c", cmd}
            }
        } else {
            begin.Command = []string{toolName}
            begin.ParsedCmd = []interface{}{toolName}
        }
        _ = tp.session.EmitEvent(ctx, &protocol.Event{ID: submissionID, Msg: begin}) // nolint:errcheck

        // Build execution context
        execCtx := &runtime.ExecutionContext{
            SessionID:      tp.session.ID(),
            TurnID:         submissionID,
            OutputWriter:   writer,
            SandboxAttempt: &runtime.SandboxAttempt{Type: runtime.SandboxNone, Policy: tp.mapSandboxPolicy(tp.session.GetTurnContext().SandboxPolicy)},
            StartTime:      time.Now(),
        }

        // Execute with approval handler if available
        var result *runtime.ExecutionResult
        var err error
        
        if tp.approvalHandler != nil && tp.session.Orchestrator() != nil {
            // Create orchestrator with our approval handler for this turn
            baseOrch := tp.session.Orchestrator()
            tempOrch := orchestrator.NewOrchestrator(
                baseOrch.GetRegistry(),
                baseOrch.GetApprovalCache(),
                tp.approvalHandler.CreateApprovalHandlerFunc(),
            )
            result, err = tempOrch.Execute(ctx, req, execCtx)
        } else if tp.session.Orchestrator() != nil {
            result, err = tp.session.Orchestrator().Execute(ctx, req, execCtx)
        } else {
            err = fmt.Errorf("no orchestrator configured")
        }
        // Close writer (signals completion)
        _ = writer.Close()

        // Emit end event and collect tool result for conversation
        end := &protocol.EventExecCommandEnd{CallID: call.ID}
        var toolResultContent string

        if err != nil {
            end.Stdout = ""
            end.Stderr = err.Error()
            end.AggregatedOutput = err.Error()
            end.ExitCode = 1
            end.Duration = "0s"
            end.FormattedOutput = err.Error()
            toolResultContent = fmt.Sprintf("Error: %s", err.Error())
        } else if result != nil && result.Response != nil {
            content := result.Response.Content
            end.Stdout = content
            end.Stderr = ""
            end.AggregatedOutput = content
            // Use exit code if available
            exit := 0
            if result.Response.ExitCode != nil {
                exit = *result.Response.ExitCode
            }
            end.ExitCode = exit
            // Duration best effort
            dur := result.Response.ExecutionTime
            if dur == 0 && !result.StartTime.IsZero() && !result.EndTime.IsZero() {
                dur = result.EndTime.Sub(result.StartTime)
            }
            end.Duration = dur.String()
            end.FormattedOutput = content
            toolResultContent = content
        }
        _ = tp.session.EmitEvent(ctx, &protocol.Event{ID: submissionID, Msg: end}) // nolint:errcheck

        // Create tool message for conversation history
        toolMsg := client.NewToolMessage(call.ID, toolResultContent)
        toolMessages = append(toolMessages, toolMsg)
    }

    return toolMessages, nil
}

// parseShellCommandFromArgs extracts the "command" field from a JSON string.
func (tp *TurnProcessor) getOrchestratorWithApprovalHandler() *orchestrator.Orchestrator {
	baseOrch := tp.session.Orchestrator()
	if baseOrch == nil {
		return nil
	}

	// If we have an approval handler, create a new orchestrator with it
	if tp.approvalHandler != nil {
		// Create new orchestrator with approval handler
		return orchestrator.NewOrchestrator(
			baseOrch.GetRegistry(),
			baseOrch.GetApprovalCache(),
			tp.approvalHandler.CreateApprovalHandlerFunc(),
		)
	}

	// Otherwise use the base orchestrator
	return baseOrch
}

func parseShellCommandFromArgs(args string) string {
    type sargs struct{ Command string `json:"command"` }
    var a sargs
    if err := json.Unmarshal([]byte(args), &a); err == nil {
        return a.Command
    }
    return ""
}

// mapSandboxPolicy maps protocol.SandboxPolicy to runtime.SandboxPolicy
func (tp *TurnProcessor) mapSandboxPolicy(p protocol.SandboxPolicy) runtime.SandboxPolicy {
    switch p.Mode {
    case "read-only":
        return runtime.SandboxReadOnly
    case "workspace-write":
        return runtime.SandboxWorkspaceWrite
    default:
        return runtime.SandboxDangerFullAccess
    }
}

// compactConversationIfNeeded truncates conversation history if it's nearing the context window limit.
// This is a simple implementation that keeps the most recent messages.
func (tp *TurnProcessor) compactConversationIfNeeded(messages []client.Message) []client.Message {
	if len(messages) == 0 {
		return messages
	}

	// Get context window limit from client
	contextWindow := tp.session.client.GetModelContextWindow()
	if contextWindow <= 0 {
		return messages // No limit configured, return as-is
	}

	// Use a simple heuristic: assume average 3 tokens per message element
	// and keep only the most recent messages if we exceed 80% of context window
	estimatedTokens := int64(len(messages) * 3)
	threshold := int64(float64(contextWindow) * 0.8)

	if estimatedTokens < threshold {
		return messages // Still within safe limit
	}

	// Calculate how many messages to keep (keep at least the last 5)
	keepRatio := float64(threshold) / float64(estimatedTokens)
	keepCount := int(float64(len(messages)) * keepRatio)
	if keepCount < 5 && len(messages) > 5 {
		keepCount = 5
	}
	if keepCount >= len(messages) {
		return messages // No truncation needed
	}

	// Keep the most recent messages
	return messages[len(messages)-keepCount:]
}

// ApprovalChecker determines if an operation needs user approval.
type ApprovalChecker struct {
	policy string
}

// NewApprovalChecker creates a new approval checker with the given policy.
func NewApprovalChecker(policy string) *ApprovalChecker {
	return &ApprovalChecker{
		policy: policy,
	}
}

// NeedsApproval checks if an operation requires user approval.
func (ac *ApprovalChecker) NeedsApproval(opType string) bool {
	switch ac.policy {
	case "auto":
		// Auto-approve everything
		return false

	case "manual":
		// Require approval for everything
		return true

	case "semi-auto":
		// Require approval for potentially dangerous operations
		return opType == "exec" || opType == "patch"

	default:
		// Default to requiring approval
		return true
	}
}

// ShouldApproveExec checks if exec commands should be auto-approved.
func (ac *ApprovalChecker) ShouldApproveExec() bool {
	return ac.policy == "auto"
}

// ShouldApprovePatch checks if patches should be auto-approved.
func (ac *ApprovalChecker) ShouldApprovePatch() bool {
	return ac.policy == "auto"
}
