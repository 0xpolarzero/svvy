package notify

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	rate       float64       // tokens per second
	capacity   int           // bucket capacity
	tokens     float64       // current tokens
	lastUpdate time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate float64, capacity int) *RateLimiter {
	return &RateLimiter{
		rate:       rate,
		capacity:   capacity,
		tokens:     float64(capacity),
		lastUpdate: time.Now(),
	}
}

// Allow checks if an event can proceed
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastUpdate).Seconds()

	// Add tokens based on time elapsed
	rl.tokens += elapsed * rl.rate
	if rl.tokens > float64(rl.capacity) {
		rl.tokens = float64(rl.capacity)
	}

	rl.lastUpdate = now

	// Check if we have tokens
	if rl.tokens >= 1.0 {
		rl.tokens -= 1.0
		return true
	}

	return false
}

// Wait blocks until a token is available
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		if rl.Allow() {
			return nil
		}

		// Calculate wait time
		rl.mu.Lock()
		waitTime := time.Duration((1.0 - rl.tokens) / rl.rate * float64(time.Second))
		rl.mu.Unlock()

		// Wait or check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Try again
		}
	}
}
