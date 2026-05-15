package client

import (
	"fmt"
	"time"
)

// Error types that mirror the Rust implementation's error handling.

// StreamError represents an error that occurred during streaming.
// This matches the Rust CodexErr::Stream variant.
type StreamError struct {
	// Message describes what went wrong
	Message string

	// RetryAfter suggests how long to wait before retrying (if applicable)
	RetryAfter *time.Duration

	// RequestID helps correlate with server logs
	RequestID string
}

func (e *StreamError) Error() string {
	msg := fmt.Sprintf("stream error: %s", e.Message)
	if e.RequestID != "" {
		msg += fmt.Sprintf(" (request_id: %s)", e.RequestID)
	}
	if e.RetryAfter != nil {
		msg += fmt.Sprintf(" (retry after: %v)", *e.RetryAfter)
	}
	return msg
}

// ConnectionError represents a network/transport failure.
// This matches the Rust CodexErr::ConnectionFailed variant.
type ConnectionError struct {
	// Cause is the underlying error
	Cause error

	// URL that failed to connect
	URL string
}

func (e *ConnectionError) Error() string {
	msg := fmt.Sprintf("connection failed to %s: %v", e.URL, e.Cause)
	msg += ". Check your network connection and verify the server is accessible"
	return msg
}

func (e *ConnectionError) Unwrap() error {
	return e.Cause
}

// UnexpectedStatusError represents an HTTP error response.
// This matches the Rust CodexErr::UnexpectedStatus variant.
type UnexpectedStatusError struct {
	// StatusCode is the HTTP status code
	StatusCode int

	// Body contains the error response body
	Body string

	// RequestID helps correlate with server logs
	RequestID string
}

func (e *UnexpectedStatusError) Error() string {
	msg := fmt.Sprintf("unexpected status %d", e.StatusCode)
	if e.RequestID != "" {
		msg += fmt.Sprintf(" (request_id: %s)", e.RequestID)
	}
	if e.Body != "" {
		// Truncate long error bodies
		body := e.Body
		if len(body) > 500 {
			body = body[:500] + "..."
		}
		msg += fmt.Sprintf(": %s", body)
	}
	return msg
}

// RetryLimitError is returned when max retries are exhausted.
// This matches the Rust CodexErr::RetryLimit variant.
type RetryLimitError struct {
	// StatusCode is the last HTTP status code received
	StatusCode int

	// Attempts is the number of attempts made
	Attempts int

	// RequestID from the last attempt
	RequestID string
}

func (e *RetryLimitError) Error() string {
	msg := fmt.Sprintf("retry limit reached after %d attempts (status: %d)", e.Attempts, e.StatusCode)
	if e.RequestID != "" {
		msg += fmt.Sprintf(" (request_id: %s)", e.RequestID)
	}
	return msg
}

// UsageLimitError represents a usage/rate limit error.
// This matches the Rust CodexErr::UsageLimitReached variant.
type UsageLimitError struct {
	// PlanType identifies the subscription plan
	PlanType string

	// ResetsAt is when the limit resets (if known)
	ResetsAt *time.Time

	// RateLimits contains current rate limit state
	RateLimits *RateLimitSnapshot
}

func (e *UsageLimitError) Error() string {
	baseMsg := "You've hit your usage limit"

	switch e.PlanType {
	case "free":
		return baseMsg + ". Upgrade to Plus to continue using Codex (https://openai.com/chatgpt/pricing)"
	case "plus":
		suffix := formatResetSuffix(e.ResetsAt, " or try again")
		return fmt.Sprintf("%s. Upgrade to Pro (https://openai.com/chatgpt/pricing)%s", baseMsg, suffix)
	case "team", "business":
		suffix := formatResetSuffix(e.ResetsAt, " or try again")
		return fmt.Sprintf("%s. To get more access now, send a request to your admin%s", baseMsg, suffix)
	case "pro", "enterprise", "edu":
		suffix := formatResetSuffix(e.ResetsAt, ". Try again")
		return fmt.Sprintf("%s%s", baseMsg, suffix)
	default:
		suffix := formatResetSuffix(e.ResetsAt, ". Try again")
		return fmt.Sprintf("%s%s", baseMsg, suffix)
	}
}

// ContextWindowExceededError indicates the prompt is too long.
// This matches the Rust CodexErr::ContextWindowExceeded variant.
type ContextWindowExceededError struct {
	// TokenCount is the number of tokens in the prompt (if known)
	TokenCount int64

	// MaxTokens is the model's context window size
	MaxTokens int64
}

func (e *ContextWindowExceededError) Error() string {
	msg := "Codex ran out of room in the model's context window. Start a new conversation or clear earlier history before retrying"
	if e.TokenCount > 0 && e.MaxTokens > 0 {
		msg = fmt.Sprintf("Context window exceeded: using %d tokens (limit: %d). %s", e.TokenCount, e.MaxTokens, msg)
	}
	return msg
}

// IdleTimeoutError indicates the stream went silent for too long.
type IdleTimeoutError struct {
	// Timeout is the configured idle timeout
	Timeout time.Duration

	// LastEventTime is when the last event was received
	LastEventTime time.Time
}

