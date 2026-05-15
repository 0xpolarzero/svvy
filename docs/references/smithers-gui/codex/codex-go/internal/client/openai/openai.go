// Package openai provides an OpenAI-compatible client implementation.
// It supports both Chat Completions API and Responses API with SSE streaming,
// retry logic, rate limiting, and comprehensive error handling.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/evmts/codex/codex-go/internal/client"
)

// Client implements the client.Client interface for OpenAI-compatible APIs.
type Client struct {
	config     client.ClientConfig
	httpClient client.HTTPClient
	rateLimits *rateLimitTracker
	metrics    *ConnectionPoolMetrics // Connection pool metrics (optional)

	// Model info cache
	modelContextWindow    int64
	autoCompactTokenLimit int64
	modelInfoInitialized  bool
	modelInfoOnce         sync.Once

	// Token refresh management
	tokenMutex   sync.RWMutex // Protects currentToken
	currentToken string       // Current API token (synchronized for concurrent access)
}

// NewClient creates a new OpenAI client with the given configuration.
func NewClient(cfg client.ClientConfig) (*Client, error) {
	// Validate configuration
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	// Set defaults
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 30 * time.Second
	}
	if cfg.StreamConfig.BufferSize == 0 {
		cfg.StreamConfig = client.DefaultStreamConfig()
	}
	if cfg.RetryConfig.MaxRetries == 0 && len(cfg.RetryConfig.RetryableStatusCodes) == 0 {
		cfg.RetryConfig = client.DefaultRetryConfig()
	}
	if cfg.ConnectionPoolConfig.MaxIdleConns == 0 {
		cfg.ConnectionPoolConfig = client.DefaultConnectionPoolConfig()
	}

	// Create default HTTP client if not provided
	var poolMetrics *ConnectionPoolMetrics
	if cfg.HTTPClient == nil {
		poolMetrics = newConnectionPoolMetrics(cfg.ConnectionPoolConfig.MaxIdleConnsPerHost)
		cfg.HTTPClient = newDefaultHTTPClientWithMetrics(cfg.RequestTimeout, cfg.ConnectionPoolConfig, poolMetrics)
	}

	c := &Client{
		config:       cfg,
		httpClient:   cfg.HTTPClient,
		rateLimits:   newRateLimitTracker(),
		metrics:      poolMetrics, // May be nil if custom HTTPClient provided
		currentToken: cfg.APIKey,  // Initialize with the provided API key
	}

	// Initialize model info
	c.initModelInfo()

	return c, nil
}

// Complete performs a non-streaming chat completion request.
func (c *Client) Complete(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) {
	// Ensure stream is false
	req.Stream = false

	// Apply retry logic
	return c.completeWithRetry(ctx, req)
}

// Stream initiates a streaming chat completion request.
func (c *Client) Stream(ctx context.Context, req *client.ChatCompletionRequest) (<-chan client.StreamEvent, error) {
	// Ensure stream is true
	req.Stream = true

	// Create event channel
	eventCh := make(chan client.StreamEvent, c.config.StreamConfig.BufferSize)

	// Start streaming in background
	go c.streamWithRetry(ctx, req, eventCh)

	return eventCh, nil
}

// GetModelContextWindow returns the effective context window for the model.
func (c *Client) GetModelContextWindow() int64 {
	c.modelInfoOnce.Do(func() {
		c.initModelInfo()
	})
	return c.modelContextWindow
}

// GetAutoCompactTokenLimit returns the token limit for auto-compaction.
func (c *Client) GetAutoCompactTokenLimit() int64 {
	c.modelInfoOnce.Do(func() {
		c.initModelInfo()
	})
	return c.autoCompactTokenLimit
}

// initModelInfo initializes model-specific information.
func (c *Client) initModelInfo() {
	info := getModelInfo(c.config.Model)
	c.modelContextWindow = info.contextWindow

	// Auto-compact at 80% of context window by default
	if info.contextWindow > 0 {
		c.autoCompactTokenLimit = int64(float64(info.contextWindow) * 0.8)
	}

	c.modelInfoInitialized = true
}

// getToken returns the current API token (thread-safe).
func (c *Client) getToken() string {
	c.tokenMutex.RLock()
	defer c.tokenMutex.RUnlock()
	return c.currentToken
}

// updateToken updates the current API token (thread-safe).
func (c *Client) updateToken(newToken string) {
	c.tokenMutex.Lock()
	defer c.tokenMutex.Unlock()
	c.currentToken = newToken
}

