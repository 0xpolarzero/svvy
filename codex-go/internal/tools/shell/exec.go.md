# Code Review: exec.go

**File:** `/Users/williamcory/codex/codex-go/internal/tools/shell/exec.go`
**Reviewed:** 2025-10-26
**Reviewer:** Automated Code Review

---

## Executive Summary

The `exec.go` file implements command execution functionality with sandboxing and environment filtering capabilities. While the code demonstrates good security practices in some areas (environment filtering, sandbox integration), it has several critical issues including incomplete validation, missing error handling, unclear feature completeness, and potential security vulnerabilities.

**Overall Rating:** ⚠️ Needs Improvement

---

## 1. Incomplete Features & Functionality

### 1.1 Limited Command Validation (MEDIUM PRIORITY)

**Location:** Lines 49-55

**Issue:** The command validation only checks if the command array is empty, but doesn't validate:
- Whether the command exists (could use `IsCommandAvailable` helper)
- Whether command elements are non-empty strings
- Whether the command contains suspicious patterns

**Current Code:**
```go
if len(spec.Command) == 0 {
    return nil, runtime.NewToolError(
        runtime.ErrorInvalidArguments,
        "command cannot be empty",
    )
}
```

**Missing Validations:**
- No check for nil command elements
- No check for empty string elements in the array
- No validation of command syntax before execution

### 1.2 Working Directory Validation Missing (HIGH PRIORITY)

**Location:** Lines 61-63

**Issue:** The working directory is set without any validation:
- No check if the directory exists
- No check if the directory is accessible
- No path traversal protection
- No symlink resolution checking

**Current Code:**
```go
if spec.WorkingDirectory != "" {
    cmd.Dir = spec.WorkingDirectory
}
```

**Security Concern:** This could allow path traversal attacks or execution in unintended directories.

### 1.3 Environment Variable Filtering Only When Map Provided

**Location:** Lines 66-78

**Issue:** Environment filtering only happens when `spec.Environment` is provided. When the map is empty or nil, the command inherits the full system environment unfiltered.

**Current Code:**
```go
if len(spec.Environment) > 0 {
    filter := NewDefaultEnvFilter()
    cmd.Env = filter.Filter()
    // ...
}
```

**Missing Logic:** When `len(spec.Environment) == 0`, the command inherits all environment variables from the parent process, potentially including sensitive credentials.

**Recommendation:** Always filter environment variables, even when no custom environment is provided:
```go
filter := NewDefaultEnvFilter()
cmd.Env = filter.Filter()

if len(spec.Environment) > 0 {
    filteredEnv := filter.FilterMap(spec.Environment)
    for k, v := range filteredEnv {
        cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
    }
}
```

### 1.4 No Output Size Limiting

**Location:** Lines 80-92

**Issue:** Output capture has no size limits. Commands producing large amounts of output could cause memory exhaustion.

**Missing Feature:** No integration with `LimitWriter` (which exists in `output.go` but is unused here).

### 1.5 Binary Output Handling Not Integrated

**Location:** Lines 119-122

**Issue:** While `IsBinaryData` exists in `binary.go`, it's not used in the main execution flow. Binary output detection and handling is incomplete.

**Current Code:**
```go
stdout := capturer.Stdout()
stderr := capturer.Stderr()
```

**Missing Logic:** No check if output contains binary data, no base64 encoding for binary output.

---

## 2. TODO Comments & Technical Debt

### 2.1 Acknowledged Technical Debt

**Location:** Lines 197-203 (`SanitizeCommand` function)

**Comment:**
```go
// SanitizeCommand performs basic sanitization on command strings.
// This is a simple implementation and should be enhanced for production use.
```

**Issue:** The comment explicitly states this is not production-ready, yet there's no TODO or tracking issue. The function only:
- Removes null bytes
- Trims whitespace

**Missing Sanitization:**
- No shell metacharacter escaping
- No command injection prevention
- No quote handling
- No path separator normalization

**Recommendation:** Add explicit TODO with requirements:
```go
// TODO(security): Enhance SanitizeCommand for production:
// - Add shell metacharacter escaping
// - Implement command injection detection
// - Add quote handling and normalization
// - Consider using shellquote library for proper escaping
```

### 2.2 Implicit Technical Debt

No explicit TODO/FIXME comments found in the file, but several areas have implicit technical debt (covered in other sections).

