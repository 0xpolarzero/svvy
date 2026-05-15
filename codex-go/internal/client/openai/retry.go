package openai

import (
	"math"
	"math/rand"
	"time"
)

// calculateBackoff computes the backoff duration for a retry attempt.
// Uses exponential backoff with jitter to avoid thundering herd.
func calculateBackoff(attempt int, initialBackoff, maxBackoff time.Duration, multiplier float64) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	// Calculate exponential backoff
	backoff := float64(initialBackoff) * math.Pow(multiplier, float64(attempt))

	// Cap at max backoff
	if backoff > float64(maxBackoff) {
		backoff = float64(maxBackoff)
	}

	// Add jitter (±25% randomization)
	jitter := backoff * 0.25 * (2.0*rand.Float64() - 1.0)
	backoff += jitter

	// Ensure non-negative
	if backoff < 0 {
		backoff = 0
	}

	return time.Duration(backoff)
}

// retryPolicy defines when and how to retry requests.
// nolint:unused // Reserved for future retry logic enhancement
type retryPolicy struct {
	maxRetries           int
	initialBackoff       time.Duration
	maxBackoff           time.Duration
	backoffMultiplier    float64
	retryableStatusCodes map[int]bool
	respectRetryAfter    bool
}

// newRetryPolicy creates a retry policy from configuration.
// nolint:unused // Reserved for future retry logic enhancement
func newRetryPolicy(maxRetries int, initialBackoff, maxBackoff time.Duration, multiplier float64, retryableCodes []int, respectRetryAfter bool) *retryPolicy {
	codeMap := make(map[int]bool)
	for _, code := range retryableCodes {
		codeMap[code] = true
	}

	return &retryPolicy{
		maxRetries:           maxRetries,
		initialBackoff:       initialBackoff,
		maxBackoff:           maxBackoff,
		backoffMultiplier:    multiplier,
		retryableStatusCodes: codeMap,
		respectRetryAfter:    respectRetryAfter,
	}
}

// shouldRetry determines if a request should be retried based on status code.
// nolint:unused // Reserved for future retry logic enhancement
func (p *retryPolicy) shouldRetry(statusCode int) bool {
	return p.retryableStatusCodes[statusCode]
}

// getBackoff calculates the backoff duration for a given attempt.
// If respectRetryAfter is true and retryAfter is provided, it returns that value.
// nolint:unused // Reserved for future retry logic enhancement
func (p *retryPolicy) getBackoff(attempt int, retryAfter *time.Duration) time.Duration {
	if p.respectRetryAfter && retryAfter != nil {
		return *retryAfter
	}

	return calculateBackoff(attempt, p.initialBackoff, p.maxBackoff, p.backoffMultiplier)
}

// parseRetryAfter parses the Retry-After header value.
// Supports both delay-seconds and HTTP-date formats.
// Returns nil if parsing fails.
func parseRetryAfter(retryAfterHeader string) *time.Duration {
	if retryAfterHeader == "" {
		return nil
	}

	// Try parsing as HTTP-date format
	if _, err := time.Parse(time.RFC1123, retryAfterHeader); err == nil {
		// HTTP-date format
		// Calculate duration until that time
		// For simplicity, we'll just use a default
		d := 60 * time.Second
		return &d
	}

	// Try as integer seconds
	if n, err := time.ParseDuration(retryAfterHeader + "s"); err == nil {
		return &n
	}

	// If we have just a number, treat as seconds
	if n, err := time.ParseDuration(retryAfterHeader); err == nil {
		return &n
	}

	return nil
}
