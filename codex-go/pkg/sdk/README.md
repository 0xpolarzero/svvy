# Codex Go SDK

The Codex Go SDK provides an ergonomic, high-level API for using Codex programmatically in Go applications. It's a facade over the internal packages that handles client configuration, conversation management, tool orchestration, and persistence.

## Features

- **Simple API**: Easy-to-use interface for creating AI-powered coding sessions
- **Streaming Support**: Both streaming and non-streaming response modes
- **Tool Execution**: Built-in support for tool approvals and sandboxing
- **Session Management**: Create and manage multiple concurrent conversation sessions
- **History Tracking**: Automatic conversation history tracking with optional persistence
- **Type-Safe**: Full type safety with comprehensive documentation

## Installation

```bash
go get github.com/evmts/codex/codex-go/pkg/sdk
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/evmts/codex/codex-go/pkg/sdk"
    "github.com/evmts/codex/codex-go/pkg/sdk/client"
)

func main() {
    // 1. Create a client
    c, err := client.New(client.Options{
        BaseURL: "https://api.openai.com/v1",
        APIKey:  "your-api-key",
        Model:   "gpt-4",
    })
    if err != nil {
        log.Fatal(err)
    }

    // 2. Create SDK instance
    codex, err := sdk.New(sdk.Options{
        Client: c,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer codex.Close()

    // 3. Create a session
    ctx := context.Background()
    session, err := codex.NewSession(ctx, sdk.SessionOptions{
        SystemPrompt: "You are a helpful coding assistant.",
    })
    if err != nil {
        log.Fatal(err)
    }

    // 4. Send a message
    response, err := session.Submit(ctx, "Write a hello world function in Go")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(response.Content)
}
```

## Client Creation

The SDK provides three convenient ways to create a client:

### From Direct Options

```go
client, err := client.New(client.Options{
    BaseURL: "https://api.openai.com/v1",
    APIKey:  "your-api-key",
    Model:   "gpt-4",
})
```

### From Environment Variables

```go
// Requires CODEX_API_KEY or OPENAI_API_KEY environment variable
client, err := client.FromEnv()
```

### From Configuration File

```go
client, err := client.FromConfig("/path/to/config.toml")
```

## Session Management

### Creating Sessions

```go
session, err := codex.NewSession(ctx, sdk.SessionOptions{
    SystemPrompt: "You are a helpful assistant.",
    Streaming:    false,  // Set to true for streaming
})
```

### Non-Streaming Requests

```go
response, err := session.Submit(ctx, "Your question here")
if err != nil {
    log.Fatal(err)
}

fmt.Println(response.Content)
fmt.Printf("Tokens used: %d\n", response.TokenUsage.TotalTokens)
```

### Streaming Requests

```go
session, _ := codex.NewSession(ctx, sdk.SessionOptions{
    SystemPrompt: "You are a helpful assistant.",
    Streaming:    true,  // Enable streaming
})

eventCh, err := session.SubmitStream(ctx, "Your question here")
if err != nil {
    log.Fatal(err)
}

for event := range eventCh {
    if event.Error != nil {
        log.Fatal(event.Error)
    }
    if event.Done {
        fmt.Println("Complete!")
        break
    }
    fmt.Print(event.Delta)  // Print incremental text
}
```

## History Management

```go
// Access conversation history
history := session.History()
for _, msg := range history {
    fmt.Printf("[%s]: %s\n", msg.Role, msg.Content)
}
```

## Tool Approval

Configure tool approval callbacks to control when tools can execute:

```go
session, _ := codex.NewSession(ctx, sdk.SessionOptions{
    SystemPrompt: "You are a helpful assistant.",
    OnToolApproval: func(toolName, operation string) bool {
        fmt.Printf("Approve %s? ", toolName)
        // Prompt user or check policy
        return true  // or false to deny
    },
    ApprovalPolicy: "always",  // "auto", "always", or "never"
})
```

## Multiple Sessions

The SDK supports managing multiple concurrent sessions:

```go
// Create multiple sessions
session1, _ := codex.NewSession(ctx, sdk.SessionOptions{
    SystemPrompt: "You are a Python expert.",
})

session2, _ := codex.NewSession(ctx, sdk.SessionOptions{
    SystemPrompt: "You are a Go expert.",
})

// List all active sessions
sessionIDs := codex.ListSessions()

// Get a specific session
session, err := codex.GetSession(sessionID)

// Close a specific session
err = codex.CloseSession(sessionID)
```

## Configuration Options

### SDK Options

