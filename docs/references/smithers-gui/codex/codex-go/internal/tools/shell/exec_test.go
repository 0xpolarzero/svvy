package shell

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// TestIsCommandAvailable tests command availability check
func TestIsCommandAvailable(t *testing.T) {
	// Common commands that should be available on most systems
	assert.True(t, IsCommandAvailable("sh"))
	assert.True(t, IsCommandAvailable("echo"))

	// Command that shouldn't exist
	assert.False(t, IsCommandAvailable("this-command-definitely-does-not-exist-12345"))
}

// TestSanitizeCommand tests command sanitization
func TestSanitizeCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal command",
			input:    "echo hello",
			expected: "echo hello",
		},
		{
			name:     "command with leading/trailing spaces",
			input:    "  echo hello  ",
			expected: "echo hello",
		},
		{
			name:     "command with null bytes",
			input:    "echo\x00hello",
			expected: "echohello",
		},
		{
			name:     "empty command",
			input:    "",
			expected: "",
		},
		{
			name:     "command with multiple spaces",
			input:    "echo    hello",
			expected: "echo    hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeCommand(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCommandExecutorNew tests executor creation
func TestCommandExecutorNew(t *testing.T) {
	executor := NewCommandExecutor()
	assert.NotNil(t, executor)
}

// TestCommandExecutorExecuteWithTimeout tests execution with timeout
func TestCommandExecutorExecuteWithTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	executor := NewCommandExecutor()
	ctx := context.Background()

	spec := &CommandSpec{
		Command:          []string{"sh", "-c", "echo test"},
		WorkingDirectory: "/tmp",
		CallID:           "test-call",
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
		SandboxAttempt: &runtime.SandboxAttempt{
			Type:             runtime.SandboxNone,
			Policy:           runtime.SandboxDangerFullAccess,
			WorkingDirectory: "/tmp",
		},
		StartTime: time.Now(),
	}

	// Execute with timeout
	resp, err := executor.ExecuteWithTimeout(ctx, spec, execCtx, 5*time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Contains(t, resp.Content, "test")
}

// TestCommandExecutorExecuteWithTimeoutExpired tests timeout expiration
func TestCommandExecutorExecuteWithTimeoutExpired(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	executor := NewCommandExecutor()
	ctx := context.Background()

	spec := &CommandSpec{
		Command:          []string{"sh", "-c", "sleep 10"},
		WorkingDirectory: "/tmp",
		CallID:           "test-call",
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
		SandboxAttempt: &runtime.SandboxAttempt{
			Type:             runtime.SandboxNone,
			Policy:           runtime.SandboxDangerFullAccess,
			WorkingDirectory: "/tmp",
		},
		StartTime: time.Now(),
	}

	// Execute with very short timeout
	_, err := executor.ExecuteWithTimeout(ctx, spec, execCtx, 100*time.Millisecond)
	assert.Error(t, err)

	toolErr, ok := err.(*runtime.ToolError)
	assert.True(t, ok)
	assert.Equal(t, runtime.ErrorTimeout, toolErr.Kind)
}

// TestCommandExecutorExecuteWithZeroTimeout tests zero timeout (no timeout)
func TestCommandExecutorExecuteWithZeroTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	executor := NewCommandExecutor()
	ctx := context.Background()

	spec := &CommandSpec{
		Command:          []string{"sh", "-c", "echo test"},
		WorkingDirectory: "/tmp",
		CallID:           "test-call",
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
		SandboxAttempt: &runtime.SandboxAttempt{
			Type:             runtime.SandboxNone,
			Policy:           runtime.SandboxDangerFullAccess,
			WorkingDirectory: "/tmp",
		},
		StartTime: time.Now(),
	}

	// Execute with zero timeout (no timeout)
	resp, err := executor.ExecuteWithTimeout(ctx, spec, execCtx, 0)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestCommandExecutorExecuteCommandNotFound tests command not found error
func TestCommandExecutorExecuteCommandNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	executor := NewCommandExecutor()
	ctx := context.Background()

	spec := &CommandSpec{
		Command:          []string{"this-command-does-not-exist-12345"},
		WorkingDirectory: "/tmp",
		CallID:           "test-call",
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
		SandboxAttempt: &runtime.SandboxAttempt{
			Type:             runtime.SandboxNone,
			Policy:           runtime.SandboxDangerFullAccess,
			WorkingDirectory: "/tmp",
		},
		StartTime: time.Now(),
	}

	_, err := executor.Execute(ctx, spec, execCtx)
	assert.Error(t, err)

	toolErr, ok := err.(*runtime.ToolError)
	assert.True(t, ok)
	assert.Equal(t, runtime.ErrorExecution, toolErr.Kind)
}

