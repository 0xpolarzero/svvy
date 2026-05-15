# Conversation Manager

The conversation manager package provides the core orchestration layer for AI-assisted coding sessions in Codex. It manages session lifecycle, turn processing, state transitions, and event handling.

## Architecture

### Components

1. **ConversationManager** (`manager.go`)
   - Main interface for managing multiple conversation sessions
   - Handles session creation, retrieval, listing, and closure
   - Routes operations to appropriate sessions
   - Thread-safe session management

2. **Session** (`session.go`)
   - Represents a single conversation session with an AI agent
   - Manages session state and lifecycle
   - Handles turn submission, interrupts, and approvals
   - Tracks token usage and agent messages
   - Emits events to registered handlers

3. **StateMachine** (`state.go`)
   - Implements the session state machine
   - Enforces valid state transitions
   - Thread-safe state management
   - States: Idle, ProcessingTurn, AwaitingApproval, Interrupted, Completed, Error, Closed

4. **TurnProcessor** (`turn.go`)
   - Processes user turns and streams responses
   - Builds completion requests for the AI client
   - Handles streaming events and emits protocol events
   - Manages approval workflows

## State Machine

The state machine enforces a strict conversation flow:

```
                     ┌─────────────┐
                     │    Idle     │
                     └──────┬──────┘
                            │ SubmitTurn
                            ▼
                   ┌─────────────────┐
          ┌────────│ ProcessingTurn  │◄────────┐
          │        └────────┬─────────┘         │
          │                 │                   │
          │         ┌───────┼───────┐          │
          │         │       │       │          │
   Interrupt   AwaitApproval │   Complete   Approve
          │         │        │       │          │
          │         ▼        │       ▼          │
          │   ┌──────────┐  │  ┌─────────┐     │
          │   │ Awaiting │──┘  │Complete │     │
          │   │Approval  │─────│         │     │
          │   └──────────┘     └─────────┘     │
          │         │                           │
          │         │ Reject                    │
          │         │                           │
          ▼         ▼                           │
     ┌──────────────────┐                      │
     │   Interrupted    │──────────────────────┘
     └───────┬──────────┘
             │
             │ ResetToIdle
             ▼
        ┌─────────┐
        │  Idle   │
        └─────────┘

        Any State
             │ Close
             ▼
        ┌─────────┐
        │ Closed  │ (Terminal)
        └─────────┘
```

### Valid State Transitions

- **From Idle**: ProcessingTurn, Closed
- **From ProcessingTurn**: AwaitingApproval, Completed, Error, Interrupted, Closed
- **From AwaitingApproval**: ProcessingTurn, Completed, Error, Interrupted, Closed
- **From Interrupted**: Idle, ProcessingTurn, Closed
- **From Completed**: Idle, ProcessingTurn, Closed
- **From Error**: Idle, ProcessingTurn, Closed
- **From Closed**: None (terminal state)

## Key Features

### Session Management

- **Create**: Initialize new conversation sessions with configuration
- **Resume**: (Future) Load sessions from history persistence
- **List**: Get all active session IDs
- **Close**: Gracefully shut down sessions

### Turn Processing

- **Submit Turn**: Process user input through the AI model
- **Stream Events**: Real-time event emission during turn processing
- **State Tracking**: Automatic state transitions based on turn lifecycle
- **Error Handling**: Graceful error recovery and reporting

### Approval Workflow

- **Exec Approval**: User approval for command execution
- **Patch Approval**: User approval for code patches
- **Auto-Approve**: Configurable approval policies (auto, manual, semi-auto)
- **Approval Tracking**: Pending approval state management

### Event Handling

The manager emits protocol events during turn processing:

- `EventTaskStarted`: Turn processing begins
- `EventAgentMessageDelta`: Incremental agent responses
- `EventAgentReasoningDelta`: Incremental reasoning output
- `EventTokenCount`: Token usage updates
- `EventTaskComplete`: Turn processing completes
- `EventError`: Error occurred during processing

## Usage Example

