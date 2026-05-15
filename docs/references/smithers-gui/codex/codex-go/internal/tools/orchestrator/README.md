# Tools Orchestrator Package

This package provides the core tool orchestration system for Codex, coordinating tool execution with approval workflows, sandbox management, and parallel execution capabilities.

## Overview

The orchestrator is the central coordinator for all tool execution in Codex. It manages the complete lifecycle of tool operations including:
- Tool registry and discovery
- User approval workflows with caching
- Sandbox selection and escalation strategies
- Parallel and sequential execution planning
- Retry logic on sandbox denials
- Error handling and result aggregation

## Architecture

The orchestrator consists of four main components:

### 1. Orchestrator (orchestrator.go)

The main coordinator that handles single and parallel tool execution. It orchestrates approval checks, sandbox selection, tool execution, and retry logic.

```go
orchestrator := NewOrchestrator(registry, approvalCache, approvalHandler)

// Execute single tool
result, err := orchestrator.Execute(ctx, request, execContext)

// Execute multiple tools (parallel where possible)
results, err := orchestrator.ExecuteParallel(ctx, requests, execContext)
```

### 2. ApprovalManager (approval.go)

Manages the approval workflow with intelligent caching and risk assessment.

**Key Features:**
- **Approval Caching**: Stores "approve for session" decisions to avoid repeated prompts
- **Risk Assessment**: Analyzes commands for security risks (low/medium/high/critical)
- **Contextual Requests**: Builds approval requests with command details, justification, and risk info
- **Decision Parsing**: Converts user input to structured decisions

```go
decision, err := approvalManager.RequestApproval(ctx, tool, req, sandboxAttempt, false, "")

// Check cached approval
cachedDecision := approvalManager.CheckCachedApproval(tool, req)
```

### 3. SandboxSelector (sandbox_selector.go)

Determines appropriate sandbox configuration using an escalation strategy.

**Sandbox Types:**
- **Bubblewrap (Native)**: Linux namespace-based sandboxing (preferred)
- **Docker**: Container-based sandboxing (fallback)
- **None**: No sandboxing (for trusted operations or when required)

**Escalation Path:**
```
bubblewrap → docker → none
```

```go
selector := NewSandboxSelector()
attempt := selector.SelectSandbox(tool, req, policy, false)

// Escalate on failure
nextAttempt := selector.EscalateSandbox(currentAttempt)
```

### 4. ExecutionEngine (execution.go)

Handles parallel and sequential tool execution with concurrency control.

**Execution Modes:**
- **Parallel**: Runs parallelizable tools concurrently (with semaphore limiting)
- **Sequential**: Runs tools one-by-one in order
- **Batched**: Executes in batches with delays (for rate limiting)
- **Cancelable**: Supports mid-execution cancellation

```go
engine := NewExecutionEngine()

// Parallel execution
results, err := engine.ExecuteParallel(ctx, orchestrator, requests, execContext)

// Sequential execution
results, err := engine.ExecuteSequential(ctx, orchestrator, requests, execContext)

// Batched execution (5 tools per batch, 100ms delay)
results, err := engine.ExecuteBatched(ctx, orchestrator, requests, execContext, 5, 100*time.Millisecond)
```

## Approval Flow

The approval system provides security and user control over tool execution:

### Approval Decisions

```go
type ApprovalDecision int

const (
    ApprovalDenied           // Deny this request
    ApprovalApproved         // Approve this request only
    ApprovalApprovedForSession // Approve for entire session
    ApprovalAbort            // Abort the entire operation
)
```

### Approval Policies

Derived from sandbox policy:

| Sandbox Policy | Approval Policy | Behavior |
|---------------|----------------|----------|
| `DangerFullAccess` | `ApprovalNever` | Never ask (trusted mode) |
| `ReadOnly` | `ApprovalOnFailure` | Ask only on sandbox denial |
| `WorkspaceWrite` | `ApprovalOnRequest` | Ask before execution |

### Approval Workflow

1. **Check Cache**: Look for cached "approve for session" decision
2. **Skip if Cached**: Execute without prompting if approved
3. **Request Approval**: Prompt user with risk assessment
4. **Execute Tool**: Run with selected sandbox
5. **Retry on Denial**: If sandbox fails and tool supports escalation:
   - Request retry approval (if not cached)
   - Execute without sandbox
6. **Cache Decision**: Store "approve for session" for future requests

### Risk Assessment

The approval manager assesses risk for commands:

```go
type RiskLevel int

const (
    RiskLow      // Read-only operations
    RiskMedium   // Write operations, network access
    RiskHigh     // Dangerous commands, system paths
    RiskCritical // Destructive operations
)
```

**Risk Factors:**
- Dangerous commands (rm -rf, dd, etc.)
- System directory access (/, /etc, /usr)
- Network access enabled
- Write operations
- Command patterns

## Planning Modes

### Parallel vs Sequential

Tools declare whether they support parallel execution:

```go
type ToolRuntime interface {
    SupportsParallel() bool // Can run concurrently with other tools?
}
```

**Parallel Tools**: Read-only operations, queries, independent writes
**Sequential Tools**: Stateful operations, order-dependent writes, interactive tools

The execution engine automatically separates requests:

```go
// Parallel tools run concurrently (up to maxParallel limit)
// Sequential tools run one-by-one
parallelReqs, sequentialReqs := engine.groupByParallelism(orchestrator, requests)
```

### Execution Planning

Preview execution strategy before running:

```go
plan := engine.PlanExecution(orchestrator, requests)
fmt.Printf("Parallel: %d tools\n", len(plan.ParallelBatch))
fmt.Printf("Sequential: %d tools\n", len(plan.SequentialBatch))
fmt.Printf("Estimated time: %v\n", plan.EstimatedTime)
```

## Sandbox Selection

### Sandbox Preferences

Tools declare sandbox preferences:

```go
type SandboxPreference int

const (
    SandboxAuto    // Use sandbox based on policy (default)
    SandboxRequire // Must run in sandbox
    SandboxForbid  // Cannot run in sandbox
)
```

### Sandbox Policies

System-wide policies control default behavior:

```go
type SandboxPolicy int

const (
    SandboxReadOnly         // Read-only access
    SandboxWorkspaceWrite   // Write to workspace only
    SandboxDangerFullAccess // Full system access
)
```

### Selection Logic

```
1. Check if tool wants escalated permissions → No sandbox
2. Check tool preference:
   - Forbid → No sandbox
   - Require → Best available sandbox
   - Auto → Use policy:
     * DangerFullAccess → No sandbox
     * ReadOnly/WorkspaceWrite → Best available sandbox
3. Select best available:
   - Try bubblewrap (if available)
   - Fall back to docker (if available)
   - Fall back to none (if not required)
```

### Sandbox Escalation

When a tool fails due to sandbox restrictions:

1. Check if tool supports escalation (`EscalateOnFailure()`)
2. Request retry approval (if needed)
3. Escalate sandbox: `bubblewrap → docker → none`
4. Retry execution

## Usage Examples

### Basic Single Tool Execution

```go
package main

import (
    "context"
    "fmt"

    "github.com/evmts/codex/codex-go/internal/tools/orchestrator"
    "github.com/evmts/codex/codex-go/internal/tools/runtime"
)

func main() {
    // Create registry and register tools
    registry := runtime.NewToolRegistry()
    registry.Register("bash", bashTool)

    // Create orchestrator
    cache := runtime.NewMemoryApprovalCache()
    handler := func(ctx context.Context, req *runtime.ApprovalRequest) (runtime.ApprovalDecision, error) {
        // Prompt user for approval
        fmt.Printf("Approve: %s\n", req.Command)
        return runtime.ApprovalApproved, nil
    }

    orch := orchestrator.NewOrchestrator(registry, cache, handler)

    // Execute tool
    request := &runtime.ToolRequest{
        CallID:   "call-1",
        ToolName: "bash",
        Arguments: map[string]interface{}{
            "command": "ls -la",
        },
        WorkingDirectory: "/workspace",
    }

    execCtx := &runtime.ExecutionContext{
        SessionID: "session-1",
        SandboxAttempt: &runtime.SandboxAttempt{
            Policy: runtime.SandboxWorkspaceWrite,
        },
    }

    result, err := orch.Execute(context.Background(), request, execCtx)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    fmt.Printf("Output: %s\n", result.Response.Output)
}
```

### Parallel Execution

```go
// Create multiple requests
requests := []*runtime.ToolRequest{
    {CallID: "1", ToolName: "read_file", Arguments: map[string]interface{}{"path": "a.txt"}},
    {CallID: "2", ToolName: "read_file", Arguments: map[string]interface{}{"path": "b.txt"}},
    {CallID: "3", ToolName: "write_file", Arguments: map[string]interface{}{"path": "c.txt"}},
}

// Execute in parallel (read_file tools run concurrently, write_file runs separately)
results, err := orch.ExecuteParallel(ctx, requests, execCtx)

for _, result := range results {
    if result.Error != nil {
        fmt.Printf("Tool %s failed: %v\n", result.Request.ToolName, result.Error)
    } else {
        fmt.Printf("Tool %s succeeded\n", result.Request.ToolName)
    }
}
```

### Approval Caching

```go
// First execution: user approves for session
req1 := &runtime.ToolRequest{
    CallID:   "call-1",
    ToolName: "bash",
    Arguments: map[string]interface{}{"command": "git status"},
}

result1, _ := orch.Execute(ctx, req1, execCtx)
// User prompted, selects "Approve for Session"

// Second execution: no prompt (cached)
req2 := &runtime.ToolRequest{
    CallID:   "call-2",
    ToolName: "bash",
    Arguments: map[string]interface{}{"command": "git diff"},
}

result2, _ := orch.Execute(ctx, req2, execCtx)
// No prompt - uses cached approval
```

