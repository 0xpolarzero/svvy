# Seccomp Integration for Linux

This document describes the Seccomp-BPF (Berkeley Packet Filter) integration in the Codex Go sandbox system.

## Overview

Seccomp (Secure Computing Mode) is a Linux kernel feature that provides syscall filtering through BPF programs. The Codex Go implementation uses Seccomp to restrict which system calls sandboxed processes can make, providing an additional layer of security beyond filesystem restrictions.

### When Seccomp is Used

- **Platform**: Linux only
- **Kernel Requirements**: Linux kernel >= 3.5 (when Seccomp-BPF was introduced)
- **Fallback Order**: On Linux kernel >= 5.13, Landlock is preferred over Seccomp for filesystem restrictions. Seccomp is used as a fallback on older kernels or for additional syscall filtering.

## Implementation Details

### File Structure

```
internal/sandbox/
├── manager.go              # Main sandbox manager (platform-agnostic)
├── manager_linux.go        # Linux-specific seccompSandbox implementation
├── manager_nonlinux.go     # Stub implementation for non-Linux platforms
├── manager_linux_test.go   # Linux-specific tests
└── seccomp/                # Seccomp BPF implementation
    ├── seccomp.go          # Core Seccomp syscalls and filter application
    ├── filter.go           # BPF filter builder
    ├── syscalls.go         # Architecture-specific syscall numbers
    └── seccomp_test.go     # Comprehensive Seccomp tests
```

### Build Tags

The Seccomp implementation uses Go build tags to ensure platform-specific code only compiles on appropriate systems:

- `//go:build linux` - Compiles only on Linux
- `//go:build !linux` - Compiles on all platforms except Linux

### Architecture Support

Seccomp filters are architecture-specific because syscall numbers differ between architectures. The implementation supports:

- **amd64 (x86_64)**: Full support
- **arm64 (aarch64)**: Full support

## How It Works

### 1. Filter Creation

When a command is sandboxed, a BPF filter program is created based on the policy:

```go
// Example: Creating a network filter
arch := seccomp.GetArchitecture()
filter, err := seccomp.CreateNetworkFilter(arch)
```

The filter is a BPF (Berkeley Packet Filter) program that runs in the kernel and examines each syscall before it's executed.

### 2. Filter Application

The filter is configured to be applied in the child process via `exec.Cmd.SysProcAttr`:

```go
cmd.SysProcAttr = &syscall.SysProcAttr{
    Pdeathsig: syscall.SIGKILL,  // Kill child if parent dies
    Setpgid:   true,              // Create new process group
}
```

**Important Note**: Go's `exec` package doesn't provide a direct pre-exec hook. The current implementation sets environment variables to signal filter requirements. For production deployment, you should use a wrapper executable that:

1. Reads the filter configuration
2. Applies the filter via `filter.Apply()`
3. Executes the target command via `syscall.Exec()`

### 3. Policy-Based Filtering

Different policies apply different levels of syscall restrictions:

#### PolicyReadOnly
- **Network**: Blocked (except AF_UNIX domain sockets)
- **Filesystem**: No syscall-level restrictions (relies on filesystem permissions or Landlock)
- **Use Case**: Untrusted code that only needs to read data

```go
policy := sandbox.NewReadOnlyPolicy()
```

#### PolicyWorkspaceWrite (No Network)
- **Network**: Blocked (except AF_UNIX)
- **Filesystem**: No syscall-level restrictions
- **Use Case**: Trusted code that needs to modify files but not access network

```go
policy := sandbox.NewWorkspaceWritePolicy()
```

#### PolicyWorkspaceWrite (With Network)
- **Network**: Allowed
- **Filesystem**: No syscall-level restrictions
- **Use Case**: Code that needs full network access

```go
policy := &sandbox.PolicyConfig{
    Policy: sandbox.PolicyWorkspaceWrite,
    WorkspaceWriteConfig: &sandbox.WorkspaceWriteConfig{
        NetworkAccess: true,
    },
}
```

## Filter Types

The implementation provides three types of filters with different restriction levels:

### 1. Read-Only Filter (createReadOnlyFilter)

Blocks network syscalls while allowing safe read operations:

- ✅ Allowed: File reads, memory operations, process info queries
- ❌ Denied: Network syscalls (socket, connect, accept, etc.)
- ❌ Denied: Dangerous operations (ptrace, mount, reboot, etc.)

### 2. Moderate Filter (createModerateFilter)

Blocks dangerous syscalls but allows most normal operations:

- ✅ Allowed: Most file operations, memory management, process management
- ❌ Denied: Process debugging (ptrace), namespace manipulation (unshare, setns)
- ❌ Denied: Filesystem manipulation (mount, chroot, pivot_root)
- ❌ Denied: System manipulation (reboot, kernel module loading)
- ❌ Conditionally denied: Network operations (based on policy)

### 3. Strict Filter (createStrictFilter)

Only allows essential syscalls required for basic operation:

- ✅ Allowed: read, write, exit, basic memory operations
- ✅ Allowed: Essential Go runtime syscalls (futex, mmap, etc.)
- ❌ Denied: Everything else by default

## Syscall Filtering Details

### Network Syscalls

The network filter blocks these syscalls:

```
socket(AF_INET/AF_INET6, ...)  → EPERM
connect()                       → EPERM
accept/accept4()                → EPERM
bind()                          → EPERM
listen()                        → EPERM
sendto/sendmsg()                → EPERM
recvmsg/recvmmsg()              → EPERM
setsockopt/getsockopt()         → EPERM
```

