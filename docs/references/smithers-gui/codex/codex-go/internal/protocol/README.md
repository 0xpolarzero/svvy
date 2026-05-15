# Protocol Package

The `protocol` package defines the core communication protocol for Codex sessions between a client and an agent.

## Overview

This package implements a **Submission Queue (SQ) / Event Queue (EQ)** pattern for asynchronous communication:

- **Submissions (SQ)**: Requests from the user to the agent, wrapped in `Submission` structures containing `Op` operations
- **Events (EQ)**: Responses from the agent to the client, wrapped in `Event` structures containing `EventMsg` messages

## Architecture

### Submission Operations (Op)

User requests are sent as operations that implement the `Op` interface:

- `OpInterrupt` - Abort the current task
- `OpUserInput` - Send user input to the agent
- `OpUserTurn` - Send a complete turn with context (cwd, policies, model, etc.)
- `OpExecApproval` - Approve/deny a command execution request
- `OpPatchApproval` - Approve/deny a code patch request
- `OpOverrideTurnContext` - Update turn context settings
- `OpAddToHistory` - Add an entry to persistent history
- `OpGetHistoryEntryRequest` - Request a specific history entry
- `OpGetPath` - Request the conversation transcript path
- `OpListMcpTools` - Request available MCP tools
- `OpListCustomPrompts` - Request available custom prompts
- `OpCompact` - Request conversation summarization
- `OpReview` - Request a code review
- `OpShutdown` - Shut down the Codex instance

### Event Messages (EventMsg)

Agent responses are sent as events that implement the `EventMsg` interface:

- `EventError` - Error during submission execution
- `EventTaskStarted` - Task has begun
- `EventTaskComplete` - Task has completed
- `EventTokenCount` - Token usage update
- `EventAgentMessage` - Text output from the agent
- `EventUserMessage` - User/system input message
- `EventAgentMessageDelta` - Streaming text delta
- `EventAgentReasoning` - Reasoning output
- `EventAgentReasoningDelta` - Streaming reasoning delta
- `EventExecCommandBegin` - Command execution starting
- `EventExecCommandOutputDelta` - Streaming command output
- `EventExecCommandEnd` - Command execution completed
- `EventShutdownComplete` - Shutdown completed

## JSON Serialization

All types use JSON with discriminated unions via the `type` field:

```json
{
  "type": "user_turn",
  "items": [{"type": "text", "text": "Hello"}],
  "cwd": "/path",
  "approval_policy": "on-request",
  "sandbox_policy": {"mode": "read-only"},
  "model": "claude-3-5-sonnet-20241022",
  "summary": "auto"
}
```

The JSON format is compatible with the Rust implementation in `codex-rs`.

## Key Types

### Submission & Event Wrappers

```go
type Submission struct {
    ID string `json:"id"`  // Correlation ID
    Op Op     `json:"op"`  // The operation
}

type Event struct {
    ID  string   `json:"id"`  // Correlation ID
    Msg EventMsg `json:"msg"` // The event message
}
```

### Sandbox Policies

Control command execution restrictions:

- `read-only` - Read-only filesystem access
- `workspace-write` - Write access to workspace only
- `danger-full-access` - Unrestricted access

### Approval Policies

Control when to ask for user approval:

- `untrusted` - Approve only trusted read-only commands
- `on-failure` - Auto-approve in sandbox, escalate on failure
- `on-request` - Model decides when to ask
- `never` - Never ask for approval

### Token Usage

Track token consumption with detailed breakdowns:

```go
type TokenUsage struct {
    InputTokens           int64
    CachedInputTokens     int64
    OutputTokens          int64
    ReasoningOutputTokens int64
    TotalTokens           int64
}
```

Helper methods:
- `CachedInput()` - Number of cached input tokens
- `NonCachedInput()` - Non-cached input tokens
- `BlendedTotal()` - Display total (non-cached input + output)
- `TokensInContextWindow()` - Tokens currently in context

## Testing

The package includes comprehensive tests:

- **Table-driven tests** for all Op and EventMsg types
- **Golden file tests** using fixtures in `test/testdata/fixtures/protocol/`
- **Round-trip tests** ensuring serialization/deserialization consistency
- **Helper function tests** for token calculations
- **Benchmarks** for performance monitoring

Run tests:
```bash
go test ./internal/protocol/...
go test ./internal/protocol/... -bench=.
go test ./internal/protocol/... -cover
```

## Design Decisions

### Interface-based Design

Uses Go interfaces (`Op`, `EventMsg`) with concrete implementations for type safety and extensibility.

### Custom JSON Marshaling

Each type implements `MarshalJSON()` to ensure exact JSON format compatibility with Rust.

### Pointer Fields for Optionals

Optional fields use pointers (`*string`, `*int64`) to distinguish between absent and zero values.

### Tagged Unions

The `type` field discriminates between variants, matching Rust's `#[serde(tag = "type")]`.

### Helper Methods

Utility methods on types (e.g., `TokenUsage` calculations) encapsulate business logic.

## Compatibility

This implementation maintains JSON-level compatibility with `codex-rs/protocol/src/protocol.rs`, ensuring seamless interoperability between Go and Rust implementations.
