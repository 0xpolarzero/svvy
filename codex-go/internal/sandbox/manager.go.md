# Code Review: manager.go

**File:** `/Users/williamcory/codex/codex-go/internal/sandbox/manager.go`
**Review Date:** 2025-10-26
**Reviewer:** Claude Code
**Severity Scale:** Critical > High > Medium > Low > Info

---

## Executive Summary

The `manager.go` file implements a sandbox manager that orchestrates OS-specific sandboxing mechanisms. While the architecture and design are sound, **the actual sandbox enforcement is not implemented** - all three sandbox types (Seatbelt, Landlock, Seccomp) currently only set environment variables as placeholders. This creates a **critical security gap** where the system appears to apply sandboxing but provides no actual isolation.

**Critical Finding:** Despite separate implementation packages existing (`/internal/sandbox/seatbelt/`, `/internal/sandbox/landlock/`, `/internal/sandbox/seccomp/`), they are not integrated into `manager.go`, leaving the sandbox enforcement non-functional.

---

## 1. Incomplete Features and Functionality

### 1.1 Seatbelt Implementation (macOS) - CRITICAL
**Lines:** 144-166
**Severity:** Critical
**Status:** Incomplete

```go
func (s *seatbeltSandbox) Apply(cmd *exec.Cmd, policy *PolicyConfig, workspace string) error {
    // On macOS, we would wrap the command with sandbox-exec
    // For now, this is a placeholder that sets environment variables
    // to indicate sandboxing is enabled.

    // TODO: Implement full Seatbelt profile generation and sandbox-exec wrapping
    // This requires generating a sandbox profile similar to Rust implementation
```

**Issues:**
- Only sets environment variables (`CODEX_SANDBOX=seatbelt`)
- Does not generate Seatbelt profiles
- Does not wrap commands with `sandbox-exec`
- No actual filesystem or network restrictions enforced
- The separate `/internal/sandbox/seatbelt/` package contains profile generation code that is not used here

**Impact:** macOS users have zero sandbox protection despite the system claiming sandboxing is applied.

**Recommendation:** Integrate the existing `seatbelt` package implementation:
- Import `github.com/evmts/codex/codex-go/internal/sandbox/seatbelt`
- Use `seatbelt.GenerateProfile()` to create profiles
- Wrap commands with `/usr/bin/sandbox-exec`

---

### 1.2 Landlock Implementation (Linux) - CRITICAL
**Lines:** 193-212
**Severity:** Critical
**Status:** Incomplete

```go
func (l *landlockSandbox) Apply(cmd *exec.Cmd, policy *PolicyConfig, workspace string) error {
    // Landlock implementation would use the landlock syscalls
    // This is a placeholder that sets environment variables

    // TODO: Implement full Landlock support using landlock syscalls
    // This requires using the landlock_create_ruleset, landlock_add_rule,
    // and landlock_restrict_self syscalls similar to Rust implementation
```

**Issues:**
- Only sets environment variables
- Does not create Landlock rulesets
- Does not add path-based rules
- Does not apply restrictions via `landlock_restrict_self`
- The `/internal/sandbox/landlock/` package has full syscall implementation that is not integrated

**Impact:** Linux users on kernel 5.13+ have no filesystem access control.

**Recommendation:** Integrate the existing `landlock` package:
- Import the landlock package
- Use `landlock.NewRuleset()` and `ruleset.AddReadOnlyPath()/AddReadWritePath()`
- Apply the ruleset before command execution

---

### 1.3 Seccomp Implementation (Linux) - CRITICAL
**Lines:** 228-247
**Severity:** Critical
**Status:** Incomplete

```go
func (s *seccompSandbox) Apply(cmd *exec.Cmd, policy *PolicyConfig, workspace string) error {
    // Seccomp implementation would use seccomp-bpf filters
    // This is a placeholder that sets environment variables

    // TODO: Implement full Seccomp-BPF support
    // This requires setting up seccomp filters to restrict syscalls
    // similar to the Rust implementation in codex-rs/linux-sandbox
```

**Issues:**
- Only sets environment variables
- Does not create BPF filters
- Does not restrict syscalls
- The `/internal/sandbox/seccomp/` package has BPF filter implementation that is not integrated

**Impact:** Linux users on older kernels have no syscall filtering protection.

---

### 1.4 Windows Support - HIGH
**Lines:** 55-58
**Severity:** High
**Status:** Not Implemented

