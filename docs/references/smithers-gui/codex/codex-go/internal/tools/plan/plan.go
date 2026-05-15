package plan

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Plan represents a task list/todo list with an optional explanation.
// Plans can be updated incrementally as tasks are added, completed, or removed.
type Plan struct {
	mu sync.RWMutex

	// Explanation is an optional description of the overall plan.
	Explanation string `json:"explanation,omitempty"`

	// Tasks is the list of tasks in the plan.
	Tasks []*Task `json:"tasks"`
}

// NewPlan creates a new empty plan.
func NewPlan() *Plan {
	return &Plan{
		Tasks: make([]*Task, 0),
	}
}

// UpdatePlanArgs represents the arguments for the update_plan tool.
// This matches the structure expected by the AI model.
type UpdatePlanArgs struct {
	// Explanation is an optional description of why the plan is being updated.
	Explanation *string `json:"explanation,omitempty"`

	// Tasks is the new list of tasks (replaces existing tasks).
	Tasks []*Task `json:"tasks"`
}

// Validate checks if the plan update arguments are valid.
func (args *UpdatePlanArgs) Validate() error {
	if len(args.Tasks) == 0 {
		return &PlanError{
			Code:    ErrEmptyPlan,
			Message: "plan must contain at least one task",
		}
	}

	// Validate each task
	for i, task := range args.Tasks {
		if err := task.Validate(); err != nil {
			return &PlanError{
				Code:    ErrInvalidTask,
				Message: fmt.Sprintf("task %d is invalid: %v", i, err),
				Cause:   err,
			}
		}
	}

	// Check that at most one task is in_progress
	inProgressCount := 0
	for _, task := range args.Tasks {
		if task.IsInProgress() {
			inProgressCount++
		}
	}

	if inProgressCount > 1 {
		return &PlanError{
			Code:    ErrMultipleInProgress,
			Message: fmt.Sprintf("plan has %d tasks in_progress, but at most one is allowed", inProgressCount),
		}
	}

	return nil
}

// Update applies the plan update arguments to this plan.
// This replaces the existing tasks with the new tasks.
func (p *Plan) Update(args *UpdatePlanArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Update explanation if provided
	if args.Explanation != nil {
		p.Explanation = *args.Explanation
	}

	// Clone tasks to avoid external modification
	p.Tasks = make([]*Task, len(args.Tasks))
	for i, task := range args.Tasks {
		p.Tasks[i] = task.Clone()
	}

	return nil
}

// GetTasks returns a copy of all tasks in the plan.
// This method is thread-safe.
func (p *Plan) GetTasks() []*Task {
	p.mu.RLock()
	defer p.mu.RUnlock()

	tasks := make([]*Task, len(p.Tasks))
	for i, task := range p.Tasks {
		tasks[i] = task.Clone()
	}
	return tasks
}

// GetInProgressTask returns the task currently in progress, if any.
// This method is thread-safe.
func (p *Plan) GetInProgressTask() *Task {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, task := range p.Tasks {
		if task.IsInProgress() {
			return task.Clone()
		}
	}
	return nil
}

// GetPendingTasks returns all tasks with pending status.
// This method is thread-safe.
func (p *Plan) GetPendingTasks() []*Task {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pending := make([]*Task, 0)
	for _, task := range p.Tasks {
		if task.IsPending() {
			pending = append(pending, task.Clone())
		}
	}
	return pending
}

// GetCompletedTasks returns all tasks with completed status.
// This method is thread-safe.
func (p *Plan) GetCompletedTasks() []*Task {
	p.mu.RLock()
	defer p.mu.RUnlock()

	completed := make([]*Task, 0)
	for _, task := range p.Tasks {
		if task.IsCompleted() {
			completed = append(completed, task.Clone())
		}
	}
	return completed
}

// CountTasks returns the number of tasks in each status.
// This method is thread-safe.
func (p *Plan) CountTasks() (pending, inProgress, completed int) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, task := range p.Tasks {
		switch task.Status {
		case TaskStatusPending:
			pending++
		case TaskStatusInProgress:
			inProgress++
		case TaskStatusCompleted:
			completed++
		}
	}
	return
}

// IsEmpty returns true if the plan has no tasks.
// This method is thread-safe.
func (p *Plan) IsEmpty() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.Tasks) == 0
}

// Clear removes all tasks from the plan.
// This method is thread-safe.
func (p *Plan) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Explanation = ""
	p.Tasks = make([]*Task, 0)
}

// Snapshot creates an immutable snapshot of the current plan.
// This method is thread-safe.
func (p *Plan) Snapshot() *PlanSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	tasks := make([]*Task, len(p.Tasks))
	for i, task := range p.Tasks {
		tasks[i] = task.Clone()
	}

	return &PlanSnapshot{
		Explanation: p.Explanation,
		Tasks:       tasks,
	}
}

// PlanSnapshot represents an immutable snapshot of a plan.
type PlanSnapshot struct {
	Explanation string  `json:"explanation,omitempty"`
	Tasks       []*Task `json:"tasks"`
}

// MarshalJSON implements custom JSON marshaling for Plan.
func (p *Plan) MarshalJSON() ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	m := map[string]interface{}{
		"tasks": p.Tasks,
	}
	if p.Explanation != "" {
		m["explanation"] = p.Explanation
	}
	return json.Marshal(m)
}

// ToJSON converts the plan to JSON for protocol events.
// This method is thread-safe.
func (p *Plan) ToJSON() (interface{}, error) {
	snapshot := p.Snapshot()

	// Convert to map for protocol
	m := map[string]interface{}{
		"tasks": snapshot.Tasks,
	}
	if snapshot.Explanation != "" {
		m["explanation"] = snapshot.Explanation
	}

	return m, nil
}

// PlanError represents an error that occurred during plan operations.
type PlanError struct {
	Code    ErrorCode
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *PlanError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap returns the underlying cause for error wrapping.
func (e *PlanError) Unwrap() error {
	return e.Cause
}

// ErrorCode categorizes different types of plan errors.
type ErrorCode int

const (
	// ErrInvalidTask indicates a task has invalid fields.
	ErrInvalidTask ErrorCode = iota

	// ErrInvalidStatus indicates an invalid task status.
	ErrInvalidStatus

	// ErrEmptyPlan indicates the plan has no tasks.
	ErrEmptyPlan

	// ErrMultipleInProgress indicates multiple tasks are in_progress.
	ErrMultipleInProgress

	// ErrInvalidArguments indicates the update arguments are malformed.
	ErrInvalidArguments
)
