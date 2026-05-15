package orchestrator

import (
	"context"
	"sync"
	"time"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// ExecutionEngine handles parallel and sequential tool execution.
// It coordinates multiple tool calls, respecting parallelism constraints and
// managing concurrent execution safely.
type ExecutionEngine struct {
	// Maximum number of concurrent tool executions
	maxParallel int
}

// NewExecutionEngine creates a new execution engine with default settings.
func NewExecutionEngine() *ExecutionEngine {
	return &ExecutionEngine{
		maxParallel: 10, // Default to 10 concurrent executions
	}
}

// NewExecutionEngineWithLimit creates an execution engine with a custom concurrency limit.
func NewExecutionEngineWithLimit(maxParallel int) *ExecutionEngine {
	if maxParallel <= 0 {
		maxParallel = 1
	}
	return &ExecutionEngine{
		maxParallel: maxParallel,
	}
}

// ExecuteParallel executes multiple tool requests concurrently where possible.
// Tools that don't support parallel execution are run sequentially.
// Returns results in the same order as requests.
func (e *ExecutionEngine) ExecuteParallel(
	ctx context.Context,
	orchestrator *Orchestrator,
	requests []*runtime.ToolRequest,
	execCtx *runtime.ExecutionContext,
) ([]*runtime.ExecutionResult, error) {
	if len(requests) == 0 {
		return []*runtime.ExecutionResult{}, nil
	}

	// Separate parallel and sequential requests
	parallelReqs, sequentialReqs := e.groupByParallelism(orchestrator, requests)

	// Create result slots matching original request order
	results := make([]*runtime.ExecutionResult, len(requests))
	resultsMu := sync.Mutex{}

	// Build index map: CallID -> original position
	indexMap := make(map[string]int)
	for i, req := range requests {
		indexMap[req.CallID] = i
	}

	var wg sync.WaitGroup
	var firstError error
	errorMu := sync.Mutex{}

	// Execute parallel requests concurrently
	if len(parallelReqs) > 0 {
		// Use semaphore to limit concurrency
		sem := make(chan struct{}, e.maxParallel)

		for _, req := range parallelReqs {
			wg.Add(1)
			go func(request *runtime.ToolRequest) {
				defer wg.Done()

				// Acquire semaphore
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					return
				}

				// Clone execution context for this goroutine
				clonedExecCtx := e.cloneExecutionContext(execCtx)

				// Execute the tool
				result, err := orchestrator.Execute(ctx, request, clonedExecCtx)

				// Store result - orchestrator.Execute now returns result even on error
				// to ensure accurate timestamp capture. Timestamps are recorded during
				// actual execution (inside orchestrator.Execute), not during aggregation.
				resultsMu.Lock()
				if result != nil {
					results[indexMap[request.CallID]] = result
				}
				resultsMu.Unlock()

				// Track first error
				if err != nil {
					errorMu.Lock()
					if firstError == nil {
						firstError = err
					}
					errorMu.Unlock()
				}
			}(req)
		}
	}

	// Execute sequential requests one by one
	if len(sequentialReqs) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for _, req := range sequentialReqs {
				// Check context cancellation
				if ctx.Err() != nil {
					return
				}

				// Clone execution context
				clonedExecCtx := e.cloneExecutionContext(execCtx)

				// Execute sequentially
				result, err := orchestrator.Execute(ctx, req, clonedExecCtx)

				// Store result - orchestrator.Execute now returns result even on error
				// to ensure accurate timestamp capture. Timestamps are recorded during
				// actual execution (inside orchestrator.Execute), not during aggregation.
				resultsMu.Lock()
				if result != nil {
					results[indexMap[req.CallID]] = result
				}
				resultsMu.Unlock()

				// Track first error (but continue executing other tools)
				if err != nil {
					errorMu.Lock()
					if firstError == nil {
						firstError = err
					}
					errorMu.Unlock()
				}
			}
		}()
	}

	// Wait for all executions to complete
	wg.Wait()

	// Note: We don't return the error here because we want to return all results.
	// Callers should check individual result.Error fields.
	return results, nil
}