func (e *IdleTimeoutError) Error() string {
	return fmt.Sprintf("stream idle timeout: no events for %v", e.Timeout)
}

// UnsupportedOperationError indicates a feature is not supported.
type UnsupportedOperationError struct {
	// Operation describes what was attempted
	Operation string

	// Provider identifies the API provider
	Provider string
}

func (e *UnsupportedOperationError) Error() string {
	msg := fmt.Sprintf("unsupported operation: %s", e.Operation)
	if e.Provider != "" {
		msg += fmt.Sprintf(" (provider: %s)", e.Provider)
	}
	return msg
}

// UnauthorizedError represents a 401 Unauthorized response.
// This typically indicates an expired or invalid API key/token.
type UnauthorizedError struct {
	// Message describes the authorization failure
	Message string

	// RequestID helps correlate with server logs
	RequestID string

	// CanRefresh indicates if token refresh should be attempted
	CanRefresh bool
}

func (e *UnauthorizedError) Error() string {
	msg := "Unauthorized: " + e.Message
	if e.RequestID != "" {
		msg += fmt.Sprintf(" (request_id: %s)", e.RequestID)
	}
	if e.CanRefresh {
		msg += ". Authentication token may have expired - attempting to refresh"
	} else {
		msg += ". Check your API key or authentication token"
	}
	return msg
}

// Helper functions for creating common errors

// NewStreamError creates a stream error.
func NewStreamError(message string) *StreamError {
	return &StreamError{Message: message}
}

// NewStreamErrorWithRetry creates a stream error with retry suggestion.
func NewStreamErrorWithRetry(message string, retryAfter time.Duration) *StreamError {
	return &StreamError{
		Message:    message,
		RetryAfter: &retryAfter,
	}
}

// NewConnectionError creates a connection error.
func NewConnectionError(url string, cause error) *ConnectionError {
	return &ConnectionError{
		URL:   url,
		Cause: cause,
	}
}

// NewUnexpectedStatusError creates an HTTP error.
func NewUnexpectedStatusError(statusCode int, body string) *UnexpectedStatusError {
	return &UnexpectedStatusError{
		StatusCode: statusCode,
		Body:       body,
	}
}

// NewRetryLimitError creates a retry exhaustion error.
func NewRetryLimitError(statusCode, attempts int) *RetryLimitError {
	return &RetryLimitError{
		StatusCode: statusCode,
		Attempts:   attempts,
	}
}

// NewUsageLimitError creates a usage limit error.
func NewUsageLimitError(planType string, resetsAt *time.Time) *UsageLimitError {
	return &UsageLimitError{
		PlanType: planType,
		ResetsAt: resetsAt,
	}
}

// NewContextWindowExceededError creates a context window error.
func NewContextWindowExceededError() *ContextWindowExceededError {
	return &ContextWindowExceededError{}
}

// NewIdleTimeoutError creates an idle timeout error.
func NewIdleTimeoutError(timeout time.Duration) *IdleTimeoutError {
	return &IdleTimeoutError{
		Timeout:       timeout,
		LastEventTime: time.Now(),
	}
}

// NewUnsupportedOperationError creates an unsupported operation error.
func NewUnsupportedOperationError(operation, provider string) *UnsupportedOperationError {
	return &UnsupportedOperationError{
		Operation: operation,
		Provider:  provider,
	}
}

// NewUnauthorizedError creates an unauthorized error.
func NewUnauthorizedError(message string, canRefresh bool) *UnauthorizedError {
	return &UnauthorizedError{
		Message:    message,
		CanRefresh: canRefresh,
	}
}

// Helper functions for error formatting

// formatResetSuffix formats a reset time into a human-readable suffix
func formatResetSuffix(resetsAt *time.Time, prefix string) string {
	if resetsAt == nil {
		return prefix + " later"
	}

	remaining := time.Until(*resetsAt)
	if remaining <= 0 {
		return prefix + " now"
	}

	duration := formatDuration(remaining)
	return prefix + " in " + duration
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "less than a minute"
	}

	totalSecs := int(d.Seconds())
	days := totalSecs / 86400
	hours := (totalSecs % 86400) / 3600
	minutes := (totalSecs % 3600) / 60

	var parts []string
	if days > 0 {
		unit := "day"
		if days > 1 {
			unit = "days"
		}
		parts = append(parts, fmt.Sprintf("%d %s", days, unit))
	}
	if hours > 0 {
		unit := "hour"
		if hours > 1 {
			unit = "hours"
		}
		parts = append(parts, fmt.Sprintf("%d %s", hours, unit))
	}
	if minutes > 0 {
		unit := "minute"
		if minutes > 1 {
			unit = "minutes"
		}
		parts = append(parts, fmt.Sprintf("%d %s", minutes, unit))
	}

	if len(parts) == 0 {
		return "less than a minute"
	}

	result := ""
	for i, part := range parts {
		if i > 0 {
			result += " "
		}
		result += part
	}
	return result
}
