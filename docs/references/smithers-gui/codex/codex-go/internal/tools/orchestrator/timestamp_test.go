package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// TestTimestampAccuracyOnSuccess verifies that execution timestamps are captured
// during actual execution, not during result aggregation, for successful executions.
func TestTimestampAccuracyOnSuccess(t *testing.T) {
	// Create a tool that sleeps for a known duration
	sleepDuration := 50 * time.Millisecond
	tool := NewMockTool("slow_tool")
	tool.executeFunc = func(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
		time.Sleep(sleepDuration)
		return &runtime.ToolResponse{
			Content: "success",
			Success: boolPtr(true),
		}, nil
	}

	// Setup orchestrator
	registry := runtime.NewToolRegistry()
	registry.Register(tool)
	cache := runtime.NewMemoryApprovalCache()
	orchestrator := NewOrchestrator(registry, cache, nil)

	// Execute the tool
	ctx := context.Background()
	req := &runtime.ToolRequest{
		CallID:           "test-1",
		ToolName:         "slow_tool",
		Arguments:        "{}",
		WorkingDirectory: "/tmp",
	}
	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
		SandboxAttempt: &runtime.SandboxAttempt{
			Type:   runtime.SandboxNone,
			Policy: runtime.SandboxDangerFullAccess,
		},
		ApprovalCache: cache,
	}

	// Record time before execution
	beforeExec := time.Now()
	result, err := orchestrator.Execute(ctx, req, execCtx)
	afterExec := time.Now()

	// Verify no error
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.Error)

	// Verify timestamps are reasonable
	assert.False(t, result.StartTime.IsZero(), "StartTime should be set")
	assert.False(t, result.EndTime.IsZero(), "EndTime should be set")

	// Verify timestamps are in correct order and range
	assert.True(t, result.StartTime.After(beforeExec) || result.StartTime.Equal(beforeExec),
		"StartTime should be after or equal to beforeExec")
	assert.True(t, result.EndTime.Before(afterExec) || result.EndTime.Equal(afterExec),
		"EndTime should be before or equal to afterExec")
	assert.True(t, result.EndTime.After(result.StartTime) || result.EndTime.Equal(result.StartTime),
		"EndTime should be after or equal to StartTime")

	// Verify execution duration matches expected sleep duration
	executionDuration := result.EndTime.Sub(result.StartTime)
	assert.GreaterOrEqual(t, executionDuration, sleepDuration,
		"Execution duration should be at least as long as sleep duration")

	// Allow some tolerance for overhead (should be less than 100ms extra)
	maxExpectedDuration := sleepDuration + 100*time.Millisecond
	assert.LessOrEqual(t, executionDuration, maxExpectedDuration,
		"Execution duration should not exceed sleep duration + overhead tolerance")
}

// TestTimestampAccuracyOnError verifies that error timestamps are captured
// during actual execution, not during result aggregation.
func TestTimestampAccuracyOnError(t *testing.T) {
	// Create a tool that sleeps then fails
	sleepDuration := 50 * time.Millisecond
	tool := NewMockTool("failing_tool")
	tool.executeFunc = func(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
		time.Sleep(sleepDuration)
		return nil, errors.New("execution failed")
	}

	// Setup orchestrator
	registry := runtime.NewToolRegistry()
	registry.Register(tool)
	cache := runtime.NewMemoryApprovalCache()
	orchestrator := NewOrchestrator(registry, cache, nil)

	// Execute the tool
	ctx := context.Background()
	req := &runtime.ToolRequest{
		CallID:           "test-1",
		ToolName:         "failing_tool",
		Arguments:        "{}",
		WorkingDirectory: "/tmp",
	}
	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
		SandboxAttempt: &runtime.SandboxAttempt{
			Type:   runtime.SandboxNone,
			Policy: runtime.SandboxDangerFullAccess,
		},
		ApprovalCache: cache,
	}

	// Record time before execution
	beforeExec := time.Now()
	result, err := orchestrator.Execute(ctx, req, execCtx)
	afterExec := time.Now()

	// Verify error occurred
	require.Error(t, err)
	require.NotNil(t, result, "Result should be returned even on error")
	assert.NotNil(t, result.Error, "Result.Error should be set")

	// Verify timestamps are reasonable
	assert.False(t, result.StartTime.IsZero(), "StartTime should be set even on error")
	assert.False(t, result.EndTime.IsZero(), "EndTime should be set even on error")

	// Verify timestamps are in correct order and range
	assert.True(t, result.StartTime.After(beforeExec) || result.StartTime.Equal(beforeExec),
		"StartTime should be after or equal to beforeExec")
	assert.True(t, result.EndTime.Before(afterExec) || result.EndTime.Equal(afterExec),
		"EndTime should be before or equal to afterExec")
	assert.True(t, result.EndTime.After(result.StartTime) || result.EndTime.Equal(result.StartTime),
		"EndTime should be after or equal to StartTime")

	// Verify execution duration matches expected sleep duration
	executionDuration := result.EndTime.Sub(result.StartTime)
	assert.GreaterOrEqual(t, executionDuration, sleepDuration,
		"Execution duration should be at least as long as sleep duration, even on error")

	// Allow some tolerance for overhead
	maxExpectedDuration := sleepDuration + 100*time.Millisecond
	assert.LessOrEqual(t, executionDuration, maxExpectedDuration,
		"Execution duration should not exceed sleep duration + overhead tolerance")
}