---

## 3. Code Quality Issues

### 3.1 Inconsistent Error Handling

**Location:** Lines 127-150

**Issue:** Error handling logic is complex and has inconsistent patterns:

1. Context errors are checked first (good)
2. ExitError is type-asserted (good)
3. Other errors return immediately (inconsistent with exit code handling)

**Problem Code:**
```go
if execErr != nil {
    success = false

    if ctx.Err() == context.DeadlineExceeded || ctx.Err() == context.Canceled {
        return nil, runtime.NewToolErrorWithCause(...)
    }

    if exitError, ok := execErr.(*exec.ExitError); ok {
        exitCode = exitError.ExitCode()
    } else {
        // Other execution errors (e.g., command not found)
        return nil, runtime.NewToolErrorWithCause(...)
    }
}
```

**Issue:** The "other execution errors" path returns early, but what if we want to capture partial output? The current design loses any stdout/stderr that was captured before the error.

### 3.2 Unclear Fallback Logic

**Location:** Lines 153-156

**Issue:** The fallback content logic is unclear:

```go
content := aggregateOutput(stdout, stderr)
if content == "" && execErr != nil {
    content = execErr.Error()
}
```

**Questions:**
- Why only fallback to `execErr.Error()` when content is empty?
- Should we append the error message rather than replace?
- What about the case where stderr has the real error but it's not being shown?

### 3.3 Magic String in Workspace Default

**Location:** Lines 97-100

**Issue:** Uses magic string "." for default workspace:

```go
workspace := spec.WorkingDirectory
if workspace == "" {
    workspace = "."
}
```

**Better Approach:** Use a named constant or document why "." is used:
```go
const defaultWorkspace = "." // Current directory
```

### 3.4 Inconsistent Nil Checking

**Location:** Lines 169-171

**Issue:** Metadata map is lazily initialized:

```go
if resp.Metadata == nil {
    resp.Metadata = make(map[string]interface{})
}
```

**Problem:** `ToolResponse.Metadata` should be initialized in the response construction, or this pattern should be used consistently for all optional fields.

### 3.5 Missing Documentation for Complex Logic

**Location:** Lines 84-92

**Issue:** Output streaming setup lacks explanation:

```go
if execCtx.OutputWriter != nil {
    cmd.Stdout = io.MultiWriter(capturer.stdout, execCtx.OutputWriter)
    cmd.Stderr = io.MultiWriter(capturer.stderr, execCtx.OutputWriter)
} else {
    cmd.Stdout = capturer.stdout
    cmd.Stderr = capturer.stderr
}
```

**Missing Documentation:** No comment explaining:
- When OutputWriter is expected to be set
- What the streaming behavior is
- Whether streaming affects the final response

---

## 4. Missing Test Coverage

### 4.1 Tests Present (Good Coverage)

The following scenarios are well-tested in `exec_test.go`:
- ✅ Command availability checking
- ✅ Command sanitization
- ✅ Executor creation
- ✅ Timeout handling (success and expiration)
- ✅ Zero timeout (no timeout) behavior
- ✅ Command not found errors
- ✅ Empty command validation
- ✅ Binary output detection
- ✅ Mixed output (stdout/stderr)
- ✅ Benchmark tests

### 4.2 Missing Test Coverage (CRITICAL GAPS)

#### 4.2.1 Environment Variable Filtering Tests
**Missing:**
- Test with empty environment map (should still filter system env)
- Test with nil environment map (should still filter system env)
- Test that sensitive variables are actually filtered in execution
- Integration test verifying credentials don't leak

#### 4.2.2 Working Directory Tests
**Missing:**
- Test with non-existent working directory
- Test with invalid working directory (e.g., a file instead of directory)
- Test with relative vs absolute paths
- Test with symlinks in working directory path
- Test working directory permissions (unreadable, no-execute)

#### 4.2.3 Sandbox Integration Tests
**Missing:**
- Test with sandbox policy applied
- Test sandbox failure handling
- Test sandbox metadata in response
- Test sandbox with different policy types

#### 4.2.4 Edge Cases Missing
**Missing:**
- Test with command array containing empty strings
- Test with command array containing nil elements (if possible in Go)
- Test with very long command arguments
- Test with special characters in arguments
- Test with commands producing output exceeding reasonable size
- Test with commands that close stdout/stderr unexpectedly

