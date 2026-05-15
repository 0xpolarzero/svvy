//go:build linux

package landlock

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// CommandSandboxer applies Landlock restrictions before executing a command.
// This provides integration with the sandbox.Command interface.
type CommandSandboxer struct {
	// ReadOnlyPaths are paths that can be read but not modified
	ReadOnlyPaths []string

	// ReadWritePaths are paths that can be both read and written
	ReadWritePaths []string

	// BestEffort enables graceful degradation on older kernels
	BestEffort bool
}

// NewCommandSandboxer creates a new command sandboxer with default settings.
func NewCommandSandboxer() *CommandSandboxer {
	return &CommandSandboxer{
		ReadOnlyPaths:  []string{"/"},
		ReadWritePaths: []string{"/dev/null"},
		BestEffort:     true,
	}
}

// WithReadOnlyPaths sets the read-only paths.
func (cs *CommandSandboxer) WithReadOnlyPaths(paths ...string) *CommandSandboxer {
	cs.ReadOnlyPaths = paths
	return cs
}

// WithReadWritePaths sets the read-write paths.
func (cs *CommandSandboxer) WithReadWritePaths(paths ...string) *CommandSandboxer {
	cs.ReadWritePaths = paths
	return cs
}

// WithBestEffort enables or disables best-effort mode.
func (cs *CommandSandboxer) WithBestEffort(enabled bool) *CommandSandboxer {
	cs.BestEffort = enabled
	return cs
}

// ApplyAndExec applies Landlock restrictions and executes a command.
// This is designed to be called in a forked process before exec.
//
// Example usage:
//
//	sandboxer := NewCommandSandboxer()
//	sandboxer.WithReadWritePaths("/tmp", "/workspace")
//	err := sandboxer.ApplyAndExec(ctx, "ls", "-l", "/tmp")
func (cs *CommandSandboxer) ApplyAndExec(ctx context.Context, program string, args ...string) error {
	// Apply Landlock restrictions first
	if err := cs.Apply(); err != nil {
		return fmt.Errorf("failed to apply landlock: %w", err)
	}

	// Execute the command
	cmd := exec.CommandContext(ctx, program, args...)
	return cmd.Run()
}

// Apply applies the Landlock restrictions without executing a command.
// This is useful when you want to apply restrictions to the current process
// before doing other operations.
func (cs *CommandSandboxer) Apply() error {
	policy := Policy{
		ReadOnlyPaths:  cs.ReadOnlyPaths,
		ReadWritePaths: cs.ReadWritePaths,
		BestEffort:     cs.BestEffort,
	}

	return policy.Apply()
}

// ExecConfig configures command execution with Landlock restrictions.
type ExecConfig struct {
	// Program is the command to execute
	Program string

	// Args are the command arguments
	Args []string

	// WorkingDirectory is the directory to execute in
	WorkingDirectory string

	// Environment contains environment variables
	Environment map[string]string

	// ReadOnlyPaths are paths that can be read
	ReadOnlyPaths []string

	// ReadWritePaths are paths that can be written
	ReadWritePaths []string

	// Timeout is the maximum execution duration
	Timeout time.Duration
}

// ExecuteWithLandlock executes a command with Landlock restrictions applied.
// This is a high-level helper that combines command execution with sandboxing.
//
// Note: This should typically be used in a forked process, as Landlock
// restrictions cannot be removed once applied.
func ExecuteWithLandlock(ctx context.Context, config ExecConfig) error {
	// Apply timeout if specified
	if config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.Timeout)
		defer cancel()
	}

	// Create and apply Landlock policy
	policy := Policy{
		ReadOnlyPaths:  config.ReadOnlyPaths,
		ReadWritePaths: config.ReadWritePaths,
		BestEffort:     true, // Graceful degradation by default
	}

	if err := policy.Apply(); err != nil {
		return fmt.Errorf("failed to apply landlock: %w", err)
	}

	// Create command
	cmd := exec.CommandContext(ctx, config.Program, config.Args...)

	if config.WorkingDirectory != "" {
		cmd.Dir = config.WorkingDirectory
	}

	if len(config.Environment) > 0 {
		cmd.Env = make([]string, 0, len(config.Environment))
		for k, v := range config.Environment {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Execute
	return cmd.Run()
}

// PreExecHook is a function that can be called before exec() in a forked process.
// It applies Landlock restrictions suitable for sandboxed command execution.
type PreExecHook func() error

// CreatePreExecHook creates a pre-exec hook that applies Landlock restrictions.
// This is designed to be used with libraries that support pre-exec hooks in forked processes.
//
// Example:
//
//	hook := CreatePreExecHook([]string{"/"}, []string{"/tmp"})
//	// Pass hook to process spawning library
func CreatePreExecHook(readOnlyPaths, readWritePaths []string) PreExecHook {
	return func() error {
		policy := Policy{
			ReadOnlyPaths:  readOnlyPaths,
			ReadWritePaths: readWritePaths,
			BestEffort:     true,
		}
		return policy.Apply()
	}
}

// SandboxedRunner wraps command execution with Landlock restrictions.
type SandboxedRunner struct {
	readOnlyPaths  []string
	readWritePaths []string
}

// NewSandboxedRunner creates a new sandboxed command runner.
func NewSandboxedRunner(readOnlyPaths, readWritePaths []string) *SandboxedRunner {
	return &SandboxedRunner{
		readOnlyPaths:  readOnlyPaths,
		readWritePaths: readWritePaths,
	}
}

// Run executes a command with Landlock restrictions.
// Note: This applies restrictions to the current process, so it's typically
// used in a forked child process.
func (sr *SandboxedRunner) Run(ctx context.Context, program string, args ...string) error {
	// Apply Landlock
	policy := Policy{
		ReadOnlyPaths:  sr.readOnlyPaths,
		ReadWritePaths: sr.readWritePaths,
		BestEffort:     true,
	}

	if err := policy.Apply(); err != nil {
		return err
	}

	// Execute command
	cmd := exec.CommandContext(ctx, program, args...)
	return cmd.Run()
}

// ApplyForSandbox applies Landlock restrictions suitable for the sandbox.Command interface.
// This is designed to match the functionality of the Rust implementation.
//
// Parameters:
//   - allowFullRead: if true, allows reading the entire filesystem
//   - writableRoots: directories that should be writable
//   - workingDirectory: the command's working directory (added to readable paths)
func ApplyForSandbox(allowFullRead bool, writableRoots []string, workingDirectory string) error {
	ruleset := NewRuleset()

	// Add read access
	if allowFullRead {
		ruleset.AddReadOnlyPath("/")
	} else {
		// Minimal read access
		ruleset.AddReadOnlyPath("/usr")
		ruleset.AddReadOnlyPath("/lib")
		ruleset.AddReadOnlyPath("/lib64")
		ruleset.AddReadOnlyPath("/bin")
		ruleset.AddReadOnlyPath("/sbin")
		ruleset.AddReadOnlyPath("/etc")
	}

	// Add /dev/null (commonly needed)
	ruleset.AddReadWritePath("/dev/null")

	// Add writable roots
	for _, root := range writableRoots {
		ruleset.AddReadWritePath(root)
	}

	// Add working directory if specified and not covered by other rules
	if workingDirectory != "" && !allowFullRead {
		covered := false
		for _, root := range writableRoots {
			if workingDirectory == root {
				covered = true
				break
			}
		}
		if !covered {
			ruleset.AddReadOnlyPath(workingDirectory)
		}
	}

	return ruleset.TryApply()
}
