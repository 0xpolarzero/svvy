package openai

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/client"
	"github.com/evmts/codex/codex-go/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegrationRetry tests retry logic with exponential backoff.
func TestIntegrationRetry(t *testing.T) {
	test.SkipInShort(t, "integration test")

	t.Run("retry on 429", func(t *testing.T) {
		var attemptCount atomic.Int32

		mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
			count := attemptCount.Add(1)

			if count <= 2 {
				// Fail first 2 attempts with 429
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]interface{}{
						"message": "Rate limit exceeded",
						"type":    "rate_limit_error",
					},
				})
				return
			}

			// Succeed on 3rd attempt
			resp := &client.ChatCompletionResponse{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "gpt-4",
				Choices: []client.Choice{
					{
						Index: 0,
						Message: client.Message{
							Role:    "assistant",
							Content: "Success after retry",
						},
						FinishReason: "stop",
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		})

		cfg := client.ClientConfig{
			BaseURL:        mockServer.URL,
			APIKey:         "test-key",
			Model:          "gpt-4",
			RequestTimeout: 30 * time.Second,
			RetryConfig: client.RetryConfig{
				MaxRetries:           3,
				InitialBackoff:       100 * time.Millisecond,
				MaxBackoff:           5 * time.Second,
				BackoffMultiplier:    2.0,
				RetryableStatusCodes: []int{429, 500, 502, 503, 504},
				RespectRetryAfter:    true,
			},
		}

		c, err := NewClient(cfg)
		require.NoError(t, err)

		ctx := test.ContextWithTimeout(t, 10*time.Second)
		req := &client.ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []client.Message{
				client.NewUserMessage("Test"),
			},
		}

		start := time.Now()
		resp, err := c.Complete(ctx, req)
		elapsed := time.Since(start)

		require.NoError(t, err)
		assert.Equal(t, int32(3), attemptCount.Load())
		assert.Equal(t, "Success after retry", resp.Choices[0].Message.Content)
		// Should have waited for retries
		assert.Greater(t, elapsed, 200*time.Millisecond)
	})

	t.Run("retry exhausted", func(t *testing.T) {
		var attemptCount atomic.Int32

		mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
			attemptCount.Add(1)

			// Always return 500
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Internal server error",
					"type":    "server_error",
				},
			})
		})

		cfg := client.ClientConfig{
			BaseURL:        mockServer.URL,
			APIKey:         "test-key",
			Model:          "gpt-4",
			RequestTimeout: 30 * time.Second,
			RetryConfig: client.RetryConfig{
				MaxRetries:           2,
				InitialBackoff:       50 * time.Millisecond,
				MaxBackoff:           1 * time.Second,
				BackoffMultiplier:    2.0,
				RetryableStatusCodes: []int{429, 500, 502, 503, 504},
				RespectRetryAfter:    false,
			},
		}

		c, err := NewClient(cfg)
		require.NoError(t, err)

		ctx := test.ContextWithTimeout(t, 10*time.Second)
		req := &client.ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []client.Message{
				client.NewUserMessage("Test"),
			},
		}

		_, err = c.Complete(ctx, req)
		require.Error(t, err)
		// Should have retried MaxRetries + 1 times (initial + retries)
		assert.GreaterOrEqual(t, attemptCount.Load(), int32(3))
		assert.IsType(t, &client.RetryLimitError{}, err)
	})

	t.Run("no retry on non-retryable status", func(t *testing.T) {
		var attemptCount atomic.Int32

		mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
			attemptCount.Add(1)

			// Return 400 (not retryable)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Bad request",
					"type":    "invalid_request_error",
				},
			})
		})

		cfg := client.ClientConfig{
			BaseURL:        mockServer.URL,
			APIKey:         "test-key",
			Model:          "gpt-4",
			RequestTimeout: 30 * time.Second,
			RetryConfig: client.RetryConfig{
				MaxRetries:           3,
				InitialBackoff:       100 * time.Millisecond,
				MaxBackoff:           5 * time.Second,
				BackoffMultiplier:    2.0,
				RetryableStatusCodes: []int{429, 500, 502, 503, 504},
			},
		}

		c, err := NewClient(cfg)
		require.NoError(t, err)

		ctx := test.ContextWithTimeout(t, 5*time.Second)
		req := &client.ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []client.Message{
				client.NewUserMessage("Test"),
			},
		}

		_, err = c.Complete(ctx, req)
		require.Error(t, err)
		// Should only attempt once (no retry)
		assert.Equal(t, int32(1), attemptCount.Load())
	})
}

