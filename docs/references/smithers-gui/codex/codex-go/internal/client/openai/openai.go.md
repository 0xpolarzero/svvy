# Code Review: openai.go

**File:** `/Users/williamcory/codex/codex-go/internal/client/openai/openai.go`
**Date:** 2025-10-26
**Reviewer:** Claude Code Comprehensive Analysis

---

## Executive Summary

This OpenAI client implementation is generally well-structured with solid retry logic, rate limiting, and token refresh capabilities. However, there are several critical issues around resource leaks, error handling edge cases, incomplete features, and potential concurrency bugs that need to be addressed before production use.

**Overall Assessment:** 6.5/10 - Good foundation with significant issues that need addressing

---

## 1. Incomplete Features & Functionality

### 1.1 Missing Retry-After Header Support ❌ CRITICAL
**Location:** Lines 152-210 (completeWithRetry), Lines 282-352 (streamWithRetry)

**Issue:** The retry configuration supports `RespectRetryAfter: true` (see client.go:177), but the implementation never reads or respects the `Retry-After` HTTP header from API responses.

**Impact:**
- Ignores server-provided backoff hints, leading to premature retries
- Could result in cascading failures or IP bans
- Wastes client resources with ineffective retry timing

**Evidence:**
```go
// RetryConfig has RespectRetryAfter field
type RetryConfig struct {
    // ...
    RespectRetryAfter bool  // Line 177 in client.go
}

// But retry.go has unused parseRetryAfter function (lines 85-110)
func parseRetryAfter(retryAfterHeader string) *time.Duration { ... }
```

**Fix Required:** Extract and use `Retry-After` header from `httpResp.Headers` before calculating backoff.

---

### 1.2 Rate Limit Tracking Not Exposed ⚠️ MODERATE
**Location:** Lines 505-508

**Issue:** Rate limits are tracked internally but never exposed to consumers. The `rateLimitTracker` is updated but has no getter methods.

**Impact:**
- Consumers cannot make informed decisions about request pacing
- No way to display rate limit status to users
- Blind retry attempts without knowing limits

**Test Evidence:** Line 639 in openai_test.go explicitly notes this limitation:
```go
// Rate limits should be tracked internally
// This would require exposing rate limit state or adding a getter method
```

**Recommendation:** Add method like `GetRateLimitStatus() *RateLimitSnapshot` to expose current rate limit state.

---

### 1.3 Custom Header Validation Missing ⚠️ MODERATE
**Location:** Lines 443-446

**Issue:** Custom headers are merged without validation or sanitization. Reserved headers could be overwritten.

```go
// Add custom headers
for k, v := range c.config.Headers {
    headers[k] = v  // No validation!
}
```

**Risk:**
- Could override critical headers like `Authorization`, `Content-Type`
- No check for malformed header values
- Potential security issue if headers come from untrusted sources

**Fix Required:** Validate headers against a reserved list and sanitize values.

---

### 1.4 Connection Pool Metrics Detection Incomplete ⚠️ MODERATE
**Location:** Lines 192-200 (metrics.go)

**Issue:** Connection reuse detection uses naive timing heuristics that are unreliable.

```go
// Heuristic: Fast responses (<50ms) are likely connection reuses
// Slower responses may indicate new connection establishment
if err == nil {
    if elapsed < 50*time.Millisecond {
        t.metrics.TrackConnectionReuse()
    } else if elapsed > 100*time.Millisecond {
        t.metrics.TrackNewConnection()
    }
}
```

**Problems:**
- Fast new connections (cached DNS, local server) incorrectly counted as reuses
- Slow reused connections (high latency network) incorrectly counted as new
- Gap between 50-100ms results in uncounted connections
- Metrics are misleading and unreliable for decision-making

**Impact:** Connection pool statistics are inaccurate and cannot be trusted for tuning.

---

## 2. Code Quality Issues

### 2.1 Duplicate Token Refresh Logic 🔥 CRITICAL
**Location:** Lines 230-262 (doComplete) and Lines 372-399 (doStream)

**Issue:** Token refresh logic is duplicated in both methods with 28 nearly identical lines.

**Problems:**
- Violates DRY principle
- Bugs must be fixed in two places
- Inconsistent error handling between the two
- Harder to maintain and test