```go
case "windows":
    // Windows: No sandbox implementation yet
    sm.logger.Println("WARNING: Sandbox not supported on Windows - commands run with full system access")
    sm.appliers = []SandboxApplier{}
```

**Issues:**
- No Windows sandboxing support
- Warning is logged but may be overlooked
- Users on Windows have completely unrestricted access

**Impact:** Windows users have zero isolation.

**Recommendation:**
- Consider implementing Windows Job Objects or AppContainer sandboxing
- At minimum, document this limitation prominently in user-facing documentation
- Consider refusing to run in restricted policy modes on Windows

---

## 2. Technical Debt and TODO Comments

### 2.1 All TODOs
**Lines:** 162-163, 207-209, 242-244
**Count:** 3 major TODOs

All three TODOs represent critical missing functionality, not minor improvements. These should be tracked as high-priority issues.

---

## 3. Code Quality Issues

### 3.1 Misleading Function Behavior - CRITICAL
**Severity:** Critical

The `ApplyToCommand` function returns `SandboxInfo` with `Applied: true` even though no actual sandboxing occurs:

```go
return &SandboxInfo{
    Type:    sandboxTypeFromName(applier.Name()),
    Applied: true,  // ← MISLEADING: No sandbox was actually applied
    Reason:  fmt.Sprintf("using %s sandbox", applier.Name()),
}, nil
```

**Issues:**
- Violates principle of least surprise
- Callers believe sandboxing is active when it isn't
- Creates a false sense of security
- No way for callers to detect placeholder implementation

**Recommendation:**
- Add a field to `SandboxInfo` like `Enforced bool` to distinguish between "applied" (set up) and "enforced" (actually restricting)
- Or update documentation to clarify what "Applied" means
- Consider returning an error until real implementation is complete

---

### 3.2 Unused workspace Parameter - MEDIUM
**Severity:** Medium

The `workspace` parameter in all `Apply()` methods is currently unused:

```go
func (s *seatbeltSandbox) Apply(cmd *exec.Cmd, policy *PolicyConfig, workspace string) error {
    // workspace parameter is never used
```

**Issues:**
- The workspace path is critical for defining writable roots in WorkspaceWrite policy
- Currently no enforcement of workspace boundaries
- When implementation is completed, this will be essential

**Recommendation:**
- Add a comment explaining it will be used in the full implementation
- Or implement basic validation that workspace exists and is absolute

---

### 3.3 Policy Parameter Partially Ignored - MEDIUM
**Severity:** Medium
**Lines:** 144-166, 193-212, 228-247

The `policy` parameter is only used to check network access:

```go
if !policy.HasFullNetworkAccess() {
    cmd.Env = append(cmd.Env, "CODEX_SANDBOX_NETWORK_DISABLED=1")
}
```

**Issues:**
- `policy.GetWritableRoots(workspace)` is never called
- `policy.GetReadOnlyRoots()` is never called
- No differentiation between ReadOnly and WorkspaceWrite policies
- Policy configuration is essentially ignored

**Impact:** All sandbox policies behave identically (no enforcement).

---

### 3.4 Environment Variable Approach - LOW
**Severity:** Low

The current approach of setting environment variables is inherently weak:

```go
cmd.Env = append(cmd.Env, "CODEX_SANDBOX=seatbelt")
cmd.Env = append(cmd.Env, "CODEX_SANDBOX_NETWORK_DISABLED=1")
```

**Issues:**
- Relies on commands voluntarily checking environment variables
- Malicious or buggy code can ignore these variables
- No kernel-level enforcement
- False sense of security

**Note:** This is acceptable as a temporary placeholder during development, but should not ship in production.

---

### 3.5 Kernel Version Parsing Fragility - LOW
**Severity:** Low
**Lines:** 274-303

```go
func parseKernelVersion(version string) (*kernelVersion, error) {
    parts := strings.Split(version, ".")
    if len(parts) < 2 {
        return nil, fmt.Errorf("invalid kernel version format: %s", version)
    }
    // ... parsing logic
    patch, _ = strconv.Atoi(patchStr)  // ← Silently ignores errors
}
```

**Issues:**
- Error from `strconv.Atoi` for patch version is silently ignored
- Assumes specific version string formats
- May fail on custom kernel builds with unusual version strings

**Recommendation:**
- Log warning when patch parsing fails
- Add more robust parsing with regex
- Add test cases for various kernel version formats

---

## 4. Missing Test Coverage

### 4.1 Integration Testing - HIGH
**Severity:** High

