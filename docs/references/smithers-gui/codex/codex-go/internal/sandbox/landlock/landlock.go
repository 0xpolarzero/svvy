//go:build linux

// Package landlock provides Linux Landlock LSM (Linux Security Module) support for filesystem access control.
//
// Landlock is a Linux kernel security module (LSM) introduced in kernel 5.13 that allows unprivileged
// processes to restrict their own filesystem access. It works by creating rulesets that specify
// allowed operations on specific paths.
//
// This implementation supports Landlock ABI v1 (kernel 5.13+) with the following features:
//   - Creating filesystem access rulesets
//   - Adding path-based rules (read-only, read-write)
//   - Applying rulesets to the current process
//   - Kernel version detection and graceful fallback
//
// Example usage:
//
//	// Create a ruleset that allows read-only access to /usr and read-write to /tmp
//	ruleset, err := NewRuleset()
//	if err != nil {
//		return err
//	}
//	if err := ruleset.AddReadOnlyPath("/usr"); err != nil {
//		return err
//	}
//	if err := ruleset.AddReadWritePath("/tmp"); err != nil {
//		return err
//	}
//	if err := ruleset.Apply(); err != nil {
//		return err
//	}
package landlock

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Landlock syscall numbers (added in Linux 5.13)
const (
	sysLandlockCreateRuleset = 444
	sysLandlockAddRule       = 445
	sysLandlockRestrictSelf  = 446
)

// Landlock rule types
const (
	landlockRulePathBeneath = 1
)

// Landlock ABI version 1 access rights (kernel 5.13+)
// These constants define filesystem operations that can be controlled by Landlock.
const (
	// AccessFSExecute allows executing a file.
	AccessFSExecute uint64 = 1 << 0

	// AccessFSWriteFile allows opening a file with write access.
	AccessFSWriteFile uint64 = 1 << 1

	// AccessFSReadFile allows opening a file with read access.
	AccessFSReadFile uint64 = 1 << 2

	// AccessFSReadDir allows opening a directory or listing its content.
	AccessFSReadDir uint64 = 1 << 3

	// AccessFSRemoveDir allows removing an empty directory or renaming one.
	AccessFSRemoveDir uint64 = 1 << 4

	// AccessFSRemoveFile allows unlinking a file.
	AccessFSRemoveFile uint64 = 1 << 5

	// AccessFSMakeChar allows creating a character device.
	AccessFSMakeChar uint64 = 1 << 6

	// AccessFSMakeDir allows creating a directory.
	AccessFSMakeDir uint64 = 1 << 7

	// AccessFSMakeReg allows creating a regular file.
	AccessFSMakeReg uint64 = 1 << 8

	// AccessFSMakeSock allows creating a UNIX domain socket.
	AccessFSMakeSock uint64 = 1 << 9

	// AccessFSMakeFifo allows creating a named pipe.
	AccessFSMakeFifo uint64 = 1 << 10

	// AccessFSMakeBlock allows creating a block device.
	AccessFSMakeBlock uint64 = 1 << 11

	// AccessFSMakeSym allows creating a symbolic link.
	AccessFSMakeSym uint64 = 1 << 12
)

// Access right combinations for common use cases
const (
	// AccessFSReadOnly combines all read-only access rights.
	// Allows reading files and directories, executing files, but no modifications.
	AccessFSReadOnly = AccessFSExecute | AccessFSReadFile | AccessFSReadDir

	// AccessFSReadWrite combines all access rights for read-write access.
	// Allows all filesystem operations including reading, writing, creating, and deleting.
	AccessFSReadWrite = AccessFSExecute | AccessFSWriteFile | AccessFSReadFile |
		AccessFSReadDir | AccessFSRemoveDir | AccessFSRemoveFile |
		AccessFSMakeChar | AccessFSMakeDir | AccessFSMakeReg |
		AccessFSMakeSock | AccessFSMakeFifo | AccessFSMakeBlock |
		AccessFSMakeSym
)

// landlockRulesetAttr defines the desired Landlock ruleset configuration.
// This structure is passed to landlock_create_ruleset().
type landlockRulesetAttr struct {
	handledAccessFS uint64
}

// landlockPathBeneathAttr defines a path-based rule.
// This structure is passed to landlock_add_rule().
type landlockPathBeneathAttr struct {
	allowedAccess uint64
	parentFd      int32
}

