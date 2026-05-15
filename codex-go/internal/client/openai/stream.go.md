# Code Review: stream.go

**File:** `/Users/williamcory/codex/codex-go/internal/client/openai/stream.go`
**Review Date:** 2025-10-26
**Lines of Code:** 360

---

## Executive Summary

The `stream.go` file implements SSE (Server-Sent Events) parsing for OpenAI's streaming chat completion API. The code quality is generally **good** with strong concurrency handling and comprehensive error management. However, there are several areas requiring attention, including incomplete features, missing edge case handling, potential performance issues, and some technical debt.

**Overall Grade: B+ (Good, with room for improvement)**

---

## 1. Incomplete Features & Functionality

### 1.1 Unused `reset()` Method (CRITICAL)
**Location:** Lines 355-359

```go
// reset clears the accumulator.
// nolint:unused // Reserved for multi-turn tool call handling
func (a *toolCallAccumulator) reset() {
	a.calls = make(map[int]*client.ToolCall)
}
```

**Issue:** The `reset()` method is defined but never called, marked with a linter suppression comment indicating it's "reserved for multi-turn tool call handling."

**Impact:**
- **Memory Leak Risk:** Tool calls accumulate indefinitely across multiple streaming chunks without ever being cleared
- **Multi-turn Conversations:** In long-running streaming sessions with multiple completions, the accumulator will grow unbounded
- **Incomplete Feature:** The comment suggests multi-turn support was planned but never implemented

**Recommendation:**
- Implement multi-turn tool call handling or remove the method
- If keeping it, document when it should be called
- Consider calling `reset()` after emitting completed tool calls in `processChunk()`

### 1.2 No Token Usage Validation
**Location:** Lines 273-275

```go
if chunk.Usage != nil {
	completedEvent.TokenUsage = chunk.Usage
}
```

**Issue:** No validation that token usage values are reasonable (non-negative, within expected ranges).

**Impact:**
- Could propagate invalid data from malformed API responses
- No protection against negative token counts or absurdly large values

**Recommendation:**
```go
if chunk.Usage != nil {
    // Validate token usage
    if chunk.Usage.InputTokens >= 0 && chunk.Usage.OutputTokens >= 0 && chunk.Usage.TotalTokens >= 0 {
        completedEvent.TokenUsage = chunk.Usage
    } else {
        log.Printf("Warning: Invalid token usage in stream chunk: %+v", chunk.Usage)
    }
}
```

---

## 2. TODO Comments & Technical Debt

### 2.1 Linter Suppression Without Justification
**Location:** Line 356

```go
// nolint:unused // Reserved for multi-turn tool call handling
```

**Issue:** Using linter suppressions as a TODO mechanism is poor practice. If the feature isn't implemented, either implement it or remove the code.

**Recommendation:**
- Add a proper TODO comment with a tracking issue number
- Consider using a feature flag if multi-turn support is WIP
- Example: `// TODO(issue-#123): Implement multi-turn tool call handling and call reset() appropriately`

---

## 3. Code Quality Issues

### 3.1 Silent Error Handling in JSON Parsing
**Location:** Lines 181-184

```go
if err := json.Unmarshal([]byte(data), &chunk); err != nil {
	// Log but continue - some providers send non-JSON comments
	continue
}
```

**Issue:** The error is silently ignored with no logging. The comment mentions "some providers send non-JSON comments," but this could hide actual parsing errors.

**Impact:**
- Debugging difficulty when legitimate JSON is malformed
- No visibility into how many chunks are being dropped
- Could mask API changes or integration issues

**Recommendation:**
```go
if err := json.Unmarshal([]byte(data), &chunk); err != nil {
	// Some providers send non-JSON comments (e.g., ": keep-alive")
	// Only log if it looks like it should have been JSON
	if !strings.HasPrefix(data, ":") && data != "" {
		log.Printf("Warning: Failed to parse stream chunk as JSON: %v (data: %q)", err, truncate(data, 100))
	}
	continue
}
```

### 3.2 Magic Numbers Without Constants
**Location:** Multiple locations

