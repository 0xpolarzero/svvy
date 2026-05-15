// Package client provides interfaces and types for interacting with AI model APIs.
// It supports both streaming (SSE) and non-streaming responses with built-in retry logic,
// rate limiting, and token counting integration.
package client

import (
	"context"
	"io"
	"time"
)

//go:generate mockgen -destination=mocks/mock_client.go -package=mocks github.com/evmts/codex/codex-go/internal/client Client,HTTPClient

// Client defines the interface for making API requests to AI model providers.
// Implementations must handle:
//   - Authentication and authorization
//   - Request/response serialization
//   - Streaming via Server-Sent Events (SSE)
//   - Automatic retry with exponential backoff
//   - Rate limit detection and handling
//   - Token usage tracking
type Client interface {
	// Stream initiates a streaming request to the model API and returns a channel
	// of response events. The stream should handle:
	//   - SSE parsing and event dispatching
	//   - Graceful error handling and recovery
	//   - Context cancellation
	//   - Idle timeout detection
	//
	// The returned channel is closed when the stream completes or encounters an error.
	// Callers must drain the channel or cancel the context to avoid goroutine leaks.
	Stream(ctx context.Context, req *ChatCompletionRequest) (<-chan StreamEvent, error)

	// Complete performs a non-streaming request and returns the full response.
	// This is useful for:
	//   - Simple request/response patterns
	//   - Testing without stream complexity
	//   - Providers that don't support streaming
	Complete(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error)

	// GetModelContextWindow returns the effective context window size for the
	// configured model, accounting for any percentage limits from configuration.
	// Returns 0 if the model context window is unknown.
	GetModelContextWindow() int64

	// GetAutoCompactTokenLimit returns the token threshold at which automatic
	// history compaction should be triggered, if configured.
	// Returns 0 if auto-compaction is not enabled for this model.
	GetAutoCompactTokenLimit() int64
}

// HTTPClient is the interface for making HTTP requests. This abstraction allows
// for easier testing and mocking of HTTP interactions.
type HTTPClient interface {
	Do(req *HTTPRequest) (*HTTPResponse, error)
}

// HTTPRequest represents an HTTP request to be executed.
type HTTPRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    io.Reader
}

// HTTPResponse represents an HTTP response.
type HTTPResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       io.ReadCloser
}

// StreamEvent represents a single event in the response stream.
// Events are emitted as the model generates its response.
type StreamEvent struct {
	// Type indicates the kind of event (e.g., "created", "delta", "completed")
	Type EventType

	// Data contains the event-specific payload
	Data interface{}

	// Error contains any error that occurred during streaming
	Error error
}

// EventType represents the type of streaming event.
type EventType string

const (
	// EventTypeCreated signals the start of a response
	EventTypeCreated EventType = "created"

	// EventTypeOutputItemDone signals completion of an output item (message, function call, etc.)
	EventTypeOutputItemDone EventType = "output_item_done"

	// EventTypeOutputTextDelta contains incremental text from the assistant
	EventTypeOutputTextDelta EventType = "output_text_delta"

	// EventTypeReasoningContentDelta contains incremental reasoning/thinking text
	EventTypeReasoningContentDelta EventType = "reasoning_content_delta"

	// EventTypeReasoningSummaryDelta contains incremental reasoning summary text
	EventTypeReasoningSummaryDelta EventType = "reasoning_summary_delta"

	// EventTypeReasoningSummaryPartAdded signals a new reasoning summary section
	EventTypeReasoningSummaryPartAdded EventType = "reasoning_summary_part_added"

	// EventTypeWebSearchCallBegin signals the start of a web search tool call
	EventTypeWebSearchCallBegin EventType = "web_search_call_begin"

	// EventTypeRateLimits contains current rate limit information
	EventTypeRateLimits EventType = "rate_limits"

	// EventTypeCompleted signals successful completion of the response
	EventTypeCompleted EventType = "completed"

	// EventTypeError signals an error occurred
	EventTypeError EventType = "error"
)

// StreamConfig controls streaming behavior.
type StreamConfig struct {
	// IdleTimeout is the maximum duration to wait between events before
	// considering the stream dead. Zero means no timeout.
	IdleTimeout time.Duration

	// BufferSize controls the channel buffer size for events.
	// Larger buffers reduce backpressure but increase memory usage.
	BufferSize int

	// EnableRawAgentReasoning, when true, streams reasoning tokens as they arrive.
	// When false, only emits aggregated reasoning at turn boundaries.
	EnableRawAgentReasoning bool

	// EnableBackpressure, when true, applies flow control when the event channel is full.
	// This prevents memory exhaustion from fast streams with slow consumers.
	// When false, events may be dropped or buffer may grow unbounded depending on implementation.
	EnableBackpressure bool

	// BackpressureThreshold is the percentage (0.0 to 1.0) of buffer fullness that triggers warnings.
	// For example, 0.8 means warnings are logged when buffer is 80% full.
	BackpressureThreshold float64
}

// DefaultStreamConfig returns sensible defaults for streaming.
func DefaultStreamConfig() StreamConfig {
	return StreamConfig{
		IdleTimeout:             90 * time.Second,
		BufferSize:              100,
		EnableRawAgentReasoning: false,
		EnableBackpressure:      true,
		BackpressureThreshold:   0.8,
	}
}

