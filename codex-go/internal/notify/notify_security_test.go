package notify

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestValidateNotificationCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid command",
			command: "echo",
			wantErr: false,
		},
		{
			name:    "command injection semicolon",
			command: "echo; rm -rf /",
			wantErr: true,
			errMsg:  "metacharacter",
		},
		{
			name:    "command injection and",
			command: "echo && evil",
			wantErr: true,
			errMsg:  "metacharacter",
		},
		{
			name:    "command injection or",
			command: "echo || evil",
			wantErr: true,
			errMsg:  "metacharacter",
		},
		{
			name:    "command injection pipe",
			command: "echo | cat",
			wantErr: true,
			errMsg:  "metacharacter",
		},
		{
			name:    "command injection redirect",
			command: "echo > /tmp/evil",
			wantErr: true,
			errMsg:  "metacharacter",
		},
		{
			name:    "command injection backtick",
			command: "echo `whoami`",
			wantErr: true,
			errMsg:  "metacharacter",
		},
		{
			name:    "command injection dollar",
			command: "echo $(whoami)",
			wantErr: true,
			errMsg:  "metacharacter",
		},
		{
			name:    "path traversal",
			command: "../../../bin/evil",
			wantErr: true,
			errMsg:  "traversal",
		},
		{
			name:    "null byte injection",
			command: "echo\x00--help",
			wantErr: true,
			errMsg:  "null byte",
		},
		{
			name:    "empty command",
			command: "",
			wantErr: true,
			errMsg:  "empty",
		},
		{
			name:    "whitespace only command",
			command: "   ",
			wantErr: true,
			errMsg:  "empty",
		},
		{
			name:    "nonexistent command",
			command: "this-command-does-not-exist-anywhere-12345",
			wantErr: true,
			errMsg:  "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNotificationCommand(tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateNotificationCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && err != nil && !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errMsg)) {
				t.Errorf("expected error containing '%s', got '%v'", tt.errMsg, err)
			}
		})
	}
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(10, 20) // 10 per second, burst of 20

	// Should allow first request
	if !rl.Allow() {
		t.Error("expected first request to be allowed")
	}

	// Exhaust the bucket
	for i := 0; i < 19; i++ {
		rl.Allow()
	}

	// Next should be denied
	if rl.Allow() {
		t.Error("expected request to be denied after exhausting bucket")
	}

	// Wait and try again
	time.Sleep(150 * time.Millisecond)
	if !rl.Allow() {
		t.Error("expected request to be allowed after waiting")
	}
}

func TestRateLimiter_Wait(t *testing.T) {
	rl := NewRateLimiter(10, 1) // 10 per second, capacity of 1

	// Exhaust the bucket
	if !rl.Allow() {
		t.Fatal("expected first request to be allowed")
	}

	// Now wait should block briefly
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := rl.Wait(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Wait() error = %v", err)
	}

	// Should have waited at least ~100ms (1/10 second)
	if elapsed < 50*time.Millisecond {
		t.Errorf("Wait() didn't wait long enough: %v", elapsed)
	}
}

func TestRateLimiter_WaitContextCancellation(t *testing.T) {
	rl := NewRateLimiter(1, 1) // 1 per second

	// Exhaust the bucket
	rl.Allow()

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := rl.Wait(ctx)
	if err == nil {
		t.Error("expected Wait() to return error when context is cancelled")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got: %v", err)
	}
}

func TestWorkerPool(t *testing.T) {
	pool := NewWorkerPool(2, 10)
	defer pool.Close(5 * time.Second)

	// Submit some jobs
	processed := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		job := &NotificationJob{
			ctx: context.Background(),
			config: &NotificationConfig{
				Command: "echo test",
				Enabled: true,
			},
			event:    NewNotificationEvent(EventTurnComplete, "session", "turn"),
			executor: NewScriptExecutor(DefaultScriptTimeout),
		}

		if err := pool.Submit(job); err != nil {
			t.Errorf("failed to submit job: %v", err)
		}
		processed <- true
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)
}

func TestWorkerPool_QueueFull(t *testing.T) {
	pool := NewWorkerPool(1, 2) // Small queue
	defer pool.Close(5 * time.Second)

	// Fill the queue
	for i := 0; i < 2; i++ {
		job := &NotificationJob{
			ctx: context.Background(),
			config: &NotificationConfig{
				Command: "sleep 1",
				Enabled: true,
			},
			event:    NewNotificationEvent(EventTurnComplete, "session", "turn"),
			executor: NewScriptExecutor(DefaultScriptTimeout),
		}
		pool.Submit(job)
	}

	// This should fail
	job := &NotificationJob{
		ctx: context.Background(),
		config: &NotificationConfig{
			Command: "echo test",
			Enabled: true,
		},
		event:    NewNotificationEvent(EventTurnComplete, "session", "turn"),
		executor: NewScriptExecutor(DefaultScriptTimeout),
	}

	err := pool.Submit(job)
	if err == nil {
		t.Error("expected Submit() to fail when queue is full")
	}
}