- Line 48: `cap(eventCh)` - Buffer capacity check
- Line 52: `p.config.BackpressureThreshold` - Threshold comparison
- Line 346: `i <= maxIndex` - Index iteration

**Issue:** No validation of configuration values like `BackpressureThreshold` (should be 0.0-1.0).

**Recommendation:**
```go
const (
    minBackpressureThreshold = 0.0
    maxBackpressureThreshold = 1.0
)

// In constructor or validation function:
if p.config.BackpressureThreshold < minBackpressureThreshold ||
   p.config.BackpressureThreshold > maxBackpressureThreshold {
    log.Printf("Warning: Invalid backpressure threshold: %.2f, using default 0.8",
        p.config.BackpressureThreshold)
    p.config.BackpressureThreshold = 0.8
}
```

### 3.3 Inconsistent Logging Strategy
**Location:** Lines 41, 53-54, 183

The code uses Go's standard `log` package without any log levels or structured logging. Mix of logging styles:
- Line 41: Warning with event type
- Line 53-54: Warning with detailed metrics
- Line 183: Silent error (no log)

**Recommendation:**
- Use structured logging (e.g., `slog`, `zap`, `logrus`)
- Implement consistent log levels (DEBUG, INFO, WARN, ERROR)
- Add context fields (stream ID, chunk ID, timestamp)

### 3.4 Complex Nested Conditionals in `processChunk()`
**Location:** Lines 223-247

```go
// Handle reasoning delta (if present)
if choice.Delta.Reasoning != nil {
	if p.config.EnableRawAgentReasoning {
		// Extract reasoning content text
		var reasoningText string
		switch v := choice.Delta.Reasoning.(type) {
		case string:
			reasoningText = v
		case map[string]interface{}:
			// Handle structured reasoning content
			if text, ok := v["text"].(string); ok {
				reasoningText = text
			} else if content, ok := v["content"].(string); ok {
				reasoningText = content
			}
		}

		if reasoningText != "" {
			events = append(events, client.StreamEvent{
				Type: client.EventTypeReasoningContentDelta,
				Data: reasoningText,
			})
		}
	}
}
```

**Issue:** Deep nesting (4 levels) makes the code hard to read and test.

**Recommendation:** Extract to a helper method:
```go
func (p *streamParser) extractReasoningText(reasoning interface{}) (string, bool) {
    switch v := reasoning.(type) {
    case string:
        return v, true
    case map[string]interface{}:
        if text, ok := v["text"].(string); ok {
            return text, true
        }
        if content, ok := v["content"].(string); ok {
            return content, true
        }
    }
    return "", false
}

// In processChunk:
if choice.Delta.Reasoning != nil && p.config.EnableRawAgentReasoning {
    if reasoningText, ok := p.extractReasoningText(choice.Delta.Reasoning); ok {
        events = append(events, client.StreamEvent{
            Type: client.EventTypeReasoningContentDelta,
            Data: reasoningText,
        })
    }
}
```

### 3.5 Potential Race Condition in Timer Usage
**Location:** Lines 126-134

```go
if idleTimer != nil {
	if !idleTimer.Stop() {
		select {
		case <-idleTimer.C:
		default:
		}
	}
	idleTimer.Reset(p.config.IdleTimeout)
}
```

**Issue:** The timer stop/drain/reset pattern is correct but complex. According to Go's documentation, this is the recommended pattern, but it's easy to get wrong.

**Assessment:** Actually, this is **correct** - the code properly drains the channel if `Stop()` returns false. No issue here, but worth documenting why this pattern is needed.

**Recommendation:** Add a comment explaining the pattern:
```go
// Properly reset timer: must drain channel if Stop() returns false
// to prevent stale timeout from triggering. See: https://pkg.go.dev/time#Timer.Reset
if idleTimer != nil {
    if !idleTimer.Stop() {
        // Timer already fired, drain channel
        select {
        case <-idleTimer.C:
        default:
        }
    }
    idleTimer.Reset(p.config.IdleTimeout)
}
```

---

