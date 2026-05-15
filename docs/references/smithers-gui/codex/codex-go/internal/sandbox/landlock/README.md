# Landlock Sandbox for Go

This package provides Linux Landlock LSM (Linux Security Module) support for filesystem access control in Go.

## Overview

Landlock is a Linux kernel security module introduced in kernel 5.13 that allows unprivileged processes to restrict their own filesystem access. It works by creating rulesets that specify allowed operations on specific paths.

This implementation provides:
- Pure Go syscall wrappers (no CGo required)
- Support for Landlock ABI v1 (kernel 5.13+)
- Fluent API for building access rules
- Automatic kernel version detection
- Graceful fallback on older kernels

## Features

### Syscall Wrappers

The package implements the three core Landlock syscalls:
- `landlock_create_ruleset()` - Create a new ruleset
- `landlock_add_rule()` - Add path-based access rules
- `landlock_restrict_self()` - Apply ruleset to current process

### Access Rights

Supports all Landlock ABI v1 filesystem access rights:
- `AccessFSExecute` - Execute files
- `AccessFSReadFile` - Read files
- `AccessFSWriteFile` - Write files
- `AccessFSReadDir` - List directories
- `AccessFSRemoveDir` - Remove directories
- `AccessFSRemoveFile` - Remove files
- `AccessFSMakeChar` - Create character devices
- `AccessFSMakeDir` - Create directories
- `AccessFSMakeReg` - Create regular files
- `AccessFSMakeSock` - Create Unix sockets
- `AccessFSMakeFifo` - Create named pipes
- `AccessFSMakeBlock` - Create block devices
- `AccessFSMakeSym` - Create symbolic links

Predefined combinations:
- `AccessFSReadOnly` - Read, execute, and list (no modifications)
- `AccessFSReadWrite` - All operations allowed

### Rule Builders

Three ways to define filesystem access:

1. **Read-only access** - Files can be read and executed but not modified
2. **Read-write access** - Full access including create, modify, delete
3. **No access (deny)** - Explicit denial (default for paths without rules)

## Usage

### Basic Example

```go
import "github.com/evmts/codex/codex-go/internal/sandbox/landlock"

// Create a ruleset
ruleset := landlock.NewRuleset()

// Allow read-only access to system directories
ruleset.AddReadOnlyPath("/usr")
ruleset.AddReadOnlyPath("/lib")
ruleset.AddReadOnlyPath("/etc")

// Allow read-write access to workspace
ruleset.AddReadWritePath("/home/user/workspace")
ruleset.AddReadWritePath("/tmp")

// Apply restrictions to current process
if err := ruleset.Apply(); err != nil {
    log.Fatal(err)
}

// From this point on, the process (and all children) can only:
// - Read from /usr, /lib, /etc
// - Read and write to /home/user/workspace and /tmp
// - Cannot write anywhere else
```

### Default Policy

The package provides a default policy matching the Rust implementation:

```go
import "github.com/evmts/codex/codex-go/internal/sandbox/landlock"

// Apply default policy:
// - Read-only access to entire filesystem
// - Read-write access to /dev/null
// - Read-write access to specified paths
writableRoots := []string{"/tmp", "/home/user/workspace"}
if err := landlock.ApplyDefault(writableRoots); err != nil {
    log.Fatal(err)
}
```

### Graceful Degradation

For optional sandboxing that works on older kernels:

```go
// Returns nil if Landlock is not supported (kernel < 5.13)
// Returns error only if Landlock is supported but application failed
if err := ruleset.TryApply(); err != nil {
    log.Fatal(err)
}

// Or use the Try* variants
landlock.TryApplyDefault(writableRoots)
landlock.TryApplyReadOnly()
```

### Policy Builder API

For more complex configurations:

```go
policy := landlock.NewPolicy().
    AddReadOnly("/usr", "/lib", "/etc").
    AddReadWrite("/tmp", "/home/user/workspace").
    WithBestEffort(true).
    Apply()

if err != nil {
    log.Fatal(err)
}
```

### Advanced Rules

For fine-grained control:

```go
ruleset := landlock.NewRuleset()

// Custom access rights
ruleset.AddRule("/opt", landlock.AccessFSReadFile | landlock.AccessFSReadDir)

// Explicit deny (optional, absence of rule implies deny)
ruleset.AddDenyPath("/secret")

// Specify which operations to restrict
ruleset.WithHandledAccess(
    landlock.AccessFSReadFile |
    landlock.AccessFSWriteFile |
    landlock.AccessFSReadDir,
)

ruleset.Apply()
```

## Kernel Compatibility

### Version Detection

```go
// Check if Landlock is supported
if landlock.IsSupported() {
    fmt.Println("Landlock is available")
}

// Get ABI version (0 if not supported)
version := landlock.GetABIVersion()

// Get detailed info
info, _ := landlock.GetInfo()
fmt.Printf("Kernel: %s, Supported: %v, ABI: %d\n",
    info.KernelVersion, info.Supported, info.ABIVersion)
```

