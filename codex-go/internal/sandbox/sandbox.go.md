# Code Review: sandbox.go

**File**: `/Users/williamcory/codex/codex-go/internal/sandbox/sandbox.go`
**Reviewed**: 2025-10-26
**Lines of Code**: 84
**Purpose**: Core interfaces and types for sandboxed command execution

---

## Executive Summary

The `sandbox.go` file defines the core abstractions for the sandbox system. It is well-structured and follows Go best practices. However, there are several areas that need attention:

- **Missing validation methods** for Command and Result types
- **No unit tests** directly testing these types
- **Incomplete documentation** on field constraints and edge cases
- **Limited error handling** patterns defined
- **Thread-safety** not documented

**Overall Assessment**: 6.5/10 - Good foundation but needs validation, testing, and better documentation.

---

## 1. Incomplete Features & Functionality

### 1.1 Missing Validation Methods

**Severity**: HIGH
**Impact**: Runtime errors, undefined behavior

The `Command` struct lacks validation methods to ensure fields are properly set before execution:

```go
// MISSING: Command validation
func (c *Command) Validate() error {
    if c.Program == "" {
        return fmt.Errorf("program is required")
    }
    if c.Timeout < 0 {
        return fmt.Errorf("timeout cannot be negative")
    }
    // Additional validation needed
}
```

**Issues**:
- No check for empty `Program` field
- No validation of `WorkingDirectory` existence
- No validation of path permissions for `ReadOnlyPaths` and `ReadWritePaths`
- No check for valid timeout values
- No validation of environment variable keys (e.g., no spaces, valid format)

### 1.2 Missing Helper Methods

**Severity**: MEDIUM

The types lack convenience methods that would improve usability:

```go
// MISSING: Result helpers
func (r *Result) Success() bool {
    return r.ExitCode == 0 && r.Error == nil && r.Violation == nil
}

func (r *Result) Failed() bool {
    return !r.Success()
}

func (r *Result) TimedOut() bool {
    // Check if timeout occurred
}

// MISSING: Command builders
func NewCommand(program string, args ...string) *Command {
    return &Command{Program: program, Args: args}
}

func (c *Command) WithTimeout(timeout time.Duration) *Command {
    c.Timeout = timeout
    return c
}
```

### 1.3 Missing Clone/Copy Methods

**Severity**: MEDIUM
**Impact**: Potential mutations in concurrent code

The `Command` struct is mutable and lacks deep copy methods:

```go
// MISSING: Deep copy for safe concurrent usage
func (c *Command) Clone() *Command {
    clone := *c
    clone.Args = make([]string, len(c.Args))
    copy(clone.Args, c.Args)
    // ... copy other slice/map fields
    return &clone
}
```

---

## 2. TODO Comments & Technical Debt

### No TODO Comments Found

**Status**: GOOD

The file contains no TODO, FIXME, HACK, or similar markers. However, the manager.go file has several TODOs for incomplete sandbox implementations:
- Line 162: Seatbelt implementation incomplete
- Line 207: Landlock implementation incomplete
- Line 243: Seccomp implementation incomplete

---

## 3. Code Quality Issues

### 3.1 Unclear Semantics

**Severity**: MEDIUM
**Location**: Lines 28-55

#### Issue: Result.Error vs ExitCode Confusion

The `Result` struct has both `Error` and `ExitCode` fields with unclear semantics:

```go
type Result struct {
    ExitCode int    // Command exit code
    Error error     // Sandbox execution error (not command failure)
    Violation *Violation
}
```

**Problems**:
1. When should `Error` be set vs a non-zero `ExitCode`?
2. Can both be set simultaneously?
3. What if command succeeds but sandbox fails?
4. Priority unclear when `Violation` is also set

