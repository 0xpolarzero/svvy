//go:build linux

// Package seccomp provides Linux Seccomp-BPF (Berkeley Packet Filter) syscall filtering.
//
// Seccomp is used for syscall filtering on older Linux kernels (< 5.13) that lack
// Landlock support. It provides security isolation by restricting which system calls
// a process can make, implementing a deny-by-default security model with explicit allowlists.
//
// The implementation uses BPF programs to filter syscalls at the kernel level with
// minimal performance overhead. Denied syscalls return EPERM errors instead of
// allowing potentially dangerous operations.
package seccomp

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// Seccomp operation modes
const (
	// SECCOMP_MODE_DISABLED - Seccomp is not in use
	SECCOMP_MODE_DISABLED = 0
	// SECCOMP_MODE_STRICT - Only read, write, _exit, and sigreturn are allowed
	SECCOMP_MODE_STRICT = 1
	// SECCOMP_MODE_FILTER - Use Berkeley Packet Filter rules
	SECCOMP_MODE_FILTER = 2
)

// prctl() commands for Seccomp
const (
	// PR_SET_SECCOMP - Set seccomp mode
	PR_SET_SECCOMP = 22
	// PR_GET_SECCOMP - Get seccomp mode
	PR_GET_SECCOMP = 21
	// PR_SET_NO_NEW_PRIVS - Disable privilege escalation
	PR_SET_NO_NEW_PRIVS = 38
)

// Seccomp filter flags
const (
	// SECCOMP_FILTER_FLAG_TSYNC - Synchronize filters across all threads
	SECCOMP_FILTER_FLAG_TSYNC = (1 << 0)
	// SECCOMP_FILTER_FLAG_LOG - Log all actions except SECCOMP_RET_ALLOW
	SECCOMP_FILTER_FLAG_LOG = (1 << 1)
	// SECCOMP_FILTER_FLAG_SPEC_ALLOW - Disable speculation for this filter
	SECCOMP_FILTER_FLAG_SPEC_ALLOW = (1 << 2)
)

// Seccomp return values (upper 16 bits of return value)
const (
	// SECCOMP_RET_KILL_PROCESS - Kill the entire process
	SECCOMP_RET_KILL_PROCESS = 0x80000000
	// SECCOMP_RET_KILL_THREAD - Kill the calling thread
	SECCOMP_RET_KILL_THREAD = 0x00000000
	// SECCOMP_RET_TRAP - Send SIGSYS signal
	SECCOMP_RET_TRAP = 0x00030000
	// SECCOMP_RET_ERRNO - Return errno value
	SECCOMP_RET_ERRNO = 0x00050000
	// SECCOMP_RET_TRACE - Notify ptrace tracer
	SECCOMP_RET_TRACE = 0x7ff00000
	// SECCOMP_RET_LOG - Allow after logging
	SECCOMP_RET_LOG = 0x7ffc0000
	// SECCOMP_RET_ALLOW - Allow the syscall
	SECCOMP_RET_ALLOW = 0x7fff0000
)

// Error codes
const (
	EPERM  = syscall.EPERM  // Operation not permitted
	EACCES = syscall.EACCES // Permission denied
)

// sock_fprog represents the BPF program structure for seccomp
type sockFprog struct {
	Len    uint16      // Number of filter instructions
	Filter *sockFilter // Pointer to filter array
}

// sock_filter represents a single BPF instruction
type sockFilter struct {
	Code uint16 // Instruction opcode
	Jt   uint8  // Jump if true
	Jf   uint8  // Jump if false
	K    uint32 // Generic multiuse field
}

// SeccompFilter represents a configured Seccomp-BPF filter
type SeccompFilter struct {
	program []sockFilter
	arch    string
}

// IsSupported checks if Seccomp-BPF is supported on the current kernel.
// It attempts to query the current seccomp mode via prctl(PR_GET_SECCOMP).
func IsSupported() bool {
	// Try to get the current seccomp mode
	_, _, errno := syscall.RawSyscall(syscall.SYS_PRCTL, PR_GET_SECCOMP, 0, 0)
	// If the syscall succeeds or fails with a known error, seccomp is supported
	// EINVAL would indicate the PR_GET_SECCOMP operation is not supported
	return errno != syscall.EINVAL
}

// SetNoNewPrivs disables privilege escalation for the calling thread.
// This must be called before installing a Seccomp filter as the kernel
// requires processes to be unable to gain new privileges before applying
// Seccomp restrictions. This prevents a restricted process from execve()ing
// a setuid binary to escape the sandbox.
func SetNoNewPrivs() error {
	_, _, errno := syscall.RawSyscall6(
		syscall.SYS_PRCTL,
		PR_SET_NO_NEW_PRIVS,
		1, // Enable
		0,
		0,
		0,
		0,
	)
	if errno != 0 {
		return fmt.Errorf("prctl(PR_SET_NO_NEW_PRIVS) failed: %v", errno)
	}
	return nil
}

// Apply installs the Seccomp-BPF filter for the current thread.
// The filter must have been properly initialized with a BPF program.
// Once applied, the filter cannot be removed - only made more restrictive.
//
// This function:
// 1. Locks the OS thread to ensure consistent application
// 2. Validates the filter program
// 3. Calls prctl(PR_SET_SECCOMP, SECCOMP_MODE_FILTER) to install the filter
//
// After successful application, any syscall not explicitly allowed by the
// filter will be denied with EPERM.
func (f *SeccompFilter) Apply() error {
	if len(f.program) == 0 {
		return fmt.Errorf("seccomp filter program is empty")
	}

	// Lock the OS thread - seccomp filters are per-thread
	runtime.LockOSThread()

	// Ensure no_new_privs is set
	if err := SetNoNewPrivs(); err != nil {
		return fmt.Errorf("failed to set no_new_privs: %w", err)
	}

	// Prepare the sock_fprog structure
	prog := sockFprog{
		Len:    uint16(len(f.program)),
		Filter: &f.program[0],
	}

	// Apply the filter using prctl(PR_SET_SECCOMP, SECCOMP_MODE_FILTER, prog)
	_, _, errno := syscall.RawSyscall(
		syscall.SYS_PRCTL,
		PR_SET_SECCOMP,
		SECCOMP_MODE_FILTER,
		uintptr(unsafe.Pointer(&prog)),
	)

	if errno != 0 {
		return fmt.Errorf("prctl(PR_SET_SECCOMP, SECCOMP_MODE_FILTER) failed: %v", errno)
	}

	return nil
}

// GetMode returns the current Seccomp mode for the calling thread.
// Returns one of:
//   - SECCOMP_MODE_DISABLED (0): Seccomp is not active
//   - SECCOMP_MODE_STRICT (1): Strict mode (only read/write/_exit/sigreturn allowed)
//   - SECCOMP_MODE_FILTER (2): Filter mode (BPF rules active)
func GetMode() (int, error) {
	mode, _, errno := syscall.RawSyscall(syscall.SYS_PRCTL, PR_GET_SECCOMP, 0, 0)
	if errno != 0 {
		return -1, fmt.Errorf("prctl(PR_GET_SECCOMP) failed: %v", errno)
	}
	return int(mode), nil
}

// GetArchitecture returns the current system architecture for BPF filtering.
// Supported architectures: amd64 (x86_64), arm64 (aarch64)
func GetArchitecture() string {
	return runtime.GOARCH
}
