# Streaming AI Interactions in Codex SDK

This document explains how to use the streaming functionality in the Codex SDK to get real-time responses from AI models.

## Overview

The Codex SDK supports two modes of interaction:

1. **Non-Streaming (`Submit`)**: Waits for the complete response before returning
2. **Streaming (`SubmitStream`)**: Returns events in real-time as the AI generates its response

## Non-Streaming Mode

Use non-streaming mode when you want to wait for the complete response:

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
    // Create client
    apiClient, err := client.NewAnthropicClient("your-api-key")
    if err != nil {
        log.Fatal(err)
    }

    // Create SDK
    codex, err := sdk.New(sdk.Options{
        Client: apiClient,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer codex.Close()

    // Create non-streaming session
    session, err := codex.NewSession(context.Background(), sdk.SessionOptions{
        SystemPrompt: "You are a helpful coding assistant.",
        Streaming:    false,
        Model:        "claude-sonnet-4-5",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Submit message and wait for complete response
    response, err := session.Submit(context.Background(), "Write a hello world program in Go")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Response: %s\n", response.Content)
    fmt.Printf("Tokens used: %d\n", response.TokenUsage.TotalTokens)
}
```

## Streaming Mode

Use streaming mode when you want real-time updates as the AI generates its response:

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
    // Create client
    apiClient, err := client.NewAnthropicClient("your-api-key")
    if err != nil {
        log.Fatal(err)
    }

    // Create SDK
    codex, err := sdk.New(sdk.Options{
        Client: apiClient,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer codex.Close()

    // Create streaming session
    session, err := codex.NewSession(context.Background(), sdk.SessionOptions{
        SystemPrompt: "You are a helpful coding assistant.",
        Streaming:    true,
        Model:        "claude-sonnet-4-5",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Submit message and get streaming channel
    eventCh, err := session.SubmitStream(context.Background(), "Write a hello world program in Go")
    if err != nil {
        log.Fatal(err)
    }

    // Process streaming events
    var contentBuilder string
    for event := range eventCh {
        // Check for errors
        if event.Error != nil {
            log.Printf("Stream error: %v", event.Error)
            continue
        }

        // Handle different event types
        switch event.Type {
        case "content_delta":
            // Incremental content update
            fmt.Print(event.Delta)
            contentBuilder += event.Delta

        case "reasoning_delta":
            // AI's reasoning process (if available)
            log.Printf("[Thinking] %s", event.Delta)

        case "tool_call_delta":
            // Tool execution updates
            if event.ToolCall != nil {
                log.Printf("[Tool] %s: %s", event.ToolCall.Name, event.ToolCall.ID)
            }

        case "done":
            // Final response with complete information
            if event.Response != nil {
                fmt.Printf("\n\n---\n")
                fmt.Printf("Finish Reason: %s\n", event.Response.FinishReason)
                fmt.Printf("Tokens Used: %d input + %d output = %d total\n",
                    event.Response.TokenUsage.InputTokens,
                    event.Response.TokenUsage.OutputTokens,
                    event.Response.TokenUsage.TotalTokens)
            }
        }
    }
}
```

## Event Types

The streaming API emits different types of events:

### Content Delta Events

```go
case "content_delta":
    // event.Delta contains incremental text
    fmt.Print(event.Delta)
```

### Reasoning Delta Events

Available for models that support extended thinking:

```go
case "reasoning_delta":
    // event.Delta contains reasoning process
    log.Printf("Thinking: %s", event.Delta)
```

### Tool Call Delta Events

When the AI uses tools (file operations, shell commands, etc.):

```go
case "tool_call_delta":
    if event.ToolCall != nil {
        fmt.Printf("Tool: %s (%s)\n", event.ToolCall.Name, event.ToolCall.ID)
        if event.ToolCall.Result != "" {
            fmt.Printf("Result: %s\n", event.ToolCall.Result)
        }
        if event.ToolCall.Error != "" {
            fmt.Printf("Error: %s\n", event.ToolCall.Error)
        }
    }
```

### Completion Event

Signals the end of the stream:

```go
case "done":
    if event.Response != nil {
        fmt.Printf("Complete! Used %d tokens\n",
            event.Response.TokenUsage.TotalTokens)
    }
```

### Error Events

```go
if event.Error != nil {
    log.Printf("Error: %v", event.Error)
    // Stream will be closed after error
}
```

## Advanced: Context Cancellation

You can cancel a streaming request by cancelling the context:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

eventCh, err := session.SubmitStream(ctx, "Write a very long story")
if err != nil {
    log.Fatal(err)
}

// Process events until timeout or completion
for event := range eventCh {
    if event.Error != nil {
        if strings.Contains(event.Error.Error(), "context deadline exceeded") {
            fmt.Println("Request timed out")
        }
        break
    }

    if event.Done {
        fmt.Println("Request completed")
        break
    }

    // Handle other events...
}
```

## Multi-Turn Conversations

Both modes support multi-turn conversations. History is automatically maintained:

```go
session, _ := codex.NewSession(ctx, sdk.SessionOptions{
    Streaming: true,
})

// First message
eventCh, _ := session.SubmitStream(ctx, "What is 2+2?")
for event := range eventCh {
    // Process events...
}

// Second message - AI remembers previous context
eventCh, _ = session.SubmitStream(ctx, "What about 3+3?")
for event := range eventCh {
    // Process events...
}

// View conversation history
history := session.History()
for _, msg := range history {
    fmt.Printf("%s: %s\n", msg.Role, msg.Content)
}
```

## Tool Execution

Enable tool execution by configuring the approval policy:

```go
session, err := codex.NewSession(ctx, sdk.SessionOptions{
    Streaming:        true,
    ApprovalPolicy:   "auto",  // Auto-approve tool calls
    WorkingDirectory: "/path/to/project",
    SandboxPolicy:    "workspace-write",  // Allow file operations
})

eventCh, _ := session.SubmitStream(ctx,
    "Read the contents of README.md and summarize it")

for event := range eventCh {
    switch event.Type {
    case "tool_call_delta":
        // Tool calls are executed automatically
        fmt.Printf("Executing: %s\n", event.ToolCall.Name)
    case "content_delta":
        // AI's response after using tools
        fmt.Print(event.Delta)
    }
}
```

## Best Practices

1. **Always check for errors**: Check `event.Error` on each event
2. **Handle context cancellation**: Use context timeouts to prevent hanging
3. **Buffer output appropriately**: The channel has a buffer size of 10
4. **Close resources**: Always close or defer close SDK resources
5. **Choose the right mode**: Use streaming for UIs, non-streaming for scripts

## Error Handling

```go
eventCh, err := session.SubmitStream(ctx, message)
if err != nil {
    // Error creating stream
    return fmt.Errorf("failed to create stream: %w", err)
}

for event := range eventCh {
    if event.Error != nil {
        // Error during streaming
        return fmt.Errorf("stream error: %w", event.Error)
    }

    // Process event...
}

// Check if stream completed successfully
// (channel closed without Done event = error)
```

## Performance Considerations

- Streaming provides lower latency (first token arrives faster)
- Non-streaming is simpler but has higher latency
- Both modes use the same underlying infrastructure
- Tool execution happens transparently in both modes
- Token usage is the same regardless of mode

## Limitations

1. Cannot use `Submit()` on a streaming session
2. Cannot use `SubmitStream()` on a non-streaming session
3. Only one operation per session at a time
4. Sessions must be closed to release resources
5. Maximum context window depends on the model
