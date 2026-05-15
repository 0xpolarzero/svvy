package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/client"
	"github.com/evmts/codex/codex-go/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewClient tests the creation of a new OpenAI client.
func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		config  client.ClientConfig
		wantErr bool
	}{
		{
			name: "valid configuration",
			config: client.ClientConfig{
				BaseURL:        "https://api.openai.com/v1",
				APIKey:         "test-key",
				Model:          "gpt-4",
				RequestTimeout: 30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "missing base URL",
			config: client.ClientConfig{
				APIKey:         "test-key",
				Model:          "gpt-4",
				RequestTimeout: 30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "missing API key",
			config: client.ClientConfig{
				BaseURL:        "https://api.openai.com/v1",
				Model:          "gpt-4",
				RequestTimeout: 30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "missing model",
			config: client.ClientConfig{
				BaseURL:        "https://api.openai.com/v1",
				APIKey:         "test-key",
				RequestTimeout: 30 * time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewClient(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, c)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, c)
			}
		})
	}
}

// TestComplete tests non-streaming completions.
func TestComplete(t *testing.T) {
	tests := []struct {
		name           string
		request        *client.ChatCompletionRequest
		mockResponse   *client.ChatCompletionResponse
		mockStatusCode int
		wantErr        bool
		errType        interface{}
	}{
		{
			name: "successful completion",
			request: &client.ChatCompletionRequest{
				Model: "gpt-4",
				Messages: []client.Message{
					client.NewUserMessage("Hello"),
				},
			},
			mockResponse: &client.ChatCompletionResponse{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "gpt-4",
				Choices: []client.Choice{
					{
						Index: 0,
						Message: client.Message{
							Role:    "assistant",
							Content: "Hi there!",
						},
						FinishReason: "stop",
					},
				},
				Usage: &client.TokenUsage{
					InputTokens:  10,
					OutputTokens: 5,
					TotalTokens:  15,
				},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name: "with tool calls",
			request: &client.ChatCompletionRequest{
				Model: "gpt-4",
				Messages: []client.Message{
					client.NewUserMessage("What's the weather?"),
				},
				Tools: []client.Tool{
					client.NewFunctionTool("get_weather", "Get weather", json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`)),
				},
			},
			mockResponse: &client.ChatCompletionResponse{
				ID:      "chatcmpl-124",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "gpt-4",
				Choices: []client.Choice{
					{
						Index: 0,
						Message: client.Message{
							Role:    "assistant",
							Content: "",
							ToolCalls: []client.ToolCall{
								{
									ID:   "call_123",
									Type: "function",
									Function: &client.FunctionCall{
										Name:      "get_weather",
										Arguments: `{"location":"San Francisco"}`,
									},
								},
							},
						},
						FinishReason: "tool_calls",
					},
				},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name: "rate limited - 429",
			request: &client.ChatCompletionRequest{
				Model: "gpt-4",
				Messages: []client.Message{
					client.NewUserMessage("Hello"),
				},
			},
			mockStatusCode: http.StatusTooManyRequests,
			wantErr:        true,
			errType:        &client.UsageLimitError{}, // 429 returns UsageLimitError
		},
		{
			name: "server error - 500",
			request: &client.ChatCompletionRequest{
				Model: "gpt-4",
				Messages: []client.Message{
					client.NewUserMessage("Hello"),
				},
			},
			mockStatusCode: http.StatusInternalServerError,
			wantErr:        true,
			errType:        &client.UnexpectedStatusError{},
		},
		{
			name: "context window exceeded",
			request: &client.ChatCompletionRequest{
				Model: "gpt-4",
				Messages: []client.Message{
					client.NewUserMessage(strings.Repeat("x", 100000)),
				},
			},
			mockStatusCode: http.StatusBadRequest,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockServer *test.HTTPMockServer

			if tt.mockResponse != nil {
				mockServer = test.NewJSONMockServer(t, tt.mockStatusCode, tt.mockResponse)
			} else {
				mockServer = test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.mockStatusCode)
					if tt.mockStatusCode == http.StatusBadRequest {
						json.NewEncoder(w).Encode(map[string]interface{}{
							"error": map[string]interface{}{
								"message": "context_length_exceeded",
								"type":    "invalid_request_error",
							},
						})
					} else if tt.mockStatusCode == http.StatusTooManyRequests {
						json.NewEncoder(w).Encode(map[string]interface{}{
							"error": map[string]interface{}{
								"message": "Rate limit exceeded",
								"type":    "rate_limit_error",
							},
						})
					} else if tt.mockStatusCode >= 500 {
						json.NewEncoder(w).Encode(map[string]interface{}{
							"error": map[string]interface{}{
								"message": "Internal server error",
								"type":    "server_error",
							},
						})
					}
				})
			}

			cfg := client.ClientConfig{
				BaseURL:        mockServer.URL,
				APIKey:         "test-key",
				Model:          "gpt-4",
				RequestTimeout: 5 * time.Second,
				RetryConfig: client.RetryConfig{
					MaxRetries:           0, // Disable retries for simpler tests
					RetryableStatusCodes: []int{429, 500, 502, 503, 504},
				},
			}

			c, err := NewClient(cfg)
			require.NoError(t, err)

			ctx := test.ContextWithTimeout(t, 5*time.Second)
			resp, err := c.Complete(ctx, tt.request)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.IsType(t, tt.errType, err)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.mockResponse.ID, resp.ID)
				assert.Equal(t, tt.mockResponse.Model, resp.Model)
				assert.Len(t, resp.Choices, len(tt.mockResponse.Choices))
			}
		})
	}
}

// TestStream tests streaming completions.
func TestStream(t *testing.T) {
	t.Run("successful stream", func(t *testing.T) {
		mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
			// Verify streaming is requested
			var req client.ChatCompletionRequest
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &req)
			assert.True(t, req.Stream)

			// Send SSE stream
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			flusher := w.(http.Flusher)

			// Send initial chunk
			w.Write([]byte("data: " + mustMarshal(t, client.ChatCompletionChunk{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   "gpt-4",
				Choices: []client.ChunkChoice{
					{
						Index: 0,
						Delta: client.MessageDelta{
							Role:    "assistant",
							Content: "",
						},
					},
				},
			}) + "\n\n"))
			flusher.Flush()

			// Send content chunks
			for _, text := range []string{"Hello", " ", "world", "!"} {
				w.Write([]byte("data: " + mustMarshal(t, client.ChatCompletionChunk{
					ID:      "chatcmpl-123",
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   "gpt-4",
					Choices: []client.ChunkChoice{
						{
							Index: 0,
							Delta: client.MessageDelta{
								Content: text,
							},
						},
					},
				}) + "\n\n"))
				flusher.Flush()
				time.Sleep(10 * time.Millisecond)
			}

			// Send final chunk
			w.Write([]byte("data: " + mustMarshal(t, client.ChatCompletionChunk{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   "gpt-4",
				Choices: []client.ChunkChoice{
					{
						Index:        0,
						Delta:        client.MessageDelta{},
						FinishReason: "stop",
					},
				},
				Usage: &client.TokenUsage{
					InputTokens:  10,
					OutputTokens: 5,
					TotalTokens:  15,
				},
			}) + "\n\n"))
			flusher.Flush()

			// Send done signal
			w.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()
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
				client.NewUserMessage("Hello"),
			},
		}

		eventCh, err := c.Stream(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, eventCh)

		var events []client.StreamEvent
		for event := range eventCh {
			events = append(events, event)
			if event.Error != nil {
				t.Fatalf("unexpected error in stream: %v", event.Error)
			}
		}

		assert.NotEmpty(t, events)
		// Should have text deltas and completion event
		hasTextDelta := false
		hasCompleted := false
		for _, e := range events {
			if e.Type == client.EventTypeOutputTextDelta {
				hasTextDelta = true
			}
			if e.Type == client.EventTypeCompleted {
				hasCompleted = true
			}
		}
		assert.True(t, hasTextDelta, "should have text delta events")
		assert.True(t, hasCompleted, "should have completion event")
	})

	t.Run("stream with context cancellation", func(t *testing.T) {
		mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			// Send one chunk then keep connection open
			w.Write([]byte("data: " + mustMarshal(t, client.ChatCompletionChunk{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   "gpt-4",
				Choices: []client.ChunkChoice{
					{
						Index: 0,
						Delta: client.MessageDelta{
							Role:    "assistant",
							Content: "Hello",
						},
					},
				},
			}) + "\n\n"))
			w.(http.Flusher).Flush()

			// Block to simulate long stream
			time.Sleep(5 * time.Second)
		})

		cfg := client.ClientConfig{
			BaseURL:        mockServer.URL,
			APIKey:         "test-key",
			Model:          "gpt-4",
			RequestTimeout: 30 * time.Second,
		}

		c, err := NewClient(cfg)
		require.NoError(t, err)

		ctx, cancel := test.ContextWithCancel(t)
		req := &client.ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []client.Message{
				client.NewUserMessage("Hello"),
			},
		}

		eventCh, err := c.Stream(ctx, req)
		require.NoError(t, err)

		// Wait for first event
		event := <-eventCh
		assert.NoError(t, event.Error)

		// Cancel context
		cancel()

		// Stream should close
		remainingEvents := 0
		for range eventCh {
			remainingEvents++
		}
		// Channel should close quickly after cancellation
		assert.LessOrEqual(t, remainingEvents, 2)
	})

	t.Run("stream with idle timeout", func(t *testing.T) {
		mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			// Send one chunk then go silent
			w.Write([]byte("data: " + mustMarshal(t, client.ChatCompletionChunk{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   "gpt-4",
				Choices: []client.ChunkChoice{
					{
						Index: 0,
						Delta: client.MessageDelta{
							Role:    "assistant",
							Content: "Hello",
						},
					},
				},
			}) + "\n\n"))
			w.(http.Flusher).Flush()

			// Simulate timeout
			time.Sleep(2 * time.Second)
		})

		cfg := client.ClientConfig{
			BaseURL:        mockServer.URL,
			APIKey:         "test-key",
			Model:          "gpt-4",
			RequestTimeout: 30 * time.Second,
			StreamConfig: client.StreamConfig{
				IdleTimeout: 500 * time.Millisecond,
				BufferSize:  16,
			},
		}

		c, err := NewClient(cfg)
		require.NoError(t, err)

		ctx := test.ContextWithTimeout(t, 5*time.Second)
		req := &client.ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []client.Message{
				client.NewUserMessage("Hello"),
			},
		}

		eventCh, err := c.Stream(ctx, req)
		require.NoError(t, err)

		// Should get first event, then timeout error
		gotTimeout := false
		for event := range eventCh {
			if event.Error != nil {
				assert.IsType(t, &client.IdleTimeoutError{}, event.Error)
				gotTimeout = true
			}
		}
		assert.True(t, gotTimeout, "should have received idle timeout error")
	})
}

// TestGetModelContextWindow tests context window retrieval.
func TestGetModelContextWindow(t *testing.T) {
	tests := []struct {
		name           string
		model          string
		expectedWindow int64
	}{
		{
			name:           "gpt-4",
			model:          "gpt-4",
			expectedWindow: 8192,
		},
		{
			name:           "gpt-4-turbo",
			model:          "gpt-4-turbo",
			expectedWindow: 128000,
		},
		{
			name:           "gpt-3.5-turbo",
			model:          "gpt-3.5-turbo",
			expectedWindow: 16385,
		},
		{
			name:           "unknown model",
			model:          "unknown-model",
			expectedWindow: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := client.ClientConfig{
				BaseURL:        "https://api.openai.com/v1",
				APIKey:         "test-key",
				Model:          tt.model,
				RequestTimeout: 30 * time.Second,
			}

			c, err := NewClient(cfg)
			require.NoError(t, err)

			window := c.GetModelContextWindow()
			assert.Equal(t, tt.expectedWindow, window)
		})
	}
}

// TestGetAutoCompactTokenLimit tests auto-compact token limit retrieval.
func TestGetAutoCompactTokenLimit(t *testing.T) {
	cfg := client.ClientConfig{
		BaseURL:        "https://api.openai.com/v1",
		APIKey:         "test-key",
		Model:          "gpt-4",
		RequestTimeout: 30 * time.Second,
	}

	c, err := NewClient(cfg)
	require.NoError(t, err)

	limit := c.GetAutoCompactTokenLimit()
	// For gpt-4, should be a reasonable default (e.g., 80% of context window)
	assert.Greater(t, limit, int64(0))
	assert.Less(t, limit, c.GetModelContextWindow())
}

// TestRateLimitParsing tests extraction of rate limit headers.
func TestRateLimitParsing(t *testing.T) {
	mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-limit-requests", "10000")
		w.Header().Set("x-ratelimit-remaining-requests", "9999")
		w.Header().Set("x-ratelimit-reset-requests", "60s")
		w.Header().Set("x-ratelimit-limit-tokens", "1000000")
		w.Header().Set("x-ratelimit-remaining-tokens", "900000")
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
						Content: "Test",
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

	resp, err := c.Complete(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Rate limits should be tracked internally
	// This would require exposing rate limit state or adding a getter method
}

// Helper function to marshal JSON for tests
func mustMarshal(t *testing.T, v interface{}) string {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return string(data)
}

// mockHTTPClient is a simple HTTP client mock for testing
type mockHTTPClient struct {
	doFunc func(req *client.HTTPRequest) (*client.HTTPResponse, error)
}

func (m *mockHTTPClient) Do(req *client.HTTPRequest) (*client.HTTPResponse, error) {
	return m.doFunc(req)
}

// TestHTTPClientIntegration tests using a custom HTTP client
func TestHTTPClientIntegration(t *testing.T) {
	t.Run("custom HTTP client", func(t *testing.T) {
		mockResp := &client.ChatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4",
			Choices: []client.Choice{
				{
					Index: 0,
					Message: client.Message{
						Role:    "assistant",
						Content: "Custom client response",
					},
					FinishReason: "stop",
				},
			},
		}

		mockClient := &mockHTTPClient{
			doFunc: func(req *client.HTTPRequest) (*client.HTTPResponse, error) {
				// Verify request
				assert.Equal(t, "POST", req.Method)
				assert.Contains(t, req.URL, "/chat/completions")
				assert.Equal(t, "Bearer test-key", req.Headers["Authorization"])

				// Return mock response
				body, _ := json.Marshal(mockResp)
				return &client.HTTPResponse{
					StatusCode: http.StatusOK,
					Headers:    make(map[string][]string),
					Body:       io.NopCloser(bytes.NewReader(body)),
				}, nil
			},
		}

		cfg := client.ClientConfig{
			BaseURL:        "https://api.openai.com/v1",
			APIKey:         "test-key",
			Model:          "gpt-4",
			HTTPClient:     mockClient,
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

		resp, err := c.Complete(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "Custom client response", resp.Choices[0].Message.Content)
	})
}

// TestRequestTimeout tests request timeout handling
func TestRequestTimeout(t *testing.T) {
	mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Delay response to trigger timeout
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&client.ChatCompletionResponse{
			ID: "chatcmpl-123",
		})
	})

	cfg := client.ClientConfig{
		BaseURL:        mockServer.URL,
		APIKey:         "test-key",
		Model:          "gpt-4",
		RequestTimeout: 500 * time.Millisecond,
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
	assert.Error(t, err)
	// Should be a timeout or context error
}

// Test401WithoutRefreshFunc tests handling 401 without token refresh configured
func Test401WithoutRefreshFunc(t *testing.T) {
	mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid API key",
				"type":    "invalid_request_error",
			},
		})
	})

	cfg := client.ClientConfig{
		BaseURL:        mockServer.URL,
		APIKey:         "invalid-key",
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

	resp, err := c.Complete(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.IsType(t, &client.UnauthorizedError{}, err)

	unauthorizedErr := err.(*client.UnauthorizedError)
	assert.False(t, unauthorizedErr.CanRefresh, "should indicate refresh is not available")
}

// Test401WithSuccessfulRefresh tests successful token refresh on 401
func Test401WithSuccessfulRefresh(t *testing.T) {
	requestCount := 0
	mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		authHeader := r.Header.Get("Authorization")

		if authHeader == "Bearer old-token" {
			// First request with old token - return 401
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Invalid or expired token",
					"type":    "invalid_request_error",
				},
			})
		} else if authHeader == "Bearer new-token" {
			// Second request with new token - return success
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
							Content: "Success with refreshed token!",
						},
						FinishReason: "stop",
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		} else {
			// Unexpected token
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Unexpected token",
					"type":    "invalid_request_error",
				},
			})
		}
	})

	cfg := client.ClientConfig{
		BaseURL:        mockServer.URL,
		APIKey:         "old-token",
		Model:          "gpt-4",
		RequestTimeout: 5 * time.Second,
		TokenRefreshFunc: func(ctx context.Context, oldToken string) (string, error) {
			assert.Equal(t, "old-token", oldToken)
			return "new-token", nil
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

	resp, err := c.Complete(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Success with refreshed token!", resp.Choices[0].Message.Content)
	assert.Equal(t, 2, requestCount, "should have made exactly 2 requests (initial + retry)")
}

// Test401WithFailedRefresh tests failed token refresh on 401
func Test401WithFailedRefresh(t *testing.T) {
	mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid token",
				"type":    "invalid_request_error",
			},
		})
	})

	cfg := client.ClientConfig{
		BaseURL:        mockServer.URL,
		APIKey:         "old-token",
		Model:          "gpt-4",
		RequestTimeout: 5 * time.Second,
		TokenRefreshFunc: func(ctx context.Context, oldToken string) (string, error) {
			return "", fmt.Errorf("refresh service unavailable")
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

	resp, err := c.Complete(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "token refresh failed")
	assert.Contains(t, err.Error(), "refresh service unavailable")
}

// Test401RefreshStillUnauthorized tests when refresh succeeds but new token is also invalid
func Test401RefreshStillUnauthorized(t *testing.T) {
	requestCount := 0
	mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		// Always return 401, even for refreshed token
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid token",
				"type":    "invalid_request_error",
			},
		})
	})

	cfg := client.ClientConfig{
		BaseURL:        mockServer.URL,
		APIKey:         "old-token",
		Model:          "gpt-4",
		RequestTimeout: 5 * time.Second,
		TokenRefreshFunc: func(ctx context.Context, oldToken string) (string, error) {
			return "new-token", nil
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

	resp, err := c.Complete(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.IsType(t, &client.UnauthorizedError{}, err)
	assert.Equal(t, 2, requestCount, "should have made exactly 2 requests (initial + retry)")
}

// TestStreamWith401AndSuccessfulRefresh tests streaming with token refresh
func TestStreamWith401AndSuccessfulRefresh(t *testing.T) {
	requestCount := 0
	mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		authHeader := r.Header.Get("Authorization")

		if authHeader == "Bearer old-token" {
			// First request with old token - return 401
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Invalid or expired token",
					"type":    "invalid_request_error",
				},
			})
		} else if authHeader == "Bearer new-token" {
			// Second request with new token - return success stream
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			flusher := w.(http.Flusher)

			// Send initial chunk
			w.Write([]byte("data: " + mustMarshal(t, client.ChatCompletionChunk{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   "gpt-4",
				Choices: []client.ChunkChoice{
					{
						Index: 0,
						Delta: client.MessageDelta{
							Role:    "assistant",
							Content: "Refreshed!",
						},
					},
				},
			}) + "\n\n"))
			flusher.Flush()

			// Send done signal
			w.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()
		}
	})

	cfg := client.ClientConfig{
		BaseURL:        mockServer.URL,
		APIKey:         "old-token",
		Model:          "gpt-4",
		RequestTimeout: 5 * time.Second,
		TokenRefreshFunc: func(ctx context.Context, oldToken string) (string, error) {
			return "new-token", nil
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

	eventCh, err := c.Stream(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, eventCh)

	var events []client.StreamEvent
	for event := range eventCh {
		events = append(events, event)
		if event.Error != nil {
			t.Fatalf("unexpected error in stream: %v", event.Error)
		}
	}

	assert.NotEmpty(t, events)
	assert.Equal(t, 2, requestCount, "should have made exactly 2 requests (initial + retry)")

	// Verify we got the content from the refreshed token
	hasExpectedContent := false
	for _, e := range events {
		if e.Type == client.EventTypeOutputTextDelta {
			delta, ok := e.Data.(string)
			if ok && strings.Contains(delta, "Refreshed") {
				hasExpectedContent = true
			}
		}
	}
	assert.True(t, hasExpectedContent, "should have received content from refreshed token")
}

// TestConcurrentRequestsWithTokenRefresh tests thread-safety of token refresh
func TestConcurrentRequestsWithTokenRefresh(t *testing.T) {
	requestCount := 0
	var requestMutex sync.Mutex

	mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestMutex.Lock()
		requestCount++
		localCount := requestCount
		requestMutex.Unlock()

		authHeader := r.Header.Get("Authorization")

		// First 3 requests get 401, rest get success
		if localCount <= 3 && authHeader == "Bearer old-token" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Invalid token",
					"type":    "invalid_request_error",
				},
			})
		} else if authHeader == "Bearer new-token" {
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
							Content: "Success",
						},
						FinishReason: "stop",
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Unexpected state",
					"type":    "invalid_request_error",
				},
			})
		}
	})

	refreshCount := 0
	var refreshMutex sync.Mutex

	cfg := client.ClientConfig{
		BaseURL:        mockServer.URL,
		APIKey:         "old-token",
		Model:          "gpt-4",
		RequestTimeout: 5 * time.Second,
		TokenRefreshFunc: func(ctx context.Context, oldToken string) (string, error) {
			refreshMutex.Lock()
			refreshCount++
			refreshMutex.Unlock()
			// Simulate refresh delay
			time.Sleep(100 * time.Millisecond)
			return "new-token", nil
		},
	}

	c, err := NewClient(cfg)
	require.NoError(t, err)

	// Make 3 concurrent requests
	var wg sync.WaitGroup
	errors := make([]error, 3)
	responses := make([]*client.ChatCompletionResponse, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx := test.ContextWithTimeout(t, 10*time.Second)
			req := &client.ChatCompletionRequest{
				Model: "gpt-4",
				Messages: []client.Message{
					client.NewUserMessage("Test"),
				},
			}
			responses[idx], errors[idx] = c.Complete(ctx, req)
		}(i)
	}

	wg.Wait()

	// All requests should succeed
	for i := 0; i < 3; i++ {
		assert.NoError(t, errors[i], "request %d should succeed", i)
		assert.NotNil(t, responses[i], "request %d should have response", i)
	}

	// Token refresh should have happened at least once
	assert.Greater(t, refreshCount, 0, "token refresh should have been called")
}

// TestResponseBodyLeakOnTokenRefresh verifies that response bodies are properly closed
// during token refresh scenarios to prevent resource leaks.
func TestResponseBodyLeakOnTokenRefresh(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "Bearer old-token" {
			// First request with old token - return 401
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":{"message":"Invalid token"}}`))
			return
		}
		// Second request with new token - return success
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(client.ChatCompletionResponse{
			ID:    "test-id",
			Model: "gpt-4",
			Choices: []client.Choice{
				{
					Index: 0,
					Message: client.Message{
						Role:    "assistant",
						Content: "Success",
					},
					FinishReason: "stop",
				},
			},
		})
	}))
	defer mockServer.Close()

	cfg := client.ClientConfig{
		BaseURL:        mockServer.URL,
		APIKey:         "old-token",
		Model:          "gpt-4",
		RequestTimeout: 5 * time.Second,
		TokenRefreshFunc: func(ctx context.Context, oldToken string) (string, error) {
			return "new-token", nil
		},
	}

	c, err := NewClient(cfg)
	require.NoError(t, err)

	// Get initial metrics
	initialStats := c.GetConnectionPoolStats()
	require.NotNil(t, initialStats, "metrics should be available")

	ctx := test.ContextWithTimeout(t, 10*time.Second)
	req := &client.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []client.Message{
			client.NewUserMessage("Test"),
		},
	}

	resp, err := c.Complete(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Get final metrics
	finalStats := c.GetConnectionPoolStats()

	// Verify that all response bodies were closed
	// We expect: 1 failed request (401) + 1 successful request = 2 total bodies
	assert.Equal(t, int64(2), finalStats.TotalResponseBodies, "should have 2 response bodies total")
	assert.Equal(t, int64(2), finalStats.ClosedResponseBodies, "all response bodies should be closed")
	assert.Equal(t, int64(0), finalStats.OpenResponseBodies, "no response bodies should remain open")
	assert.Equal(t, int64(0), finalStats.LeakedResponseBodies, "no response bodies should be leaked")
}

