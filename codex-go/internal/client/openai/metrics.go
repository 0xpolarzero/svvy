package openai

import (
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionPoolMetrics tracks connection pool performance and health.
type ConnectionPoolMetrics struct {
	// Atomic counters for thread-safe tracking
	totalRequests      atomic.Int64
	activeConnections  atomic.Int64
	idleConnections    atomic.Int64
	connectionReuses   atomic.Int64
	newConnections     atomic.Int64
	poolExhaustions    atomic.Int64
	connectionTimeouts atomic.Int64

	// Resource leak tracking
	openResponseBodies   atomic.Int64 // Currently open response bodies
	totalResponseBodies  atomic.Int64 // Total response bodies created
	closedResponseBodies atomic.Int64 // Total response bodies closed
	leakedResponseBodies atomic.Int64 // Detected leaks (bodies not closed properly)

	// Configuration thresholds
	maxIdleConnsPerHost int
	warnThreshold       float64 // Percentage threshold for exhaustion warnings

	// Last warning time to prevent log spam
	lastWarningTime time.Time
	warningMutex    sync.RWMutex
}

// newConnectionPoolMetrics creates a new metrics tracker.
func newConnectionPoolMetrics(maxIdleConnsPerHost int) *ConnectionPoolMetrics {
	return &ConnectionPoolMetrics{
		maxIdleConnsPerHost: maxIdleConnsPerHost,
		warnThreshold:       0.8, // Warn at 80% capacity
	}
}

// TrackRequest increments the total request counter.
func (m *ConnectionPoolMetrics) TrackRequest() {
	m.totalRequests.Add(1)
}

// TrackConnectionReuse increments the connection reuse counter.
func (m *ConnectionPoolMetrics) TrackConnectionReuse() {
	m.connectionReuses.Add(1)
}

// TrackNewConnection increments the new connection counter.
func (m *ConnectionPoolMetrics) TrackNewConnection() {
	m.newConnections.Add(1)
}

// TrackActiveConnection increments the active connection counter.
func (m *ConnectionPoolMetrics) TrackActiveConnection() {
	m.activeConnections.Add(1)
}

// ReleaseActiveConnection decrements the active connection counter.
func (m *ConnectionPoolMetrics) ReleaseActiveConnection() {
	m.activeConnections.Add(-1)
}

// TrackPoolExhaustion increments the pool exhaustion counter and logs a warning.
func (m *ConnectionPoolMetrics) TrackPoolExhaustion() {
	m.poolExhaustions.Add(1)
	m.logWarningThrottled("Connection pool exhausted - consider increasing MaxIdleConnsPerHost")
}

// TrackConnectionTimeout increments the connection timeout counter.
func (m *ConnectionPoolMetrics) TrackConnectionTimeout() {
	m.connectionTimeouts.Add(1)
	m.logWarningThrottled("Connection timeout - check network latency or increase ConnectionTimeout")
}

// TrackResponseBodyOpened increments the open response body counter.
// Call this when a response body is created.
func (m *ConnectionPoolMetrics) TrackResponseBodyOpened() {
	m.totalResponseBodies.Add(1)
	m.openResponseBodies.Add(1)

	// Check for potential leaks
	open := m.openResponseBodies.Load()
	if open > 100 {
		m.logWarningThrottled("High number of open response bodies: %d - potential resource leak", open)
	}
}

// TrackResponseBodyClosed decrements the open response body counter.
// Call this when a response body is closed properly.
func (m *ConnectionPoolMetrics) TrackResponseBodyClosed() {
	m.closedResponseBodies.Add(1)
	m.openResponseBodies.Add(-1)
}

// TrackResponseBodyLeak increments the leaked response body counter.
// Call this when a leak is detected (e.g., body not closed before GC).
func (m *ConnectionPoolMetrics) TrackResponseBodyLeak() {
	m.leakedResponseBodies.Add(1)
	m.logWarningThrottled("Response body leak detected - ensure all HTTP response bodies are closed")
}

// CheckPoolUtilization checks if the pool is approaching exhaustion and logs a warning.
func (m *ConnectionPoolMetrics) CheckPoolUtilization(idle, active int) {
	if m.maxIdleConnsPerHost <= 0 {
		return
	}

	total := idle + active
	utilization := float64(total) / float64(m.maxIdleConnsPerHost)

	if utilization >= m.warnThreshold {
		m.logWarningThrottled("Connection pool utilization high: %.1f%% (%d/%d connections)",
			utilization*100, total, m.maxIdleConnsPerHost)
	}
}

// GetReuseRate returns the connection reuse rate as a percentage.
func (m *ConnectionPoolMetrics) GetReuseRate() float64 {
	total := m.totalRequests.Load()
	if total == 0 {
		return 0
	}
	reuses := m.connectionReuses.Load()
	return float64(reuses) / float64(total) * 100
}

// GetStats returns a snapshot of current metrics.
func (m *ConnectionPoolMetrics) GetStats() ConnectionPoolStats {
	total := m.totalRequests.Load()
	reuses := m.connectionReuses.Load()
	newConns := m.newConnections.Load()

	var reuseRate float64
	if total > 0 {
		reuseRate = float64(reuses) / float64(total) * 100
	}

	return ConnectionPoolStats{
		TotalRequests:      total,
		ActiveConnections:  m.activeConnections.Load(),
		IdleConnections:    m.idleConnections.Load(),
		ConnectionReuses:   reuses,
		NewConnections:     newConns,
		PoolExhaustions:    m.poolExhaustions.Load(),
		ConnectionTimeouts: m.connectionTimeouts.Load(),
		ReuseRate:          reuseRate,

		// Resource leak tracking
		OpenResponseBodies:   m.openResponseBodies.Load(),
		TotalResponseBodies:  m.totalResponseBodies.Load(),
		ClosedResponseBodies: m.closedResponseBodies.Load(),
		LeakedResponseBodies: m.leakedResponseBodies.Load(),
	}
}

// LogStats logs current connection pool statistics.
func (m *ConnectionPoolMetrics) LogStats() {
	stats := m.GetStats()
	log.Printf("Connection pool stats: requests=%d active=%d idle=%d reuses=%d (%.1f%%) new=%d exhaustions=%d timeouts=%d | response_bodies: open=%d total=%d closed=%d leaked=%d",
		stats.TotalRequests,
		stats.ActiveConnections,
		stats.IdleConnections,
		stats.ConnectionReuses,
		stats.ReuseRate,
		stats.NewConnections,
		stats.PoolExhaustions,
		stats.ConnectionTimeouts,
		stats.OpenResponseBodies,
		stats.TotalResponseBodies,
		stats.ClosedResponseBodies,
		stats.LeakedResponseBodies,
	)
}

// logWarningThrottled logs a warning message, but throttles repeated warnings to once per minute.
func (m *ConnectionPoolMetrics) logWarningThrottled(format string, args ...interface{}) {
	m.warningMutex.Lock()
	defer m.warningMutex.Unlock()

	now := time.Now()
	if now.Sub(m.lastWarningTime) < time.Minute {
		return // Throttle warnings
	}

	m.lastWarningTime = now
	log.Printf("[WARNING] "+format, args...)
}

// ConnectionPoolStats contains a snapshot of connection pool metrics.
type ConnectionPoolStats struct {
	TotalRequests      int64   `json:"total_requests"`
	ActiveConnections  int64   `json:"active_connections"`
	IdleConnections    int64   `json:"idle_connections"`
	ConnectionReuses   int64   `json:"connection_reuses"`
	NewConnections     int64   `json:"new_connections"`
	PoolExhaustions    int64   `json:"pool_exhaustions"`
	ConnectionTimeouts int64   `json:"connection_timeouts"`
	ReuseRate          float64 `json:"reuse_rate_percent"`

	// Resource leak tracking
	OpenResponseBodies   int64 `json:"open_response_bodies"`
	TotalResponseBodies  int64 `json:"total_response_bodies"`
	ClosedResponseBodies int64 `json:"closed_response_bodies"`
	LeakedResponseBodies int64 `json:"leaked_response_bodies"`
}

// metricsTransport wraps http.RoundTripper to track connection metrics.
type metricsTransport struct {
	base    http.RoundTripper
	metrics *ConnectionPoolMetrics
}

// newMetricsTransport creates a transport wrapper that tracks connection metrics.
func newMetricsTransport(base http.RoundTripper, metrics *ConnectionPoolMetrics) http.RoundTripper {
	return &metricsTransport{
		base:    base,
		metrics: metrics,
	}
}

// RoundTrip implements http.RoundTripper and tracks request metrics.
func (t *metricsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.metrics.TrackRequest()
	t.metrics.TrackActiveConnection()
	defer t.metrics.ReleaseActiveConnection()

	// Track if this is likely a new connection or reuse
	// This is a heuristic since Go's transport doesn't directly expose this
	startTime := time.Now()

	resp, err := t.base.RoundTrip(req)

	elapsed := time.Since(startTime)

	// Heuristic: Fast responses (<50ms) are likely connection reuses
	// Slower responses may indicate new connection establishment
	if err == nil {
		if elapsed < 50*time.Millisecond {
			t.metrics.TrackConnectionReuse()
		} else if elapsed > 100*time.Millisecond {
			t.metrics.TrackNewConnection()
		}

		// Track response body creation
		t.metrics.TrackResponseBodyOpened()

		// Wrap the response body to track closure
		resp.Body = &trackedReadCloser{
			ReadCloser: resp.Body,
			metrics:    t.metrics,
		}
	}

	return resp, err
}

// trackedReadCloser wraps an io.ReadCloser to track when it's closed.
type trackedReadCloser struct {
	io.ReadCloser
	metrics *ConnectionPoolMetrics
	closed  atomic.Bool
}

// Close tracks the response body closure and delegates to the underlying closer.
func (t *trackedReadCloser) Close() error {
	// Use atomic CAS to ensure we only track closure once
	if t.closed.CompareAndSwap(false, true) {
		t.metrics.TrackResponseBodyClosed()
	}
	return t.ReadCloser.Close()
}
