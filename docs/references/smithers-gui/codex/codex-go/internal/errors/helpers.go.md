# Code Review: helpers.go

**File:** `/Users/williamcory/codex/codex-go/internal/errors/helpers.go`
**Review Date:** 2025-10-26
**Lines of Code:** 541
**Purpose:** Custom error types and helper functions for the Codex Go error handling system

---

## Executive Summary

The `helpers.go` file provides a comprehensive set of custom error types designed for domain-specific error handling in the Codex Go application. The code is generally well-structured with good test coverage (623 test lines). However, there are several areas that need attention regarding incomplete features, edge cases, and potential bugs.

**Overall Assessment:** 7/10
- Strengths: Good structure, comprehensive error types, helpful error messages, good test coverage
- Weaknesses: Missing features, inconsistent constructors, potential edge cases, some security concerns

---

## 1. Incomplete Features & Functionality

### 1.1 Missing FileErrorType Constructors

**Severity:** Medium
**Lines:** 14-31

Several `FileErrorType` enum values lack dedicated constructor functions:

- `FileErrorAlreadyExists` (line 21-22) - No `NewFileAlreadyExistsError` constructor
- `FileErrorInvalidPath` (line 23-24) - No `NewInvalidPathError` constructor
- `FileErrorTooLarge` (line 27-28) - No `NewFileTooLargeError` constructor
- `FileErrorReadOnly` (line 29-30) - No `NewReadOnlyError` constructor

**Impact:** Inconsistent API surface, developers must manually construct these errors.

**Recommendation:**
```go
// Missing constructors that should be added:
func NewFileAlreadyExistsError(path, operation string) *FileError
func NewInvalidPathError(path, operation string, reason string) *FileError
func NewFileTooLargeError(path string, size, maxSize int64) *FileError
func NewReadOnlyError(path, operation string) *FileError
```

### 1.2 Incomplete getSuggestion() Implementation

**Severity:** Low
**Lines:** 82-99

The `getSuggestion()` method doesn't provide suggestions for:
- `FileErrorAlreadyExists` (line 21)
- `FileErrorInvalidPath` (line 23)

**Recommendation:**
```go
case FileErrorAlreadyExists:
    return "Use a different filename or remove the existing file first"
case FileErrorInvalidPath:
    return "Ensure the path is properly formatted and contains valid characters"
```

### 1.3 ToolExecutionError Missing Fields Usage

**Severity:** Low
**Lines:** 416-472

The `ToolExecutionError` struct has `Args` and `Stdout` fields (lines 419-421) that are never used in the `Error()` method or constructor. These fields exist but provide no value.

**Recommendation:** Either use these fields in error messages or remove them to reduce confusion.

---

## 2. TODO Comments & Technical Debt

### 2.1 No Explicit TODOs Found

**Status:** Good

No TODO, FIXME, HACK, or BUG comments were found in the file. This is positive for code maturity.

---

## 3. Code Quality Issues

### 3.1 Inconsistent Error Message Formatting

**Severity:** Low
**Lines:** Multiple locations

Error message formatting is inconsistent across error types:

- `FileError` (line 47): `"failed to %s file"` (lowercase)
- `ValidationError` (line 177): `"validation error"` (lowercase)
- `ConfigError` (line 230): `"configuration error"` (lowercase)
- `PathError` (line 289): `"path error: %s"` (lowercase)
- `APIError` (line 356): `"API error"` (mixed case)
- `ToolExecutionError` (line 431): `"tool '%s' execution failed"` (lowercase)

**Recommendation:** Establish consistent capitalization rules for all error messages.

### 3.2 Magic Numbers in formatBytes()

**Severity:** Low
**Lines:** 492-509

Uses magic numbers instead of constants at package level:

```go
const (
    KB = 1024
    MB = 1024 * KB
    GB = 1024 * MB
)
```

These are defined within the function scope (lines 493-497), making them inaccessible for reuse elsewhere.

**Recommendation:** Move constants to package or file level if they might be reused.

### 3.3 String Concatenation with Builder

**Severity:** Low
**Lines:** Multiple

While using `strings.Builder` is good practice, some methods mix direct builder writes with formatted strings:

```go
// Line 54
builder.WriteString(fmt.Sprintf(" '%s'", e.Path))
// Could be:
builder.WriteString(" '")
builder.WriteString(e.Path)
builder.WriteString("'")
```

**Impact:** Minor performance overhead from `fmt.Sprintf` allocation.

**Recommendation:** For simple concatenations, use direct `WriteString` calls instead of `fmt.Sprintf`.

### 3.4 Redundant Zero Checks

**Severity:** Low
**Lines:** 433, 362

Zero value checks that could be implicit:

```go
// Line 433
if e.ExitCode != 0 {
```

