# Code Review: pkg/sdk/sdk.go

**Date:** 2025-10-26
**Reviewer:** Claude Code
**File:** `/Users/williamcory/codex/codex-go/pkg/sdk/sdk.go`

---

## Executive Summary

The SDK file provides a high-level API for using Codex programmatically. While the code structure is clean and follows good Go practices, **the core functionality is incomplete**. The SDK creates sessions but doesn't actually execute AI interactions - both `Submit()` and `SubmitStream()` methods in `session.go` return placeholder responses rather than invoking the underlying conversation manager and orchestrator.

**Critical Issues:** 1 high-priority (incomplete core functionality)
**Major Issues:** 4 (race condition, missing functionality, resource management, validation)
**Minor Issues:** 6 (documentation, error handling, edge cases)

---

## 1. Incomplete Features or Functionality

### Critical: Core AI Interaction Not Implemented

**Severity:** HIGH
**Lines:** 165-191 (session.go)

The `Submit()` and `SubmitStream()` methods in `session.go` return hardcoded placeholder responses instead of actually communicating with the AI model:

```go
// session.go:173-175
response := &Response{
    Content:      "This is a placeholder response. Full implementation pending.",
    FinishReason: "stop",
    // ...
}
```

**Impact:** The SDK appears functional but doesn't perform its primary purpose - enabling AI-powered conversations.

**Missing Implementation:**
- No invocation of `s.manager` (ConversationManager) to create turns
- No use of `s.orchestrator` to execute tools
- No actual API calls to the AI model
- Token usage values are fake (hardcoded to 10/20/30)

**Available But Unused Resources:**
The SDK initializes these components but never uses them:
- `s.manager` (ConversationManager) - has `CreateSession()`, `SubmitOp()`, etc.
- `s.orchestrator` (Orchestrator) - has `Execute()` for tool execution
- `s.approvalCache` - for caching approval decisions
- Session-level approval handlers and policies

### Missing History Persistence

**Severity:** MEDIUM
**Lines:** 30-31, 92-93

The SDK accepts `EnableHistory` and `HistoryPath` options but never uses them:

```go
enableHistory bool
historyPath   string
```

**Impact:**
- Conversations cannot be persisted or resumed
- The `ConversationManager` has `ResumeSession()` capability that's unused
- Users expect history to work based on the API

**Recommendation:** Implement history persistence using the conversation manager's capabilities or remove these unused fields and document that history isn't supported yet.

### Incomplete Session Management

**Severity:** MEDIUM
**Lines:** 100-126

`NewSession()` creates a session object but doesn't:
1. Register it with the conversation manager
2. Initialize any turn processing
3. Set up the orchestrator with session-specific approval handlers
4. Validate session options (approval policy, sandbox policy, model)

**Missing Validations:**
- `ApprovalPolicy` values ("auto", "always", "never") are never validated
- `SandboxPolicy` values ("read_only", "workspace_write", "full_access") are never validated
- `Model` string is never checked against supported models
- `WorkingDirectory` path is never verified to exist

---

## 2. TODO Comments and Technical Debt

**Status:** GOOD - No TODO, FIXME, HACK, XXX, or BUG markers found in the code.

However, there are implicit TODOs based on placeholder implementations:
1. Implement actual AI message submission in `Submit()`
2. Implement actual streaming in `SubmitStream()`
3. Implement history persistence
4. Connect session to conversation manager
5. Wire up tool orchestration

---

## 3. Code Quality Issues

### Race Condition in Session ID Generation

**Severity:** HIGH
**Lines:** 206-212

```go
var sessionIDCounter int64

func generateSessionID() string {
    // Use a counter to ensure unique IDs in tests
    sessionIDCounter++
    return fmt.Sprintf("session_%d", sessionIDCounter)
}
```

**Problem:**
- Global variable `sessionIDCounter` is accessed without synchronization
- Concurrent calls to `NewSession()` can cause race conditions
- The comment says "in tests" but this is used in production code

**Race Scenario:**
```go
// Goroutine 1: reads sessionIDCounter = 5
// Goroutine 2: reads sessionIDCounter = 5
// Goroutine 1: writes sessionIDCounter = 6
// Goroutine 2: writes sessionIDCounter = 6
// Both return "session_6" - collision!
```

