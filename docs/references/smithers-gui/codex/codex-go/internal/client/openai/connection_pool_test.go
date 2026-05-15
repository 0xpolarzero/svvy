package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/client"
	"github.com/evmts/codex/codex-go/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectionPoolConfiguration tests connection pool configuration.
func TestConnectionPoolConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		poolConfig client.ConnectionPoolConfig
		wantErr    bool
	}{
		{
			name: "default configuration",
			poolConfig: client.ConnectionPoolConfig{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				MaxConnsPerHost:     0,
				IdleConnTimeout:     90 * time.Second,
				ConnectionTimeout:   10 * time.Second,
				EnableHTTP2:         true,
			},
			wantErr: false,
		},
		{
			name: "custom limited pool",
			poolConfig: client.ConnectionPoolConfig{
				MaxIdleConns:        50,
				MaxIdleConnsPerHost: 5,
				MaxConnsPerHost:     10,
				IdleConnTimeout:     60 * time.Second,
				ConnectionTimeout:   5 * time.Second,
				EnableHTTP2:         false,
			},
			wantErr: false,
		},
		{
			name: "large pool for high throughput",
			poolConfig: client.ConnectionPoolConfig{
				MaxIdleConns:        200,
				MaxIdleConnsPerHost: 20,
				MaxConnsPerHost:     0,
				IdleConnTimeout:     120 * time.Second,
				ConnectionTimeout:   15 * time.Second,
				EnableHTTP2:         true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := test.NewJSONMockServer(t, http.StatusOK, &client.ChatCompletionResponse{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "gpt-4",
				Choices: []client.Choice{
					{
						Index: 0,
						Message: client.Message{
							Role:    "assistant",
							Content: "Test response",
						},
						FinishReason: "stop",
					},
				},
			})

			cfg := client.ClientConfig{
				BaseURL:              mockServer.URL,
				APIKey:               "test-key",
				Model:                "gpt-4",
				RequestTimeout:       5 * time.Second,
				ConnectionPoolConfig: tt.poolConfig,
			}

			c, err := NewClient(cfg)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, c)

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
		})
	}
}

// TestConcurrentRequestsWithConnectionPool tests concurrent requests with connection pooling.
func TestConcurrentRequestsWithConnectionPool(t *testing.T) {
	requestCount := 0
	var requestMutex sync.Mutex

	mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestMutex.Lock()
		requestCount++
		requestMutex.Unlock()

		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)

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
	})

	cfg := client.ClientConfig{
		BaseURL:        mockServer.URL,
		APIKey:         "test-key",
		Model:          "gpt-4",
		RequestTimeout: 5 * time.Second,
		ConnectionPoolConfig: client.ConnectionPoolConfig{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			MaxConnsPerHost:     0,
			IdleConnTimeout:     90 * time.Second,
			ConnectionTimeout:   10 * time.Second,
			EnableHTTP2:         true,
		},
	}

	c, err := NewClient(cfg)
	require.NoError(t, err)

	// Make 20 concurrent requests
	concurrency := 20
	var wg sync.WaitGroup
	errors := make([]error, concurrency)
	responses := make([]*client.ChatCompletionResponse, concurrency)

	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
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
	elapsed := time.Since(startTime)

	// All requests should succeed
	for i := 0; i < concurrency; i++ {
		assert.NoError(t, errors[i], "request %d should succeed", i)
		assert.NotNil(t, responses[i], "request %d should have response", i)
	}

	// Verify all requests were processed
	assert.Equal(t, concurrency, requestCount, "should have processed all requests")

	// With connection pooling, concurrent requests should complete faster
	// Each request takes ~10ms, so serial would take ~200ms
	// With pooling and concurrency, should be much faster
	assert.Less(t, elapsed, 100*time.Millisecond, "concurrent requests should complete faster with pooling")

	// Check metrics if available
	if stats := c.GetConnectionPoolStats(); stats != nil {
		t.Logf("Connection pool stats: %+v", stats)
		assert.Equal(t, int64(concurrency), stats.TotalRequests)
		assert.GreaterOrEqual(t, stats.ConnectionReuses+stats.NewConnections, int64(0))
	}
}

// TestConnectionPoolMetrics tests metrics tracking.
func TestConnectionPoolMetrics(t *testing.T) {
	mockServer := test.NewJSONMockServer(t, http.StatusOK, &client.ChatCompletionResponse{
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
	})

	cfg := client.ClientConfig{
		BaseURL:        mockServer.URL,
		APIKey:         "test-key",
		Model:          "gpt-4",
		RequestTimeout: 5 * time.Second,
		ConnectionPoolConfig: client.ConnectionPoolConfig{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			ConnectionTimeout:   10 * time.Second,
			EnableHTTP2:         true,
		},
	}

	c, err := NewClient(cfg)
	require.NoError(t, err)

	ctx := test.ContextWithTimeout(t, 5*time.Second)

	// Make multiple requests
	for i := 0; i < 5; i++ {
		req := &client.ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []client.Message{
				client.NewUserMessage("Test"),
			},
		}
		_, err := c.Complete(ctx, req)
		require.NoError(t, err)
	}

	// Get metrics
	stats := c.GetConnectionPoolStats()
	require.NotNil(t, stats, "metrics should be available")

	// Verify metrics
	assert.Equal(t, int64(5), stats.TotalRequests, "should track 5 requests")
	assert.GreaterOrEqual(t, stats.ConnectionReuses+stats.NewConnections, int64(0))
	assert.GreaterOrEqual(t, stats.ReuseRate, 0.0)
	assert.LessOrEqual(t, stats.ReuseRate, 100.0)

	// Log stats (for visual verification during test runs)
	c.LogConnectionPoolStats()
}