// IsSupported checks if Landlock is supported on the current kernel.
// It attempts to create a ruleset with minimal permissions to detect support.
// Returns true if Landlock is available (kernel >= 5.13), false otherwise.
func IsSupported() bool {
	attr := landlockRulesetAttr{
		handledAccessFS: AccessFSReadFile,
	}

	fd, _, errno := syscall.Syscall6(
		sysLandlockCreateRuleset,
		uintptr(unsafe.Pointer(&attr)),
		uintptr(unsafe.Sizeof(attr)),
		0, // flags (must be 0)
		0, 0, 0,
	)

	if errno != 0 {
		return false
	}

	// Close the test ruleset fd
	if fd > 0 {
		syscall.Close(int(fd))
	}

	return true
}

// GetABIVersion returns the Landlock ABI version supported by the kernel.
// Returns 0 if Landlock is not supported.
// ABI version 1 was introduced in kernel 5.13.
func GetABIVersion() int {
	// Try to create a ruleset with invalid size to probe ABI version
	// The kernel returns the supported ABI version even on error
	attr := landlockRulesetAttr{
		handledAccessFS: AccessFSReadFile,
	}

	_, _, errno := syscall.Syscall6(
		sysLandlockCreateRuleset,
		uintptr(unsafe.Pointer(&attr)),
		uintptr(unsafe.Sizeof(attr)),
		0, // flags
		0, 0, 0,
	)

	// If ENOSYS, Landlock is not supported
	if errno == syscall.ENOSYS {
		return 0
	}

	// If EOPNOTSUPP, kernel was built without Landlock support
	if errno == syscall.EOPNOTSUPP {
		return 0
	}

	// For ABI v1, we assume support if the syscall exists
	// More sophisticated version detection could be added in the future
	if errno == 0 || errno == syscall.EINVAL || errno == syscall.ENOMSG {
		return 1
	}

	return 0
}

// createRuleset creates a new Landlock ruleset with the specified access rights.
// Returns a file descriptor for the ruleset, or an error.
func createRuleset(handledAccessFS uint64) (int, error) {
	attr := landlockRulesetAttr{
		handledAccessFS: handledAccessFS,
	}

	fd, _, errno := syscall.Syscall6(
		sysLandlockCreateRuleset,
		uintptr(unsafe.Pointer(&attr)),
		uintptr(unsafe.Sizeof(attr)),
		0, // flags (must be 0 for ABI v1)
		0, 0, 0,
	)

	if errno != 0 {
		return -1, fmt.Errorf("landlock_create_ruleset failed: %w", errno)
	}

	return int(fd), nil
}

// addRule adds a path-based rule to a Landlock ruleset.
// The rulesetFd is the file descriptor returned by createRuleset.
// The pathFd is a file descriptor for the path to restrict.
// The allowedAccess specifies which operations are allowed on this path.
func addRule(rulesetFd int, pathFd int, allowedAccess uint64) error {
	attr := landlockPathBeneathAttr{
		allowedAccess: allowedAccess,
		parentFd:      int32(pathFd),
	}

	_, _, errno := syscall.Syscall6(
		sysLandlockAddRule,
		uintptr(rulesetFd),
		uintptr(landlockRulePathBeneath),
		uintptr(unsafe.Pointer(&attr)),
		0, // flags (must be 0)
		0, 0,
	)

	if errno != 0 {
		return fmt.Errorf("landlock_add_rule failed: %w", errno)
	}

	return nil
}

// restrictSelf applies a Landlock ruleset to the current thread.
// After this call, the process and all its children are restricted by the ruleset.
// The rulesetFd is the file descriptor returned by createRuleset.
// This operation is irreversible - once applied, restrictions cannot be removed.
func restrictSelf(rulesetFd int) error {
	_, _, errno := syscall.Syscall6(
		sysLandlockRestrictSelf,
		uintptr(rulesetFd),
		0, // flags (must be 0)
		0, 0, 0, 0,
	)

	if errno != 0 {
		return fmt.Errorf("landlock_restrict_self failed: %w", errno)
	}

	return nil
}

// prctl is used to set PR_SET_NO_NEW_PRIVS.
// This prevents gaining new privileges through execve() and is required before applying Landlock.
func prctl(option int, arg2, arg3, arg4, arg5 uintptr) error {
	_, _, errno := syscall.Syscall6(
		syscall.SYS_PRCTL,
		uintptr(option),
		arg2, arg3, arg4, arg5, 0,
	)

	if errno != 0 {
		return fmt.Errorf("prctl failed: %w", errno)
	}

	return nil
}

// setNoNewPrivs sets PR_SET_NO_NEW_PRIVS on the current thread.
// This is required before applying a Landlock ruleset.
// It prevents the process from gaining additional privileges through execve().
func setNoNewPrivs() error {
	const prSetNoNewPrivs = 38 // PR_SET_NO_NEW_PRIVS
	return prctl(prSetNoNewPrivs, 1, 0, 0, 0)
}