**Evidence:**
```go
// doComplete (lines 230-262)
if httpResp.StatusCode == http.StatusUnauthorized && c.config.TokenRefreshFunc != nil {
    // ... 32 lines of refresh logic
}

// doStream (lines 372-399)
if httpResp.StatusCode == http.StatusUnauthorized && c.config.TokenRefreshFunc != nil {
    // ... 27 lines of similar refresh logic
}
```

**Fix Required:** Extract to a shared method: `refreshTokenAndRetry(ctx, req, httpResp) (*HTTPResponse, error)`

---

### 2.2 Unclear Model Info Initialization Pattern ⚠️ MODERATE
**Location:** Lines 80-81, 111-123, 126-136

**Issue:** Model info is initialized three ways: in constructor (line 81), via `Once` (lines 111-122), and via direct call (line 126). This is confusing and error-prone.

```go
// Constructor calls initModelInfo
c.initModelInfo()  // Line 81

// GetModelContextWindow uses Once wrapper
c.modelInfoOnce.Do(func() {
    c.initModelInfo()  // Line 112
})

// But initModelInfo is also public and can be called directly
func (c *Client) initModelInfo() { ... }  // Line 126
```

**Problems:**
- `sync.Once` is unnecessary since constructor already calls `initModelInfo()`
- The `Once` wrapper is misleading - makes it seem initialization is deferred
- `modelInfoInitialized` flag is set but never checked
- Public `initModelInfo()` could be called multiple times, causing data races

**Fix Required:** Remove `Once`, make `initModelInfo()` private, remove unused flag.

---

### 2.3 Inconsistent Error Handling in Token Refresh ⚠️ MODERATE
**Location:** Lines 230-262 (doComplete) vs Lines 372-399 (doStream)

**Issue:** Error handling differs between streaming and non-streaming token refresh.

**In doComplete:**
```go
_, readErr := io.ReadAll(httpResp.Body)  // Line 232
if readErr != nil {
    return nil, httpResp.StatusCode, fmt.Errorf("failed to read error response body: %w", readErr)
}
```

**In doStream:**
```go
// No attempt to read response body before refresh (line 372)
oldToken := c.getToken()
```

**Impact:**
- Response body in streaming may not be fully consumed, causing connection leaks
- Inconsistent behavior between streaming and non-streaming paths
- Potential for subtle bugs

---

### 2.4 Unused Variable Assignments ⚠️ LOW
**Location:** Line 185

```go
_ = statusCode // Use statusCode to avoid unused variable
```

**Issue:** This is a code smell. If `statusCode` from line 180 isn't needed, don't capture it. The comment admits this is a workaround.

**Better approach:** Either use the status code (e.g., in logging) or restructure the code.

---

### 2.5 Inconsistent Error Type Conversions ⚠️ LOW
**Location:** Lines 457-493

**Issue:** Error response parsing has complex nested conditionals that are hard to follow.

```go
if err := json.Unmarshal(body, &errorResp); err == nil {
    bodyStr = errorResp.Error.Message

    // Check for specific error types
    if statusCode == http.StatusUnauthorized {
        // ...
    }

    if errorResp.Error.Code == "context_length_exceeded" || ... {
        // ...
    }
    // ... more conditions
} else if statusCode == http.StatusUnauthorized {
    // Duplicate 401 handling!
}
```

**Problems:**
- 401 handling appears in two places (lines 472 and 486)
- Error type detection is fragile (relies on string matching)
- Complex nesting makes logic hard to verify

---

## 3. Potential Bugs & Edge Cases

### 3.1 Resource Leak: Response Body Not Closed 🔥 CRITICAL
**Location:** Lines 254-258 (doComplete)

**Issue:** When token refresh succeeds and retries the request, the FIRST response body's `defer httpResp.Body.Close()` is shadowed by the SECOND response's defer, causing the first body to never be closed.

```go
httpResp, err = c.httpClient.Do(httpReq)  // Line 254
if err != nil {
    return nil, 0, client.NewConnectionError(c.config.BaseURL, err)
}
defer httpResp.Body.Close()  // Line 258 - This shadows the earlier defer at line 225!
```

**Impact:**
- Connection leak on every 401 with successful token refresh
- Eventually exhausts connection pool
- Memory leak from unclosed response bodies
- Critical in high-traffic scenarios

**Fix Required:** Explicitly close first response body before retry:
```go
httpResp.Body.Close()  // Close first response
httpResp, err = c.httpClient.Do(httpReq)  // Get new response
```

**Same issue exists in doStream at line 395.**

---

