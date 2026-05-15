// Package progress provides progress tracking for long-running operations in Codex.
//
// This package enables emitting progress events for operations that take significant time,
// allowing clients to display progress feedback to users. It supports operations like:
//   - File reading/writing for large files
//   - Patch application for large diffs
//   - MCP tool discovery
//   - History reconstruction
//
// Progress events include start, progress updates (with percentage), and completion status.
package progress

import (
	"context"
	"time"
)

// OperationType identifies the kind of operation being tracked.
type OperationType string

const (
	// OperationFileRead tracks reading a file
	OperationFileRead OperationType = "file_read"

	// OperationFileWrite tracks writing a file
	OperationFileWrite OperationType = "file_write"

	// OperationPatchApply tracks applying patches
	OperationPatchApply OperationType = "patch_apply"

	// OperationMcpDiscovery tracks MCP tool discovery
	OperationMcpDiscovery OperationType = "mcp_discovery"

	// OperationHistoryReconstruct tracks history loading
	OperationHistoryReconstruct OperationType = "history_reconstruct"
)

// OperationStatus indicates the completion status of an operation.
type OperationStatus string

const (
	// StatusSuccess indicates successful completion
	StatusSuccess OperationStatus = "success"

	// StatusFailed indicates the operation failed
	StatusFailed OperationStatus = "failed"

	// StatusCancelled indicates the operation was cancelled
	StatusCancelled OperationStatus = "cancelled"
)

// Config controls progress tracking behavior.
type Config struct {
	// Enabled determines if progress tracking is active
	Enabled bool

	// MinDuration specifies the minimum operation duration to track.
	// Operations shorter than this are not reported to reduce noise.
	MinDuration time.Duration

	// UpdateInterval specifies how often progress updates should be sent.
	// This prevents overwhelming clients with too frequent updates.
	UpdateInterval time.Duration
}

// DefaultConfig returns the default progress configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:        true,
		MinDuration:    100 * time.Millisecond,
		UpdateInterval: 250 * time.Millisecond,
	}
}

// Event represents a progress event that can be emitted during operation execution.
type Event interface {
	// EventType returns the type discriminator for this event
	EventType() string

	// Operation returns the operation type being tracked
	Operation() OperationType

	// isProgressEvent is a marker method to ensure type safety
	isProgressEvent()
}

// StartedEvent indicates an operation has started.
type StartedEvent struct {
	// Op is the type of operation
	Op OperationType

	// EstimatedDuration is an optional estimate of how long the operation will take
	EstimatedDuration *time.Duration

	// Total is the total number of units to process (bytes, files, etc.)
	// Nil if unknown
	Total *int64

	// Message provides additional context about what's starting
	Message string
}

func (e *StartedEvent) EventType() string        { return "operation_started" }
func (e *StartedEvent) Operation() OperationType { return e.Op }
func (e *StartedEvent) isProgressEvent()         {}

// ProgressEvent reports incremental progress during an operation.
type ProgressEvent struct {
	// Op is the type of operation
	Op OperationType

	// Current is the current progress value (bytes processed, files done, etc.)
	Current int64

	// Total is the total expected value
	// Nil if unknown or indeterminate
	Total *int64

	// Percentage is the completion percentage (0-100)
	// Nil if progress is indeterminate
	Percentage *float64

	// Message provides additional context about the current progress
	Message string
}

func (e *ProgressEvent) EventType() string        { return "operation_progress" }
func (e *ProgressEvent) Operation() OperationType { return e.Op }
func (e *ProgressEvent) isProgressEvent()         {}

// CompletedEvent indicates an operation has finished.
type CompletedEvent struct {
	// Op is the type of operation
	Op OperationType

	// Duration is how long the operation took
	Duration time.Duration

	// Status indicates success or failure
	Status OperationStatus

	// Message provides additional completion context
	Message string

	// ProcessedCount is the total number of items processed
	ProcessedCount *int64
}

func (e *CompletedEvent) EventType() string        { return "operation_completed" }
func (e *CompletedEvent) Operation() OperationType { return e.Op }
func (e *CompletedEvent) isProgressEvent()         {}

// Reporter is the interface for emitting progress events.
// Implementations can send events to protocol event queues, logs, or metrics.
type Reporter interface {
	// ReportStarted emits an operation started event
	ReportStarted(ctx context.Context, event *StartedEvent) error

	// ReportProgress emits a progress update event
	ReportProgress(ctx context.Context, event *ProgressEvent) error

	// ReportCompleted emits an operation completed event
	ReportCompleted(ctx context.Context, event *CompletedEvent) error
}

// NoOpReporter is a reporter that does nothing.
// Used when progress tracking is disabled.
type NoOpReporter struct{}

func (n *NoOpReporter) ReportStarted(ctx context.Context, event *StartedEvent) error {
	return nil
}

func (n *NoOpReporter) ReportProgress(ctx context.Context, event *ProgressEvent) error {
	return nil
}

func (n *NoOpReporter) ReportCompleted(ctx context.Context, event *CompletedEvent) error {
	return nil
}

// calculatePercentage computes percentage from current and total values.
func calculatePercentage(current, total int64) float64 {
	if total <= 0 {
		return 0
	}
	pct := (float64(current) / float64(total)) * 100.0
	if pct > 100.0 {
		return 100.0
	}
	if pct < 0 {
		return 0
	}
	return pct
}