// TestCommandExecutorExecuteEmptyCommand tests empty command error
func TestCommandExecutorExecuteEmptyCommand(t *testing.T) {
	executor := NewCommandExecutor()
	ctx := context.Background()

	spec := &CommandSpec{
		Command:          []string{},
		WorkingDirectory: "/tmp",
		CallID:           "test-call",
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
		SandboxAttempt: &runtime.SandboxAttempt{
			Type:             runtime.SandboxNone,
			Policy:           runtime.SandboxDangerFullAccess,
			WorkingDirectory: "/tmp",
		},
		StartTime: time.Now(),
	}

	_, err := executor.Execute(ctx, spec, execCtx)
	assert.Error(t, err)

	toolErr, ok := err.(*runtime.ToolError)
	assert.True(t, ok)
	assert.Equal(t, runtime.ErrorInvalidArguments, toolErr.Kind)
}

// TestCommandExecutorExecuteBinaryOutput tests execution of commands producing binary output
func TestCommandExecutorExecuteBinaryOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	executor := NewCommandExecutor()
	ctx := context.Background()

	tests := []struct {
		name        string
		command     []string
		expectBinary bool
	}{
		{
			name:        "gzip output",
			command:     []string{"sh", "-c", "echo 'test data' | gzip"},
			expectBinary: true,
		},
		{
			name:        "tar output",
			command:     []string{"sh", "-c", "echo 'test' > /tmp/test.txt && tar -cf - /tmp/test.txt 2>/dev/null"},
			expectBinary: true,
		},
		{
			name:        "printf binary",
			command:     []string{"sh", "-c", "printf '\\x00\\x01\\x02\\x03\\x04'"},
			expectBinary: true,
		},
		{
			name:        "text output",
			command:     []string{"echo", "hello world"},
			expectBinary: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &CommandSpec{
				Command:          tt.command,
				WorkingDirectory: "/tmp",
				CallID:           "test-binary",
			}

			execCtx := &runtime.ExecutionContext{
				SessionID: "test-session",
				TurnID:    "test-turn",
				SandboxAttempt: &runtime.SandboxAttempt{
					Type:             runtime.SandboxNone,
					Policy:           runtime.SandboxDangerFullAccess,
					WorkingDirectory: "/tmp",
				},
				StartTime: time.Now(),
			}

			resp, err := executor.Execute(ctx, spec, execCtx)

			// Some commands may not be available on all systems
			if err != nil {
				toolErr, ok := err.(*runtime.ToolError)
				if ok && toolErr.Kind == runtime.ErrorExecution {
					t.Skipf("Command not available: %v", err)
				}
				t.Fatalf("Unexpected error: %v", err)
			}

			assert.NotNil(t, resp)
			assert.NotEmpty(t, resp.Content)

			// Verify binary detection matches expectation
			isBinary := IsBinaryData([]byte(resp.Content))
			if tt.expectBinary {
				assert.True(t, isBinary, "Expected binary output but got text")
			}
		})
	}
}

// TestCommandExecutorExecuteMixedOutput tests commands that produce both text and binary
func TestCommandExecutorExecuteMixedOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	executor := NewCommandExecutor()
	ctx := context.Background()

	// Command that outputs text to stderr and binary to stdout
	spec := &CommandSpec{
		Command:          []string{"sh", "-c", "echo 'Status: processing' >&2; printf '\\x00\\x01\\x02\\x03'"},
		WorkingDirectory: "/tmp",
		CallID:           "test-mixed",
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
		SandboxAttempt: &runtime.SandboxAttempt{
			Type:             runtime.SandboxNone,
			Policy:           runtime.SandboxDangerFullAccess,
			WorkingDirectory: "/tmp",
		},
		StartTime: time.Now(),
	}

	resp, err := executor.Execute(ctx, spec, execCtx)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// BenchmarkCommandExecutor benchmarks command execution
func BenchmarkCommandExecutor(b *testing.B) {
	executor := NewCommandExecutor()
	ctx := context.Background()

	spec := &CommandSpec{
		Command:          []string{"sh", "-c", "echo benchmark"},
		WorkingDirectory: "/tmp",
		CallID:           "bench-call",
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "bench-session",
		TurnID:    "bench-turn",
		SandboxAttempt: &runtime.SandboxAttempt{
			Type:             runtime.SandboxNone,
			Policy:           runtime.SandboxDangerFullAccess,
			WorkingDirectory: "/tmp",
		},
		StartTime: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := executor.Execute(ctx, spec, execCtx)
		if err != nil {
			b.Fatal(err)
		}
	}
}