// TestIntegrationStreaming tests full streaming scenarios.
func TestIntegrationStreaming(t *testing.T) {
	test.SkipInShort(t, "integration test")

	t.Run("complete conversation flow", func(t *testing.T) {
		mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			flusher := w.(http.Flusher)

			// Simulate a realistic conversation response
			chunks := []client.ChatCompletionChunk{
				{
					ID:      "chatcmpl-123",
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   "gpt-4",
					Choices: []client.ChunkChoice{
						{Index: 0, Delta: client.MessageDelta{Role: "assistant"}},
					},
				},
				{
					ID:      "chatcmpl-123",
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   "gpt-4",
					Choices: []client.ChunkChoice{
						{Index: 0, Delta: client.MessageDelta{Content: "The"}},
					},
				},
				{
					ID:      "chatcmpl-123",
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   "gpt-4",
					Choices: []client.ChunkChoice{
						{Index: 0, Delta: client.MessageDelta{Content: " weather"}},
					},
				},
				{
					ID:      "chatcmpl-123",
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   "gpt-4",
					Choices: []client.ChunkChoice{
						{Index: 0, Delta: client.MessageDelta{Content: " is"}},
					},
				},
				{
					ID:      "chatcmpl-123",
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   "gpt-4",
					Choices: []client.ChunkChoice{
						{Index: 0, Delta: client.MessageDelta{Content: " sunny"}},
					},
				},
				{
					ID:      "chatcmpl-123",
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   "gpt-4",
					Choices: []client.ChunkChoice{
						{Index: 0, Delta: client.MessageDelta{Content: "."}},
					},
				},
				{
					ID:      "chatcmpl-123",
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   "gpt-4",
					Choices: []client.ChunkChoice{
						{Index: 0, Delta: client.MessageDelta{}, FinishReason: "stop"},
					},
					Usage: &client.TokenUsage{
						InputTokens:  15,
						OutputTokens: 8,
						TotalTokens:  23,
					},
				},
			}

			for _, chunk := range chunks {
				data, _ := json.Marshal(chunk)
				w.Write([]byte("data: " + string(data) + "\n\n"))
				flusher.Flush()
				time.Sleep(20 * time.Millisecond)
			}

			w.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()
		})

		cfg := client.ClientConfig{
			BaseURL:        mockServer.URL,
			APIKey:         "test-key",
			Model:          "gpt-4",
			RequestTimeout: 30 * time.Second,
			StreamConfig: client.StreamConfig{
				IdleTimeout: 5 * time.Second,
				BufferSize:  16,
			},
		}

		c, err := NewClient(cfg)
		require.NoError(t, err)

		ctx := test.ContextWithTimeout(t, 10*time.Second)
		req := &client.ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []client.Message{
				client.NewSystemMessage("You are a helpful assistant."),
				client.NewUserMessage("What's the weather like?"),
			},
		}

		eventCh, err := c.Stream(ctx, req)
		require.NoError(t, err)

		var textParts []string
		var completed bool
		var tokenUsage *client.TokenUsage

		for event := range eventCh {
			require.NoError(t, event.Error)

			switch event.Type {
			case client.EventTypeOutputTextDelta:
				if text, ok := event.Data.(string); ok {
					textParts = append(textParts, text)
				}
			case client.EventTypeCompleted:
				completed = true
				if completedEvent, ok := event.Data.(*client.CompletedEvent); ok {
					tokenUsage = completedEvent.TokenUsage
				}
			}
		}

		fullText := strings.Join(textParts, "")
		assert.Equal(t, "The weather is sunny.", fullText)
		assert.True(t, completed)
		assert.NotNil(t, tokenUsage)
		assert.Equal(t, int64(15), tokenUsage.InputTokens)
		assert.Equal(t, int64(8), tokenUsage.OutputTokens)
	})

	t.Run("stream with tool calls", func(t *testing.T) {
		mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			flusher := w.(http.Flusher)

			// Stream tool call chunks
			chunks := []string{
				`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}`,
				`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"location\""}}]},"finish_reason":null}]}`,
				`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"San Francisco\"}"}}]},"finish_reason":null}]}`,
				`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
			}

			for _, chunk := range chunks {
				w.Write([]byte("data: " + chunk + "\n\n"))
				flusher.Flush()
				time.Sleep(20 * time.Millisecond)
			}

			w.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()
		})

		cfg := client.ClientConfig{
			BaseURL:        mockServer.URL,
			APIKey:         "test-key",
			Model:          "gpt-4",
			RequestTimeout: 30 * time.Second,
		}

		c, err := NewClient(cfg)
		require.NoError(t, err)

		ctx := test.ContextWithTimeout(t, 10*time.Second)
		req := &client.ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []client.Message{
				client.NewUserMessage("What's the weather in San Francisco?"),
			},
			Tools: []client.Tool{
				client.NewFunctionTool("get_weather", "Get weather", json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`)),
			},
		}

		eventCh, err := c.Stream(ctx, req)
		require.NoError(t, err)

		var events []client.StreamEvent
		for event := range eventCh {
			require.NoError(t, event.Error)
			events = append(events, event)
		}

		// Should have tool call events
		assert.NotEmpty(t, events)
	})
}