## 4. Missing Test Coverage

### 4.1 Not Tested: Error Cases in `sendEventWithBackpressure()`
**Missing Tests:**
- What happens when `eventCh` is closed during send?
- Channel closure detection
- Panic recovery if channel is closed

**Recommended Test:**
```go
func TestStreamParser_SendToClosedChannel(t *testing.T) {
    parser := newStreamParser(client.StreamConfig{})
    eventCh := make(chan client.StreamEvent, 1)
    close(eventCh) // Close immediately

    event := client.StreamEvent{Type: client.EventTypeOutputTextDelta, Data: "test"}

    // Should either return error or recover from panic
    err := parser.sendEventWithBackpressure(context.Background(), eventCh, event)
    // Assert expected behavior
}
```

### 4.2 Not Tested: Malformed SSE Data
**Missing Tests:**
- Lines without "data: " prefix (should be skipped)
- Incomplete SSE messages
- Very long lines (bufio.Scanner has 64KB default limit)
- Invalid UTF-8 sequences
- Mixed line endings (CR, LF, CRLF)

**Recommended Test:**
```go
func TestStreamParser_MalformedSSE(t *testing.T) {
    tests := []struct {
        name       string
        streamData string
        expectErr  bool
    }{
        {
            name:       "missing data prefix",
            streamData: "event: test\n{\"invalid\": true}\n\n",
            expectErr:  false, // Should skip
        },
        {
            name:       "line too long",
            streamData: "data: " + strings.Repeat("a", 100000) + "\n\n",
            expectErr:  true, // Scanner buffer overflow
        },
        // ... more cases
    }
}
```

### 4.3 Not Tested: Tool Call Accumulator Edge Cases
**Missing Tests:**
- Non-sequential tool call indices (e.g., 0, 2, 5)
- Duplicate indices
- Negative indices
- Very large index values
- Tool calls with missing required fields

**Recommended Test:**
```go
func TestToolCallAccumulator_NonSequentialIndices(t *testing.T) {
    acc := newToolCallAccumulator()

    acc.add(client.ToolCallDelta{Index: 0, ID: "call_1", Type: "function"})
    acc.add(client.ToolCallDelta{Index: 5, ID: "call_2", Type: "function"})
    acc.add(client.ToolCallDelta{Index: 2, ID: "call_3", Type: "function"})

    calls := acc.getCompleted()
    // Should return 3 calls, may have gaps in the slice
    if len(calls) != 3 {
        t.Errorf("expected 3 calls, got %d", len(calls))
    }
}
```

### 4.4 Not Tested: Reasoning Content Edge Cases
**Missing Tests:**
- `choice.Delta.Reasoning` as unexpected type (e.g., array, number)
- Empty reasoning object
- Reasoning with both "text" and "content" fields
- Very large reasoning content

### 4.5 Not Tested: Resource Cleanup on Panic
**Missing Tests:**
- Panic in `processChunk()` - does goroutine cleanup?
- Panic in event processing - are resources leaked?
- Multiple rapid panics and recoveries

---

## 5. Potential Bugs & Edge Cases

### 5.1 Scanner Buffer Size Limitation (HIGH PRIORITY)
**Location:** Line 75

```go
scanner := bufio.NewScanner(r)
```

**Issue:** `bufio.Scanner` has a default maximum token size of 64KB. If a streaming chunk exceeds this (e.g., large function arguments), the scanner will error with `bufio.Scanner: token too long`.

**Impact:**
- Stream parsing fails silently (error at line 153)
- Large tool calls will be truncated or cause stream failure
- No retry or recovery mechanism

**Proof of Issue:**
```go
// This will fail if a single SSE line exceeds ~64KB
data: {"id":"test","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"..." /* 100KB JSON */}}]}}]}
```

**Recommendation:**
```go
scanner := bufio.NewScanner(r)
// Increase buffer size to handle large chunks (e.g., 1MB)
const maxScanTokenSize = 1024 * 1024 // 1MB
buf := make([]byte, maxScanTokenSize)
scanner.Buffer(buf, maxScanTokenSize)
```

