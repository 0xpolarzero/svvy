# Code Review: errors.go

**File:** `/Users/williamcory/codex/codex-go/internal/errors/errors.go`
**Review Date:** 2025-10-26
**Test Coverage:** 75.6% overall

---

## Executive Summary

The `errors.go` file provides error types and utilities for the Codex Go project, implementing error types that match `codex-rs/core/src/error.rs` and following Go 1.13+ error wrapping idioms. The code is generally well-structured with good test coverage, but there are several areas requiring attention including incomplete test coverage for critical functions, potential edge case handling issues, and some code quality improvements.

**Overall Assessment:** Good foundation with room for improvement
**Priority Issues:** 3 High, 5 Medium, 4 Low

---

## 1. Incomplete Features or Functionality

### High Priority

#### 1.1 Missing Unwrap for SandboxError
**Location:** Line 88-110
**Issue:** `SandboxError` does not implement the `Unwrap()` method, breaking the error chain pattern used by other custom error types.

**Current Code:**
```go
type SandboxError struct {
    Type     SandboxErrorType
    ExitCode int
    Stdout   string
    Stderr   string
    Duration time.Duration
    Signal   int
}
```

**Impact:** Cannot use `errors.Is()` or `errors.As()` to unwrap a potential underlying error. This breaks consistency with other error types in the package.

**Recommendation:** Consider if `SandboxError` should have an `Err` field and implement `Unwrap()` method for consistency with other error types.

---

#### 1.2 Missing Unwrap for UnexpectedStatusError
**Location:** Line 118-131
**Issue:** `UnexpectedStatusError` lacks an `Unwrap()` method and underlying error field.

**Current Code:**
```go
type UnexpectedStatusError struct {
    Status    int
    Body      string
    RequestID string
}
```

**Impact:** Cannot wrap underlying errors (e.g., network errors that caused the unexpected status).

**Recommendation:** Add an `Err error` field and implement `Unwrap()` method.

---

#### 1.3 Missing Unwrap for RetryLimitError
**Location:** Line 151-163
**Issue:** Similar to above, lacks error wrapping capability.

**Recommendation:** Add underlying error support for better error context preservation.

---

### Medium Priority

#### 1.4 StreamError Retryability Not Exposed
**Location:** Line 276-292
**Issue:** `StreamError` has a `RetryDelay` field but no mechanism to indicate if the error is retryable. The `IsRetryable()` function in helpers.go doesn't check for StreamError.

**Current Code:**
```go
type StreamError struct {
    Message    string
    RetryDelay time.Duration
}
```

**Recommendation:** Add a `CanRetry bool` field and update `IsRetryable()` to check for StreamError.

---

#### 1.5 ConversationNotFoundError Doesn't Implement Is Pattern
**Location:** Line 294-306
**Issue:** `ConversationNotFoundError` could implement the `Is` method to match against `ErrNotFound` sentinel error for semantic error checking.

**Recommendation:** Implement `Is(error) bool` method:
```go
func (e *ConversationNotFoundError) Is(target error) bool {
    return target == ErrNotFound
}
```

---

## 2. TODO Comments and Technical Debt

### Status: Clean
No TODO, FIXME, HACK, XXX, BUG, or NOTE comments found in the codebase. This is positive and indicates good code maintenance practices.

---

## 3. Code Quality Issues

### Medium Priority

#### 3.1 Hardcoded User-Facing Messages in Sentinel Errors
**Location:** Lines 14-46
**Issue:** Error messages contain user-facing instructions with specific formatting (e.g., backticks, URLs) which may be inappropriate for programmatic error handling.

**Examples:**
```go
ErrInterrupted = errors.New("interrupted (Ctrl-C). Something went wrong? Hit `/feedback` to report the issue.")
ErrContextWindowExceeded = errors.New("Codex ran out of room in the model's context window. Start a new conversation or clear earlier history before retrying.")
ErrUsageNotIncluded = errors.New("To use Codex with your ChatGPT plan, upgrade to Plus: https://openai.com/chatgpt/pricing.")
```