**Recommendation**:
```go
// Enhanced documentation needed:
type Result struct {
    // ExitCode is the command's exit status (0 = success).
    // This reflects the command's execution result, not sandbox failures.
    // When Error is set, ExitCode may be undefined.
    ExitCode int

    // Error indicates a sandbox infrastructure failure (not command failure).
    // Examples: Docker daemon error, container creation failure, timeout.
    // When Error is set, the command may not have executed at all.
    // Check this field first before evaluating ExitCode.
    Error error

    // Violation is set when sandbox policy was violated during execution.
    // The command may have partially executed before violation detection.
    // When set, check Violation.ExitCode for the actual command exit status.
    Violation *Violation
}
```

### 3.2 Field Naming Inconsistency

**Severity**: LOW
**Location**: Lines 42-45

The `ReadOnlyPaths` and `ReadWritePaths` fields are plural while most other fields are singular or clearly countable:

```go
// Current
ReadOnlyPaths []string
ReadWritePaths []string

// More consistent with Go conventions would be:
ReadOnlyMounts []string  // or ReadOnlyDirs
ReadWriteMounts []string // or ReadWriteDirs
```

However, changing this would break the API, so document the pattern clearly instead.

### 3.3 Missing Constraints Documentation

**Severity**: MEDIUM
**Location**: Lines 28-55

Field constraints are not documented:

```go
type Command struct {
    // Program is the command to execute
    // MISSING: Can this be a relative path? Absolute only? Must exist?
    Program string

    // WorkingDirectory is the directory to execute the command in
    // MISSING: What if it doesn't exist? Created automatically? Error?
    // MISSING: Relative to what? Host system or sandbox?
    WorkingDirectory string

    // Timeout specifies the maximum execution duration
    // MISSING: What if 0? Infinite? Default timeout?
    // MISSING: Includes cleanup time or just execution?
    Timeout time.Duration
}
```

### 3.4 Environment Variable Handling

**Severity**: MEDIUM
**Location**: Line 39

```go
// Environment contains environment variables as key-value pairs
Environment map[string]string
```

**Issues**:
1. No documentation on merging with parent environment
2. No handling of special characters in keys/values
3. No escaping documented
4. Platform-specific differences not mentioned (Windows vs Unix)

### 3.5 Interface Design

**Severity**: LOW
**Location**: Lines 13-25

The `Sandbox` interface is minimal but potentially limiting:

```go
type Sandbox interface {
    Type() string
    IsAvailable() bool
    Execute(ctx context.Context, cmd *Command) (*Result, error)
    Cleanup(ctx context.Context) error
}
```

**Missing capabilities**:
1. No health check method
2. No warm-up/pre-initialize method
3. No resource usage statistics
4. No concurrent execution limits
5. No way to query sandbox capabilities

---

## 4. Missing Test Coverage

### 4.1 No Direct Unit Tests

**Severity**: HIGH
**Impact**: Type safety, validation, edge cases uncovered

The file has **NO direct unit tests**. Test coverage comes only from integration tests in implementations.

**Missing test categories**:

1. **Command struct tests**:
   - Field validation
   - Edge cases (empty program, nil maps, negative timeout)
   - Slice/map mutation safety
   - String representation

2. **Result struct tests**:
   - Multiple error conditions (Error + ExitCode + Violation)
   - Success detection logic
   - Timeout detection
   - Edge case combinations

3. **Sandbox interface tests**:
   - Mock implementation verification
   - Interface compliance tests
   - Contract validation

4. **Commander interface tests**:
   - Mock implementation
   - Error propagation
   - Context cancellation

**Recommendation**: Create `sandbox_test.go` with:

```go
package sandbox_test

import "testing"

func TestCommand_Validation(t *testing.T) { /* ... */ }
func TestCommand_EmptyProgram(t *testing.T) { /* ... */ }
func TestCommand_NegativeTimeout(t *testing.T) { /* ... */ }
func TestCommand_EnvironmentMutation(t *testing.T) { /* ... */ }

func TestResult_Success(t *testing.T) { /* ... */ }
func TestResult_MultipleErrors(t *testing.T) { /* ... */ }
func TestResult_ViolationPriority(t *testing.T) { /* ... */ }

func TestSandboxInterface_MockCompliance(t *testing.T) { /* ... */ }
```

