//go:build linux

package landlock

import (
	"fmt"
	"os"
	"syscall"
)

// Rule represents a filesystem access rule for a specific path.
// Rules define which operations are allowed on a path and its descendants.
type Rule struct {
	// Path is the filesystem path this rule applies to.
	// The rule applies to this path and all paths beneath it.
	Path string

	// Access specifies the allowed access rights (combination of AccessFS* constants).
	Access uint64
}

// Ruleset represents a collection of Landlock rules that can be applied to the current process.
// A ruleset defines the complete filesystem access policy for a sandboxed process.
type Ruleset struct {
	// rules is the list of path-based rules to apply.
	rules []Rule

	// handledAccess is the set of all access rights handled by this ruleset.
	// Any access right not in this set is implicitly denied.
	handledAccess uint64

	// fd is the file descriptor for the created ruleset (set after calling create).
	fd int

	// created indicates whether the ruleset has been created via the kernel.
	created bool
}

// NewRuleset creates a new empty Landlock ruleset.
// Rules can be added using AddRule, AddReadOnlyPath, and AddReadWritePath methods.
func NewRuleset() *Ruleset {
	return &Ruleset{
		rules:         make([]Rule, 0),
		handledAccess: 0,
		fd:            -1,
		created:       false,
	}
}

// AddRule adds a custom rule to the ruleset with specific access rights.
// The path must be an absolute path. The access parameter should be a combination
// of AccessFS* constants.
//
// Example:
//
//	ruleset.AddRule("/home/user", AccessFSReadFile | AccessFSReadDir)
func (r *Ruleset) AddRule(path string, access uint64) *Ruleset {
	r.rules = append(r.rules, Rule{
		Path:   path,
		Access: access,
	})

	// Update handled access to include all access rights from all rules
	r.handledAccess |= access

	return r
}

// AddReadOnlyPath adds a rule that allows read-only access to the specified path.
// This allows reading files, listing directories, and executing programs, but no modifications.
//
// Example:
//
//	ruleset.AddReadOnlyPath("/usr")
//	ruleset.AddReadOnlyPath("/etc")
func (r *Ruleset) AddReadOnlyPath(path string) *Ruleset {
	return r.AddRule(path, AccessFSReadOnly)
}

// AddReadWritePath adds a rule that allows full read-write access to the specified path.
// This allows all filesystem operations including creating, modifying, and deleting files.
//
// Example:
//
//	ruleset.AddReadWritePath("/tmp")
//	ruleset.AddReadWritePath("/home/user/workspace")
func (r *Ruleset) AddReadWritePath(path string) *Ruleset {
	return r.AddRule(path, AccessFSReadWrite)
}

// AddDenyPath adds a rule that denies all access to the specified path.
// This is equivalent to not adding any rule for the path, but can be used for clarity.
// Note: In Landlock, absence of a rule means no access, so this is typically not needed.
func (r *Ruleset) AddDenyPath(path string) *Ruleset {
	// In Landlock, deny is the default - no rule means no access.
	// We add a rule with zero access rights for explicit documentation.
	return r.AddRule(path, 0)
}

// WithHandledAccess explicitly sets the access rights that this ruleset handles.
// This allows more fine-grained control over which operations are restricted.
// If not called, handledAccess is automatically set to include all access rights
// from all added rules.
//
// Example:
//
//	// Only handle read and execute, allow all write operations
//	ruleset.WithHandledAccess(AccessFSReadFile | AccessFSReadDir | AccessFSExecute)
func (r *Ruleset) WithHandledAccess(access uint64) *Ruleset {
	r.handledAccess = access
	return r
}

// create creates the kernel-level ruleset and returns its file descriptor.
// This method is called internally by Apply() and should not be called directly.
func (r *Ruleset) create() error {
	if r.created {
		return fmt.Errorf("ruleset already created")
	}

	// If no handled access is set, default to all access rights from all rules
	if r.handledAccess == 0 {
		// If no rules, default to full read-write access handling
		if len(r.rules) == 0 {
			r.handledAccess = AccessFSReadWrite
		}
	}

	// Create the ruleset in the kernel
	fd, err := createRuleset(r.handledAccess)
	if err != nil {
		return fmt.Errorf("failed to create landlock ruleset: %w", err)
	}

	r.fd = fd
	r.created = true

	return nil
}