While `manager_test.go` has extensive unit tests, there's no integration testing that validates:
- Commands actually fail when accessing restricted paths
- Network restrictions actually block connections
- Filesystem writes are actually blocked in read-only mode

**Current Test Limitation:**
```go
func TestSandboxManagerIntegration(t *testing.T) {
    // ... applies sandbox
    output, err := cmd.Output()
    if err != nil {
        t.Fatalf("Failed to run sandboxed command: %v", err)
    }
    // Only checks command runs, not that restrictions work
}
```

**Recommendation:**
- Add tests that verify restricted operations actually fail
- Test file writes to forbidden directories
- Test network access when disabled
- Test that sandboxed processes cannot escape restrictions

---

### 4.2 Error Path Testing - MEDIUM
**Severity:** Medium

No tests for error conditions:
- What happens when `sandbox-exec` is not found?
- What happens when kernel doesn't support Landlock despite version check passing?
- What happens when Seccomp application fails?

**Recommendation:**
- Add negative test cases
- Mock syscall failures
- Test fallback behavior

---

### 4.3 Concurrency Testing - LOW
**Severity:** Low

No tests for concurrent sandbox application:
- Multiple goroutines calling `ApplyToCommand`
- Thread safety of `SandboxManager`

**Note:** While `SandboxManager` appears stateless after initialization, this should be verified with race detector tests.

---

## 5. Potential Bugs and Edge Cases

### 5.1 Race Condition in Environment Variables - MEDIUM
**Severity:** Medium
**Lines:** 152-160

```go
if cmd.Env == nil {
    cmd.Env = os.Environ()
}
cmd.Env = append(cmd.Env, "CODEX_SANDBOX=seatbelt")
```

**Issues:**
- If `cmd.Env` is shared between goroutines, appending creates a race condition
- `append` may allocate a new slice, making the original reference stale
- Not thread-safe if the same `exec.Cmd` is passed to multiple `Apply` calls

**Likelihood:** Low (typically each command is unique), but possible.

**Recommendation:**
- Document that `exec.Cmd` should not be shared between goroutines
- Consider making a defensive copy of the environment

---

### 5.2 Landlock Version Check May Give False Positives - MEDIUM
**Severity:** Medium
**Lines:** 178-191

```go
func (l *landlockSandbox) IsAvailable() bool {
    if runtime.GOOS != "linux" {
        return false
    }

    version, err := getLinuxKernelVersion()
    if err != nil {
        return false
    }

    // Landlock was introduced in kernel 5.13
    return version.Major > 5 || (version.Major == 5 && version.Minor >= 13)
}
```

**Issues:**
- Kernel version check doesn't guarantee Landlock is compiled into the kernel
- Some distributions disable Landlock via kernel config
- Should attempt to actually call Landlock syscall to verify support

**Impact:** System may report Landlock as available when it actually fails at runtime.

**Recommendation:**
- The `/internal/sandbox/landlock/` package has an `IsSupported()` function that tests actual syscall availability
- Use that instead of version checking
- Import: `landlockPkg "github.com/evmts/codex/codex-go/internal/sandbox/landlock"` and call `landlockPkg.IsSupported()`

---

### 5.3 getLinuxKernelVersion Only Works on Linux - LOW
**Severity:** Low
**Lines:** 259-272

```go
func getLinuxKernelVersion() (*kernelVersion, error) {
    if runtime.GOOS != "linux" {
        return nil, fmt.Errorf("not running on Linux")
    }

    cmd := exec.Command("uname", "-r")
    output, err := cmd.Output()
    // ...
}
```

**Issues:**
- Called by `landlockSandbox.IsAvailable()` which already checks OS
- Redundant OS check
- `uname` command may not be in PATH in some minimal container environments

**Recommendation:**
- Remove redundant OS check (caller already verified)
- Consider using `syscall.Uname()` instead of executing external command
- Add fallback to `/proc/version` parsing

---

### 5.4 Missing Error Handling in Environment Setup - LOW
**Severity:** Low

When setting up environment variables, there's no validation that:
- Environment variables were successfully added
- Command's environment slice has sufficient capacity
- Duplicate environment variables aren't created

**Impact:** Minimal, but could lead to inconsistent behavior.

---

## 6. Documentation Issues

### 6.1 Missing Package Documentation - MEDIUM
**Severity:** Medium

The package has no package-level comment explaining:
- Overall architecture
- Relationship to separate implementation packages
- Why placeholder implementation exists
- Migration path to full implementation