// TestIntegrationRateLimiting tests rate limiting behavior.
func TestIntegrationRateLimiting(t *testing.T) {
	test.SkipInShort(t, "integration test")

	t.Run("rate limit headers parsed", func(t *testing.T) {
		mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
			// Set rate limit headers
			w.Header().Set("x-ratelimit-limit-requests", "10000")
			w.Header().Set("x-ratelimit-remaining-requests", "9990")
			w.Header().Set("x-ratelimit-reset-requests", "60s")
			w.Header().Set("x-ratelimit-limit-tokens", "1000000")
			w.Header().Set("x-ratelimit-remaining-tokens", "990000")
			w.Header().Set("x-ratelimit-reset-tokens", "3600s")

			resp := &client.ChatCompletionResponse{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "gpt-4",
				Choices: []client.Choice{
					{
						Index: 0,
						Message: client.Message{
							Role:    "assistant",
							Content: "Response",
						},
						FinishReason: "stop",
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		})

		cfg := client.ClientConfig{
			BaseURL:        mockServer.URL,
			APIKey:         "test-key",
			Model:          "gpt-4",
			RequestTimeout: 5 * time.Second,
		}

		c, err := NewClient(cfg)
		require.NoError(t, err)

		ctx := test.ContextWithTimeout(t, 5*time.Second)
		req := &client.ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []client.Message{
				client.NewUserMessage("Test"),
			},
		}

		_, err = c.Complete(ctx, req)
		require.NoError(t, err)

		// Rate limits should be tracked internally
		// Would need to expose rate limit state to test fully
	})
}

// TestIntegrationErrorHandling tests various error scenarios.
func TestIntegrationErrorHandling(t *testing.T) {
	test.SkipInShort(t, "integration test")

	t.Run("authentication error", func(t *testing.T) {
		mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Invalid API key",
					"type":    "authentication_error",
				},
			})
		})

		cfg := client.ClientConfig{
			BaseURL:        mockServer.URL,
			APIKey:         "invalid-key",
			Model:          "gpt-4",
			RequestTimeout: 5 * time.Second,
			RetryConfig:    client.RetryConfig{MaxRetries: 0},
		}

		c, err := NewClient(cfg)
		require.NoError(t, err)

		ctx := test.ContextWithTimeout(t, 5*time.Second)
		req := &client.ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []client.Message{
				client.NewUserMessage("Test"),
			},
		}

		_, err = c.Complete(ctx, req)
		require.Error(t, err)
		assert.IsType(t, &client.UnauthorizedError{}, err)

		authErr := err.(*client.UnauthorizedError)
		assert.Contains(t, authErr.Message, "Invalid API key")
	})

	t.Run("context window exceeded", func(t *testing.T) {
		mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "This model's maximum context length is 8192 tokens",
					"type":    "invalid_request_error",
					"code":    "context_length_exceeded",
				},
			})
		})

		cfg := client.ClientConfig{
			BaseURL:        mockServer.URL,
			APIKey:         "test-key",
			Model:          "gpt-4",
			RequestTimeout: 5 * time.Second,
			RetryConfig:    client.RetryConfig{MaxRetries: 0},
		}

		c, err := NewClient(cfg)
		require.NoError(t, err)

		ctx := test.ContextWithTimeout(t, 5*time.Second)
		req := &client.ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []client.Message{
				client.NewUserMessage(strings.Repeat("x", 100000)),
			},
		}

		_, err = c.Complete(ctx, req)
		require.Error(t, err)
		// Should detect context window error
		assert.Contains(t, err.Error(), "context")
	})

	t.Run("connection error", func(t *testing.T) {
		cfg := client.ClientConfig{
			BaseURL:        "http://localhost:1", // Invalid port
			APIKey:         "test-key",
			Model:          "gpt-4",
			RequestTimeout: 1 * time.Second,
			RetryConfig:    client.RetryConfig{MaxRetries: 0},
		}

		c, err := NewClient(cfg)
		require.NoError(t, err)

		ctx := test.ContextWithTimeout(t, 5*time.Second)
		req := &client.ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []client.Message{
				client.NewUserMessage("Test"),
			},
		}

		_, err = c.Complete(ctx, req)
		require.Error(t, err)
		// Should be a connection error
		assert.IsType(t, &client.ConnectionError{}, err)
	})
}

// TestIntegrationConcurrency tests concurrent requests.
func TestIntegrationConcurrency(t *testing.T) {
	test.SkipInShort(t, "integration test")

	var requestCount atomic.Int32

	mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		// Simulate some processing time
		time.Sleep(50 * time.Millisecond)

		resp := &client.ChatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4",
			Choices: []client.Choice{
				{
					Index: 0,
					Message: client.Message{
						Role:    "assistant",
						Content: "Response",
					},
					FinishReason: "stop",
				},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	})

	cfg := client.ClientConfig{
		BaseURL:        mockServer.URL,
		APIKey:         "test-key",
		Model:          "gpt-4",
		RequestTimeout: 30 * time.Second,
	}

	c, err := NewClient(cfg)
	require.NoError(t, err)

	// Make 10 concurrent requests
	const numRequests = 10
	errCh := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(i int) {
			ctx := test.ContextWithTimeout(t, 5*time.Second)
			req := &client.ChatCompletionRequest{
				Model: "gpt-4",
				Messages: []client.Message{
					client.NewUserMessage("Test"),
				},
			}

			_, err := c.Complete(ctx, req)
			errCh <- err
		}(i)
	}

	// Wait for all requests
	for i := 0; i < numRequests; i++ {
		err := <-errCh
		assert.NoError(t, err)
	}

	assert.Equal(t, int32(numRequests), requestCount.Load())
}