**Issues:**
- Mixing error identification with user instructions
- Hard to localize/internationalize
- Cannot distinguish between error type and user message programmatically
- Makes testing more brittle

**Recommendation:** Separate error identification from user messaging:
```go
// Error identification only
ErrInterrupted = errors.New("interrupted")

// User messages handled at presentation layer
func FormatUserError(err error) string {
    if errors.Is(err, ErrInterrupted) {
        return "Interrupted (Ctrl-C). Something went wrong? Hit `/feedback` to report the issue."
    }
    // ... handle other errors
}
```

---

#### 3.2 SandboxError Switch Missing Validation
**Location:** Line 98-110
**Issue:** The `default` case returns a generic "sandbox error" which provides no useful information.

**Current Code:**
```go
func (e *SandboxError) Error() string {
    switch e.Type {
    case SandboxDenied:
        return fmt.Sprintf("sandbox denied exec error, exit code: %d, stdout: %s, stderr: %s",
            e.ExitCode, e.Stdout, e.Stderr)
    case SandboxTimeout:
        return fmt.Sprintf("command timed out after %v", e.Duration)
    case SandboxSignal:
        return fmt.Sprintf("command was killed by signal %d", e.Signal)
    default:
        return "sandbox error"
    }
}
```

**Issues:**
- Default case loses potentially useful information (ExitCode, Stdout, Stderr)
- No indication of invalid SandboxErrorType

**Recommendation:**
```go
default:
    return fmt.Sprintf("unknown sandbox error type (%d), exit code: %d, stdout: %s, stderr: %s",
        e.Type, e.ExitCode, e.Stdout, e.Stderr)
```

---

#### 3.3 formatDuration Logic Could Be Simplified
**Location:** Line 229-274
**Issue:** Complex string building logic with redundant checks.

**Current Code:**
```go
func formatDuration(d time.Duration) string {
    if d < time.Minute {
        return "less than a minute"
    }

    totalSecs := int(d.Seconds())
    days := totalSecs / 86400
    hours := (totalSecs % 86400) / 3600
    minutes := (totalSecs % 3600) / 60

    var parts []string
    if days > 0 {
        unit := "day"
        if days > 1 {
            unit = "days"
        }
        parts = append(parts, fmt.Sprintf("%d %s", days, unit))
    }
    if hours > 0 {
        unit := "hour"
        if hours > 1 {
            unit = "hours"
        }
        parts = append(parts, fmt.Sprintf("%d %s", hours, unit))
    }
    if minutes > 0 {
        unit := "minute"
        if minutes > 1 {
            unit = "minutes"
        }
        parts = append(parts, fmt.Sprintf("%d %s", minutes, unit))
    }

    if len(parts) == 0 {
        return "less than a minute"
    }

    result := ""
    for i, part := range parts {
        if i > 0 {
            result += " "
        }
        result += part
    }
    return result
}
```

**Issues:**
- Repetitive unit pluralization logic
- Manual string concatenation instead of using strings.Join
- Double-check for "less than a minute" (line 230-232 and 262-264)

**Recommendation:**
```go
func formatDuration(d time.Duration) string {
    if d < time.Minute {
        return "less than a minute"
    }

    totalSecs := int(d.Seconds())
    days := totalSecs / 86400
    hours := (totalSecs % 86400) / 3600
    minutes := (totalSecs % 3600) / 60

    var parts []string
    addPart := func(value int, singular string) {
        if value > 0 {
            unit := singular
            if value > 1 {
                unit += "s"
            }
            parts = append(parts, fmt.Sprintf("%d %s", value, unit))
        }
    }

    addPart(days, "day")
    addPart(hours, "hour")
    addPart(minutes, "minute")

    if len(parts) == 0 {
        return "less than a minute"
    }

    return strings.Join(parts, " ")
}
```

