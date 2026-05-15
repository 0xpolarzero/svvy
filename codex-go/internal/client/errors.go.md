# Code Review: errors.go

**File**: `/Users/williamcory/codex/codex-go/internal/client/errors.go`
**Reviewed**: 2025-10-26
**Lines of Code**: 352

## Executive Summary

This file implements a comprehensive error handling system for the Codex Go client, mirroring a Rust implementation's error types. The code is generally well-structured with clear error types, good formatting helpers, and meaningful error messages. However, there are several areas requiring attention:

- **Critical**: No unit tests exist for this file
- **High**: Hard-coded OpenAI URLs in error messages (incorrect for a generic client)
- **Medium**: Missing input validation in several constructors
- **Medium**: Incomplete error type implementations
- **Low**: Minor code quality improvements needed

## 1. Incomplete Features & Functionality

### 1.1 Missing Error Type Implementations

**Severity**: Medium
**Location**: Throughout the file

Several error types are missing standard Go error interface implementations that would improve usability:

1. **Missing `Unwrap()` implementations**: Only `ConnectionError` implements `Unwrap()`. Consider implementing for other errors that wrap underlying causes.

2. **Missing `Is()` and `As()` implementations**: None of the error types implement these methods, which are useful for error comparison with `errors.Is()` and `errors.As()`.

3. **Missing `Temporary()` and `Timeout()` implementations**: For errors like `ConnectionError`, `StreamError`, and `IdleTimeoutError`, these interfaces could help callers determine retry strategies.

### 1.2 Incomplete Constructor Functions

**Severity**: Medium
**Location**: Lines 214-286

Several constructor functions don't allow setting all fields:

```go
// NewStreamError only sets Message, but RequestID field is never set
func NewStreamError(message string) *StreamError {
    return &StreamError{Message: message}
}

// NewContextWindowExceededError doesn't allow setting TokenCount/MaxTokens
func NewContextWindowExceededError() *ContextWindowExceededError {
    return &ContextWindowExceededError{}
}
```

**Recommendation**: Add additional constructors or use the functional options pattern:
```go
func NewContextWindowExceededErrorWithDetails(tokenCount, maxTokens int64) *ContextWindowExceededError
func NewStreamErrorWithRequestID(message, requestID string) *StreamError
```

### 1.3 UsageLimitError RateLimits Field Never Used

**Severity**: Low
**Location**: Line 114

The `RateLimits` field in `UsageLimitError` is defined but never used in the error message formatting or constructors.

```go
type UsageLimitError struct {
    // ...
    RateLimits *RateLimitSnapshot  // Never read or displayed
}
```

**Recommendation**: Either use this field in the error message or remove it if not needed.

## 2. TODO Comments & Technical Debt

**Severity**: None
**Status**: ✓ Clean

No TODO, FIXME, XXX, HACK, or BUG comments found in the file. This is excellent code hygiene.

## 3. Code Quality Issues

### 3.1 Hard-coded OpenAI URLs

**Severity**: High
**Location**: Lines 122, 125

The error messages contain hard-coded OpenAI URLs, which is problematic for a generic client that claims to support "OpenAI-compatible APIs":

```go
case "free":
    return baseMsg + ". Upgrade to Plus to continue using Codex (https://openai.com/chatgpt/pricing)"
case "plus":
    suffix := formatResetSuffix(e.ResetsAt, " or try again")
    return fmt.Sprintf("%s. Upgrade to Pro (https://openai.com/chatgpt/pricing)%s", baseMsg, suffix)
```

**Issues**:
1. Assumes the user is using OpenAI's service
2. Not accurate for Anthropic Claude, Azure OpenAI, or other providers
3. "Codex" is an OpenAI product name, creating confusion
4. Links may become outdated or incorrect

**Recommendation**:
- Remove hard-coded URLs or make them configurable per provider
- Use generic upgrade messaging
- Consider adding a `Provider` field to `UsageLimitError` to customize messages

### 3.2 String Concatenation in Error Messages

**Severity**: Low
**Location**: Lines 46-47, 344-349

Using `+=` for string concatenation is less efficient than using `strings.Builder` or `fmt.Sprintf`:

```go
// Current approach (line 46)
msg := fmt.Sprintf("connection failed to %s: %v", e.URL, e.Cause)
msg += ". Check your network connection and verify the server is accessible"

// Better approach
msg := fmt.Sprintf("connection failed to %s: %v. Check your network connection and verify the server is accessible",
    e.URL, e.Cause)
```

For the `formatDuration` function (lines 344-349), consider using `strings.Join()`:

```go
// Instead of manual concatenation
for i, part := range parts {
    if i > 0 {
        result += " "
    }
    result += part
}

// Use strings.Join
return strings.Join(parts, " ")
```

### 3.3 Magic Numbers

**Severity**: Low
**Location**: Lines 75-76, 311-314

Magic numbers should be extracted as constants:

```go
// Line 75-76
if len(body) > 500 {
    body = body[:500] + "..."
}

// Should be:
const maxErrorBodyLength = 500
if len(body) > maxErrorBodyLength {
    body = body[:maxErrorBodyLength] + "..."
}

// Lines 311-314
totalSecs := int(d.Seconds())
days := totalSecs / 86400      // Magic number: seconds in a day
hours := (totalSecs % 86400) / 3600  // Magic number: seconds in an hour
minutes := (totalSecs % 3600) / 60   // Magic number: seconds in a minute

// Should use constants:
const (
    secondsPerMinute = 60
    secondsPerHour   = 3600
    secondsPerDay    = 86400
)
```

### 3.4 Inconsistent Constructor Naming

**Severity**: Low
**Location**: Lines 214-286

Most constructors follow the pattern `New<ErrorType>`, but there's also `NewStreamErrorWithRetry`. Consider a more consistent approach:

```go
// Current:
func NewStreamError(message string) *StreamError
func NewStreamErrorWithRetry(message string, retryAfter time.Duration) *StreamError

// Better naming:
func NewStreamError(message string) *StreamError
func NewStreamErrorWithRetryAfter(message string, retryAfter time.Duration) *StreamError
```

### 3.5 Potential Nil Pointer Dereference

**Severity**: Low
**Location**: Line 29

While the nil check exists for `e.RetryAfter`, dereferencing without additional checks could be clearer:

```go
if e.RetryAfter != nil {
    msg += fmt.Sprintf(" (retry after: %v)", *e.RetryAfter)
}
```

This is actually safe, but the pattern could be more defensive. Consider storing the duration directly instead of a pointer, using 0 as "not set".

## 4. Missing Test Coverage

**Severity**: Critical
**Location**: Entire file

**Issue**: No dedicated test file (`errors_test.go`) exists for this file. While integration tests in `/Users/williamcory/codex/codex-go/internal/client/openai/openai_test.go` verify error type assertions, there are no unit tests for:

1. **Error message formatting**: Each `Error()` method should be tested
2. **Constructor functions**: All `New*` functions need tests
3. **Helper functions**: `formatResetSuffix()` and `formatDuration()` are untested
4. **Edge cases**: Empty strings, nil values, zero durations, etc.

### 4.1 Recommended Test Cases

```go
// Minimum test coverage needed:

func TestStreamError_Error(t *testing.T) {
    // Test basic message
    // Test with RequestID
    // Test with RetryAfter
    // Test with all fields populated
}

func TestConnectionError_Error(t *testing.T) {
    // Test basic message
    // Test Unwrap functionality
}

func TestUnexpectedStatusError_Error(t *testing.T) {
    // Test basic message
    // Test with RequestID
    // Test body truncation at 500 chars
    // Test empty body
}

func TestUsageLimitError_Error(t *testing.T) {
    // Test each plan type: free, plus, team, business, pro, enterprise, edu, default
    // Test with nil ResetsAt
    // Test with future ResetsAt
    // Test with past ResetsAt
}

func TestContextWindowExceededError_Error(t *testing.T) {
    // Test with zero TokenCount
    // Test with valid TokenCount and MaxTokens
}

func TestFormatDuration(t *testing.T) {
    // Test < 1 minute
    // Test minutes only
    // Test hours only
    // Test days only
    // Test combinations: days+hours, hours+minutes, days+hours+minutes
    // Test edge cases: exactly 1 day, exactly 1 hour, exactly 1 minute
}

func TestFormatResetSuffix(t *testing.T) {
    // Test nil time
    // Test past time
    // Test future time
    // Test different prefixes
}

func TestConstructors(t *testing.T) {
    // Test each New* function creates correct error type with correct fields
}
```