Exit code 0 typically means success, so this check makes sense. However, consider documenting this assumption.

```go
// Line 362
if e.StatusCode > 0 {
```

This check assumes 0 is invalid, but HTTP status codes can theoretically be 0 in error conditions. Consider checking for valid ranges (100-599) instead.

---

## 4. Missing Test Coverage

### 4.1 Error Edge Cases

**Severity:** Medium
**Lines:** Various

While test coverage exists (helpers_test.go), some edge cases are missing:

1. **Empty Field Values:** What happens when all optional fields are empty?
2. **Nil Receiver Tests:** No tests verify behavior when error types have nil underlying errors
3. **Unicode in Paths:** No tests for non-ASCII characters in file paths (line 54, 292)
4. **Very Long Paths:** No tests for path truncation (Windows MAX_PATH, Linux PATH_MAX)
5. **Concurrent Error Creation:** No concurrency tests for error constructors

### 4.2 PathError Relative Path Edge Cases

**Severity:** Medium
**Lines:** 296-299

The `PathError.Error()` method attempts to compute relative paths:

```go
relPath, err := filepath.Rel(e.Workspace, e.Path)
if err == nil && !strings.HasPrefix(relPath, "..") {
    builder.WriteString(fmt.Sprintf(" [relative: %s]", relPath))
}
```

**Missing Tests:**
- What happens when `Workspace` or `Path` is empty?
- What happens with symbolic links?
- What happens with case-insensitive filesystems?
- What happens when paths are on different drives (Windows)?

### 4.3 formatBytes Edge Cases

**Severity:** Low
**Lines:** 492-509

**Missing Tests:**
- Negative values: `formatBytes(-100)`
- Zero value: `formatBytes(0)` (should return "0 B")
- Maximum int64: `formatBytes(math.MaxInt64)`
- Values just below thresholds (1023 bytes, 1023 KB, etc.)

---

## 5. Potential Bugs & Unhandled Edge Cases

### 5.1 NewFileError Type Detection Incomplete

**Severity:** High
**Lines:** 101-124

The `NewFileError` function only detects 3 error types (NotExist, Permission, Exist) but the `FileErrorType` enum has 8 variants. All other OS errors will be categorized as generic "operation failed":

```go
} else {
    fileErr.Message = "operation failed"  // Lines 120-121
}
```

