# OpenAI Client Package

This package provides a production-ready OpenAI-compatible client implementation with advanced features including streaming, retry logic, rate limiting, and comprehensive error handling.

## Overview

The OpenAI client implements the `client.Client` interface for OpenAI-compatible APIs. It's designed to be robust, with built-in retry mechanisms, rate limit tracking, and SSE (Server-Sent Events) streaming support for real-time responses.

## Features

### Streaming Support

The client supports both streaming and non-streaming completions via SSE:

```go
// Non-streaming completion
resp, err := client.Complete(ctx, req)

// Streaming completion
eventCh, err := client.Stream(ctx, req)
for event := range eventCh {
    switch event.Type {
    case client.EventTypeOutputTextDelta:
        fmt.Print(event.Data.(string))
    case client.EventTypeCompleted:
        // Stream completed
    case client.EventTypeError:
        // Handle error
    }
}
```

**Streaming Features:**
- **SSE Parsing**: Robust Server-Sent Events parser with proper line buffering
- **Tool Call Accumulation**: Automatically reconstructs streaming tool calls from deltas
- **Idle Timeout**: Configurable timeout to detect stalled streams
- **Context Cancellation**: Respects context deadlines and cancellations
- **Event Types**: Supports creation, text deltas, tool calls, reasoning (if enabled), and completion events

### Retry Logic

Automatic retry with exponential backoff for transient failures:

```go
cfg := client.ClientConfig{
    BaseURL: "https://api.openai.com/v1",
    APIKey:  os.Getenv("OPENAI_API_KEY"),
    Model:   "gpt-4",
    RetryConfig: client.RetryConfig{
        MaxRetries:           3,
        InitialBackoff:       1 * time.Second,
        MaxBackoff:           30 * time.Second,
        BackoffMultiplier:    2.0,
        RetryableStatusCodes: []int{429, 500, 502, 503, 504},
    },
}
```

**Retry Features:**
- **Exponential Backoff**: Configurable initial delay, max delay, and multiplier
- **Jitter**: Adds ±25% randomization to prevent thundering herd
- **Configurable Status Codes**: Define which HTTP status codes trigger retries
- **Context-Aware**: Respects context cancellation during backoff delays
- **Per-Request Retry**: Both streaming and non-streaming requests benefit from retry logic

### Rate Limiting

Transparent rate limit tracking from API response headers:

```go
// Rate limits are automatically updated from response headers:
// x-ratelimit-limit-requests, x-ratelimit-remaining-requests
// x-ratelimit-limit-tokens, x-ratelimit-remaining-tokens
```

**Rate Limit Features:**
- **Automatic Tracking**: Parses standard OpenAI rate limit headers
- **Dual Tracking**: Monitors both request-based and token-based limits
- **Reset Time Tracking**: Tracks when rate limits will reset
- **Usage Percentage**: Calculates percentage of limit consumed
- **Thread-Safe**: Uses read-write mutex for concurrent access

### Error Handling

Comprehensive error types for better error handling:

- **Connection Errors**: Network and connection failures
- **Status Errors**: HTTP error responses with status codes
- **Context Window Exceeded**: Token limit errors
- **Usage Limit Errors**: Rate limiting errors
- **Retry Limit Errors**: Maximum retry attempts reached

```go
if err != nil {
    switch e := err.(type) {
    case *client.ToolError:
        // Handle specific error types
    case *client.ConnectionError:
        // Network issues
    }
}
```

## Usage Examples

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/evmts/codex/codex-go/internal/client"
    "github.com/evmts/codex/codex-go/internal/client/openai"
)

func main() {
    // Create client
    cfg := client.ClientConfig{
        BaseURL:        "https://api.openai.com/v1",
        APIKey:         os.Getenv("OPENAI_API_KEY"),
        Model:          "gpt-4",
        RequestTimeout: 30 * time.Second,
    }

    c, err := openai.NewClient(cfg)
    if err != nil {
        panic(err)
    }

    // Make request
    req := &client.ChatCompletionRequest{
        Messages: []client.Message{
            {Role: "user", Content: "Hello!"},
        },
    }

    resp, err := c.Complete(context.Background(), req)
    if err != nil {
        panic(err)
    }

    fmt.Println(resp.Choices[0].Message.Content)
}
```

### Streaming with Custom Configuration

```go
cfg := client.ClientConfig{
    BaseURL: "https://api.openai.com/v1",
    APIKey:  os.Getenv("OPENAI_API_KEY"),
    Model:   "gpt-4",
    StreamConfig: client.StreamConfig{
        BufferSize:               100,
        IdleTimeout:              60 * time.Second,
        EnableRawAgentReasoning:  false,
    },
}