**Recommendation:**
Add package documentation:
```go
// Package sandbox provides a cross-platform sandbox manager that orchestrates
// OS-specific sandboxing mechanisms (Seatbelt on macOS, Landlock/Seccomp on Linux).
//
// Current Status: The sandbox implementations are placeholders that set environment
// variables but do not enforce actual restrictions. Full implementations exist in
// subpackages (seatbelt/, landlock/, seccomp/) but are not yet integrated.
//
// Architecture:
//   - SandboxManager: Selects and applies appropriate sandbox for the OS
//   - SandboxApplier: Interface for OS-specific implementations
//   - PolicyConfig: Defines access control policies (ReadOnly, WorkspaceWrite, FullAccess)
```

---

### 6.2 Misleading Function Comments - HIGH
**Severity:** High

Function comments claim functionality that doesn't exist:

```go
// ApplyToCommand applies the appropriate sandbox to the given command based on the policy.
```

This suggests the sandbox is actually enforced, which is false.

**Recommendation:**
```go
// ApplyToCommand configures the command with sandbox environment variables based on the policy.
// Note: This is currently a placeholder implementation that does not enforce restrictions.
// TODO: Integrate actual sandbox implementations from seatbelt/, landlock/, seccomp/ packages.
```

---

### 6.3 Missing Policy Documentation - MEDIUM
**Severity:** Medium

The file doesn't explain:
- What each policy type does (or should do)
- Security implications of each policy
- When to use which policy
- Performance characteristics

**Recommendation:**
Add detailed comments to policy-related code or create a POLICIES.md document.

---

### 6.4 Environment Variable Constants Documentation - LOW
**Severity:** Low
**Lines:** 305-309

```go
// SetNetworkDisabled is a helper to set the network disabled environment variable
// This can be checked by commands to voluntarily disable network access
const EnvNetworkDisabled = "CODEX_SANDBOX_NETWORK_DISABLED"
const EnvSandboxType = "CODEX_SANDBOX"
```

**Issues:**
- `SetNetworkDisabled` function mentioned in comment doesn't exist
- "voluntarily disable" is concerning from a security perspective
- No documentation of expected values

**Recommendation:**
```go
// EnvNetworkDisabled is set to "1" when network access should be restricted.
// Note: This is informational only - commands can ignore this variable.
const EnvNetworkDisabled = "CODEX_SANDBOX_NETWORK_DISABLED"

// EnvSandboxType indicates which sandbox type was configured (e.g., "seatbelt", "landlock").
// Note: Presence of this variable does not guarantee enforcement.
const EnvSandboxType = "CODEX_SANDBOX"
```

---

## 7. Security Concerns

### 7.1 No Actual Sandbox Enforcement - CRITICAL
**Severity:** Critical
**CVSS Estimate:** 9.1 (Critical)

**Vulnerability:** The system claims to apply sandboxing but provides zero security isolation.

**Attack Scenario:**
1. User configures `PolicyReadOnly` expecting filesystem protection
2. System logs "using seatbelt sandbox" and returns `Applied: true`
3. Malicious code runs with full filesystem access
4. User's trust in sandbox leads to running untrusted code
5. System compromise

**Impact:**
- Complete bypass of intended security controls
- False sense of security may lead to dangerous behavior
- Violates user expectations and trust

**Recommendation:**
- **Do not release this to production without implementing actual enforcement**
- Add prominent warnings in logs and documentation
- Consider failing-closed (refusing to run) instead of failing-open
- Add build tags or feature flags to disable placeholder behavior

---

### 7.2 Environment Variables Provide No Security - CRITICAL
**Severity:** Critical

```go
cmd.Env = append(cmd.Env, "CODEX_SANDBOX_NETWORK_DISABLED=1")
```

**Issues:**
- Any process can ignore environment variables
- No kernel-level enforcement
- Trivially bypassed by `unsetenv()` or simply ignoring the variable
- Child processes may not inherit or may modify these variables

**This is not a sandbox - it's a suggestion.**

---

### 7.3 Workspace Parameter Not Validated - HIGH
**Severity:** High

```go
func (sm *SandboxManager) ApplyToCommand(cmd *exec.Cmd, policy *PolicyConfig, workspace string) (*SandboxInfo, error) {
```

**Issues:**
- `workspace` parameter is not validated
- Could be empty string, relative path, or non-existent path
- Could point outside intended boundaries (path traversal)
- When implementation is completed, this could allow sandbox escape

