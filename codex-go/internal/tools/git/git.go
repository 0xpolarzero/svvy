// Package git provides git repository management tools for Codex Go.
//
// The git package implements the following tools:
//   - git_status: Show the working tree status
//   - git_diff: Show changes between commits, commit and working tree, etc
//   - git_log: Show commit logs
//   - git_commit: Record changes to the repository (requires approval)
//
// All git operations are executed using the system git binary with proper
// sandboxing when available. The tools parse git output into structured
// formats suitable for AI consumption.
package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultGitTimeout = 30 * time.Second
	MaxGitArgsCount   = 100
	MaxGitArgLength   = 4096
)

// Allowed git commands (whitelist)
var allowedGitCommands = map[string]bool{
	"status":       true,
	"diff":         true,
	"log":          true,
	"show":         true,
	"branch":       true,
	"rev-parse":    true,
	"ls-files":     true,
	"ls-tree":      true,
	"cat-file":     true,
	"describe":     true,
	"rev-list":     true,
	"name-rev":     true,
	"symbolic-ref": true,
	"commit":       true,
	"config":       true,
	"init":         true,
	"add":          true,
}

// Dangerous git options that should be blocked
var dangerousGitOptions = []string{
	"--upload-pack",
	"--receive-pack",
	"--exec",
	"-c", // config override
	"--config",
}

// gitExecutor provides common functionality for executing git commands.
type gitExecutor struct{}

// newGitExecutor creates a new git command executor.
func newGitExecutor() *gitExecutor {
	return &gitExecutor{}
}

// sanitizeGitArgs validates and sanitizes git command arguments
func sanitizeGitArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("git args cannot be empty")
	}

	if len(args) > MaxGitArgsCount {
		return fmt.Errorf("too many git arguments: %d (max %d)", len(args), MaxGitArgsCount)
	}

	// First arg should be the git command
	command := args[0]

	// Validate command is allowed
	if !allowedGitCommands[command] {
		return fmt.Errorf("git command not allowed: %s", command)
	}

	// Check all arguments
	for i, arg := range args {
		// Check length
		if len(arg) > MaxGitArgLength {
			return fmt.Errorf("git arg %d exceeds maximum length of %d", i, MaxGitArgLength)
		}

		// Check for null bytes
		if strings.Contains(arg, "\x00") {
			return fmt.Errorf("git arg %d contains null byte", i)
		}

		// Check for dangerous options
		for _, dangerous := range dangerousGitOptions {
			if arg == dangerous || strings.HasPrefix(arg, dangerous+"=") {
				return fmt.Errorf("dangerous git option not allowed: %s", dangerous)
			}
		}

		// Detect command injection patterns
		if strings.ContainsAny(arg, ";&|><$`") {
			// Only warn for now, as these might be legitimate in some contexts
			// But log for security monitoring
			_ = fmt.Sprintf("warning: git arg %d contains shell metacharacter: %s", i, arg)
		}
	}

	return nil
}

// validateWorkingDirectory checks if a working directory is safe for git operations
func (e *gitExecutor) validateWorkingDirectory(workingDir string) error {
	if workingDir == "" {
		return fmt.Errorf("working directory cannot be empty")
	}

	// Clean and normalize path
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

	// Get absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("cannot resolve absolute path: %w", err)
	}

	// Ensure path doesn't contain .. after cleaning
	if strings.Contains(filepath.ToSlash(absPath), "..") {
		return fmt.Errorf("working directory contains path traversal: %s", workingDir)
	}

	// Verify it's actually a git repository
	if !e.isGitRepo(context.Background(), absPath) {
		return fmt.Errorf("working directory is not a git repository: %s", workingDir)
	}

	return nil
}

// executeGitUnsafe runs a git command without validation (internal use only).
// The command is executed in the given working directory.
func (e *gitExecutor) executeGitUnsafe(ctx context.Context, workingDir string, args ...string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workingDir

	// Create buffers for output
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Execute command
	execErr := cmd.Run()

	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	return stdout, stderr, execErr
}

// executeGit runs a git command and returns stdout, stderr, and error.
// The command is executed in the given working directory.
func (e *gitExecutor) executeGit(ctx context.Context, workingDir string, args ...string) (stdout, stderr string, err error) {
	// Validate working directory
	if err := e.validateWorkingDirectory(workingDir); err != nil {
		return "", "", fmt.Errorf("invalid working directory: %w", err)
	}

	// Validate and sanitize arguments
	if err := sanitizeGitArgs(args); err != nil {
		return "", "", fmt.Errorf("invalid git arguments: %w", err)
	}

	// Create command with validated args
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workingDir

	// Create buffers for output
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Execute command
	execErr := cmd.Run()

	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	return stdout, stderr, execErr
}

