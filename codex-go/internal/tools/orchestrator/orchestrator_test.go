package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// ============================================================================
// Mock Tools for Testing
// ============================================================================

// MockTool is a configurable mock tool for testing
type MockTool struct {
	name                   string
	executeFunc            func(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error)
	approvalKeyFunc        func(req *runtime.ToolRequest) string
	needsInitialApproval   bool
	needsRetryApprovalFlag bool
	sandboxPreference      runtime.SandboxPreference
	escalateOnFailure      bool
	wantsEscalated         bool
	supportsParallel       bool
	sandboxRetryData       *runtime.SandboxRetryData
	executionCount         int
	mu                     sync.Mutex
}

func NewMockTool(name string) *MockTool {
	return &MockTool{
		name:              name,
		supportsParallel:  true,
		sandboxPreference: runtime.SandboxAuto,
		escalateOnFailure: true,
		approvalKeyFunc:   func(req *runtime.ToolRequest) string { return name + ":" + req.Arguments },
		executeFunc: func(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
			return &runtime.ToolResponse{
				Content: "mock output",
				Success: boolPtr(true),
			}, nil
		},
	}
}

func (m *MockTool) Name() string { return m.name }

func (m *MockTool) Execute(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
	m.mu.Lock()
	m.executionCount++
	m.mu.Unlock()
	return m.executeFunc(ctx, req, execCtx)
}

func (m *MockTool) ApprovalKey(req *runtime.ToolRequest) string {
	return m.approvalKeyFunc(req)
}

func (m *MockTool) NeedsInitialApproval(req *runtime.ToolRequest, approvalPolicy runtime.ApprovalPolicy, sandboxPolicy runtime.SandboxPolicy) bool {
	return m.needsInitialApproval
}

func (m *MockTool) NeedsRetryApproval(approvalPolicy runtime.ApprovalPolicy) bool {
	return m.needsRetryApprovalFlag
}

func (m *MockTool) SandboxPreference() runtime.SandboxPreference {
	return m.sandboxPreference
}

func (m *MockTool) EscalateOnFailure() bool {
	return m.escalateOnFailure
}

func (m *MockTool) WantsEscalatedFirstAttempt(req *runtime.ToolRequest) bool {
	return m.wantsEscalated
}

func (m *MockTool) SupportsParallel() bool {
	return m.supportsParallel
}

func (m *MockTool) SandboxRetryData(req *runtime.ToolRequest) *runtime.SandboxRetryData {
	return m.sandboxRetryData
}

func (m *MockTool) GetExecutionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.executionCount
}

// MockApprovalHandler simulates user approval decisions
type MockApprovalHandler struct {
	decision      runtime.ApprovalDecision
	approvalCalls int
	mu            sync.Mutex
}

func NewMockApprovalHandler(decision runtime.ApprovalDecision) *MockApprovalHandler {
	return &MockApprovalHandler{
		decision: decision,
	}
}

func (m *MockApprovalHandler) RequestApproval(ctx context.Context, req *runtime.ApprovalRequest) (runtime.ApprovalDecision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.approvalCalls++
	return m.decision, nil
}

func (m *MockApprovalHandler) GetApprovalCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.approvalCalls
}

// ============================================================================
// Orchestrator Tests
// ============================================================================

func TestNewOrchestrator(t *testing.T) {
	registry := runtime.NewToolRegistry()
	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApproved)

	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	require.NotNil(t, orch)
	assert.Equal(t, registry, orch.registry)
	assert.Equal(t, cache, orch.approvalCache)
}

func TestOrchestratorExecuteSingleTool(t *testing.T) {
	registry := runtime.NewToolRegistry()
	mockTool := NewMockTool("test_tool")
	registry.Register(mockTool)

	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApproved)
	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	req := &runtime.ToolRequest{
		CallID:           "call1",
		ToolName:         "test_tool",
		Arguments:        `{"arg": "value"}`,
		WorkingDirectory: "/workspace",
	}

	ctx := context.Background()
	execCtx := &runtime.ExecutionContext{
		SessionID:      "session1",
		TurnID:         "turn1",
		ApprovalCache:  cache,
		SandboxAttempt: createMockSandboxAttempt(runtime.SandboxNone),
	}

	result, err := orch.Execute(ctx, req, execCtx)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "mock output", result.Response.Content)
	assert.Equal(t, 1, mockTool.GetExecutionCount())
}

