# SDK Submit() Implementation Notes

## Date: 2025-10-26

## Summary

Implemented actual AI interaction in the `Submit()` method in `/Users/williamcory/codex/codex-go/pkg/sdk/session.go`, replacing the previous placeholder implementation that returned hardcoded responses.

## Changes Made

### 1. Core Files Modified

#### `/Users/williamcory/codex/codex-go/pkg/sdk/session.go`
- **Updated `Submit()` method** (lines 156-194):
  - Added message validation (empty check)
  - Calls new `submitInternal()` method to interact with manager
  - Properly adds user and assistant messages to history
  - Returns real AI responses with actual token counts

#### `/Users/williamcory/codex/codex-go/pkg/sdk/session_impl.go` (NEW FILE)
- **`submitInternal()` method**:
  - Creates `protocol.OpUserTurn` with user message
  - Gets or creates manager session with event handler
  - Submits operation via `manager.SubmitOp()`
  - Waits for completion with proper timeout (5 minutes)
  - Handles context cancellation
  - Returns `Response` with real data

- **`getOrCreateManagerSession()` method**:
  - Tries to get existing manager session
  - Creates new session if doesn't exist
  - Configures session with:
    - Working directory
    - Approval policy (defaults to "auto")
    - Sandbox policy (defaults to "native")
    - Model (defaults to "claude-sonnet-4-5")
    - Event handler for collecting responses
    - Orchestrator for tool execution

- **`responseCollector` type**:
  - Thread-safe event collection
  - Accumulates content from `EventAgentMessageDelta` events
  - Extracts token usage from `EventTokenCount` events
  - Detects completion via `EventTaskComplete` event
  - Handles errors via `EventError` event
  - Uses channels for synchronization

## How It Works

### Request Flow

1. **User calls `Submit(ctx, message)`**
   ```go
   response, err := session.Submit(ctx, "Write a hello world function")
   ```

2. **Validation and History**
   - Validates message is not empty
   - Adds user message to session history

3. **Manager Interaction**
   - `submitInternal()` creates `OpUserTurn` with:
     - Message text as `UserInput` item
     - Working directory (from session config or cwd)
     - Approval policy (from session config or "auto")
     - Sandbox policy (from session config or "native")
     - Model name (from session config or "claude-sonnet-4-5")

4. **Session Creation/Retrieval**
   - First checks if manager session exists
   - If not, creates new manager session with:
     - Event handler for response collection
     - Turn context with session configuration
     - Client and orchestrator references

5. **Operation Submission**
   - Calls `manager.SubmitOp(ctx, sessionID, op)`
   - Manager processes turn asynchronously in background goroutine
   - Manager emits protocol events as turn processes

6. **Event Collection**
   - `responseCollector` handles events via registered event handler
   - Accumulates content from delta events
   - Tracks token usage from token count events
   - Signals completion when task complete event received
   - Captures errors if error event received

7. **Response Return**
   - Waits for collector to signal completion (via channel)
   - Returns `Response` with:
     - Complete content text
     - Finish reason ("stop", "length", etc.)
     - Actual token usage (input, output, total)

8. **History Update**
   - Adds assistant message to session history
   - Returns response to caller

### Event Processing

The `responseCollector` processes these protocol events:

- **`EventAgentMessageDelta`**: Accumulates text content
  ```go
  collector.content += msg.Delta
  ```

- **`EventTokenCount`**: Tracks token usage
  ```go
  collector.tokenUsage = TokenUsage{
      InputTokens:  msg.Info.TotalTokenUsage.InputTokens,
      OutputTokens: msg.Info.TotalTokenUsage.OutputTokens,
      TotalTokens:  msg.Info.TotalTokenUsage.TotalTokens,
  }
  ```

- **`EventTaskComplete`**: Signals turn completion
  ```go
  collector.finishReason = "stop"
  collector.completed = true
  close(collector.done)
  ```

- **`EventError`**: Captures errors
  ```go
  collector.errors <- fmt.Errorf("turn processing error: %s", msg.Message)
  ```

### Context Cancellation

Context cancellation is properly handled at multiple levels:

1. **In `submitInternal()`**:
   ```go
   select {
   case <-collector.done:
       return collector.getResponse(), nil
   case <-ctx.Done():
       return nil, ctx.Err()
   }
   ```

2. **In manager's turn processing**: Manager checks context throughout turn execution

3. **In event handler**: Events stop being processed when context is cancelled