// completeWithRetry performs a non-streaming request with retry logic.
func (c *Client) completeWithRetry(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) {
	var lastErr error
	var lastStatusCode int

	for attempt := 0; attempt <= c.config.RetryConfig.MaxRetries; attempt++ {
		// Check context before attempting
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// Wait for backoff if this is a retry
		if attempt > 0 {
			backoff := calculateBackoff(
				attempt-1,
				c.config.RetryConfig.InitialBackoff,
				c.config.RetryConfig.MaxBackoff,
				c.config.RetryConfig.BackoffMultiplier,
			)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		// Attempt request
		resp, statusCode, err := c.doComplete(ctx, req)
		if err == nil {
			return resp, nil
		}

		_ = statusCode // Use statusCode to avoid unused variable

		lastErr = err
		lastStatusCode = statusCode

		// Check if error is retryable
		if !c.isRetryable(statusCode) {
			return nil, err
		}

		// If this was the last attempt, return appropriate error
		if attempt == c.config.RetryConfig.MaxRetries {
			// If no retries configured, return original error
			if c.config.RetryConfig.MaxRetries == 0 {
				return nil, err
			}
			return nil, client.NewRetryLimitError(statusCode, attempt+1)
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return nil, client.NewRetryLimitError(lastStatusCode, c.config.RetryConfig.MaxRetries+1)
}

// doComplete performs a single completion request without retry.
//
// Resource Management:
// This function carefully manages HTTP response body lifecycle to prevent resource leaks.
// Response bodies must be closed to release network connections back to the pool.
// During token refresh, the first response body is explicitly closed before the retry
// to avoid shadowing the deferred close statement. See RESOURCE_MANAGEMENT.md for details.
func (c *Client) doComplete(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, int, error) {
	// Build request
	httpReq, err := c.buildRequest(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	// Execute request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, 0, client.NewConnectionError(c.config.BaseURL, err)
	}
	defer httpResp.Body.Close()

	// Update rate limits
	c.updateRateLimits(httpResp.Headers)

	// Check for 401 Unauthorized and attempt token refresh if available
	if httpResp.StatusCode == http.StatusUnauthorized && c.config.TokenRefreshFunc != nil {
		_, readErr := io.ReadAll(httpResp.Body)
		if readErr != nil {
			return nil, httpResp.StatusCode, fmt.Errorf("failed to read error response body: %w", readErr)
		}

		// Explicitly close the first response body to prevent resource leak
		// before making the retry request. This must be done before the retry
		// because the deferred close at line 225 would be shadowed by a new defer.
		httpResp.Body.Close()

		// Attempt token refresh
		oldToken := c.getToken()
		newToken, refreshErr := c.config.TokenRefreshFunc(ctx, oldToken)
		if refreshErr != nil {
			// Refresh failed, return authorization error with refresh failure
			return nil, httpResp.StatusCode, fmt.Errorf("token refresh failed: %w", refreshErr)
		}

		// Update token
		c.updateToken(newToken)

		// Retry the request with new token
		httpReq, err = c.buildRequest(ctx, req)
		if err != nil {
			return nil, 0, err
		}

		httpResp, err = c.httpClient.Do(httpReq)
		if err != nil {
			return nil, 0, client.NewConnectionError(c.config.BaseURL, err)
		}
		defer httpResp.Body.Close()

		// Update rate limits from retry
		c.updateRateLimits(httpResp.Headers)
	}

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(httpResp.Body)
		if readErr != nil {
			return nil, httpResp.StatusCode, fmt.Errorf("failed to read error response body: %w", readErr)
		}
		return nil, httpResp.StatusCode, c.handleErrorResponse(httpResp.StatusCode, body)
	}

	// Parse response
	var resp client.ChatCompletionResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, httpResp.StatusCode, fmt.Errorf("failed to decode response: %w", err)
	}

	return &resp, httpResp.StatusCode, nil
}

// streamWithRetry performs streaming with retry logic.
func (c *Client) streamWithRetry(ctx context.Context, req *client.ChatCompletionRequest, eventCh chan<- client.StreamEvent) {
	defer close(eventCh)

	var lastErr error

	for attempt := 0; attempt <= c.config.RetryConfig.MaxRetries; attempt++ {
		// Check context
		if err := ctx.Err(); err != nil {
			eventCh <- client.StreamEvent{
				Type:  client.EventTypeError,
				Error: err,
			}
			return
		}

		// Wait for backoff if retry
		if attempt > 0 {
			backoff := calculateBackoff(
				attempt-1,
				c.config.RetryConfig.InitialBackoff,
				c.config.RetryConfig.MaxBackoff,
				c.config.RetryConfig.BackoffMultiplier,
			)

			select {
			case <-ctx.Done():
				eventCh <- client.StreamEvent{
					Type:  client.EventTypeError,
					Error: ctx.Err(),
				}
				return
			case <-time.After(backoff):
			}
		}

		// Attempt streaming
		err, statusCode := c.doStream(ctx, req, eventCh)
		if err == nil {
			return // Stream completed successfully
		}

		lastErr = err

		// Check if retryable
		if !c.isRetryable(statusCode) {
			eventCh <- client.StreamEvent{
				Type:  client.EventTypeError,
				Error: err,
			}
			return
		}

		// Last attempt?
		if attempt == c.config.RetryConfig.MaxRetries {
			eventCh <- client.StreamEvent{
				Type:  client.EventTypeError,
				Error: client.NewRetryLimitError(statusCode, attempt+1),
			}
			return
		}
	}

	// Should not reach here
	if lastErr != nil {
		eventCh <- client.StreamEvent{
			Type:  client.EventTypeError,
			Error: lastErr,
		}
	}
}

// doStream performs a single streaming request.
//
// Resource Management:
// Like doComplete, this function manages HTTP response body lifecycle carefully.
// During token refresh, the first response body is explicitly closed before retry.
// The streaming parser manages its scanner goroutine lifecycle to prevent leaks.
// See RESOURCE_MANAGEMENT.md for details on streaming resource management.
func (c *Client) doStream(ctx context.Context, req *client.ChatCompletionRequest, eventCh chan<- client.StreamEvent) (error, int) {
	// Build request
	httpReq, err := c.buildRequest(ctx, req)
	if err != nil {
		return err, 0
	}

	// Execute request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return client.NewConnectionError(c.config.BaseURL, err), 0
	}
	defer httpResp.Body.Close()

	// Update rate limits
	c.updateRateLimits(httpResp.Headers)

	// Check for 401 Unauthorized and attempt token refresh if available
	if httpResp.StatusCode == http.StatusUnauthorized && c.config.TokenRefreshFunc != nil {
		// Read and drain the first response body before closing
		_, readErr := io.ReadAll(httpResp.Body)
		if readErr != nil {
			return fmt.Errorf("failed to read error response body: %w", readErr), httpResp.StatusCode
		}

		// Explicitly close the first response body to prevent resource leak
		// before making the retry request. This must be done before the retry
		// because the deferred close at line 367 would be shadowed by a new defer.
		httpResp.Body.Close()

		// Attempt token refresh
		oldToken := c.getToken()
		newToken, refreshErr := c.config.TokenRefreshFunc(ctx, oldToken)
		if refreshErr != nil {
			// Refresh failed, return authorization error with refresh failure
			return fmt.Errorf("token refresh failed: %w", refreshErr), httpResp.StatusCode
		}

		// Update token
		c.updateToken(newToken)

		// Retry the request with new token
		httpReq, err = c.buildRequest(ctx, req)
		if err != nil {
			return err, 0
		}

		httpResp, err = c.httpClient.Do(httpReq)
		if err != nil {
			return client.NewConnectionError(c.config.BaseURL, err), 0
		}
		defer httpResp.Body.Close()

		// Update rate limits from retry
		c.updateRateLimits(httpResp.Headers)
	}

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(httpResp.Body)
		if readErr != nil {
			return fmt.Errorf("failed to read error response body: %w", readErr), httpResp.StatusCode
		}
		err := c.handleErrorResponse(httpResp.StatusCode, body)
		return err, httpResp.StatusCode
	}

	// Parse SSE stream
	parser := newStreamParser(c.config.StreamConfig)
	if err := parser.parse(ctx, httpResp.Body, eventCh); err != nil {
		return err, httpResp.StatusCode
	}

	return nil, httpResp.StatusCode
}