```go
sdk.New(sdk.Options{
    Client:        client,           // Required: OpenAI-compatible client
    Tools:         []runtime.Tool,   // Optional: Custom tools
    ToolRegistry:  registry,         // Optional: Pre-configured tool registry
    EnableHistory: true,             // Optional: Enable persistence
    HistoryPath:   "/path/to/file",  // Optional: History file path
})
```

### Session Options

```go
codex.NewSession(ctx, sdk.SessionOptions{
    SystemPrompt:     "You are...",       // System message
    Streaming:        true,               // Enable streaming
    OnToolApproval:   func() bool {...},  // Approval callback
    ApprovalPolicy:   "always",           // Approval policy
    SandboxPolicy:    "read_only",        // Sandbox policy
    WorkingDirectory: "/path",            // Working directory
    Model:            "gpt-4",            // Override model
    ConversationID:   "existing-id",      // Resume conversation
})
```

## Response Types

### Response

```go
type Response struct {
    Content      string      // AI's text response
    ToolCalls    []ToolCall  // Any tools the AI wants to execute
    FinishReason string      // Why the response ended
    TokenUsage   TokenUsage  // Token consumption details
}
```

### StreamEvent

```go
type StreamEvent struct {
    Type     string    // Event type
    Delta    string    // Incremental text
    ToolCall *ToolCall // Incremental tool call info
    Done     bool      // Whether this is the final event
    Error    error     // Any error that occurred
    Response *Response // Final response (only when Done=true)
}
```

### Message

```go
type Message struct {
    Role       string     // "system", "user", "assistant", "tool"
    Content    string     // Message content
    ToolCalls  []ToolCall // Tool invocations (assistant messages)
    ToolCallID string     // Tool call reference (tool messages)
}
```

## Error Handling

All SDK methods return errors that should be checked:

```go
session, err := codex.NewSession(ctx, options)
if err != nil {
    // Handle error (e.g., invalid configuration)
}

response, err := session.Submit(ctx, message)
if err != nil {
    // Handle error (e.g., API error, network issue)
}
```

## Best Practices

1. **Always defer Close()**: Ensure resources are cleaned up
   ```go
   codex, _ := sdk.New(options)
   defer codex.Close()
   ```

2. **Use contexts for cancellation**: Pass contexts to support timeouts and cancellation
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
   defer cancel()
   response, err := session.Submit(ctx, message)
   ```

3. **Handle streaming errors**: Check for errors in stream events
   ```go
   for event := range eventCh {
       if event.Error != nil {
           // Handle error
           break
       }
   }
   ```

4. **Reuse sessions**: Sessions maintain conversation context, so reuse them for multi-turn conversations

5. **Check session state**: Verify session isn't closed before submitting
   ```go
   if session.IsClosed() {
       // Handle closed session
   }
   ```

## Testing

The SDK includes comprehensive test coverage. To run tests:

```bash
go test ./pkg/sdk/...
```

To see example usage:

```bash
go test ./pkg/sdk -v -run Example
```

## Examples

See [example_test.go](./example_test.go) for complete working examples. All examples are runnable with `go test -v -run Example`.

### Basic Usage Examples

- **Example_basicUsage** - Simple question/answer interaction
- **Example_withStreaming** - Streaming responses with real-time feedback
- **Example_withHistory** - Multi-turn conversations with context
- **Example_resumeSession** - Resume conversations from previous sessions

### Configuration Examples

- **Example_customModel** - Using specific models per session
- **Example_approvalPolicies** - Different tool approval modes (auto, always, never)
- **Example_sandboxPolicies** - File system access control (read-only, workspace, full)
- **Example_systemPrompt** - Custom system prompts for specialized assistants

### Tool Usage Examples

- **Example_fileOperations** - File reading and writing operations
- **Example_shellCommands** - Executing shell commands with approval
- **Example_multipleTools** - Chaining multiple tools for complex tasks

### Error Handling Examples

- **Example_errorHandling** - Proper error checking and handling patterns
- **Example_contextCancellation** - Timeout and cancellation handling
- **Example_approvalDenied** - Handling denied tool operations

### Advanced Features Examples

- **Example_multipleSessionsConcurrent** - Running multiple sessions concurrently
- **Example_tokenTracking** - Monitoring token usage and costs
- **Example_customTools** - Extending SDK with custom tools

### Complete Workflow Examples

- **Example_basicWorkflow** - End-to-end basic workflow
- **Example_multipleSessions** - Managing multiple sessions
- **Example_toolApproval** - Tool approval callback setup
- **ExampleNew** - SDK initialization
- **ExampleSDK_NewSession** - Session creation patterns
- **ExampleSession_Submit** - Non-streaming message submission
- **ExampleSession_SubmitStream** - Streaming message submission
- **ExampleSession_History** - Accessing conversation history

## License

See the main project LICENSE file.