### 3.2 Context Deadline Ignored During Retry Backoff ⚠️ MODERATE
**Location:** Lines 172-176 (completeWithRetry)

**Issue:** Context deadline may expire DURING the backoff sleep, but this won't be detected until after the full backoff completes.

```go
select {
case <-ctx.Done():
    return nil, ctx.Err()
case <-time.After(backoff):  // May sleep past context deadline
}
```

**Scenario:**
- Context has 5-second deadline
- Backoff calculates 10 seconds
- Code sleeps for full 10 seconds even though context expired at 5 seconds
- Wastes 5 seconds before detecting cancellation

**Impact:** Poor responsiveness to cancellation, wasted resources.

**Fix:** Create timer that respects context:
```go
timer := time.NewTimer(backoff)
defer timer.Stop()
select {
case <-ctx.Done():
    return nil, ctx.Err()
case <-timer.C:
}
```

---

### 3.3 Race Condition in Retry Count Logic ⚠️ MODERATE
**Location:** Lines 157-202 (completeWithRetry)

**Issue:** The retry loop has subtle off-by-one behavior and dead code.

```go
for attempt := 0; attempt <= c.config.RetryConfig.MaxRetries; attempt++ {
    // ...
    if attempt == c.config.RetryConfig.MaxRetries {
        // If no retries configured, return original error
        if c.config.RetryConfig.MaxRetries == 0 {  // Line 198
            return nil, err
        }
        return nil, client.NewRetryLimitError(statusCode, attempt+1)
    }
}

// Lines 205-209: Dead code that can never execute
if lastErr != nil {
    return nil, lastErr
}
return nil, client.NewRetryLimitError(lastStatusCode, c.config.RetryConfig.MaxRetries+1)
```

**Problems:**
- When `MaxRetries == 0`, line 199 returns on first failure, but line 198 check is redundant
- Lines 205-209 are unreachable dead code
- Attempt counting is confusing (0-indexed but reports as 1-indexed)

**Fix Required:** Simplify logic and remove dead code.

---

### 3.4 Panic Risk: Nil Channel Send ⚠️ MODERATE
**Location:** Lines 101-106 (Stream method)

**Issue:** If `streamWithRetry` (line 104) panics before line 284 executes, the channel will never be closed and consumers will deadlock.

```go
func (c *Client) Stream(ctx context.Context, req *client.ChatCompletionRequest) (<-chan client.StreamEvent, error) {
    eventCh := make(chan client.StreamEvent, c.config.StreamConfig.BufferSize)
    go c.streamWithRetry(ctx, req, eventCh)  // If this panics...
    return eventCh, nil  // ...channel never closed
}
```

**Impact:** Consumer hangs indefinitely waiting for events.

**Fix Required:** Add panic recovery:
```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            eventCh <- client.StreamEvent{Type: client.EventTypeError, Error: fmt.Errorf("panic: %v", r)}
            close(eventCh)
        }
    }()
    c.streamWithRetry(ctx, req, eventCh)
}()
```

---

### 3.5 Memory Leak: Scanner Not Properly Closed 🔥 CRITICAL
**Location:** Lines 74-195 (stream.go parse method)

**Issue:** The scanner goroutine (lines 95-122) may not exit cleanly if the function returns early.

```go
go func() {
    defer close(scanCh)
    for {
        ok := scanner.Scan()
        // ...
        select {
        case scanCh <- scanResult{...}:
            if !ok {
                return
            }
        case <-scanDone:
            return
        }
    }
}()
```

**Problem:** If `scanner.Scan()` blocks indefinitely (e.g., network stall), the goroutine won't detect `scanDone` until after the scan completes. This is a goroutine leak.

**Impact:**
- Goroutine leak on every failed/cancelled stream
- Memory accumulation over time
- Resource exhaustion in long-running processes

**Fix Required:** Use scanner with timeout or make scanner.Scan() interruptible.

---

### 3.6 Integer Overflow in Token Limit Calculation ⚠️ LOW
**Location:** Lines 131-133

```go
if info.contextWindow > 0 {
    c.autoCompactTokenLimit = int64(float64(info.contextWindow) * 0.8)
}
```

**Issue:** For very large context windows (e.g., GPT-5 with 200,000 tokens), the float64 conversion and multiplication could lose precision.

**Impact:** Minimal for current models, but could cause issues with future large-context models.

---

### 3.7 Model Info Race Condition 🔥 CRITICAL
**Location:** Lines 28-31, 80-81, 126-136