// TestTimestampAccuracyParallelExecution verifies that timestamps are accurate
// when executing multiple tools in parallel.
func TestTimestampAccuracyParallelExecution(t *testing.T) {
	sleepDuration := 50 * time.Millisecond

	// Create multiple tools with varying execution times
	tools := []*MockTool{
		NewMockTool("tool1"),
		NewMockTool("tool2"),
		NewMockTool("tool3"),
	}

	for i, tool := range tools {
		toolIndex := i
		tool.executeFunc = func(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
			// Each tool sleeps for a different duration
			time.Sleep(sleepDuration * time.Duration(toolIndex+1))
			return &runtime.ToolResponse{
				Content: "success",
				Success: boolPtr(true),
			}, nil
		}
	}

	// Setup orchestrator
	registry := runtime.NewToolRegistry()
	for _, tool := range tools {
		registry.Register(tool)
	}
	cache := runtime.NewMemoryApprovalCache()
	orchestrator := NewOrchestrator(registry, cache, nil)

	// Create requests
	requests := []*runtime.ToolRequest{
		{
			CallID:           "call-1",
			ToolName:         "tool1",
			Arguments:        "{}",
			WorkingDirectory: "/tmp",
		},
		{
			CallID:           "call-2",
			ToolName:         "tool2",
			Arguments:        "{}",
			WorkingDirectory: "/tmp",
		},
		{
			CallID:           "call-3",
			ToolName:         "tool3",
			Arguments:        "{}",
			WorkingDirectory: "/tmp",
		},
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
		SandboxAttempt: &runtime.SandboxAttempt{
			Type:   runtime.SandboxNone,
			Policy: runtime.SandboxDangerFullAccess,
		},
		ApprovalCache: cache,
	}

	// Execute in parallel
	ctx := context.Background()
	beforeExec := time.Now()
	results, err := orchestrator.ExecuteParallel(ctx, requests, execCtx)
	afterExec := time.Now()

	// Verify no error
	require.NoError(t, err)
	require.Len(t, results, 3)

	// Verify each result has accurate timestamps
	for i, result := range results {
		require.NotNil(t, result, "Result %d should not be nil", i)
		assert.Nil(t, result.Error, "Result %d should not have error", i)

		// Verify timestamps are set
		assert.False(t, result.StartTime.IsZero(), "Result %d StartTime should be set", i)
		assert.False(t, result.EndTime.IsZero(), "Result %d EndTime should be set", i)

		// Verify timestamps are in correct range
		assert.True(t, result.StartTime.After(beforeExec) || result.StartTime.Equal(beforeExec),
			"Result %d StartTime should be after or equal to beforeExec", i)
		assert.True(t, result.EndTime.Before(afterExec) || result.EndTime.Equal(afterExec),
			"Result %d EndTime should be before or equal to afterExec", i)
		assert.True(t, result.EndTime.After(result.StartTime) || result.EndTime.Equal(result.StartTime),
			"Result %d EndTime should be after or equal to StartTime", i)

		// Verify execution duration reflects the actual sleep time
		expectedSleep := sleepDuration * time.Duration(i+1)
		executionDuration := result.EndTime.Sub(result.StartTime)
		assert.GreaterOrEqual(t, executionDuration, expectedSleep,
			"Result %d execution duration should be at least %v", i, expectedSleep)

		// Allow some tolerance for overhead
		maxExpected := expectedSleep + 100*time.Millisecond
		assert.LessOrEqual(t, executionDuration, maxExpected,
			"Result %d execution duration should not exceed %v", i, maxExpected)
	}
}

