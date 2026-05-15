// Package shell provides the shell command execution tool runtime.
//
// The shell tool enables execution of shell commands with support for:
//   - Command approval workflows
//   - Sandbox isolation (bubblewrap/docker)
//   - Output streaming
//   - Escalated permissions for trusted operations
//   - Automatic retry without sandbox on permission denial
package shell

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// shellArgs represents the parsed arguments for the shell tool.
type shellArgs struct {
	// Command is the shell command to execute
	Command string `json:"command"`

	// WorkingDirectory overrides the default working directory (optional)
	WorkingDirectory string `json:"working_directory,omitempty"`

	// Timeout is the maximum execution time in milliseconds (optional)
	Timeout int `json:"timeout,omitempty"`

	// WithEscalatedPermissions requests execution without sandbox (optional)
	WithEscalatedPermissions bool `json:"with_escalated_permissions,omitempty"`

	// Justification explains why escalated permissions are needed (optional)
	Justification string `json:"justification,omitempty"`

	// Environment contains additional environment variables (optional)
	Environment map[string]string `json:"environment,omitempty"`
}

// ShellTool implements the ToolRuntime interface for shell command execution.
type ShellTool struct {
	executor *CommandExecutor
}

// NewShellTool creates a new shell tool instance.
func NewShellTool() *ShellTool {
	return &ShellTool{
		executor: NewCommandExecutor(),
	}
}

// Name returns the unique identifier for this tool.
func (s *ShellTool) Name() string {
	return "shell"
}