func TestOrchestratorToolNotFound(t *testing.T) {
	registry := runtime.NewToolRegistry()
	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApproved)
	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	req := &runtime.ToolRequest{
		CallID:           "call1",
		ToolName:         "nonexistent_tool",
		Arguments:        `{}`,
		WorkingDirectory: "/workspace",
	}

	ctx := context.Background()
	execCtx := &runtime.ExecutionContext{
		SessionID:     "session1",
		TurnID:        "turn1",
		ApprovalCache: cache,
	}

	result, err := orch.Execute(ctx, req, execCtx)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "tool not found")
}

func TestOrchestratorApprovalRequired(t *testing.T) {
	registry := runtime.NewToolRegistry()
	mockTool := NewMockTool("approval_tool")
	mockTool.needsInitialApproval = true
	registry.Register(mockTool)

	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApproved)
	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	req := &runtime.ToolRequest{
		CallID:           "call1",
		ToolName:         "approval_tool",
		Arguments:        `{"command": "rm -rf /"}`,
		WorkingDirectory: "/workspace",
	}

	ctx := context.Background()
	execCtx := &runtime.ExecutionContext{
		SessionID:     "session1",
		TurnID:        "turn1",
		ApprovalCache: cache,
		SandboxAttempt: &runtime.SandboxAttempt{
			Type:   runtime.SandboxNone,
			Policy: runtime.SandboxReadOnly,
		},
	}

	result, err := orch.Execute(ctx, req, execCtx)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.ApprovalRequired)
	assert.Equal(t, 1, handler.GetApprovalCount())
}

func TestOrchestratorApprovalDenied(t *testing.T) {
	registry := runtime.NewToolRegistry()
	mockTool := NewMockTool("dangerous_tool")
	mockTool.needsInitialApproval = true
	registry.Register(mockTool)

	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalDenied)
	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	req := &runtime.ToolRequest{
		CallID:           "call1",
		ToolName:         "dangerous_tool",
		Arguments:        `{}`,
		WorkingDirectory: "/workspace",
	}

	ctx := context.Background()
	execCtx := &runtime.ExecutionContext{
		SessionID:     "session1",
		TurnID:        "turn1",
		ApprovalCache: cache,
		SandboxAttempt: &runtime.SandboxAttempt{
			Type:   runtime.SandboxNone,
			Policy: runtime.SandboxReadOnly,
		},
	}

	result, err := orch.Execute(ctx, req, execCtx)

	require.Error(t, err)
	assert.Nil(t, result)

	var toolErr *runtime.ToolError
	require.True(t, errors.As(err, &toolErr))
	assert.Equal(t, runtime.ErrorRejected, toolErr.Kind)
}

func TestOrchestratorApprovalCaching(t *testing.T) {
	registry := runtime.NewToolRegistry()
	mockTool := NewMockTool("cacheable_tool")
	mockTool.needsInitialApproval = true
	registry.Register(mockTool)

	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApprovedForSession)
	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	req1 := &runtime.ToolRequest{
		CallID:           "call1",
		ToolName:         "cacheable_tool",
		Arguments:        `{"command": "ls"}`,
		WorkingDirectory: "/workspace",
	}

	ctx := context.Background()
	execCtx := &runtime.ExecutionContext{
		SessionID:     "session1",
		TurnID:        "turn1",
		ApprovalCache: cache,
		SandboxAttempt: &runtime.SandboxAttempt{
			Type:   runtime.SandboxNone,
			Policy: runtime.SandboxReadOnly,
		},
	}

	// First execution should request approval
	result1, err := orch.Execute(ctx, req1, execCtx)
	require.NoError(t, err)
	require.NotNil(t, result1)
	assert.Equal(t, 1, handler.GetApprovalCount())

	// Second execution with same approval key should use cache
	req2 := &runtime.ToolRequest{
		CallID:           "call2",
		ToolName:         "cacheable_tool",
		Arguments:        `{"command": "ls"}`,
		WorkingDirectory: "/workspace",
	}

	execCtx.TurnID = "turn2"
	result2, err := orch.Execute(ctx, req2, execCtx)
	require.NoError(t, err)
	require.NotNil(t, result2)

	// Should still be 1 because second request used cache
	assert.Equal(t, 1, handler.GetApprovalCount())
}

