package plan

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// TestTaskValidation tests task validation logic.
func TestTaskValidation(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid task",
			task: &Task{
				Content:    "Run tests",
				Status:     TaskStatusPending,
				ActiveForm: "Running tests",
			},
			wantErr: false,
		},
		{
			name: "empty content",
			task: &Task{
				Content:    "",
				Status:     TaskStatusPending,
				ActiveForm: "Running tests",
			},
			wantErr: true,
			errMsg:  "content cannot be empty",
		},
		{
			name: "empty active form",
			task: &Task{
				Content:    "Run tests",
				Status:     TaskStatusPending,
				ActiveForm: "",
			},
			wantErr: true,
			errMsg:  "active_form cannot be empty",
		},
		{
			name: "invalid status",
			task: &Task{
				Content:    "Run tests",
				Status:     "invalid",
				ActiveForm: "Running tests",
			},
			wantErr: true,
			errMsg:  "invalid task status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.task.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				planErr, ok := err.(*PlanError)
				if !ok {
					t.Errorf("expected PlanError, got %T", err)
					return
				}
				if planErr.Message == "" {
					t.Errorf("expected error message containing %q, got empty", tt.errMsg)
				}
			}
		})
	}
}

// TestTaskStatus tests task status helper methods.
func TestTaskStatus(t *testing.T) {
	tests := []struct {
		name           string
		status         TaskStatus
		expectPending  bool
		expectProgress bool
		expectComplete bool
	}{
		{
			name:           "pending task",
			status:         TaskStatusPending,
			expectPending:  true,
			expectProgress: false,
			expectComplete: false,
		},
		{
			name:           "in_progress task",
			status:         TaskStatusInProgress,
			expectPending:  false,
			expectProgress: true,
			expectComplete: false,
		},
		{
			name:           "completed task",
			status:         TaskStatusCompleted,
			expectPending:  false,
			expectProgress: false,
			expectComplete: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				Content:    "Test task",
				Status:     tt.status,
				ActiveForm: "Testing task",
			}

			if task.IsPending() != tt.expectPending {
				t.Errorf("IsPending() = %v, want %v", task.IsPending(), tt.expectPending)
			}
			if task.IsInProgress() != tt.expectProgress {
				t.Errorf("IsInProgress() = %v, want %v", task.IsInProgress(), tt.expectProgress)
			}
			if task.IsCompleted() != tt.expectComplete {
				t.Errorf("IsCompleted() = %v, want %v", task.IsCompleted(), tt.expectComplete)
			}
		})
	}
}

// TestPlanUpdate tests plan update operations.
func TestPlanUpdate(t *testing.T) {
	p := NewPlan()

	// Test initial state
	if !p.IsEmpty() {
		t.Error("new plan should be empty")
	}

	// Create update args
	explanation := "Initial plan"
	args := &UpdatePlanArgs{
		Explanation: &explanation,
		Tasks: []*Task{
			{
				Content:    "Task 1",
				Status:     TaskStatusPending,
				ActiveForm: "Doing task 1",
			},
			{
				Content:    "Task 2",
				Status:     TaskStatusInProgress,
				ActiveForm: "Doing task 2",
			},
			{
				Content:    "Task 3",
				Status:     TaskStatusCompleted,
				ActiveForm: "Doing task 3",
			},
		},
	}

	// Apply update
	if err := p.Update(args); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify plan state
	if p.IsEmpty() {
		t.Error("plan should not be empty after update")
	}

	if p.Explanation != explanation {
		t.Errorf("Explanation = %q, want %q", p.Explanation, explanation)
	}

	// Check task counts
	pending, inProgress, completed := p.CountTasks()
	if pending != 1 {
		t.Errorf("pending count = %d, want 1", pending)
	}
	if inProgress != 1 {
		t.Errorf("inProgress count = %d, want 1", inProgress)
	}
	if completed != 1 {
		t.Errorf("completed count = %d, want 1", completed)
	}

	// Get tasks by status
	pendingTasks := p.GetPendingTasks()
	if len(pendingTasks) != 1 {
		t.Errorf("GetPendingTasks() returned %d tasks, want 1", len(pendingTasks))
	}

	inProgressTask := p.GetInProgressTask()
	if inProgressTask == nil {
		t.Error("GetInProgressTask() returned nil")
	} else if inProgressTask.Content != "Task 2" {
		t.Errorf("GetInProgressTask().Content = %q, want %q", inProgressTask.Content, "Task 2")
	}

	completedTasks := p.GetCompletedTasks()
	if len(completedTasks) != 1 {
		t.Errorf("GetCompletedTasks() returned %d tasks, want 1", len(completedTasks))
	}
}