```go
// Create manager
client := anthropic.NewClient(cfg)
mgr, err := manager.NewManager(manager.ManagerConfig{
    Client: client,
})

// Create a session
session, err := mgr.CreateSession(ctx, manager.SessionConfig{
    ID: "session-123",
    TurnContext: &manager.TurnContext{
        Cwd:            "/workspace",
        ApprovalPolicy: "auto",
        Model:          "claude-sonnet-4",
        Summary:        "off",
    },
    EventHandlers: []manager.EventHandler{
        func(ctx context.Context, event *protocol.Event) error {
            // Handle events (e.g., update UI, log, etc.)
            return nil
        },
    },
})

// Submit a user turn
op := &protocol.OpUserTurn{
    Items: []protocol.UserInput{
        {Type: "text", Text: strPtr("Implement a new feature")},
    },
    Cwd:            "/workspace",
    ApprovalPolicy: "auto",
    SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
    Model:          "claude-sonnet-4",
    Summary:        "off",
}

err = mgr.SubmitOp(ctx, "session-123", op)

// Submit an interrupt
err = mgr.SubmitOp(ctx, "session-123", &protocol.OpInterrupt{})

// Close session when done
err = mgr.CloseSession("session-123")

// Close manager
err = mgr.Close()
```

## Thread Safety

All components are designed to be thread-safe:

- **Manager**: RWMutex protects session map
- **Session**: RWMutex protects mutable state
- **StateMachine**: RWMutex protects state transitions

Concurrent operations on different sessions are fully parallel. Operations on the same session are serialized.

## Test Coverage

The package has comprehensive test coverage: **80.6%**

### Test Files

- `state_test.go`: State machine and transition tests (100% coverage)
- `session_test.go`: Session lifecycle and operation tests
- `manager_test.go`: Manager coordination and routing tests

### Key Test Scenarios

- Session creation, lifecycle, and closure
- State transitions (valid and invalid)
- Turn submission and processing
- Interrupt handling
- Approval workflows (exec and patch)
- Event emission and handling
- Thread safety and concurrent operations
- Error handling and recovery

## Integration Points

### Client Interface

The manager integrates with the `client.Client` interface for AI model communication:

```go
type Client interface {
    Stream(ctx context.Context, req *ChatCompletionRequest) (<-chan StreamEvent, error)
    Complete(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error)
    GetModelContextWindow() int64
    GetAutoCompactTokenLimit() int64
}
```

### Protocol Types

Uses protocol types from `internal/protocol`:

- **Operations**: OpUserTurn, OpInterrupt, OpExecApproval, OpPatchApproval, etc.
- **Events**: EventTaskStarted, EventAgentMessage, EventTokenCount, etc.

### History Persistence (Future)

The manager is designed to integrate with a history persistence layer (not yet implemented):

- `HistoryStore` interface for saving/loading conversation history
- `ResumeSession` for restoring sessions from storage
- History compaction and context management

## Design Patterns

### State Machine Pattern

Explicit state machine with validated transitions ensures conversation flow integrity.

### Event-Driven Architecture

Event handlers decouple the manager from UI/logging concerns, enabling flexible integration.

### Strategy Pattern

ApprovalChecker encapsulates approval policy logic, making it easy to add new policies.

### Builder Pattern

SessionConfig and ManagerConfig use builder pattern for flexible initialization.

## Future Enhancements

1. **History Persistence**
   - Save conversation history to disk/database
   - Resume sessions across application restarts
   - History compaction and pruning

2. **Context Management**
   - Automatic context window monitoring
   - Smart history compaction when approaching limits
   - Context caching optimization

3. **Advanced Approval Policies**
   - Pattern-based approval rules
   - Risk assessment for operations
   - Learning from user approval patterns

4. **Metrics and Observability**
   - Performance metrics (latency, throughput)
   - State transition tracking
   - Token usage analytics

5. **Multi-Model Support**
   - Session-level model switching
   - Model capability detection
   - Fallback model strategies

## Performance Characteristics

- **Session Operations**: O(1) with RWMutex locking
- **State Transitions**: O(1) map lookup
- **Event Emission**: O(n) where n = number of handlers
- **Memory**: ~1KB per session baseline + conversation history

## Error Handling

The manager uses Go error wrapping for context:

```go
fmt.Errorf("failed to submit turn: %w", err)
```

Errors are propagated up the call stack with context, enabling:
- Precise error reporting
- Easy debugging
- Structured logging integration

## Dependencies

- `internal/client`: AI model client interface
- `internal/protocol`: Protocol types (Op, Event, etc.)
- `go.uber.org/mock`: Test mocking framework
- `github.com/stretchr/testify`: Test assertions