### 4.2 Coverage Analysis

Current package coverage: **77.6%** (from test run)

However, `sandbox.go` itself likely has **0% direct coverage** since it only defines types and interfaces. This is acceptable for pure interface definitions, but the types need validation methods that should be tested.

---

## 5. Potential Bugs & Edge Cases

### 5.1 Nil Map Handling

**Severity**: HIGH
**Location**: Line 39

```go
Environment map[string]string
```

**Problem**: Nil map vs empty map semantics undefined.

```go
cmd := &Command{
    Environment: nil, // Should this inherit parent env?
}

cmd2 := &Command{
    Environment: make(map[string]string), // Empty = no env vars?
}
```

**Impact**: Implementations may handle differently, leading to inconsistent behavior.

### 5.2 Path Overlap

**Severity**: MEDIUM
**Location**: Lines 42-45

```go
ReadOnlyPaths []string
ReadWritePaths []string
```

**Problem**: What if paths overlap?

```go
cmd := &Command{
    ReadOnlyPaths:  []string{"/home"},
    ReadWritePaths: []string{"/home/user/workspace"},
}
```

**Questions**:
1. Which takes precedence?
2. Is this an error?
3. How do implementations handle this?

### 5.3 Timeout Precision

**Severity**: LOW
**Location**: Line 54

```go
Timeout time.Duration
```

**Problem**: No documentation on precision or enforcement.

**Questions**:
1. Is timeout enforced by context deadline?
2. What about cleanup time?
3. Sub-second precision supported?
4. Zero value behavior undefined

### 5.4 Empty String Fields

**Severity**: MEDIUM
**Location**: Lines 30-36

No validation prevents empty strings:

```go
cmd := &Command{
    Program: "",           // Should error
    WorkingDirectory: "",  // Should error or use default?
    Stdin: "",            // Valid (no stdin) or error?
}
```

### 5.5 Concurrent Execution

**Severity**: MEDIUM
**Location**: Entire file

**Problem**: Thread safety not documented.

```go
// Is this safe?
var cmd Command
go sandbox.Execute(ctx, &cmd)
go sandbox.Execute(ctx, &cmd) // Same command, concurrent
```

**Questions**:
1. Can Command be reused?
2. Can Sandbox execute concurrently?
3. Is Result thread-safe after return?

### 5.6 Context Cancellation

**Severity**: MEDIUM
**Location**: Line 21

```go
Execute(ctx context.Context, cmd *Command) (*Result, error)
```

**Problem**: Behavior on context cancellation not specified.

**Questions**:
1. Is partial Result returned on cancellation?
2. Is Error set to context.Canceled?
3. Is Cleanup called automatically?
4. What's the ExitCode on cancellation?

---

## 6. Documentation Issues

### 6.1 Missing Package Examples

**Severity**: MEDIUM

No `example_test.go` file exists demonstrating basic usage:

```go
// MISSING: Example_basicUsage
func Example() {
    ctx := context.Background()
    sb := native.New()

    result, err := sb.Execute(ctx, &sandbox.Command{
        Program: "echo",
        Args:    []string{"hello"},
    })

    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result.Stdout)
    // Output: hello
}
```

### 6.2 Incomplete Field Documentation

**Severity**: MEDIUM
**Location**: Throughout

Many fields lack complete documentation:

```go
// Current:
// Environment contains environment variables as key-value pairs
Environment map[string]string

// Should be:
// Environment contains environment variables as key-value pairs.
// If nil, implementations may inherit the parent process environment.
// If empty (non-nil), no environment variables are passed.
// Keys should follow OS conventions (no special characters).
// Values are passed as-is without escaping.
Environment map[string]string
```

### 6.3 Missing Interface Contracts

**Severity**: HIGH
**Location**: Lines 13-25, 80-83

Interfaces lack detailed contracts:

```go
type Sandbox interface {
    // Type returns the sandbox type identifier (e.g., "native", "docker", "kubernetes")
    // MISSING: Must it be unique? Lowercase? Enumerated values?
    Type() string

    // IsAvailable checks if this sandbox type is available on the current system
    // MISSING: Can this change at runtime? Should it be cached?
    // MISSING: Does this perform I/O? Should it be fast?
    IsAvailable() bool

    // Execute runs a command in the sandbox and returns the result
    // MISSING: Idempotent? Can cmd be reused after call?
    // MISSING: Thread-safe? Can execute concurrently?
    // MISSING: What errors are possible besides context cancellation?
    Execute(ctx context.Context, cmd *Command) (*Result, error)

    // Cleanup performs any necessary cleanup (e.g., removing containers, pods)
    // MISSING: Idempotent? Safe to call multiple times?
    // MISSING: Called automatically or manually?
    // MISSING: What if cleanup fails?
    Cleanup(ctx context.Context) error
}
```

### 6.4 No Decision Records

**Severity**: LOW

No documentation explains design decisions:
- Why separate Error and ExitCode?
- Why string slices instead of structured Path types?
- Why map for Environment instead of []string?
- Why no resource limits in Command?

### 6.5 Missing Migration Guide

**Severity**: LOW

Based on git history, this is a port from Rust. No documentation exists explaining:
- Differences from Rust implementation
- Breaking changes
- Migration path for users

---

## 7. Security Concerns

### 7.1 Path Traversal

**Severity**: HIGH
**Location**: Lines 42-45

```go
ReadOnlyPaths []string
ReadWritePaths []string
```

**Problem**: No validation prevents path traversal:

```go
cmd := &Command{
    ReadWritePaths: []string{
        "/workspace/../../../etc",
        "../../../../etc/passwd",
        "/workspace/../.ssh",
    },
}
```

**Recommendation**: Document that implementations MUST:
1. Resolve all paths to absolute
2. Check for traversal attempts
3. Validate paths are within allowed boundaries
4. Reject symlinks or resolve them safely

### 7.2 Command Injection

**Severity**: MEDIUM
**Location**: Line 30

```go
Program string
```

**Problem**: No guidance on preventing command injection:

```go
// Potentially dangerous if Program comes from untrusted input
userInput := "echo hello; rm -rf /"
cmd := &Command{Program: userInput}
```

**Recommendation**: Document that:
1. Program should be validated
2. Use absolute paths when possible
3. Avoid shell wrappers (use direct execution)
4. Args should be passed separately (not shell-parsed)

### 7.3 Environment Variable Injection

**Severity**: MEDIUM
**Location**: Line 39

```go
Environment map[string]string
```

**Problem**: No validation of dangerous environment variables:

```go
cmd := &Command{
    Environment: map[string]string{
        "LD_PRELOAD": "/tmp/malicious.so",
        "PATH": "/tmp/evil:/usr/bin",
        "LD_LIBRARY_PATH": "/tmp/backdoor",
    },
}
```

**Recommendation**: Document dangerous variables and suggest:
1. Whitelist approach for known-safe variables
2. Sanitize PATH, LD_PRELOAD, etc.
3. Consider a SecurityPolicy field

### 7.4 Resource Exhaustion

**Severity**: HIGH
**Location**: Lines 42-45, 54

**Problem**: No resource limits defined in Command:

```go
// No protection against:
cmd := &Command{
    Program: ":(){ :|:& };:",  // Fork bomb
    Timeout: 24 * time.Hour,   // Excessive timeout
    ReadWritePaths: []string{"/"},  // Write everywhere
}
```

**Recommendation**: Consider adding:
```go
type Command struct {
    // ... existing fields ...

    // Resource limits
    MaxMemory     int64         // Maximum memory in bytes
    MaxCPU        float64       // Maximum CPU cores
    MaxProcesses  int           // Maximum processes/threads
    MaxFileSize   int64         // Maximum file size for writes
}
```

### 7.5 Stdin Injection

**Severity**: LOW
**Location**: Line 50

