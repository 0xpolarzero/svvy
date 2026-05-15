package progress

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockReporter records all progress events for testing
type mockReporter struct {
	mu         sync.Mutex
	started    []*StartedEvent
	progress   []*ProgressEvent
	completed  []*CompletedEvent
	shouldFail bool
}

func newMockReporter() *mockReporter {
	return &mockReporter{
		started:   make([]*StartedEvent, 0),
		progress:  make([]*ProgressEvent, 0),
		completed: make([]*CompletedEvent, 0),
	}
}

func (m *mockReporter) ReportStarted(ctx context.Context, event *StartedEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.shouldFail {
		return context.Canceled
	}
	m.started = append(m.started, event)
	return nil
}

func (m *mockReporter) ReportProgress(ctx context.Context, event *ProgressEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.shouldFail {
		return context.Canceled
	}
	m.progress = append(m.progress, event)
	return nil
}

func (m *mockReporter) ReportCompleted(ctx context.Context, event *CompletedEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.shouldFail {
		return context.Canceled
	}
	m.completed = append(m.completed, event)
	return nil
}

func (m *mockReporter) getStarted() []*StartedEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*StartedEvent, len(m.started))
	copy(result, m.started)
	return result
}

func (m *mockReporter) getProgress() []*ProgressEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*ProgressEvent, len(m.progress))
	copy(result, m.progress)
	return result
}

func (m *mockReporter) getCompleted() []*CompletedEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*CompletedEvent, len(m.completed))
	copy(result, m.completed)
	return result
}

