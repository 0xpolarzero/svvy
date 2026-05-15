# Conversation State Package

This package provides comprehensive conversation state tracking with thread-safe operations for the Codex conversation manager.

## Overview

The state package implements:
- **Immutable state updates** with snapshot capability
- **Turn context management** with accumulation
- **Message history management** with filtering and compaction
- **Policy enforcement** with violation tracking
- **Tool call lifecycle tracking** (pending → approved → executed)
- **Token usage tracking** per turn and cumulative
- **Thread-safe operations** throughout

## Test Coverage

**98.1%** statement coverage with comprehensive test suite including:
- Unit tests for all core functionality
- Thread safety tests with concurrent operations
- Serialization/deserialization tests
- Edge case handling
- Integration scenarios

## Architecture

### Core Components

#### 1. ConversationState (`state.go`)
Main state tracking structure supporting:
```go
state := NewConversationState()

// Add messages
state.AddMessage(Message{
    Role:      "user",
    Content:   "Hello",
    Timestamp: time.Now(),
})

// Track tool calls
state.AddToolCall(ToolCall{
    ID:        "call_1",
    Name:      "read_file",
    Arguments: map[string]interface{}{"path": "/test.txt"},
    Status:    ToolCallPending,
})

// Update tool status
state.UpdateToolCallStatus("call_1", ToolCallApproved)

// Track token usage
state.AddTokenUsage(TokenUsage{
    InputTokens:  1000,
    OutputTokens: 500,
    TotalTokens:  1500,
})

// Create immutable snapshot
snapshot := state.Snapshot()
```

**Features:**
- Thread-safe with RWMutex
- Validates all inputs (roles, content, IDs)
- Enforces valid status transitions
- Accumulates token usage across turns
- Provides immutable snapshots for serialization

#### 2. TurnContext (`context.go`)
Manages context for a single conversation turn:
```go
ctx := NewTurnContext("user_123", "What files are in this directory?")

// Add tool results
ctx.AddToolResult(ToolResult{
    CallID:    "call_1",
    Name:      "list_files",
    Output:    []string{"file1.go", "file2.go"},
    Duration:  50 * time.Millisecond,
})

// Add system messages
ctx.AddSystemMessage("Tool execution completed")

// Set metadata
ctx.SetMetadata("model", "claude-3-opus")
ctx.SetMetadata("temperature", 0.7)

// Mark complete
ctx.Complete()
duration := ctx.Duration()
```

**Context History:**
```go
history := NewContextHistory()
history.Add(ctx)

// Query contexts
latest := history.Latest()
recent := history.Since(time.Now().Add(-1 * time.Hour))
last10 := history.Limit(10)
```

#### 3. MessageHistory (`message.go`)
Manages conversation message list:
```go
history := NewMessageHistory()

// Add messages
history.Append(Message{
    Role:      "user",
    Content:   "Hello",
    Timestamp: time.Now(),
})

// Query messages
all := history.All()
last5 := history.GetLastN(5)
userMessages := history.GetByRole("user")
recent := history.GetSince(cutoff)

// Manage size
history.Compact(100) // Keep last 100 messages
history.Clear()      // Remove all
```

**Features:**
- Thread-safe operations
- Role-based filtering
- Time-based querying
- Automatic compaction support
- Preserves message order

#### 4. Policy Enforcement (`policy.go`)
Enforces conversation policies:
```go
policy := NewPolicyWithOptions(PolicyOptions{
    RequireToolApproval:   true,
    MaxTokensPerTurn:      100000,
    MaxMessagesInHistory:  100,
    AllowedTools:          []string{"read_file", "write_file"},
    BlockedTools:          []string{"exec", "delete"},
    DangerousTools:        []string{"exec"},
})

// Validate operations
err := policy.ValidateToolCall("read_file", args)
err = policy.ValidateTokenUsage(usage)
err = policy.ValidateMessageHistory(history)

// Check approval requirements
needsApproval := policy.ShouldApproveToolCall("read_file")

// Custom validators
policy.AddCustomValidator(func(tool string, args map[string]interface{}) error {
    // Custom validation logic
    return nil
})
```