// RetryConfig controls retry behavior for failed requests.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts.
	// 0 means no retries, requests fail on first error.
	MaxRetries int

	// InitialBackoff is the initial delay before the first retry.
	InitialBackoff time.Duration

	// MaxBackoff is the maximum delay between retries.
	MaxBackoff time.Duration

	// BackoffMultiplier controls exponential backoff growth.
	// Each retry waits BackoffMultiplier times longer than the previous.
	BackoffMultiplier float64

	// RetryableStatusCodes lists HTTP status codes that should trigger a retry.
	// Common retryable codes: 429 (rate limit), 500, 502, 503, 504 (server errors)
	RetryableStatusCodes []int

	// RespectRetryAfter, when true, honors the Retry-After response header.
	RespectRetryAfter bool
}

// DefaultRetryConfig returns sensible defaults for retries.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:           3,
		InitialBackoff:       1 * time.Second,
		MaxBackoff:           60 * time.Second,
		BackoffMultiplier:    2.0,
		RetryableStatusCodes: []int{429, 500, 502, 503, 504},
		RespectRetryAfter:    true,
	}
}

// TokenRefreshFunc is called when a 401 Unauthorized is received.
// It should return a new API key/token or an error if refresh fails.
// The context can be used to implement timeouts for the refresh operation.
type TokenRefreshFunc func(ctx context.Context, oldToken string) (newToken string, err error)

// ConnectionPoolConfig controls HTTP connection pooling behavior.
type ConnectionPoolConfig struct {
	// MaxIdleConns controls the maximum number of idle (keep-alive) connections across all hosts.
	// Default: 100
	MaxIdleConns int

	// MaxIdleConnsPerHost controls the maximum idle (keep-alive) connections to keep per-host.
	// Default: 10
	MaxIdleConnsPerHost int

	// MaxConnsPerHost optionally limits the total number of connections per host, including
	// connections in the dialing, active, and idle states. Default: 0 (unlimited)
	MaxConnsPerHost int

	// IdleConnTimeout is the maximum amount of time an idle (keep-alive) connection will remain
	// idle before closing itself. Default: 90 seconds
	IdleConnTimeout time.Duration

	// ConnectionTimeout is the maximum amount of time to wait for a connection to be established.
	// This is implemented via TLSHandshakeTimeout. Default: 10 seconds
	ConnectionTimeout time.Duration

	// EnableHTTP2 controls whether HTTP/2 support is enabled. Default: true
	EnableHTTP2 bool
}

// DefaultConnectionPoolConfig returns sensible defaults for connection pooling.
// These defaults match the Rust reqwest library's behavior.
func DefaultConnectionPoolConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     0, // unlimited
		IdleConnTimeout:     90 * time.Second,
		ConnectionTimeout:   10 * time.Second,
		EnableHTTP2:         true,
	}
}

// ClientConfig holds configuration for creating a client.
type ClientConfig struct {
	// BaseURL is the API endpoint (e.g., "https://api.openai.com/v1")
	BaseURL string

	// APIKey for authentication
	APIKey string

	// Model is the model identifier (e.g., "gpt-4", "claude-3-opus")
	Model string

	// HTTPClient is the underlying HTTP client to use
	HTTPClient HTTPClient

	// StreamConfig controls streaming behavior
	StreamConfig StreamConfig

	// RetryConfig controls retry behavior
	RetryConfig RetryConfig

	// ConnectionPoolConfig controls HTTP connection pooling
	ConnectionPoolConfig ConnectionPoolConfig

	// Headers contains additional HTTP headers to include in requests
	Headers map[string]string

	// RequestTimeout is the maximum duration for a complete request
	RequestTimeout time.Duration

	// ConversationID is used for prompt caching and session tracking
	ConversationID string

	// TokenRefreshFunc is called when a 401 Unauthorized response is received.
	// If set, the client will attempt to refresh the token and retry the request.
	// Only one retry attempt will be made per request.
	TokenRefreshFunc TokenRefreshFunc
}

// RateLimitSnapshot captures rate limit information from API responses.
type RateLimitSnapshot struct {
	// Primary rate limit window (if present)
	Primary *RateLimitWindow `json:"primary,omitempty"`

	// Secondary rate limit window (if present)
	Secondary *RateLimitWindow `json:"secondary,omitempty"`
}

// RateLimitWindow represents a single rate limit tracking window.
type RateLimitWindow struct {
	// UsedPercent is the percentage of the rate limit consumed (0.0 to 100.0)
	UsedPercent float64 `json:"used_percent"`

	// WindowMinutes is the duration of the rate limit window in minutes
	WindowMinutes *int64 `json:"window_minutes,omitempty"`

	// ResetsAt is the Unix timestamp when the rate limit resets
	ResetsAt *int64 `json:"resets_at,omitempty"`
}

// TokenUsage tracks token consumption for a request/response.
type TokenUsage struct {
	// InputTokens is the number of tokens in the prompt
	InputTokens int64 `json:"input_tokens"`

	// CachedInputTokens is the number of input tokens served from cache
	CachedInputTokens int64 `json:"cached_input_tokens"`

	// OutputTokens is the number of tokens in the completion
	OutputTokens int64 `json:"output_tokens"`

	// ReasoningOutputTokens is the number of tokens used for reasoning (e.g., o1)
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`

	// TotalTokens is the sum of input and output tokens
	TotalTokens int64 `json:"total_tokens"`
}

// CompletedEvent is emitted when a response stream completes successfully.
type CompletedEvent struct {
	// ResponseID is the unique identifier for this response
	ResponseID string `json:"response_id"`

	// TokenUsage contains token consumption details
	TokenUsage *TokenUsage `json:"token_usage,omitempty"`
}

// WebSearchCallBeginEvent is emitted when a web search tool call starts.
type WebSearchCallBeginEvent struct {
	// CallID is the unique identifier for this tool call
	CallID string `json:"call_id"`
}