// addRulesToKernel adds all rules from the ruleset to the kernel.
// This method is called internally by Apply() and should not be called directly.
func (r *Ruleset) addRulesToKernel() error {
	if !r.created {
		return fmt.Errorf("ruleset not created")
	}

	for _, rule := range r.rules {
		// Open the path to get a file descriptor
		// O_PATH allows opening any path type (file, directory, symlink) without access
		// O_CLOEXEC ensures the fd is closed on exec
		pathFd, err := syscall.Open(rule.Path, syscall.O_PATH|syscall.O_CLOEXEC, 0)
		if err != nil {
			return fmt.Errorf("failed to open path %s: %w", rule.Path, err)
		}

		// Add the rule to the kernel ruleset
		err = addRule(r.fd, pathFd, rule.Access)

		// Close the path fd (no longer needed after adding the rule)
		closeErr := syscall.Close(pathFd)

		// Check for errors
		if err != nil {
			return fmt.Errorf("failed to add rule for path %s: %w", rule.Path, err)
		}
		if closeErr != nil {
			return fmt.Errorf("failed to close path fd for %s: %w", rule.Path, closeErr)
		}
	}

	return nil
}

// Close closes the ruleset file descriptor if it has been created.
// This should be called after Apply() to clean up resources.
// After closing, the ruleset cannot be reused.
func (r *Ruleset) Close() error {
	if !r.created || r.fd < 0 {
		return nil
	}

	err := syscall.Close(r.fd)
	r.fd = -1
	r.created = false

	if err != nil {
		return fmt.Errorf("failed to close ruleset fd: %w", err)
	}

	return nil
}

// Apply applies the ruleset to the current process, restricting filesystem access.
// This operation is irreversible - once applied, restrictions cannot be removed.
// The process and all its children will be subject to these restrictions.
//
// The ruleset must have at least one rule before calling Apply.
//
// Returns an error if:
//   - Landlock is not supported on the current kernel
//   - The ruleset has no rules
//   - Failed to create the kernel ruleset
//   - Failed to add rules to the kernel
//   - Failed to restrict the current process
//
// Example:
//
//	ruleset := NewRuleset()
//	ruleset.AddReadOnlyPath("/")
//	ruleset.AddReadWritePath("/tmp")
//	if err := ruleset.Apply(); err != nil {
//		return fmt.Errorf("failed to apply landlock: %w", err)
//	}
func (r *Ruleset) Apply() error {
	// Check if Landlock is supported
	if !IsSupported() {
		return fmt.Errorf("landlock is not supported on this kernel (requires Linux 5.13+)")
	}

	// Validate ruleset has rules
	if len(r.rules) == 0 {
		return fmt.Errorf("cannot apply empty ruleset: add at least one rule")
	}

	// Set no_new_privs before creating ruleset
	if err := setNoNewPrivs(); err != nil {
		return fmt.Errorf("failed to set no_new_privs: %w", err)
	}

	// Create the kernel ruleset if not already created
	if !r.created {
		if err := r.create(); err != nil {
			return err
		}
	}

	// Add all rules to the kernel ruleset
	if err := r.addRulesToKernel(); err != nil {
		// Clean up on error
		r.Close()
		return err
	}

	// Apply the ruleset to the current process
	if err := restrictSelf(r.fd); err != nil {
		// Clean up on error
		r.Close()
		return fmt.Errorf("failed to restrict process: %w", err)
	}

	// Close the ruleset fd after successful application
	// The restrictions persist even after closing the fd
	return r.Close()
}

// ApplyDefault applies a default Landlock policy that:
//   - Allows read-only access to the entire filesystem (/)
//   - Allows read-write access to /dev/null
//   - Allows read-write access to the specified writable roots
//
// This is a convenience function that matches the Rust implementation's default policy.
// It's suitable for most sandboxing use cases where you want to restrict write access
// while allowing reads everywhere.
//
// Example:
//
//	if err := ApplyDefault([]string{"/tmp", "/home/user/workspace"}); err != nil {
//		return err
//	}
func ApplyDefault(writableRoots []string) error {
	ruleset := NewRuleset()

	// Allow read-only access to the entire filesystem
	ruleset.AddReadOnlyPath("/")

	// Allow writing to /dev/null (commonly needed by programs)
	ruleset.AddReadWritePath("/dev/null")

	// Allow writing to specified writable roots
	for _, root := range writableRoots {
		ruleset.AddReadWritePath(root)
	}

	return ruleset.Apply()
}

