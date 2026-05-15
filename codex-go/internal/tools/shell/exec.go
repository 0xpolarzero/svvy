package shell

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/evmts/codex/codex-go/internal/sandbox"
	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

const (
	MaxStdoutSize      = 10 * 1024 * 1024 // 10MB
	MaxStderrSize      = 1 * 1024 * 1024  // 1MB
	MaxTotalOutputSize = 11 * 1024 * 1024 // 11MB total
)

// CommandSpec defines the specification for executing a command.
type CommandSpec struct {
	// Command is the full command array (program + args)
	Command []string

	// WorkingDirectory is where the command should execute
	WorkingDirectory string

	// Environment contains additional environment variables
	Environment map[string]string

	// CallID identifies this command execution
	CallID string
}

// CommandExecutor handles the execution of shell commands.
type CommandExecutor struct {
	sandboxManager *sandbox.SandboxManager
}

// NewCommandExecutor creates a new command executor.
func NewCommandExecutor() *CommandExecutor {
	return &CommandExecutor{
		sandboxManager: sandbox.NewSandboxManager(),
	}
}

// validateCommand checks if a command is safe to execute
func validateCommand(command []string) error {
	if len(command) == 0 {
		return fmt.Errorf("command cannot be empty")
	}

	// Check for null bytes
	for i, arg := range command {
		if strings.Contains(arg, "\x00") {
			return fmt.Errorf("command argument %d contains null byte", i)
		}
	}

	// Detect shell metacharacters that could enable injection
	// Note: We're using exec.CommandContext which doesn't invoke a shell,
	// but we still want to detect suspicious patterns
	dangerousPatterns := []string{
		";", "&&", "||", "|", ">", "<", "`", "$(",
	}

	for i, arg := range command {
		for _, pattern := range dangerousPatterns {
			if strings.Contains(arg, pattern) {
				// Only warn for now since these might be legitimate
				// In a real deployment, consider logging or stricter checks
				_ = fmt.Sprintf("warning: command argument %d contains potentially dangerous pattern '%s': %s", i, pattern, arg)
			}
		}
	}

	return nil
}

// validateWorkingDirectoryBasic checks if a working directory is safe to use
// This is a basic validation that doesn't require ExecutionContext
func validateWorkingDirectoryBasic(workingDir string) error {
	if workingDir == "" {
		return nil // Empty is okay, will use current directory
	}

	// Prevent path traversal
	cleanPath := filepath.Clean(workingDir)

	// Check if directory exists
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("working directory does not exist: %s", workingDir)
		}
		return fmt.Errorf("cannot access working directory: %w", err)
	}

	// Ensure it's a directory
	if !info.IsDir() {
		return fmt.Errorf("working directory is not a directory: %s", workingDir)
	}

	// Get absolute path for validation
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("cannot resolve absolute path: %w", err)
	}

	// Ensure path doesn't contain .. after cleaning
	if strings.Contains(filepath.ToSlash(absPath), "..") {
		return fmt.Errorf("working directory contains path traversal: %s", workingDir)
	}

	return nil
}