### 4.2 Test Coverage Metrics

Run the following to check current coverage:

```bash
go test -coverprofile=coverage.out ./internal/client/
go tool cover -func=coverage.out | grep errors.go
```

**Expected result**: 0% coverage for `errors.go` (needs verification)

## 5. Potential Bugs & Edge Cases

### 5.1 Time-related Edge Cases

**Severity**: Medium
**Location**: Lines 291-303, 306-351

Several issues with time handling:

1. **Race condition in `NewIdleTimeoutError`** (line 268):
```go
func NewIdleTimeoutError(timeout time.Duration) *IdleTimeoutError {
    return &IdleTimeoutError{
        Timeout:       timeout,
        LastEventTime: time.Now(), // This timestamp may not be accurate
    }
}
```
The constructor sets `LastEventTime` to "now", but the caller might want to specify when the last event actually occurred.

2. **`formatDuration` doesn't handle negative durations** (line 306):
```go
func formatDuration(d time.Duration) string {
    if d < time.Minute {
        return "less than a minute"
    }
    // No check for negative duration
    totalSecs := int(d.Seconds())
```
If a negative duration is passed, it could produce confusing output.

3. **Integer overflow in duration calculation** (line 311):
```go
totalSecs := int(d.Seconds()) // Could overflow for very large durations
```

**Recommendations**:
```go
func NewIdleTimeoutError(timeout time.Duration, lastEventTime time.Time) *IdleTimeoutError
func formatDuration(d time.Duration) string {
    if d < 0 {
        return "invalid duration"
    }
    // ... rest of function
}
```

### 5.2 String Truncation Edge Case

**Severity**: Low
**Location**: Lines 75-76

UTF-8 string truncation could split a multi-byte character:

```go
if len(body) > 500 {
    body = body[:500] + "..."  // Could split UTF-8 character
}
```

**Recommendation**: Use a UTF-8 aware truncation method or ensure truncation at rune boundaries:

```go
if len(body) > maxErrorBodyLength {
    runes := []rune(body)
    if len(runes) > maxErrorBodyLength {
        body = string(runes[:maxErrorBodyLength]) + "..."
    }
}
```

However, this has performance implications. Since error bodies are typically ASCII JSON, the current implementation may be acceptable.

### 5.3 Missing Validation in Constructors

**Severity**: Low
**Location**: Lines 214-286

No validation is performed in constructor functions:

```go
func NewRetryLimitError(statusCode, attempts int) *RetryLimitError {
    // No validation: attempts could be negative, statusCode could be invalid
    return &RetryLimitError{
        StatusCode: statusCode,
        Attempts:   attempts,
    }
}
```

**Recommendation**: Add basic validation:
```go
func NewRetryLimitError(statusCode, attempts int) *RetryLimitError {
    if attempts < 0 {
        attempts = 0
    }
    if statusCode < 100 || statusCode > 599 {
        statusCode = 0 // or panic
    }
    return &RetryLimitError{
        StatusCode: statusCode,
        Attempts:   attempts,
    }
}
```

### 5.4 Pluralization Edge Cases

**Severity**: Low
**Location**: Lines 318-336

The pluralization logic works for English but could be more robust:

```go
unit := "day"
if days > 1 {
    unit = "days"
}
```

This doesn't handle the case where `days == 0` (which shouldn't occur in practice, but defensively should be considered).

## 6. Documentation Issues

### 6.1 Missing Package-level Documentation

**Severity**: Low
**Location**: Line 1

The file has a comment about error types but no formal package documentation. Consider adding:

```go
// Package client provides error types for the Codex API client.
// These error types mirror the Rust implementation's error handling
// and provide rich context for error scenarios including retries,
// rate limits, and network failures.
package client
```

### 6.2 Incomplete Struct Field Documentation

**Severity**: Low
**Location**: Various structs

Some fields lack documentation:

```go
type IdleTimeoutError struct {
    Timeout time.Duration      // Documented
    LastEventTime time.Time    // Not documented
}
```

**Recommendation**: Add documentation for all fields:
```go
type IdleTimeoutError struct {
    // Timeout is the configured idle timeout duration
    Timeout time.Duration

    // LastEventTime records when the last stream event was received
    LastEventTime time.Time
}
```

