// Package orchestrator provides the core tool orchestration system for Codex.
//
// The orchestrator coordinates tool execution including:
//   - Tool registration and discovery
//   - Approval workflow management with caching
//   - Sandbox selection and escalation
//   - Parallel tool execution
//   - Streaming output support
//   - Error handling and retry logic
//
// Architecture:
//   - Orchestrator: Main coordinator that handles single and parallel tool execution
//   - ApprovalManager: Manages approval requests and caching
//   - SandboxSelector: Determines appropriate sandbox configuration
//   - ExecutionEngine: Handles parallel execution with dependency resolution
package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// ApprovalHandler is a function that requests user approval for a tool operation.
// It receives an approval request and returns the user's decision or an error.
type ApprovalHandler func(ctx context.Context, req *runtime.ApprovalRequest) (runtime.ApprovalDecision, error)

// Orchestrator coordinates tool execution with approval, sandboxing, and retry logic.
// It serves as the main entry point for executing tools in the Codex system.
type Orchestrator struct {
	registry        *runtime.ToolRegistry
	approvalCache   runtime.ApprovalCache
	approvalHandler ApprovalHandler
	sandboxSelector *SandboxSelector
	approvalManager *ApprovalManager
	executionEngine *ExecutionEngine
}

// NewOrchestrator creates a new tool orchestrator with the given configuration.
func NewOrchestrator(
	registry *runtime.ToolRegistry,
	approvalCache runtime.ApprovalCache,
	approvalHandler ApprovalHandler,
) *Orchestrator {
	approvalManager := NewApprovalManager(approvalCache, approvalHandler)
	sandboxSelector := NewSandboxSelector()
	executionEngine := NewExecutionEngine()

	return &Orchestrator{
		registry:        registry,
		approvalCache:   approvalCache,
		approvalHandler: approvalHandler,
		sandboxSelector: sandboxSelector,
		approvalManager: approvalManager,
		executionEngine: executionEngine,
	}
}