**Policy Enforcer:**
```go
enforcer := NewPolicyEnforcer(policy)

// Enforce with violation tracking
enforcer.EnforceToolCall("exec", args)
enforcer.EnforceTokenUsage(usage)
enforcer.EnforceMessageHistory(history)

// Check violations
violations := enforcer.Violations()
for _, v := range violations {
    fmt.Printf("%s: %s\n", v.Type, v.Message)
}

enforcer.ClearViolations()
```

## Data Structures

### Message
```go
type Message struct {
    Role      string    // "user", "assistant", "system", "tool"
    Content   string    // Message content
    Timestamp time.Time // When message was created
    ID        string    // Optional unique identifier
}
```

**Supported Roles:**
- `user`: Messages from the user
- `assistant`: Messages from the AI assistant
- `system`: System-level instructions or messages
- `tool`: Tool execution results (OpenAI API compatible)

### ToolCall
```go
type ToolCall struct {
    ID        string                 // Unique call ID
    Name      string                 // Tool name
    Arguments map[string]interface{} // Tool arguments
    Status    ToolCallStatus         // pending/approved/executed/rejected
    Result    interface{}            // Execution result
    Error     string                 // Error message if failed
    Timestamp time.Time              // When call was made
}
```

### TokenUsage
```go
type TokenUsage struct {
    InputTokens           int64
    CachedInputTokens     int64
    OutputTokens          int64
    ReasoningOutputTokens int64
    TotalTokens           int64
}
```

## Thread Safety

All types are thread-safe:

- **ConversationState**: Uses `sync.RWMutex` for reads and writes
- **MessageHistory**: Uses `sync.RWMutex` for all operations
- **ContextHistory**: Uses `sync.RWMutex` for concurrent access
- **PolicyEnforcer**: Uses `sync.RWMutex` for violation tracking

Tested with 100+ concurrent goroutines performing reads and writes.

## Design Decisions

### 1. Immutable Snapshots
- `Snapshot()` returns copies to prevent external mutations
- Enables safe serialization and state persistence
- Supports rollback and state comparison

### 2. Status Transition Validation
Tool calls follow strict lifecycle:
```
pending → approved → executed
        ↘ rejected
```
Invalid transitions are rejected with descriptive errors.

### 3. Policy Separation
- Policy definition separate from enforcement
- Supports policy cloning for inheritance
- Custom validators for extensibility

### 4. Context Accumulation
Turn contexts accumulate:
- User input
- Tool results (with timing)
- System messages
- Arbitrary metadata

### 5. History Management
- Automatic ordering preservation
- Efficient compaction without reallocation
- Time-based and count-based querying

## Integration with Manager

This package is designed to integrate with `internal/conversation/manager/`:

```go
// In manager
type Manager struct {
    state    *state.ConversationState
    policy   *state.Policy
    enforcer *state.PolicyEnforcer
    contexts *state.ContextHistory
    messages *state.MessageHistory
}

func (m *Manager) HandleUserInput(input string) error {
    // Create turn context
    ctx := state.NewTurnContext(m.userID, input)

    // Validate policy
    if err := m.enforcer.EnforceMessageHistory(m.messages); err != nil {
        return err
    }

    // Add message
    msg := state.Message{
        Role:      "user",
        Content:   input,
        Timestamp: time.Now(),
    }
    m.messages.Append(msg)
    m.state.AddMessage(msg)

    // Store context
    m.contexts.Add(ctx)

    return nil
}
```

## Testing

Run tests:
```bash
go test ./internal/conversation/state/...
```

Run with coverage:
```bash
go test -cover ./internal/conversation/state/...
```

Generate coverage report:
```bash
go test -coverprofile=coverage.out ./internal/conversation/state/...
go tool cover -html=coverage.out
```

## Performance

- **Message operations**: O(1) append, O(n) filtering
- **Tool call lookups**: O(1) by ID using map
- **Token aggregation**: O(n) where n = number of turns
- **Snapshot creation**: O(n+m) where n=messages, m=tool calls
- **Lock contention**: Minimized with RWMutex (multiple concurrent readers)

## Future Enhancements

Potential additions:
- State persistence to disk/database
- Event-based state notifications
- State compression for long histories
- Query optimization with indexing
- Metrics and observability hooks

## References

- Rust implementation: `/Users/williamcory/codex/codex-rs/core/src/state/`
- Protocol types: `internal/protocol/protocol.go`
- Test helpers: `test/testhelpers.go`
