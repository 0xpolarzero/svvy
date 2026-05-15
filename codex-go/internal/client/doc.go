// Package client provides a robust, production-ready interface for interacting with
// AI model APIs, including OpenAI's Chat Completions and experimental Responses APIs.
//
// # Overview
//
// This package defines interfaces and types for:
//   - Streaming and non-streaming chat completions
//   - Server-Sent Events (SSE) parsing and event dispatching
//   - Automatic retry with exponential backoff
//   - Rate limit detection and handling
//   - Token usage tracking and context window management
//   - Function/tool calling with various tool types
//
// # Architecture
//
// The Client interface is the primary abstraction. Implementations handle:
//   - HTTP request construction and execution
//   - Authentication and authorization
//   - Request/response serialization (JSON)
//   - SSE stream parsing
//   - Error recovery and retry logic
//
// The package supports two API styles:
//
//  1. Chat Completions API (standard OpenAI format)
//     - Uses Messages array with role/content structure
//     - Supports function calling via ToolCalls
//     - Streaming via text/event-stream
//
//  2. Responses API (OpenAI experimental)
//     - Uses Instructions + Input array
//     - Enhanced reasoning support
//     - Richer tool types (local_shell, web_search, custom)
//
// # Usage Example
//
// Basic streaming request:
//
//	config := ClientConfig{
//		BaseURL:        "https://api.openai.com/v1",
//		APIKey:         os.Getenv("OPENAI_API_KEY"),
//		Model:          "gpt-4",
//		StreamConfig:   DefaultStreamConfig(),
//		RetryConfig:    DefaultRetryConfig(),
//		RequestTimeout: 5 * time.Minute,
//	}
//
//	client := NewClient(config)
//
//	req := &ChatCompletionRequest{
//		Model: "gpt-4",
//		Messages: []Message{
//			NewSystemMessage("You are a helpful assistant."),
//			NewUserMessage("What is the capital of France?"),
//		},
//		Stream: true,
//	}
//
//	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
//	defer cancel()
//
//	events, err := client.Stream(ctx, req)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	for event := range events {
//		if event.Error != nil {
//			log.Printf("Error: %v", event.Error)
//			continue
//		}
//
//		switch event.Type {
//		case EventTypeOutputTextDelta:
//			if delta, ok := event.Data.(string); ok {
//				fmt.Print(delta)
//			}
//		case EventTypeCompleted:
//			if completed, ok := event.Data.(*CompletedEvent); ok {
//				fmt.Printf("\nTokens used: %d\n", completed.TokenUsage.TotalTokens)
//			}
//		}
//	}
//
// # Function Calling
//
// Define tools the model can invoke:
//
//	weatherTool := NewFunctionTool(
//		"get_weather",
//		"Get current weather for a location",
//		json.RawMessage(`{
//			"type": "object",
//			"properties": {
//				"location": {"type": "string"},
//				"unit": {"type": "string", "enum": ["celsius", "fahrenheit"]}
//			},
//			"required": ["location"]
//		}`),
//	)
//
//	req := &ChatCompletionRequest{
//		Model:    "gpt-4",
//		Messages: []Message{NewUserMessage("What's the weather in Paris?")},
//		Tools:    []Tool{weatherTool},
//		Stream:   true,
//	}
//
// Process tool calls from the response:
//
//	for event := range events {
//		if event.Type == EventTypeOutputItemDone {
//			if item, ok := event.Data.(*ResponseItem); ok && item.Type == "function_call" {
//				// Execute the function with item.Arguments
//				result := executeFunction(item.Name, item.Arguments)
//
//				// Send the result back
//				req.Messages = append(req.Messages, NewToolMessage(item.CallID, result))
//			}
//		}
//	}
//
// # Error Handling
//
// The package distinguishes between retryable and fatal errors:
//
// Retryable errors (automatically retried per RetryConfig):
//   - Network timeouts and transient connection failures
//   - HTTP 429 (rate limit exceeded)
//   - HTTP 5xx (server errors)
//
// Fatal errors (fail immediately):
//   - HTTP 400 (bad request)
//   - HTTP 401/403 (authentication/authorization)
//   - Context cancellation
//   - Invalid request structure
//
// Stream-specific errors:
//   - Idle timeout (no events within IdleTimeout)
//   - SSE parse errors
//   - Mid-stream disconnections
//
// Check errors in the event stream:
//
//	for event := range events {
//		if event.Error != nil {
//			// Handle error
//			if errors.Is(event.Error, context.Canceled) {
//				// User canceled
//			} else if errors.Is(event.Error, context.DeadlineExceeded) {
//				// Request timeout
//			} else {
//				// API or network error
//			}
//			break
//		}
//	}
//
// # Rate Limiting
//
// The client tracks rate limits from response headers:
//
//	for event := range events {
//		if event.Type == EventTypeRateLimits {
//			if limits, ok := event.Data.(*RateLimitSnapshot); ok {
//				if limits.Primary != nil && limits.Primary.UsedPercent > 90 {
//					log.Println("Warning: approaching rate limit")
//				}
//			}
//		}
//	}
//
// When rate limited (HTTP 429), the client:
//  1. Respects Retry-After header if present
//  2. Falls back to exponential backoff
//  3. Retries up to RetryConfig.MaxRetries times
//
// # Context and Cancellation
//
// All operations accept a context.Context for:
//   - Request timeouts
//   - User cancellation
//   - Graceful shutdown
//
// Always use timeouts to prevent unbounded waits:
//
//	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
//	defer cancel()
//
//	events, err := client.Stream(ctx, req)
//
// Cancel streaming from another goroutine:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	go func() {
//		<-stopSignal
//		cancel() // Stops the stream
//	}()
//
//	events, err := client.Stream(ctx, req)
//
// # Token Management
//
// Query model capabilities:
//
//	contextWindow := client.GetModelContextWindow()
//	compactLimit := client.GetAutoCompactTokenLimit()
//
//	if totalTokens > compactLimit {
//		// Trigger conversation compaction
//	}
//
// Track usage per request:
//
//	for event := range events {
//		if event.Type == EventTypeCompleted {
//			completed := event.Data.(*CompletedEvent)
//			logTokenUsage(completed.TokenUsage)
//		}
//	}
//
// # Testing
//
// Mock the Client interface for testing:
//
//	//go:generate mockgen -destination=mocks/mock_client.go -package=mocks github.com/evmts/codex/codex-go/internal/client Client
//
//	func TestMyCode(t *testing.T) {
//		ctrl := gomock.NewController(t)
//		defer ctrl.Finish()
//
//		mockClient := mocks.NewMockClient(ctrl)
//		mockClient.EXPECT().
//			Stream(gomock.Any(), gomock.Any()).
//			Return(makeTestChannel(), nil)
//
//		// Test code using mockClient
//	}
//
// # Thread Safety
//
// Client implementations must be safe for concurrent use by multiple goroutines.
// The returned event channels are goroutine-safe.
//
// # Implementation Notes
//
// This package defines only interfaces and types. Concrete implementations
// are provided by other packages:
//   - internal/client/openai: OpenAI-compatible API client
//   - internal/client/anthropic: Anthropic API client (future)
//   - internal/client/azure: Azure OpenAI client (future)
//
// Each implementation may support different subsets of functionality
// (e.g., some providers don't support reasoning or certain tool types).
package client
