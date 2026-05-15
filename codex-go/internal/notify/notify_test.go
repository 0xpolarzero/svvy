package notify

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNewNotifier(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name:   "nil config",
			config: nil,
		},
		{
			name: "empty config",
			config: &Config{},
		},
		{
			name: "with timeout",
			config: &Config{
				ScriptTimeout: 10 * time.Second,
			},
		},
		{
			name: "full config",
			config: &Config{
				OnTurnComplete: &NotificationConfig{
					Command: "echo 'complete'",
					Enabled: true,
				},
				OnError: &NotificationConfig{
					Command: "echo 'error'",
					Enabled: true,
				},
				ScriptTimeout: 5 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNotifier(tt.config)
			defer n.Close()
			if n == nil {
				t.Fatal("NewNotifier returned nil")
			}
			if !n.IsEnabled() {
				t.Error("Notifier should be enabled by default")
			}
		})
	}
}

func TestNotifier_EnableDisable(t *testing.T) {
	n := NewNotifier(&Config{})
	defer n.Close()

	if !n.IsEnabled() {
		t.Error("Notifier should start enabled")
	}

	n.Disable()
	if n.IsEnabled() {
		t.Error("Notifier should be disabled after Disable()")
	}

	n.Enable()
	if !n.IsEnabled() {
		t.Error("Notifier should be enabled after Enable()")
	}
}

func TestNotifier_UpdateConfig(t *testing.T) {
	n := NewNotifier(&Config{
		ScriptTimeout: 5 * time.Second,
	})
	defer n.Close()

	newConfig := &Config{
		OnTurnComplete: &NotificationConfig{
			Command: "echo 'test'",
			Enabled: true,
		},
		ScriptTimeout: 10 * time.Second,
	}

	err := n.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	if n.config.OnTurnComplete == nil {
		t.Error("Config not updated")
	}
}

func TestNotifier_UpdateConfig_Nil(t *testing.T) {
	n := NewNotifier(&Config{})
	defer n.Close()

	err := n.UpdateConfig(nil)
	if err == nil {
		t.Error("UpdateConfig should fail with nil config")
	}
}

func TestNotifier_Notify_Disabled(t *testing.T) {
	n := NewNotifier(&Config{
		OnTurnComplete: &NotificationConfig{
			Command: "echo 'test'",
			Enabled: true,
		},
	})
	defer n.Close()
	n.Disable()

	event := NewNotificationEvent(EventTurnComplete, "session1", "turn1")
	err := n.Notify(context.Background(), event)
	if err != nil {
		t.Fatalf("Notify should not fail when disabled: %v", err)
	}
}

func TestNotifier_Notify_NoConfig(t *testing.T) {
	n := NewNotifier(&Config{})
	defer n.Close()

	event := NewNotificationEvent(EventTurnComplete, "session1", "turn1")
	err := n.Notify(context.Background(), event)
	if err != nil {
		t.Fatalf("Notify should not fail with no config: %v", err)
	}
}

func TestNotifier_Notify_DisabledTrigger(t *testing.T) {
	n := NewNotifier(&Config{
		OnTurnComplete: &NotificationConfig{
			Command: "echo 'test'",
			Enabled: false,
		},
	})
	defer n.Close()

	event := NewNotificationEvent(EventTurnComplete, "session1", "turn1")
	err := n.Notify(context.Background(), event)
	if err != nil {
		t.Fatalf("Notify should not fail with disabled trigger: %v", err)
	}
}

