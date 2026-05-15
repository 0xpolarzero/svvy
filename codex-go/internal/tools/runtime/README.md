# Tools Runtime System

This package defines the core abstraction for tool execution in Codex Go, providing a clean interface for implementing tools with approval workflows, sandboxing, streaming output, and error handling.

## Architecture Overview

The tools runtime system follows these key principles:

1. **Interface-based design**: All tools implement the `ToolRuntime` interface
2. **Separation of concerns**: Approval, sandboxing, and execution are distinct phases
3. **Extensibility**: New tools can be added without modifying core logic
4. **Context propagation**: Execution context flows through all operations
5. **Streaming support**: Tools can emit incremental output for real-time display

## Core Components

### ToolRuntime Interface

The central interface that all tools must implement:

```go
type ToolRuntime interface {
    Name() string
    Execute(ctx context.Context, req *ToolRequest, execCtx *ExecutionContext) (*ToolResponse, error)
    ApprovalKey(req *ToolRequest) string
    NeedsInitialApproval(req *ToolRequest, approvalPolicy ApprovalPolicy, sandboxPolicy SandboxPolicy) bool
    NeedsRetryApproval(approvalPolicy ApprovalPolicy) bool
    SandboxPreference() SandboxPreference
    EscalateOnFailure() bool
    WantsEscalatedFirstAttempt(req *ToolRequest) bool
    SupportsParallel() bool
    SandboxRetryData(req *ToolRequest) *SandboxRetryData
}
```

### Execution Lifecycle

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. Tool Request arrives from AI model                           │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Check Approval Requirements                                  │
│    - NeedsInitialApproval()?                                    │
│    - Check approval cache                                       │
│    - Request approval if needed                                 │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Select Sandbox Configuration                                 │
│    - Check WantsEscalatedFirstAttempt()                        │
│    - Use SandboxPreference() to choose sandbox type            │
│    - Consider system SandboxPolicy                             │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. Execute Tool                                                  │
│    - Create ExecutionContext with SandboxAttempt               │
│    - Call Execute() with streaming support                      │
│    - Collect output and timing                                  │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
                    Success?
                    │      │
              ┌─────┘      └─────┐
              │                   │
              ▼                   ▼
         ┌─────────┐       ┌──────────────────┐
         │ Return  │       │ Sandbox Denied?  │
         │ Response│       └────┬─────────────┘
         └─────────┘            │
                                ▼
                        EscalateOnFailure()?
                                │
                          ┌─────┴─────┐
                          │           │
                      Yes │           │ No
                          │           │
                          ▼           ▼
                    ┌─────────┐  ┌─────────┐
                    │ Request │  │ Return  │
                    │ Retry   │  │ Error   │
                    │ Approval│  └─────────┘
                    └────┬────┘
                         │
                         ▼
                    Approved?
                         │
                   ┌─────┴─────┐
                   │           │
               Yes │           │ No
                   │           │
                   ▼           ▼
            ┌──────────┐  ┌─────────┐
            │ Retry    │  │ Return  │
            │ Without  │  │ Error   │
            │ Sandbox  │  └─────────┘
            └─────┬────┘
                  │
                  ▼
            ┌──────────┐
            │ Return   │
            │ Response │
            └──────────┘
```

## Design Decisions

### 1. Interface vs. Trait Pattern

**Rust (trait-based):**
```rust
pub trait ToolRuntime<Req, Out>: Approvable<Req> + Sandboxable {
    async fn run(&mut self, req: &Req, attempt: &SandboxAttempt, ctx: &ToolCtx)
        -> Result<Out, ToolError>;
}
```

**Go (interface-based):**
```go
type ToolRuntime interface {
    Execute(ctx context.Context, req *ToolRequest, execCtx *ExecutionContext)
        (*ToolResponse, error)
    // ... other methods
}
```

**Rationale**: Go doesn't have trait composition, so we use a single interface with all required methods. This is more explicit and easier to understand at the cost of a larger interface.

### 2. Generic vs. Concrete Types

**Rust**: Uses generics for request/response types:
```rust
pub trait ToolRuntime<Req, Out> { ... }
```

**Go**: Uses concrete types with flexible structure:
```go
type ToolRequest struct {
    Arguments string  // JSON string parsed by each tool
    Metadata map[string]interface{}  // Tool-specific data
}
```

**Rationale**: Go's lack of generics (pre-1.18) and limited support afterward makes concrete types more practical. The `Arguments` field as JSON string + `Metadata` map provides flexibility without generics.

### 3. Context Propagation

**Go-specific design**: We use `context.Context` as the first parameter to `Execute()` following Go conventions for:
- Cancellation
- Deadline/timeout enforcement
- Request-scoped values

This aligns with Go's standard library patterns and makes integration with other Go code natural.

### 4. Streaming Output

**Design**: `ExecutionContext.OutputWriter` provides streaming capability:
```go
type ExecutionContext struct {
    OutputWriter io.Writer  // Nil if streaming not supported
    // ...
}
```

Tools can write to `OutputWriter` during execution for real-time feedback. The `StreamWriter` helper handles formatting as `OutputDelta` events.

### 5. Error Handling

**Structured errors**:
```go
type ToolError struct {
    Kind ToolErrorKind
    Message string
    Cause error
}
```

This provides:
- Categorization (`ErrorRejected`, `ErrorSandboxDenied`, etc.)
- User-friendly messages
- Error wrapping for debugging

### 6. Approval Caching

**Key-based caching**:
```go
ApprovalKey(req *ToolRequest) string
```

Each tool generates a unique key for its approval decisions. Identical operations (same command + cwd) can reuse cached approvals within a session.

### 7. Sandbox Preference

**Three-level preference system**:
```go
const (
    SandboxAuto     // Use if available, escalate on failure
    SandboxRequire  // Must use sandbox
    SandboxForbid   // Cannot use sandbox
)
```

This gives tools control over their isolation requirements while letting the orchestrator handle policy enforcement.

## Comparison with Rust Implementation

| Aspect | Rust | Go |
|--------|------|-----|
| **Interface** | Multiple traits (`ToolRuntime`, `Approvable`, `Sandboxable`) | Single `ToolRuntime` interface |
| **Types** | Generic `<Req, Out>` | Concrete `ToolRequest`, `ToolResponse` |
| **Async** | `async fn` with futures | `context.Context` + goroutines |
| **Error handling** | `Result<T, ToolError>` | `(*ToolResponse, error)` |
| **Streaming** | Custom stream types | `io.Writer` interface |
| **Approval cache** | `HashMap<K, ReviewDecision>` with serde | `sync.Map` with string keys |
| **Mutability** | Explicit `&mut self` | Implicit (pointer receivers) |

## Extension Points

### Adding a New Tool

1. **Implement ToolRuntime interface**:
```go
type MyTool struct {
    // tool state
}

