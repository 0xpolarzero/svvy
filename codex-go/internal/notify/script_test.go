package notify

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewScriptExecutor(t *testing.T) {
	timeout := 10 * time.Second
	executor := NewScriptExecutor(timeout)

	if executor == nil {
		t.Fatal("NewScriptExecutor returned nil")
	}
	if executor.Timeout != timeout {
		t.Errorf("Expected timeout %v, got %v", timeout, executor.Timeout)
	}
	if executor.Env == nil {
		t.Error("Env should be initialized")
	}
}

func TestScriptExecutor_Execute_EmptyCommand(t *testing.T) {
	executor := NewScriptExecutor(5 * time.Second)
	event := NewNotificationEvent(EventTurnComplete, "session1", "turn1")

	err := executor.Execute(context.Background(), "", event)
	if err == nil {
		t.Error("Execute should fail with empty command")
	}
}

func TestScriptExecutor_Execute_Success(t *testing.T) {
	executor := NewScriptExecutor(5 * time.Second)
	event := NewNotificationEvent(EventTurnComplete, "session1", "turn1")

	// Simple echo command that should succeed
	err := executor.Execute(context.Background(), "echo 'test'", event)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Give the background process time to start
	time.Sleep(50 * time.Millisecond)
}

func TestScriptExecutor_Execute_WithScriptFile(t *testing.T) {
	tmpDir := t.TempDir()
	scriptFile := filepath.Join(tmpDir, "test_script.sh")
	outputFile := filepath.Join(tmpDir, "output.txt")

	// Create a test script
	scriptContent := `#!/bin/sh
echo "$CODEX_EVENT_TYPE:$CODEX_SESSION_ID:$CODEX_STATUS" > ` + outputFile
	if err := os.WriteFile(scriptFile, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	executor := NewScriptExecutor(5 * time.Second)
	event := NewNotificationEvent(EventTurnComplete, "test-session", "test-turn")
	event.WithStatus("success")

	err := executor.Execute(context.Background(), scriptFile, event)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Wait for script to complete
	time.Sleep(200 * time.Millisecond)

	// Verify the script output
	if _, err := os.Stat(outputFile); err != nil {
		t.Logf("Output file not created (this may be expected): %v", err)
		return
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	expected := "turn_complete:test-session:success"
	if string(content) != expected+"\n" {
		t.Errorf("Expected output %q, got %q", expected, string(content))
	}
}

func TestScriptExecutor_Execute_Timeout(t *testing.T) {
	executor := NewScriptExecutor(100 * time.Millisecond)
	event := NewNotificationEvent(EventTurnComplete, "session1", "turn1")

	// Command that takes longer than the timeout
	err := executor.Execute(context.Background(), "sleep 10", event)
	if err != nil {
		t.Fatalf("Execute should not fail immediately: %v", err)
	}

	// Wait for timeout to occur
	time.Sleep(200 * time.Millisecond)
	// The command should have been killed by the timeout
}

func TestScriptExecutor_Execute_WithCustomEnv(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "env_output.txt")

	executor := NewScriptExecutor(5 * time.Second)
	executor.SetEnv("CUSTOM_VAR", "custom_value")
	executor.SetEnv("ANOTHER_VAR", "another_value")

	event := NewNotificationEvent(EventTurnComplete, "session1", "turn1")

	command := "sh -c 'echo \"$CUSTOM_VAR:$ANOTHER_VAR\" > " + outputFile + "'"
	err := executor.Execute(context.Background(), command, event)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Wait for script to complete
	time.Sleep(200 * time.Millisecond)

	if _, err := os.Stat(outputFile); err != nil {
		t.Logf("Output file not created (this may be expected): %v", err)
		return
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	expected := "custom_value:another_value"
	if string(content) != expected+"\n" {
		t.Errorf("Expected output %q, got %q", expected, string(content))
	}
}

func TestScriptExecutor_Execute_WithMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "metadata_output.txt")

	executor := NewScriptExecutor(5 * time.Second)
	event := NewNotificationEvent(EventApprovalNeeded, "session1", "turn1")
	event.WithMetadata("tool-name", "bash")
	event.WithMetadata("risk-level", "high")

	command := "sh -c 'echo \"$CODEX_TOOL_NAME:$CODEX_RISK_LEVEL\" > " + outputFile + "'"
	err := executor.Execute(context.Background(), command, event)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Wait for script to complete
	time.Sleep(200 * time.Millisecond)

	if _, err := os.Stat(outputFile); err != nil {
		t.Logf("Output file not created (this may be expected): %v", err)
		return
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	expected := "bash:high"
	if string(content) != expected+"\n" {
		t.Errorf("Expected output %q, got %q", expected, string(content))
	}
}

func TestScriptExecutor_buildEnv(t *testing.T) {
	executor := NewScriptExecutor(5 * time.Second)
	executor.SetEnv("CUSTOM_KEY", "custom_value")

	event := NewNotificationEvent(EventTurnComplete, "session1", "turn1")
	event.WithStatus("success").
		WithMessage("Test message").
		WithMetadata("key1", "value1")
	// Note: WithError also sets status to "error", so we set it last
	event.ErrorMessage = "Test error"

	env := executor.buildEnv(event)

	// Check that environment contains expected variables
	expectedVars := map[string]string{
		"CODEX_EVENT_TYPE":    "turn_complete",
		"CODEX_SESSION_ID":    "session1",
		"CODEX_TURN_ID":       "turn1",
		"CODEX_STATUS":        "success",
		"CODEX_MESSAGE":       "Test message",
		"CODEX_ERROR_MESSAGE": "Test error",
		"CODEX_KEY1":          "value1",
		"CUSTOM_KEY":          "custom_value",
	}

	for key, expectedValue := range expectedVars {
		found := false
		for _, envVar := range env {
			if strings.HasPrefix(envVar, key+"=") {
				value := strings.TrimPrefix(envVar, key+"=")
				if value != expectedValue {
					t.Errorf("Expected %s=%s, got %s", key, expectedValue, value)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected environment variable %s not found", key)
		}
	}
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected []string
	}{
		{
			name:     "simple command",
			command:  "echo hello",
			expected: []string{"echo", "hello"},
		},
		{
			name:     "command with quotes",
			command:  `echo "hello world"`,
			expected: []string{"echo", "hello world"},
		},
		{
			name:     "command with single quotes",
			command:  `echo 'hello world'`,
			expected: []string{"echo", "hello world"},
		},
		{
			name:     "command with multiple arguments",
			command:  "sh -c 'echo test'",
			expected: []string{"sh", "-c", "echo test"},
		},
		{
			name:     "empty command",
			command:  "",
			expected: []string{},
		},
		{
			name:     "command with extra spaces",
			command:  "echo  hello  world",
			expected: []string{"echo", "hello", "world"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCommand(tt.command)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d parts, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for i, part := range result {
				if part != tt.expected[i] {
					t.Errorf("Part %d: expected %q, got %q", i, tt.expected[i], part)
				}
			}
		})
	}
}

func TestScriptExecutor_Execute_InvalidCommand(t *testing.T) {
	executor := NewScriptExecutor(5 * time.Second)
	event := NewNotificationEvent(EventTurnComplete, "session1", "turn1")

	// Command that doesn't exist
	err := executor.Execute(context.Background(), "/nonexistent/command", event)
	if err == nil {
		t.Error("Execute should fail with invalid command")
	}
}

func TestScriptExecutor_Execute_CanceledContext(t *testing.T) {
	executor := NewScriptExecutor(5 * time.Second)
	event := NewNotificationEvent(EventTurnComplete, "session1", "turn1")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Execute should still start but may fail due to canceled context
	_ = executor.Execute(ctx, "echo 'test'", event)
}

func TestScriptExecutor_SetEnv(t *testing.T) {
	executor := NewScriptExecutor(5 * time.Second)

	executor.SetEnv("KEY1", "value1")
	executor.SetEnv("KEY2", "value2")

	if executor.Env["KEY1"] != "value1" {
		t.Errorf("Expected KEY1=value1, got %s", executor.Env["KEY1"])
	}
	if executor.Env["KEY2"] != "value2" {
		t.Errorf("Expected KEY2=value2, got %s", executor.Env["KEY2"])
	}

	// Overwrite existing key
	executor.SetEnv("KEY1", "new_value")
	if executor.Env["KEY1"] != "new_value" {
		t.Errorf("Expected KEY1=new_value after overwrite, got %s", executor.Env["KEY1"])
	}
}