// ExecuteSequential executes requests one by one in order.
// Useful for tools that must run in strict sequence.
func (e *ExecutionEngine) ExecuteSequential(
	ctx context.Context,
	orchestrator *Orchestrator,
	requests []*runtime.ToolRequest,
	execCtx *runtime.ExecutionContext,
) ([]*runtime.ExecutionResult, error) {
	results := make([]*runtime.ExecutionResult, 0, len(requests))

	for _, req := range requests {
		// Check context cancellation
		if ctx.Err() != nil {
			return results, ctx.Err()
		}

		// Clone execution context for each request
		clonedExecCtx := e.cloneExecutionContext(execCtx)

		// Execute the tool
		result, _ := orchestrator.Execute(ctx, req, clonedExecCtx)

		// Append result - orchestrator.Execute now returns result even on error
		// to ensure accurate timestamp capture. Timestamps are recorded during
		// actual execution (inside orchestrator.Execute), not during aggregation.
		if result != nil {
			results = append(results, result)
		}

		// Continue executing remaining tools even if one fails
		// (orchestrator behavior: collect all results)
	}

	return results, nil
}

// ExecuteBatched executes requests in batches with a delay between batches.
// Useful for rate limiting or avoiding resource exhaustion.
func (e *ExecutionEngine) ExecuteBatched(
	ctx context.Context,
	orchestrator *Orchestrator,
	requests []*runtime.ToolRequest,
	execCtx *runtime.ExecutionContext,
	batchSize int,
	batchDelay time.Duration,
) ([]*runtime.ExecutionResult, error) {
	if batchSize <= 0 {
		batchSize = 5
	}

	allResults := make([]*runtime.ExecutionResult, 0, len(requests))

	for i := 0; i < len(requests); i += batchSize {
		// Check context cancellation
		if ctx.Err() != nil {
			return allResults, ctx.Err()
		}

		// Get batch
		end := i + batchSize
		if end > len(requests) {
			end = len(requests)
		}
		batch := requests[i:end]

		// Execute batch
		results, err := e.ExecuteParallel(ctx, orchestrator, batch, execCtx)
		if err != nil {
			return allResults, err
		}

		allResults = append(allResults, results...)

		// Wait between batches (except after last batch)
		if end < len(requests) && batchDelay > 0 {
			select {
			case <-time.After(batchDelay):
			case <-ctx.Done():
				return allResults, ctx.Err()
			}
		}
	}

	return allResults, nil
}

// groupByParallelism separates requests into parallel and sequential groups.
func (e *ExecutionEngine) groupByParallelism(
	orchestrator *Orchestrator,
	requests []*runtime.ToolRequest,
) (parallel []*runtime.ToolRequest, sequential []*runtime.ToolRequest) {
	parallel = []*runtime.ToolRequest{}
	sequential = []*runtime.ToolRequest{}

	for _, req := range requests {
		tool := orchestrator.registry.Get(req.ToolName)
		if tool == nil {
			// Unknown tool - treat as sequential to be safe
			sequential = append(sequential, req)
			continue
		}

		if tool.SupportsParallel() {
			parallel = append(parallel, req)
		} else {
			sequential = append(sequential, req)
		}
	}

	return parallel, sequential
}

// cloneExecutionContext creates a shallow copy of the execution context.
// This is necessary for concurrent execution to avoid race conditions.
func (e *ExecutionEngine) cloneExecutionContext(execCtx *runtime.ExecutionContext) *runtime.ExecutionContext {
	if execCtx == nil {
		return nil
	}

	return &runtime.ExecutionContext{
		SessionID:       execCtx.SessionID,
		TurnID:          execCtx.TurnID,
		SandboxAttempt:  execCtx.SandboxAttempt,
		ApprovalCache:   execCtx.ApprovalCache,
		OutputWriter:    execCtx.OutputWriter,
		StartTime:       execCtx.StartTime,
		AlreadyApproved: execCtx.AlreadyApproved,
	}
}