func (t *MyTool) Name() string { return "my_tool" }
func (t *MyTool) Execute(ctx context.Context, req *ToolRequest, execCtx *ExecutionContext) (*ToolResponse, error) {
    // tool logic
}
// ... implement other methods
```

2. **Register with builder**:
```go
builder := NewToolRegistryBuilder()
builder.RegisterTool(&MyTool{}, ToolSpec{
    Name: "my_tool",
    Description: "Does something useful",
    ParametersSchema: mySchema,
})
```

3. **Add to orchestrator** (future work):
The orchestrator will handle the approval → sandbox → execute flow automatically.

### Custom Approval Logic

Override `NeedsInitialApproval()` to implement tool-specific approval rules:

```go
func (t *MyTool) NeedsInitialApproval(req *ToolRequest, approvalPolicy ApprovalPolicy, sandboxPolicy SandboxPolicy) bool {
    // Custom logic, e.g., check if operation is destructive
    if t.isDestructive(req) {
        return true
    }
    // Fall back to default behavior
    return approvalPolicy == ApprovalUnlessTrusted
}
```

### Custom Sandbox Behavior

Override `SandboxPreference()` and `EscalateOnFailure()`:

```go
func (t *MyTool) SandboxPreference() SandboxPreference {
    return SandboxRequire  // Must run in sandbox
}

func (t *MyTool) EscalateOnFailure() bool {
    return false  // Never retry without sandbox
}
```

## Testing Strategy

### Unit Tests

Test individual tool implementations:
```go
func TestShellTool_Execute(t *testing.T) {
    tool := &ShellTool{}
    req := &ToolRequest{
        ToolName: "shell",
        Arguments: `{"command": ["echo", "hello"]}`,
    }
    execCtx := &ExecutionContext{
        SandboxAttempt: &SandboxAttempt{Type: SandboxNone},
    }

    resp, err := tool.Execute(context.Background(), req, execCtx)
    assert.NoError(t, err)
    assert.Contains(t, resp.Content, "hello")
}
```

### Integration Tests

Test full lifecycle including approval and sandboxing:
```go
func TestToolOrchestrator_RunWithApproval(t *testing.T) {
    orchestrator := NewToolOrchestrator()
    tool := &ShellTool{}

    // Mock approval handler
    approvalHandler := &MockApprovalHandler{
        decision: ApprovalApproved,
    }

    result := orchestrator.Run(context.Background(), tool, req, approvalHandler)
    assert.True(t, result.ApprovalRequired)
    assert.NoError(t, result.Error)
}
```

## Future Work

1. **Orchestrator implementation**: The high-level orchestrator that drives the approval → sandbox → execute flow
2. **Concrete tool implementations**: Shell, file operations, patch application
3. **Sandbox integration**: Connect to bubblewrap/docker sandbox managers
4. **Streaming infrastructure**: Full implementation of output delta handling
5. **Metrics and telemetry**: Execution timing, success rates, approval patterns
6. **Tool composition**: Higher-level tools built from primitive tools
7. **Parallel execution**: Safe concurrent execution of parallel-capable tools

## Files

- `runtime.go`: Core interfaces and types
- `types.go`: Supporting types, helpers, and utilities
- `README.md`: This documentation

## Related Packages

- `internal/tools/shell`: Shell command execution tool (pending)
- `internal/tools/file`: File operation tools (pending)
- `internal/tools/patch`: Patch application tool (pending)
- `internal/sandbox`: Sandbox management (pending)
- `internal/protocol`: Protocol types and conversions (pending)