### 6.3 Missing Usage Examples

**Severity**: Low
**Location**: Throughout file

The file lacks usage examples. Consider adding examples using Go's example test format:

```go
func ExampleStreamError() {
    err := NewStreamError("connection lost")
    fmt.Println(err)
    // Output: stream error: connection lost
}
```

### 6.4 Incomplete Error Type Documentation

**Severity**: Low
**Location**: Lines 156-184

Some error types have minimal documentation:

```go
// IdleTimeoutError indicates the stream went silent for too long.
type IdleTimeoutError struct {
```

Could be enhanced:
```go
// IdleTimeoutError indicates the stream went silent for too long.
// This error occurs when no events are received within the configured
// idle timeout period. It suggests a stalled connection that should be
// retried or abandoned.
type IdleTimeoutError struct {
```

## 7. Security Concerns

### 7.1 Sensitive Information Disclosure

**Severity**: Medium
**Location**: Lines 67-80

Error messages include response bodies that may contain sensitive information:

```go
func (e *UnexpectedStatusError) Error() string {
    msg := fmt.Sprintf("unexpected status %d", e.StatusCode)
    if e.RequestID != "" {
        msg += fmt.Sprintf(" (request_id: %s)", e.RequestID)
    }
    if e.Body != "" {
        // Truncate long error bodies
        body := e.Body
        if len(body) > 500 {
            body = body[:500] + "..."
        }
        msg += fmt.Sprintf(": %s", body)  // May contain sensitive data
    }
    return msg
}
```

**Risk**: Error bodies from API responses may contain:
- API keys or tokens (if echoed back)
- User data
- Internal system information
- PII (Personally Identifiable Information)

**Recommendation**:
1. Sanitize error bodies before including in error messages
2. Add a flag to control whether full error details are included
3. Log full details separately with proper security controls
4. Consider parsing JSON error bodies and only including safe fields

Example:
```go
func sanitizeErrorBody(body string) string {
    // Parse JSON and extract only safe fields like "error" and "message"
    // Redact any fields that might contain sensitive data
}
```

### 7.2 RequestID Exposure

**Severity**: Low
**Location**: Various error types

Request IDs are included in error messages and could potentially be used for reconnaissance or correlation attacks. However, this is generally considered acceptable as request IDs are designed to be shared for debugging.

**Recommendation**: Document that request IDs will be exposed in error messages and ensure they don't contain sensitive information.

### 7.3 URL Exposure in ConnectionError

**Severity**: Low
**Location**: Lines 44-47

URLs are included in error messages, which could expose:
- Internal hostnames
- IP addresses
- Port numbers
- Path information

```go
func (e *ConnectionError) Error() string {
    msg := fmt.Sprintf("connection failed to %s: %v", e.URL, e.Cause)
    // ...
}
```

**Recommendation**: Consider sanitizing URLs to remove sensitive query parameters or credentials:
```go
import "net/url"

func sanitizeURL(rawURL string) string {
    u, err := url.Parse(rawURL)
    if err != nil {
        return "[invalid URL]"
    }
    u.User = nil  // Remove credentials
    u.RawQuery = ""  // Remove query parameters
    return u.String()
}
```

## 8. Performance Considerations

### 8.1 Repeated String Allocations

**Severity**: Low
**Location**: Lines 23-31, 344-349

Multiple string concatenations cause repeated allocations:

```go
msg := fmt.Sprintf("stream error: %s", e.Message)
if e.RequestID != "" {
    msg += fmt.Sprintf(" (request_id: %s)", e.RequestID)
}
if e.RetryAfter != nil {
    msg += fmt.Sprintf(" (retry after: %v)", *e.RetryAfter)
}
```

**Recommendation**: Use `strings.Builder` for better performance:

```go
var builder strings.Builder
builder.WriteString("stream error: ")
builder.WriteString(e.Message)
if e.RequestID != "" {
    builder.WriteString(" (request_id: ")
    builder.WriteString(e.RequestID)
    builder.WriteString(")")
}
// ... etc
return builder.String()
```

However, since error formatting is not typically performance-critical, the current implementation is acceptable.

## 9. Dependency Analysis

### 9.1 Minimal Dependencies

**Status**: ✓ Excellent