#### 4.2.5 Output Handling Tests
**Missing:**
- Test streaming vs non-streaming output
- Test with OutputWriter set
- Test output truncation or size limiting (feature doesn't exist)
- Test Unicode handling in output
- Test malformed UTF-8 in output

#### 4.2.6 Error Scenarios
**Missing:**
- Test context cancellation mid-execution
- Test execution with expired context from the start
- Test error messages format and content
- Test ToolError Kind values are correct for each scenario

#### 4.2.7 Concurrent Execution Tests
**Missing:**
- Test multiple concurrent executions
- Test thread safety of CommandExecutor
- Test race conditions in output capture

---

## 5. Potential Bugs & Edge Cases

### 5.1 Race Condition in Context Checking (LOW SEVERITY)

**Location:** Line 131

**Issue:**
```go
if ctx.Err() == context.DeadlineExceeded || ctx.Err() == context.Canceled {
```

**Problem:** `ctx.Err()` is called twice. Between calls, the context state could theoretically change (though unlikely in practice).

**Better Code:**
```go
ctxErr := ctx.Err()
if ctxErr == context.DeadlineExceeded || ctxErr == context.Canceled {
    return nil, runtime.NewToolErrorWithCause(
        runtime.ErrorTimeout,
        "command execution timed out or was cancelled",
        ctxErr, // Use captured error
    )
}
```

### 5.2 Command Array Slice Bug Potential (LOW SEVERITY)

**Location:** Line 58

**Issue:**
```go
cmd := exec.CommandContext(ctx, spec.Command[0], spec.Command[1:]...)
```

**Potential Bug:** If `spec.Command` has length 1, `spec.Command[1:]` creates an empty slice (fine). But if someone modifies the slice after passing it, there could be unexpected behavior.

**Recommendation:** Make a defensive copy if Command is modified after Execute is called.

### 5.3 Missing Error Wrapping Context

**Location:** Lines 104-108

**Issue:**
```go
return nil, runtime.NewToolErrorWithCause(
    runtime.ErrorExecution,
    fmt.Sprintf("failed to apply sandbox: %v", err),
    err,
)
```

**Problem:** Error message doesn't include:
- What command was being run
- What sandbox policy was being applied
- What workspace was being used

**Better Error:**
```go
return nil, runtime.NewToolErrorWithCause(
    runtime.ErrorExecution,
    fmt.Sprintf("failed to apply sandbox policy to command '%s': %v",
        spec.Command[0], err),
    err,
)
```

### 5.4 Silent Environment Inheritance (HIGH SEVERITY)

**Location:** Lines 66-78

**Bug:** When `spec.Environment` is empty/nil, the command gets FULL unfiltered access to parent environment.

```go
if len(spec.Environment) > 0 {
    filter := NewDefaultEnvFilter()
    cmd.Env = filter.Filter()
    // filtered env added
}
// else: cmd.Env remains nil, inherits everything!
```

**Impact:** This could leak:
- API keys from parent process
- Database credentials
- SSH keys
- Any sensitive environment variables

**This is a CRITICAL security vulnerability.**

### 5.5 Timeout Not Applied Correctly (MEDIUM SEVERITY)

**Location:** Lines 181-188

**Issue:**
```go
func (e *CommandExecutor) ExecuteWithTimeout(ctx context.Context, spec *CommandSpec, execCtx *runtime.ExecutionContext, timeout time.Duration) (*runtime.ToolResponse, error) {
    if timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, timeout)
        defer cancel()
    }
    return e.Execute(ctx, spec, execCtx)
}
```

**Problem:** If `timeout <= 0`, no timeout is applied. But what about negative timeouts? Should they error?

**Edge Case:** `timeout == -1 * time.Second` - no timeout applied, might be confusing.

### 5.6 Potential Panic in Metadata Setting

**Location:** Lines 168-175

**Issue:**
```go
if sandboxInfo != nil && sandboxInfo.Applied {
    if resp.Metadata == nil {
        resp.Metadata = make(map[string]interface{})
    }
    resp.Metadata["sandbox_type"] = sandboxInfo.Type.String()
```

**Potential Bug:** If `sandboxInfo.Type` is nil or invalid, `String()` could panic.

**Recommendation:** Add nil check or handle error from `String()`.

### 5.7 No Stdin Support

**Missing Feature:** There's no way to provide stdin to commands. The `exec.Cmd.Stdin` field is never set.

**Impact:** Commands expecting stdin will hang or fail unexpectedly.

---

## 6. Documentation Issues

### 6.1 Missing Package-Level Documentation

**Issue:** The file has no package-level documentation explaining:
- Purpose of the shell package
- Security model
- Usage examples
- Integration with sandbox system

### 6.2 Incomplete Function Documentation

**Location:** Multiple functions

**Issues:**

1. **`Execute` (line 45):**
   - Missing parameter documentation
   - Missing return value documentation
   - Missing error conditions documentation
   - No examples

2. **`ExecuteWithTimeout` (line 180):**
   - Missing behavior documentation for `timeout <= 0`
   - Missing explanation of how timeout interacts with context
   - No mention that timeout of 0 means "no timeout"

3. **`NewCommandExecutor` (line 38):**
   - Missing thread-safety documentation
   - Missing resource cleanup requirements (if any)

4. **`IsCommandAvailable` (line 191):**
   - Missing note about PATH dependency
   - Missing note about race conditions (command could become unavailable after check)

5. **`SanitizeCommand` (line 197):**
   - Documentation admits it's not production-ready but doesn't say what's needed
   - Missing examples of what is and isn't sanitized

### 6.3 Missing Struct Field Documentation

**Location:** Lines 15-31 (`CommandSpec`)

**Issues:**
- `CallID` - What format should it have? Is it required? What happens if duplicated?
- `SandboxPolicy` - What happens if nil? What are valid values?
- `Environment` - Is it merged with system env or replacement?
- `WorkingDirectory` - What happens if it doesn't exist?

### 6.4 No Security Documentation

**Missing:** No documentation about:
- Security implications of command execution
- Sandbox requirements
- Environment filtering behavior
- When to use which sandbox policies

**Recommendation:** Add a SECURITY.md reference or inline security notes.

---

## 7. Security Concerns

### 7.1 CRITICAL: Unfiltered Environment Inheritance

**Severity:** 🔴 CRITICAL
**Location:** Lines 66-78
**CVE-Potential:** Yes

**Issue:** When `spec.Environment` is empty or nil, commands inherit the full parent process environment without filtering.

**Attack Scenario:**
```go
// Attacker can read sensitive env vars
spec := &CommandSpec{
    Command: []string{"env"},
    // Environment: nil - gets EVERYTHING
}
// Output will include API_KEY, AWS_SECRET_ACCESS_KEY, etc.
```

**Fix Required:** Always filter environment, even when no custom environment is provided.

### 7.2 HIGH: No Working Directory Validation

**Severity:** 🔴 HIGH
**Location:** Lines 61-63

**Issue:** Working directory is used without validation:
- Could be set to `/etc` or other sensitive directories
- Could include path traversal (`../../etc/passwd`)
- Could be a symlink to sensitive location
- Could be non-existent (causing confusing errors)

**Attack Scenario:**
```go
spec := &CommandSpec{
    Command: []string{"cat", "shadow"},
    WorkingDirectory: "/etc", // Direct access to sensitive dir
}
```

### 7.3 HIGH: Command Injection via Arguments

**Severity:** 🔴 HIGH
**Location:** Line 58

**Issue:** While using `exec.CommandContext` with separate arguments (good!), there's no validation of argument content.

**Current Protection:** ✅ Using `exec.CommandContext(ctx, cmd, args...)` prevents shell injection.

**However:** No validation that arguments don't contain:
- Null bytes
- Path traversal sequences
- Excessive length
- Binary data

**Potential Issue with SanitizeCommand:**
The `SanitizeCommand` function exists but is NEVER USED in `Execute`. It's only exported for external use but not applied internally.

### 7.4 MEDIUM: No Resource Limits

**Severity:** 🟡 MEDIUM
**Location:** Execute function overall

**Issue:** No limits on:
- CPU usage
- Memory consumption
- Output size
- Number of file descriptors
- Disk usage

**Denial of Service Scenarios:**
```go
// Infinite output
Command: []string{"yes"}

// Fork bomb (if sandbox not applied)
Command: []string{"sh", "-c", ":(){ :|:& };:"}

// Memory exhaustion
Command: []string{"sh", "-c", "cat /dev/zero"}
```

**Mitigation:** Sandbox should handle this, but there's no validation that sandbox is actually applied.

### 7.5 MEDIUM: Information Disclosure via Error Messages

**Severity:** 🟡 MEDIUM
**Location:** Lines 144-148

**Issue:** Error messages may leak information:

```go
return nil, runtime.NewToolErrorWithCause(
    runtime.ErrorExecution,
    fmt.Sprintf("failed to execute command: %v", execErr),
    execErr,
)
```

**Information Leaked:**
- Full command path
- Argument values (could contain sensitive data)
- File system structure
- System configuration

### 7.6 LOW: Timing Attacks Possible

**Severity:** 🟢 LOW
**Location:** Line 117

**Issue:** Execution time is recorded and returned:

```go
executionTime := time.Since(startTime)
```

**Potential Issue:** Timing information could be used for:
- Determining if files exist (quick fail vs slow processing)
- Brute-force optimization
- System profiling

**Note:** This is generally acceptable, but should be documented as observable behavior.

### 7.7 Security Best Practices Missing

**Missing Security Features:**

1. **No allow-list for commands:**
   - Any command in PATH can be executed
   - No restriction to safe commands only

2. **No deny-list for dangerous commands:**
   - Could execute `rm`, `dd`, `mkfs`, etc.
   - Relies entirely on sandbox (if applied)

3. **No audit logging:**
   - No record of what commands were executed
   - No security event logging
   - Hard to detect abuse

4. **No rate limiting:**
   - Same command can be executed repeatedly
   - No protection against resource exhaustion via repeated calls

---

## 8. Recommendations

### 8.1 Immediate Actions (CRITICAL - Fix Now)

1. **Fix environment variable inheritance vulnerability:**
   ```go
   // ALWAYS filter environment
   filter := NewDefaultEnvFilter()
   cmd.Env = filter.Filter()

   // Then add custom environment if provided
   if len(spec.Environment) > 0 {
       filteredEnv := filter.FilterMap(spec.Environment)
       for k, v := range filteredEnv {
           cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
       }
   }
   ```

2. **Add working directory validation:**
   ```go
   if spec.WorkingDirectory != "" {
       // Validate directory exists and is accessible
       if err := validateWorkingDirectory(spec.WorkingDirectory); err != nil {
           return nil, runtime.NewToolError(
               runtime.ErrorInvalidArguments,
               fmt.Sprintf("invalid working directory: %v", err),
           )
       }
       cmd.Dir = spec.WorkingDirectory
   }
   ```

3. **Add comprehensive tests for environment filtering:**
   - Test empty environment map
   - Test nil environment map
   - Verify sensitive vars are filtered
   - Add integration tests

### 8.2 High Priority (Fix Soon)

1. **Implement output size limiting:**
   - Use `LimitWriter` from `output.go`
   - Add configurable limit (e.g., 10MB default)
   - Return error when limit exceeded

2. **Add command validation:**
   - Check command elements are non-empty
   - Optionally verify command exists before execution
   - Add length limits on arguments

3. **Improve error messages:**
   - Add more context to errors
   - Sanitize error messages to prevent information disclosure
   - Include CallID in error messages for debugging

4. **Add security documentation:**
   - Document security model
   - Add examples of safe usage
   - Document when sandboxing is required

### 8.3 Medium Priority (Technical Debt)

1. **Enhance `SanitizeCommand` or remove it:**
   - Either implement proper sanitization
   - Or remove the function if not used
   - Add tests if keeping

2. **Add stdin support:**
   - Add `Stdin` field to `CommandSpec`
   - Support io.Reader for stdin
   - Add size limits on stdin

3. **Implement binary output handling:**
   - Integrate `IsBinaryData` check
   - Use base64 encoding for binary output
   - Add metadata indicating binary content

4. **Add audit logging:**
   - Log all command executions
   - Include timestamp, user context, command, exit code
   - Make logging configurable

### 8.4 Low Priority (Nice to Have)

1. **Add metrics/monitoring:**
   - Track execution counts
   - Monitor execution times
   - Track error rates

2. **Add command allow/deny lists:**
   - Configurable safe command list
   - Block dangerous commands by default
   - Make overridable with explicit permission

3. **Improve streaming support:**
   - Add line-by-line callbacks
   - Support progress reporting
   - Add streaming context/metadata

4. **Add caching for `IsCommandAvailable`:**
   - Cache results for performance
   - Add TTL for cache entries
   - Thread-safe cache implementation

---

## 9. Test Coverage Analysis

### Current Test Coverage: ~65% (Estimated)

**Well-Covered Areas:**
- Basic execution (echo, simple commands)
- Timeout handling
- Error cases (command not found, empty command)
- Binary output detection

**Poorly Covered Areas:**
- Environment variable filtering (0% coverage)
- Working directory validation (0% coverage)
- Sandbox integration (0% coverage)
- Concurrent execution (0% coverage)
- Edge cases with special characters (0% coverage)

**Recommended Test Additions:**

```go
// Test environment filtering
func TestCommandExecutorEnvironmentFiltering(t *testing.T) { }
func TestCommandExecutorEnvironmentInheritance(t *testing.T) { }
func TestCommandExecutorEmptyEnvironmentFiltered(t *testing.T) { }

// Test working directory
func TestCommandExecutorInvalidWorkingDirectory(t *testing.T) { }
func TestCommandExecutorRelativeWorkingDirectory(t *testing.T) { }
func TestCommandExecutorWorkingDirectoryPermissions(t *testing.T) { }

// Test sandbox
func TestCommandExecutorSandboxIntegration(t *testing.T) { }
func TestCommandExecutorSandboxFailure(t *testing.T) { }
func TestCommandExecutorSandboxMetadata(t *testing.T) { }

// Test edge cases
func TestCommandExecutorSpecialCharacters(t *testing.T) { }
func TestCommandExecutorLargeOutput(t *testing.T) { }
func TestCommandExecutorConcurrentExecution(t *testing.T) { }
func TestCommandExecutorContextCancellation(t *testing.T) { }

// Test streaming
func TestCommandExecutorStreamingOutput(t *testing.T) { }
func TestCommandExecutorOutputWriter(t *testing.T) { }
```

---

## 10. Conclusion

The `exec.go` file provides a solid foundation for command execution with good intentions around security (environment filtering, sandboxing). However, it suffers from several critical issues:

### Critical Issues:
1. 🔴 **Environment variable inheritance vulnerability** - Credentials can leak when no custom environment provided
2. 🔴 **No working directory validation** - Path traversal and other attacks possible
3. 🔴 **Incomplete security measures** - No output limits, no command validation

### Strengths:
- ✅ Good use of `exec.CommandContext` preventing shell injection
- ✅ Environment filtering infrastructure exists
- ✅ Sandbox integration designed-in
- ✅ Decent test coverage for basic scenarios
- ✅ Thread-safe output capture

### Must-Fix Before Production:
1. Fix environment inheritance (CRITICAL)
2. Add working directory validation (CRITICAL)
3. Add output size limits (HIGH)
4. Expand test coverage (HIGH)
5. Complete security documentation (HIGH)

### Estimated Effort:
- Critical fixes: 1-2 days
- High priority items: 2-3 days
- Medium priority items: 3-5 days
- Low priority items: 5-7 days

**Total effort to production-ready: ~2 weeks**

---

## Appendix A: Related Files to Review

The following files are closely related and should be reviewed together:

1. `/Users/williamcory/codex/codex-go/internal/tools/shell/envfilter.go` - Environment filtering implementation
2. `/Users/williamcory/codex/codex-go/internal/tools/shell/output.go` - Output capture implementation
3. `/Users/williamcory/codex/codex-go/internal/tools/shell/binary.go` - Binary detection (underutilized)
4. `/Users/williamcory/codex/codex-go/internal/sandbox/manager.go` - Sandbox integration
5. `/Users/williamcory/codex/codex-go/internal/tools/runtime/runtime.go` - Error types and execution context

---

## Appendix B: Security Checklist

- [ ] Environment variables are always filtered
- [ ] Working directory is validated before use
- [ ] Command arguments are validated
- [ ] Output size is limited
- [ ] Timeout is always enforced
- [ ] Sandbox is applied for untrusted commands
- [ ] Error messages don't leak sensitive information
- [ ] Audit logging is implemented
- [ ] Rate limiting is in place
- [ ] stdin is validated if supported
- [ ] Binary output is properly handled
- [ ] Resource limits are enforced

**Current Score: 3/12 (25%)**

---

*End of Review*
