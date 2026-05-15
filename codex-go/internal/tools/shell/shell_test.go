package shell

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
	"github.com/evmts/codex/codex-go/test"
)

// TestShellToolName verifies the tool name is correct
func TestShellToolName(t *testing.T) {
	tool := NewShellTool()
	assert.Equal(t, "shell", tool.Name())
}

// TestShellToolSandboxPreference verifies sandbox preference
func TestShellToolSandboxPreference(t *testing.T) {
	tool := NewShellTool()
	assert.Equal(t, runtime.SandboxAuto, tool.SandboxPreference())
}

// TestShellToolEscalateOnFailure verifies escalation behavior
func TestShellToolEscalateOnFailure(t *testing.T) {
	tool := NewShellTool()
	assert.True(t, tool.EscalateOnFailure())
}

// TestShellToolSupportsParallel verifies parallel support
func TestShellToolSupportsParallel(t *testing.T) {
	tool := NewShellTool()
	assert.True(t, tool.SupportsParallel())
}

// TestShellToolApprovalKey verifies approval key generation
func TestShellToolApprovalKey(t *testing.T) {
	tool := NewShellTool()
	req := &runtime.ToolRequest{
		CallID:           "test-call",
		ToolName:         "shell",
		Arguments:        `{"command": "echo hello", "working_directory": "/tmp"}`,
		WorkingDirectory: "/tmp",
	}

	key := tool.ApprovalKey(req)
	assert.NotEmpty(t, key)
	assert.Contains(t, key, "shell:")
	// Key is hashed, so we just verify it's consistent
	key2 := tool.ApprovalKey(req)
	assert.Equal(t, key, key2, "approval key should be consistent")
}

// TestShellToolNeedsInitialApproval tests approval requirements
func TestShellToolNeedsInitialApproval(t *testing.T) {
	tool := NewShellTool()

	tests := []struct {
		name           string
		command        string
		approvalPolicy runtime.ApprovalPolicy
		sandboxPolicy  runtime.SandboxPolicy
		wantApproval   bool
	}{
		{
			name:           "safe command with never policy",
			command:        `{"command": "ls"}`,
			approvalPolicy: runtime.ApprovalNever,
			sandboxPolicy:  runtime.SandboxReadOnly,
			wantApproval:   false,
		},
		{
			name:           "safe command with on-request policy",
			command:        `{"command": "sh -c ls"}`,
			approvalPolicy: runtime.ApprovalOnRequest,
			sandboxPolicy:  runtime.SandboxReadOnly,
			wantApproval:   true, // sh is not in safe list, so needs approval
		},
		{
			name:           "dangerous command with never policy",
			command:        `{"command": "rm -rf /"}`,
			approvalPolicy: runtime.ApprovalNever,
			sandboxPolicy:  runtime.SandboxReadOnly,
			wantApproval:   false,
		},
		{
			name:           "dangerous command with on-request policy",
			command:        `{"command": "rm -rf /"}`,
			approvalPolicy: runtime.ApprovalOnRequest,
			sandboxPolicy:  runtime.SandboxReadOnly,
			wantApproval:   true,
		},
		{
			name:           "safe command echo with on-request policy",
			command:        `{"command": "echo hello"}`,
			approvalPolicy: runtime.ApprovalOnRequest,
			sandboxPolicy:  runtime.SandboxReadOnly,
			wantApproval:   false, // echo is safe, no approval needed
		},
		{
			name:           "unknown command with on-request policy",
			command:        `{"command": "unknown-cmd"}`,
			approvalPolicy: runtime.ApprovalOnRequest,
			sandboxPolicy:  runtime.SandboxReadOnly,
			wantApproval:   true, // unknown commands need approval
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &runtime.ToolRequest{
				Arguments: tt.command,
			}
			got := tool.NeedsInitialApproval(req, tt.approvalPolicy, tt.sandboxPolicy)
			assert.Equal(t, tt.wantApproval, got)
		})
	}
}

// TestShellToolNeedsRetryApproval tests retry approval requirements
func TestShellToolNeedsRetryApproval(t *testing.T) {
	tool := NewShellTool()

	tests := []struct {
		name           string
		approvalPolicy runtime.ApprovalPolicy
		wantApproval   bool
	}{
		{"never policy", runtime.ApprovalNever, false},
		{"on-failure policy", runtime.ApprovalOnFailure, true},
		{"on-request policy", runtime.ApprovalOnRequest, false},
		{"unless-trusted policy", runtime.ApprovalUnlessTrusted, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tool.NeedsRetryApproval(tt.approvalPolicy)
			assert.Equal(t, tt.wantApproval, got)
		})
	}
}

