package plan

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// EventEmitter is the interface for emitting protocol events.
// This allows the plan tool to emit EventPlanUpdate events.
type EventEmitter interface {
	EmitEvent(ctx context.Context, event *protocol.Event) error
}

// UpdatePlanTool implements the update_plan tool for AI model task tracking.
// This tool allows the model to create and update task lists during execution.
type UpdatePlanTool struct {
	// emitter is used to emit EventPlanUpdate events (optional).
	emitter EventEmitter
}

// NewUpdatePlanTool creates a new update_plan tool.
// The emitter can be nil for testing or if events are not needed.
func NewUpdatePlanTool(emitter EventEmitter) *UpdatePlanTool {
	return &UpdatePlanTool{
		emitter: emitter,
	}
}

// NewBasicUpdatePlanTool creates a plan tool without event emission.
// This is useful for testing or when the tool is used in isolation.
func NewBasicUpdatePlanTool() *UpdatePlanTool {
	return &UpdatePlanTool{
		emitter: nil,
	}
}

// Name returns the tool name.
func (t *UpdatePlanTool) Name() string {
	return "update_plan"
}

// Execute runs the update_plan tool.
func (t *UpdatePlanTool) Execute(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
	// Parse the arguments
	var args UpdatePlanArgs
	if err := json.Unmarshal([]byte(req.Arguments), &args); err != nil {
		return nil, runtime.NewToolErrorWithCause(
			runtime.ErrorInvalidArguments,
			"failed to parse update_plan arguments",
			err,
		)
	}

	// Validate the plan update
	if err := args.Validate(); err != nil {
		return nil, runtime.NewToolErrorWithCause(
			runtime.ErrorInvalidArguments,
			"invalid plan update",
			err,
		)
	}

	// Create plan structure for the event
	planData := make(map[string]interface{})
	if args.Explanation != nil {
		planData["explanation"] = *args.Explanation
	}

	// Convert tasks to map format for protocol
	tasksData := make([]map[string]interface{}, len(args.Tasks))
	for i, task := range args.Tasks {
		tasksData[i] = map[string]interface{}{
			"content":     task.Content,
			"status":      string(task.Status),
			"active_form": task.ActiveForm,
		}
	}
	planData["tasks"] = tasksData

	// Emit the EventPlanUpdate event
	if t.emitter != nil {
		event := &protocol.Event{
			ID: req.CallID,
			Msg: &protocol.EventPlanUpdate{
				Plan: planData,
			},
		}
		// Emit but don't fail the tool call if emission fails
		_ = t.emitter.EmitEvent(ctx, event)
	}

	// Return success response
	success := true
	return &runtime.ToolResponse{
		Content: "Plan updated successfully",
		Success: &success,
		Metadata: map[string]interface{}{
			"task_count": len(args.Tasks),
		},
	}, nil
}

// ApprovalKey returns the approval key for caching.
// Plan updates don't require approval, so we return an empty string.
func (t *UpdatePlanTool) ApprovalKey(req *runtime.ToolRequest) string {
	return ""
}

// NeedsInitialApproval returns false because plan updates don't require approval.
func (t *UpdatePlanTool) NeedsInitialApproval(req *runtime.ToolRequest, approvalPolicy runtime.ApprovalPolicy, sandboxPolicy runtime.SandboxPolicy) bool {
	return false
}

// NeedsRetryApproval returns false because plan updates don't retry.
func (t *UpdatePlanTool) NeedsRetryApproval(approvalPolicy runtime.ApprovalPolicy) bool {
	return false
}

// SandboxPreference returns SandboxForbid because plan updates are pure metadata.
func (t *UpdatePlanTool) SandboxPreference() runtime.SandboxPreference {
	return runtime.SandboxForbid
}

// EscalateOnFailure returns false because plan updates don't interact with sandbox.
func (t *UpdatePlanTool) EscalateOnFailure() bool {
	return false
}

// WantsEscalatedFirstAttempt returns false because plan updates don't need escalation.
func (t *UpdatePlanTool) WantsEscalatedFirstAttempt(req *runtime.ToolRequest) bool {
	return false
}

// SupportsParallel returns true because plan updates can run concurrently.
func (t *UpdatePlanTool) SupportsParallel() bool {
	return true
}

// SandboxRetryData returns nil because plan updates don't support sandbox retry.
func (t *UpdatePlanTool) SandboxRetryData(req *runtime.ToolRequest) *runtime.SandboxRetryData {
	return nil
}

// GetToolSpec returns the tool specification for the AI model.
func GetToolSpec() runtime.ToolSpec {
	return runtime.ToolSpec{
		Name: "update_plan",
		Description: `Updates the task plan.

Provide an optional explanation and a list of plan items, each with:
- content: imperative form (e.g., "Run tests")
- status: one of "pending", "in_progress", "completed"
- active_form: present continuous form (e.g., "Running tests")

At most one task can have status "in_progress" at a time.`,
		ParametersSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"explanation": map[string]interface{}{
					"type":        "string",
					"description": "Optional explanation of the plan update",
				},
				"tasks": map[string]interface{}{
					"type":        "array",
					"description": "The list of tasks in the plan",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"content": map[string]interface{}{
								"type":        "string",
								"description": "Imperative form describing what needs to be done",
							},
							"status": map[string]interface{}{
								"type":        "string",
								"description": "Task status: pending, in_progress, or completed",
								"enum":        []string{"pending", "in_progress", "completed"},
							},
							"active_form": map[string]interface{}{
								"type":        "string",
								"description": "Present continuous form shown during execution",
							},
						},
						"required": []string{"content", "status", "active_form"},
					},
				},
			},
			"required": []string{"tasks"},
		},
		Strict:           false,
		SupportsParallel: true,
	}
}

// FormatPlanForDisplay formats a plan for display in the UI.
// This is a helper function for rendering plan updates.
func FormatPlanForDisplay(planData interface{}) string {
	planMap, ok := planData.(map[string]interface{})
	if !ok {
		return "Invalid plan format"
	}

	var result string

	// Add explanation if present
	if explanation, ok := planMap["explanation"].(string); ok && explanation != "" {
		result += fmt.Sprintf("Plan: %s\n\n", explanation)
	}

	// Add tasks
	if tasksData, ok := planMap["tasks"].([]interface{}); ok {
		for i, taskData := range tasksData {
			if taskMap, ok := taskData.(map[string]interface{}); ok {
				status := "?"
				if s, ok := taskMap["status"].(string); ok {
					switch s {
					case "pending":
						status = "⏸"
					case "in_progress":
						status = "▶"
					case "completed":
						status = "✓"
					}
				}

				content := "Unknown task"
				if c, ok := taskMap["content"].(string); ok {
					content = c
				}

				result += fmt.Sprintf("%d. [%s] %s\n", i+1, status, content)
			}
		}
	}

	return result
}