The file only imports standard library packages:
- `fmt` - formatting
- `time` - time handling

This is ideal for a core error handling package as it minimizes dependency risks and improves compilation time.

### 9.2 Missing Dependency on RateLimitSnapshot

**Severity**: Low
**Location**: Line 114

The `RateLimitSnapshot` type is referenced but defined in another file (`client.go`). This creates a coupling that could be documented:

```go
// RateLimits contains current rate limit state
// See RateLimitSnapshot in client.go for details
RateLimits *RateLimitSnapshot
```

## 10. Recommendations Summary

### Critical Priority

1. **Add comprehensive unit tests** (Section 4)
   - Target: 100% coverage for error formatting
   - Estimate: 2-4 hours of work

2. **Fix hard-coded OpenAI URLs** (Section 3.1)
   - Remove or make configurable
   - Estimate: 30 minutes

### High Priority

3. **Implement complete error constructors** (Section 1.2)
   - Add missing constructor variants
   - Estimate: 1 hour

4. **Sanitize error bodies for security** (Section 7.1)
   - Prevent sensitive data exposure
   - Estimate: 2 hours

### Medium Priority

5. **Add `Unwrap()`, `Is()`, `As()` implementations** (Section 1.1)
   - Improve error handling ergonomics
   - Estimate: 1 hour

6. **Fix time-related edge cases** (Section 5.1)
   - Handle negative durations
   - Fix LastEventTime in constructor
   - Estimate: 30 minutes

7. **Add input validation to constructors** (Section 5.3)
   - Validate status codes, attempt counts
   - Estimate: 1 hour

### Low Priority

8. **Extract magic numbers to constants** (Section 3.3)
   - Improve code readability
   - Estimate: 15 minutes

9. **Use `strings.Join()` in formatDuration** (Section 3.2)
   - Minor optimization
   - Estimate: 5 minutes

10. **Improve documentation** (Section 6)
    - Add package docs
    - Complete field documentation
    - Add usage examples
    - Estimate: 1 hour

11. **Fix UTF-8 truncation** (Section 5.2)
    - Use rune-aware truncation
    - Estimate: 15 minutes (if needed)

## 11. Positive Aspects

Despite the issues identified, the code has several strengths:

1. **Clean architecture**: Error types are well-organized and follow Go conventions
2. **Good naming**: Type and function names are clear and descriptive
3. **Helpful error messages**: Error messages provide actionable guidance to users
4. **No technical debt markers**: No TODO/FIXME comments
5. **Minimal dependencies**: Only uses standard library
6. **Rust compatibility**: Maintains parity with Rust implementation as designed
7. **Rich context**: Errors include request IDs, retry suggestions, and timing information

## 12. Overall Assessment

**Code Quality**: 7/10
**Test Coverage**: 2/10 (critical gap)
**Documentation**: 6/10
**Security**: 6/10
**Maintainability**: 8/10

**Overall Score**: 6.5/10

The error handling implementation is solid and well-structured, but the lack of unit tests is a critical gap that must be addressed. The hard-coded OpenAI URLs are also problematic for a supposedly generic client. Once tests are added and the high-priority issues are resolved, this would be an 8.5/10 implementation.

## 13. Action Items

- [ ] Create `errors_test.go` with comprehensive test coverage
- [ ] Remove hard-coded OpenAI URLs or make them configurable
- [ ] Add missing error constructor variants
- [ ] Implement error sanitization for security
- [ ] Add `Unwrap()`, `Is()`, `As()` methods to appropriate error types
- [ ] Fix time-related edge cases in constructors and formatters
- [ ] Add input validation to all constructors
- [ ] Extract magic numbers to named constants
- [ ] Improve documentation with examples and complete field docs
- [ ] Consider whether `RateLimits` field in `UsageLimitError` is needed

## 14. Questions for Code Owner

1. Is the `RateLimits` field in `UsageLimitError` intended to be used? If so, how should it be displayed?
2. Should error bodies be sanitized before inclusion in error messages for security?
3. Is the hard-coded OpenAI URL intentional, or should this be provider-agnostic?
4. Should `LastEventTime` in `IdleTimeoutError` be set by the caller or constructor?
5. What is the target test coverage percentage for this codebase?
6. Should errors implement the `Temporary()` and `Timeout()` interfaces for retry logic?