**Recommendation:**
```go
// Validate workspace before using it
if workspace == "" {
    return nil, fmt.Errorf("workspace cannot be empty")
}
workspace, err := filepath.Abs(workspace)
if err != nil {
    return nil, fmt.Errorf("invalid workspace path: %w", err)
}
if _, err := os.Stat(workspace); err != nil {
    return nil, fmt.Errorf("workspace does not exist: %w", err)
}
```

---

### 7.4 Policy Bypass via PolicyDangerFullAccess - MEDIUM
**Severity:** Medium

```go
// If policy doesn't require sandboxing, skip it
if !policy.ShouldSandbox() {
    return &SandboxInfo{
        Type:    SandboxTypeNone,
        Applied: false,
        Reason:  "policy allows full access",
    }, nil
}
```

**Issues:**
- No audit logging when full access is granted
- Easy to accidentally use wrong policy
- No confirmation or additional checks

**Recommendation:**
- Add warning log when `PolicyDangerFullAccess` is used
- Require explicit opt-in (e.g., environment variable confirmation)
- Log to audit trail for security reviews

---

### 7.5 No Logging of Applied Restrictions - MEDIUM
**Severity:** Medium

When a sandbox is applied, there's no detailed logging of:
- Which paths are writable
- Which paths are read-only
- Network access status
- Policy specifics

**Impact:** Debugging and security audits are difficult.

**Recommendation:**
- Add debug-level logging of complete sandbox configuration
- Log successful sandbox application at info level
- Include policy details in SandboxInfo

---

## 8. Performance Considerations

### 8.1 os.Environ() Called on Every Apply - LOW
**Severity:** Low
**Lines:** 153

```go
if cmd.Env == nil {
    cmd.Env = os.Environ()
}
```

**Issues:**
- `os.Environ()` copies all environment variables
- Called for every command when `cmd.Env` is nil
- May be hundreds of environment variables

**Impact:** Minor performance overhead, negligible in most cases.

**Recommendation:**
- Consider caching environment if applying to many commands
- Document that callers should pre-set `cmd.Env` if performance-sensitive

---

### 8.2 Kernel Version Parsing on Every Availability Check - LOW
**Severity:** Low

`getLinuxKernelVersion()` executes `uname -r` every time `landlockSandbox.IsAvailable()` is called.

**Impact:** Minimal, but wasteful.

**Recommendation:**
- Cache kernel version in `landlockSandbox` struct
- Or perform check once in `registerDefaultAppliers`

---

## 9. Architectural Concerns

### 9.1 Disconnect Between Manager and Implementation Packages - HIGH
**Severity:** High

**Issue:** Full-featured sandbox implementations exist in separate packages but are completely unused:
- `/internal/sandbox/seatbelt/` - Profile generation, sandbox-exec wrapping
- `/internal/sandbox/landlock/` - Landlock syscall implementation
- `/internal/sandbox/seccomp/` - BPF filter creation and application

**Why This Is Problematic:**
- Code duplication and wasted effort
- Confusing for developers (which implementation is used?)
- Tests may pass for implementations that aren't used
- Makes it unclear what the actual system behavior is

**Recommendation:**
- Integrate the subpackage implementations into manager.go
- Or document why they're separate and what the integration plan is
- Consider creating integration tests that verify the actual implementations work end-to-end

---

### 9.2 Interface Design Limitation - MEDIUM
**Severity:** Medium

The `SandboxApplier` interface requires modifying `exec.Cmd`:

```go
Apply(cmd *exec.Cmd, policy *PolicyConfig, workspace string) error
```

**Issues:**
- Some sandboxes (like Landlock) need to be applied to the current process, not a command
- Can't be applied before fork/exec
- Limits flexibility for different sandbox architectures

**Recommendation:**
- Consider splitting into two interfaces:
  - `PreExecSandbox` - Applied to current process (Landlock, Seccomp)
  - `CmdWrapperSandbox` - Wraps command (Seatbelt with sandbox-exec)
- Or add a method to detect which approach the sandbox uses

---

## 10. Recommendations Summary

### Immediate Actions (Before Any Release)

1. **CRITICAL:** Either implement actual sandbox enforcement or remove the feature entirely
2. **CRITICAL:** Add prominent warnings that sandbox is not enforced
3. **HIGH:** Integrate existing implementation packages from `seatbelt/`, `landlock/`, `seccomp/` directories
4. **HIGH:** Update all misleading documentation and comments
5. **HIGH:** Add integration tests that verify restrictions actually work

### Short-term Improvements