---

### Low Priority

#### 3.4 Magic Numbers in formatResetSuffix
**Location:** Line 214-226
**Issue:** Time comparison at line 220 checks `remaining <= 0` but could be more explicit.

**Recommendation:** Use named constants or more explicit logic:
```go
const immediateReset = 0

if remaining <= immediateReset {
    return prefix + " now"
}
```

---

#### 3.5 UsageLimitError Plan Types Not Enumerated
**Location:** Line 188-212
**Issue:** Plan types are strings without type safety. Typos would not be caught at compile time.

**Recommendation:** Consider using a custom type with constants:
```go
type PlanType string

const (
    PlanTypeFree       PlanType = "free"
    PlanTypePlus       PlanType = "plus"
    PlanTypePro        PlanType = "pro"
    PlanTypeTeam       PlanType = "team"
    PlanTypeBusiness   PlanType = "business"
    PlanTypeEnterprise PlanType = "enterprise"
    PlanTypeEdu        PlanType = "edu"
)

type UsageLimitError struct {
    PlanType PlanType
    ResetsAt time.Time
}
```

---

## 4. Missing Test Coverage

### High Priority

#### 4.1 formatDuration Function Has 0% Coverage
**Location:** Line 229-274
**Coverage:** 0.0%
**Issue:** Critical function for user-facing time formatting has no tests.

**Required Tests:**
- Test durations less than a minute
- Test durations with only minutes
- Test durations with hours and minutes
- Test durations with days, hours, and minutes
- Test edge cases (exactly 1 minute, exactly 1 hour, exactly 1 day)
- Test large durations (multiple days)

---

#### 4.2 Helper Functions Have 0% Coverage
**Location:** Lines 337-349
**Coverage:** 0.0%
**Functions:** `IsCancelled()`, `IsTimeout()`, `IsNotFound()`

**Required Tests:**
- Test each function with matching sentinel error
- Test each function with wrapped matching sentinel error
- Test each function with non-matching error
- Test each function with nil error

---

### Medium Priority

#### 4.3 formatResetSuffix Partial Coverage
**Location:** Line 214-226
**Coverage:** 71.4%
**Missing:** Tests for zero ResetsAt, negative remaining time edge cases

---

#### 4.4 UsageLimitError.Error() Partial Coverage
**Location:** Line 193-212
**Coverage:** 63.6%
**Missing:** Tests for all plan types (team, business, enterprise, edu)

---

#### 4.5 SandboxError.Error() Partial Coverage
**Location:** Line 98-110
**Coverage:** 80.0%
**Missing:** Test for the `default` case

---

#### 4.6 NewFileError Partial Coverage
**Location:** helpers.go line 102-124
**Coverage:** 66.7%
**Missing:** Test for unknown error types (not NotExist, Permission, or Exist)

---

## 5. Potential Bugs and Edge Cases

### High Priority

#### 5.1 formatDuration Integer Overflow Risk
**Location:** Line 234
**Issue:** Converting duration to int seconds could overflow on 32-bit systems with very large durations.

**Current Code:**
```go
totalSecs := int(d.Seconds())
```

**Risk:** On 32-bit systems, `int` is 32 bits and can hold ~68 years worth of seconds. Duration can represent up to 290 years.

**Recommendation:**
```go
totalSecs := int64(d.Seconds())
days := int(totalSecs / 86400)
hours := int((totalSecs % 86400) / 3600)
minutes := int((totalSecs % 3600) / 60)
```

---

### Medium Priority

#### 5.2 SandboxError Stdout/Stderr Could Be Very Large
**Location:** Line 89-95
**Issue:** No size limit on Stdout/Stderr fields which are directly interpolated into error messages.

