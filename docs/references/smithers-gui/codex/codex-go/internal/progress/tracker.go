package progress

import (
	"context"
	"sync"
	"time"
)

// Tracker manages progress tracking for a single operation.
// It handles throttling of progress updates and automatic completion reporting.
type Tracker struct {
	config   Config
	reporter Reporter
	op       OperationType

	mu             sync.Mutex
	startTime      time.Time
	lastUpdateTime time.Time
	total          *int64
	current        int64
	completed      bool
}

// NewTracker creates a new progress tracker for an operation.
func NewTracker(config Config, reporter Reporter, op OperationType) *Tracker {
	return &Tracker{
		config:   config,
		reporter: reporter,
		op:       op,
	}
}

// Start reports that the operation has started.
// If total is provided, it enables percentage-based progress reporting.
// Returns immediately if progress tracking is disabled or the context is cancelled.
func (t *Tracker) Start(ctx context.Context, total *int64, message string) error {
	if !t.config.Enabled {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.startTime = time.Now()
	t.lastUpdateTime = t.startTime
	t.total = total
	t.current = 0
	t.completed = false

	// Estimate duration based on operation type and size
	var estimatedDuration *time.Duration
	if total != nil && *total > 0 {
		est := estimateDuration(t.op, *total)
		estimatedDuration = &est
	}

	event := &StartedEvent{
		Op:                t.op,
		EstimatedDuration: estimatedDuration,
		Total:             total,
		Message:           message,
	}

	return t.reporter.ReportStarted(ctx, event)
}

// Update reports progress for the operation.
// It automatically throttles updates based on UpdateInterval to avoid overwhelming clients.
// Returns immediately if progress tracking is disabled or the context is cancelled.
func (t *Tracker) Update(ctx context.Context, current int64, message string) error {
	if !t.config.Enabled {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.completed {
		return nil
	}

	t.current = current

	// Throttle updates based on interval
	now := time.Now()
	if now.Sub(t.lastUpdateTime) < t.config.UpdateInterval {
		return nil
	}
	t.lastUpdateTime = now

	event := &ProgressEvent{
		Op:      t.op,
		Current: current,
		Total:   t.total,
		Message: message,
	}

	// Calculate percentage if total is known
	if t.total != nil && *t.total > 0 {
		pct := calculatePercentage(current, *t.total)
		event.Percentage = &pct
	}

	return t.reporter.ReportProgress(ctx, event)
}

// Increment advances progress by the specified delta.
// This is a convenience method for operations that process items sequentially.
func (t *Tracker) Increment(ctx context.Context, delta int64, message string) error {
	if !t.config.Enabled {
		return nil
	}

	t.mu.Lock()
	newCurrent := t.current + delta
	t.mu.Unlock()

	return t.Update(ctx, newCurrent, message)
}

// Complete reports that the operation has finished successfully.
// It automatically calculates duration from the start time.
// Returns immediately if progress tracking is disabled or already completed.
func (t *Tracker) Complete(ctx context.Context, message string) error {
	return t.finish(ctx, StatusSuccess, message)
}

// Fail reports that the operation has failed.
func (t *Tracker) Fail(ctx context.Context, message string) error {
	return t.finish(ctx, StatusFailed, message)
}

// Cancel reports that the operation was cancelled.
func (t *Tracker) Cancel(ctx context.Context, message string) error {
	return t.finish(ctx, StatusCancelled, message)
}

// finish completes the operation with the given status.
func (t *Tracker) finish(ctx context.Context, status OperationStatus, message string) error {
	if !t.config.Enabled {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.completed {
		return nil
	}

	duration := time.Since(t.startTime)

	// Don't report if operation was too fast (below minimum threshold)
	if duration < t.config.MinDuration {
		t.completed = true
		return nil
	}

	t.completed = true

	var processedCount *int64
	if t.current > 0 {
		processedCount = &t.current
	}

	event := &CompletedEvent{
		Op:             t.op,
		Duration:       duration,
		Status:         status,
		Message:        message,
		ProcessedCount: processedCount,
	}

	return t.reporter.ReportCompleted(ctx, event)
}

// estimateDuration provides a rough estimate of operation duration based on size.
// These are heuristics and will vary based on hardware and system load.
func estimateDuration(op OperationType, size int64) time.Duration {
	switch op {
	case OperationFileRead:
		// Assume ~100 MB/s read speed
		mbSize := float64(size) / (1024 * 1024)
		seconds := mbSize / 100.0
		return time.Duration(seconds * float64(time.Second))

	case OperationFileWrite:
		// Assume ~50 MB/s write speed (slower than reads)
		mbSize := float64(size) / (1024 * 1024)
		seconds := mbSize / 50.0
		return time.Duration(seconds * float64(time.Second))

	case OperationPatchApply:
		// Rough estimate: ~1000 lines per second
		lines := size // Assume size is line count
		seconds := float64(lines) / 1000.0
		return time.Duration(seconds * float64(time.Second))

	case OperationMcpDiscovery:
		// Typically fast, but can vary with network
		return 2 * time.Second

	case OperationHistoryReconstruct:
		// Depends on history size, assume ~100 entries per second
		entries := size
		seconds := float64(entries) / 100.0
		return time.Duration(seconds * float64(time.Second))

	default:
		return 1 * time.Second
	}
}

// Track is a convenience function that creates a tracker, starts it, and returns it.
// This is useful for simple progress tracking scenarios.
func Track(ctx context.Context, config Config, reporter Reporter, op OperationType, total *int64, message string) (*Tracker, error) {
	tracker := NewTracker(config, reporter, op)
	if err := tracker.Start(ctx, total, message); err != nil {
		return nil, err
	}
	return tracker, nil
}

// TrackOperation wraps an operation with automatic progress tracking.
// It starts tracking, runs the operation, and completes tracking based on the result.
// The operation function receives the tracker for progress updates.
func TrackOperation(
	ctx context.Context,
	config Config,
	reporter Reporter,
	op OperationType,
	total *int64,
	message string,
	fn func(context.Context, *Tracker) error,
) error {
	tracker := NewTracker(config, reporter, op)

	if err := tracker.Start(ctx, total, message); err != nil {
		return err
	}

	// Run the operation
	err := fn(ctx, tracker)

	// Complete or fail based on result
	if err != nil {
		if ctx.Err() == context.Canceled {
			_ = tracker.Cancel(ctx, "Operation cancelled")
		} else {
			_ = tracker.Fail(ctx, err.Error())
		}
		return err
	}

	return tracker.Complete(ctx, message)
}