// ExecutionStats provides statistics about a batch execution.
type ExecutionStats struct {
	TotalRequests    int
	SuccessCount     int
	ErrorCount       int
	ParallelCount    int
	SequentialCount  int
	TotalDuration    time.Duration
	AverageDuration  time.Duration
	FastestExecution time.Duration
	SlowestExecution time.Duration
}

// ComputeStats analyzes execution results and computes statistics.
func (e *ExecutionEngine) ComputeStats(results []*runtime.ExecutionResult) *ExecutionStats {
	stats := &ExecutionStats{
		TotalRequests: len(results),
	}

	if len(results) == 0 {
		return stats
	}

	var totalDuration time.Duration
	var minDuration time.Duration
	var maxDuration time.Duration
	firstResult := true

	for _, result := range results {
		if result == nil {
			continue
		}

		// Count successes and errors
		if result.Error == nil {
			stats.SuccessCount++
		} else {
			stats.ErrorCount++
		}

		// Count parallel vs sequential
		// We'd need access to registry to check SupportsParallel()
		// For now, just track based on whether sandbox was used
		// (this is a simplification)
		_ = result.Request.ToolName

		// Calculate durations
		duration := result.EndTime.Sub(result.StartTime)
		totalDuration += duration

		if firstResult {
			minDuration = duration
			maxDuration = duration
			firstResult = false
		} else {
			if duration < minDuration {
				minDuration = duration
			}
			if duration > maxDuration {
				maxDuration = duration
			}
		}
	}

	stats.TotalDuration = totalDuration
	if stats.TotalRequests > 0 {
		stats.AverageDuration = totalDuration / time.Duration(stats.TotalRequests)
	}
	stats.FastestExecution = minDuration
	stats.SlowestExecution = maxDuration

	return stats
}

// ExecutionPlan represents a plan for executing multiple tools.
// It can be used to preview execution strategy before running.
type ExecutionPlan struct {
	ParallelBatch   []*runtime.ToolRequest
	SequentialBatch []*runtime.ToolRequest
	TotalRequests   int
	EstimatedTime   time.Duration
}

// PlanExecution creates an execution plan without actually running tools.
func (e *ExecutionEngine) PlanExecution(
	orchestrator *Orchestrator,
	requests []*runtime.ToolRequest,
) *ExecutionPlan {
	parallel, sequential := e.groupByParallelism(orchestrator, requests)

	plan := &ExecutionPlan{
		ParallelBatch:   parallel,
		SequentialBatch: sequential,
		TotalRequests:   len(requests),
	}

	// Estimate time (rough heuristic)
	// Assume parallel tools take 1 time unit together, sequential add up
	if len(parallel) > 0 {
		plan.EstimatedTime += time.Second // Parallel batch
	}
	plan.EstimatedTime += time.Duration(len(sequential)) * time.Second

	return plan
}

// CancelableExecution wraps an execution with cancellation support.
type CancelableExecution struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	result []*runtime.ExecutionResult
	err    error
	mu     sync.Mutex
}

// StartCancelableExecution starts a tool execution that can be cancelled.
func (e *ExecutionEngine) StartCancelableExecution(
	parentCtx context.Context,
	orchestrator *Orchestrator,
	requests []*runtime.ToolRequest,
	execCtx *runtime.ExecutionContext,
) *CancelableExecution {
	ctx, cancel := context.WithCancel(parentCtx)

	exec := &CancelableExecution{
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	go func() {
		defer close(exec.done)

		result, err := e.ExecuteParallel(ctx, orchestrator, requests, execCtx)

		exec.mu.Lock()
		exec.result = result
		exec.err = err
		exec.mu.Unlock()
	}()

	return exec
}

// Cancel stops the execution.
func (c *CancelableExecution) Cancel() {
	c.cancel()
}

// Wait blocks until execution completes or is cancelled.
func (c *CancelableExecution) Wait() ([]*runtime.ExecutionResult, error) {
	<-c.done
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.result, c.err
}

// IsDone returns true if execution has completed.
func (c *CancelableExecution) IsDone() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}