### Custom Execution Engine

```go
// Create engine with custom concurrency limit
engine := orchestrator.NewExecutionEngineWithLimit(5) // Max 5 concurrent

// Execute with batching for rate limiting
results, err := engine.ExecuteBatched(
    ctx,
    orch,
    requests,
    execCtx,
    10,              // 10 tools per batch
    500*time.Millisecond, // 500ms delay between batches
)
```

### Cancelable Execution

```go
// Start long-running execution
exec := engine.StartCancelableExecution(ctx, orch, requests, execCtx)

// Cancel from another goroutine
go func() {
    time.Sleep(5 * time.Second)
    exec.Cancel()
}()

// Wait for completion or cancellation
results, err := exec.Wait()
```

## Registry Helpers

The package includes `RegistryHelper` with convenience methods:

```go
helper := orchestrator.NewRegistryHelper(registry)

// Query tools
parallelTools := helper.GetParallelTools()
sandboxedTools := helper.GetToolsRequiringSandbox()

// Validate requests
err := helper.ValidateToolRequests(requests)

// Group requests
parallel, sequential := helper.GroupRequestsByParallelism(requests)

// Get snapshot
snapshot := helper.GetSnapshot()
fmt.Printf("Total tools: %d\n", snapshot.ToolCount)
fmt.Printf("Parallel: %d, Sequential: %d\n", snapshot.Parallel, snapshot.Sequential)
```

## Design Decisions

### Why Approval Caching?

Repeated prompts for similar operations create poor UX. The approval cache:
- Stores "approve for session" decisions keyed by tool + request signature
- Expires with session (not persisted)
- Allows users to grant temporary trust
- Still validates each request (just skips prompt)

### Why Sandbox Escalation?

Some operations legitimately need to escape the sandbox (e.g., package installation). The escalation strategy:
- Tries sandbox first (safe by default)
- Requests explicit approval before retry
- Provides clear risk assessment
- Maintains audit trail of escalations

### Why Separate Parallel and Sequential?

Not all tools can safely run concurrently:
- Stateful tools (editors, shells) need sequential execution
- File writes to same file need serialization
- Read-only tools can run freely in parallel

The execution engine respects these constraints automatically.

### Why Multiple Execution Modes?

Different scenarios need different strategies:
- **Parallel**: Maximum throughput for independent operations
- **Sequential**: Strict ordering for dependent operations
- **Batched**: Rate limiting for API calls or resource management
- **Cancelable**: Long-running operations need interruption

### Why Risk Assessment?

Users need context to make informed approval decisions:
- Command complexity varies widely
- System vs workspace operations have different risk profiles
- Network access adds new attack vectors
- Risk level helps users decide: "Is this safe?"

## Configuration

### Orchestrator Configuration

Configured via constructor dependencies:

```go
type Orchestrator struct {
    registry        *runtime.ToolRegistry  // Tool registry
    approvalCache   runtime.ApprovalCache  // Approval cache implementation
    approvalHandler ApprovalHandler        // User approval callback
    // ... internal components
}
```

### Execution Engine Configuration

```go
// Default: 10 concurrent tools
engine := NewExecutionEngine()

// Custom limit
engine := NewExecutionEngineWithLimit(5)
```

### Sandbox Configuration

Auto-detects available sandboxing:
- Checks for `/usr/bin/bwrap` (bubblewrap)
- Checks for `/usr/bin/docker` (Docker)
- Falls back gracefully if neither available

## Testing

The orchestrator includes comprehensive tests:
- Unit tests for each component
- Integration tests for full workflow
- Mock tools for controlled testing
- Parallel execution race detection

Run tests:
```bash
go test -v ./internal/tools/orchestrator/...
go test -race ./internal/tools/orchestrator/... # Check for race conditions
```

## Execution Statistics

Track execution metrics:

```go
results, _ := engine.ExecuteParallel(ctx, orch, requests, execCtx)

stats := engine.ComputeStats(results)
fmt.Printf("Total: %d\n", stats.TotalRequests)
fmt.Printf("Success: %d, Errors: %d\n", stats.SuccessCount, stats.ErrorCount)
fmt.Printf("Average duration: %v\n", stats.AverageDuration)
fmt.Printf("Fastest: %v, Slowest: %v\n", stats.FastestExecution, stats.SlowestExecution)
```

## Future Enhancements

Potential improvements:
- [ ] DAG-based dependency resolution for parallel execution
- [ ] Streaming execution results (partial results as tools complete)
- [ ] Execution checkpointing for long-running operations
- [ ] Tool execution telemetry and metrics
- [ ] Dynamic sandbox policy adjustment based on tool behavior
- [ ] Tool execution replay for debugging
- [ ] Resource quotas (CPU, memory, disk) per tool
- [ ] Distributed execution across multiple workers