**Current Code:**
```go
case SandboxDenied:
    return fmt.Sprintf("sandbox denied exec error, exit code: %d, stdout: %s, stderr: %s",
        e.ExitCode, e.Stdout, e.Stderr)
```

**Risk:** Could create extremely large error messages consuming memory and making logs unreadable.

**Recommendation:** Truncate large outputs:
```go
const maxOutputLength = 500

truncate := func(s string, max int) string {
    if len(s) <= max {
        return s
    }
    return s[:max] + "... (truncated)"
}

case SandboxDenied:
    return fmt.Sprintf("sandbox denied exec error, exit code: %d, stdout: %s, stderr: %s",
        e.ExitCode, truncate(e.Stdout, maxOutputLength), truncate(e.Stderr, maxOutputLength))
```

Note: helpers.go already implements truncation for ToolExecutionError.Stderr at 200 chars, but not for SandboxError.

---

#### 5.3 ConnectionError Context Field May Contain Sensitive Information
**Location:** Line 49-52
**Issue:** No guidance or validation on what should be in the Context field. Could accidentally include sensitive data.

**Recommendation:** Add documentation comment:
```go
// ConnectionError represents a connection failure
type ConnectionError struct {
    Err     error  // The underlying error
    Context string // Optional context about where the connection failed (avoid including sensitive data)
}
```

---

#### 5.4 formatDuration Doesn't Handle Negative Durations
**Location:** Line 229-274
**Issue:** No explicit handling of negative durations.

**Current Behavior:** Would return "less than a minute" for negative durations, which is misleading.

**Recommendation:** Add explicit check:
```go
func formatDuration(d time.Duration) string {
    if d < 0 {
        return "invalid duration (negative)"
    }
    if d < time.Minute {
        return "less than a minute"
    }
    // ... rest of function
}
```

---

#### 5.5 UsageLimitError Doesn't Validate PlanType
**Location:** Line 188-212
**Issue:** Invalid plan types fall through to default case with no indication the plan type is invalid.

**Recommendation:** Log or indicate when an unknown plan type is encountered:
```go
default:
    suffix := e.formatResetSuffix(". Try again")
    return fmt.Sprintf("%s (unknown plan: %q)%s.", baseMsg, e.PlanType, suffix)
```

---

### Low Priority

#### 5.6 Nil Error Check Missing in ConnectionError.Error()
**Location:** Line 54-59
**Issue:** If `e.Err` is nil, `%v` formatter will print "<nil>" which is not user-friendly.

**Recommendation:**
```go
func (e *ConnectionError) Error() string {
    if e.Err == nil {
        if e.Context != "" {
            return fmt.Sprintf("connection failed: %s", e.Context)
        }
        return "connection failed"
    }
    if e.Context != "" {
        return fmt.Sprintf("connection failed: %s: %v", e.Context, e.Err)
    }
    return fmt.Sprintf("connection failed: %v", e.Err)
}
```

---

#### 5.7 NewConnectionError Doesn't Validate Non-Nil Error
**Location:** Line 66-68
**Issue:** Allows creating ConnectionError with nil underlying error.

**Recommendation:** Add validation or documentation:
```go
// NewConnectionError creates a new ConnectionError.
// Panics if err is nil.
func NewConnectionError(err error) *ConnectionError {
    if err == nil {
        panic("errors: NewConnectionError called with nil error")
    }
    return &ConnectionError{Err: err}
}
```

---

## 6. Documentation Issues

### Medium Priority

#### 6.1 Missing Package-Level Examples
**Issue:** No example usage in package documentation or _example_test.go file.

**Recommendation:** Create `errors_example_test.go` with examples for common use cases:
```go
func ExampleNewConnectionError() {
    err := NewConnectionError(fmt.Errorf("dial tcp: connection refused"))
    fmt.Println(err)
    // Output: connection failed: dial tcp: connection refused
}

func ExampleUsageLimitError() {
    err := &UsageLimitError{
        PlanType: "free",
    }
    fmt.Println(err)
    // Output: you've hit your usage limit. Upgrade to Plus to continue using Codex (https://openai.com/chatgpt/pricing).
}
```