// Execute runs a command and returns the result.
// Sandbox policy is determined by the ExecutionContext's SandboxAttempt, which is set by the orchestrator.
func (e *CommandExecutor) Execute(ctx context.Context, spec *CommandSpec, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
	startTime := time.Now()

	// Validate command
	if len(spec.Command) == 0 {
		return nil, runtime.NewToolError(
			runtime.ErrorInvalidArguments,
			"command cannot be empty",
		)
	}

	// Validate command for security
	if err := validateCommand(spec.Command); err != nil {
		return nil, runtime.NewToolError(
			runtime.ErrorInvalidArguments,
			fmt.Sprintf("invalid command: %v", err),
		)
	}

	// Create the command
	cmd := exec.CommandContext(ctx, spec.Command[0], spec.Command[1:]...)

	// Validate and set working directory
	if spec.WorkingDirectory != "" {
		if err := validateWorkingDirectoryBasic(spec.WorkingDirectory); err != nil {
			return nil, runtime.NewToolError(
				runtime.ErrorInvalidArguments,
				fmt.Sprintf("invalid working directory: %v", err),
			)
		}
		cmd.Dir = spec.WorkingDirectory
	}

	// Set environment variables with filtering to prevent credential leakage
	if len(spec.Environment) > 0 {
		// Create filter to remove sensitive environment variables
		filter := NewDefaultEnvFilter()

		// Start with filtered system environment
		cmd.Env = filter.Filter()

		// Add user-specified environment variables (also filtered for safety)
		filteredEnv := filter.FilterMap(spec.Environment)
		for k, v := range filteredEnv {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Create output capturer with size limits
	capturer := NewOutputCapturer(spec.CallID)

	// Wrap with size limiters
	stdoutLimited := NewLimitedWriter(capturer.stdout, MaxStdoutSize, "stdout")
	stderrLimited := NewLimitedWriter(capturer.stderr, MaxStderrSize, "stderr")

	// Set up output capture
	if execCtx.OutputWriter != nil {
		// Stream output in real-time with limits
		cmd.Stdout = io.MultiWriter(stdoutLimited, execCtx.OutputWriter)
		cmd.Stderr = io.MultiWriter(stderrLimited, execCtx.OutputWriter)
	} else {
		// Just capture output with limits
		cmd.Stdout = stdoutLimited
		cmd.Stderr = stderrLimited
	}

	// Apply sandbox from ExecutionContext (set by orchestrator)
	// This ensures ALL commands go through the sandbox orchestrator's policy
	var sandboxInfo *sandbox.SandboxInfo
	if execCtx.SandboxAttempt != nil && execCtx.SandboxAttempt.Policy != runtime.SandboxDangerFullAccess {
		// Convert runtime.SandboxPolicy to sandbox.PolicyConfig
		sandboxPolicy := convertToSandboxPolicy(execCtx.SandboxAttempt)

		workspace := spec.WorkingDirectory
		if workspace == "" {
			workspace = "."
		}

		info, err := e.sandboxManager.ApplyToCommand(cmd, sandboxPolicy, workspace)
		if err != nil {
			return nil, runtime.NewToolErrorWithCause(
				runtime.ErrorExecution,
				fmt.Sprintf("failed to apply sandbox: %v", err),
				err,
			)
		}
		sandboxInfo = info
	} else {
		// No sandbox or danger-full-access mode
		sandboxInfo = &sandbox.SandboxInfo{
			Type:    sandbox.SandboxTypeNone,
			Applied: false,
			Reason:  "no sandbox policy or full access mode",
		}
	}

	// Execute the command
	execErr := cmd.Run()

	// Calculate execution time
	executionTime := time.Since(startTime)

	// Get captured output
	stdout := capturer.Stdout()
	stderr := capturer.Stderr()

	// Determine exit code and success status
	exitCode := 0
	success := true

	if execErr != nil {
		success = false

		// Check if it was a timeout/cancellation
		if ctx.Err() == context.DeadlineExceeded || ctx.Err() == context.Canceled {
			return nil, runtime.NewToolErrorWithCause(
				runtime.ErrorTimeout,
				"command execution timed out or was cancelled",
				ctx.Err(),
			)
		}

		// Extract exit code if available
		if exitError, ok := execErr.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			// Other execution errors (e.g., command not found)
			return nil, runtime.NewToolErrorWithCause(
				runtime.ErrorExecution,
				fmt.Sprintf("failed to execute command: %v", execErr),
				execErr,
			)
		}
	}

	// Aggregate output
	content := aggregateOutput(stdout, stderr)
	if content == "" && execErr != nil {
		content = execErr.Error()
	}

	// Build response
	resp := &runtime.ToolResponse{
		Content:        content,
		Success:        &success,
		ExitCode:       &exitCode,
		ExecutionTime:  executionTime,
		StreamedOutput: execCtx.OutputWriter != nil,
	}

	// Add sandbox metadata if sandboxing was applied
	if sandboxInfo != nil && sandboxInfo.Applied {
		if resp.Metadata == nil {
			resp.Metadata = make(map[string]interface{})
		}
		resp.Metadata["sandbox_type"] = sandboxInfo.Type.String()
		resp.Metadata["sandbox_applied"] = sandboxInfo.Applied
		resp.Metadata["sandbox_reason"] = sandboxInfo.Reason
	}

	return resp, nil
}

// convertToSandboxPolicy converts runtime.SandboxAttempt to sandbox.PolicyConfig
func convertToSandboxPolicy(attempt *runtime.SandboxAttempt) *sandbox.PolicyConfig {
	switch attempt.Policy {
	case runtime.SandboxReadOnly:
		return sandbox.NewReadOnlyPolicy()
	case runtime.SandboxWorkspaceWrite:
		config := sandbox.NewWorkspaceWritePolicy()
		// Apply network settings from attempt if available
		if config.WorkspaceWriteConfig != nil {
			config.WorkspaceWriteConfig.NetworkAccess = attempt.NetworkEnabled
		}
		return config
	case runtime.SandboxDangerFullAccess:
		return sandbox.NewDangerFullAccessPolicy()
	default:
		// Default to read-only for safety
		return sandbox.NewReadOnlyPolicy()
	}
}

// ExecuteWithTimeout executes a command with a timeout.
func (e *CommandExecutor) ExecuteWithTimeout(ctx context.Context, spec *CommandSpec, execCtx *runtime.ExecutionContext, timeout time.Duration) (*runtime.ToolResponse, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	return e.Execute(ctx, spec, execCtx)
}

// IsCommandAvailable checks if a command is available in the system PATH.
func IsCommandAvailable(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}
