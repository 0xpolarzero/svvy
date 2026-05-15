package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// ============================================================================
// Registry Helper Tests
// ============================================================================

func TestRegistryHelper_GetOrError(t *testing.T) {
	registry := runtime.NewToolRegistry()
	tool := NewMockTool("test_tool")
	registry.Register(tool)

	helper := NewRegistryHelper(registry)

	// Test successful get
	foundTool, err := helper.GetOrError("test_tool")
	require.NoError(t, err)
	assert.Equal(t, "test_tool", foundTool.Name())

	// Test not found
	_, err = helper.GetOrError("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool not found")
}

func TestRegistryHelper_ListSorted(t *testing.T) {
	registry := runtime.NewToolRegistry()
	registry.Register(NewMockTool("zebra"))
	registry.Register(NewMockTool("apple"))
	registry.Register(NewMockTool("banana"))

	helper := NewRegistryHelper(registry)
	names := helper.ListSorted()

	require.Len(t, names, 3)
	assert.Equal(t, []string{"apple", "banana", "zebra"}, names)
}

func TestRegistryHelper_CountTools(t *testing.T) {
	registry := runtime.NewToolRegistry()
	helper := NewRegistryHelper(registry)

	assert.Equal(t, 0, helper.CountTools())

	registry.Register(NewMockTool("tool1"))
	assert.Equal(t, 1, helper.CountTools())

	registry.Register(NewMockTool("tool2"))
	assert.Equal(t, 2, helper.CountTools())
}

func TestRegistryHelper_HasTool(t *testing.T) {
	registry := runtime.NewToolRegistry()
	registry.Register(NewMockTool("exists"))

	helper := NewRegistryHelper(registry)

	assert.True(t, helper.HasTool("exists"))
	assert.False(t, helper.HasTool("nonexistent"))
}

func TestRegistryHelper_GetParallelTools(t *testing.T) {
	registry := runtime.NewToolRegistry()

	parallel := NewMockTool("parallel_tool")
	parallel.supportsParallel = true
	registry.Register(parallel)

	sequential := NewMockTool("sequential_tool")
	sequential.supportsParallel = false
	registry.Register(sequential)

	helper := NewRegistryHelper(registry)
	parallelTools := helper.GetParallelTools()

	require.Len(t, parallelTools, 1)
	assert.Equal(t, "parallel_tool", parallelTools[0].Name())
}

func TestRegistryHelper_GetSequentialTools(t *testing.T) {
	registry := runtime.NewToolRegistry()

	parallel := NewMockTool("parallel_tool")
	parallel.supportsParallel = true
	registry.Register(parallel)

	sequential := NewMockTool("sequential_tool")
	sequential.supportsParallel = false
	registry.Register(sequential)

	helper := NewRegistryHelper(registry)
	sequentialTools := helper.GetSequentialTools()

	require.Len(t, sequentialTools, 1)
	assert.Equal(t, "sequential_tool", sequentialTools[0].Name())
}

func TestRegistryHelper_GetToolsRequiringSandbox(t *testing.T) {
	registry := runtime.NewToolRegistry()

	require := NewMockTool("require_tool")
	require.sandboxPreference = runtime.SandboxRequire
	registry.Register(require)

	forbid := NewMockTool("forbid_tool")
	forbid.sandboxPreference = runtime.SandboxForbid
	registry.Register(forbid)

	helper := NewRegistryHelper(registry)
	requiringTools := helper.GetToolsRequiringSandbox()

	assert.Len(t, requiringTools, 1)
	assert.Equal(t, "require_tool", requiringTools[0].Name())
}

func TestRegistryHelper_GetToolsForbiddingSandbox(t *testing.T) {
	registry := runtime.NewToolRegistry()

	require := NewMockTool("require_tool")
	require.sandboxPreference = runtime.SandboxRequire
	registry.Register(require)

	forbid := NewMockTool("forbid_tool")
	forbid.sandboxPreference = runtime.SandboxForbid
	registry.Register(forbid)

	helper := NewRegistryHelper(registry)
	forbiddingTools := helper.GetToolsForbiddingSandbox()

	assert.Len(t, forbiddingTools, 1)
	assert.Equal(t, "forbid_tool", forbiddingTools[0].Name())
}

func TestRegistryHelper_GetToolInfo(t *testing.T) {
	registry := runtime.NewToolRegistry()
	tool := NewMockTool("test_tool")
	tool.supportsParallel = true
	tool.sandboxPreference = runtime.SandboxAuto
	tool.escalateOnFailure = true
	registry.Register(tool)

	helper := NewRegistryHelper(registry)
	info, err := helper.GetToolInfo("test_tool")

	require.NoError(t, err)
	assert.Equal(t, "test_tool", info.Name)
	assert.True(t, info.SupportsParallel)
	assert.Equal(t, runtime.SandboxAuto, info.SandboxPreference)
	assert.True(t, info.EscalateOnFailure)
}