// buildRequest constructs an HTTP request for the API.
func (c *Client) buildRequest(ctx context.Context, req *client.ChatCompletionRequest) (*client.HTTPRequest, error) {
	// Set model if not set
	if req.Model == "" {
		req.Model = c.config.Model
	}

	// Marshal request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build headers
	headers := make(map[string]string)
	headers["Content-Type"] = "application/json"
	headers["Authorization"] = "Bearer " + c.getToken() // Use current token (thread-safe)

	// Add conversation ID if present
	if c.config.ConversationID != "" {
		headers["X-Conversation-ID"] = c.config.ConversationID
	}

	// Add custom headers
	for k, v := range c.config.Headers {
		headers[k] = v
	}

	return &client.HTTPRequest{
		Method:  "POST",
		URL:     c.config.BaseURL + "/chat/completions",
		Headers: headers,
		Body:    bytes.NewReader(body),
	}, nil
}

// handleErrorResponse processes error responses from the API.
func (c *Client) handleErrorResponse(statusCode int, body []byte) error {
	// Try to parse error response
	var errorResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	bodyStr := string(body)
	if err := json.Unmarshal(body, &errorResp); err == nil {
		bodyStr = errorResp.Error.Message

		// Check for specific error types
		if statusCode == http.StatusUnauthorized {
			// 401 Unauthorized
			canRefresh := c.config.TokenRefreshFunc != nil
			return client.NewUnauthorizedError(bodyStr, canRefresh)
		}

		if errorResp.Error.Code == "context_length_exceeded" ||
			strings.Contains(errorResp.Error.Message, "context") {
			return client.NewContextWindowExceededError()
		}

		if errorResp.Error.Type == "rate_limit_error" || statusCode == http.StatusTooManyRequests {
			return client.NewUsageLimitError("", nil)
		}
	} else if statusCode == http.StatusUnauthorized {
		// Failed to parse error, but still 401
		canRefresh := c.config.TokenRefreshFunc != nil
		return client.NewUnauthorizedError(bodyStr, canRefresh)
	}

	return client.NewUnexpectedStatusError(statusCode, bodyStr)
}