// TestPlanUpdateValidation tests plan update validation.
func TestPlanUpdateValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    *UpdatePlanArgs
		wantErr bool
		errCode ErrorCode
	}{
		{
			name: "valid plan",
			args: &UpdatePlanArgs{
				Tasks: []*Task{
					{
						Content:    "Task 1",
						Status:     TaskStatusPending,
						ActiveForm: "Doing task 1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty plan",
			args: &UpdatePlanArgs{
				Tasks: []*Task{},
			},
			wantErr: true,
			errCode: ErrEmptyPlan,
		},
		{
			name: "multiple in_progress tasks",
			args: &UpdatePlanArgs{
				Tasks: []*Task{
					{
						Content:    "Task 1",
						Status:     TaskStatusInProgress,
						ActiveForm: "Doing task 1",
					},
					{
						Content:    "Task 2",
						Status:     TaskStatusInProgress,
						ActiveForm: "Doing task 2",
					},
				},
			},
			wantErr: true,
			errCode: ErrMultipleInProgress,
		},
		{
			name: "invalid task",
			args: &UpdatePlanArgs{
				Tasks: []*Task{
					{
						Content:    "",
						Status:     TaskStatusPending,
						ActiveForm: "Doing task",
					},
				},
			},
			wantErr: true,
			errCode: ErrInvalidTask,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				planErr, ok := err.(*PlanError)
				if !ok {
					t.Errorf("expected PlanError, got %T", err)
					return
				}
				if planErr.Code != tt.errCode {
					t.Errorf("error code = %v, want %v", planErr.Code, tt.errCode)
				}
			}
		})
	}
}

// TestPlanConcurrency tests plan thread safety.
func TestPlanConcurrency(t *testing.T) {
	p := NewPlan()

	// Update plan
	explanation := "Test plan"
	args := &UpdatePlanArgs{
		Explanation: &explanation,
		Tasks: []*Task{
			{
				Content:    "Task 1",
				Status:     TaskStatusPending,
				ActiveForm: "Doing task 1",
			},
		},
	}
	if err := p.Update(args); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Read from multiple goroutines
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = p.GetTasks()
			_ = p.GetInProgressTask()
			_ = p.GetPendingTasks()
			_ = p.GetCompletedTasks()
			_, _, _ = p.CountTasks()
			_ = p.IsEmpty()
			_ = p.Snapshot()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestPlanSnapshot tests plan snapshot immutability.
func TestPlanSnapshot(t *testing.T) {
	p := NewPlan()

	explanation := "Test plan"
	args := &UpdatePlanArgs{
		Explanation: &explanation,
		Tasks: []*Task{
			{
				Content:    "Task 1",
				Status:     TaskStatusPending,
				ActiveForm: "Doing task 1",
			},
		},
	}
	if err := p.Update(args); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Take snapshot
	snapshot := p.Snapshot()

	// Modify plan
	explanation2 := "Modified plan"
	args2 := &UpdatePlanArgs{
		Explanation: &explanation2,
		Tasks: []*Task{
			{
				Content:    "Task 2",
				Status:     TaskStatusCompleted,
				ActiveForm: "Doing task 2",
			},
		},
	}
	if err := p.Update(args2); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify snapshot is unchanged
	if snapshot.Explanation != explanation {
		t.Errorf("snapshot.Explanation = %q, want %q", snapshot.Explanation, explanation)
	}
	if len(snapshot.Tasks) != 1 {
		t.Errorf("snapshot.Tasks length = %d, want 1", len(snapshot.Tasks))
	}
	if snapshot.Tasks[0].Content != "Task 1" {
		t.Errorf("snapshot.Tasks[0].Content = %q, want %q", snapshot.Tasks[0].Content, "Task 1")
	}
}

// mockEventEmitter implements EventEmitter for testing.
type mockEventEmitter struct {
	events []*protocol.Event
}

func (m *mockEventEmitter) EmitEvent(ctx context.Context, event *protocol.Event) error {
	m.events = append(m.events, event)
	return nil
}

// TestUpdatePlanTool tests the update_plan tool execution.
func TestUpdatePlanTool(t *testing.T) {
	emitter := &mockEventEmitter{}
	tool := NewUpdatePlanTool(emitter)

	if tool.Name() != "update_plan" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "update_plan")
	}

	// Test tool properties
	if tool.NeedsInitialApproval(nil, runtime.ApprovalOnRequest, runtime.SandboxWorkspaceWrite) {
		t.Error("NeedsInitialApproval() should return false")
	}
	if tool.NeedsRetryApproval(runtime.ApprovalOnRequest) {
		t.Error("NeedsRetryApproval() should return false")
	}
	if tool.SandboxPreference() != runtime.SandboxForbid {
		t.Errorf("SandboxPreference() = %v, want %v", tool.SandboxPreference(), runtime.SandboxForbid)
	}
	if tool.EscalateOnFailure() {
		t.Error("EscalateOnFailure() should return false")
	}
	if !tool.SupportsParallel() {
		t.Error("SupportsParallel() should return true")
	}
	if tool.SandboxRetryData(nil) != nil {
		t.Error("SandboxRetryData() should return nil")
	}

	// Create request
	args := UpdatePlanArgs{
		Tasks: []*Task{
			{
				Content:    "Task 1",
				Status:     TaskStatusPending,
				ActiveForm: "Doing task 1",
			},
		},
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "test-call-1",
		ToolName:         "update_plan",
		Arguments:        string(argsJSON),
		WorkingDirectory: "/test",
	}

	ctx := context.Background()
	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
	}

	// Execute tool
	resp, err := tool.Execute(ctx, req, execCtx)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if resp.Content != "Plan updated successfully" {
		t.Errorf("Content = %q, want %q", resp.Content, "Plan updated successfully")
	}

	if resp.Success == nil || !*resp.Success {
		t.Error("Success should be true")
	}

	// Check that event was emitted
	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitter.events))
	}

	event := emitter.events[0]
	if event.ID != "test-call-1" {
		t.Errorf("event.ID = %q, want %q", event.ID, "test-call-1")
	}

	planUpdate, ok := event.Msg.(*protocol.EventPlanUpdate)
	if !ok {
		t.Fatalf("expected EventPlanUpdate, got %T", event.Msg)
	}

	// Verify plan data
	planMap, ok := planUpdate.Plan.(map[string]interface{})
	if !ok {
		t.Fatalf("expected plan to be map[string]interface{}, got %T", planUpdate.Plan)
	}

	tasksData, ok := planMap["tasks"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected tasks to be []map[string]interface{}, got %T", planMap["tasks"])
	}

	if len(tasksData) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasksData))
	}

	if tasksData[0]["content"] != "Task 1" {
		t.Errorf("task content = %q, want %q", tasksData[0]["content"], "Task 1")
	}
}