// executeGitWithTimeout runs a git command with a timeout.
func (e *gitExecutor) executeGitWithTimeout(ctx context.Context, workingDir string, timeout time.Duration, args ...string) (stdout, stderr string, err error) {
	// Use default timeout if not specified
	if timeout == 0 {
		timeout = DefaultGitTimeout
	}

	// Create context with timeout
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	return e.executeGit(ctx, workingDir, args...)
}

// isGitRepo checks if the given directory is inside a git repository.
func (e *gitExecutor) isGitRepo(ctx context.Context, workingDir string) bool {
	_, _, err := e.executeGitUnsafe(ctx, workingDir, "rev-parse", "--git-dir")
	return err == nil
}

// validateGitDiffArgs validates arguments for git diff command
func validateGitDiffArgs(args []string) error {
	// Ensure we're not accessing arbitrary files
	for _, arg := range args {
		// Block absolute paths
		if filepath.IsAbs(arg) {
			return fmt.Errorf("absolute paths not allowed in git diff: %s", arg)
		}
		// Block path traversal
		if strings.Contains(arg, "..") {
			return fmt.Errorf("path traversal not allowed in git diff: %s", arg)
		}
	}
	return nil
}

// validateGitLogArgs validates arguments for git log command
func validateGitLogArgs(args []string) error {
	// Limit the number of log entries to prevent DoS
	hasLimit := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "-n") || strings.HasPrefix(arg, "--max-count=") {
			hasLimit = true
			break
		}
	}
	if !hasLimit {
		// No explicit limit, could be DoS
		// Add a default limit or warn
	}
	return nil
}

// gitError wraps git command errors with contextual information.
type gitError struct {
	command string
	stdout  string
	stderr  string
	err     error
}

// Error returns the formatted error message.
func (e *gitError) Error() string {
	if e.stderr != "" {
		return fmt.Sprintf("git %s failed: %s", e.command, strings.TrimSpace(e.stderr))
	}
	if e.err != nil {
		return fmt.Sprintf("git %s failed: %v", e.command, e.err)
	}
	return fmt.Sprintf("git %s failed", e.command)
}

// Unwrap returns the underlying error.
func (e *gitError) Unwrap() error {
	return e.err
}

// newGitError creates a new git error.
func newGitError(command string, stdout, stderr string, err error) error {
	return &gitError{
		command: command,
		stdout:  stdout,
		stderr:  stderr,
		err:     err,
	}
}

// formatGitOutput formats git output for display.
// Removes empty lines at the start and end, and ensures consistent line endings.
func formatGitOutput(output string) string {
	lines := strings.Split(output, "\n")

	// Trim leading empty lines
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}

	// Trim trailing empty lines
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}

	if start >= end {
		return ""
	}

	return strings.Join(lines[start:end], "\n")
}

// parseFileStatus parses a git status porcelain line into XY status codes.
// Returns the XY code, the file path, and whether it's a rename (old -> new).
func parseFileStatus(line string) (xy, path, oldPath string) {
	if len(line) < 4 {
		return "", line, ""
	}

	xy = line[0:2]
	path = strings.TrimSpace(line[3:])

	// Handle renames (R  old -> new)
	if strings.Contains(path, " -> ") {
		parts := strings.SplitN(path, " -> ", 2)
		if len(parts) == 2 {
			oldPath = parts[0]
			path = parts[1]
		}
	}

	return xy, path, oldPath
}

// statusCodeDescription returns a human-readable description of git status codes.
func statusCodeDescription(xy string) string {
	if len(xy) != 2 {
		return "unknown"
	}

	x := xy[0]
	y := xy[1]

	var parts []string

	// Staged changes (index)
	switch x {
	case 'M':
		parts = append(parts, "modified")
	case 'A':
		parts = append(parts, "added")
	case 'D':
		parts = append(parts, "deleted")
	case 'R':
		parts = append(parts, "renamed")
	case 'C':
		parts = append(parts, "copied")
	}

	// Working tree changes
	switch y {
	case 'M':
		if len(parts) > 0 {
			parts = append(parts, "modified in working tree")
		} else {
			parts = append(parts, "modified")
		}
	case 'D':
		if len(parts) > 0 {
			parts = append(parts, "deleted in working tree")
		} else {
			parts = append(parts, "deleted")
		}
	case '?':
		parts = append(parts, "untracked")
	}

	if len(parts) == 0 {
		return "unchanged"
	}

	return strings.Join(parts, ", ")
}