c, _ := openai.NewClient(cfg)

eventCh, _ := c.Stream(ctx, req)
for event := range eventCh {
    // Process events
}
```

### With Custom HTTP Client

```go
httpClient := &http.Client{
    Timeout: 60 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:    100,
        IdleConnTimeout: 90 * time.Second,
    },
}

cfg := client.ClientConfig{
    BaseURL:    "https://api.openai.com/v1",
    APIKey:     os.Getenv("OPENAI_API_KEY"),
    Model:      "gpt-4",
    HTTPClient: &customHTTPClient{httpClient},
}
```

## Design Decisions

### Why SSE for Streaming?

Server-Sent Events (SSE) is the standard streaming protocol used by OpenAI and compatible APIs. Our implementation:
- Uses `bufio.Scanner` for line-by-line parsing
- Handles `data:` prefixed lines according to SSE spec
- Detects `[DONE]` sentinel for stream completion
- Gracefully handles malformed or comment lines

### Tool Call Accumulation

Streaming tool calls arrive as deltas (fragments). The `toolCallAccumulator`:
- Reconstructs complete tool calls from index-keyed deltas
- Handles interleaved tool call fragments
- Preserves order using index-based sorting
- Emits complete tool calls only on finish

### Model Context Window

The client includes a model information system:
- Maps common models to their context windows
- Auto-compacts at 80% of context window by default
- Extensible for new models
- Supports custom overrides via configuration

### Thread Safety

All shared state is protected:
- Rate limit tracker uses `sync.RWMutex`
- Model info initialized with `sync.Once`
- Client is safe for concurrent use

### Retry Strategy

The exponential backoff with jitter strategy:
- **Prevents Thundering Herd**: Jitter spreads out retry attempts
- **Respects Context**: Always checks context before sleeping
- **Configurable**: Can be tuned per deployment needs
- **Status-Based**: Only retries errors that make sense (429, 5xx)

### Error Wrapping

Errors maintain context through the stack:
- Original errors preserved via `Cause` field
- Structured error types for programmatic handling
- Human-readable messages for logging
- Status codes preserved for debugging

## Configuration Reference

### ClientConfig

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `BaseURL` | string | API base URL | Required |
| `APIKey` | string | Authentication key | Required |
| `Model` | string | Model name | Required |
| `RequestTimeout` | duration | HTTP timeout | 30s |
| `ConversationID` | string | Optional conversation ID | "" |
| `Headers` | map[string]string | Custom headers | {} |
| `HTTPClient` | HTTPClient | Custom HTTP client | Default |
| `StreamConfig` | StreamConfig | Streaming settings | Default |
| `RetryConfig` | RetryConfig | Retry settings | Default |

### RetryConfig

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `MaxRetries` | int | Maximum retry attempts | 3 |
| `InitialBackoff` | duration | Initial backoff delay | 1s |
| `MaxBackoff` | duration | Maximum backoff delay | 30s |
| `BackoffMultiplier` | float64 | Backoff growth rate | 2.0 |
| `RetryableStatusCodes` | []int | Status codes to retry | [429, 500, 502, 503, 504] |

### StreamConfig

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `BufferSize` | int | Event channel buffer | 100 |
| `IdleTimeout` | duration | Stream idle timeout | 60s |
| `EnableRawAgentReasoning` | bool | Include reasoning events | false |

## Testing

The package includes comprehensive tests:
- Unit tests for retry logic, rate limiting, and streaming
- Integration tests against real APIs (when configured)
- Mock HTTP clients for deterministic testing
- Error scenario coverage

Run tests:
```bash
go test -v ./internal/client/openai/...
```

## Future Enhancements

Potential improvements:
- [ ] Response streaming with incremental parsing
- [ ] Automatic retry after rate limit reset
- [ ] Request batching support
- [ ] Response caching
- [ ] Metrics and telemetry hooks
- [ ] Circuit breaker pattern
