package progress

import (
	"context"

	"github.com/evmts/codex/codex-go/internal/protocol"
)

// ProtocolReporter is a Reporter that emits progress events to a protocol event queue.
type ProtocolReporter struct {
	// EventFunc is called to emit events to the protocol event queue
	// It receives the event to emit
	EventFunc func(protocol.EventMsg) error
}

// NewProtocolReporter creates a new ProtocolReporter with the given event emission function.
func NewProtocolReporter(eventFunc func(protocol.EventMsg) error) *ProtocolReporter {
	return &ProtocolReporter{
		EventFunc: eventFunc,
	}
}

// ReportStarted emits an operation started event.
func (r *ProtocolReporter) ReportStarted(ctx context.Context, event *StartedEvent) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	var estimatedDurationMs *int64
	if event.EstimatedDuration != nil {
		ms := event.EstimatedDuration.Milliseconds()
		estimatedDurationMs = &ms
	}

	protoEvent := &protocol.EventOperationStarted{
		Operation:         string(event.Op),
		EstimatedDuration: estimatedDurationMs,
		Total:             event.Total,
		Message:           event.Message,
	}

	return r.EventFunc(protoEvent)
}

// ReportProgress emits a progress update event.
func (r *ProtocolReporter) ReportProgress(ctx context.Context, event *ProgressEvent) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	protoEvent := &protocol.EventOperationProgress{
		Operation:  string(event.Op),
		Current:    event.Current,
		Total:      event.Total,
		Percentage: event.Percentage,
		Message:    event.Message,
	}

	return r.EventFunc(protoEvent)
}

// ReportCompleted emits an operation completed event.
func (r *ProtocolReporter) ReportCompleted(ctx context.Context, event *CompletedEvent) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	protoEvent := &protocol.EventOperationCompleted{
		Operation:      string(event.Op),
		DurationMs:     event.Duration.Milliseconds(),
		Status:         string(event.Status),
		Message:        event.Message,
		ProcessedCount: event.ProcessedCount,
	}

	return r.EventFunc(protoEvent)
}