### 5.2 Tool Call Index Gaps Not Handled
**Location:** Lines 346-350

```go
for i := 0; i <= maxIndex; i++ {
	if call, exists := a.calls[i]; exists {
		calls = append(calls, *call)
	}
}
```

**Issue:** If tool call indices have gaps (e.g., received index 0, 2, 4 but not 1, 3), the returned slice will have different length than expected by the caller.

**Impact:**
- Incorrect tool call ordering
- Tools might be skipped
- Downstream code may assume contiguous indices

**Recommendation:**
- Document that gaps are possible
- Return tool calls in a map instead of slice, OR
- Sort by actual indices and warn about gaps:

```go
func (a *toolCallAccumulator) getCompleted() []client.ToolCall {
    if len(a.calls) == 0 {
        return nil
    }

    // Extract and sort indices
    indices := make([]int, 0, len(a.calls))
    for idx := range a.calls {
        indices = append(indices, idx)
    }
    sort.Ints(indices)

    // Check for gaps
    for i := 1; i < len(indices); i++ {
        if indices[i] != indices[i-1]+1 {
            log.Printf("Warning: Tool call index gap detected: %d to %d", indices[i-1], indices[i])
        }
    }

    calls := make([]client.ToolCall, len(indices))
    for i, idx := range indices {
        calls[i] = *a.calls[idx]
    }

    return calls
}
```

### 5.3 No Validation of Chunk Structure
**Location:** Lines 202-282

**Issue:** The code assumes `chunk.Choices` is valid without checking:
- Empty choices array
- Nil choices
- Multiple choices (which one to process?)

**Recommendation:**
```go
func (p *streamParser) processChunk(chunk *client.ChatCompletionChunk, accumulator *toolCallAccumulator) []client.StreamEvent {
    var events []client.StreamEvent

    if len(chunk.Choices) == 0 {
        log.Printf("Warning: Stream chunk has no choices: %+v", chunk)
        return events
    }

    if len(chunk.Choices) > 1 {
        log.Printf("Warning: Stream chunk has multiple choices (%d), processing all", len(chunk.Choices))
    }

    // ... rest of processing
}
```

### 5.4 Context Cancellation in Goroutine May Not Be Immediate
**Location:** Lines 110-120

```go
select {
case scanCh <- scanResult{ok: ok, line: line, err: err}:
	if !ok {
		return
	}
case <-scanDone:
	return
}
```

**Issue:** If the scanner goroutine is blocked in `scanner.Scan()` (line 99), it won't respond to context cancellation until the scan completes. This could be slow for network streams with no data.

**Impact:**
- Delayed cancellation in slow network scenarios
- Goroutine may linger for up to the read timeout period

**Assessment:** This is a known limitation of `bufio.Scanner` - there's no way to interrupt a blocking `Read()`. The code handles it as well as possible by checking `scanDone` after each scan.

**Recommendation:** Document this limitation:
```go
// Note: The scanner goroutine cannot be interrupted mid-scan due to bufio.Scanner
// limitations. In slow network conditions, cancellation may be delayed until the
// next read completes. Ensure the underlying reader has appropriate timeouts.
```

### 5.5 Memory Leak in Tool Call Accumulator
**Location:** Lines 300-332

**Issue:** The accumulator never removes or clears tool calls. In a long-running streaming session, if the same accumulator instance is reused, memory will grow indefinitely.

**Impact:**
- Unbounded memory growth in long sessions
- Old tool calls from previous turns might be included

**Recommendation:**
- Call `reset()` after emitting tool calls (line 267)
- Make accumulator lifecycle per-message, not per-stream

```go
// In processChunk, after emitting tool calls:
if len(toolCalls) > 0 {
    events = append(events, client.StreamEvent{
        Type: client.EventTypeOutputItemDone,
        Data: map[string]interface{}{
            "tool_calls": toolCalls,
        },
    })
    // Clear accumulator for next message
    accumulator.reset()
}
```

### 5.6 No Protection Against Infinite Streams
**Location:** Lines 124-194

