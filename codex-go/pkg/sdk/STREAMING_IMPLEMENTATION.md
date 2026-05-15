# Streaming Implementation Details

## Overview

This document describes the implementation of streaming AI interactions in the Codex SDK's `SubmitStream()` method.

## Architecture

### High-Level Flow

```
User Call          SDK Session              Manager Session           AI Model
    |                    |                          |                     |
    |--SubmitStream()--->|                          |                     |
    |                    |--buildUserTurnOp()------>|                     |
    |                    |                          |                     |
    |                    |--CreateSession()-------->|                     |
    |                    |                          |                     |
    |                    |--SubmitOp()------------->|                     |
    |                    |                          |--Stream()---------->|
    |                    |                          |                     |
    |                    |<---EventAgentMessageDelta-|<---content chunk---|
    |<--content_delta----|                          |                     |
    |                    |                          |                     |
    |                    |<---EventTokenCount-------|                     |
    |                    |                          |                     |
    |                    |<---EventTaskComplete-----|                     |
    |<--done-------------|                          |                     |
    |                    |                          |                     |
    |<--channel closed---|                          |                     |
```

## Implementation Components

### 1. Session Structure (`session.go`)

The `Session` struct maintains:
- SDK reference for accessing the manager
- Session configuration (streaming mode, approval policy, etc.)
- Conversation history (messages array)
- Manager session reference (created lazily)

### 2. Submit Methods

#### `Submit()` - Non-Streaming

```go
func (s *Session) Submit(ctx context.Context, message string) (*Response, error)
```

- Validates session is not in streaming mode
- Calls `submitInternal()` to get event channel
- Collects all events into a single Response
- Returns complete response when done

#### `SubmitStream()` - Streaming

```go
func (s *Session) SubmitStream(ctx context.Context, message string) (<-chan StreamEvent, error)
```

- Validates session is in streaming mode
- Calls `submitInternal()` to get event channel
- Returns channel immediately for real-time event consumption

### 3. Internal Submission (`session_streaming.go`)

#### `submitInternal()`

Core implementation that:

1. **Adds user message to history**
   ```go
   userMsg := &Message{Role: "user", Content: message}
   s.addMessage(userMsg)
   ```

2. **Builds protocol operation**
   ```go
   op := s.buildUserTurnOp(message)
   ```
   - Creates `protocol.OpUserTurn` with user input
   - Sets working directory, approval policy, sandbox policy
   - Includes system prompt if configured

3. **Creates temporary manager session**
   ```go
   tempSession, err := s.sdk.manager.CreateSession(ctx, manager.SessionConfig{
       ID: tempSessionID,
       Client: s.sdk.client.Internal(),
       TurnContext: turnCtx,
       EventHandlers: []manager.EventHandler{eventHandler},
       Orchestrator: s.sdk.orchestrator,
   })
   ```
   - Uses unique session ID for each turn
   - Registers event handler to capture protocol events
   - Cleans up when done

4. **Submits operation to manager**
   ```go
   err = s.sdk.manager.SubmitOp(ctx, tempSessionID, op.Op)
   ```
   - Manager processes turn asynchronously
   - Emits events via registered handler

5. **Processes events and forwards to channel**
   ```go
   for event := range handlerEventCh {
       s.processEvent(event, collector, eventCh, ctx)
   }
   ```

#### `processEvent()`

Transforms protocol events into SDK StreamEvents:

| Protocol Event | SDK Event Type | Description |
|----------------|----------------|-------------|
| `EventAgentMessageDelta` | `content_delta` | Incremental content |
| `EventAgentReasoningDelta` | `reasoning_delta` | AI reasoning process |
| `EventTokenCount` | (internal) | Updates token usage |
| `EventTaskComplete` | `done` | Final response |
| `EventError` | `error` | Error occurred |
| `EventExecCommandBegin` | `tool_call_delta` | Tool execution started |
| `EventExecCommandEnd` | `tool_call_delta` | Tool execution completed |

### 4. Event Collector

Accumulates data across events:

```go
type eventCollector struct {
    content      string       // Accumulated content
    tokenUsage   TokenUsage   // Token consumption
    finishReason string       // Why response ended
    done         bool         // Completion flag
    err          error        // Error if any
    mu           sync.Mutex   // Thread safety
}
```

## Event Flow

### Content Streaming

1. AI generates token
2. Manager receives from client
3. Emits `EventAgentMessageDelta`
4. SDK converts to `StreamEvent{Type: "content_delta"}`
5. Forwards to caller's channel
6. Accumulates in collector

### Completion