**Issue:** Model context window fields are not protected by mutex but are accessed concurrently.

```go
type Client struct {
    // ...
    modelContextWindow    int64  // No mutex protection!
    autoCompactTokenLimit int64  // No mutex protection!
    modelInfoInitialized  bool   // No mutex protection!
    modelInfoOnce         sync.Once
}

func (c *Client) initModelInfo() {
    info := getModelInfo(c.config.Model)
    c.modelContextWindow = info.contextWindow  // Race: writes without lock
}

func (c *Client) GetModelContextWindow() int64 {
    c.modelInfoOnce.Do(func() {
        c.initModelInfo()
    })
    return c.modelContextWindow  // Race: reads without lock
}
```

**Race Scenario:**
1. Goroutine A calls `GetModelContextWindow()`, triggers `Do()`
2. Goroutine B calls `GetModelContextWindow()` while A is in `initModelInfo()`
3. B reads `c.modelContextWindow` (line 114) while A writes it (line 128)
4. **Data race!**

**Evidence from code:**
- The `sync.Once` only protects calling `initModelInfo()`, NOT the field reads
- The constructor calls `initModelInfo()` at line 81 without any synchronization
- Multiple goroutines can read the fields (lines 114, 122) during initialization

**Impact:**
- Undefined behavior (data race)
- Possible corrupted token limit values
- Could affect auto-compaction decisions

**Fix Required:** Either:
1. Only initialize in constructor before returning client (simplest)
2. OR protect reads/writes with the existing `tokenMutex`

---

## 4. Missing Test Coverage

### 4.1 Error Path Testing Gaps ❌

**Not tested:**
1. **Retry-After header parsing** - The `parseRetryAfter` function (retry.go:85-110) has zero test coverage
2. **Context cancellation during backoff** - No tests for cancelling during the sleep phase
3. **Token refresh during stream with retry** - Tests exist for first attempt, but not when retry also gets 401
4. **Connection pool exhaustion recovery** - Metrics track exhaustion but no tests verify recovery
5. **Multiple simultaneous token refreshes** - Line 1050's test only covers 3 concurrent requests, not race conditions

### 4.2 Edge Cases Not Covered ⚠️

**Missing tests:**
1. Empty API key after successful client creation
2. Token refresh that returns empty string
3. Malformed JSON in error responses
4. Very large context windows (overflow scenarios)
5. Rate limit with malformed headers
6. Idle timeout exactly at message boundary
7. Custom headers overriding reserved headers

### 4.3 Integration Test Limitations ⚠️

The integration test file exists but likely has limited real API testing due to API key requirements. Mock-based unit tests don't catch:
- Actual API contract changes
- Real network timeout scenarios
- Actual SSL/TLS issues
- Real rate limit response formats

---

## 5. Documentation Issues

### 5.1 Missing Package-Level Examples ⚠️ MODERATE

**Issue:** No examples in godoc comments showing common usage patterns.

**Needed:**
```go
// Example:
//   cfg := client.ClientConfig{
//       BaseURL: "https://api.openai.com/v1",
//       APIKey: os.Getenv("OPENAI_API_KEY"),
//       Model: "gpt-4",
//   }
//   c, err := openai.NewClient(cfg)
//   if err != nil { ... }
//   resp, err := c.Complete(ctx, &client.ChatCompletionRequest{...})
```

### 5.2 Undocumented Thread Safety Guarantees ⚠️ MODERATE

**Location:** Type `Client` (lines 20-36)

**Issue:** No documentation about whether `Client` is safe for concurrent use.

**Question for users:**
- Can multiple goroutines call `Complete()` simultaneously?
- Is `Stream()` safe to call from multiple goroutines?
- What about calling `Complete()` and `Stream()` concurrently?

**Current state:** Code APPEARS thread-safe (uses mutexes), but this is not guaranteed or documented.

### 5.3 Configuration Defaults Not Documented ⚠️ LOW

**Location:** Lines 51-63

**Issue:** Default values are set but not documented in `ClientConfig` godoc.

```go
// NewClient creates a new OpenAI client with the given configuration.
func NewClient(cfg client.ClientConfig) (*Client, error) {
    // Validate configuration
    // ...

    // Set defaults
    if cfg.RequestTimeout == 0 {
        cfg.RequestTimeout = 30 * time.Second  // Not documented!
    }
    // ... more undocumented defaults
}
```

**Impact:** Users don't know what values are used when fields are zero-valued.

