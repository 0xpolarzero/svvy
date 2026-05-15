//go:build linux

/*
Package landlock provides Linux Landlock LSM (Linux Security Module) support for filesystem access control.

Landlock is a Linux kernel security module introduced in kernel 5.13 that allows unprivileged
processes to restrict their own filesystem access. This package provides pure Go bindings to
the Landlock syscalls without requiring CGo.

# Overview

Landlock works by creating rulesets that define allowed filesystem operations on specific paths.
Once a ruleset is applied to a process, the restrictions cannot be removed and apply to all
child processes.

# Basic Usage

Create a ruleset, add rules, and apply it:

	ruleset := landlock.NewRuleset()
	ruleset.AddReadOnlyPath("/usr")
	ruleset.AddReadOnlyPath("/lib")
	ruleset.AddReadWritePath("/tmp")
	if err := ruleset.Apply(); err != nil {
		log.Fatal(err)
	}

# Default Policy

Apply a sensible default policy:

	writableRoots := []string{"/tmp", "/home/user/workspace"}
	if err := landlock.ApplyDefault(writableRoots); err != nil {
		log.Fatal(err)
	}

This allows read-only access to the entire filesystem and read-write access to
specified paths plus /dev/null.

# Graceful Degradation

Use TryApply methods to support older kernels gracefully:

	// Returns nil if Landlock is not supported (kernel < 5.13)
	// Returns error only if Landlock is supported but application failed
	if err := ruleset.TryApply(); err != nil {
		log.Fatal(err)
	}

# Policy Builder API

For complex configurations, use the policy builder:

	policy := landlock.NewPolicy().
		AddReadOnly("/usr", "/lib").
		AddReadWrite("/tmp").
		WithBestEffort(true).
		Apply()

# Access Rights

The package supports all Landlock ABI v1 filesystem access rights:

  - AccessFSExecute - Execute files
  - AccessFSReadFile - Read files
  - AccessFSWriteFile - Write files
  - AccessFSReadDir - List directories
  - AccessFSRemoveDir - Remove directories
  - AccessFSRemoveFile - Remove files
  - AccessFSMakeChar - Create character devices
  - AccessFSMakeDir - Create directories
  - AccessFSMakeReg - Create regular files
  - AccessFSMakeSock - Create Unix sockets
  - AccessFSMakeFifo - Create named pipes
  - AccessFSMakeBlock - Create block devices
  - AccessFSMakeSym - Create symbolic links

Predefined combinations:
  - AccessFSReadOnly - Read, execute, and list (no modifications)
  - AccessFSReadWrite - All operations allowed

# Custom Rules

For fine-grained control:

	ruleset := landlock.NewRuleset()

	// Custom access rights
	ruleset.AddRule("/opt", landlock.AccessFSReadFile | landlock.AccessFSReadDir)

	// Explicit deny (optional, absence of rule implies deny)
	ruleset.AddDenyPath("/secret")

	ruleset.Apply()

# Kernel Compatibility

Check if Landlock is supported:

	if landlock.IsSupported() {
		fmt.Println("Landlock is available")
	}

Get detailed information:

	info, _ := landlock.GetInfo()
	fmt.Printf("Kernel: %s, ABI: %d\n", info.KernelVersion, info.ABIVersion)

Minimum kernel version: Linux 5.13 (Landlock ABI v1)

# Implementation Details

This package uses raw syscalls via syscall.Syscall6() without CGo:
  - SYS_LANDLOCK_CREATE_RULESET (444)
  - SYS_LANDLOCK_ADD_RULE (445)
  - SYS_LANDLOCK_RESTRICT_SELF (446)

Before applying restrictions, the package automatically sets PR_SET_NO_NEW_PRIVS
to prevent gaining additional privileges through execve().

# Security Considerations

  - Restrictions are IRREVERSIBLE once applied
  - Restrictions apply to the entire process and all children
  - Restrictions persist across fork() and execve()
  - Only controls filesystem access (not network, IPC, etc.)
  - Path-based restrictions (not inode-based)
  - Cannot restrict already-open file descriptors

# Platform Support

This package is only available on Linux systems with kernel 5.13 or later.
On non-Linux systems, stub implementations are provided that:
  - Return false for IsSupported()
  - Return errors for Apply() functions
  - Return nil for TryApply() functions (graceful degradation)

# References

  - Linux Landlock Documentation: https://www.kernel.org/doc/html/latest/userspace-api/landlock.html
  - Landlock man pages: https://man7.org/linux/man-pages/man7/landlock.7.html
  - Landlock project: https://landlock.io/
*/
package landlock