func TestTrackerBasicFlow(t *testing.T) {
	ctx := context.Background()
	reporter := newMockReporter()
	config := DefaultConfig()
	config.UpdateInterval = 0 // Disable throttling for tests
	config.MinDuration = 0     // No minimum duration for tests

	tracker := NewTracker(config, reporter, OperationFileRead)

	// Start
	total := int64(1000)
	if err := tracker.Start(ctx, &total, "Reading file"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify started event
	started := reporter.getStarted()
	if len(started) != 1 {
		t.Fatalf("Expected 1 started event, got %d", len(started))
	}
	if started[0].Op != OperationFileRead {
		t.Errorf("Expected operation %s, got %s", OperationFileRead, started[0].Op)
	}
	if started[0].Total == nil || *started[0].Total != 1000 {
		t.Errorf("Expected total 1000, got %v", started[0].Total)
	}

	// Progress updates
	if err := tracker.Update(ctx, 250, "25% complete"); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if err := tracker.Update(ctx, 500, "50% complete"); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if err := tracker.Update(ctx, 750, "75% complete"); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify progress events
	progress := reporter.getProgress()
	if len(progress) != 3 {
		t.Fatalf("Expected 3 progress events, got %d", len(progress))
	}

	// Check percentages
	for i, expected := range []float64{25.0, 50.0, 75.0} {
		if progress[i].Percentage == nil {
			t.Errorf("Progress event %d missing percentage", i)
			continue
		}
		if *progress[i].Percentage != expected {
			t.Errorf("Progress event %d: expected %.1f%%, got %.1f%%", i, expected, *progress[i].Percentage)
		}
	}

	// Complete
	if err := tracker.Complete(ctx, "Done"); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Verify completed event
	completed := reporter.getCompleted()
	if len(completed) != 1 {
		t.Fatalf("Expected 1 completed event, got %d", len(completed))
	}
	if completed[0].Status != StatusSuccess {
		t.Errorf("Expected status %s, got %s", StatusSuccess, completed[0].Status)
	}
	if completed[0].ProcessedCount == nil || *completed[0].ProcessedCount != 750 {
		t.Errorf("Expected processed count 750, got %v", completed[0].ProcessedCount)
	}
}

func TestTrackerIncrement(t *testing.T) {
	ctx := context.Background()
	reporter := newMockReporter()
	config := DefaultConfig()
	config.UpdateInterval = 0
	config.MinDuration = 0

	tracker := NewTracker(config, reporter, OperationPatchApply)

	total := int64(100)
	if err := tracker.Start(ctx, &total, "Applying patches"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Use Increment instead of Update
	for i := 0; i < 10; i++ {
		if err := tracker.Increment(ctx, 10, "Processing..."); err != nil {
			t.Fatalf("Increment failed: %v", err)
		}
	}

	progress := reporter.getProgress()
	if len(progress) != 10 {
		t.Fatalf("Expected 10 progress events, got %d", len(progress))
	}

	// Check that current values are incrementing
	for i, p := range progress {
		expected := int64((i + 1) * 10)
		if p.Current != expected {
			t.Errorf("Progress event %d: expected current %d, got %d", i, expected, p.Current)
		}
	}

	if err := tracker.Complete(ctx, "All patches applied"); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
}

func TestTrackerDisabled(t *testing.T) {
	ctx := context.Background()
	reporter := newMockReporter()
	config := DefaultConfig()
	config.Enabled = false // Disable progress tracking

	tracker := NewTracker(config, reporter, OperationFileWrite)

	total := int64(1000)
	if err := tracker.Start(ctx, &total, "Writing"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := tracker.Update(ctx, 500, "Half way"); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if err := tracker.Complete(ctx, "Done"); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// No events should be emitted when disabled
	if len(reporter.getStarted()) != 0 {
		t.Error("Expected no started events when disabled")
	}
	if len(reporter.getProgress()) != 0 {
		t.Error("Expected no progress events when disabled")
	}
	if len(reporter.getCompleted()) != 0 {
		t.Error("Expected no completed events when disabled")
	}
}

func TestTrackerMinDuration(t *testing.T) {
	ctx := context.Background()
	reporter := newMockReporter()
	config := DefaultConfig()
	config.MinDuration = 100 * time.Millisecond
	config.UpdateInterval = 0

	tracker := NewTracker(config, reporter, OperationFileRead)

	// Quick operation that finishes below minimum duration
	if err := tracker.Start(ctx, nil, "Quick read"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	// Complete immediately (no delay)
	if err := tracker.Complete(ctx, "Done"); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Started event should be emitted
	if len(reporter.getStarted()) != 1 {
		t.Error("Expected started event even for quick operations")
	}

	// Completed event should NOT be emitted (below min duration)
	if len(reporter.getCompleted()) != 0 {
		t.Error("Expected no completed event for operation below min duration")
	}
}

func TestTrackerUpdateThrottling(t *testing.T) {
	ctx := context.Background()
	reporter := newMockReporter()
	config := DefaultConfig()
	config.UpdateInterval = 50 * time.Millisecond

	tracker := NewTracker(config, reporter, OperationFileWrite)

	total := int64(1000)
	if err := tracker.Start(ctx, &total, "Writing"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Send rapid updates
	for i := 0; i < 10; i++ {
		if err := tracker.Update(ctx, int64(i*100), "Writing..."); err != nil {
			t.Fatalf("Update failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // 10ms between updates
	}

	// Should get fewer progress events due to throttling
	progress := reporter.getProgress()
	if len(progress) >= 10 {
		t.Errorf("Expected throttling to reduce events, got %d", len(progress))
	}
	if len(progress) == 0 {
		t.Error("Expected at least some progress events")
	}

	if err := tracker.Complete(ctx, "Done"); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
}

func TestTrackerFailure(t *testing.T) {
	ctx := context.Background()
	reporter := newMockReporter()
	config := DefaultConfig()
	config.UpdateInterval = 0
	config.MinDuration = 0

	tracker := NewTracker(config, reporter, OperationPatchApply)

	if err := tracker.Start(ctx, nil, "Starting"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Simulate failure
	if err := tracker.Fail(ctx, "Something went wrong"); err != nil {
		t.Fatalf("Fail failed: %v", err)
	}

	completed := reporter.getCompleted()
	if len(completed) != 1 {
		t.Fatalf("Expected 1 completed event, got %d", len(completed))
	}
	if completed[0].Status != StatusFailed {
		t.Errorf("Expected status %s, got %s", StatusFailed, completed[0].Status)
	}
	if completed[0].Message != "Something went wrong" {
		t.Errorf("Expected message 'Something went wrong', got '%s'", completed[0].Message)
	}
}

func TestTrackerCancellation(t *testing.T) {
	ctx := context.Background()
	reporter := newMockReporter()
	config := DefaultConfig()
	config.UpdateInterval = 0
	config.MinDuration = 0

	tracker := NewTracker(config, reporter, OperationHistoryReconstruct)

	if err := tracker.Start(ctx, nil, "Loading history"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Simulate cancellation
	if err := tracker.Cancel(ctx, "User cancelled"); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	completed := reporter.getCompleted()
	if len(completed) != 1 {
		t.Fatalf("Expected 1 completed event, got %d", len(completed))
	}
	if completed[0].Status != StatusCancelled {
		t.Errorf("Expected status %s, got %s", StatusCancelled, completed[0].Status)
	}
}

func TestTrackerMultipleCompletions(t *testing.T) {
	ctx := context.Background()
	reporter := newMockReporter()
	config := DefaultConfig()
	config.UpdateInterval = 0
	config.MinDuration = 0

	tracker := NewTracker(config, reporter, OperationFileRead)

	if err := tracker.Start(ctx, nil, "Reading"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// First completion
	if err := tracker.Complete(ctx, "Done"); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Second completion should be ignored
	if err := tracker.Complete(ctx, "Done again"); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Only one completed event should be emitted
	completed := reporter.getCompleted()
	if len(completed) != 1 {
		t.Errorf("Expected 1 completed event, got %d", len(completed))
	}
}

func TestCalculatePercentage(t *testing.T) {
	tests := []struct {
		name     string
		current  int64
		total    int64
		expected float64
	}{
		{"zero total", 50, 0, 0},
		{"negative total", 50, -100, 0},
		{"25 percent", 25, 100, 25.0},
		{"50 percent", 50, 100, 50.0},
		{"75 percent", 75, 100, 75.0},
		{"100 percent", 100, 100, 100.0},
		{"over 100 percent", 150, 100, 100.0},
		{"negative current", -50, 100, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculatePercentage(tt.current, tt.total)
			if result != tt.expected {
				t.Errorf("calculatePercentage(%d, %d) = %.1f, expected %.1f",
					tt.current, tt.total, result, tt.expected)
			}
		})
	}
}

func TestTrackOperation(t *testing.T) {
	ctx := context.Background()
	reporter := newMockReporter()
	config := DefaultConfig()
	config.UpdateInterval = 0
	config.MinDuration = 0

	total := int64(100)
	operationRan := false

	err := TrackOperation(ctx, config, reporter, OperationFileWrite, &total, "Writing file", func(ctx context.Context, tracker *Tracker) error {
		operationRan = true
		// Simulate some work
		if err := tracker.Update(ctx, 50, "Half done"); err != nil {
			return err
		}
		if err := tracker.Update(ctx, 100, "Complete"); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		t.Fatalf("TrackOperation failed: %v", err)
	}
	if !operationRan {
		t.Error("Operation function was not executed")
	}

	// Verify events were emitted
	if len(reporter.getStarted()) != 1 {
		t.Error("Expected started event")
	}
	if len(reporter.getProgress()) != 2 {
		t.Errorf("Expected 2 progress events, got %d", len(reporter.getProgress()))
	}
	if len(reporter.getCompleted()) != 1 {
		t.Error("Expected completed event")
	}

	// Should have success status
	completed := reporter.getCompleted()
	if completed[0].Status != StatusSuccess {
		t.Errorf("Expected status %s, got %s", StatusSuccess, completed[0].Status)
	}
}

func TestTrackOperationWithError(t *testing.T) {
	ctx := context.Background()
	reporter := newMockReporter()
	config := DefaultConfig()
	config.UpdateInterval = 0
	config.MinDuration = 0

	err := TrackOperation(ctx, config, reporter, OperationPatchApply, nil, "Applying patch", func(ctx context.Context, tracker *Tracker) error {
		return context.DeadlineExceeded
	})

	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded error, got %v", err)
	}

	// Should have fail status
	completed := reporter.getCompleted()
	if len(completed) != 1 {
		t.Fatalf("Expected 1 completed event, got %d", len(completed))
	}
	if completed[0].Status != StatusFailed {
		t.Errorf("Expected status %s, got %s", StatusFailed, completed[0].Status)
	}
}

func TestNoOpReporter(t *testing.T) {
	ctx := context.Background()
	reporter := &NoOpReporter{}

	// Should not panic or error
	if err := reporter.ReportStarted(ctx, &StartedEvent{Op: OperationFileRead}); err != nil {
		t.Errorf("NoOpReporter.ReportStarted returned error: %v", err)
	}
	if err := reporter.ReportProgress(ctx, &ProgressEvent{Op: OperationFileRead}); err != nil {
		t.Errorf("NoOpReporter.ReportProgress returned error: %v", err)
	}
	if err := reporter.ReportCompleted(ctx, &CompletedEvent{Op: OperationFileRead}); err != nil {
		t.Errorf("NoOpReporter.ReportCompleted returned error: %v", err)
	}
}

func TestEstimateDuration(t *testing.T) {
	tests := []struct {
		name     string
		op       OperationType
		size     int64
		minDur   time.Duration
		maxDur   time.Duration
	}{
		{"small file read", OperationFileRead, 1024, 0, 10 * time.Millisecond},
		{"large file read", OperationFileRead, 100 * 1024 * 1024, 500 * time.Millisecond, 2 * time.Second},
		{"file write", OperationFileWrite, 50 * 1024 * 1024, 500 * time.Millisecond, 2 * time.Second},
		{"small patch", OperationPatchApply, 100, 0, 200 * time.Millisecond},
		{"large patch", OperationPatchApply, 10000, 5 * time.Second, 20 * time.Second},
		{"mcp discovery", OperationMcpDiscovery, 0, 1 * time.Second, 5 * time.Second},
		{"history reconstruct", OperationHistoryReconstruct, 1000, 5 * time.Second, 20 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimateDuration(tt.op, tt.size)
			if result < tt.minDur || result > tt.maxDur {
				t.Errorf("estimateDuration(%s, %d) = %v, expected between %v and %v",
					tt.op, tt.size, result, tt.minDur, tt.maxDur)
			}
		})
	}
}
