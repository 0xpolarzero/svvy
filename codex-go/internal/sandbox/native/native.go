// Package native provides a native (non-isolated) sandbox implementation.
//
// # WARNING: NO SECURITY ISOLATION
//
// The native sandbox executes commands directly on the host system without
// ANY isolation or security restrictions. It provides:
//   - NO filesystem isolation - full read/write access to all files
//   - NO network isolation - full network access regardless of settings
//   - NO resource limits - can consume unlimited CPU/memory
//   - NO protection against malicious code, fork bombs, or system damage
//
// This sandbox is ONLY suitable for executing TRUSTED code in controlled
// environments. For untrusted code, use Docker or Kubernetes sandboxes.
//
// # Security Warnings
//
// The following Command fields are SILENTLY IGNORED:
//   - ReadOnlyPaths: No filesystem restrictions are enforced
//   - ReadWritePaths: No filesystem restrictions are enforced
//   - NetworkEnabled: Network access cannot be controlled
//
// # Command Injection Risks
//
// When executing shell commands (sh, bash, etc.), ensure user input is properly
// escaped or use separate Program + Args instead of shell -c commands.
//
// Unsafe example (DO NOT DO THIS):
//
//	cmd := &sandbox.Command{
//	    Program: "sh",
//	    Args:    []string{"-c", "echo " + userInput}, // DANGEROUS - injection risk
//	}
//
// Safe example:
//
//	cmd := &sandbox.Command{
//	    Program: "echo",
//	    Args:    []string{userInput}, // Safe - no shell interpretation
//	}
//
// # Environment Variable Leakage
//
// If Command.Environment is empty, ALL parent environment variables are inherited,
// including any secrets (API keys, tokens). If Command.Environment is set, it ADDS
// to (not replaces) the parent environment. There is no way to run with a clean
// environment in the native sandbox.
package native

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/evmts/codex/codex-go/internal/sandbox"
)

// ExitCodeSystemError is used when a command fails to start or context is cancelled.
// This distinguishes system-level errors from command failures (non-zero exit codes).
const ExitCodeSystemError = -1

// Default configuration values.
const (
	// DefaultMaxOutputSize is the default limit for stdout/stderr (10MB).
	DefaultMaxOutputSize = 10 * 1024 * 1024

	// DefaultWarnOnIgnoredSecurity controls whether warnings are logged by default.
	DefaultWarnOnIgnoredSecurity = true
)

// Options configures the native sandbox behavior.
type Options struct {
	// MaxOutputSize limits the combined stdout/stderr output to prevent OOM.
	// If output exceeds this size, it will be truncated.
	// Set to 0 for unlimited output (not recommended for untrusted commands).
	// Default: 10MB (DefaultMaxOutputSize)
	MaxOutputSize int64

	// WarnOnIgnoredSecurity enables logging when security-critical fields are ignored.
	// When true, warnings are logged if ReadOnlyPaths, ReadWritePaths, or
	// NetworkEnabled fields are set but cannot be enforced.
	// Default: true (DefaultWarnOnIgnoredSecurity)
	WarnOnIgnoredSecurity bool
}

// NativeSandbox implements the Sandbox interface using os/exec directly.
//
// # WARNING: NO ISOLATION
//
// Commands run with full system access. See package documentation for security implications.
//
// # Thread Safety
//
// NativeSandbox is safe for concurrent use. Each Execute call creates
// an independent os/exec.Cmd with no shared state.
type NativeSandbox struct {
	opts *Options
}

// New creates a new native sandbox instance with default options.
//
// Default configuration:
//   - MaxOutputSize: 10MB (prevents OOM from large output)
//   - WarnOnIgnoredSecurity: true (logs warnings for ignored security fields)
//
// Use NewWithOptions to customize these settings.
func New() *NativeSandbox {
	return NewWithOptions(nil)
}

// NewWithOptions creates a new native sandbox with custom options.
// If opts is nil, default options are used.
func NewWithOptions(opts *Options) *NativeSandbox {
	if opts == nil {
		opts = &Options{
			MaxOutputSize:         DefaultMaxOutputSize,
			WarnOnIgnoredSecurity: DefaultWarnOnIgnoredSecurity,
		}
	}

	// Apply defaults for zero values
	if opts.MaxOutputSize == 0 {
		opts.MaxOutputSize = DefaultMaxOutputSize
	}

	return &NativeSandbox{opts: opts}
}

// Type returns the identifier for this sandbox type.
func (n *NativeSandbox) Type() string {
	return "native"
}

// IsAvailable checks if the native sandbox is available.
// It's always available since it uses standard os/exec.
func (n *NativeSandbox) IsAvailable() bool {
	return true
}

// Cleanup performs any necessary cleanup after command execution.
// Native sandbox has no resources to clean up, so this is a no-op.
func (n *NativeSandbox) Cleanup(ctx context.Context) error {
	return nil
}