### 5.4 Error Return Documentation Incomplete ⚠️ LOW

**Location:** Methods like `Complete` (line 87)

**Issue:** Godoc doesn't specify what error types can be returned.

**Should document:**
```go
// Complete performs a non-streaming chat completion request.
//
// Returns:
//   - *UnauthorizedError: if API key is invalid (can auto-refresh if configured)
//   - *UsageLimitError: if rate limit is exceeded
//   - *ContextWindowExceededError: if prompt is too long
//   - *ConnectionError: if network fails
//   - *RetryLimitError: if max retries exhausted
//   - *UnexpectedStatusError: for other HTTP errors
func (c *Client) Complete(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error)
```

---

## 6. Security Concerns

### 6.1 Token Logging Risk ⚠️ MODERATE

**Location:** Lines 138-150 (token getters)

**Issue:** No guards against token logging. If logging is added for debugging, tokens could leak.

**Recommendation:**
- Add comment warning against logging token values
- Consider a `String()` method that redacts tokens

### 6.2 No Input Sanitization ⚠️ MODERATE

**Location:** Lines 420-454 (buildRequest)

**Issue:** Request body is marshaled directly without validation.

**Risks:**
- No check for extremely large message content (could cause memory issues)
- No validation of tool call arguments
- No limits on number of messages
- Custom headers not sanitized (mentioned in 1.3)

**Recommendation:** Add reasonable limits before marshaling:
```go
if len(req.Messages) > 1000 {
    return nil, fmt.Errorf("too many messages: %d (max 1000)", len(req.Messages))
}
```

### 6.3 Sensitive Data in Error Messages ⚠️ LOW

**Location:** Lines 457-493 (handleErrorResponse)

**Issue:** Error bodies are included directly in error messages (line 469). These might contain sensitive data.

**Example:**
```go
bodyStr = errorResp.Error.Message  // Could contain user data or internal info
```

**Recommendation:** Sanitize or truncate error messages before including in returned errors.

---

## 7. Performance Concerns

### 7.1 Inefficient Model Info Lookup ⚠️ LOW

**Location:** Lines 598-621 (getModelInfo)

**Issue:** String operations performed on every call:
```go
model = strings.ToLower(model)  // Allocation
switch {
case strings.Contains(model, "gpt-4-turbo"):  // Linear search
case strings.Contains(model, "gpt-4-32k"):    // Linear search
// ...
}
```

**Impact:** Minor but unnecessary allocations on every request (if called repeatedly).

**Fix:** Cache lowercased model name in Client, or use a map for O(1) lookup.

### 7.2 JSON Unmarshaling in Error Path ⚠️ LOW

**Location:** Lines 458-469

**Issue:** Every error response gets unmarshaled twice - once for struct, once for string.

```go
bodyStr := string(body)  // Conversion #1
if err := json.Unmarshal(body, &errorResp); err == nil {  // Unmarshal attempt
    bodyStr = errorResp.Error.Message  // Override with parsed value
}
```

**Impact:** Minor performance hit on error paths.

### 7.3 Backoff Calculation Uses Non-Crypto Random ⚠️ LOW

**Location:** Line 25 (retry.go)

```go
jitter := backoff * 0.25 * (2.0*rand.Float64() - 1.0)
```

**Issue:** Uses `math/rand` which is not thread-safe without seeding. Also, no seed is set anywhere.

**Impact:**
- Potential for duplicate jitter values across goroutines
- Possible thundering herd if multiple clients start at same time

**Fix:** Use `math/rand.NewSource()` with unique seed or use `math/rand` v2 in Go 1.20+.

---

## 8. Maintainability Issues

### 8.1 Magic Numbers ⚠️ LOW

**Location:** Multiple

```go
c.autoCompactTokenLimit = int64(float64(info.contextWindow) * 0.8)  // Line 132: 0.8 magic
jitter := backoff * 0.25 * (2.0*rand.Float64() - 1.0)  // Line 25: 0.25 magic
if elapsed < 50*time.Millisecond {  // Line 196: 50ms, 100ms magic
```

**Issue:** Constants are inline without explanation.

**Fix:** Define as named constants:
```go
const (
    defaultAutoCompactPercentage = 0.8
    jitterPercentage = 0.25
    connectionReuseThresholdMs = 50
    newConnectionThresholdMs = 100
)
```

### 8.2 Complex Boolean Expressions ⚠️ LOW

**Location:** Lines 478-480