**Recommendation:** Use `sync/atomic` or UUID generation:
```go
import "sync/atomic"

var sessionIDCounter atomic.Int64

func generateSessionID() string {
    id := sessionIDCounter.Add(1)
    return fmt.Sprintf("session_%d", id)
}
```

Or better yet, use proper UUIDs:
```go
import "github.com/google/uuid"

func generateSessionID() string {
    return uuid.New().String()
}
```

### Inconsistent Error Aggregation

**Severity:** LOW
**Lines:** 177-202

In `Close()`, errors are collected in a slice but the final error message is poorly formatted:

```go
if len(errs) > 0 {
    return fmt.Errorf("errors during shutdown: %v", errs)
}
```

This produces ugly output like: `errors during shutdown: [error1 error2]`

**Recommendation:** Use `errors.Join()` (Go 1.20+) or manually format errors with newlines.

### Missing Context Propagation

**Severity:** LOW
**Lines:** 100, 155, 177

`NewSession()` accepts a `context.Context` but doesn't use it:
```go
func (s *SDK) NewSession(ctx context.Context, opts SessionOptions) (*Session, error) {
    // ctx is never used
}
```

Similarly, `CloseSession()` and `Close()` don't accept contexts, making them uninterruptible.

**Recommendation:** Either use the context or remove it to avoid confusion.

### Tool Registry Initialization Logic

**Severity:** LOW
**Lines:** 43-66

The tool initialization logic is complex with multiple branches:
- If `ToolRegistry` is provided, use it (ignoring `Tools`)
- If `Tools` is provided, register them
- Otherwise, register default tools

**Issues:**
1. Silent behavior: If both `ToolRegistry` and `Tools` are provided, `Tools` is silently ignored
2. Default tools are always file system and shell tools - no validation that they're appropriate
3. All default tools share the same `afero.Fs` instance - could cause issues if fs needs different configs

**Recommendation:** Document the precedence clearly and consider warning when `Tools` is ignored.

---

## 4. Missing Test Coverage

### Coverage Analysis

**Existing Tests** (from `sdk_test.go`):
- Basic SDK creation (with/without client)
- Session creation with various options
- Session retrieval and listing
- Session closing
- SDK closing

**Missing Test Coverage:**

1. **Concurrency Tests:**
   - Multiple goroutines creating sessions simultaneously (would catch race condition)
   - Concurrent access to session map
   - Closing SDK while sessions are being created
   - Getting sessions while closing

2. **Error Path Tests:**
   - What happens when conversation manager creation fails
   - Behavior when session close fails during SDK close
   - Double-close scenarios (currently untested for SDK)

3. **Integration Tests:**
   - Actual message submission (blocked by placeholder implementation)
   - Tool execution during conversations
   - History persistence
   - Approval workflows

4. **Edge Cases:**
   - Creating session with empty/invalid options
   - Maximum number of concurrent sessions
   - Memory leaks from unclosed sessions
   - Behavior with nil tool registry after creation

5. **Resource Cleanup:**
   - Verify all resources are properly released on Close()
   - Check for goroutine leaks
   - Verify approval cache is cleared

**Test Quality Issues:**
- Tests use real file system (`afero.NewOsFs()`) instead of memory fs
- No benchmarks for session creation/destruction
- No property-based tests for concurrency

---

## 5. Potential Bugs and Edge Cases

### Bug #1: Session Map Not Cleaned on Session Close Failure

**Severity:** MEDIUM
**Lines:** 155-173

In `CloseSession()`:
```go
if err := session.close(); err != nil {
    return fmt.Errorf("failed to close session: %w", err)
}
// Remove from SDK
delete(s.sessions, sessionID)
```

If `session.close()` fails, the session remains in the map but might be in an inconsistent state.

**Recommendation:** Consider whether to remove from map even on error, or mark as "closing failed" state.

### Bug #2: Double-Close Not Prevented at SDK Level

**Severity:** LOW
**Lines:** 177-203