### Kernel Requirements

- **Minimum**: Linux 5.13 (Landlock ABI v1)
- **Recommended**: Linux 5.13+
- **Build**: Kernel must be compiled with `CONFIG_SECURITY_LANDLOCK=y`

### Fallback Behavior

On kernels < 5.13 or without Landlock support:
- `IsSupported()` returns `false`
- `Apply()` returns an error
- `TryApply()` returns `nil` (graceful degradation)

## Architecture

### File Structure

- `landlock.go` - Core syscall wrappers and low-level functions
- `rules.go` - Ruleset builder API and rule management
- `apply.go` - High-level policy application helpers
- `landlock_test.go` - Comprehensive test suite
- `landlock_stub.go` - Stub implementation for non-Linux systems

### Build Tags

All Linux-specific code uses `//go:build linux` build constraint. Non-Linux systems get stub implementations that:
- Return `false` for `IsSupported()`
- Return errors for `Apply()` functions
- Return `nil` for `TryApply()` functions (graceful degradation)

## Testing

### Running Tests

```bash
# Run all tests (Linux only)
go test -v ./internal/sandbox/landlock

# Run with race detection
go test -race ./internal/sandbox/landlock

# Run benchmarks
go test -bench=. ./internal/sandbox/landlock
```

### Test Coverage

The test suite includes:
- Kernel support detection
- ABI version detection
- Ruleset creation and configuration
- Access right validation
- Policy builder API
- Graceful degradation
- Benchmarks for performance

### Mock Testing

Note: Most tests don't actually apply Landlock restrictions (which would affect the test process). Instead, they verify:
- API correctness
- Structure validation
- Error handling
- Support detection

For integration testing that actually applies restrictions, see the Rust implementation's test suite.

## Implementation Notes

### Syscall Interface

This implementation uses raw syscalls via `syscall.Syscall6()` without CGo. Syscall numbers are:
- `SYS_LANDLOCK_CREATE_RULESET = 444`
- `SYS_LANDLOCK_ADD_RULE = 445`
- `SYS_LANDLOCK_RESTRICT_SELF = 446`

### PR_SET_NO_NEW_PRIVS

Before applying Landlock, the package automatically sets `PR_SET_NO_NEW_PRIVS` using `prctl()`. This:
- Prevents gaining additional privileges through `execve()`
- Is required by Landlock
- Cannot be reversed once set

### File Descriptors

The implementation:
- Opens paths with `O_PATH | O_CLOEXEC`
- Closes file descriptors after adding rules
- Closes ruleset fd after applying
- Restrictions persist even after closing fds

## Security Considerations

### Irreversible Restrictions

Landlock restrictions are **irreversible**:
- Once applied, cannot be removed
- Apply to the entire process and all children
- Persist across `fork()` and `execve()`

### Best Practices

1. **Apply early**: Apply restrictions before executing untrusted code
2. **Minimal access**: Grant only necessary filesystem access
3. **Test thoroughly**: Verify application works with restrictions
4. **Graceful degradation**: Use `TryApply()` for optional sandboxing
5. **Combine with other security**: Use with seccomp, capabilities, namespaces

### Limitations

- Only controls filesystem access (not network, IPC, etc.)
- Requires kernel 5.13+ for support
- Cannot restrict already-open file descriptors
- Path-based (not inode-based) - symlinks can bypass restrictions
- Does not prevent reading through `/proc/self/mem` or similar

## Comparison with Rust Implementation

This Go implementation mirrors the Rust codebase's Landlock support:

### Similarities
- Same default policy (read-only root, writable roots + /dev/null)
- Landlock ABI v1 support
- Graceful degradation on older kernels
- Path-based rule system

### Differences
- **Rust**: Uses `landlock` crate (higher-level API)
- **Go**: Direct syscall wrappers (lower-level, no dependencies)
- **Rust**: Combined with seccomp for network filtering
- **Go**: Landlock only (network filtering separate)

### API Comparison

**Rust**:
```rust
install_filesystem_landlock_rules_on_current_thread(vec![PathBuf::from("/tmp")])?;
```

**Go**:
```go
landlock.ApplyDefault([]string{"/tmp"})
```

## Future Enhancements

Potential improvements for future ABI versions:

1. **ABI v2 support** (Linux 5.19+)
   - File renaming restrictions
   - File truncation control

2. **ABI v3 support** (Linux 6.2+)
   - File truncation refinements

3. **Network restrictions**
   - TCP connect/bind restrictions (future ABI)

4. **Integration helpers**
   - Automatic detection of required paths
   - Policy templates for common scenarios
   - Better error messages with suggested fixes

## References

- [Linux Landlock Documentation](https://www.kernel.org/doc/html/latest/userspace-api/landlock.html)
- [Landlock man pages](https://man7.org/linux/man-pages/man7/landlock.7.html)
- [Landlock paper](https://landlock.io/)
- [Kernel source](https://github.com/torvalds/linux/tree/master/security/landlock)

## License

This implementation is part of the Codex project and follows the same license.