func TestOrchestratorSandboxRetry(t *testing.T) {
	registry := runtime.NewToolRegistry()
	mockTool := NewMockTool("sandbox_tool")

	// First call fails with sandbox denial, second succeeds
	callCount := 0
	mockTool.executeFunc = func(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
		callCount++
		if callCount == 1 && execCtx.SandboxAttempt.Type != runtime.SandboxNone {
			return nil, runtime.NewToolError(runtime.ErrorSandboxDenied, "permission denied")
		}
		return &runtime.ToolResponse{
			Content: "success after retry",
			Success: boolPtr(true),
		}, nil
	}

	mockTool.sandboxRetryData = &runtime.SandboxRetryData{
		Command:          []string{"ls", "-la"},
		WorkingDirectory: "/workspace",
	}
	registry.Register(mockTool)

	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApproved)
	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	req := &runtime.ToolRequest{
		CallID:           "call1",
		ToolName:         "sandbox_tool",
		Arguments:        `{}`,
		WorkingDirectory: "/workspace",
	}

	ctx := context.Background()
	execCtx := &runtime.ExecutionContext{
		SessionID:     "session1",
		TurnID:        "turn1",
		ApprovalCache: cache,
		SandboxAttempt: &runtime.SandboxAttempt{
			Type:   runtime.SandboxDocker,
			Policy: runtime.SandboxReadOnly,
		},
	}

	result, err := orch.Execute(ctx, req, execCtx)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "success after retry", result.Response.Content)
	assert.Equal(t, 1, result.RetryCount)
}