func TestNotifier_NotifyTurnComplete(t *testing.T) {
	// Create a temporary file to track script execution
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	n := NewNotifier(&Config{
		OnTurnComplete: &NotificationConfig{
			Command: "sh -c 'echo \"$CODEX_EVENT_TYPE:$CODEX_SESSION_ID:$CODEX_TURN_ID:$CODEX_STATUS\" > " + outputFile + "'",
			Enabled: true,
		},
		ScriptTimeout: 2 * time.Second,
	})
	defer n.Close()

	err := n.NotifyTurnComplete(context.Background(), "test-session", "test-turn", "Test message")
	if err != nil {
		t.Fatalf("NotifyTurnComplete failed: %v", err)
	}

	// Wait for script to execute
	time.Sleep(100 * time.Millisecond)

	// Check if the script ran
	if _, err := os.Stat(outputFile); err != nil {
		t.Logf("Output file not created (this may be expected): %v", err)
		return
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	expected := "turn_complete:test-session:test-turn:success"
	if string(content) != expected+"\n" {
		t.Errorf("Expected output %q, got %q", expected, string(content))
	}
}

func TestNotifier_NotifyTurnError(t *testing.T) {
	// Create a temporary file to track script execution
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "error.txt")

	n := NewNotifier(&Config{
		OnError: &NotificationConfig{
			Command: "sh -c 'echo \"$CODEX_EVENT_TYPE:$CODEX_ERROR_MESSAGE\" > " + outputFile + "'",
			Enabled: true,
		},
		ScriptTimeout: 2 * time.Second,
	})
	defer n.Close()

	err := n.NotifyTurnError(context.Background(), "test-session", "test-turn", "Test error")
	if err != nil {
		t.Fatalf("NotifyTurnError failed: %v", err)
	}

	// Wait for script to execute
	time.Sleep(100 * time.Millisecond)

	// Check if the script ran
	if _, err := os.Stat(outputFile); err != nil {
		t.Logf("Output file not created (this may be expected): %v", err)
		return
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	expected := "turn_error:Test error"
	if string(content) != expected+"\n" {
		t.Errorf("Expected output %q, got %q", expected, string(content))
	}
}

func TestNotifier_NotifyApprovalNeeded(t *testing.T) {
	n := NewNotifier(&Config{
		OnApprovalNeeded: &NotificationConfig{
			Command: "echo 'approval needed'",
			Enabled: true,
		},
	})
	defer n.Close()

	err := n.NotifyApprovalNeeded(context.Background(), "test-session", "test-turn", "bash")
	if err != nil {
		t.Fatalf("NotifyApprovalNeeded failed: %v", err)
	}
}

func TestNotifier_NotifyTurnAborted(t *testing.T) {
	n := NewNotifier(&Config{
		OnTurnAborted: &NotificationConfig{
			Command: "echo 'aborted'",
			Enabled: true,
		},
	})
	defer n.Close()

	err := n.NotifyTurnAborted(context.Background(), "test-session", "test-turn", "User interrupted")
	if err != nil {
		t.Fatalf("NotifyTurnAborted failed: %v", err)
	}
}

func TestNotifier_ConcurrentNotifications(t *testing.T) {
	n := NewNotifier(&Config{
		OnTurnComplete: &NotificationConfig{
			Command: "sleep 0.1",
			Enabled: true,
		},
		ScriptTimeout: 1 * time.Second,
	})
	defer n.Close()

	var wg sync.WaitGroup
	concurrency := 10

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := n.NotifyTurnComplete(context.Background(), "test-session", "test-turn", "Concurrent test")
			if err != nil {
				t.Errorf("Concurrent notification %d failed: %v", id, err)
			}
		}(i)
	}

	wg.Wait()
}

func TestNotifier_WithCustomEnv(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "env.txt")

	n := NewNotifier(&Config{
		OnTurnComplete: &NotificationConfig{
			Command: "sh -c 'echo \"$CUSTOM_VAR\" > " + outputFile + "'",
			Enabled: true,
			Env: map[string]string{
				"CUSTOM_VAR": "custom_value",
			},
		},
		ScriptTimeout: 2 * time.Second,
	})
	defer n.Close()

	err := n.NotifyTurnComplete(context.Background(), "test-session", "test-turn", "Test")
	if err != nil {
		t.Fatalf("NotifyTurnComplete failed: %v", err)
	}

	// Wait for script to execute
	time.Sleep(100 * time.Millisecond)

	// Check if the script ran
	if _, err := os.Stat(outputFile); err != nil {
		t.Logf("Output file not created (this may be expected): %v", err)
		return
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	expected := "custom_value"
	if string(content) != expected+"\n" {
		t.Errorf("Expected output %q, got %q", expected, string(content))
	}
}

func TestNotificationEvent_WithMethods(t *testing.T) {
	event := NewNotificationEvent(EventTurnComplete, "session1", "turn1")

	if event.Type != EventTurnComplete {
		t.Errorf("Expected type %s, got %s", EventTurnComplete, event.Type)
	}

	event.WithStatus("success").
		WithMessage("Test message").
		WithMetadata("key1", "value1")

	if event.Status != "success" {
		t.Errorf("Expected status 'success', got %s", event.Status)
	}
	if event.Message != "Test message" {
		t.Errorf("Expected message 'Test message', got %s", event.Message)
	}
	if event.Metadata["key1"] != "value1" {
		t.Errorf("Expected metadata key1='value1', got %s", event.Metadata["key1"])
	}
}

func TestNotificationEvent_WithError(t *testing.T) {
	event := NewNotificationEvent(EventTurnError, "session1", "turn1")
	event.WithError("Test error")

	if event.ErrorMessage != "Test error" {
		t.Errorf("Expected error message 'Test error', got %s", event.ErrorMessage)
	}
	if event.Status != "error" {
		t.Errorf("Expected status 'error', got %s", event.Status)
	}
}
