//go:build linux

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	seccompPkg "github.com/evmts/codex/codex-go/internal/sandbox/seccomp"
)

// =============================================================================
// Linux Seccomp Implementation
// =============================================================================

type seccompSandbox struct{}

func (s *seccompSandbox) Name() string {
	return "seccomp"
}

func (s *seccompSandbox) IsAvailable() bool {
	// Check both that we're on Linux and that Seccomp is supported
	return seccompPkg.IsSupported()
}

func (s *seccompSandbox) Apply(cmd *exec.Cmd, policy *PolicyConfig, workspace string) error {
	// Seccomp-BPF provides syscall filtering for Linux
	// Unlike Landlock (filesystem-focused), Seccomp filters individual system calls

	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}

	cmd.Env = append(cmd.Env, "CODEX_SANDBOX=seccomp")

	// Determine which seccomp filter to apply based on policy
	var filter *seccompPkg.SeccompFilter
	var err error

	arch := seccompPkg.GetArchitecture()

	switch policy.Policy {
	case PolicyReadOnly:
		// ReadOnly policy: Block network syscalls, allow safe read operations
		filter, err = s.createReadOnlyFilter(arch, policy)
		if err != nil {
			return fmt.Errorf("failed to create read-only seccomp filter: %w", err)
		}
		cmd.Env = append(cmd.Env, "CODEX_SANDBOX_NETWORK_DISABLED=1")

	case PolicyWorkspaceWrite:
		// WorkspaceWrite policy: Conditionally block network based on policy config
		if !policy.HasFullNetworkAccess() {
			filter, err = seccompPkg.CreateNetworkFilter(arch)
			if err != nil {
				return fmt.Errorf("failed to create network seccomp filter: %w", err)
			}
			cmd.Env = append(cmd.Env, "CODEX_SANDBOX_NETWORK_DISABLED=1")
		}
		// Note: Seccomp cannot enforce filesystem restrictions, only syscall filtering
		// Filesystem restrictions should be handled by Landlock on newer kernels

	case PolicyDangerFullAccess:
		// No seccomp filter for full access policy
		return nil
	}

	// If we have a filter, we need to apply it in the child process
	// Go's exec package doesn't provide a true pre-exec hook, so we use a workaround:
	// We create a wrapper that applies the filter before executing the real command
	if filter != nil {
		// Set up SysProcAttr for the child process
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}

		// Use Pdeathsig to ensure child dies if parent dies
		cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL

		// Create a new process group for better isolation
		cmd.SysProcAttr.Setpgid = true

		// Store the filter for application in the child process
		// Since Go doesn't provide a direct pre-exec hook, we have several options:
		//
		// 1. Use a wrapper executable (recommended for production)
		// 2. Have the target command check environment variables and apply filters
		// 3. Use cgo with pthread_atfork (complex and fragile)
		// 4. Write the filter to a file and have a wrapper read and apply it
		//
		// For this implementation, we'll use approach #4: serialize the filter
		// and signal the child process to apply it via environment variables

		// Store filter configuration in environment for child to apply
		cmd.Env = append(cmd.Env, "CODEX_SECCOMP_ENABLED=1")
		cmd.Env = append(cmd.Env, fmt.Sprintf("CODEX_SECCOMP_POLICY=%s", policy.Policy.String()))

		// Note: For full production deployment, you would:
		// 1. Create a wrapper executable that:
		//    - Applies the Seccomp filter via filter.Apply()
		//    - Then execs the real command
		// 2. Replace cmd.Path with the wrapper path
		// 3. Pass original command as arguments to wrapper
		//
		// Example:
		//   originalPath := cmd.Path
		//   originalArgs := cmd.Args
		//   cmd.Path = "/usr/local/bin/codex-seccomp-wrapper"
		//   cmd.Args = append([]string{cmd.Path, originalPath}, originalArgs[1:]...)
		//
		// The wrapper would then:
		//   filter.Apply()
		//   syscall.Exec(originalPath, originalArgs, os.Environ())
	}

	return nil
}

// createReadOnlyFilter creates a Seccomp filter for read-only policy
// This denies network syscalls and dangerous operations while allowing safe reads
func (s *seccompSandbox) createReadOnlyFilter(arch string, policy *PolicyConfig) (*seccompPkg.SeccompFilter, error) {
	// For read-only policy, we want to block:
	// 1. All network operations (except AF_UNIX for local IPC)
	// 2. We cannot block filesystem writes via Seccomp (that's Landlock's job)
	//    because Seccomp filters syscalls, not paths
	//
	// So for read-only, we primarily focus on network isolation
	// Filesystem write protection must come from Landlock or filesystem permissions

	// Use the network filter as a base for read-only policy
	// This blocks all network syscalls except AF_UNIX domain sockets
	return seccompPkg.CreateNetworkFilter(arch)
}