6. **MEDIUM:** Add workspace path validation
7. **MEDIUM:** Improve error handling and logging
8. **MEDIUM:** Document policy behaviors and security implications
9. **MEDIUM:** Add Windows support or document limitation
10. **MEDIUM:** Fix kernel version detection to use actual syscall testing

### Long-term Enhancements

11. **LOW:** Optimize performance (cache kernel version, etc.)
12. **LOW:** Add more robust kernel version parsing
13. **LOW:** Add concurrency testing
14. **LOW:** Improve architecture to support different sandbox types better

---

## 11. Test Coverage Analysis

**Existing Test Coverage:** Good unit test coverage in `manager_test.go` with 793 lines of tests.

**Missing Coverage:**
- ❌ Actual sandbox enforcement verification
- ❌ Integration tests with real filesystem restrictions
- ❌ Network restriction verification
- ❌ Error path testing
- ❌ Concurrency/race condition testing
- ❌ Edge cases (missing uname, kernel without Landlock support, etc.)

**Test Quality:** Tests verify the placeholder behavior works correctly, but don't test the intended security properties.

---

## 12. Comparison with Reference Implementation

The comments reference a Rust implementation in `codex-rs/`:
```go
// This mirrors the functionality of Rust's SandboxManager in codex-rs/core/src/sandboxing/mod.rs
```

**Discrepancy:** The Go implementation is significantly less functional than implied. The Rust implementation likely has actual enforcement while this is a placeholder.

**Recommendation:** Review the Rust implementation and ensure feature parity.

---

## 13. Positive Aspects

Despite the critical issues, the code has some good qualities:

✅ **Good Architecture:** Clean interface design with `SandboxApplier`
✅ **OS Detection:** Proper handling of OS-specific implementations
✅ **Fallback Logic:** Graceful degradation when sandbox unavailable
✅ **Test Structure:** Well-organized tests covering unit-level behavior
✅ **Policy Design:** Clean policy abstraction with clear types
✅ **Error Handling:** Generally good error handling patterns
✅ **Documentation Intent:** Functions have comments (even if misleading)

The bones are good - this just needs the actual implementation connected.

---

## 14. Conclusion

This file represents a **well-designed but critically incomplete sandbox system**. The architecture is sound, but the complete lack of enforcement creates a severe security vulnerability that could lead to false confidence and system compromise.

**Overall Assessment:** 🔴 **Not Production Ready**

**Blocker Issues:** 3 critical, 4 high priority
**Estimated Effort to Production Ready:** 2-4 weeks (implementing actual enforcement)

**Critical Path to Production:**
1. Integrate existing `seatbelt/landlock/seccomp` implementations
2. Add integration tests verifying actual restrictions
3. Update all documentation to reflect actual behavior
4. Security review of integrated implementation
5. Performance testing of sandbox overhead

**Alternative Recommendation:** If full implementation is not planned soon, consider:
- Removing the sandbox feature entirely until ready
- Adding a `--disable-sandbox` flag and refusing to run without it
- Making the incomplete status extremely obvious to users
- Not exposing this API to end users until functional

---

## Appendix A: Related Files

Files that should be reviewed together with this one:
- `/Users/williamcory/codex/codex-go/internal/sandbox/interface.go` - Interfaces and types
- `/Users/williamcory/codex/codex-go/internal/sandbox/policy.go` - Policy definitions
- `/Users/williamcory/codex/codex-go/internal/sandbox/seatbelt/seatbelt.go` - Actual Seatbelt implementation
- `/Users/williamcory/codex/codex-go/internal/sandbox/landlock/landlock.go` - Actual Landlock implementation
- `/Users/williamcory/codex/codex-go/internal/sandbox/seccomp/seccomp.go` - Actual Seccomp implementation
- `/Users/williamcory/codex/codex-go/internal/sandbox/manager_test.go` - Test suite

---

## Appendix B: Security Impact Matrix

| Scenario | Expected Behavior | Actual Behavior | Risk Level |
|----------|-------------------|-----------------|------------|
| ReadOnly policy with file write | Write fails | Write succeeds | Critical |
| ReadOnly policy with network access | Network fails | Network succeeds | Critical |
| WorkspaceWrite with write outside workspace | Write fails | Write succeeds | Critical |
| WorkspaceWrite with network disabled | Network fails | Network succeeds | High |
| Malicious code ignoring env vars | Sandbox blocks | No protection | Critical |
| Path traversal in workspace | Detected/blocked | Allowed | High |

---

**End of Review**