// TestUpdatePlanToolInvalidArgs tests the tool with invalid arguments.
func TestUpdatePlanToolInvalidArgs(t *testing.T) {
	tool := NewBasicUpdatePlanTool()

	tests := []struct {
		name string
		args string
	}{
		{
			name: "invalid JSON",
			args: "not json",
		},
		{
			name: "empty plan",
			args: `{"tasks": []}`,
		},
		{
			name: "multiple in_progress",
			args: `{"tasks": [
				{"content": "Task 1", "status": "in_progress", "active_form": "Doing 1"},
				{"content": "Task 2", "status": "in_progress", "active_form": "Doing 2"}
			]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &runtime.ToolRequest{
				CallID:           "test-call",
				ToolName:         "update_plan",
				Arguments:        tt.args,
				WorkingDirectory: "/test",
			}

			ctx := context.Background()
			execCtx := &runtime.ExecutionContext{
				SessionID: "test-session",
				TurnID:    "test-turn",
			}

			_, err := tool.Execute(ctx, req, execCtx)
			if err == nil {
				t.Error("Execute() should return error")
			}

			toolErr, ok := err.(*runtime.ToolError)
			if !ok {
				t.Errorf("expected ToolError, got %T", err)
			} else if toolErr.Kind != runtime.ErrorInvalidArguments {
				t.Errorf("error kind = %v, want %v", toolErr.Kind, runtime.ErrorInvalidArguments)
			}
		})
	}
}

// TestFormatPlanForDisplay tests the plan display formatter.
func TestFormatPlanForDisplay(t *testing.T) {
	planData := map[string]interface{}{
		"explanation": "Test plan",
		"tasks": []interface{}{
			map[string]interface{}{
				"content":     "Task 1",
				"status":      "pending",
				"active_form": "Doing task 1",
			},
			map[string]interface{}{
				"content":     "Task 2",
				"status":      "in_progress",
				"active_form": "Doing task 2",
			},
			map[string]interface{}{
				"content":     "Task 3",
				"status":      "completed",
				"active_form": "Doing task 3",
			},
		},
	}

	output := FormatPlanForDisplay(planData)

	// Check that output contains expected elements
	if output == "" {
		t.Error("FormatPlanForDisplay() returned empty string")
	}

	// Should contain explanation
	if !contains(output, "Test plan") {
		t.Error("output should contain explanation")
	}

	// Should contain task contents
	if !contains(output, "Task 1") || !contains(output, "Task 2") || !contains(output, "Task 3") {
		t.Error("output should contain all task contents")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