---

#### 6.2 Incomplete Field Documentation
**Issue:** Several struct fields lack documentation comments.

**Examples:**
- `SandboxError.Duration` - no comment explaining what duration this represents
- `SandboxError.Signal` - no comment explaining Unix signal numbers
- `UsageLimitError.ResetsAt` - could clarify timezone expectations

**Recommendation:** Add detailed field comments:
```go
type SandboxError struct {
    Type     SandboxErrorType
    ExitCode int
    Stdout   string
    Stderr   string
    Duration time.Duration // Time the command ran before timeout/signal
    Signal   int          // Unix signal number that killed the process
}
```

---

#### 6.3 No Error Hierarchy Documentation
**Issue:** No high-level documentation explaining error classification or when to use which error type.

**Recommendation:** Add detailed package documentation:
```go
// Package errors provides error types and utilities for Codex Go.
//
// Error Types:
//
// Sentinel Errors:
//   - ErrCancelled: Operation was cancelled by user or system
//   - ErrNotFound: Resource not found
//   - ErrTimeout: Operation timed out
//   - ErrInterrupted: User interrupted with Ctrl-C
//   ... etc
//
// Structured Errors:
//   - ConnectionError: Network/connection failures
//   - SandboxError: Sandbox execution failures
//   - UnexpectedStatusError: HTTP status code errors
//   ... etc
//
// Error Wrapping:
// All custom error types implement Unwrap() to support errors.Is and errors.As.
// Use WrapWithContext or WrapWithContextf to add context to errors.
//
// Example Usage:
//   if err := connectToAPI(); err != nil {
//       return NewConnectionError(err)
//   }
package errors
```

---

### Low Priority

#### 6.4 Missing Error Handling Best Practices Guide
**Issue:** No guidance on when to wrap vs. return sentinel errors.

**Recommendation:** Add to package docs or README:
- When to use sentinel errors vs. structured errors
- How to properly wrap errors while preserving the error chain
- When to use Is vs. As for error checking

---

#### 6.5 SandboxErrorType Constants Need Better Descriptions
**Location:** Line 79-86
**Issue:** Comments are minimal and don't explain when each type occurs.

**Current:**
```go
const (
    // SandboxDenied indicates the sandbox denied execution
    SandboxDenied SandboxErrorType = iota
    // SandboxTimeout indicates the command timed out
    SandboxTimeout
    // SandboxSignal indicates the command was killed by a signal
    SandboxSignal
)
```

**Recommendation:**
```go
const (
    // SandboxDenied indicates the sandbox denied execution, typically due to
    // security policy violations or missing permissions.
    SandboxDenied SandboxErrorType = iota

    // SandboxTimeout indicates the command exceeded the maximum execution time
    // allowed by the sandbox. Duration field contains the timeout period.
    SandboxTimeout

    // SandboxSignal indicates the command was killed by a Unix signal,
    // such as SIGKILL (9) or SIGTERM (15). Signal field contains the signal number.
    SandboxSignal
)
```

---

## 7. Security Concerns

### Medium Priority

#### 7.1 Potential Information Disclosure in Error Messages
**Location:** Multiple locations
**Issue:** Error messages may expose internal system details, paths, or URLs.

**Examples:**
```go
ErrUsageNotIncluded = errors.New("To use Codex with your ChatGPT plan, upgrade to Plus: https://openai.com/chatgpt/pricing.")
```

**Concerns:**
- Exposes internal product URLs
- May reveal pricing/licensing structure
- Could be used for reconnaissance

**Recommendation:**
- Review all error messages for information disclosure
- Consider different error messages for production vs. development
- Use error codes instead of descriptive messages for sensitive operations

---