**Issue:** The main parsing loop has no upper bound on iterations or events. A malicious or buggy server could send infinite data, causing unbounded memory consumption.

**Recommendation:**
```go
const maxEventsPerStream = 100000 // Configurable

eventCount := 0
for {
    eventCount++
    if eventCount > maxEventsPerStream {
        return fmt.Errorf("stream exceeded maximum event count: %d", maxEventsPerStream)
    }
    // ... rest of loop
}
```

### 5.7 Race Condition: Closing Scanner Channel
**Location:** Line 96

```go
defer close(scanCh)
```

**Issue:** If the parsing function returns while the scanner goroutine is trying to send on `scanCh`, the goroutine could panic due to send-on-closed-channel.

**Assessment:** Actually, this is **safe** - the select statement at lines 110-120 includes the `scanDone` case, which prevents sending after the main function returns.

**No action needed**, but consider adding a comment to clarify the safety:
```go
// Safe to close: the goroutine checks scanDone before sending, preventing
// send-on-closed-channel panics
defer close(scanCh)
```

---

## 6. Documentation Issues

### 6.1 Missing Package-Level Documentation
**Location:** Line 1

**Issue:** No package comment explaining the purpose, architecture, or SSE protocol details.

**Recommendation:**
```go
// Package openai provides streaming SSE (Server-Sent Events) parsing for OpenAI's
// chat completion API. It handles:
//   - SSE protocol parsing (RFC 8895)
//   - Tool call accumulation across streaming chunks
//   - Backpressure control for slow consumers
//   - Idle timeout detection
//   - Graceful cancellation without goroutine leaks
//
// The parser is designed for concurrent use with proper context cancellation
// and resource cleanup.
package openai
```

### 6.2 Incomplete Function Documentation
**Location:** Multiple functions lack comprehensive docs

Examples:
- `parse()` (line 73): Doesn't document return values or error conditions
- `sendEventWithBackpressure()` (line 31): Doesn't explain backpressure algorithm
- `processChunk()` (line 197): Doesn't explain event ordering guarantees

**Recommendation:** Add detailed godoc comments:
```go
// parse reads an SSE stream and emits events to the provided channel.
//
// The function spawns a single goroutine to perform blocking I/O, ensuring
// proper cancellation and preventing goroutine leaks. It handles:
//   - SSE line parsing (lines prefixed with "data: ")
//   - JSON deserialization of chat completion chunks
//   - Tool call accumulation across multiple deltas
//   - Idle timeout detection if configured
//
// Errors:
//   - Returns ctx.Err() if the context is cancelled
//   - Returns IdleTimeoutError if no events arrive within IdleTimeout
//   - Returns scanner errors for malformed streams
//
// Note: Lines exceeding bufio.Scanner's buffer size (~64KB by default) will
// cause scan errors. Use scanner.Buffer() to increase if needed.
func (p *streamParser) parse(ctx context.Context, r io.Reader, eventCh chan<- client.StreamEvent) error {
```

### 6.3 Missing Architecture Diagram
**Issue:** Complex goroutine coordination between scanner, parser, and event sender is not visually documented.

**Recommendation:** Add a comment block with ASCII diagram:
```
// Goroutine Architecture:
//
//   ┌─────────────┐
//   │   Reader    │ (network stream)
//   └──────┬──────┘
//          │
//          ▼
//   ┌─────────────┐
//   │  Scanner    │ (goroutine: scan lines)
//   │  Goroutine  │
//   └──────┬──────┘
//          │ scanCh
//          ▼
//   ┌─────────────┐
//   │   Parser    │ (parse() main loop)
//   │    Loop     │
//   └──────┬──────┘
//          │ eventCh
//          ▼
//   ┌─────────────┐
//   │  Consumer   │ (caller's goroutine)
//   └─────────────┘
//
// Cancellation:
//   ctx.Done() → parse() returns → scanDone closed → scanner exits
```

### 6.4 No Examples in Documentation
**Issue:** No usage examples showing how to set up streaming, handle events, or configure backpressure.