func TestOrchestratorContextCancellation(t *testing.T) {
	registry := runtime.NewToolRegistry()
	mockTool := NewMockTool("slow_tool")
	mockTool.executeFunc = func(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
		select {
		case <-time.After(5 * time.Second):
			return &runtime.ToolResponse{Content: "completed"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	registry.Register(mockTool)

	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApproved)
	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	req := &runtime.ToolRequest{
		CallID:           "call1",
		ToolName:         "slow_tool",
		Arguments:        `{}`,
		WorkingDirectory: "/workspace",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	execCtx := &runtime.ExecutionContext{
		SessionID:      "session1",
		TurnID:         "turn1",
		ApprovalCache:  cache,
		SandboxAttempt: createMockSandboxAttempt(runtime.SandboxNone),
	}

	result, err := orch.Execute(ctx, req, execCtx)

	require.Error(t, err)
	// Result should be returned even on error to capture accurate timestamps
	require.NotNil(t, result)
	assert.NotNil(t, result.Error)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
	// Verify timestamps were captured
	assert.False(t, result.StartTime.IsZero())
	assert.False(t, result.EndTime.IsZero())
}

func TestOrchestratorStreamingOutput(t *testing.T) {
	registry := runtime.NewToolRegistry()
	mockTool := NewMockTool("streaming_tool")
	mockTool.executeFunc = func(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
		if execCtx.OutputWriter != nil {
			execCtx.OutputWriter.Write([]byte("chunk1\n"))
			execCtx.OutputWriter.Write([]byte("chunk2\n"))
			execCtx.OutputWriter.Write([]byte("chunk3\n"))
		}
		return &runtime.ToolResponse{
			Content:        "chunk1\nchunk2\nchunk3\n",
			StreamedOutput: true,
		}, nil
	}
	registry.Register(mockTool)

	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApproved)
	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	req := &runtime.ToolRequest{
		CallID:           "call1",
		ToolName:         "streaming_tool",
		Arguments:        `{}`,
		WorkingDirectory: "/workspace",
	}

	ctx := context.Background()

	var output []byte
	var mu sync.Mutex

	execCtx := &runtime.ExecutionContext{
		SessionID:      "session1",
		TurnID:         "turn1",
		ApprovalCache:  cache,
		SandboxAttempt: createMockSandboxAttempt(runtime.SandboxNone),
		OutputWriter: &writerFunc{
			writeFunc: func(p []byte) (int, error) {
				mu.Lock()
				output = append(output, p...)
				mu.Unlock()
				return len(p), nil
			},
		},
	}

	result, err := orch.Execute(ctx, req, execCtx)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Response.StreamedOutput)

	mu.Lock()
	assert.Equal(t, "chunk1\nchunk2\nchunk3\n", string(output))
	mu.Unlock()
}

// ============================================================================
// Parallel Execution Tests
// ============================================================================

func TestOrchestratorExecuteParallel(t *testing.T) {
	registry := runtime.NewToolRegistry()

	// Create tools that track execution timing
	var mu sync.Mutex
	executionTimes := make(map[string]time.Time)

	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("parallel_tool_%d", i)
		tool := NewMockTool(name)
		tool.executeFunc = func(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
			mu.Lock()
			executionTimes[req.ToolName] = time.Now()
			mu.Unlock()
			time.Sleep(50 * time.Millisecond) // Simulate work
			return &runtime.ToolResponse{
				Content: "output from " + req.ToolName,
			}, nil
		}
		registry.Register(tool)
	}

	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApproved)
	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	requests := []*runtime.ToolRequest{
		{CallID: "call1", ToolName: "parallel_tool_1", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call2", ToolName: "parallel_tool_2", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call3", ToolName: "parallel_tool_3", Arguments: `{}`, WorkingDirectory: "/workspace"},
	}

	ctx := context.Background()
	execCtx := &runtime.ExecutionContext{
		SessionID:      "session1",
		TurnID:         "turn1",
		ApprovalCache:  cache,
		SandboxAttempt: createMockSandboxAttempt(runtime.SandboxNone),
	}

	start := time.Now()
	results, err := orch.ExecuteParallel(ctx, requests, execCtx)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Len(t, results, 3)

	// Parallel execution should take roughly the same time as one execution,
	// not 3x the time (allowing for overhead)
	assert.Less(t, elapsed, 200*time.Millisecond, "parallel execution should be faster than sequential")

	// Verify all tools executed
	for _, result := range results {
		require.NotNil(t, result.Response)
		assert.Contains(t, result.Response.Content, "output from")
	}
}

func TestOrchestratorExecuteParallelWithErrors(t *testing.T) {
	registry := runtime.NewToolRegistry()

	successTool := NewMockTool("success_tool")
	registry.Register(successTool)

	failTool := NewMockTool("fail_tool")
	failTool.executeFunc = func(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
		return nil, errors.New("intentional failure")
	}
	registry.Register(failTool)

	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApproved)
	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	requests := []*runtime.ToolRequest{
		{CallID: "call1", ToolName: "success_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call2", ToolName: "fail_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
	}

	ctx := context.Background()
	execCtx := &runtime.ExecutionContext{
		SessionID:      "session1",
		TurnID:         "turn1",
		ApprovalCache:  cache,
		SandboxAttempt: createMockSandboxAttempt(runtime.SandboxNone),
	}

	results, err := orch.ExecuteParallel(ctx, requests, execCtx)

	// ExecuteParallel should not return error, but individual results should contain errors
	require.NoError(t, err)
	require.Len(t, results, 2)

	// First result should succeed
	assert.NotNil(t, results[0].Response)
	assert.Nil(t, results[0].Error)

	// Second result should fail
	assert.Nil(t, results[1].Response)
	assert.NotNil(t, results[1].Error)
	assert.Contains(t, results[1].Error.Error(), "intentional failure")
}