// TestConnectionPoolWithCustomHTTPClient tests that metrics are not available with custom client.
func TestConnectionPoolWithCustomHTTPClient(t *testing.T) {
	mockClient := &customMockHTTPClient{
		doFunc: func(req *client.HTTPRequest) (*client.HTTPResponse, error) {
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
			body, _ := json.Marshal(resp)
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

	_, err = c.Complete(ctx, req)
	require.NoError(t, err)

	// Metrics should not be available with custom HTTP client
	stats := c.GetConnectionPoolStats()
	assert.Nil(t, stats, "metrics should not be available with custom HTTPClient")

	// Log should not panic
	c.LogConnectionPoolStats()
}

// TestConnectionReuseSequential tests connection reuse with sequential requests.
func TestConnectionReuseSequential(t *testing.T) {
	mockServer := test.NewJSONMockServer(t, http.StatusOK, &client.ChatCompletionResponse{
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
	})

	cfg := client.ClientConfig{
		BaseURL:        mockServer.URL,
		APIKey:         "test-key",
		Model:          "gpt-4",
		RequestTimeout: 5 * time.Second,
		ConnectionPoolConfig: client.ConnectionPoolConfig{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			ConnectionTimeout:   10 * time.Second,
			EnableHTTP2:         true,
		},
	}

	c, err := NewClient(cfg)
	require.NoError(t, err)

	ctx := test.ContextWithTimeout(t, 10*time.Second)

	// Make sequential requests to same host
	numRequests := 10
	for i := 0; i < numRequests; i++ {
		req := &client.ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []client.Message{
				client.NewUserMessage("Test"),
			},
		}
		_, err := c.Complete(ctx, req)
		require.NoError(t, err)
	}

	stats := c.GetConnectionPoolStats()
	require.NotNil(t, stats)

	// Should have high reuse rate for sequential requests
	assert.Equal(t, int64(numRequests), stats.TotalRequests)
	// With sequential requests to same host, we expect high connection reuse
	// (after the first connection is established)
	if stats.NewConnections+stats.ConnectionReuses > 0 {
		assert.Greater(t, stats.ReuseRate, 50.0, "should have >50% reuse rate for sequential requests")
	}
}

// TestConnectionPoolDefaultConfig tests that default config is applied.
func TestConnectionPoolDefaultConfig(t *testing.T) {
	mockServer := test.NewJSONMockServer(t, http.StatusOK, &client.ChatCompletionResponse{
		ID: "chatcmpl-123",
	})

	cfg := client.ClientConfig{
		BaseURL:        mockServer.URL,
		APIKey:         "test-key",
		Model:          "gpt-4",
		RequestTimeout: 5 * time.Second,
		// ConnectionPoolConfig intentionally left empty to test defaults
	}

	c, err := NewClient(cfg)
	require.NoError(t, err)
	require.NotNil(t, c)

	// Should have applied default connection pool config
	assert.Equal(t, 100, c.config.ConnectionPoolConfig.MaxIdleConns)
	assert.Equal(t, 10, c.config.ConnectionPoolConfig.MaxIdleConnsPerHost)
	assert.Equal(t, 90*time.Second, c.config.ConnectionPoolConfig.IdleConnTimeout)
	assert.Equal(t, 10*time.Second, c.config.ConnectionPoolConfig.ConnectionTimeout)
	assert.True(t, c.config.ConnectionPoolConfig.EnableHTTP2)
}

// BenchmarkConnectionPoolConcurrency benchmarks concurrent request throughput.
func BenchmarkConnectionPoolConcurrency(b *testing.B) {
	t := &testing.T{} // Create a testing.T for mock server
	mockServer := test.NewJSONMockServer(t, http.StatusOK, &client.ChatCompletionResponse{
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
	})

	cfg := client.ClientConfig{
		BaseURL:        mockServer.URL,
		APIKey:         "test-key",
		Model:          "gpt-4",
		RequestTimeout: 30 * time.Second,
		ConnectionPoolConfig: client.ConnectionPoolConfig{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			ConnectionTimeout:   10 * time.Second,
			EnableHTTP2:         true,
		},
	}

	c, err := NewClient(cfg)
	require.NoError(b, err)

	req := &client.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []client.Message{
			client.NewUserMessage("Test"),
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			_, err := c.Complete(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.StopTimer()
	if stats := c.GetConnectionPoolStats(); stats != nil {
		b.Logf("Final stats: requests=%d reuses=%d (%.1f%%) new=%d",
			stats.TotalRequests, stats.ConnectionReuses, stats.ReuseRate, stats.NewConnections)
	}
}

// customMockHTTPClient is a simple HTTP client mock for testing (renamed to avoid conflict)
type customMockHTTPClient struct {
	doFunc func(req *client.HTTPRequest) (*client.HTTPResponse, error)
}

func (m *customMockHTTPClient) Do(req *client.HTTPRequest) (*client.HTTPResponse, error) {
	return m.doFunc(req)
}