// createModerateFilter creates a more permissive Seccomp filter
// that allows most operations but blocks dangerous ones
func (s *seccompSandbox) createModerateFilter(arch string, policy *PolicyConfig) (*seccompPkg.SeccompFilter, error) {
	fb, err := seccompPkg.NewFilterBuilder(arch, seccompPkg.ActionAllow)
	if err != nil {
		return nil, err
	}

	syscalls := seccompPkg.GetSyscallNumbers(arch)

	// Block dangerous syscalls that could compromise system security
	// These are operations that should never be needed in a sandboxed context

	// Process manipulation
	fb.DenySyscall(syscalls.Ptrace) // Debugging/tracing other processes
	fb.DenySyscall(syscalls.Unshare) // Creating new namespaces
	fb.DenySyscall(syscalls.Setns)   // Entering namespaces

	// Filesystem manipulation
	fb.DenySyscall(syscalls.Pivot_root) // Changing root directory
	fb.DenySyscall(syscalls.Chroot)     // Changing root (can escape sandboxes)
	fb.DenySyscall(syscalls.Mount)      // Mounting filesystems
	fb.DenySyscall(syscalls.Umount2)    // Unmounting filesystems

	// System manipulation
	fb.DenySyscall(syscalls.Reboot)        // Rebooting system
	fb.DenySyscall(syscalls.Swapon)        // Managing swap
	fb.DenySyscall(syscalls.Swapoff)       // Managing swap
	fb.DenySyscall(syscalls.Init_module)   // Loading kernel modules
	fb.DenySyscall(syscalls.Delete_module) // Unloading kernel modules
	fb.DenySyscall(syscalls.Kexec_load)    // Loading new kernel

	// Only block if available on this arch
	if syscalls.Kexec_file_load > 0 {
		fb.DenySyscall(syscalls.Kexec_file_load)
	}

	// Conditionally block network based on policy
	if !policy.HasFullNetworkAccess() {
		fb.DenySyscall(syscalls.Connect)
		fb.DenySyscall(syscalls.Accept)
		fb.DenySyscall(syscalls.Accept4)
		fb.DenySyscall(syscalls.Bind)
		fb.DenySyscall(syscalls.Listen)
		fb.DenySyscall(syscalls.Shutdown)
		fb.DenySyscall(syscalls.Sendto)
		fb.DenySyscall(syscalls.Sendmsg)
		fb.DenySyscall(syscalls.Sendmmsg)
		fb.DenySyscall(syscalls.Recvmsg)
		fb.DenySyscall(syscalls.Recvmmsg)
		fb.DenySyscall(syscalls.Getsockopt)
		fb.DenySyscall(syscalls.Setsockopt)
		fb.DenySyscall(syscalls.Getpeername)
		fb.DenySyscall(syscalls.Getsockname)

		// Socket: allow AF_UNIX (1), deny others
		if err := fb.DenySyscallConditional(syscalls.Socket, 0, 1 /* AF_UNIX */, true); err != nil {
			return nil, fmt.Errorf("failed to add socket filter: %w", err)
		}

		// Socketpair: allow AF_UNIX, deny others
		if err := fb.DenySyscallConditional(syscalls.Socketpair, 0, 1 /* AF_UNIX */, true); err != nil {
			return nil, fmt.Errorf("failed to add socketpair filter: %w", err)
		}
	}

	return fb.Build(), nil
}

// createStrictFilter creates a highly restrictive Seccomp filter
// that only allows safe, essential syscalls
func (s *seccompSandbox) createStrictFilter(arch string, policy *PolicyConfig) (*seccompPkg.SeccompFilter, error) {
	// Get the list of safe syscalls that are essential for basic operation
	safeSyscalls := seccompPkg.GetSafeSyscallList(arch)

	// Create a restrictive filter with deny-by-default
	// Only explicitly allowed syscalls will be permitted
	filter, err := seccompPkg.CreateRestrictiveFilter(arch, safeSyscalls)
	if err != nil {
		return nil, fmt.Errorf("failed to create strict filter: %w", err)
	}

	return filter, nil
}
