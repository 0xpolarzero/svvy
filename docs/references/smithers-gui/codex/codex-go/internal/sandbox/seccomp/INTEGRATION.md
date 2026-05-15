# Seccomp Integration Guide

This document describes how to integrate Seccomp-BPF with the Codex sandbox system, including fallback strategies for different kernel versions.

## Fallback Strategy

The sandbox system should prefer newer, more capable isolation mechanisms:

1. **Landlock (Linux 5.13+)** - Path-based filesystem access control
2. **Seccomp (Linux 3.5+)** - Syscall filtering for network isolation
3. **No isolation** - Fallback on very old kernels

## Integration Pattern

### Example: Network Isolation

```go
package sandbox

import (
    "fmt"
    "github.com/evmts/codex/codex-go/internal/sandbox/seccomp"
)

// ApplyNetworkIsolation applies network restrictions using the best available method
func ApplyNetworkIsolation() error {
    // Check if Seccomp is supported
    if !seccomp.IsSupported() {
        return fmt.Errorf("seccomp not supported on this kernel")
    }

    // Get current architecture
    arch := seccomp.GetArchitecture()

    // Create network filter
    filter, err := seccomp.CreateNetworkFilter(arch)
    if err != nil {
        return fmt.Errorf("failed to create network filter: %w", err)
    }

    // Apply the filter
    if err := filter.Apply(); err != nil {
        return fmt.Errorf("failed to apply seccomp filter: %w", err)
    }

    return nil
}
```

### Example: Combined Landlock + Seccomp

```go
// ApplySandboxPolicy applies both filesystem and network restrictions
func ApplySandboxPolicy(allowWrite []string, allowNetwork bool) error {
    // 1. Apply Landlock for filesystem isolation (if available)
    if landlockSupported() {
        if err := applyLandlockRules(allowWrite); err != nil {
            return fmt.Errorf("landlock failed: %w", err)
        }
    }

    // 2. Apply Seccomp for network isolation (if needed)
    if !allowNetwork && seccomp.IsSupported() {
        arch := seccomp.GetArchitecture()
        filter, err := seccomp.CreateNetworkFilter(arch)
        if err != nil {
            return fmt.Errorf("seccomp filter creation failed: %w", err)
        }

        if err := filter.Apply(); err != nil {
            return fmt.Errorf("seccomp apply failed: %w", err)
        }
    }

    return nil
}
```

### Example: Restrictive Read-Only Mode

```go
// ApplyReadOnlyMode applies maximum restrictions for read-only access
func ApplyReadOnlyMode() error {
    if !seccomp.IsSupported() {
        return fmt.Errorf("seccomp not supported")
    }

    arch := seccomp.GetArchitecture()
    safeSyscalls := seccomp.GetSafeSyscallList(arch)

    // Create restrictive filter (deny-by-default)
    filter, err := seccomp.CreateRestrictiveFilter(arch, safeSyscalls)
    if err != nil {
        return fmt.Errorf("failed to create restrictive filter: %w", err)
    }

    if err := filter.Apply(); err != nil {
        return fmt.Errorf("failed to apply restrictive filter: %w", err)
    }

    return nil
}
```

## Sandbox Modes

### 1. Read-Only Mode

```go
// Landlock: Allow read everywhere, no writes
// Seccomp: Allow only safe syscalls (read, stat, etc.)

landlockRules := LandlockRules{
    ReadPaths:  []string{"/"},
    WritePaths: []string{}, // No writes allowed
}

seccompFilter, _ := seccomp.CreateRestrictiveFilter(
    arch,
    seccomp.GetSafeSyscallList(arch),
)
```

### 2. Workspace-Write Mode

```go
// Landlock: Allow read everywhere, write to workspace only
// Seccomp: Block network, allow file operations

landlockRules := LandlockRules{
    ReadPaths:  []string{"/"},
    WritePaths: []string{"/home/user/workspace", "/tmp"},
}

seccompFilter, _ := seccomp.CreateNetworkFilter(arch)
```

### 3. Full-Auto Mode

```go
// Landlock: Allow read everywhere, write to workspace + /tmp
// Seccomp: Optional network filter (configurable)

landlockRules := LandlockRules{
    ReadPaths:  []string{"/"},
    WritePaths: []string{"/home/user/workspace", "/tmp"},
}

if !config.AllowNetwork {
    seccompFilter, _ := seccomp.CreateNetworkFilter(arch)
    seccompFilter.Apply()
}
```

## Detection and Capability Checking

```go
package sandbox

import (
    "github.com/evmts/codex/codex-go/internal/sandbox/seccomp"
)

// SandboxCapabilities represents available sandboxing features
type SandboxCapabilities struct {
    HasLandlock bool
    HasSeccomp  bool
    KernelVersion string
}

// DetectCapabilities determines which sandbox features are available
func DetectCapabilities() (*SandboxCapabilities, error) {
    caps := &SandboxCapabilities{
        HasSeccomp: seccomp.IsSupported(),
    }

    // Check Landlock availability (pseudo-code)
    caps.HasLandlock = checkLandlockSupport()

    // Get kernel version for logging
    caps.KernelVersion = getKernelVersion()

    return caps, nil
}

// SelectSandboxStrategy chooses the best sandboxing approach
func SelectSandboxStrategy(caps *SandboxCapabilities) string {
    if caps.HasLandlock && caps.HasSeccomp {
        return "landlock+seccomp" // Best: combined isolation
    } else if caps.HasLandlock {
        return "landlock-only" // Filesystem only
    } else if caps.HasSeccomp {
        return "seccomp-only" // Network only
    } else {
        return "none" // No kernel-level isolation
    }
}
```