While `Session.close()` checks for double-close, `SDK.Close()` doesn't:
```go
func (s *SDK) Close() error {
    s.mu.Lock()
    defer s.mu.Unlock()
    // No check if already closed
```

Multiple calls to `SDK.Close()` will iterate over already-closed sessions and try to close the manager multiple times.

**Recommendation:** Add a `closed` flag to SDK similar to Session.

### Edge Case #1: Empty Session Map After Close

**Severity:** LOW
**Lines:** 196

```go
s.sessions = make(map[string]*Session)
```

This happens even if Close() returns errors. A subsequent call to `ListSessions()` would return an empty list even though some sessions failed to close properly.

**Recommendation:** Only clear the map if all closes succeeded, or maintain a "failed" state.

### Edge Case #2: GetSession After SDK Close

**Severity:** LOW
**Lines:** 129-139

```go
func (s *SDK) GetSession(sessionID string) (*Session, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    // No check if SDK is closed
```

After `SDK.Close()`, `GetSession()` will return "session not found" instead of a clearer "SDK is closed" error.

### Edge Case #3: Concurrent NewSession and Close

**Severity:** MEDIUM
**Lines:** 100-126, 177-203

If `SDK.Close()` is called while `NewSession()` is executing:
1. NewSession locks SDK.mu
2. Creates session
3. Stores in map
4. Unlocks
5. Close() could immediately close that new session

**Recommendation:** Add SDK-level closed flag and check it in NewSession.

### Edge Case #4: Conversation Manager Configuration Mismatch

**Severity:** MEDIUM
**Lines:** 78-84

The manager is created with `Client` but not with the `Orchestrator`:
```go
mgr, err := manager.NewManager(manager.ManagerConfig{
    Client: opts.Client.Internal(),
    // Orchestrator not passed
})
```

Looking at the manager code, it expects an orchestrator in the config. This might work because orchestrator is optional, but it means the manager and SDK each have their own orchestrator instance.

**Impact:** Tool execution coordination might not work properly since tools registered with SDK's orchestrator won't be available to the manager's orchestrator.

---

## 6. Documentation Issues

### Missing Package Examples

**Severity:** LOW

While `example_test.go` exists, more comprehensive examples are needed:
- Basic usage with AI interaction (blocked by implementation)
- Streaming responses
- Tool approval workflows
- Error handling patterns
- Graceful shutdown

### Incomplete API Documentation

**Severity:** LOW

Several exported functions lack complete documentation:

1. **NewSession()** (line 98):
   - Doesn't document what happens if ConversationID already exists
   - Doesn't explain session isolation guarantees
   - Missing example of resuming existing conversation

2. **CloseSession()** (line 155):
   - Doesn't explain whether in-progress operations are cancelled
   - Missing guidance on when to use CloseSession vs letting SDK.Close handle it

3. **Options struct** (options.go):
   - `ToolRegistry` and `Tools` interaction not documented
   - No guidance on which approach to use
   - Missing examples

### Misleading Comments

**Severity:** LOW
**Line:** 209

```go
// Use a counter to ensure unique IDs in tests
```

This comment is misleading because:
1. The function is used in production, not just tests
2. The counter is not thread-safe
3. Implies this is temporary/test-only code

### Missing Design Documentation