// Execute runs a single tool with the given request and execution context.
// It handles the complete execution lifecycle including:
//   - Tool lookup and validation
//   - Approval checking and requesting
//   - Sandbox selection
//   - Tool execution
//   - Retry on sandbox denial
//   - Error handling
func (o *Orchestrator) Execute(
	ctx context.Context,
	req *runtime.ToolRequest,
	execCtx *runtime.ExecutionContext,
) (*runtime.ExecutionResult, error) {
	startTime := time.Now()

	// Look up the tool
	tool := o.registry.Get(req.ToolName)
	if tool == nil {
		return nil, &runtime.ToolError{
			Kind:    runtime.ErrorInternal,
			Message: fmt.Sprintf("tool not found: %s", req.ToolName),
		}
	}

	// Initialize execution result
	// Note: StartTime represents when the request was received, not when execution begins.
	// Actual execution start is tracked at execCtx.StartTime (set just before tool.Execute).
	result := &runtime.ExecutionResult{
		Request:   req,
		StartTime: startTime,
	}

	// Check for cached approval
	approvalKey := tool.ApprovalKey(req)
	cachedDecision := o.approvalCache.Get(approvalKey)
	alreadyApproved := cachedDecision != nil && *cachedDecision == runtime.ApprovalApprovedForSession
	execCtx.AlreadyApproved = alreadyApproved

	// Select initial sandbox configuration
	policy := execCtx.SandboxAttempt.Policy
	wantsEscalated := tool.WantsEscalatedFirstAttempt(req)
	sandboxAttempt := o.sandboxSelector.SelectSandbox(tool, req, policy, wantsEscalated)
	execCtx.SandboxAttempt = sandboxAttempt

	// Check if approval is needed for initial execution
	approvalPolicy := o.getApprovalPolicyFromSandboxPolicy(policy)
	needsApproval := tool.NeedsInitialApproval(req, approvalPolicy, policy)

	if needsApproval && !alreadyApproved {
		// Request approval
		decision, err := o.approvalManager.RequestApproval(ctx, tool, req, sandboxAttempt, false, "")
		if err != nil {
			return nil, err
		}

		result.ApprovalRequired = true

		// Handle approval decision
		switch decision {
		case runtime.ApprovalDenied:
			// Pre-execution rejection - no timing info available yet
			return nil, &runtime.ToolError{
				Kind:    runtime.ErrorRejected,
				Message: "user denied approval",
			}
		case runtime.ApprovalAbort:
			// Pre-execution abort - no timing info available yet
			return nil, &runtime.ToolError{
				Kind:    runtime.ErrorRejected,
				Message: "user aborted operation",
			}
		case runtime.ApprovalApprovedForSession:
			// Cache the decision
			o.approvalCache.Put(approvalKey, decision)
		}
	}

	// Execute the tool
	execCtx.StartTime = time.Now()
	response, execErr := tool.Execute(ctx, req, execCtx)

	// Check if we got a sandbox denial and should retry
	if execErr != nil {
		if toolErr, ok := execErr.(*runtime.ToolError); ok && toolErr.Kind == runtime.ErrorSandboxDenied {
			if tool.EscalateOnFailure() && tool.SandboxRetryData(req) != nil {
				// Retry without sandbox
				result.RetryCount++

				// Request approval for retry if needed
				needsRetryApproval := tool.NeedsRetryApproval(approvalPolicy)
				if needsRetryApproval && !alreadyApproved {
					retryDecision, err := o.approvalManager.RequestApproval(
						ctx, tool, req, sandboxAttempt, true, toolErr.Message,
					)
					if err != nil {
						// Execution already happened, set end time and return result
						result.EndTime = time.Now()
						result.Error = toolErr
						result.SandboxUsed = execCtx.SandboxAttempt.Type != runtime.SandboxNone
						return result, err
					}

					result.ApprovalRequired = true

					switch retryDecision {
					case runtime.ApprovalDenied:
						// Execution already happened (sandbox attempt), set end time and return result
						result.EndTime = time.Now()
						result.Error = &runtime.ToolError{
							Kind:    runtime.ErrorRejected,
							Message: "user denied retry approval",
						}
						result.SandboxUsed = execCtx.SandboxAttempt.Type != runtime.SandboxNone
						return result, result.Error
					case runtime.ApprovalAbort:
						// Execution already happened (sandbox attempt), set end time and return result
						result.EndTime = time.Now()
						result.Error = &runtime.ToolError{
							Kind:    runtime.ErrorRejected,
							Message: "user aborted retry",
						}
						result.SandboxUsed = execCtx.SandboxAttempt.Type != runtime.SandboxNone
						return result, result.Error
					case runtime.ApprovalApprovedForSession:
						o.approvalCache.Put(approvalKey, retryDecision)
					}
				}

				// Retry without sandbox
				noSandboxAttempt := o.sandboxSelector.SelectSandbox(tool, req, policy, true)
				execCtx.SandboxAttempt = noSandboxAttempt
				execCtx.StartTime = time.Now()

				response, execErr = tool.Execute(ctx, req, execCtx)
			}
		}
	}

	// Build final result
	result.EndTime = time.Now()
	result.SandboxUsed = execCtx.SandboxAttempt.Type != runtime.SandboxNone

	if execErr != nil {
		// Convert to ToolError if needed
		if toolErr, ok := execErr.(*runtime.ToolError); ok {
			result.Error = toolErr
		} else {
			// Wrap non-ToolError errors
			result.Error = &runtime.ToolError{
				Kind:    runtime.ErrorExecution,
				Message: execErr.Error(),
				Cause:   execErr,
			}
		}
		// Return result with accurate timestamps even on error
		return result, execErr
	}

	result.Response = response
	return result, nil
}

// ExecuteParallel runs multiple tools concurrently when possible.
// Tools that don't support parallel execution are run sequentially.
// Returns a slice of execution results matching the order of requests.
func (o *Orchestrator) ExecuteParallel(
	ctx context.Context,
	requests []*runtime.ToolRequest,
	execCtx *runtime.ExecutionContext,
) ([]*runtime.ExecutionResult, error) {
	return o.executionEngine.ExecuteParallel(ctx, o, requests, execCtx)
}

// getApprovalPolicyFromSandboxPolicy converts sandbox policy to approval policy.
func (o *Orchestrator) getApprovalPolicyFromSandboxPolicy(sandboxPolicy runtime.SandboxPolicy) runtime.ApprovalPolicy {
	switch sandboxPolicy {
	case runtime.SandboxDangerFullAccess:
		return runtime.ApprovalNever
	case runtime.SandboxReadOnly:
		return runtime.ApprovalOnFailure
	case runtime.SandboxWorkspaceWrite:
		return runtime.ApprovalOnRequest
	default:
		return runtime.ApprovalOnRequest
	}
}

// GetRegistry returns the tool registry (for creating new orchestrators with different handlers).
func (o *Orchestrator) GetRegistry() *runtime.ToolRegistry {
	return o.registry
}

// GetApprovalCache returns the approval cache (for creating new orchestrators with different handlers).
func (o *Orchestrator) GetApprovalCache() runtime.ApprovalCache {
	return o.approvalCache
}
