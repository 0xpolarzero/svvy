// Package sandbox provides interfaces and types for sandboxed command execution.
//
// The sandbox package defines the abstraction for executing commands in various
// isolation environments including native, Docker, and Kubernetes.
package sandbox

import (
	"context"
	"time"
)

// Sandbox represents a command execution environment with isolation guarantees.
type Sandbox interface {
	// Type returns the sandbox type identifier (e.g., "native", "docker", "kubernetes")
	Type() string

	// IsAvailable checks if this sandbox type is available on the current system
	IsAvailable() bool

	// Execute runs a command in the sandbox and returns the result
	Execute(ctx context.Context, cmd *Command) (*Result, error)

	// Cleanup performs any necessary cleanup (e.g., removing containers, pods)
	Cleanup(ctx context.Context) error
}

// Command represents a command to be executed in a sandbox.
type Command struct {
	// Program is the command to execute
	Program string

	// Args are the command arguments
	Args []string

	// WorkingDirectory is the directory to execute the command in
	WorkingDirectory string

	// Environment contains environment variables as key-value pairs
	Environment map[string]string

	// ReadOnlyPaths are filesystem paths to mount read-only
	ReadOnlyPaths []string

	// ReadWritePaths are filesystem paths to mount read-write
	ReadWritePaths []string

	// NetworkEnabled controls network access
	NetworkEnabled bool

	// Stdin is optional input to pass to the command
	Stdin string

	// Timeout specifies the maximum execution duration
	Timeout time.Duration
}

// Result contains the output and status of a sandbox execution.
type Result struct {
	// Stdout is the standard output
	Stdout string

	// Stderr is the standard error output
	Stderr string

	// ExitCode is the command exit code
	ExitCode int

	// ExecutionTime is how long the command took to execute
	ExecutionTime time.Duration

	// Error is set if sandbox execution failed (not command failure)
	Error error

	// Violation is set if a sandbox policy violation was detected
	Violation *Violation
}

// Commander is an interface for executing commands (used internally by sandboxes).
// This abstraction allows sandboxes to be tested without actually executing commands.
type Commander interface {
	// Run executes a command and returns stdout, stderr, exit code, and error
	Run(ctx context.Context, program string, args ...string) (stdout, stderr string, exitCode int, err error)
}