// Execute runs the shell command with the given request under the specified execution context.
func (s *ShellTool) Execute(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
	// Parse arguments
	args, err := s.parseArguments(req.Arguments)
	if err != nil {
		return nil, runtime.NewToolErrorWithCause(
			runtime.ErrorInvalidArguments,
			"failed to parse shell arguments",
			err,
		)
	}

	// Validate command
	if args.Command == "" {
		return nil, runtime.NewToolError(
			runtime.ErrorInvalidArguments,
			"command cannot be empty",
		)
	}

	// Sanitize command to prevent basic injection attacks
	sanitizedCommand := SanitizeCommand(args.Command)
	if sanitizedCommand == "" {
		return nil, runtime.NewToolError(
			runtime.ErrorInvalidArguments,
			"command is empty after sanitization",
		)
	}

	// Build command array
	cmdArray := buildCommandArray(sanitizedCommand)

	// Determine working directory
	workingDir := req.WorkingDirectory
	if args.WorkingDirectory != "" {
		workingDir = args.WorkingDirectory
	}
	if workingDir == "" {
		workingDir = "."
	}

	// Validate working directory
	if err := validateWorkingDirectory(workingDir, execCtx); err != nil {
		return nil, err
	}

	// Merge and validate environment variables
	env := make(map[string]string)
	for k, v := range req.Environment {
		env[k] = v
	}
	for k, v := range args.Environment {
		env[k] = v
	}

	// Validate environment variables for security
	if err := validateEnvironmentVariables(env); err != nil {
		return nil, err
	}

	// Create command spec
	spec := &CommandSpec{
		Command:          cmdArray,
		WorkingDirectory: workingDir,
		Environment:      env,
		CallID:           req.CallID,
	}

	// Apply timeout from arguments if specified
	if args.Timeout > 0 {
		timeout := time.Duration(args.Timeout) * time.Millisecond
		return s.executor.ExecuteWithTimeout(ctx, spec, execCtx, timeout)
	}

	// Execute the command with default timeout from request
	if req.Timeout > 0 {
		return s.executor.ExecuteWithTimeout(ctx, spec, execCtx, req.Timeout)
	}

	// Execute without timeout
	result, err := s.executor.Execute(ctx, spec, execCtx)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ApprovalKey generates a unique key for caching approval decisions.
func (s *ShellTool) ApprovalKey(req *runtime.ToolRequest) string {
	args, err := s.parseArguments(req.Arguments)
	if err != nil {
		return ""
	}

	// Create a key from tool name, command, and working directory
	key := fmt.Sprintf("shell:%s:%s", args.Command, req.WorkingDirectory)

	// Hash for consistent length
	hash := sha256.Sum256([]byte(key))
	return "shell:" + hex.EncodeToString(hash[:8])
}

// NeedsInitialApproval determines if approval is required before the first execution attempt.
func (s *ShellTool) NeedsInitialApproval(req *runtime.ToolRequest, approvalPolicy runtime.ApprovalPolicy, sandboxPolicy runtime.SandboxPolicy) bool {
	// Never policy means no approval needed
	if approvalPolicy == runtime.ApprovalNever {
		return false
	}

	// Danger full access mode doesn't need approval for on-request policy
	if approvalPolicy == runtime.ApprovalOnRequest && sandboxPolicy == runtime.SandboxDangerFullAccess {
		return false
	}

	// Parse command to check if it's safe
	args, err := s.parseArguments(req.Arguments)
	if err != nil {
		return true // Invalid arguments should require approval
	}

	cmdArray := buildCommandArray(args.Command)

	// Known safe commands don't need approval with on-request policy
	if approvalPolicy == runtime.ApprovalOnRequest && runtime.IsKnownSafeCommand(cmdArray) {
		return false
	}

	// Unless-trusted policy requires approval for everything except known safe commands
	if approvalPolicy == runtime.ApprovalUnlessTrusted && !runtime.IsKnownSafeCommand(cmdArray) {
		return true
	}

	// On-request policy needs approval for non-safe commands
	if approvalPolicy == runtime.ApprovalOnRequest {
		return true
	}

	return false
}

// NeedsRetryApproval determines if approval is required before retrying without sandbox.
func (s *ShellTool) NeedsRetryApproval(approvalPolicy runtime.ApprovalPolicy) bool {
	// On-failure policy specifically requires approval for retry
	// Unless-trusted also requires approval for escalation
	return approvalPolicy == runtime.ApprovalOnFailure || approvalPolicy == runtime.ApprovalUnlessTrusted
}

// SandboxPreference indicates how this tool wants to interact with sandboxing.
func (s *ShellTool) SandboxPreference() runtime.SandboxPreference {
	return runtime.SandboxAuto
}

// EscalateOnFailure returns true if the tool should retry without sandbox
// when the initial sandboxed attempt fails with a permission denial.
func (s *ShellTool) EscalateOnFailure() bool {
	return true
}

// WantsEscalatedFirstAttempt returns true if the request explicitly asks
// for escalated permissions (bypass sandbox on first attempt).
func (s *ShellTool) WantsEscalatedFirstAttempt(req *runtime.ToolRequest) bool {
	args, err := s.parseArguments(req.Arguments)
	if err != nil {
		return false
	}
	return args.WithEscalatedPermissions
}

// SupportsParallel returns true if multiple invocations of this tool
// can safely execute concurrently.
func (s *ShellTool) SupportsParallel() bool {
	return true
}

// SandboxRetryData extracts command metadata needed for re-running without sandbox.
func (s *ShellTool) SandboxRetryData(req *runtime.ToolRequest) *runtime.SandboxRetryData {
	args, err := s.parseArguments(req.Arguments)
	if err != nil {
		return nil
	}

	cmdArray := buildCommandArray(args.Command)
	workingDir := req.WorkingDirectory
	if args.WorkingDirectory != "" {
		workingDir = args.WorkingDirectory
	}

	return &runtime.SandboxRetryData{
		Command:          cmdArray,
		WorkingDirectory: workingDir,
	}
}

// parseArguments parses the JSON arguments for the shell tool.
func (s *ShellTool) parseArguments(arguments string) (*shellArgs, error) {
	if arguments == "" {
		return nil, fmt.Errorf("arguments cannot be empty")
	}

	var args shellArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return nil, fmt.Errorf("failed to parse JSON arguments: %w", err)
	}

	return &args, nil
}

// buildCommandArray builds a command array suitable for os/exec.
// Shell commands are executed via "sh -c" to support pipes, redirects, etc.
func buildCommandArray(command string) []string {
	// Always use sh -c for shell command execution
	// This allows pipes, redirects, and other shell features
	return []string{"sh", "-c", command}
}

// aggregateOutput combines stdout and stderr into a single output string.
func aggregateOutput(stdout, stderr string) string {
	var parts []string

	if stdout != "" {
		parts = append(parts, strings.TrimSpace(stdout))
	}

	if stderr != "" {
		parts = append(parts, strings.TrimSpace(stderr))
	}

	return strings.Join(parts, "\n")
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		seconds := d.Seconds()
		if seconds == float64(int(seconds)) {
			return fmt.Sprintf("%ds", int(seconds))
		}
		return d.Round(100 * time.Millisecond).String()
	}
	return d.Round(time.Second).String()
}

