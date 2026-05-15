//go:build linux

package landlock

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Policy represents a high-level Landlock policy configuration.
// This provides a more convenient API for common sandboxing scenarios.
type Policy struct {
	// ReadOnlyPaths are paths that can be read but not modified
	ReadOnlyPaths []string

	// ReadWritePaths are paths that can be both read and written
	ReadWritePaths []string

	// AllowedAccessFS explicitly sets which filesystem access rights to handle.
	// If 0, defaults to AccessFSReadWrite.
	AllowedAccessFS uint64

	// BestEffort enables graceful degradation on older kernels.
	// If true, returns nil when Landlock is not supported instead of an error.
	BestEffort bool
}

// Apply applies the policy to the current process using Landlock.
func (p *Policy) Apply() error {
	// Check support
	if !IsSupported() {
		if p.BestEffort {
			return nil
		}
		return fmt.Errorf("landlock not supported (requires kernel 5.13+)")
	}

	// Validate policy has at least one path
	if len(p.ReadOnlyPaths) == 0 && len(p.ReadWritePaths) == 0 {
		return fmt.Errorf("policy must have at least one path")
	}

	// Create ruleset
	ruleset := NewRuleset()

	// Set handled access if specified
	if p.AllowedAccessFS != 0 {
		ruleset.WithHandledAccess(p.AllowedAccessFS)
	}

	// Add read-only paths
	for _, path := range p.ReadOnlyPaths {
		ruleset.AddReadOnlyPath(path)
	}

	// Add read-write paths
	for _, path := range p.ReadWritePaths {
		ruleset.AddReadWritePath(path)
	}

	// Apply the ruleset
	return ruleset.Apply()
}

// PolicyBuilder provides a fluent API for building Landlock policies.
type PolicyBuilder struct {
	policy Policy
}

// NewPolicy creates a new policy builder.
func NewPolicy() *PolicyBuilder {
	return &PolicyBuilder{
		policy: Policy{
			ReadOnlyPaths:  make([]string, 0),
			ReadWritePaths: make([]string, 0),
		},
	}
}

// AddReadOnly adds one or more read-only paths to the policy.
func (pb *PolicyBuilder) AddReadOnly(paths ...string) *PolicyBuilder {
	pb.policy.ReadOnlyPaths = append(pb.policy.ReadOnlyPaths, paths...)
	return pb
}

// AddReadWrite adds one or more read-write paths to the policy.
func (pb *PolicyBuilder) AddReadWrite(paths ...string) *PolicyBuilder {
	pb.policy.ReadWritePaths = append(pb.policy.ReadWritePaths, paths...)
	return pb
}

// WithHandledAccess sets the filesystem access rights to handle.
func (pb *PolicyBuilder) WithHandledAccess(access uint64) *PolicyBuilder {
	pb.policy.AllowedAccessFS = access
	return pb
}

// WithBestEffort enables graceful degradation on older kernels.
func (pb *PolicyBuilder) WithBestEffort(enabled bool) *PolicyBuilder {
	pb.policy.BestEffort = enabled
	return pb
}

// Build returns the built policy.
func (pb *PolicyBuilder) Build() *Policy {
	return &pb.policy
}

// Apply builds and applies the policy in one step.
func (pb *PolicyBuilder) Apply() error {
	return pb.policy.Apply()
}

// ApplyToCurrentProcess applies a Landlock policy to the current process.
// This is a convenience function that wraps the Policy API.
func ApplyToCurrentProcess(readOnlyPaths, readWritePaths []string) error {
	policy := Policy{
		ReadOnlyPaths:  readOnlyPaths,
		ReadWritePaths: readWritePaths,
	}
	return policy.Apply()
}

// RestrictFilesystemAccess restricts filesystem access for the current process.
// This applies a default policy that:
//   - Allows read-only access to system directories
//   - Allows read-write access to specified work directories
//   - Allows write access to /dev/null and /dev/zero
//
// This is suitable for most sandboxing scenarios.
func RestrictFilesystemAccess(workDirectories []string) error {
	// Common read-only system paths
	readOnlyPaths := []string{
		"/",      // Root filesystem (read-only by default)
		"/usr",   // System binaries and libraries
		"/lib",   // System libraries
		"/lib64", // 64-bit system libraries
		"/bin",   // Essential binaries
		"/sbin",  // System binaries
		"/etc",   // System configuration
		"/opt",   // Optional software
		"/proc",  // Process information
		"/sys",   // System information
	}

	// Always-writable paths
	readWritePaths := []string{
		"/dev/null",
		"/dev/zero",
	}

	// Add user-specified work directories
	readWritePaths = append(readWritePaths, workDirectories...)

	policy := Policy{
		ReadOnlyPaths:  readOnlyPaths,
		ReadWritePaths: readWritePaths,
	}

	return policy.Apply()
}

// SandboxOptions configures sandboxing behavior for command execution.
type SandboxOptions struct {
	// WorkingDirectory is the current working directory for the command
	WorkingDirectory string

	// WritableRoots are directories that should be writable
	WritableRoots []string

	// AllowFullRead allows reading from the entire filesystem
	// If false, only specific paths are readable
	AllowFullRead bool

	// AllowDevNull allows writing to /dev/null (usually desired)
	AllowDevNull bool
}