// Execute runs a command directly using os/exec.
//
// # Input Validation
//
// Returns an error if:
//   - ctx is nil
//   - cmd is nil
//   - cmd.Program is empty
//
// # Output Handling
//
// Stdout and stderr are captured up to MaxOutputSize bytes (default 10MB).
// Output exceeding this limit will be truncated to prevent OOM.
//
// # Error Handling
//
// Return values:
//   - result: Always returned (even on error), contains stdout/stderr/exitCode
//   - error: Only set for system-level failures, NOT for non-zero exit codes
//
// Error categories:
//   - Non-zero exit code: result.ExitCode != 0, error == nil
//   - Timeout/cancellation: result.ExitCode == -1, error != nil
//   - Command not found: result.ExitCode == -1, error != nil
//   - Invalid input: result == nil, error != nil
//
// # Security Field Warnings
//
// If WarnOnIgnoredSecurity is enabled (default), warnings are logged when:
//   - ReadOnlyPaths is set (filesystem restrictions not enforced)
//   - ReadWritePaths is set (filesystem restrictions not enforced)
//   - NetworkEnabled is false (network access cannot be disabled)
func (n *NativeSandbox) Execute(ctx context.Context, cmd *sandbox.Command) (*sandbox.Result, error) {
	// Validate inputs
	if ctx == nil {
		return nil, errors.New("native sandbox: context cannot be nil")
	}
	if cmd == nil {
		return nil, errors.New("native sandbox: command cannot be nil")
	}
	if cmd.Program == "" {
		return nil, errors.New("native sandbox: command program cannot be empty")
	}

	// Warn about ignored security fields
	if n.opts.WarnOnIgnoredSecurity {
		n.warnIgnoredSecurityFields(cmd)
	}

	startTime := time.Now()

	// Apply timeout if specified
	if cmd.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cmd.Timeout)
		defer cancel()
	}

	// Create the command with context for cancellation support
	execCmd := exec.CommandContext(ctx, cmd.Program, cmd.Args...)

	// Set working directory if specified
	if cmd.WorkingDirectory != "" {
		execCmd.Dir = cmd.WorkingDirectory
	}

	// Set environment variables if specified
	if len(cmd.Environment) > 0 {
		// Start with the parent process environment
		execCmd.Env = execCmd.Environ()

		// Add custom environment variables
		for key, value := range cmd.Environment {
			execCmd.Env = append(execCmd.Env, key+"="+value)
		}
	}

	// Setup stdin
	if cmd.Stdin != "" {
		execCmd.Stdin = strings.NewReader(cmd.Stdin)
	}

	// Capture stdout and stderr with size limits to prevent OOM
	var stdout, stderr limitedBuffer
	stdout.maxSize = n.opts.MaxOutputSize
	stderr.maxSize = n.opts.MaxOutputSize
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	// Execute the command
	err := execCmd.Run()

	executionTime := time.Since(startTime)

	// Build result
	result := &sandbox.Result{
		Stdout:        stdout.String(),
		Stderr:        stderr.String(),
		ExitCode:      0,
		ExecutionTime: executionTime,
		Error:         nil,
	}

	// Handle execution error
	if err != nil {
		// Check for context-related errors (timeout, cancellation) first
		if ctx.Err() != nil {
			result.ExitCode = ExitCodeSystemError
			result.Error = ctx.Err()
			return result, ctx.Err()
		}

		if exitErr, ok := err.(*exec.ExitError); ok {
			// Command ran but returned non-zero exit code
			result.ExitCode = exitErr.ExitCode()
			result.Error = nil // Non-zero exit is not an error in our model

			// Check for sandbox violations
			detector := sandbox.NewViolationDetector(n.Type())
			if violation := detector.DetectViolation(result); violation != nil {
				result.Violation = violation
			}

			return result, nil
		}

		// Command failed to start or other system error
		result.ExitCode = ExitCodeSystemError
		result.Error = err
		return result, err
	}

	return result, nil
}

// warnIgnoredSecurityFields logs warnings when security-critical Command fields
// are set but cannot be enforced by the native sandbox.
func (n *NativeSandbox) warnIgnoredSecurityFields(cmd *sandbox.Command) {
	if len(cmd.ReadOnlyPaths) > 0 {
		log.Printf("WARNING [native sandbox]: ReadOnlyPaths field is ignored - native sandbox provides NO filesystem isolation. Command has full read/write access to: %v. Use Docker or Kubernetes sandboxes for filesystem restrictions.", cmd.ReadOnlyPaths)
	}
	if len(cmd.ReadWritePaths) > 0 {
		log.Printf("WARNING [native sandbox]: ReadWritePaths field is ignored - native sandbox provides NO filesystem isolation. Command has full read/write access to entire filesystem, not just: %v. Use Docker or Kubernetes sandboxes for filesystem restrictions.", cmd.ReadWritePaths)
	}
	if !cmd.NetworkEnabled {
		log.Printf("WARNING [native sandbox]: NetworkEnabled=false is ignored - native sandbox provides NO network isolation. Command has full network access. Use Docker or Kubernetes sandboxes to disable network access.")
	}
}

// limitedBuffer implements io.Writer with a maximum size limit.
// Once the limit is reached, additional writes are silently discarded.
// This prevents OOM from commands producing excessive output.
type limitedBuffer struct {
	buf     bytes.Buffer
	maxSize int64
	size    int64
}

// Write implements io.Writer.
// Returns the number of bytes written (which may be less than len(p) if limit is reached).
func (lb *limitedBuffer) Write(p []byte) (n int, err error) {
	if lb.maxSize <= 0 {
		// No limit - write everything
		return lb.buf.Write(p)
	}

	available := lb.maxSize - lb.size
	if available <= 0 {
		// Already at limit - discard all input
		return len(p), nil
	}

	// Write as much as we can within the limit
	toWrite := int64(len(p))
	if toWrite > available {
		toWrite = available
	}

	written, err := lb.buf.Write(p[:toWrite])
	lb.size += int64(written)

	// Return full length to indicate "success" even if we truncated
	// This prevents the command from failing due to write errors
	return len(p), err
}

// String returns the buffered content as a string.
// If the output was truncated, it includes a truncation notice.
func (lb *limitedBuffer) String() string {
	s := lb.buf.String()
	if lb.maxSize > 0 && lb.size >= lb.maxSize {
		s += fmt.Sprintf("\n... [output truncated at %d bytes]", lb.maxSize)
	}
	return s
}