```go
if errorResp.Error.Code == "context_length_exceeded" ||
    strings.Contains(errorResp.Error.Message, "context") {
    return client.NewContextWindowExceededError()
}
```

**Issue:** String matching for error detection is fragile and could cause false positives.

**Example:** An error message like "Please provide more context" would trigger context window error.

**Fix:** Use strict error code matching only, or maintain a list of known error codes.

---

## Recommendations by Priority

### Immediate (Critical) 🔥

1. **Fix response body leak** (3.1) - Add explicit `Close()` before token refresh retry
2. **Fix goroutine leak** (3.5) - Make scanner interruptible or add timeout
3. **Fix model info race condition** (3.7) - Remove `Once` usage or add proper synchronization
4. **Implement Retry-After support** (1.1) - Honor server backoff hints

### High Priority ⚠️

5. **Extract duplicate token refresh** (2.1) - DRY violation with 28 lines duplicated
6. **Fix context deadline handling** (3.2) - Respect deadline during backoff
7. **Expose rate limit status** (1.2) - Add getter for rate limit info
8. **Add header validation** (1.3) - Prevent reserved header overwrites
9. **Add panic recovery** (3.4) - Prevent channel deadlocks

### Medium Priority

10. **Simplify model info initialization** (2.2) - Remove confusing `Once` pattern
11. **Clean up retry logic** (3.3) - Remove dead code, fix off-by-one
12. **Improve error handling consistency** (2.3) - Unify streaming/non-streaming paths
13. **Fix connection metrics** (1.4) - Replace timing heuristics with better detection
14. **Add comprehensive tests** (4.1, 4.2) - Cover error paths and edge cases

### Low Priority

15. **Add documentation** (5.1-5.4) - Examples, thread safety, error types
16. **Remove unused code** (2.4) - Clean up status code assignment
17. **Add input validation** (6.2) - Sanitize and limit inputs
18. **Optimize performance** (7.1-7.3) - Cache, reduce allocations
19. **Extract magic numbers** (8.1) - Named constants
20. **Improve error detection** (8.2) - Use strict error codes

---

## Testing Recommendations

### Required Tests

1. **Token refresh response body leak test**
   ```go
   // Verify first 401 response body is closed before retry
   ```

2. **Context cancellation during backoff test**
   ```go
   // Cancel context while sleeping between retries, verify immediate return
   ```

3. **Concurrent model info access test**
   ```go
   // 1000 goroutines calling GetModelContextWindow simultaneously
   ```

4. **Scanner goroutine leak test**
   ```go
   // Verify no leaked goroutines after stream cancellation
   ```

5. **Custom header override test**
   ```go
   // Attempt to override Authorization header via custom headers
   ```

6. **Retry-After header test**
   ```go
   // Mock server returns Retry-After, verify client respects it
   ```

### Test Coverage Goals

- **Current estimated coverage:** ~70% (based on test file)
- **Target coverage:** 85%+
- **Critical path coverage:** 95%+ (retry, token refresh, streaming)

---

## Conclusion

This OpenAI client implementation demonstrates good software engineering practices in many areas:
- Clear separation of concerns
- Comprehensive retry logic
- Good test coverage for happy paths
- Well-structured configuration

However, it has several critical issues that must be addressed:
- **Resource leaks** (response bodies, goroutines)
- **Race conditions** (model info access)
- **Incomplete features** (Retry-After, rate limit exposure)
- **Code duplication** (token refresh logic)

**Recommendation:** Address critical issues (🔥) before production use. High priority issues (⚠️) should be fixed within the next sprint. Medium/low priority items can be tackled as technical debt reduction.

The codebase shows promise and with the recommended fixes would be production-ready.

---

## Appendix: Files Reviewed

- `/Users/williamcory/codex/codex-go/internal/client/openai/openai.go` (622 lines)
- `/Users/williamcory/codex/codex-go/internal/client/openai/openai_test.go` (1154 lines)
- `/Users/williamcory/codex/codex-go/internal/client/openai/stream.go` (360 lines)
- `/Users/williamcory/codex/codex-go/internal/client/openai/retry.go` (111 lines)
- `/Users/williamcory/codex/codex-go/internal/client/openai/metrics.go` (205 lines)
- `/Users/williamcory/codex/codex-go/internal/client/client.go` (352 lines)
- `/Users/williamcory/codex/codex-go/internal/client/errors.go` (352 lines)

**Total lines reviewed:** ~3,156 lines