func TestWorkerPool_GracefulShutdown(t *testing.T) {
	defer goleak.VerifyNone(t)

	pool := NewWorkerPool(2, 10)

	// Submit some jobs
	for i := 0; i < 5; i++ {
		job := &NotificationJob{
			ctx: context.Background(),
			config: &NotificationConfig{
				Command: "echo test",
				Enabled: true,
			},
			event:    NewNotificationEvent(EventTurnComplete, "session", "turn"),
			executor: NewScriptExecutor(DefaultScriptTimeout),
		}
		pool.Submit(job)
	}

	// Close should wait for workers
	err := pool.Close(5 * time.Second)
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestNotifier_ResourceCleanup(t *testing.T) {
	defer goleak.VerifyNone(t)

	config := &Config{
		OnTurnComplete: &NotificationConfig{
			Command: "echo test",
			Enabled: true,
		},
	}

	notifier := NewNotifier(config)

	// Send some notifications
	for i := 0; i < 5; i++ {
		notifier.NotifyTurnComplete(context.Background(), "session", "turn", "message")
	}

	// Wait a bit for processing
	time.Sleep(200 * time.Millisecond)

	// Close and verify no goroutine leaks
	if err := notifier.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestNotifier_CommandValidation(t *testing.T) {
	defer goleak.VerifyNone(t)

	config := &Config{
		OnTurnComplete: &NotificationConfig{
			Command: "echo; rm -rf /", // Malicious command
			Enabled: true,
		},
	}

	notifier := NewNotifier(config)
	defer notifier.Close()

	// This should not execute due to validation failure
	err := notifier.NotifyTurnComplete(context.Background(), "session", "turn", "message")
	if err != nil {
		t.Errorf("Notify should not return error for validation failure: %v", err)
	}

	// Give it time to potentially execute (it shouldn't)
	time.Sleep(100 * time.Millisecond)
}

func TestNotifier_RateLimiting(t *testing.T) {
	defer goleak.VerifyNone(t)

	config := &Config{
		OnTurnComplete: &NotificationConfig{
			Command: "echo test",
			Enabled: true,
		},
	}

	notifier := NewNotifier(config)
	defer notifier.Close()

	// Send more notifications than the rate limit allows rapidly
	// The rate limiter should drop some notifications
	for i := 0; i < 50; i++ {
		notifier.NotifyTurnComplete(context.Background(), "session", "turn", "message")
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Test that rate limiter is functional by checking it directly
	limiter := notifier.rateLimiter

	// Exhaust the bucket
	allowed := 0
	for i := 0; i < 100; i++ {
		if limiter.Allow() {
			allowed++
		}
	}

	// Should not allow all 100 requests immediately
	if allowed > 25 {
		t.Errorf("rate limiter allowed too many requests: %d (expected < 25)", allowed)
	}
}

func TestNotifier_ClosedState(t *testing.T) {
	defer goleak.VerifyNone(t)

	config := &Config{
		OnTurnComplete: &NotificationConfig{
			Command: "echo test",
			Enabled: true,
		},
	}

	notifier := NewNotifier(config)
	notifier.Close()

	// Notifications after close should be no-op
	err := notifier.NotifyTurnComplete(context.Background(), "session", "turn", "message")
	if err != nil {
		t.Errorf("Notify after Close should not return error: %v", err)
	}

	// Double close should be safe
	err = notifier.Close()
	if err != nil {
		t.Errorf("double Close() should not error: %v", err)
	}
}

func TestNotifier_DisabledState(t *testing.T) {
	defer goleak.VerifyNone(t)

	config := &Config{
		OnTurnComplete: &NotificationConfig{
			Command: "echo test",
			Enabled: false, // Disabled
		},
	}

	notifier := NewNotifier(config)
	defer notifier.Close()

	// Should be no-op
	err := notifier.NotifyTurnComplete(context.Background(), "session", "turn", "message")
	if err != nil {
		t.Errorf("Notify with disabled config should not return error: %v", err)
	}
}

func TestNotifier_ConcurrentNotificationsWithWorkerPool(t *testing.T) {
	defer goleak.VerifyNone(t)

	config := &Config{
		OnTurnComplete: &NotificationConfig{
			Command: "echo test",
			Enabled: true,
		},
	}

	notifier := NewNotifier(config)
	defer notifier.Close()

	// Send notifications from multiple goroutines
	const numGoroutines = 10
	const numNotifications = 5

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numNotifications; j++ {
				notifier.NotifyTurnComplete(context.Background(), "session", "turn", "message")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)
}

func TestNotifier_UpdateConfigWhileRunning(t *testing.T) {
	defer goleak.VerifyNone(t)

	config := &Config{
		OnTurnComplete: &NotificationConfig{
			Command: "echo test1",
			Enabled: true,
		},
	}

	notifier := NewNotifier(config)
	defer notifier.Close()

	// Send some notifications
	notifier.NotifyTurnComplete(context.Background(), "session", "turn", "message")

	// Update config
	newConfig := &Config{
		OnTurnComplete: &NotificationConfig{
			Command: "echo test2",
			Enabled: true,
		},
	}
	err := notifier.UpdateConfig(newConfig)
	if err != nil {
		t.Errorf("UpdateConfig() error = %v", err)
	}

	// Send more notifications
	notifier.NotifyTurnComplete(context.Background(), "session", "turn", "message")

	time.Sleep(200 * time.Millisecond)
}

func TestNotifier_AllEventTypes(t *testing.T) {
	defer goleak.VerifyNone(t)

	config := &Config{
		OnTurnComplete: &NotificationConfig{
			Command: "echo complete",
			Enabled: true,
		},
		OnError: &NotificationConfig{
			Command: "echo error",
			Enabled: true,
		},
		OnApprovalNeeded: &NotificationConfig{
			Command: "echo approval",
			Enabled: true,
		},
		OnTurnAborted: &NotificationConfig{
			Command: "echo aborted",
			Enabled: true,
		},
	}

	notifier := NewNotifier(config)
	defer notifier.Close()

	// Test all event types
	notifier.NotifyTurnComplete(context.Background(), "s1", "t1", "done")
	notifier.NotifyTurnError(context.Background(), "s2", "t2", "error")
	notifier.NotifyApprovalNeeded(context.Background(), "s3", "t3", "tool")
	notifier.NotifyTurnAborted(context.Background(), "s4", "t4", "cancelled")

	time.Sleep(500 * time.Millisecond)
}

func TestNotifier_EnableDisableWithWorkerPool(t *testing.T) {
	defer goleak.VerifyNone(t)

	config := &Config{
		OnTurnComplete: &NotificationConfig{
			Command: "echo test",
			Enabled: true,
		},
	}

	notifier := NewNotifier(config)
	defer notifier.Close()

	// Should be enabled initially
	if !notifier.IsEnabled() {
		t.Error("notifier should be enabled initially")
	}

	// Disable
	notifier.Disable()
	if notifier.IsEnabled() {
		t.Error("notifier should be disabled")
	}

	// Notifications should be no-op
	notifier.NotifyTurnComplete(context.Background(), "session", "turn", "message")

	// Enable again
	notifier.Enable()
	if !notifier.IsEnabled() {
		t.Error("notifier should be enabled")
	}

	notifier.NotifyTurnComplete(context.Background(), "session", "turn", "message")

	time.Sleep(200 * time.Millisecond)
}

func TestNotifier_NilConfig(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Should not panic with nil config
	notifier := NewNotifier(nil)
	defer notifier.Close()

	// Should be no-op
	err := notifier.NotifyTurnComplete(context.Background(), "session", "turn", "message")
	if err != nil {
		t.Errorf("Notify with nil config should not return error: %v", err)
	}
}

func TestNotifier_CustomEnvironment(t *testing.T) {
	defer goleak.VerifyNone(t)

	config := &Config{
		OnTurnComplete: &NotificationConfig{
			Command: "echo test",
			Enabled: true,
			Env: map[string]string{
				"CUSTOM_VAR": "custom_value",
			},
		},
	}

	notifier := NewNotifier(config)
	defer notifier.Close()

	notifier.NotifyTurnComplete(context.Background(), "session", "turn", "message")

	time.Sleep(200 * time.Millisecond)
}

func BenchmarkNotifier_Notify(b *testing.B) {
	config := &Config{
		OnTurnComplete: &NotificationConfig{
			Command: "echo test",
			Enabled: true,
		},
	}

	notifier := NewNotifier(config)
	defer notifier.Close()

	event := NewNotificationEvent(EventTurnComplete, "session", "turn")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		notifier.Notify(context.Background(), event)
	}
}

func BenchmarkRateLimiter_Allow(b *testing.B) {
	rl := NewRateLimiter(1000, 2000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Allow()
	}
}

func BenchmarkWorkerPool_Submit(b *testing.B) {
	pool := NewWorkerPool(5, 1000)
	defer pool.Close(5 * time.Second)

	job := &NotificationJob{
		ctx: context.Background(),
		config: &NotificationConfig{
			Command: "echo test",
			Enabled: true,
		},
		event:    NewNotificationEvent(EventTurnComplete, "session", "turn"),
		executor: NewScriptExecutor(DefaultScriptTimeout),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Submit(job)
	}
}
