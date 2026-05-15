// Package plan provides task/todo management functionality for Codex Go.
//
// This package implements the plan/todo system that allows the AI model to
// create, update, and track task lists during conversation turns. The plan
// is emitted via EventPlanUpdate events and can be rendered by clients.
package plan

import "encoding/json"

// TaskStatus represents the lifecycle status of a task in the plan.
type TaskStatus string

const (
	// TaskStatusPending indicates the task has not been started yet.
	TaskStatusPending TaskStatus = "pending"
	// TaskStatusInProgress indicates the task is currently being worked on.
	TaskStatusInProgress TaskStatus = "in_progress"
	// TaskStatusCompleted indicates the task has been finished.
	TaskStatusCompleted TaskStatus = "completed"
)

// Task represents a single task/todo item in the plan.
// Each task has a description (content), a status, and an active form
// for display during execution.
type Task struct {
	// Content is the imperative form describing what needs to be done.
	// Example: "Run tests", "Build the project"
	Content string `json:"content"`

	// Status is the current state of the task (pending, in_progress, completed).
	Status TaskStatus `json:"status"`

	// ActiveForm is the present continuous form shown during execution.
	// Example: "Running tests", "Building the project"
	ActiveForm string `json:"active_form"`
}

// Validate checks if a task has valid fields.
func (t *Task) Validate() error {
	if t.Content == "" {
		return &PlanError{
			Code:    ErrInvalidTask,
			Message: "task content cannot be empty",
		}
	}

	if t.ActiveForm == "" {
		return &PlanError{
			Code:    ErrInvalidTask,
			Message: "task active_form cannot be empty",
		}
	}

	switch t.Status {
	case TaskStatusPending, TaskStatusInProgress, TaskStatusCompleted:
		// Valid status
	default:
		return &PlanError{
			Code:    ErrInvalidStatus,
			Message: "invalid task status: " + string(t.Status),
		}
	}

	return nil
}

// IsInProgress returns true if the task is currently being worked on.
func (t *Task) IsInProgress() bool {
	return t.Status == TaskStatusInProgress
}

// IsCompleted returns true if the task has been finished.
func (t *Task) IsCompleted() bool {
	return t.Status == TaskStatusCompleted
}

// IsPending returns true if the task has not been started yet.
func (t *Task) IsPending() bool {
	return t.Status == TaskStatusPending
}

// Clone creates a deep copy of the task.
func (t *Task) Clone() *Task {
	return &Task{
		Content:    t.Content,
		Status:     t.Status,
		ActiveForm: t.ActiveForm,
	}
}

// MarshalJSON implements custom JSON marshaling for Task.
func (t *Task) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"content":     t.Content,
		"status":      t.Status,
		"active_form": t.ActiveForm,
	})
}

// UnmarshalJSON implements custom JSON unmarshaling for Task.
func (t *Task) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if content, ok := raw["content"].(string); ok {
		t.Content = content
	}

	if status, ok := raw["status"].(string); ok {
		t.Status = TaskStatus(status)
	}

	if activeForm, ok := raw["active_form"].(string); ok {
		t.ActiveForm = activeForm
	}

	return nil
}