**Recommendation:** Add examples in the test file or godoc:
```go
// Example usage:
//
//   config := client.StreamConfig{
//       EnableBackpressure:    true,
//       BackpressureThreshold: 0.8,
//       IdleTimeout:           30 * time.Second,
//   }
//   parser := newStreamParser(config)
//
//   eventCh := make(chan client.StreamEvent, 100)
//   go func() {
//       err := parser.parse(ctx, resp.Body, eventCh)
//       close(eventCh)
//       if err != nil {
//           log.Printf("Stream error: %v", err)
//       }
//   }()
//
//   for event := range eventCh {
//       switch event.Type {
//       case client.EventTypeOutputTextDelta:
//           fmt.Print(event.Data)
//       case client.EventTypeCompleted:
//           // Handle completion
//       }
//   }
```

### 6.5 Unclear Ownership of Accumulator
**Location:** Line 87

```go
toolCallAccumulator := newToolCallAccumulator()
```

**Issue:** It's unclear if the accumulator should be per-message, per-choice, or per-stream. The current implementation is per-stream, but the lifecycle isn't documented.

**Recommendation:**
```go
// Track accumulated tool calls for the entire stream.
// Note: This accumulator is reused across all chunks in the stream.
// Tool calls are emitted once when finish_reason is set, then should
// be cleared for multi-turn conversations (currently not implemented).
toolCallAccumulator := newToolCallAccumulator()
```

---

## 7. Security Concerns

### 7.1 No Input Validation on Stream Data (MEDIUM)
**Location:** Lines 167-177

**Issue:** No validation that SSE data is well-formed before parsing:
- No length checks before `strings.TrimPrefix()`
- No validation that data is printable/safe
- Could include control characters or malicious content

**Impact:**
- Log injection via malicious SSE comments
- Potential DoS via very long lines
- Control character injection in logs

**Recommendation:**
```go
const maxSSELineLength = 1024 * 1024 // 1MB

if len(line) > maxSSELineLength {
    log.Printf("Warning: SSE line exceeds maximum length: %d bytes", len(line))
    continue
}

// Sanitize before logging to prevent log injection
sanitized := strings.Map(func(r rune) rune {
    if r < 32 && r != '\t' && r != '\n' {
        return -1 // Remove control characters
    }
    return r
}, data)
```

### 7.2 Potential for Memory Exhaustion (HIGH)
**Location:** Lines 300-332 (Tool Call Accumulator)

**Issue:** No limits on:
- Number of tool calls per stream
- Size of accumulated function arguments
- Number of concurrent streams

**Impact:**
- DoS via resource exhaustion
- OOM if server sends malicious stream with thousands of tool calls

**Recommendation:**
```go
const (
    maxToolCallsPerStream = 100
    maxFunctionArgsSize   = 1024 * 1024 // 1MB
)

func (a *toolCallAccumulator) add(delta client.ToolCallDelta) error {
    if len(a.calls) >= maxToolCallsPerStream {
        return fmt.Errorf("exceeded maximum tool calls per stream: %d", maxToolCallsPerStream)
    }

    call := a.calls[delta.Index]
    if call != nil && call.Function != nil {
        newSize := len(call.Function.Arguments) + len(delta.Function.Arguments)
        if newSize > maxFunctionArgsSize {
            return fmt.Errorf("function arguments exceed maximum size: %d", maxFunctionArgsSize)
        }
    }

    // ... rest of method
}
```

### 7.3 Unbounded Logging
**Location:** Lines 41, 53-54

**Issue:** Logging with user-controlled data (event types, metrics) could be used for log spam or injection attacks.

**Impact:**
- Log file exhaustion
- Log injection attacks
- Information disclosure in logs

**Recommendation:**
- Use structured logging with sanitized fields
- Implement log rate limiting
- Redact sensitive data from logs

### 7.4 No TLS/Certificate Validation Context
**Issue:** The parser operates on `io.Reader` without knowing if the connection is secure. This is fine architecturally (separation of concerns), but there's no documentation about expected security guarantees.