// validateWorkingDirectory validates the working directory for security issues.
// It checks for:
//   - Directory existence
//   - Path traversal attempts
//   - Workspace boundary violations (when sandboxed)
func validateWorkingDirectory(workingDir string, execCtx *runtime.ExecutionContext) error {
	// Resolve to absolute path
	absPath, err := filepath.Abs(workingDir)
	if err != nil {
		return runtime.NewToolErrorWithCause(
			runtime.ErrorInvalidArguments,
			fmt.Sprintf("invalid working directory path: %s", workingDir),
			err,
		)
	}

	// Check if directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return runtime.NewToolError(
				runtime.ErrorInvalidArguments,
				fmt.Sprintf("working directory does not exist: %s", absPath),
			)
		}
		return runtime.NewToolErrorWithCause(
			runtime.ErrorInvalidArguments,
			fmt.Sprintf("cannot access working directory: %s", absPath),
			err,
		)
	}

	// Verify it's actually a directory
	if !info.IsDir() {
		return runtime.NewToolError(
			runtime.ErrorInvalidArguments,
			fmt.Sprintf("working directory path is not a directory: %s", absPath),
		)
	}

	// Check for path traversal attempts by detecting ".." in the path
	cleanPath := filepath.Clean(absPath)
	if strings.Contains(cleanPath, "..") {
		return runtime.NewToolError(
			runtime.ErrorInvalidArguments,
			fmt.Sprintf("path traversal detected in working directory: %s", workingDir),
		)
	}

	// When sandboxed with workspace restrictions, validate we're within workspace bounds
	if execCtx.SandboxAttempt != nil {
		policy := execCtx.SandboxAttempt.Policy
		if policy == runtime.SandboxReadOnly || policy == runtime.SandboxWorkspaceWrite {
			workspaceRoot := execCtx.SandboxAttempt.WorkingDirectory
			if workspaceRoot != "" {
				absWorkspace, err := filepath.Abs(workspaceRoot)
				if err == nil {
					// Check if the working directory is within workspace bounds
					relPath, err := filepath.Rel(absWorkspace, absPath)
					if err != nil || strings.HasPrefix(relPath, "..") {
						return runtime.NewToolError(
							runtime.ErrorInvalidArguments,
							fmt.Sprintf("working directory %s is outside workspace bounds %s", absPath, absWorkspace),
						)
					}
				}
			}
		}
	}

	return nil
}

// validateEnvironmentVariables validates environment variables for security issues.
// It checks for dangerous variables that could compromise sandbox security.
func validateEnvironmentVariables(env map[string]string) error {
	// Dangerous environment variables that should never be overridden in sandboxed contexts
	dangerousVars := []string{
		"LD_PRELOAD",      // Can inject malicious libraries
		"LD_LIBRARY_PATH", // Can redirect library loading
		"DYLD_INSERT_LIBRARIES", // macOS equivalent of LD_PRELOAD
		"DYLD_LIBRARY_PATH",     // macOS equivalent of LD_LIBRARY_PATH
		"DYLD_FRAMEWORK_PATH",   // Can redirect framework loading on macOS
		"PYTHONPATH",      // Can inject malicious Python modules
		"PERLLIB",         // Can inject malicious Perl modules
		"PERL5LIB",        // Can inject malicious Perl modules
		"RUBYLIB",         // Can inject malicious Ruby modules
		"NODE_PATH",       // Can inject malicious Node.js modules
	}

	for _, dangerous := range dangerousVars {
		if _, exists := env[dangerous]; exists {
			return runtime.NewToolError(
				runtime.ErrorInvalidArguments,
				fmt.Sprintf("dangerous environment variable not allowed: %s", dangerous),
			)
		}
	}

	// Validate PATH doesn't contain suspicious entries
	if pathValue, exists := env["PATH"]; exists {
		// Check for path entries that could be dangerous
		pathParts := filepath.SplitList(pathValue)
		for _, part := range pathParts {
			// Detect relative paths in PATH (security risk)
			if !filepath.IsAbs(part) && part != "" {
				return runtime.NewToolError(
					runtime.ErrorInvalidArguments,
					fmt.Sprintf("relative path in PATH environment variable not allowed: %s", part),
				)
			}
			// Detect world-writable directories in PATH
			if info, err := os.Stat(part); err == nil {
				mode := info.Mode()
				if mode.Perm()&0002 != 0 { // World-writable
					return runtime.NewToolError(
						runtime.ErrorInvalidArguments,
						fmt.Sprintf("world-writable directory in PATH not allowed: %s", part),
					)
				}
			}
		}
	}

	return nil
}

// SanitizeCommand performs sanitization on command strings to prevent injection.
func SanitizeCommand(command string) string {
	// Remove null bytes
	command = strings.ReplaceAll(command, "\x00", "")

	// Remove other control characters except newlines and tabs
	var sanitized strings.Builder
	for _, r := range command {
		// Allow printable characters, newlines, and tabs
		if r >= 32 || r == '\n' || r == '\t' {
			sanitized.WriteRune(r)
		}
	}

	return strings.TrimSpace(sanitized.String())
}