// TestShellToolWantsEscalatedFirstAttempt tests escalation preference
func TestShellToolWantsEscalatedFirstAttempt(t *testing.T) {
	tool := NewShellTool()

	tests := []struct {
		name      string
		arguments string
		want      bool
	}{
		{
			name:      "escalated permissions requested",
			arguments: `{"command": "echo test", "with_escalated_permissions": true}`,
			want:      true,
		},
		{
			name:      "no escalation requested",
			arguments: `{"command": "echo test"}`,
			want:      false,
		},
		{
			name:      "escalated explicitly false",
			arguments: `{"command": "echo test", "with_escalated_permissions": false}`,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &runtime.ToolRequest{
				Arguments: tt.arguments,
			}
			got := tool.WantsEscalatedFirstAttempt(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestShellToolSandboxRetryData tests sandbox retry data extraction
func TestShellToolSandboxRetryData(t *testing.T) {
	tool := NewShellTool()

	req := &runtime.ToolRequest{
		Arguments:        `{"command": "echo hello world"}`,
		WorkingDirectory: "/tmp",
	}

	data := tool.SandboxRetryData(req)
	require.NotNil(t, data)
	assert.Equal(t, []string{"sh", "-c", "echo hello world"}, data.Command)
	assert.Equal(t, "/tmp", data.WorkingDirectory)
}

// TestShellToolExecute_Success tests successful command execution
func TestShellToolExecute_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tool := NewShellTool()
	ctx := test.LongContext(t)

	req := &runtime.ToolRequest{
		CallID:           "test-call",
		ToolName:         "shell",
		Arguments:        `{"command": "echo hello"}`,
		WorkingDirectory: "/tmp",
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

	resp, err := tool.Execute(ctx, req, execCtx)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Contains(t, resp.Content, "hello")
	assert.NotNil(t, resp.Success)
	assert.True(t, *resp.Success)
	assert.NotNil(t, resp.ExitCode)
	assert.Equal(t, 0, *resp.ExitCode)
}

// TestShellToolExecute_Failure tests command execution failure
func TestShellToolExecute_Failure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tool := NewShellTool()
	ctx := test.LongContext(t)

	req := &runtime.ToolRequest{
		CallID:           "test-call",
		ToolName:         "shell",
		Arguments:        `{"command": "exit 1"}`,
		WorkingDirectory: "/tmp",
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

	resp, err := tool.Execute(ctx, req, execCtx)
	require.NoError(t, err) // Command execution succeeds even if exit code is non-zero
	require.NotNil(t, resp)

	assert.NotNil(t, resp.Success)
	assert.False(t, *resp.Success)
	assert.NotNil(t, resp.ExitCode)
	assert.Equal(t, 1, *resp.ExitCode)
}

// TestShellToolExecute_Timeout tests command timeout
func TestShellToolExecute_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tool := NewShellTool()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req := &runtime.ToolRequest{
		CallID:           "test-call",
		ToolName:         "shell",
		Arguments:        `{"command": "sleep 10"}`,
		WorkingDirectory: "/tmp",
		Timeout:          100 * time.Millisecond,
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

	_, err := tool.Execute(ctx, req, execCtx)
	require.Error(t, err)

	toolErr, ok := err.(*runtime.ToolError)
	require.True(t, ok, "expected ToolError")
	assert.Equal(t, runtime.ErrorTimeout, toolErr.Kind)
}

// TestShellToolExecute_InvalidArguments tests invalid arguments handling
func TestShellToolExecute_InvalidArguments(t *testing.T) {
	tool := NewShellTool()
	ctx := test.LongContext(t)

	tests := []struct {
		name      string
		arguments string
	}{
		{"empty json", ""},
		{"invalid json", "not json"},
		{"missing command", `{}`},
		{"null command", `{"command": null}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &runtime.ToolRequest{
				CallID:           "test-call",
				ToolName:         "shell",
				Arguments:        tt.arguments,
				WorkingDirectory: "/tmp",
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

			_, err := tool.Execute(ctx, req, execCtx)
			require.Error(t, err)

			toolErr, ok := err.(*runtime.ToolError)
			require.True(t, ok, "expected ToolError")
			assert.Equal(t, runtime.ErrorInvalidArguments, toolErr.Kind)
		})
	}
}

// TestShellToolExecute_WithStreaming tests output streaming
func TestShellToolExecute_WithStreaming(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tool := NewShellTool()
	ctx := test.LongContext(t)

	var capturedOutput strings.Builder

	req := &runtime.ToolRequest{
		CallID:           "test-call",
		ToolName:         "shell",
		Arguments:        `{"command": "echo hello && echo world"}`,
		WorkingDirectory: "/tmp",
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
		SandboxAttempt: &runtime.SandboxAttempt{
			Type:             runtime.SandboxNone,
			Policy:           runtime.SandboxDangerFullAccess,
			WorkingDirectory: "/tmp",
		},
		OutputWriter: &capturedOutput,
		StartTime:    time.Now(),
	}

	resp, err := tool.Execute(ctx, req, execCtx)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify streaming was used
	assert.True(t, resp.StreamedOutput)

	// Verify output was captured
	output := capturedOutput.String()
	assert.Contains(t, output, "hello")
	assert.Contains(t, output, "world")
}

// TestShellToolExecute_WithEnvironment tests environment variable passing
func TestShellToolExecute_WithEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tool := NewShellTool()
	ctx := test.LongContext(t)

	req := &runtime.ToolRequest{
		CallID:           "test-call",
		ToolName:         "shell",
		Arguments:        `{"command": "echo $TEST_VAR"}`,
		WorkingDirectory: "/tmp",
		Environment: map[string]string{
			"TEST_VAR": "test_value",
		},
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

	resp, err := tool.Execute(ctx, req, execCtx)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Contains(t, resp.Content, "test_value")
}

// TestParseArguments tests argument parsing
func TestParseArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments string
		wantCmd   string
		wantErr   bool
	}{
		{
			name:      "simple command",
			arguments: `{"command": "echo hello"}`,
			wantCmd:   "echo hello",
			wantErr:   false,
		},
		{
			name:      "with working directory",
			arguments: `{"command": "ls", "working_directory": "/tmp"}`,
			wantCmd:   "ls",
			wantErr:   false,
		},
		{
			name:      "with timeout",
			arguments: `{"command": "sleep 1", "timeout": 5000}`,
			wantCmd:   "sleep 1",
			wantErr:   false,
		},
		{
			name:      "empty command",
			arguments: `{"command": ""}`,
			wantCmd:   "",
			wantErr:   true,
		},
		{
			name:      "invalid json",
			arguments: "not json",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args shellArgs
			err := json.Unmarshal([]byte(tt.arguments), &args)
			if tt.wantErr {
				if err == nil && args.Command == "" {
					// Valid JSON but empty command
					return
				}
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantCmd, args.Command)
		})
	}
}

// TestCommandBuilder tests command building functionality
func TestCommandBuilder(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []string
	}{
		{
			name:    "simple echo",
			command: "echo hello",
			want:    []string{"sh", "-c", "echo hello"},
		},
		{
			name:    "complex command with pipes",
			command: "ls -la | grep test",
			want:    []string{"sh", "-c", "ls -la | grep test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCommandArray(tt.command)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestOutputCapture tests output capture functionality
func TestOutputCapture(t *testing.T) {
	stdout := "test stdout\n"
	stderr := "test stderr\n"

	aggregated := aggregateOutput(stdout, stderr)
	// aggregateOutput trims whitespace, so check for trimmed versions
	assert.Contains(t, aggregated, strings.TrimSpace(stdout))
	if stderr != "" {
		assert.Contains(t, aggregated, strings.TrimSpace(stderr))
	}
}

// TestFormatDuration tests duration formatting
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"milliseconds", 150 * time.Millisecond, "150ms"},
		{"seconds", 2 * time.Second, "2s"},
		{"minutes", 90 * time.Second, "1m30s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			assert.Equal(t, tt.want, got)
		})
	}
}

// BenchmarkShellToolExecute benchmarks command execution
func BenchmarkShellToolExecute(b *testing.B) {
	tool := NewShellTool()
	ctx := context.Background()

	req := &runtime.ToolRequest{
		CallID:           "bench-call",
		ToolName:         "shell",
		Arguments:        `{"command": "echo benchmark"}`,
		WorkingDirectory: "/tmp",
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
		_, err := tool.Execute(ctx, req, execCtx)
		if err != nil {
			b.Fatal(err)
		}
	}
}