### Timeout Handling

- Default timeout: 5 minutes per turn
- Can be cancelled earlier via context
- Timeout returns descriptive error:
  ```go
  "operation timeout after 5 minutes"
  ```

## Configuration Defaults

When session options are not provided, these defaults are used:

- **Model**: `claude-sonnet-4-5`
- **Approval Policy**: `auto` (auto-approve tool executions)
- **Sandbox Policy**: `native` (no sandboxing)
- **Working Directory**: Current working directory

## Error Handling

The implementation handles these error scenarios:

1. **Empty message**: Returns error before submitting
2. **Session closed**: Checked at start of `Submit()`
3. **Manager session creation failure**: Proper error propagation
4. **Operation submission failure**: Returns wrapped error
5. **Turn processing errors**: Collected from `EventError`
6. **Context cancellation**: Returns `ctx.Err()`
7. **Timeout**: Returns timeout error after 5 minutes

## Thread Safety

- **Session mutex**: Protects session state (closed flag, etc.)
- **Collector mutex**: Protects response accumulation
- **Manager sessions**: Thread-safe via manager's internal synchronization
- **Event handlers**: Called sequentially by manager

## Integration with Manager

The implementation properly integrates with the conversation manager:

### Manager's Role
- Accepts `OpUserTurn` via `SubmitOp()`
- Creates background goroutine for turn processing
- Builds completion request with conversation history
- Streams response from AI model
- Executes tools if requested by model
- Emits protocol events throughout process
- Handles multi-turn tool execution automatically

### SDK's Role
- Creates user-friendly `Submit()` API
- Manages session lifecycle
- Converts user messages to `OpUserTurn`
- Collects events into simple `Response` struct
- Maintains local message history
- Provides synchronous (blocking) interface

## Testing Considerations

To properly test this implementation:

1. **Unit Tests**:
   - Test response collection from various event sequences
   - Test error handling paths
   - Test context cancellation
   - Test timeout behavior

2. **Integration Tests**:
   - Test actual AI interaction (requires API key)
   - Test tool execution flow
   - Test multi-turn conversations
   - Test concurrent submissions

3. **Mock Tests**:
   - Mock manager.SubmitOp() to test event handling
   - Mock event emissions to test collector
   - Test with various event orderings

## Known Limitations

1. **No streaming support yet**: `SubmitStream()` still uses placeholder
2. **Event handler management**: Event handlers are appended but not properly removed
3. **Manager dependency**: Requires pre-existing manager compilation issues to be fixed
4. **History reconstruction**: Not implemented for session resume
5. **Tool call details**: Not captured in SDK's `Message` history

## Future Enhancements

1. **Implement `SubmitStream()`**: Use same pattern but forward events in real-time
2. **Tool call tracking**: Capture tool executions in message history
3. **Approval callbacks**: Wire up `onToolApproval` callback
4. **Session resume**: Support resuming from persisted history
5. **Progress callbacks**: Allow streaming progress updates
6. **Better error types**: Define specific error types for different failures

## Compliance with Requirements

✅ **Creates protocol.UserTurn with user message**
   - Done in `submitInternal()` lines 42-55

✅ **Calls manager.SubmitOp() with session context**
   - Done in `submitInternal()` line 72

✅ **Waits for completion via manager events**
   - Done via `responseCollector` and channel synchronization

✅ **Extracts assistant message from conversation state**
   - Done via `EventTaskComplete` and `EventAgentMessageDelta`

✅ **Populates Response with real data**
   - Content: from event deltas
   - Tokens: from `EventTokenCount`
   - Finish reason: set to "stop" on completion

✅ **Handles context cancellation**
   - Done in `submitInternal()` select statement

✅ **Handles manager errors**
   - Collected via `EventError` and error channel

✅ **Handles turn processing failures**
   - Errors propagated from manager events

✅ **Implements token usage tracking**
   - Real token counts from `EventTokenCount`
   - Tracks input/output/total tokens

## References

- Original issue: Lines 173-175 in session.go returned hardcoded placeholder
- Review docs:
  - `/Users/williamcory/codex/codex-go/pkg/sdk/session.go.md`
  - `/Users/williamcory/codex/codex-go/pkg/sdk/sdk.go.md`
- Manager implementation: `/Users/williamcory/codex/codex-go/internal/conversation/manager/manager.go`
- Turn processor: `/Users/williamcory/codex/codex-go/internal/conversation/manager/turn.go`