**Recommendation:** Add security documentation:
```go
// Security Note: This parser operates on the I/O level and makes no assumptions
// about transport security. Callers MUST ensure:
//   - TLS is used for network connections (https://)
//   - Certificate validation is enabled
//   - The io.Reader comes from a trusted source
//
// The parser does NOT validate:
//   - Stream source authenticity
//   - Content signatures or checksums
//   - Rate limiting or abuse prevention
```

---

## 8. Performance Concerns

### 8.1 Inefficient String Concatenation in Tool Calls
**Location:** Line 329

```go
call.Function.Arguments += delta.Function.Arguments
```

**Issue:** Using `+=` for string concatenation in a loop creates new string allocations on each append. For large function arguments streamed in small chunks, this is O(n²).

**Impact:**
- Slow performance for large tool arguments
- Increased GC pressure
- Memory fragmentation

**Recommendation:**
```go
// In toolCallAccumulator:
type toolCallAccumulator struct {
    calls map[int]*accumulatingToolCall
}

type accumulatingToolCall struct {
    call     *client.ToolCall
    argsBuilder strings.Builder // Use strings.Builder for efficient accumulation
}

func (a *toolCallAccumulator) add(delta client.ToolCallDelta) {
    // ...
    if delta.Function != nil {
        if acc.argsBuilder.Len() == 0 && delta.Function.Arguments != "" {
            // Initialize with first chunk
            acc.argsBuilder.WriteString(delta.Function.Arguments)
        } else if delta.Function.Arguments != "" {
            acc.argsBuilder.WriteString(delta.Function.Arguments)
        }
    }
}

func (a *toolCallAccumulator) getCompleted() []client.ToolCall {
    // ... convert builders to strings at the end
    for _, acc := range a.calls {
        if acc.call.Function != nil && acc.argsBuilder.Len() > 0 {
            acc.call.Function.Arguments = acc.argsBuilder.String()
        }
    }
}
```

### 8.2 Redundant Channel Capacity Check
**Location:** Lines 48-50

```go
bufferSize := cap(eventCh)
currentLen := len(eventCh)
usagePercent := float64(currentLen) / float64(bufferSize)
```

**Issue:** These calculations happen on every event send when backpressure is enabled, even if the threshold isn't reached.

**Recommendation:**
```go
// Only calculate if we're approaching the threshold
currentLen := len(eventCh)
if currentLen > 0 { // Quick check before expensive calculation
    bufferSize := cap(eventCh)
    usagePercent := float64(currentLen) / float64(bufferSize)
    if usagePercent >= p.config.BackpressureThreshold {
        log.Printf("Warning: Stream buffer usage high: %d/%d (%.1f%%) - applying backpressure",
            currentLen, bufferSize, usagePercent*100)
    }
}
```

### 8.3 Unnecessary Memory Allocations in `getCompleted()`
**Location:** Lines 338-350

**Issue:** The function finds `maxIndex` by iterating all keys, then iterates again up to `maxIndex`. For sparse indices (e.g., 0, 99), this wastes iterations.

**Recommendation:**
```go
func (a *toolCallAccumulator) getCompleted() []client.ToolCall {
    if len(a.calls) == 0 {
        return nil
    }

    // Pre-allocate exact size needed
    calls := make([]client.ToolCall, 0, len(a.calls))

    // Collect indices for sorting
    indices := make([]int, 0, len(a.calls))
    for idx := range a.calls {
        indices = append(indices, idx)
    }
    sort.Ints(indices) // O(n log n) but only for actual indices

    // Build result in order
    for _, idx := range indices {
        calls = append(calls, *a.calls[idx])
    }

    return calls
}
```

### 8.4 Timer Allocations on Every Iteration
**Location:** Lines 77-84

**Issue:** Timer is created once but reset on every loop iteration (line 133). This is actually correct and efficient - no issue here.

---

## 9. Testing Observations

### 9.1 Excellent Goroutine Leak Testing
**Strengths:**
- Comprehensive tests for goroutine leaks (lines 15-127 in test file)
- Tests cover cancellation, normal completion, and idle timeout
- Proper use of `runtime.GC()` and delays to ensure cleanup