1. AI finishes generation
2. Manager emits `EventTaskComplete`
3. SDK:
   - Marks collector as done
   - Adds assistant message to history
   - Creates final Response
   - Sends `StreamEvent{Type: "done", Response: ...}`
   - Closes channel

### Error Handling

1. Error occurs (model, network, context cancelled)
2. Manager emits `EventError` OR context cancelled
3. SDK:
   - Marks collector as done with error
   - Sends `StreamEvent{Type: "error", Error: ...}`
   - Closes channel

## Thread Safety

- **Session state**: Protected by `sync.RWMutex`
- **Event collector**: Protected by `sync.Mutex`
- **Channel operations**: Non-blocking with context cancellation
- **Manager session**: Uses reference counting for safe cleanup

## Resource Management

### Session Lifecycle

```go
// Create temporary session for each turn
tempSession, err := manager.CreateSession(...)

// Clean up when done
defer manager.CloseSession(tempSessionID)
```

### Channel Lifecycle

```go
// Create channel
eventCh := make(chan StreamEvent, 10)

// Always close when done
defer close(eventCh)

// Handle context cancellation
select {
case eventCh <- event:
case <-ctx.Done():
    return
}
```

### Goroutine Management

- Submission processing runs in goroutine
- Event processing runs in nested goroutine
- Both respect context cancellation
- Proper cleanup via defer statements

## Context Cancellation

Context cancellation is handled at multiple levels:

1. **Caller cancels**: Context passed to `SubmitStream()`
2. **Event processor checks**: Before sending each event
3. **Manager respects**: Session context cancellation
4. **Channel cleanup**: Closed on cancellation

## Configuration

### Session Options

```go
SessionOptions{
    SystemPrompt:     "...",           // Prepended to each message
    Streaming:        true/false,      // Enable streaming mode
    Model:            "claude-sonnet-4-5",
    ApprovalPolicy:   "auto/manual",   // Tool approval
    WorkingDirectory: "/path",         // For tool execution
    SandboxPolicy:    "native/...",    // Execution restrictions
}
```

### Turn Context

Automatically constructed from session options:

```go
TurnContext{
    Cwd:            workingDirectory,
    ApprovalPolicy: approvalPolicy,
    SandboxPolicy:  sandboxPolicy,
    Model:          model,
    MaxTurns:       10,  // Prevents infinite loops
}
```

## Testing

Comprehensive tests in `session_streaming_test.go`:

- `TestSession_SubmitStream_ContentDeltas`: Verifies delta events
- `TestSession_SubmitStream_ReasoningDeltas`: Tests reasoning output
- `TestSession_SubmitStream_ToolCallDeltas`: Tests tool execution events
- `TestSession_SubmitStream_Completion`: Verifies completion event
- `TestSession_SubmitStream_ContextCancellation`: Tests cancellation
- `TestSession_SubmitStream_ErrorHandling`: Tests error scenarios
- `TestSession_SubmitStream_MultipleMessages`: Tests conversation flow
- `TestSession_Submit_UsesStreaming`: Verifies Submit uses streaming internally
- `TestSession_SubmitStream_ConcurrentStreams`: Tests concurrent usage

## Performance Considerations

### Memory

- Channel buffer size: 10 events
- Event collector: O(content size)
- History: O(messages)
- Temporary session: Created per turn, cleaned up after

### Latency

- First token: Depends on model and prompt
- Subsequent tokens: Streamed immediately
- Network: Minimal buffering
- Channel: Non-blocking sends with context

### Throughput

- Concurrent sessions: Supported (each has own manager session)
- Event processing: Asynchronous goroutines
- No artificial rate limiting

## Future Improvements

1. **Event Subscription**: Proper subscription mechanism instead of temporary sessions
2. **Session Pooling**: Reuse manager sessions across turns
3. **Event Batching**: Batch small deltas for efficiency
4. **Metrics**: Add instrumentation for monitoring
5. **Backpressure**: Handle slow consumers gracefully
6. **Resume**: Support resuming interrupted streams

## Related Files

- `/Users/williamcory/codex/codex-go/pkg/sdk/session.go` - Session structure and Submit()
- `/Users/williamcory/codex/codex-go/pkg/sdk/session_streaming.go` - Streaming implementation
- `/Users/williamcory/codex/codex-go/pkg/sdk/session_streaming_test.go` - Tests
- `/Users/williamcory/codex/codex-go/pkg/sdk/STREAMING.md` - User documentation
- `/Users/williamcory/codex/codex-go/examples/streaming/main.go` - Example usage
- `/Users/williamcory/codex/codex-go/internal/conversation/manager/` - Manager implementation
- `/Users/williamcory/codex/codex-go/internal/protocol/protocol.go` - Protocol definitions