// ApplyForCommand applies Landlock restrictions suitable for running a sandboxed command.
// This is designed to work with the sandbox.Command interface.
func (opts *SandboxOptions) ApplyForCommand() error {
	if !IsSupported() {
		// Landlock not supported, continue without sandboxing
		return nil
	}

	ruleset := NewRuleset()

	// Add full read access if requested
	if opts.AllowFullRead {
		ruleset.AddReadOnlyPath("/")
	} else {
		// Add minimal read access
		ruleset.AddReadOnlyPath("/usr")
		ruleset.AddReadOnlyPath("/lib")
		ruleset.AddReadOnlyPath("/lib64")
		ruleset.AddReadOnlyPath("/bin")
		ruleset.AddReadOnlyPath("/sbin")
		ruleset.AddReadOnlyPath("/etc")
	}

	// Add /dev/null if requested
	if opts.AllowDevNull {
		ruleset.AddReadWritePath("/dev/null")
	}

	// Add writable roots
	for _, root := range opts.WritableRoots {
		// Resolve to absolute path
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return fmt.Errorf("failed to resolve path %s: %w", root, err)
		}
		ruleset.AddReadWritePath(absRoot)
	}

	// Add working directory if specified and not already included
	if opts.WorkingDirectory != "" {
		absWD, err := filepath.Abs(opts.WorkingDirectory)
		if err != nil {
			return fmt.Errorf("failed to resolve working directory %s: %w", opts.WorkingDirectory, err)
		}

		// Check if working directory is already covered by writable roots
		covered := false
		for _, root := range opts.WritableRoots {
			absRoot, _ := filepath.Abs(root)
			if strings.HasPrefix(absWD, absRoot) {
				covered = true
				break
			}
		}

		if !covered {
			// Working directory not writable, add as read-only
			ruleset.AddReadOnlyPath(absWD)
		}
	}

	return ruleset.TryApply()
}

// DefaultSandboxOptions returns default sandbox options for command execution.
func DefaultSandboxOptions() *SandboxOptions {
	cwd, _ := os.Getwd()
	return &SandboxOptions{
		WorkingDirectory: cwd,
		WritableRoots:    []string{},
		AllowFullRead:    true,
		AllowDevNull:     true,
	}
}

// ApplySandboxForPaths is a convenience function that applies Landlock based on
// allowed read and write paths, similar to the Rust implementation.
//
// This matches the behavior of the Rust `install_filesystem_landlock_rules_on_current_thread` function.
func ApplySandboxForPaths(writableRoots []string) error {
	if !IsSupported() {
		return nil // Best effort - continue without Landlock on older kernels
	}

	ruleset := NewRuleset()

	// Allow read-only access to entire filesystem
	ruleset.AddReadOnlyPath("/")

	// Allow write access to /dev/null
	ruleset.AddReadWritePath("/dev/null")

	// Allow write access to specified roots
	for _, root := range writableRoots {
		// Ensure path exists before adding
		if _, err := os.Stat(root); err == nil {
			ruleset.AddReadWritePath(root)
		}
	}

	return ruleset.Apply()
}

// SandboxFunc is a function that runs under Landlock restrictions.
type SandboxFunc func() error

// RunSandboxed executes a function with Landlock restrictions applied.
// The restrictions are applied before the function runs and persist for
// the lifetime of the process.
//
// Note: Landlock restrictions cannot be removed once applied, so this
// affects the entire process, not just the function execution.
//
// Example:
//
//	err := RunSandboxed(func() error {
//		// This code runs with Landlock restrictions
//		return doSomething()
//	}, []string{"/tmp"})
func RunSandboxed(fn SandboxFunc, writableRoots []string) error {
	// Apply sandbox
	if err := ApplySandboxForPaths(writableRoots); err != nil {
		return fmt.Errorf("failed to apply sandbox: %w", err)
	}

	// Run the function
	return fn()
}

// TestSupport checks if Landlock is supported and returns a detailed error message if not.
// This is useful for testing and debugging.
func TestSupport() error {
	info, err := GetInfo()
	if err != nil {
		return fmt.Errorf("failed to get landlock info: %w", err)
	}

	if !info.Supported {
		return fmt.Errorf("landlock not supported: kernel=%s, abi_version=%d (requires Linux 5.13+)",
			info.KernelVersion, info.ABIVersion)
	}

	return nil
}

// EnableBestEffort returns true if Landlock should use best-effort mode.
// This can be controlled via the LANDLOCK_BEST_EFFORT environment variable.
func EnableBestEffort() bool {
	return os.Getenv("LANDLOCK_BEST_EFFORT") != ""
}

// CheckAccess verifies if a specific operation would be allowed on a path
// under the given ruleset configuration. This is a testing/validation helper.
//
// Note: This only checks if the path exists and is within allowed rules.
// It doesn't actually enforce Landlock restrictions.
func CheckAccess(path string, access uint64, allowedPaths map[string]uint64) bool {
	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// Check each allowed path
	for allowedPath, allowedAccess := range allowedPaths {
		// Check if path is under this allowed path
		if strings.HasPrefix(absPath, allowedPath) {
			// Check if requested access is allowed
			if (allowedAccess & access) == access {
				return true
			}
		}
	}

	return false
}