**Assessment:** ⭐⭐⭐⭐⭐ (Excellent)

### 9.2 Good Backpressure Testing
**Strengths:**
- Tests for slow consumer scenarios (lines 569-672)
- Tests with and without backpressure enabled
- Context cancellation during backpressure

**Assessment:** ⭐⭐⭐⭐ (Very Good)

### 9.3 Reasoning Content Testing
**Strengths:**
- Tests multiple reasoning content formats (string, object)
- Tests mixed content and reasoning
- Tests feature flag behavior

**Assessment:** ⭐⭐⭐⭐ (Very Good)

### 9.4 Missing: Error Path Testing
**Weaknesses:**
- No tests for malformed SSE
- No tests for scanner buffer overflow
- No tests for invalid JSON structure

**Assessment:** ⭐⭐⭐ (Good, but incomplete)

---

## 10. Recommendations Summary

### High Priority (Fix Immediately)

1. **Scanner Buffer Size Limit** - Add `scanner.Buffer()` to handle large chunks
2. **Memory Leak in Tool Call Accumulator** - Implement `reset()` or document lifecycle
3. **Input Validation** - Add limits on tool calls, argument sizes
4. **Silent JSON Parse Errors** - Add logging with truncation

### Medium Priority (Fix in Next Sprint)

5. **Extract Complex Nested Logic** - Refactor `processChunk()` reasoning handling
6. **Improve Error Handling** - Log dropped events, validate chunk structure
7. **Add Security Documentation** - Document transport security expectations
8. **Performance: String Builder** - Use `strings.Builder` for tool call arguments

### Low Priority (Technical Debt)

9. **Remove or Implement `reset()`** - Either use it or remove the unused method
10. **Structured Logging** - Migrate from `log` to `slog` or similar
11. **Add Package Documentation** - Write comprehensive package-level docs
12. **Add Usage Examples** - Include godoc examples

---

## 11. Positive Aspects

### What This Code Does Well ⭐

1. **Excellent Goroutine Management** - Single scanner goroutine with proper cleanup via `scanDone` channel
2. **Proper Context Cancellation** - All blocking operations respect context cancellation
3. **Comprehensive Testing** - Strong test coverage for concurrency and goroutine leaks
4. **Clear Separation of Concerns** - Parser, accumulator, and event emitter are well-separated
5. **Backpressure Handling** - Thoughtful implementation with warnings and metrics
6. **Timer Handling** - Correct timer stop/drain/reset pattern
7. **SSE Protocol Compliance** - Properly handles "[DONE]" marker and empty lines

---

## 12. Final Verdict

**Code Quality: B+ (83/100)**

| Category | Score | Notes |
|----------|-------|-------|
| Functionality | 7/10 | Missing reset() usage, no validation |
| Reliability | 8/10 | Good concurrency, but edge cases exist |
| Performance | 7/10 | String concatenation issue, but otherwise good |
| Security | 6/10 | No input validation, potential DoS vectors |
| Maintainability | 8/10 | Well-structured, but needs better docs |
| Testing | 8/10 | Excellent goroutine tests, missing error paths |

**Recommendation:** Fix high-priority issues before production use at scale, especially scanner buffer size and input validation. The code is generally solid but has some critical edge cases that could cause issues in production.

---

## Appendix: Quick Wins Checklist

- [ ] Add `scanner.Buffer(buf, 1024*1024)` after line 75
- [ ] Call `accumulator.reset()` after emitting tool calls (line 267)
- [ ] Add logging to JSON parse error (line 181)
- [ ] Add validation for `len(chunk.Choices) == 0` (line 202)
- [ ] Document security expectations in package comment
- [ ] Add constants for magic numbers (thresholds, limits)
- [ ] Use `strings.Builder` for tool call argument accumulation (line 329)
- [ ] Add limits: max tool calls, max argument size, max events per stream
- [ ] Extract `extractReasoningText()` helper method (lines 223-247)
- [ ] Add godoc examples in test file