func TestRegistryHelper_GetAllToolInfo(t *testing.T) {
	registry := runtime.NewToolRegistry()
	registry.Register(NewMockTool("tool1"))
	registry.Register(NewMockTool("tool2"))

	helper := NewRegistryHelper(registry)
	infos := helper.GetAllToolInfo()

	assert.Len(t, infos, 2)
}

func TestRegistryHelper_ValidateToolRequests(t *testing.T) {
	registry := runtime.NewToolRegistry()
	registry.Register(NewMockTool("valid_tool"))

	helper := NewRegistryHelper(registry)

	// Valid requests
	validReqs := []*runtime.ToolRequest{
		{CallID: "call1", ToolName: "valid_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call2", ToolName: "valid_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
	}
	err := helper.ValidateToolRequests(validReqs)
	assert.NoError(t, err)

	// Invalid request - missing CallID
	invalidReqs := []*runtime.ToolRequest{
		{CallID: "", ToolName: "valid_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
	}
	err = helper.ValidateToolRequests(invalidReqs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CallID")

	// Invalid request - missing ToolName
	invalidReqs = []*runtime.ToolRequest{
		{CallID: "call1", ToolName: "", Arguments: `{}`, WorkingDirectory: "/workspace"},
	}
	err = helper.ValidateToolRequests(invalidReqs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ToolName")

	// Invalid request - tool not found
	invalidReqs = []*runtime.ToolRequest{
		{CallID: "call1", ToolName: "nonexistent", Arguments: `{}`, WorkingDirectory: "/workspace"},
	}
	err = helper.ValidateToolRequests(invalidReqs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tool not found")

	// Nil request
	err = helper.ValidateToolRequest(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request is nil")
}

func TestRegistryHelper_GroupRequestsByParallelism(t *testing.T) {
	registry := runtime.NewToolRegistry()

	parallelTool := NewMockTool("parallel_tool")
	parallelTool.supportsParallel = true
	registry.Register(parallelTool)

	sequentialTool := NewMockTool("sequential_tool")
	sequentialTool.supportsParallel = false
	registry.Register(sequentialTool)

	helper := NewRegistryHelper(registry)

	requests := []*runtime.ToolRequest{
		{CallID: "call1", ToolName: "parallel_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call2", ToolName: "sequential_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call3", ToolName: "parallel_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call4", ToolName: "nonexistent", Arguments: `{}`, WorkingDirectory: "/workspace"},
	}

	parallel, sequential := helper.GroupRequestsByParallelism(requests)

	assert.Len(t, parallel, 2)
	assert.Len(t, sequential, 2) // includes unknown tool
}

func TestRegistryHelper_FilterRequestsByTool(t *testing.T) {
	registry := runtime.NewToolRegistry()
	helper := NewRegistryHelper(registry)

	requests := []*runtime.ToolRequest{
		{CallID: "call1", ToolName: "tool1", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call2", ToolName: "tool2", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call3", ToolName: "tool1", Arguments: `{}`, WorkingDirectory: "/workspace"},
	}

	filtered := helper.FilterRequestsByTool(requests, "tool1")

	assert.Len(t, filtered, 2)
	assert.Equal(t, "call1", filtered[0].CallID)
	assert.Equal(t, "call3", filtered[1].CallID)
}

func TestRegistryHelper_DeduplicateRequests(t *testing.T) {
	registry := runtime.NewToolRegistry()
	helper := NewRegistryHelper(registry)

	requests := []*runtime.ToolRequest{
		{CallID: "call1", ToolName: "tool1", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call2", ToolName: "tool2", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call1", ToolName: "tool1", Arguments: `{}`, WorkingDirectory: "/workspace"}, // duplicate
	}

	deduplicated := helper.DeduplicateRequests(requests)

	assert.Len(t, deduplicated, 2)
	assert.Equal(t, "call1", deduplicated[0].CallID)
	assert.Equal(t, "call2", deduplicated[1].CallID)
}

func TestRegistryHelper_GetSnapshot(t *testing.T) {
	registry := runtime.NewToolRegistry()

	tool1 := NewMockTool("tool1")
	tool1.supportsParallel = true
	tool1.sandboxPreference = runtime.SandboxAuto
	registry.Register(tool1)

	tool2 := NewMockTool("tool2")
	tool2.supportsParallel = false
	tool2.sandboxPreference = runtime.SandboxForbid
	registry.Register(tool2)

	helper := NewRegistryHelper(registry)
	snapshot := helper.GetSnapshot()

	assert.Equal(t, 2, snapshot.ToolCount)
	assert.Equal(t, 1, snapshot.Parallel)
	assert.Equal(t, 1, snapshot.Sequential)
	assert.Equal(t, 1, snapshot.Sandboxed)
	assert.Equal(t, 1, snapshot.Unsandboxed)
}

// ============================================================================
// Approval Manager Tests
// ============================================================================

func TestApprovalManager_CheckCachedApproval(t *testing.T) {
	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApproved)
	manager := NewApprovalManager(cache, handler.RequestApproval)

	tool := NewMockTool("test_tool")
	req := &runtime.ToolRequest{
		CallID:    "call1",
		ToolName:  "test_tool",
		Arguments: `{}`,
	}

	// No cached decision initially
	decision := manager.CheckCachedApproval(tool, req)
	assert.Nil(t, decision)

	// Cache a decision
	manager.CacheApproval(tool, req, runtime.ApprovalApprovedForSession)

	// Should find cached decision
	decision = manager.CheckCachedApproval(tool, req)
	require.NotNil(t, decision)
	assert.Equal(t, runtime.ApprovalApprovedForSession, *decision)
}

func TestFormatApprovalRequest(t *testing.T) {
	req := &runtime.ApprovalRequest{
		CallID:           "call1",
		ToolName:         "shell",
		Command:          []string{"ls", "-la"},
		WorkingDirectory: "/workspace",
		Justification:    "listing files",
		IsRetry:          false,
	}

	output := FormatApprovalRequest(req)

	assert.Contains(t, output, "shell")
	assert.Contains(t, output, "ls")
	assert.Contains(t, output, "/workspace")
	assert.Contains(t, output, "listing files")
}

func TestFormatApprovalRequest_WithRisk(t *testing.T) {
	req := &runtime.ApprovalRequest{
		CallID:           "call1",
		ToolName:         "shell",
		Command:          []string{"rm", "-rf", "/"},
		WorkingDirectory: "/",
		IsRetry:          true,
		RetryReason:      "permission denied",
		Risk: &runtime.RiskAssessment{
			Level:      runtime.RiskHigh,
			Reasons:    []string{"destructive command", "system directory"},
			Mitigation: "Sandbox would prevent this",
		},
	}

	output := FormatApprovalRequest(req)

	assert.Contains(t, output, "Retry approval")
	assert.Contains(t, output, "permission denied")
	assert.Contains(t, output, "Risk Assessment")
	assert.Contains(t, output, "High")
	assert.Contains(t, output, "destructive command")
	assert.Contains(t, output, "Sandbox would prevent this")
}

func TestParseApprovalDecisionString(t *testing.T) {
	tests := []struct {
		input    string
		expected runtime.ApprovalDecision
		hasError bool
	}{
		{"approve", runtime.ApprovalApproved, false},
		{"approved", runtime.ApprovalApproved, false},
		{"yes", runtime.ApprovalApproved, false},
		{"deny", runtime.ApprovalDenied, false},
		{"denied", runtime.ApprovalDenied, false},
		{"no", runtime.ApprovalDenied, false},
		{"approve_session", runtime.ApprovalApprovedForSession, false},
		{"always", runtime.ApprovalApprovedForSession, false},
		{"all", runtime.ApprovalApprovedForSession, false},
		{"abort", runtime.ApprovalAbort, false},
		{"cancel", runtime.ApprovalAbort, false},
		{"invalid", runtime.ApprovalDenied, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			decision, err := ParseApprovalDecisionString(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, decision)
			}
		})
	}
}

func TestApprovalRequestToJSON(t *testing.T) {
	req := &runtime.ApprovalRequest{
		CallID:           "call1",
		ToolName:         "shell",
		Command:          []string{"ls"},
		WorkingDirectory: "/workspace",
	}

	jsonStr, err := ApprovalRequestToJSON(req)
	require.NoError(t, err)
	assert.Contains(t, jsonStr, "call1")
	assert.Contains(t, jsonStr, "shell")
}

// ============================================================================
// Execution Engine Tests
// ============================================================================

func TestExecutionEngine_ExecuteSequential(t *testing.T) {
	registry := runtime.NewToolRegistry()
	tool := NewMockTool("test_tool")
	registry.Register(tool)

	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApproved)
	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	engine := NewExecutionEngine()

	requests := []*runtime.ToolRequest{
		{CallID: "call1", ToolName: "test_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call2", ToolName: "test_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call3", ToolName: "test_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
	}

	ctx := context.Background()
	execCtx := &runtime.ExecutionContext{
		SessionID:      "session1",
		TurnID:         "turn1",
		ApprovalCache:  cache,
		SandboxAttempt: createMockSandboxAttempt(runtime.SandboxNone),
	}

	results, err := engine.ExecuteSequential(ctx, orch, requests, execCtx)

	require.NoError(t, err)
	require.Len(t, results, 3)

	for _, result := range results {
		assert.NotNil(t, result.Response)
		assert.Nil(t, result.Error)
	}
}

func TestExecutionEngine_ExecuteBatched(t *testing.T) {
	registry := runtime.NewToolRegistry()
	tool := NewMockTool("test_tool")
	registry.Register(tool)

	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApproved)
	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	engine := NewExecutionEngine()

	requests := []*runtime.ToolRequest{
		{CallID: "call1", ToolName: "test_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call2", ToolName: "test_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call3", ToolName: "test_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call4", ToolName: "test_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call5", ToolName: "test_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
	}

	ctx := context.Background()
	execCtx := &runtime.ExecutionContext{
		SessionID:      "session1",
		TurnID:         "turn1",
		ApprovalCache:  cache,
		SandboxAttempt: createMockSandboxAttempt(runtime.SandboxNone),
	}

	start := time.Now()
	results, err := engine.ExecuteBatched(ctx, orch, requests, execCtx, 2, 10*time.Millisecond)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Len(t, results, 5)

	// Should have taken at least 20ms due to batch delay (2 delays for 3 batches: 0-1, 2-3, 4)
	assert.GreaterOrEqual(t, elapsed, 20*time.Millisecond)
}

func TestExecutionEngine_ComputeStats(t *testing.T) {
	engine := NewExecutionEngine()

	results := []*runtime.ExecutionResult{
		{
			Request:   &runtime.ToolRequest{CallID: "call1", ToolName: "tool1"},
			Response:  &runtime.ToolResponse{Content: "success"},
			StartTime: time.Now(),
			EndTime:   time.Now().Add(100 * time.Millisecond),
		},
		{
			Request:   &runtime.ToolRequest{CallID: "call2", ToolName: "tool2"},
			Error:     &runtime.ToolError{Kind: runtime.ErrorExecution, Message: "failed"},
			StartTime: time.Now(),
			EndTime:   time.Now().Add(50 * time.Millisecond),
		},
	}

	stats := engine.ComputeStats(results)

	assert.Equal(t, 2, stats.TotalRequests)
	assert.Equal(t, 1, stats.SuccessCount)
	assert.Equal(t, 1, stats.ErrorCount)
	assert.Greater(t, stats.TotalDuration, time.Duration(0))
}

func TestExecutionEngine_PlanExecution(t *testing.T) {
	registry := runtime.NewToolRegistry()

	parallelTool := NewMockTool("parallel_tool")
	parallelTool.supportsParallel = true
	registry.Register(parallelTool)

	sequentialTool := NewMockTool("sequential_tool")
	sequentialTool.supportsParallel = false
	registry.Register(sequentialTool)

	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApproved)
	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	engine := NewExecutionEngine()

	requests := []*runtime.ToolRequest{
		{CallID: "call1", ToolName: "parallel_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call2", ToolName: "parallel_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call3", ToolName: "sequential_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
	}

	plan := engine.PlanExecution(orch, requests)

	assert.Equal(t, 3, plan.TotalRequests)
	assert.Len(t, plan.ParallelBatch, 2)
	assert.Len(t, plan.SequentialBatch, 1)
	assert.Greater(t, plan.EstimatedTime, time.Duration(0))
}

func TestExecutionEngine_CancelableExecution(t *testing.T) {
	registry := runtime.NewToolRegistry()
	tool := NewMockTool("slow_tool")
	tool.executeFunc = func(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
		select {
		case <-time.After(5 * time.Second):
			return &runtime.ToolResponse{Content: "completed"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	registry.Register(tool)

	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApproved)
	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	engine := NewExecutionEngine()

	requests := []*runtime.ToolRequest{
		{CallID: "call1", ToolName: "slow_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
	}

	ctx := context.Background()
	execCtx := &runtime.ExecutionContext{
		SessionID:      "session1",
		TurnID:         "turn1",
		ApprovalCache:  cache,
		SandboxAttempt: createMockSandboxAttempt(runtime.SandboxNone),
	}

	exec := engine.StartCancelableExecution(ctx, orch, requests, execCtx)

	// Should not be done immediately
	assert.False(t, exec.IsDone())

	// Cancel execution
	exec.Cancel()

	// Wait for cancellation
	results, _ := exec.Wait()

	// Should be cancelled
	assert.True(t, exec.IsDone())
	assert.NotNil(t, results)

	// Verify execution was cancelled
	if len(results) > 0 && results[0] != nil {
		assert.NotNil(t, results[0].Error)
	}
}

func TestExecutionEngine_WithCustomLimit(t *testing.T) {
	engine := NewExecutionEngineWithLimit(5)
	assert.Equal(t, 5, engine.maxParallel)

	// Test with invalid limit
	engine = NewExecutionEngineWithLimit(0)
	assert.Equal(t, 1, engine.maxParallel)

	engine = NewExecutionEngineWithLimit(-1)
	assert.Equal(t, 1, engine.maxParallel)
}

// ============================================================================
// Sandbox Selector Additional Tests
// ============================================================================

func TestSandboxSelector_EscalateSandbox(t *testing.T) {
	selector := NewSandboxSelector()

	// Test bubblewrap to docker escalation
	current := &runtime.SandboxAttempt{
		Type:             runtime.SandboxBubblewrap,
		Policy:           runtime.SandboxReadOnly,
		WorkingDirectory: "/workspace",
	}

	// If Docker is available, should escalate to docker
	// Otherwise, should escalate to none
	escalated := selector.EscalateSandbox(current)
	assert.NotEqual(t, runtime.SandboxBubblewrap, escalated.Type)

	// Test docker escalation (should try kubernetes if available, otherwise none)
	dockerAttempt := &runtime.SandboxAttempt{
		Type:             runtime.SandboxDocker,
		Policy:           runtime.SandboxReadOnly,
		WorkingDirectory: "/workspace",
	}

	escalated = selector.EscalateSandbox(dockerAttempt)
	// Should escalate to kubernetes or none depending on availability
	assert.True(t, escalated.Type == runtime.SandboxKubernetes || escalated.Type == runtime.SandboxNone)

	// Test kubernetes to none escalation
	k8sAttempt := &runtime.SandboxAttempt{
		Type:             runtime.SandboxKubernetes,
		Policy:           runtime.SandboxReadOnly,
		WorkingDirectory: "/workspace",
	}

	escalated = selector.EscalateSandbox(k8sAttempt)
	assert.Equal(t, runtime.SandboxNone, escalated.Type)

	// Test none (should stay none)
	noneAttempt := &runtime.SandboxAttempt{
		Type:             runtime.SandboxNone,
		Policy:           runtime.SandboxReadOnly,
		WorkingDirectory: "/workspace",
	}

	escalated = selector.EscalateSandbox(noneAttempt)
	assert.Equal(t, runtime.SandboxNone, escalated.Type)
}

func TestSandboxSelector_ShouldRetryWithoutSandbox(t *testing.T) {
	selector := NewSandboxSelector()

	tool := NewMockTool("test_tool")
	tool.escalateOnFailure = true
	tool.sandboxRetryData = &runtime.SandboxRetryData{
		Command:          []string{"ls"},
		WorkingDirectory: "/workspace",
	}

	attempt := &runtime.SandboxAttempt{
		Type:   runtime.SandboxDocker,
		Policy: runtime.SandboxReadOnly,
	}

	// Should retry with sandbox denial error
	sandboxErr := &runtime.ToolError{
		Kind:    runtime.ErrorSandboxDenied,
		Message: "permission denied",
	}
	assert.True(t, selector.ShouldRetryWithoutSandbox(tool, attempt, sandboxErr))

	// Should not retry with other errors
	otherErr := &runtime.ToolError{
		Kind:    runtime.ErrorExecution,
		Message: "command failed",
	}
	assert.False(t, selector.ShouldRetryWithoutSandbox(tool, attempt, otherErr))

	// Should not retry if tool doesn't support escalation
	tool.escalateOnFailure = false
	assert.False(t, selector.ShouldRetryWithoutSandbox(tool, attempt, sandboxErr))

	// Should not retry if no sandbox retry data
	tool.escalateOnFailure = true
	tool.sandboxRetryData = nil
	assert.False(t, selector.ShouldRetryWithoutSandbox(tool, attempt, sandboxErr))

	// Should not retry if already running without sandbox
	tool.sandboxRetryData = &runtime.SandboxRetryData{Command: []string{"ls"}}
	noneAttempt := &runtime.SandboxAttempt{Type: runtime.SandboxNone}
	assert.False(t, selector.ShouldRetryWithoutSandbox(tool, noneAttempt, sandboxErr))
}
