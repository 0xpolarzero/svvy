package openai

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/evmts/codex/codex-go/internal/client"
)

// rateLimitTracker tracks rate limit information from API responses.
type rateLimitTracker struct {
	mu       sync.RWMutex
	snapshot *client.RateLimitSnapshot
}

// newRateLimitTracker creates a new rate limit tracker.
func newRateLimitTracker() *rateLimitTracker {
	return &rateLimitTracker{
		snapshot: &client.RateLimitSnapshot{},
	}
}

// update updates rate limit information from response headers.
// OpenAI returns rate limit headers like:
//   - x-ratelimit-limit-requests: 10000
//   - x-ratelimit-remaining-requests: 9999
//   - x-ratelimit-reset-requests: 60s
//   - x-ratelimit-limit-tokens: 1000000
//   - x-ratelimit-remaining-tokens: 990000
//   - x-ratelimit-reset-tokens: 3600s
func (t *rateLimitTracker) update(headers map[string][]string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Parse request-based rate limits
	if limitReq := getHeader(headers, "x-ratelimit-limit-requests"); limitReq != "" {
		remainingReq := getHeader(headers, "x-ratelimit-remaining-requests")
		resetReq := getHeader(headers, "x-ratelimit-reset-requests")

		if limit, err := strconv.ParseInt(limitReq, 10, 64); err == nil {
			if remaining, err := strconv.ParseInt(remainingReq, 10, 64); err == nil {
				usedPercent := float64(limit-remaining) / float64(limit) * 100.0

				window := &client.RateLimitWindow{
					UsedPercent: usedPercent,
				}

				// Parse reset time
				if resetDuration := parseDuration(resetReq); resetDuration > 0 {
					windowMinutes := int64(resetDuration.Minutes())
					resetsAt := time.Now().Add(resetDuration).Unix()
					window.WindowMinutes = &windowMinutes
					window.ResetsAt = &resetsAt
				}

				t.snapshot.Primary = window
			}
		}
	}

	// Parse token-based rate limits
	if limitTokens := getHeader(headers, "x-ratelimit-limit-tokens"); limitTokens != "" {
		remainingTokens := getHeader(headers, "x-ratelimit-remaining-tokens")
		resetTokens := getHeader(headers, "x-ratelimit-reset-tokens")

		if limit, err := strconv.ParseInt(limitTokens, 10, 64); err == nil {
			if remaining, err := strconv.ParseInt(remainingTokens, 10, 64); err == nil {
				usedPercent := float64(limit-remaining) / float64(limit) * 100.0

				window := &client.RateLimitWindow{
					UsedPercent: usedPercent,
				}

				// Parse reset time
				if resetDuration := parseDuration(resetTokens); resetDuration > 0 {
					windowMinutes := int64(resetDuration.Minutes())
					resetsAt := time.Now().Add(resetDuration).Unix()
					window.WindowMinutes = &windowMinutes
					window.ResetsAt = &resetsAt
				}

				t.snapshot.Secondary = window
			}
		}
	}
}

// get returns the current rate limit snapshot.
// nolint:unused // Reserved for future rate limit reporting
func (t *rateLimitTracker) get() *client.RateLimitSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Return a copy
	snapshot := &client.RateLimitSnapshot{}
	if t.snapshot.Primary != nil {
		primary := *t.snapshot.Primary
		snapshot.Primary = &primary
	}
	if t.snapshot.Secondary != nil {
		secondary := *t.snapshot.Secondary
		snapshot.Secondary = &secondary
	}

	return snapshot
}

// isNearLimit checks if we're approaching rate limits.
// Returns true if any rate limit is above the threshold percentage.
// nolint:unused // Reserved for future rate limit handling
func (t *rateLimitTracker) isNearLimit(thresholdPercent float64) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.snapshot.Primary != nil && t.snapshot.Primary.UsedPercent >= thresholdPercent {
		return true
	}

	if t.snapshot.Secondary != nil && t.snapshot.Secondary.UsedPercent >= thresholdPercent {
		return true
	}

	return false
}

// waitIfNeeded blocks if we're near rate limits until they reset.
// Returns true if waiting occurred.
// nolint:unused // Reserved for future rate limit handling
func (t *rateLimitTracker) waitIfNeeded(thresholdPercent float64) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var waitDuration time.Duration

	// Check primary limit
	if t.snapshot.Primary != nil && t.snapshot.Primary.UsedPercent >= thresholdPercent {
		if t.snapshot.Primary.ResetsAt != nil {
			resetTime := time.Unix(*t.snapshot.Primary.ResetsAt, 0)
			untilReset := time.Until(resetTime)
			if untilReset > 0 && untilReset > waitDuration {
				waitDuration = untilReset
			}
		}
	}

	// Check secondary limit
	if t.snapshot.Secondary != nil && t.snapshot.Secondary.UsedPercent >= thresholdPercent {
		if t.snapshot.Secondary.ResetsAt != nil {
			resetTime := time.Unix(*t.snapshot.Secondary.ResetsAt, 0)
			untilReset := time.Until(resetTime)
			if untilReset > 0 && untilReset > waitDuration {
				waitDuration = untilReset
			}
		}
	}

	if waitDuration > 0 {
		// Add a small buffer
		waitDuration += 1 * time.Second
		time.Sleep(waitDuration)
		return true
	}

	return false
}

// getHeader retrieves a header value (case-insensitive).
func getHeader(headers map[string][]string, key string) string {
	key = strings.ToLower(key)

	for k, values := range headers {
		if strings.ToLower(k) == key && len(values) > 0 {
			return values[0]
		}
	}

	return ""
}

// parseDuration parses duration strings like "60s", "1h", etc.
func parseDuration(s string) time.Duration {
	if s == "" {
		return 0
	}

	// Try direct parsing
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}

	// Try as integer seconds
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Duration(n) * time.Second
	}

	return 0
}
