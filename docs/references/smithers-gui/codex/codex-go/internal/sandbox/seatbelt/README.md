# Seatbelt Sandbox Package

This package provides macOS Seatbelt (Sandbox Profile Language) integration for sandboxing subprocess execution in the Codex Go implementation.

## Overview

Seatbelt is macOS's native sandboxing mechanism that uses the Sandbox Profile Language (SBPL) to define security policies. This package:

1. **Generates sandbox profiles** for different security policies
2. **Applies profiles** to processes using `sandbox_init()` via CGo
3. **Provides predefined profiles** for common use cases

## Architecture

The package consists of four main components:

### 1. Core Profile Generation (`seatbelt.go`)

- `GenerateProfile()` - Generates a Seatbelt profile from a `ProfileConfig`
- `GenerateProfileFromSandboxPolicy()` - Converts a `protocol.SandboxPolicy` to a Seatbelt profile
- `BasePolicyTemplate` - The base policy template inspired by Chrome's sandbox

### 2. Predefined Profiles (`profiles.go`)

Three main profiles are provided:

- **ReadOnlyProfile** - Allows read everywhere, denies all writes
- **WorkspaceWriteProfile** - Allows read everywhere, writes only in workspace (with `.git` protection)
- **DangerFullAccessProfile** - No restrictions (for CI/Docker)

### 3. CGo Integration (`apply.go`)

- `ApplyProfile()` - Applies a profile to the current process using `sandbox_init()`
- Convenience functions: `ApplyReadOnlyProfile()`, `ApplyWorkspaceWriteProfile()`, etc.
- Platform-specific implementation (darwin only)

### 4. Tests (`seatbelt_test.go`)

Comprehensive test coverage for profile generation and policy conversion.

## Sandbox Policies

### ReadOnly

Allows read access to the entire filesystem but denies all write operations.

```go
profile := seatbelt.ReadOnlyProfile()
```

### WorkspaceWrite

Allows read access everywhere and write access only in specified workspace directories. Automatically protects `.git` directories from writes.

```go
profile := seatbelt.WorkspaceWriteProfile("/path/to/workspace", networkAccess, excludeTmpDir, excludeSlashTmp)
```

Features:
- Automatically detects and protects `.git` directories
- Supports multiple writable roots
- Optional network access
- Optional temporary directory access (`/tmp`, `TMPDIR`)

### DangerFullAccess

Allows all operations with no restrictions. Use only in trusted environments like CI or Docker.

```go
profile := seatbelt.DangerFullAccessProfile()
```

## Profile Generation

Profiles are generated using the Sandbox Profile Language (SBPL). The base policy includes:

- Process execution and forking
- Basic system calls for process functionality
- IPC for Python multiprocessing
- User preference read access
- Sysctls for hardware information

Additional permissions are layered on top:

```go
config := &seatbelt.ProfileConfig{
    AllowFileRead:  true,
    AllowFileWrite: true,
    WritableRoots: []seatbelt.WritableRoot{
        {
            Root:             "/workspace",
            ReadOnlySubpaths: []string{"/workspace/.git"},
        },
    },
    AllowNetworkOutbound: true,
}

profile := seatbelt.GenerateProfile(config)
```

## CGo Integration

The package uses CGo to call macOS's `sandbox_init()` system call:

```go
// Apply a profile to the current process
err := seatbelt.ApplyProfile(profile)
if err != nil {
    log.Fatalf("Failed to apply sandbox: %v", err)
}
```

**Important Notes:**
- Sandbox application is **irreversible** for the current process
- All child processes inherit the sandbox policy
- The sandbox cannot be removed once applied

## Build Tags

The package uses Go build tags to restrict compilation to macOS:

- `seatbelt.go`, `profiles.go`, `apply.go`, `seatbelt_test.go` - `//go:build darwin`
- `apply_other.go` - `//go:build !darwin` (stub implementation returning errors)

## Testing

Run tests on macOS:

```bash
go test ./internal/sandbox/seatbelt/...
```

Tests cover:
- Profile generation for all policy types
- Protocol policy conversion
- `.git` directory protection
- Network access configuration
- Multi-root workspace support

## Implementation Notes

### Git Directory Protection

When a workspace root contains a `.git` directory, it's automatically marked as read-only to prevent accidental corruption:

```sbpl
(allow file-write*
  (require-all
    (subpath "/workspace")
    (require-not (subpath "/workspace/.git"))
  )
)
```

### Path Canonicalization

The Rust implementation uses path canonicalization to resolve symlinks (e.g., `/tmp` → `/private/tmp` on macOS). The Go implementation should do the same in production use.

### Comparison with Rust Implementation

This implementation mirrors the Rust implementation found in `codex-rs/core/src/seatbelt.rs`:

| Feature | Rust | Go |
|---------|------|-----|
| Base Policy | `seatbelt_base_policy.sbpl` | `BasePolicyTemplate` constant |
| Profile Generation | `create_seatbelt_command_args()` | `GenerateProfile()` |
| .git Protection | Automatic | Automatic |
| CGo Integration | Uses `sandbox-exec` binary | Uses `sandbox_init()` directly |
| Policy Types | ReadOnly, WorkspaceWrite, DangerFullAccess | Same |

### Key Difference: Execution Model

**Rust**: Uses `/usr/bin/sandbox-exec` to wrap command execution:
```rust
sandbox-exec -p "profile" -- command args...
```

**Go**: Applies sandbox directly to the current process using `sandbox_init()`:
```go
ApplyProfile(profile)
// All subsequent operations in this process are sandboxed
```

This difference means:
- Rust wraps each command individually
- Go applies the sandbox once to the process, affecting all operations

## Usage Example

```go
package main

import (
    "github.com/evmts/codex/codex-go/internal/sandbox/seatbelt"
    "github.com/evmts/codex/codex-go/internal/protocol"
)

func main() {
    // Check if seatbelt is supported
    if !seatbelt.IsSupported() {
        log.Fatal("Seatbelt not supported on this platform")
    }

    // Option 1: Use a predefined profile
    err := seatbelt.ApplyWorkspaceWriteProfile("/workspace", false, false, false)
    if err != nil {
        log.Fatalf("Failed to apply sandbox: %v", err)
    }

    // Option 2: Generate from protocol.SandboxPolicy
    policy := &protocol.SandboxPolicy{
        Mode:          "workspace-write",
        WritableRoots: []string{"/workspace"},
        NetworkAccess: false,
    }
    profile := seatbelt.GenerateProfileFromSandboxPolicy(policy, "/workspace")
    err = seatbelt.ApplyProfile(profile)

    // All operations after this point are sandboxed
    // ...
}
```

## Security Considerations

1. **Trust Boundary**: The sandbox provides defense-in-depth but is not a substitute for proper input validation
2. **Path Traversal**: Ensure writable roots don't allow escaping the intended workspace
3. **.git Protection**: Automatically enabled for git repositories to prevent corruption
4. **Network Access**: Disable unless explicitly required for the operation
5. **Temporary Directories**: Consider excluding `/tmp` and `TMPDIR` if not needed

## References

- [Apple Sandbox Guide](https://developer.apple.com/library/archive/documentation/Security/Conceptual/AppSandboxDesignGuide/)
- [Chromium Sandbox Policy](https://source.chromium.org/chromium/chromium/src/+/main:sandbox/policy/mac/common.sb)
- Rust implementation: `codex-rs/core/src/seatbelt.rs`