**Missing Detection:**
- `FileErrorIsDirectory` - Should check `os.ErrIsDir` or stat the path
- `FileErrorInvalidPath` - Should validate path format
- `FileErrorBinary` - Requires content inspection (can't detect from error alone)
- `FileErrorTooLarge` - Requires size check (can't detect from error alone)
- `FileErrorReadOnly` - Could check file mode bits or `os.ErrReadOnly`

**Recommendation:** Document that `NewFileError` only handles basic OS errors, or enhance it to detect more types.

### 5.2 formatBytes Division by Zero

**Severity:** Low
**Lines:** 500-505

The function doesn't handle zero explicitly:

```go
default:
    return fmt.Sprintf("%d B", bytes)
}
```

This works correctly for zero (returns "0 B"), but it's not explicitly tested or documented.

### 5.3 Potential Panic in getSuggestion()

**Severity:** Low
**Lines:** 82-99

If a new `FileErrorType` is added to the enum but not handled in the switch statement, it returns empty string. This is safe but could mask implementation errors.

**Recommendation:** Add a panic or log warning in the default case during development builds to catch missing cases early.

### 5.4 PathError Relative Path Computation

**Severity:** Medium
**Lines:** 296-299

The relative path computation silently fails and skips output on error:

```go
relPath, err := filepath.Rel(e.Workspace, e.Path)
if err == nil && !strings.HasPrefix(relPath, "..") {
    // ...
}
```

**Issues:**
1. Errors from `filepath.Rel` are silently ignored
2. No validation that `Workspace` is actually a directory
3. No handling of empty `Workspace` or `Path` (will cause errors)
4. Potential issue with symbolic links (not resolved before comparison)

**Edge Cases:**
- Empty workspace: `filepath.Rel("", "/path")` returns error
- Same paths: `filepath.Rel("/a", "/a")` returns "."
- Network paths on Windows: May fail

**Recommendation:**
```go
if e.Workspace != "" && e.Path != "" {
    relPath, err := filepath.Rel(e.Workspace, e.Path)
    if err == nil && !strings.HasPrefix(relPath, "..") && relPath != "." {
        builder.WriteString(fmt.Sprintf(" [relative: %s]", relPath))
    }
}
```

### 5.5 IsRetryable ConnectionError Check

**Severity:** Medium
**Lines:** 519-522

The function assumes ALL connection errors are retryable:

```go
var connErr *ConnectionError
if AsError(err, &connErr) {
    return true // Connection errors are typically retryable
}
```

**Problem:** This is not always true. Some connection errors are permanent:
- DNS lookup failure for invalid domain
- Refused connections to blocked IPs
- TLS certificate validation failures
- Authentication failures

**Recommendation:** Add a field to `ConnectionError` to indicate if it's retryable, or implement more sophisticated detection logic.

### 5.6 ToolExecutionError Stderr Truncation

**Severity:** Low
**Lines:** 442-448

The stderr truncation logic is simplistic:

```go
if len(stderr) > 200 {
    stderr = stderr[:200] + "..."
}
```

**Issues:**
1. Truncates in the middle of UTF-8 characters (potential corruption)
2. 200 bytes is hardcoded (should be a constant)
3. No consideration for line boundaries (could cut mid-line)
4. The "..." suffix makes the total length 203, not 200

**Recommendation:**
```go
const maxStderrLength = 200
if len(stderr) > maxStderrLength {
    // Find the last valid UTF-8 boundary
    stderr = stderr[:maxStderrLength]
    for len(stderr) > 0 && stderr[len(stderr)-1] >= 0x80 {
        stderr = stderr[:len(stderr)-1]
    }
    stderr = stderr + "..."
}
```

---

## 6. Documentation Issues

### 6.1 Package-Level Documentation

**Severity:** Low
**Lines:** 1

The file lacks package-level documentation. While `errors.go` has it (lines 1-4), `helpers.go` has no comment explaining its purpose relative to `errors.go`.

**Recommendation:** Add a file-level comment explaining the relationship:
```go
// File helpers.go provides domain-specific error types for file operations,
// validation, configuration, paths, APIs, and tool execution. These complement
// the core error types in errors.go.
```

### 6.2 Undocumented Behavior in NewFileError

**Severity:** Medium
**Lines:** 101-124

The `NewFileError` function claims to have "automatic type detection" but doesn't document:
1. Which error types it can actually detect (only 3 of 8)
2. What happens for undetectable types (sets generic message)
3. That it wraps the original error for unwrapping

### 6.3 Missing Field Documentation

**Severity:** Low
**Lines:** 34-40, 167-172, etc.

Some struct fields lack comments explaining their purpose:

```go
type FileError struct {
    Type      FileErrorType
    Path      string
    Operation string // e.g., "read", "write", "delete"  <-- Only this field has a comment
    Message   string
    Err       error
}
```

**Recommendation:** Document all fields, especially non-obvious ones like `Message` vs `Err`.

### 6.4 NewBinaryFileError Parameter Documentation

**Severity:** Low
**Lines:** 156-164

The `size` parameter is used only for formatting the error message, but this isn't documented. Users might expect it to be validated or stored.

---

## 7. Security Concerns

### 7.1 Path Traversal Detection is Basic

**Severity:** Medium
**Lines:** 321-329

`NewPathTraversalError` accepts raw path strings without validation:

```go
func NewPathTraversalError(path, workspace string) *PathError {
    return &PathError{
        Path:         path,
        Workspace:    workspace,
        Reason:       "path traversal detected",
        SecurityNote: "attempting to access files outside the workspace is not allowed",
    }
}
```

**Issues:**
1. No actual validation that traversal occurred (trusts caller)
2. Paths are not cleaned or resolved before storage
3. Could leak sensitive path information in error messages
4. No validation that `workspace` is absolute

**Recommendation:** Add helper function:
```go
func ValidatePathInWorkspace(path, workspace string) error {
    cleanPath := filepath.Clean(path)
    cleanWorkspace := filepath.Clean(workspace)

    absPath, err := filepath.Abs(cleanPath)
    if err != nil {
        return NewInvalidPathError(path, "validate", err.Error())
    }

    absWorkspace, err := filepath.Abs(cleanWorkspace)
    if err != nil {
        return NewInvalidPathError(workspace, "validate", err.Error())
    }

    relPath, err := filepath.Rel(absWorkspace, absPath)
    if err != nil || strings.HasPrefix(relPath, "..") {
        return NewPathTraversalError(path, workspace)
    }

    return nil
}
```

### 7.2 Sensitive Information in Error Messages

**Severity:** Medium
**Lines:** Multiple

Error messages may leak sensitive information:

1. **Full file paths** (line 54): Could reveal directory structure
2. **API request IDs** (lines 369-371): Could be used for correlation attacks
3. **Stderr output** (lines 442-448): Could contain secrets, passwords, tokens
4. **Configuration paths** (lines 232-234): Could reveal deployment details

**Recommendation:**
- Add an option to redact sensitive information in production
- Provide sanitized error messages for logging vs user display
- Document which error types may contain sensitive data
- Consider adding a `Sanitize()` method to each error type

### 7.3 NewSensitivePathError Lacks Validation

**Severity:** Low
**Lines:** 331-339

Similar to path traversal, this function doesn't validate that the path is actually sensitive:

```go
func NewSensitivePathError(path, operation string) *PathError {
    return &PathError{
        Path:         path,
        Operation:    operation,
        Reason:       "sensitive system path",
        SecurityNote: "modifying system paths is not allowed for security reasons",
    }
}
```

**Recommendation:** Add a companion function to check if a path is sensitive:
```go
var sensitivePaths = []string{
    "/etc", "/sys", "/proc", "/dev",
    "/boot", "/root", "C:\\Windows", "C:\\System32",
}

func IsSensitivePath(path string) bool {
    // Implementation to check against known sensitive paths
}
```

### 7.4 WrapWithContextf Format String Injection

**Severity:** Low
**Lines:** 482-489

The `WrapWithContextf` function is vulnerable to format string bugs if used incorrectly:

```go
func WrapWithContextf(err error, format string, args ...interface{}) error {
    if err == nil {
        return nil
    }
    context := fmt.Sprintf(format, args...)
    return fmt.Errorf("%s: %w", context, err)
}
```

If a caller passes user input as the format string, it could cause crashes or unexpected behavior.

**Recommendation:** Document that `format` must be a compile-time constant, not user input. Consider adding a lint rule to enforce this.

---

## 8. Additional Observations

### 8.1 Good Practices Observed

1. **Error Unwrapping:** All error types properly implement `Unwrap()` method (lines 78-80, 197-199, etc.)
2. **Builder Pattern:** Uses `strings.Builder` efficiently for constructing error messages
3. **Type Safety:** Uses typed enums instead of strings for error categories
4. **Helpful Suggestions:** Provides actionable suggestions to users
5. **Comprehensive Testing:** 623 lines of test code with good coverage

### 8.2 Performance Considerations

**Severity:** Low

1. **String Allocations:** Excessive `fmt.Sprintf` calls could be optimized
2. **Builder Capacity:** `strings.Builder` doesn't pre-allocate capacity (could use `Grow()`)
3. **Filepath.Rel Calls:** Called unconditionally in hot path (line 296)

**Recommendation for hot paths:**
```go
var builder strings.Builder
builder.Grow(256) // Pre-allocate reasonable capacity
```

### 8.3 Consistency with errors.go

The file is part of a two-file error system. Cross-checking with `errors.go`:

1. **ConnectionError** is defined in `errors.go` (lines 48-68) but used in `helpers.go` (line 519)
2. **ErrTimeout** is defined in `errors.go` (line 21) but checked in `helpers.go` (line 525)
3. Good separation of concerns: Core errors in `errors.go`, domain-specific in `helpers.go`

---

## 9. Recommendations Summary

### High Priority

1. **Fix NewFileError type detection** - Detect more error types or document limitations
2. **Add missing FileError constructors** - Complete the API surface for all error types
3. **Review IsRetryable logic** - Not all connection errors are retryable
4. **Validate security error constructors** - Actually check for path traversal and sensitive paths

### Medium Priority

5. **Add edge case tests** - Empty values, unicode, concurrent access
6. **Document error field meanings** - Especially Message vs Err distinction
7. **Improve PathError relative path handling** - Handle edge cases properly
8. **Sanitize sensitive information** - Provide methods to redact secrets from errors

### Low Priority

9. **Consistent error message formatting** - Establish capitalization rules
10. **Optimize string building** - Pre-allocate builder capacity, reduce Sprintf calls
11. **Add package-level documentation** - Explain relationship with errors.go
12. **Extract magic numbers** - Make constants reusable
13. **Fix UTF-8 truncation** - Don't cut in the middle of characters

---

## 10. Conclusion

The `helpers.go` file provides a solid foundation for error handling with good structure and test coverage. However, there are notable gaps in functionality (missing constructors, incomplete type detection), potential bugs (path handling, retryability detection), and security concerns (information leakage, validation).

**Recommended Actions:**

1. Complete the missing constructor functions (1-2 hours)
2. Enhance `NewFileError` type detection (2-3 hours)
3. Fix edge cases in path handling and error detection (3-4 hours)
4. Add security validation to path-related constructors (2-3 hours)
5. Write additional tests for edge cases (4-5 hours)
6. Improve documentation throughout (1-2 hours)

**Estimated Total Effort:** 13-19 hours to address all issues

**Risk Assessment:**
- Current bugs: Low-Medium risk (mostly edge cases, unlikely to hit in normal use)
- Missing features: Medium impact (developers will work around inconsistencies)
- Security issues: Medium risk (information disclosure, not remote code execution)
- Test coverage gaps: Low risk (existing tests cover common paths well)