// ApplyReadOnly applies a Landlock policy that allows read-only access to the entire filesystem.
// No write operations are permitted anywhere (except to already-open file descriptors).
//
// This is the most restrictive policy and is suitable for running untrusted code that
// only needs to read existing files.
//
// Example:
//
//	if err := ApplyReadOnly(); err != nil {
//		return err
//	}
func ApplyReadOnly() error {
	ruleset := NewRuleset()
	ruleset.AddReadOnlyPath("/")
	return ruleset.Apply()
}

// TryApply attempts to apply the ruleset, but returns nil if Landlock is not supported.
// This is useful for optional sandboxing that gracefully degrades on older kernels.
//
// Returns:
//   - nil if Landlock is not supported (kernel < 5.13)
//   - nil if the ruleset was successfully applied
//   - error if Landlock is supported but application failed
//
// Example:
//
//	// Apply sandboxing if available, continue without it if not
//	if err := ruleset.TryApply(); err != nil {
//		return fmt.Errorf("sandboxing failed: %w", err)
//	}
func (r *Ruleset) TryApply() error {
	// Check if Landlock is supported
	if !IsSupported() {
		// Not supported, but that's okay - return nil
		return nil
	}

	// Landlock is supported, so we should apply it
	return r.Apply()
}

// TryApplyDefault is like ApplyDefault but returns nil if Landlock is not supported.
// This provides optional sandboxing that gracefully degrades on older kernels.
func TryApplyDefault(writableRoots []string) error {
	if !IsSupported() {
		return nil
	}
	return ApplyDefault(writableRoots)
}

// TryApplyReadOnly is like ApplyReadOnly but returns nil if Landlock is not supported.
// This provides optional sandboxing that gracefully degrades on older kernels.
func TryApplyReadOnly() error {
	if !IsSupported() {
		return nil
	}
	return ApplyReadOnly()
}

// GetKernelVersion returns the Linux kernel version as a string.
// This is useful for debugging and logging.
func GetKernelVersion() (string, error) {
	var utsname syscall.Utsname
	if err := syscall.Uname(&utsname); err != nil {
		return "", fmt.Errorf("failed to get kernel version: %w", err)
	}

	// Convert byte array to string
	release := make([]byte, 0, len(utsname.Release))
	for _, b := range utsname.Release {
		if b == 0 {
			break
		}
		release = append(release, byte(b))
	}

	return string(release), nil
}

// GetInfo returns information about Landlock support on the current system.
// This includes kernel version, Landlock support status, and ABI version.
type Info struct {
	// KernelVersion is the Linux kernel version string
	KernelVersion string

	// Supported indicates if Landlock is supported
	Supported bool

	// ABIVersion is the Landlock ABI version (0 if not supported)
	ABIVersion int
}

// GetInfo returns detailed information about Landlock support on the current system.
func GetInfo() (*Info, error) {
	kernelVersion, err := GetKernelVersion()
	if err != nil {
		kernelVersion = "unknown"
	}

	return &Info{
		KernelVersion: kernelVersion,
		Supported:     IsSupported(),
		ABIVersion:    GetABIVersion(),
	}, nil
}

// MustApply is like Apply but panics on error.
// This is useful for initialization code where sandboxing is mandatory.
func (r *Ruleset) MustApply() {
	if err := r.Apply(); err != nil {
		panic(fmt.Sprintf("failed to apply landlock ruleset: %v", err))
	}
}

// MustApplyDefault is like ApplyDefault but panics on error.
func MustApplyDefault(writableRoots []string) {
	if err := ApplyDefault(writableRoots); err != nil {
		panic(fmt.Sprintf("failed to apply default landlock policy: %v", err))
	}
}

// MustApplyReadOnly is like ApplyReadOnly but panics on error.
func MustApplyReadOnly() {
	if err := ApplyReadOnly(); err != nil {
		panic(fmt.Sprintf("failed to apply read-only landlock policy: %v", err))
	}
}

// ValidatePath checks if a path is accessible under the current restrictions.
// This is a best-effort check and may not reflect all Landlock restrictions.
func ValidatePath(path string) error {
	// Try to stat the path
	_, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path not accessible: %w", err)
	}
	return nil
}