// TestTimestampAccuracyParallelWithErrors verifies that timestamps are accurate
// when executing multiple tools in parallel, some of which fail.
func TestTimestampAccuracyParallelWithErrors(t *testing.T) {
	sleepDuration := 50 * time.Millisecond

	// Create tools where some succeed and some fail
	successTool := NewMockTool("success_tool")
	successTool.executeFunc = func(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
		time.Sleep(sleepDuration)
		return &runtime.ToolResponse{
			Content: "success",
			Success: boolPtr(true),
		}, nil
	}

	errorTool := NewMockTool("error_tool")
	errorTool.executeFunc = func(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
		time.Sleep(sleepDuration * 2)
		return nil, errors.New("execution failed")
	}

	// Setup orchestrator
	registry := runtime.NewToolRegistry()
	registry.Register(successTool)
	registry.Register(errorTool)
	cache := runtime.NewMemoryApprovalCache()
	orchestrator := NewOrchestrator(registry, cache, nil)

	// Create requests
	requests := []*runtime.ToolRequest{
		{
			CallID:           "call-1",
			ToolName:         "success_tool",
			Arguments:        "{}",
			WorkingDirectory: "/tmp",
		},
		{
			CallID:           "call-2",
			ToolName:         "error_tool",
			Arguments:        "{}",
			WorkingDirectory: "/tmp",
		},
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
		SandboxAttempt: &runtime.SandboxAttempt{
			Type:   runtime.SandboxNone,
			Policy: runtime.SandboxDangerFullAccess,
		},
		ApprovalCache: cache,
	}

	// Execute in parallel
	ctx := context.Background()
	beforeExec := time.Now()
	results, err := orchestrator.ExecuteParallel(ctx, requests, execCtx)
	afterExec := time.Now()

	// Verify no error from ExecuteParallel itself
	require.NoError(t, err)
	require.Len(t, results, 2)

	// Verify success result
	successResult := results[0]
	require.NotNil(t, successResult)
	assert.Nil(t, successResult.Error)
	assert.False(t, successResult.StartTime.IsZero())
	assert.False(t, successResult.EndTime.IsZero())
	successDuration := successResult.EndTime.Sub(successResult.StartTime)
	assert.GreaterOrEqual(t, successDuration, sleepDuration)
	assert.LessOrEqual(t, successDuration, sleepDuration+100*time.Millisecond)

	// Verify error result - THIS IS THE CRITICAL TEST FOR THE BUG FIX
	errorResult := results[1]
	require.NotNil(t, errorResult, "Error result should be returned")
	assert.NotNil(t, errorResult.Error, "Error result should have Error field set")
	assert.False(t, errorResult.StartTime.IsZero(), "Error result StartTime should be set")
	assert.False(t, errorResult.EndTime.IsZero(), "Error result EndTime should be set")

	// Verify error result timestamps are in correct range
	assert.True(t, errorResult.StartTime.After(beforeExec) || errorResult.StartTime.Equal(beforeExec),
		"Error result StartTime should be after or equal to beforeExec")
	assert.True(t, errorResult.EndTime.Before(afterExec) || errorResult.EndTime.Equal(afterExec),
		"Error result EndTime should be before or equal to afterExec")
	assert.True(t, errorResult.EndTime.After(errorResult.StartTime) || errorResult.EndTime.Equal(errorResult.StartTime),
		"Error result EndTime should be after or equal to StartTime")

	// MOST IMPORTANT: Verify error result execution duration reflects actual execution time
	errorDuration := errorResult.EndTime.Sub(errorResult.StartTime)
	expectedErrorSleep := sleepDuration * 2
	assert.GreaterOrEqual(t, errorDuration, expectedErrorSleep,
		"Error result execution duration should be at least %v (the time it actually ran), not ~0ms",
		expectedErrorSleep)
	assert.LessOrEqual(t, errorDuration, expectedErrorSleep+100*time.Millisecond,
		"Error result execution duration should not exceed %v + overhead", expectedErrorSleep)
}