#### 7.2 No Input Validation on Error Construction
**Location:** Multiple constructor functions
**Issue:** No validation that provided strings don't contain injection attacks or malicious content.

**Example Risk:**
```go
err := NewConversationNotFoundError(userInput) // userInput not validated
```

**Recommendation:** While error messages should generally be logged/displayed safely by the consumer, consider:
- Adding validation functions for untrusted input
- Documenting that callers must sanitize user input
- Consider length limits to prevent DoS via extremely long error messages

---

### Low Priority

#### 7.3 SandboxError Output Could Expose Sensitive Data
**Location:** Line 89-110
**Issue:** Stdout/Stderr from sandbox execution might contain sensitive data (API keys, tokens, etc.).

**Recommendation:**
- Add warning in documentation
- Consider providing a sanitization function
- Or truncate/redact sensitive patterns in output

---

## 8. Recommendations Summary

### Immediate Actions (High Priority)
1. **Add test coverage** for `formatDuration()`, `IsCancelled()`, `IsTimeout()`, `IsNotFound()`
2. **Fix integer overflow** in `formatDuration()` on 32-bit systems
3. **Add Unwrap methods** to `SandboxError`, `UnexpectedStatusError`, `RetryLimitError`
4. **Implement output truncation** for `SandboxError` Stdout/Stderr fields

### Short-term Improvements (Medium Priority)
5. **Separate error identification from user messages** for sentinel errors
6. **Add validation** to error constructors (nil checks, plan type validation)
7. **Improve test coverage** to reach 90%+ (currently 75.6%)
8. **Add package-level documentation** with examples
9. **Handle negative durations** explicitly in `formatDuration()`

### Long-term Enhancements (Low Priority)
10. **Create type-safe enums** for plan types and other string constants
11. **Add example tests** for common error patterns
12. **Create error handling guide** in documentation
13. **Consider internationalization** strategy for user-facing error messages
14. **Implement error code system** for better programmatic error handling

---

## 9. Test Coverage Analysis

### Current Coverage: 75.6%

#### Functions with 0% Coverage:
- `formatDuration()` - **CRITICAL**
- `IsCancelled()` - **HIGH PRIORITY**
- `IsTimeout()` - **HIGH PRIORITY**
- `IsNotFound()` - **HIGH PRIORITY**
- Multiple `Unwrap()` methods - **MEDIUM PRIORITY**

#### Functions with Partial Coverage:
- `SandboxError.Error()` - 80.0%
- `UsageLimitError.Error()` - 63.6%
- `formatResetSuffix()` - 71.4%
- `NewFileError()` - 66.7%
- `getSuggestion()` - 62.5%

#### Well-Tested Functions (100% Coverage):
- All constructor functions
- Most error type checking functions
- String formatting functions

### Coverage Goals:
- **Target:** 90%+ overall coverage
- **Minimum:** 100% coverage for public API functions
- **Focus:** User-facing formatting functions and error helpers

---

## 10. Code Metrics

- **Total Lines:** 350 (errors.go)
- **Error Types:** 12 custom types + 12 sentinel errors
- **Constructor Functions:** 10
- **Helper Functions:** 6
- **Test Files:** 2 (errors_test.go, helpers_test.go)
- **Test Functions:** 30+
- **Cyclomatic Complexity:** Low to Medium (highest in `formatDuration` and `UsageLimitError.Error()`)

---

## Conclusion

The `errors.go` file demonstrates solid engineering with good separation of concerns and comprehensive error types. The main issues are:

1. **Incomplete test coverage** for critical formatting functions
2. **Missing error wrapping** for some error types
3. **Potential edge cases** not handled (overflow, negative durations, large outputs)
4. **User-facing messages** mixed with error identification

The codebase would benefit from:
- Completing test coverage
- Adding error wrapping support to all error types
- Separating error identification from presentation
- Adding comprehensive documentation and examples

**Overall Grade: B+** (Good implementation with room for polish and completeness)
