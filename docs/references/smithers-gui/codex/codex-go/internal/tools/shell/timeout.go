package shell

import (
	"context"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// TimeoutManager manages command timeouts and cancellation.
type TimeoutManager struct {
	mu       sync.Mutex
	timers   map[string]*time.Timer
	commands map[string]*exec.Cmd
}

// NewTimeoutManager creates a new timeout manager.
func NewTimeoutManager() *TimeoutManager {
	return &TimeoutManager{
		timers:   make(map[string]*time.Timer),
		commands: make(map[string]*exec.Cmd),
	}
}

// Register registers a command with the timeout manager.
func (tm *TimeoutManager) Register(callID string, cmd *exec.Cmd, timeout time.Duration) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.commands[callID] = cmd

	if timeout > 0 {
		timer := time.AfterFunc(timeout, func() {
			// Best effort termination on timeout
			_ = tm.Terminate(callID) // nolint:errcheck
		})
		tm.timers[callID] = timer
	}
}

// Unregister removes a command from the timeout manager.
func (tm *TimeoutManager) Unregister(callID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Stop and remove timer
	if timer, ok := tm.timers[callID]; ok {
		timer.Stop()
		delete(tm.timers, callID)
	}

	// Remove command
	delete(tm.commands, callID)
}

// Terminate terminates a command by call ID.
func (tm *TimeoutManager) Terminate(callID string) error {
	tm.mu.Lock()
	cmd, ok := tm.commands[callID]
	tm.mu.Unlock()

	if !ok || cmd.Process == nil {
		return nil
	}

	// Send SIGTERM first for graceful shutdown
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// If SIGTERM fails, try SIGKILL
		return cmd.Process.Kill()
	}

	// Wait a bit for graceful shutdown
	time.Sleep(100 * time.Millisecond)

	// Check if process is still running
	if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
		// Force kill if still running
		return cmd.Process.Kill()
	}

	return nil
}

// TerminateAll terminates all registered commands.
func (tm *TimeoutManager) TerminateAll() {
	tm.mu.Lock()
	callIDs := make([]string, 0, len(tm.commands))
	for callID := range tm.commands {
		callIDs = append(callIDs, callID)
	}
	tm.mu.Unlock()

	for _, callID := range callIDs {
		// Best effort termination
		_ = tm.Terminate(callID) // nolint:errcheck
	}
}

// SignalHandler handles OS signals for graceful shutdown.
type SignalHandler struct {
	mu       sync.Mutex
	handlers map[os.Signal][]func()
}

// NewSignalHandler creates a new signal handler.
func NewSignalHandler() *SignalHandler {
	return &SignalHandler{
		handlers: make(map[os.Signal][]func()),
	}
}

// Register registers a handler for a signal.
func (sh *SignalHandler) Register(sig os.Signal, handler func()) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	sh.handlers[sig] = append(sh.handlers[sig], handler)
}

// Handle handles a signal by calling all registered handlers.
func (sh *SignalHandler) Handle(sig os.Signal) {
	sh.mu.Lock()
	handlers := sh.handlers[sig]
	sh.mu.Unlock()

	for _, handler := range handlers {
		handler()
	}
}

// CancellationMonitor monitors context cancellation and terminates commands accordingly.
type CancellationMonitor struct {
	mu       sync.Mutex
	commands map[string]*exec.Cmd
	cancels  map[string]context.CancelFunc
}

// NewCancellationMonitor creates a new cancellation monitor.
func NewCancellationMonitor() *CancellationMonitor {
	return &CancellationMonitor{
		commands: make(map[string]*exec.Cmd),
		cancels:  make(map[string]context.CancelFunc),
	}
}

// Monitor monitors a context for cancellation and terminates the command when cancelled.
func (cm *CancellationMonitor) Monitor(ctx context.Context, callID string, cmd *exec.Cmd) {
	cm.mu.Lock()
	cm.commands[callID] = cmd
	cm.mu.Unlock()

	go func() {
		<-ctx.Done()
		// Best effort termination on cancel
		_ = cm.Terminate(callID) // nolint:errcheck
	}()
}

// Terminate terminates a command by call ID.
func (cm *CancellationMonitor) Terminate(callID string) error {
	cm.mu.Lock()
	cmd, ok := cm.commands[callID]
	cancel, hasCancel := cm.cancels[callID]
	cm.mu.Unlock()

	if hasCancel {
		cancel()
	}

	if !ok || cmd.Process == nil {
		return nil
	}

	return terminateProcess(cmd.Process)
}

// Unregister removes a command from the cancellation monitor.
func (cm *CancellationMonitor) Unregister(callID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	delete(cm.commands, callID)
	delete(cm.cancels, callID)
}

// terminateProcess terminates a process gracefully with fallback to force kill.
func terminateProcess(proc *os.Process) error {
	if proc == nil {
		return nil
	}

	// Try SIGTERM first for graceful shutdown
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		// If SIGTERM fails, force kill
		return proc.Kill()
	}

	// Give the process time to exit gracefully
	done := make(chan error, 1)
	go func() {
		_, err := proc.Wait()
		done <- err
	}()

	select {
	case <-time.After(2 * time.Second):
		// Timeout waiting for graceful exit, force kill
		return proc.Kill()
	case err := <-done:
		return err
	}
}

// WithTimeout creates a context with timeout that automatically cancels.
func WithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, timeout)
}

// WithDeadline creates a context with deadline that automatically cancels.
func WithDeadline(parent context.Context, deadline time.Time) (context.Context, context.CancelFunc) {
	return context.WithDeadline(parent, deadline)
}

// ExecutionContext holds context for command execution with timeout support.
type ExecutionContext struct {
	ctx       context.Context
	cancel    context.CancelFunc
	startTime time.Time
	timeout   time.Duration
}

// NewExecutionContext creates a new execution context with timeout.
func NewExecutionContext(parent context.Context, timeout time.Duration) *ExecutionContext {
	ctx, cancel := WithTimeout(parent, timeout)
	return &ExecutionContext{
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
		timeout:   timeout,
	}
}

// Context returns the underlying context.
func (ec *ExecutionContext) Context() context.Context {
	return ec.ctx
}

// Cancel cancels the execution context.
func (ec *ExecutionContext) Cancel() {
	if ec.cancel != nil {
		ec.cancel()
	}
}

// Elapsed returns the time elapsed since execution started.
func (ec *ExecutionContext) Elapsed() time.Duration {
	return time.Since(ec.startTime)
}

// Remaining returns the time remaining before timeout.
func (ec *ExecutionContext) Remaining() time.Duration {
	if ec.timeout <= 0 {
		return 0
	}
	remaining := ec.timeout - ec.Elapsed()
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsTimedOut returns true if the execution has timed out.
func (ec *ExecutionContext) IsTimedOut() bool {
	if ec.timeout <= 0 {
		return false
	}
	return ec.Elapsed() >= ec.timeout
}

// IsCancelled returns true if the context has been cancelled.
func (ec *ExecutionContext) IsCancelled() bool {
	return ec.ctx.Err() != nil
}