// TestResponseBodyLeakOnStreamingTokenRefresh verifies that streaming response bodies
// are properly closed during token refresh scenarios.
func TestResponseBodyLeakOnStreamingTokenRefresh(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "Bearer old-token" {
			// First request with old token - return 401
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":{"message":"Invalid token"}}`))
			return
		}
		// Second request with new token - return streaming response
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		require.True(t, ok, "ResponseWriter should support flushing")

		// Send streaming chunks
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"1","choices":[{"delta":{"role":"assistant"},"index":0}]}`)
		flusher.Flush()

		fmt.Fprintf(w, "data: %s\n\n", `{"id":"1","choices":[{"delta":{"content":"Hello"},"index":0}]}`)
		flusher.Flush()

		fmt.Fprintf(w, "data: %s\n\n", `{"id":"1","choices":[{"delta":{"content":" World"},"index":0,"finish_reason":"stop"}]}`)
		flusher.Flush()

		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer mockServer.Close()

	cfg := client.ClientConfig{
		BaseURL:        mockServer.URL,
		APIKey:         "old-token",
		Model:          "gpt-4",
		RequestTimeout: 5 * time.Second,
		TokenRefreshFunc: func(ctx context.Context, oldToken string) (string, error) {
			return "new-token", nil
		},
	}

	c, err := NewClient(cfg)
	require.NoError(t, err)

	// Get initial metrics
	initialStats := c.GetConnectionPoolStats()
	require.NotNil(t, initialStats, "metrics should be available")

	ctx := test.ContextWithTimeout(t, 10*time.Second)
	req := &client.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []client.Message{
			client.NewUserMessage("Test"),
		},
	}

	eventCh, err := c.Stream(ctx, req)
	require.NoError(t, err)

	// Consume all events
	eventCount := 0
	for event := range eventCh {
		if event.Type == client.EventTypeError {
			t.Fatalf("unexpected error event: %v", event.Error)
		}
		eventCount++
	}

	assert.Greater(t, eventCount, 0, "should receive events")

	// Get final metrics
	finalStats := c.GetConnectionPoolStats()

	// Verify that all response bodies were closed
	// We expect: 1 failed request (401) + 1 successful streaming request = 2 total bodies
	assert.Equal(t, int64(2), finalStats.TotalResponseBodies, "should have 2 response bodies total")
	assert.Equal(t, int64(2), finalStats.ClosedResponseBodies, "all response bodies should be closed")
	assert.Equal(t, int64(0), finalStats.OpenResponseBodies, "no response bodies should remain open")
	assert.Equal(t, int64(0), finalStats.LeakedResponseBodies, "no response bodies should be leaked")
}

// TestMultipleTokenRefreshesNoLeak verifies that multiple token refresh attempts
// don't accumulate leaked response bodies.
func TestMultipleTokenRefreshesNoLeak(t *testing.T) {
	var requestMutex sync.Mutex
	requestCount := 0

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMutex.Lock()
		requestCount++
		currentCount := requestCount
		requestMutex.Unlock()

		auth := r.Header.Get("Authorization")

		// Each "old-token-X" gets a 401, each "new-token-X" succeeds
		if auth == "Bearer old-token-1" || auth == "Bearer old-token-2" || auth == "Bearer old-token-3" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":{"message":"Invalid token"}}`))
			return
		}

		// All other tokens succeed
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(client.ChatCompletionResponse{
			ID:    fmt.Sprintf("test-id-%d", currentCount),
			Model: "gpt-4",
			Choices: []client.Choice{
				{
					Index: 0,
					Message: client.Message{
						Role:    "assistant",
						Content: "Success",
					},
					FinishReason: "stop",
				},
			},
		})
	}))
	defer mockServer.Close()

	var refreshMutex sync.Mutex
	refreshCount := 0

	cfg := client.ClientConfig{
		BaseURL:        mockServer.URL,
		APIKey:         "old-token-1",
		Model:          "gpt-4",
		RequestTimeout: 5 * time.Second,
		TokenRefreshFunc: func(ctx context.Context, oldToken string) (string, error) {
			refreshMutex.Lock()
			refreshCount++
			count := refreshCount
			refreshMutex.Unlock()
			return fmt.Sprintf("new-token-%d", count), nil
		},
	}

	c, err := NewClient(cfg)
	require.NoError(t, err)

	ctx := test.ContextWithTimeout(t, 10*time.Second)

	// Make 3 requests, each will initially fail and trigger token refresh
	for i := 0; i < 3; i++ {
		// Reset token to old one to force refresh
		c.updateToken(fmt.Sprintf("old-token-%d", i+1))

		req := &client.ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []client.Message{
				client.NewUserMessage("Test"),
			},
		}

		resp, err := c.Complete(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	}

	// Get final metrics
	finalStats := c.GetConnectionPoolStats()

	// Verify no leaks
	// Each iteration: 1 failed request (401) + 1 successful retry = 2 bodies
	// Total: 3 iterations * 2 = 6 bodies
	assert.Equal(t, int64(6), finalStats.TotalResponseBodies, "should have 6 response bodies total")
	assert.Equal(t, finalStats.TotalResponseBodies, finalStats.ClosedResponseBodies,
		"all response bodies should be closed")
	assert.Equal(t, int64(0), finalStats.OpenResponseBodies, "no response bodies should remain open")
	assert.Equal(t, int64(0), finalStats.LeakedResponseBodies, "no response bodies should be leaked")

	// We should have had 3 refresh attempts
	assert.Equal(t, 3, refreshCount, "token refresh should have been called 3 times")
}