## Error Handling

### Seccomp Errors

```go
import "syscall"

// Handle seccomp application errors
if err := filter.Apply(); err != nil {
    // Check for specific errors
    if errors.Is(err, syscall.EINVAL) {
        return fmt.Errorf("seccomp not supported or invalid filter")
    } else if errors.Is(err, syscall.EFAULT) {
        return fmt.Errorf("seccomp filter pointer invalid")
    } else if errors.Is(err, syscall.EACCES) {
        return fmt.Errorf("seccomp permission denied (need no_new_privs)")
    }
    return fmt.Errorf("seccomp apply failed: %w", err)
}
```

### Filter Verification

```go
// Verify filter is active after application
mode, err := seccomp.GetMode()
if err != nil {
    return fmt.Errorf("failed to verify seccomp mode: %w", err)
}

if mode != seccomp.SECCOMP_MODE_FILTER {
    return fmt.Errorf("seccomp filter not active (mode=%d)", mode)
}
```

## Testing Strategy

### Unit Tests

```go
// Test seccomp availability
func TestSeccompSupport(t *testing.T) {
    if !seccomp.IsSupported() {
        t.Skip("Seccomp not supported on this kernel")
    }
    // ... test logic
}
```

### Integration Tests

```go
// Test sandbox in a subprocess
func TestSandboxedExecution(t *testing.T) {
    cmd := exec.Command("./test-binary")

    // Apply sandbox before exec
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setpgid: true,
    }

    // Custom pre-exec hook to apply seccomp
    cmd.Env = append(cmd.Env, "APPLY_SECCOMP=1")

    output, err := cmd.CombinedOutput()
    // Verify sandboxed behavior
}
```

## Performance Considerations

### Overhead

- **Landlock**: ~100-500 ns per filesystem operation
- **Seccomp**: ~100-200 ns per syscall
- **Combined**: Overhead is additive but still negligible

### Optimization Tips

1. **Apply filters once** - Don't recreate filters for each operation
2. **Use specific rules** - More specific rules = fewer BPF instructions
3. **Minimize filter size** - Consolidate similar rules
4. **Cache filter objects** - Reuse `SeccompFilter` instances

```go
// Good: Cache and reuse filters
var (
    networkFilter *seccomp.SeccompFilter
    filterOnce    sync.Once
)

func getNetworkFilter() (*seccomp.SeccompFilter, error) {
    var err error
    filterOnce.Do(func() {
        arch := seccomp.GetArchitecture()
        networkFilter, err = seccomp.CreateNetworkFilter(arch)
    })
    return networkFilter, err
}
```

## Debugging

### Check Current Seccomp Mode

```go
mode, err := seccomp.GetMode()
if err != nil {
    log.Printf("Failed to get seccomp mode: %v", err)
} else {
    switch mode {
    case seccomp.SECCOMP_MODE_DISABLED:
        log.Println("Seccomp: disabled")
    case seccomp.SECCOMP_MODE_STRICT:
        log.Println("Seccomp: strict mode")
    case seccomp.SECCOMP_MODE_FILTER:
        log.Println("Seccomp: filter mode active")
    }
}
```

### Log Denied Syscalls

```go
// Use strace to debug seccomp denials
// $ strace -f ./codex-go 2>&1 | grep EPERM
```

### Verify Filter Behavior

```go
// Test specific syscall after applying filter
func verifyFilterWorks() error {
    // Try a blocked syscall (should fail)
    _, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
    if err != syscall.EPERM {
        return fmt.Errorf("filter not blocking network: got %v", err)
    }

    // Try an allowed syscall (should succeed)
    _, err = syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
    if err != nil {
        return fmt.Errorf("filter blocking AF_UNIX: %v", err)
    }

    return nil
}
```

## Migration from Rust Implementation

The Go implementation matches the Rust seccomp filter semantics:

| Rust (seccompiler) | Go (internal/sandbox/seccomp) |
|-------------------|------------------------------|
| `SeccompFilter::new()` | `NewFilterBuilder()` |
| `SeccompAction::Allow` | `ActionAllow` |
| `SeccompAction::Errno(EPERM)` | `ActionErrno` |
| `SeccompRule::new()` | `DenySyscallConditional()` |
| `apply_filter()` | `filter.Apply()` |

### Rust Code

```rust
let filter = SeccompFilter::new(
    rules,
    SeccompAction::Allow,
    SeccompAction::Errno(libc::EPERM as u32),
    TargetArch::x86_64,
)?;
apply_filter(&prog)?;
```

### Equivalent Go Code

```go
fb, _ := seccomp.NewFilterBuilder("amd64", seccomp.ActionAllow)
fb.DenySyscall(syscalls.Socket)
filter := fb.Build()
filter.Apply()
```

## Security Considerations

1. **Irreversible** - Seccomp filters cannot be removed, only made stricter
2. **Apply early** - Apply filters before loading untrusted code
3. **Test thoroughly** - Ensure critical syscalls are allowed (futex, rt_sigreturn, etc.)
4. **Combine defenses** - Use Landlock + Seccomp + AppArmor/SELinux when possible
5. **Monitor logs** - Watch for unexpected EPERM errors indicating missing syscalls

## References

- [Landlock Documentation](https://www.kernel.org/doc/html/latest/userspace-api/landlock.html)
- [Seccomp Documentation](https://www.kernel.org/doc/Documentation/prctl/seccomp_filter.txt)
- [Codex Rust Implementation](../../codex-rs/linux-sandbox/src/landlock.rs)
- [Codex Sandbox Docs](../../../docs/sandbox.md)