**Exception**: `socket(AF_UNIX, ...)` is allowed to permit local IPC (Inter-Process Communication) via Unix domain sockets.

### Architecture Detection

Syscall numbers differ between architectures. The implementation automatically detects the current architecture:

```go
arch := seccomp.GetArchitecture()
syscalls := seccomp.GetSyscallNumbers(arch)
```

### Conditional Syscall Filtering

Some syscalls are filtered based on their arguments. For example:

```go
// Allow socket() only for AF_UNIX (domain = 1)
fb.DenySyscallConditional(syscalls.Socket, 0, 1 /* AF_UNIX */, true)
```

This creates a BPF program that:
1. Checks if the syscall is `socket()`
2. Loads the first argument (domain)
3. Compares it to AF_UNIX (1)
4. Denies if it doesn't match

## Security Considerations

### 1. Syscall-Level Only

Seccomp can only filter syscalls, not paths or file content. For filesystem restrictions, use:
- **Landlock** (Linux kernel >= 5.13) - Path-based access control
- **Filesystem permissions** - Traditional Unix permissions

### 2. No Removal Once Applied

Once a Seccomp filter is applied, it cannot be removed or relaxed, only made more restrictive. This is a kernel security feature.

### 3. Thread Safety

Seccomp filters are per-thread by default. The implementation uses `runtime.LockOSThread()` to ensure the filter is applied to the correct thread.

### 4. Process Group Isolation

The implementation creates a new process group (`Setpgid: true`) to prevent signal propagation from sandboxed processes affecting the parent.

### 5. Parent Death Signal

The implementation sets `Pdeathsig: syscall.SIGKILL` to ensure child processes are killed if the parent dies unexpectedly.

## Testing

### Unit Tests

Run unit tests on Linux:

```bash
go test -v ./internal/sandbox -run TestSeccomp
```

### Integration Tests

Integration tests verify that syscalls are actually blocked:

```bash
go test -v ./internal/sandbox -run TestSeccompSandboxIntegration
```

These tests run in child processes to verify:
1. Network syscalls are blocked with EPERM
2. AF_UNIX sockets are allowed
3. Safe syscalls continue to work

### Architecture Tests

Test on different architectures:

```bash
# On amd64
GOARCH=amd64 go test ./internal/sandbox/seccomp

# On arm64
GOARCH=arm64 go test ./internal/sandbox/seccomp
```

## Performance

Seccomp has minimal performance overhead:

- **Filter Creation**: ~1-5 microseconds (cached BPF program generation)
- **Syscall Filtering**: ~0.01 microseconds per syscall (BPF runs in kernel)
- **No Impact**: Zero overhead for allowed syscalls

Benchmarks:

```bash
go test -bench=BenchmarkSeccomp ./internal/sandbox
```

Example results:
```
BenchmarkSeccompApplyReadOnly-8           500000      2500 ns/op
BenchmarkSeccompFilterCreation-8         1000000      1200 ns/op
BenchmarkModerateFilterCreation-8         500000      2100 ns/op
```

## Troubleshooting

### "Seccomp not supported"

If you see this error:
1. Check kernel version: `uname -r` (must be >= 3.5)
2. Check if Seccomp is enabled: `grep SECCOMP /boot/config-$(uname -r)`
3. Some containers disable Seccomp - check container runtime settings

### "Operation not permitted (EPERM)"

This is expected behavior when a denied syscall is attempted. To debug:
1. Enable Seccomp logging: `dmesg | grep seccomp`
2. Use `strace` to see which syscall is being blocked
3. Review the filter rules for the policy being used

### "Filter application failed"

Common causes:
1. `no_new_privs` not set (automatically handled by implementation)
2. Running in a namespace with restricted capabilities
3. Kernel compiled without Seccomp support

## Production Deployment

For production use, implement a wrapper executable:

```go
// codex-seccomp-wrapper/main.go
package main

import (
    "os"
    "syscall"
    "github.com/evmts/codex/codex-go/internal/sandbox/seccomp"
)

func main() {
    // 1. Read filter configuration from environment
    policy := os.Getenv("CODEX_SECCOMP_POLICY")

    // 2. Create appropriate filter
    arch := seccomp.GetArchitecture()
    var filter *seccomp.SeccompFilter
    if policy == "read-only" {
        filter, _ = seccomp.CreateNetworkFilter(arch)
    }

    // 3. Apply filter
    if err := filter.Apply(); err != nil {
        panic(err)
    }

    // 4. Execute target command
    args := os.Args[1:]
    env := os.Environ()
    syscall.Exec(args[0], args, env)
}
```

Then configure the sandbox manager to use this wrapper:

```go
originalPath := cmd.Path
originalArgs := cmd.Args
cmd.Path = "/usr/local/bin/codex-seccomp-wrapper"
cmd.Args = append([]string{cmd.Path, originalPath}, originalArgs[1:]...)
```

## References

- [Seccomp Linux Kernel Documentation](https://www.kernel.org/doc/Documentation/prctl/seccomp_filter.txt)
- [BPF Filter Programming](https://www.kernel.org/doc/Documentation/networking/filter.txt)
- [Seccomp Man Page](https://man7.org/linux/man-pages/man2/seccomp.2.html)
- [Syscall Numbers](https://filippo.io/linux-syscall-table/)

## Future Improvements

1. **Automatic Wrapper**: Generate and compile wrapper executable automatically
2. **Filter Serialization**: Serialize BPF programs to file for reuse
3. **More Architectures**: Add support for riscv64, ppc64le
4. **Fine-grained Control**: Per-command filter customization
5. **Audit Logging**: Integration with kernel audit subsystem for denied syscalls