// isRetryable checks if a status code should trigger a retry.
func (c *Client) isRetryable(statusCode int) bool {
	for _, code := range c.config.RetryConfig.RetryableStatusCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

// updateRateLimits updates internal rate limit tracking from response headers.
func (c *Client) updateRateLimits(headers map[string][]string) {
	c.rateLimits.update(headers)
}

// GetConnectionPoolStats returns current connection pool metrics.
// Returns nil if metrics are not available (e.g., when using a custom HTTPClient).
func (c *Client) GetConnectionPoolStats() *ConnectionPoolStats {
	if c.metrics == nil {
		return nil
	}
	stats := c.metrics.GetStats()
	return &stats
}

// LogConnectionPoolStats logs current connection pool statistics.
// Does nothing if metrics are not available.
func (c *Client) LogConnectionPoolStats() {
	if c.metrics != nil {
		c.metrics.LogStats()
	}
}

// defaultHTTPClient wraps standard http.Client to implement client.HTTPClient.
type defaultHTTPClient struct {
	client *http.Client
}

// newDefaultHTTPClientWithMetrics creates a new default HTTP client with connection pooling and metrics.
func newDefaultHTTPClientWithMetrics(timeout time.Duration, poolConfig client.ConnectionPoolConfig, metrics *ConnectionPoolMetrics) client.HTTPClient {
	// Create connection pool configuration
	transport := &http.Transport{
		// Connection pool settings from config
		MaxIdleConns:        poolConfig.MaxIdleConns,
		MaxIdleConnsPerHost: poolConfig.MaxIdleConnsPerHost,
		MaxConnsPerHost:     poolConfig.MaxConnsPerHost,
		IdleConnTimeout:     poolConfig.IdleConnTimeout,

		// Connection timeout settings
		DisableKeepAlives:  false, // Enable connection reuse
		DisableCompression: false,
		ForceAttemptHTTP2:  poolConfig.EnableHTTP2,

		// Timeouts for establishing connections
		TLSHandshakeTimeout:   poolConfig.ConnectionTimeout,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Wrap transport with metrics if provided
	var finalTransport http.RoundTripper = transport
	if metrics != nil {
		finalTransport = newMetricsTransport(transport, metrics)
	}

	return &defaultHTTPClient{
		client: &http.Client{
			Timeout:   timeout,
			Transport: finalTransport,
		},
	}
}

// Do executes an HTTP request.
func (d *defaultHTTPClient) Do(req *client.HTTPRequest) (*client.HTTPResponse, error) {
	// Convert to http.Request
	httpReq, err := http.NewRequest(req.Method, req.URL, req.Body)
	if err != nil {
		return nil, err
	}

	// Set headers
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Execute
	httpResp, err := d.client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	return &client.HTTPResponse{
		StatusCode: httpResp.StatusCode,
		Headers:    httpResp.Header,
		Body:       httpResp.Body,
	}, nil
}

// modelInfo contains information about a specific model.
type modelInfo struct {
	contextWindow int64
}

// getModelInfo returns information for a given model.
func getModelInfo(model string) modelInfo {
	// Normalize model name
	model = strings.ToLower(model)

	// Check for specific models
	switch {
	case strings.Contains(model, "gpt-4-turbo"):
		return modelInfo{contextWindow: 128000}
	case strings.Contains(model, "gpt-4-32k"):
		return modelInfo{contextWindow: 32768}
	case strings.Contains(model, "gpt-4"):
		return modelInfo{contextWindow: 8192}
	case strings.Contains(model, "gpt-3.5-turbo-16k"):
		return modelInfo{contextWindow: 16385}
	case strings.Contains(model, "gpt-3.5-turbo"):
		return modelInfo{contextWindow: 16385}
	case strings.Contains(model, "gpt-5"):
		return modelInfo{contextWindow: 200000}
	default:
		// Unknown model, return 0
		return modelInfo{contextWindow: 0}
	}
}