```go
Stdin string
```

**Problem**: Unlimited stdin could exhaust memory:

```go
cmd := &Command{
    Stdin: strings.Repeat("A", 1<<30), // 1GB of data
}
```

**Recommendation**: Document maximum stdin size and enforce limits.

### 7.6 Working Directory Creation

**Severity**: MEDIUM
**Location**: Line 36

```go
WorkingDirectory string
```

**Problem**: If implementation creates missing directories, could create arbitrary paths:

```go
cmd := &Command{
    WorkingDirectory: "/etc/evil",  // Should not be created
}
```

**Recommendation**: Document that:
1. Directory must pre-exist OR
2. Only create within workspace boundary
3. Fail with clear error if missing

### 7.7 Network Access Control

**Severity**: HIGH
**Location**: Line 48

```go
NetworkEnabled bool
```

**Problem**: Binary flag is insufficient for fine-grained control:

```go
// Cannot express:
// - Allow only specific hosts
// - Allow only specific ports
// - Rate limiting
// - Protocol restrictions
```

**Recommendation**: Consider expanding:
```go
type NetworkPolicy struct {
    Enabled        bool
    AllowedHosts   []string  // Whitelist
    AllowedPorts   []int
    MaxConnections int
    RateLimit      time.Duration
}
```

---

## 8. Additional Recommendations

### 8.1 Add String Methods

Implement `String()` methods for better debugging:

```go
func (c *Command) String() string {
    return fmt.Sprintf("Command{Program=%q, Args=%v, Dir=%q}",
        c.Program, c.Args, c.WorkingDirectory)
}

func (r *Result) String() string {
    return fmt.Sprintf("Result{ExitCode=%d, Duration=%v, Error=%v}",
        r.ExitCode, r.ExecutionTime, r.Error)
}
```

### 8.2 Add Equality Methods

For testing and comparison:

```go
func (c *Command) Equal(other *Command) bool {
    // Deep comparison implementation
}
```

### 8.3 Consider Immutability

Make Command immutable with builder pattern:

```go
type CommandBuilder struct {
    cmd Command
}

func NewCommandBuilder(program string) *CommandBuilder {
    return &CommandBuilder{cmd: Command{Program: program}}
}

func (b *CommandBuilder) WithArgs(args ...string) *CommandBuilder {
    b.cmd.Args = append([]string{}, args...)
    return b
}

func (b *CommandBuilder) Build() *Command {
    clone := b.cmd
    return &clone
}
```

### 8.4 Add Telemetry Hooks

For observability:

```go
type Command struct {
    // ... existing fields ...

    // Optional telemetry callback
    OnStart  func(ctx context.Context)
    OnFinish func(ctx context.Context, result *Result)
}
```

### 8.5 Add Retry Policy

```go
type Command struct {
    // ... existing fields ...

    RetryPolicy *RetryPolicy
}

type RetryPolicy struct {
    MaxAttempts int
    Backoff     time.Duration
    RetryIf     func(*Result) bool
}
```

### 8.6 Structured Errors

Define specific error types:

```go
var (
    ErrInvalidCommand    = errors.New("invalid command")
    ErrSandboxUnavailable = errors.New("sandbox unavailable")
    ErrTimeout           = errors.New("execution timeout")
    ErrViolation         = errors.New("sandbox violation")
)

type ExecutionError struct {
    Sandbox string
    Command *Command
    Err     error
}

func (e *ExecutionError) Error() string {
    return fmt.Sprintf("sandbox %s: %v", e.Sandbox, e.Err)
}
```

---

## 9. Comparison with Related Code

### 9.1 Consistency with Implementations

Checking native, docker, and kubernetes implementations shows:

1. **Good**: All implementations properly implement the interface
2. **Good**: Timeout handling is consistent
3. **Issue**: Environment handling differs between implementations
4. **Issue**: Path resolution inconsistent

### 9.2 Rust Implementation Parity

From git history referencing Rust implementation:

1. **Complete**: Core interfaces match
2. **Missing**: Some Rust features not ported:
   - Detailed resource limits
   - Advanced network policies
   - Fine-grained capability controls

---

## 10. Priority Action Items

### Critical (Fix Immediately)

1. Add Command validation methods
2. Document Error vs ExitCode semantics clearly
3. Add security documentation for path handling
4. Create unit tests for Command and Result types

### High Priority (Next Sprint)

1. Add example_test.go with usage examples
2. Document thread-safety guarantees
3. Add structured error types
4. Implement validation for path traversal
5. Document resource exhaustion risks

### Medium Priority (Next Release)

1. Add helper methods (Success(), Failed(), etc.)
2. Add String() methods for debugging
3. Expand NetworkEnabled to NetworkPolicy
4. Add resource limit fields to Command
5. Create migration guide from Rust version

### Low Priority (Future)

1. Consider immutable Command with builder
2. Add telemetry hooks
3. Add retry policy support
4. Add equality methods for testing

---

## 11. Conclusion

The `sandbox.go` file provides a solid foundation for the sandbox system with clean interfaces and well-defined types. However, it requires significant additions in validation, documentation, testing, and security considerations to be production-ready.

**Key Strengths**:
- Clean interface design
- Good separation of concerns
- Flexible command configuration
- Proper use of context

**Key Weaknesses**:
- Missing validation
- No direct unit tests
- Incomplete documentation
- Security concerns not addressed
- Edge cases not handled

**Recommended Next Steps**:
1. Add validation methods (1 day)
2. Write comprehensive unit tests (2 days)
3. Enhance documentation with examples (1 day)
4. Add security validation and docs (1 day)
5. Review and address thread-safety (1 day)

**Total Effort Estimate**: 1 week for critical improvements

---

## Appendix A: Reference Implementation Examples

### Example: Comprehensive Command Validation

```go
func (c *Command) Validate() error {
    if c.Program == "" {
        return fmt.Errorf("program is required")
    }

    if c.Timeout < 0 {
        return fmt.Errorf("timeout cannot be negative: %v", c.Timeout)
    }

    if c.WorkingDirectory != "" {
        if !filepath.IsAbs(c.WorkingDirectory) {
            return fmt.Errorf("working directory must be absolute: %s", c.WorkingDirectory)
        }
        if strings.Contains(c.WorkingDirectory, "..") {
            return fmt.Errorf("working directory contains path traversal: %s", c.WorkingDirectory)
        }
    }

    for _, path := range append(c.ReadOnlyPaths, c.ReadWritePaths...) {
        if !filepath.IsAbs(path) {
            return fmt.Errorf("path must be absolute: %s", path)
        }
        if strings.Contains(path, "..") {
            return fmt.Errorf("path contains traversal: %s", path)
        }
    }

    for key := range c.Environment {
        if key == "" {
            return fmt.Errorf("environment variable key cannot be empty")
        }
        if strings.ContainsAny(key, " \t\n\r") {
            return fmt.Errorf("environment variable key contains whitespace: %q", key)
        }
    }

    return nil
}
```

### Example: Comprehensive Test Suite

```go
func TestCommand_Validate(t *testing.T) {
    tests := []struct {
        name    string
        cmd     *Command
        wantErr bool
    }{
        {
            name:    "empty program",
            cmd:     &Command{Program: ""},
            wantErr: true,
        },
        {
            name:    "negative timeout",
            cmd:     &Command{Program: "echo", Timeout: -1},
            wantErr: true,
        },
        {
            name:    "relative working directory",
            cmd:     &Command{Program: "echo", WorkingDirectory: "relative/path"},
            wantErr: true,
        },
        {
            name:    "path traversal in working directory",
            cmd:     &Command{Program: "echo", WorkingDirectory: "/foo/../bar"},
            wantErr: true,
        },
        {
            name:    "valid command",
            cmd:     &Command{Program: "/bin/echo", Args: []string{"hello"}},
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.cmd.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

---

**End of Review**