The file lacks:
- Thread-safety guarantees (what's safe to call concurrently?)
- Resource lifecycle documentation
- Performance characteristics (how many sessions can be created?)
- Relationship between SDK, Manager, and Orchestrator

---

## 7. Security Concerns

### Minimal Security Issues (Given Current State)

Since the SDK doesn't actually execute any operations yet, security impact is limited. However, when implemented:

### Concern #1: No Input Validation

**Severity:** MEDIUM (future concern)

Currently no validation of:
- `SystemPrompt` content (could be extremely large, causing DoS)
- `WorkingDirectory` path (could escape intended boundaries)
- `Model` name (could request non-existent or unauthorized models)
- Message content length in Submit()

### Concern #2: Tool Registry Without Restrictions

**Severity:** LOW (future concern)
**Lines:** 43-66

Any tool can be registered without validation:
```go
for _, tool := range opts.Tools {
    toolRegistry.Register(tool)
}
```

No checks for:
- Duplicate tool names (could override default tools)
- Tool capabilities vs approval policies
- Tool sandboxing requirements

### Concern #3: Approval Cache Shared Across Sessions

**Severity:** LOW
**Lines:** 69, 91

```go
approvalCache := runtime.NewMemoryApprovalCache()
```

A single approval cache is shared across all sessions. This means:
- If user approves a dangerous operation in Session A, it's auto-approved in Session B
- No way to clear approvals for a specific session
- No expiration mechanism

**Recommendation:** Consider per-session approval caches or namespace the cache keys by session ID.

### Concern #4: No Rate Limiting

**Severity:** LOW (future concern)

No limits on:
- Number of sessions per SDK instance
- Message submission rate
- Token usage per session
- Concurrent operations

---

## 8. Additional Observations

### Good Practices

1. **Proper Locking:** Uses `sync.RWMutex` correctly with separate read/write locks
2. **Error Wrapping:** Uses `fmt.Errorf` with `%w` for proper error chains
3. **Clean Separation:** Clear distinction between SDK, Session, and internal components
4. **Type Safety:** Good use of structs instead of primitive types
5. **Defensive Copying:** Session.History() returns a deep copy to prevent modification

### Architecture Concerns

1. **Dual Session Management:** Both SDK and ConversationManager manage sessions independently. This seems redundant and error-prone.

2. **Unused Components:** The SDK creates a manager and orchestrator but Session never uses them. This suggests incomplete integration.

3. **Missing Abstraction:** Session directly depends on SDK, creating tight coupling. Consider an interface.

4. **No Metrics/Observability:** No instrumentation for monitoring session count, message throughput, errors, etc.

### Performance Considerations

1. **Session Map Growth:** Sessions are only removed on explicit close - no automatic cleanup. Long-running SDK instances could accumulate closed sessions if not managed properly.

2. **Deep Copy in History():** Every call creates a full copy of all messages. For long conversations, this could be expensive.

3. **Unbuffered Channel in Submit:** While SubmitStream uses buffered channel (size 10), there's no backpressure handling if consumer is slow.

---

## Priority Recommendations

### Must Fix (P0)

1. **Fix race condition in `generateSessionID()`** - Use atomic operations or UUID
2. **Implement core Submit/SubmitStream functionality** - Connect to manager and orchestrator
3. **Add SDK-level closed flag** - Prevent operations after Close()

### Should Fix (P1)

4. **Implement history persistence** - Or remove the unused fields
5. **Add input validation** - For session options and message content
6. **Fix manager/orchestrator configuration** - Ensure they share the same orchestrator
7. **Add comprehensive tests** - Especially for concurrency

### Nice to Have (P2)

8. **Improve documentation** - Add examples and clarify design decisions
9. **Add metrics/observability** - For monitoring and debugging
10. **Consider session lifecycle** - Auto-cleanup, max sessions, etc.
11. **Improve error aggregation** - Better formatting in Close()

---

## Conclusion

The `sdk.go` file provides a **well-structured but incomplete** foundation for the Codex SDK. The code quality is generally good with proper locking and error handling, but the **critical AI interaction functionality is not implemented**.

The most pressing issue is the **race condition in session ID generation**, which should be fixed immediately. Once the core functionality is implemented, focus should shift to validation, testing, and documentation.

**Overall Grade: C** (Good structure, incomplete implementation, one critical bug)

**Estimated Effort to Complete:**
- Fix critical race condition: 1 hour
- Implement core functionality: 2-3 days
- Add comprehensive tests: 2 days
- Complete documentation: 1 day

---

## Appendix: Related Files Reviewed

- `/Users/williamcory/codex/codex-go/pkg/sdk/session.go` - Session implementation
- `/Users/williamcory/codex/codex-go/pkg/sdk/options.go` - Configuration types
- `/Users/williamcory/codex/codex-go/pkg/sdk/sdk_test.go` - Test coverage
- `/Users/williamcory/codex/codex-go/internal/conversation/manager/manager.go` - Manager interface
- `/Users/williamcory/codex/codex-go/internal/tools/orchestrator/orchestrator.go` - Orchestrator interface