func TestOrchestratorNonParallelToolsSequential(t *testing.T) {
	registry := runtime.NewToolRegistry()

	var mu sync.Mutex
	executionOrder := []string{}

	tool := NewMockTool("sequential_tool")
	tool.supportsParallel = false
	tool.executeFunc = func(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
		mu.Lock()
		executionOrder = append(executionOrder, req.CallID)
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		return &runtime.ToolResponse{Content: "done"}, nil
	}
	registry.Register(tool)

	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApproved)
	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	requests := []*runtime.ToolRequest{
		{CallID: "call1", ToolName: "sequential_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call2", ToolName: "sequential_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
		{CallID: "call3", ToolName: "sequential_tool", Arguments: `{}`, WorkingDirectory: "/workspace"},
	}

	ctx := context.Background()
	execCtx := &runtime.ExecutionContext{
		SessionID:      "session1",
		TurnID:         "turn1",
		ApprovalCache:  cache,
		SandboxAttempt: createMockSandboxAttempt(runtime.SandboxNone),
	}

	results, err := orch.ExecuteParallel(ctx, requests, execCtx)

	require.NoError(t, err)
	require.Len(t, results, 3)

	mu.Lock()
	// For non-parallel tools, execution should be sequential
	assert.Equal(t, []string{"call1", "call2", "call3"}, executionOrder)
	mu.Unlock()
}

// ============================================================================
// Sandbox Selection Tests
// ============================================================================

func TestSelectSandbox_NativeFirst(t *testing.T) {
	selector := NewSandboxSelector()

	tool := NewMockTool("test_tool")
	tool.sandboxPreference = runtime.SandboxAuto

	req := &runtime.ToolRequest{
		WorkingDirectory: "/workspace",
	}

	policy := runtime.SandboxPolicy(runtime.SandboxReadOnly)

	attempt := selector.SelectSandbox(tool, req, policy, false)

	require.NotNil(t, attempt)
	// First attempt should try available sandbox (bubblewrap, docker, or none)
	assert.True(t,
		attempt.Type == runtime.SandboxBubblewrap ||
			attempt.Type == runtime.SandboxDocker ||
			attempt.Type == runtime.SandboxNone,
		"expected sandbox type to be Bubblewrap, Docker, or None, got %v", attempt.Type,
	)
}

func TestSelectSandbox_RequireSandbox(t *testing.T) {
	selector := NewSandboxSelector()

	tool := NewMockTool("test_tool")
	tool.sandboxPreference = runtime.SandboxRequire

	req := &runtime.ToolRequest{
		WorkingDirectory: "/workspace",
	}

	policy := runtime.SandboxPolicy(runtime.SandboxReadOnly)

	attempt := selector.SelectSandbox(tool, req, policy, false)

	require.NotNil(t, attempt)
	// Should use sandbox
	assert.NotEqual(t, runtime.SandboxNone, attempt.Type)
}

func TestSelectSandbox_ForbidSandbox(t *testing.T) {
	selector := NewSandboxSelector()

	tool := NewMockTool("test_tool")
	tool.sandboxPreference = runtime.SandboxForbid

	req := &runtime.ToolRequest{
		WorkingDirectory: "/workspace",
	}

	policy := runtime.SandboxPolicy(runtime.SandboxReadOnly)

	attempt := selector.SelectSandbox(tool, req, policy, false)

	require.NotNil(t, attempt)
	// Should not use sandbox
	assert.Equal(t, runtime.SandboxNone, attempt.Type)
}

