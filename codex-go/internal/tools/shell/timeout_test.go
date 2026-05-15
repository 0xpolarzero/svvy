package shell

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestWithTimeout tests context timeout creation
func TestWithTimeout(t *testing.T) {
	parent := context.Background()

	// With timeout
	ctx, cancel := WithTimeout(parent, 100*time.Millisecond)
	defer cancel()

	assert.NotNil(t, ctx)

	// Should timeout
	<-ctx.Done()
	assert.Error(t, ctx.Err())
}

// TestWithTimeoutZero tests timeout with zero duration
func TestWithTimeoutZero(t *testing.T) {
	parent := context.Background()

	ctx, cancel := WithTimeout(parent, 0)
	defer cancel()

	assert.NotNil(t, ctx)

	// Should not timeout immediately
	select {
	case <-ctx.Done():
		t.Fatal("context should not be done immediately with zero timeout")
	case <-time.After(10 * time.Millisecond):
		// OK
	}
}

// TestWithDeadline tests context deadline creation
func TestWithDeadline(t *testing.T) {
	parent := context.Background()
	deadline := time.Now().Add(100 * time.Millisecond)

	ctx, cancel := WithDeadline(parent, deadline)
	defer cancel()

	assert.NotNil(t, ctx)

	// Should timeout
	<-ctx.Done()
	assert.Error(t, ctx.Err())
}

// TestExecutionContext tests execution context
func TestExecutionContext(t *testing.T) {
	parent := context.Background()

	// Create with timeout
	execCtx := NewExecutionContext(parent, 1*time.Second)
	defer execCtx.Cancel()

	assert.NotNil(t, execCtx)
	assert.NotNil(t, execCtx.Context())
	assert.False(t, execCtx.IsTimedOut())
	assert.False(t, execCtx.IsCancelled())

	// Check elapsed time
	time.Sleep(10 * time.Millisecond)
	elapsed := execCtx.Elapsed()
	assert.Greater(t, elapsed, 10*time.Millisecond)

	// Check remaining time
	remaining := execCtx.Remaining()
	assert.Greater(t, remaining, time.Duration(0))
	assert.Less(t, remaining, 1*time.Second)

	// Cancel
	execCtx.Cancel()
	time.Sleep(10 * time.Millisecond) // Give it time to propagate
	assert.True(t, execCtx.IsCancelled())
}

// TestExecutionContextNoTimeout tests execution context without timeout
func TestExecutionContextNoTimeout(t *testing.T) {
	parent := context.Background()

	execCtx := NewExecutionContext(parent, 0)
	defer execCtx.Cancel()

	assert.NotNil(t, execCtx)
	assert.False(t, execCtx.IsTimedOut())
	assert.Equal(t, time.Duration(0), execCtx.Remaining())
}

// TestExecutionContextTimeout tests execution context timeout
func TestExecutionContextTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	parent := context.Background()

	execCtx := NewExecutionContext(parent, 50*time.Millisecond)
	defer execCtx.Cancel()

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	assert.True(t, execCtx.IsTimedOut())
	assert.True(t, execCtx.IsCancelled())
	assert.Equal(t, time.Duration(0), execCtx.Remaining())
}

// TestTimeoutManager tests timeout manager
func TestTimeoutManager(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tm := NewTimeoutManager()

	// Unregister non-existent command should not panic
	tm.Unregister("non-existent")

	// Terminate non-existent command should not panic
	err := tm.Terminate("non-existent")
	assert.NoError(t, err)

	// TerminateAll should not panic with no commands
	tm.TerminateAll()
}

// TestCancellationMonitor tests cancellation monitor
func TestCancellationMonitor(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cm := NewCancellationMonitor()

	// Terminate non-existent command should not panic
	err := cm.Terminate("non-existent")
	assert.NoError(t, err)

	// Unregister non-existent command should not panic
	cm.Unregister("non-existent")
}

// TestSignalHandler tests signal handler
func TestSignalHandler(t *testing.T) {
	sh := NewSignalHandler()

	called := false
	handler := func() {
		called = true
	}

	// Register handler
	sh.Register(nil, handler)

	// Handle signal
	sh.Handle(nil)

	assert.True(t, called)
}

// TestSignalHandlerMultiple tests multiple signal handlers
func TestSignalHandlerMultiple(t *testing.T) {
	sh := NewSignalHandler()

	count := 0
	handler1 := func() { count++ }
	handler2 := func() { count++ }
	handler3 := func() { count++ }

	sh.Register(nil, handler1)
	sh.Register(nil, handler2)
	sh.Register(nil, handler3)

	sh.Handle(nil)

	assert.Equal(t, 3, count)
}

// BenchmarkExecutionContext benchmarks execution context creation
func BenchmarkExecutionContext(b *testing.B) {
	parent := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		execCtx := NewExecutionContext(parent, 1*time.Second)
		execCtx.Cancel()
	}
}