func TestSelectSandbox_EscalatedPermissions(t *testing.T) {
	selector := NewSandboxSelector()

	tool := NewMockTool("test_tool")
	tool.wantsEscalated = true

	req := &runtime.ToolRequest{
		WorkingDirectory: "/workspace",
		Metadata: map[string]interface{}{
			"with_escalated_permissions": true,
		},
	}

	policy := runtime.SandboxPolicy(runtime.SandboxReadOnly)

	attempt := selector.SelectSandbox(tool, req, policy, false)

	require.NotNil(t, attempt)
	// Should bypass sandbox when escalated permissions requested
	assert.Equal(t, runtime.SandboxNone, attempt.Type)
}

func TestSelectSandbox_RetryWithoutSandbox(t *testing.T) {
	selector := NewSandboxSelector()

	tool := NewMockTool("test_tool")
	tool.sandboxPreference = runtime.SandboxAuto

	req := &runtime.ToolRequest{
		WorkingDirectory: "/workspace",
	}

	policy := runtime.SandboxPolicy(runtime.SandboxReadOnly)

	// First attempt
	firstAttempt := selector.SelectSandbox(tool, req, policy, false)
	require.NotNil(t, firstAttempt)

	// Retry attempt should not use sandbox
	retryAttempt := selector.SelectSandbox(tool, req, policy, true)
	require.NotNil(t, retryAttempt)
	assert.Equal(t, runtime.SandboxNone, retryAttempt.Type)
}

// ============================================================================
// Helper Functions and Types
// ============================================================================

func boolPtr(b bool) *bool {
	return &b
}

func createMockSandboxAttempt(sandboxType runtime.SandboxType) *runtime.SandboxAttempt {
	return &runtime.SandboxAttempt{
		Type:             sandboxType,
		Policy:           runtime.SandboxReadOnly,
		WorkingDirectory: "/workspace",
		ReadOnlyPaths:    []string{"/usr", "/lib"},
		ReadWritePaths:   []string{"/workspace"},
		NetworkEnabled:   false,
	}
}

// writerFunc is a simple io.Writer implementation for testing
type writerFunc struct {
	writeFunc func([]byte) (int, error)
}

func (w *writerFunc) Write(p []byte) (int, error) {
	return w.writeFunc(p)
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkOrchestratorExecuteSingle(b *testing.B) {
	registry := runtime.NewToolRegistry()
	mockTool := NewMockTool("bench_tool")
	registry.Register(mockTool)

	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApproved)
	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	req := &runtime.ToolRequest{
		CallID:           "bench_call",
		ToolName:         "bench_tool",
		Arguments:        `{}`,
		WorkingDirectory: "/workspace",
	}

	ctx := context.Background()
	execCtx := &runtime.ExecutionContext{
		SessionID:      "bench_session",
		TurnID:         "bench_turn",
		ApprovalCache:  cache,
		SandboxAttempt: createMockSandboxAttempt(runtime.SandboxNone),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		orch.Execute(ctx, req, execCtx)
	}
}

func BenchmarkOrchestratorExecuteParallel(b *testing.B) {
	registry := runtime.NewToolRegistry()
	for i := 1; i <= 5; i++ {
		tool := NewMockTool(fmt.Sprintf("bench_tool_%d", i))
		registry.Register(tool)
	}

	cache := runtime.NewMemoryApprovalCache()
	handler := NewMockApprovalHandler(runtime.ApprovalApproved)
	orch := NewOrchestrator(registry, cache, handler.RequestApproval)

	requests := make([]*runtime.ToolRequest, 5)
	for i := 0; i < 5; i++ {
		requests[i] = &runtime.ToolRequest{
			CallID:           fmt.Sprintf("bench_call_%d", i),
			ToolName:         fmt.Sprintf("bench_tool_%d", i+1),
			Arguments:        `{}`,
			WorkingDirectory: "/workspace",
		}
	}

	ctx := context.Background()
	execCtx := &runtime.ExecutionContext{
		SessionID:      "bench_session",
		TurnID:         "bench_turn",
		ApprovalCache:  cache,
		SandboxAttempt: createMockSandboxAttempt(runtime.SandboxNone),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		orch.ExecuteParallel(ctx, requests, execCtx)
	}
}
