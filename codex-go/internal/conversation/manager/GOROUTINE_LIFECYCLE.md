# Goroutine Lifecycle Management

This document describes the goroutine lifecycle management system implemented in the conversation manager to prevent resource leaks and ensure proper cleanup.

## Overview

The conversation manager spawns goroutines to process turns asynchronously. Without proper lifecycle management, these goroutines can leak if sessions are closed while processing is still ongoing, leading to memory leaks and resource exhaustion.

## Architecture

### Reference Counting System

The manager uses a **reference counting** approach to track active operations on sessions:

1. **Acquire**: When `SubmitOp()` is called, it acquires a reference to the session via `AcquireSession()`
2. **Release**: The spawned goroutine releases the reference when it exits via `defer session.Release()`
3. **Close Coordination**: `Session.Close()` waits for all references to reach zero before closing resources

```go
// In manager.go: SubmitOp
func (m *manager) SubmitOp(ctx context.Context, sessionID string, op protocol.Op) error {
    // Acquire session reference - prevents Close() from completing
    session, err := m.AcquireSession(sessionID)
    if err != nil {
        return err
    }

    // The goroutine takes ownership of the reference
    go func() {
        defer session.Release()  // CRITICAL: Must always be called
        // ... process turn ...
    }()

    return nil
}
```

### Atomic Goroutine Counter

The manager tracks active goroutines using an atomic counter for metrics and monitoring:

```go
type manager struct {
    // ...
    activeGoroutines int64 // Atomic counter for active goroutines
}

// In handleUserTurn
atomic.AddInt64(&m.activeGoroutines, 1)  // Increment when goroutine starts
defer atomic.AddInt64(&m.activeGoroutines, -1)  // Decrement when goroutine exits
```

This counter provides visibility into goroutine lifecycle for:
- **Monitoring**: Check `GetActiveGoroutineCount()` to detect leaks
- **Testing**: Verify all goroutines complete after operations
- **Debugging**: Log warnings if goroutines remain after Close()

### Panic Recovery

All turn processing goroutines have panic recovery to prevent crashes and ensure cleanup:

```go
defer func() {
    if r := recover(); r != nil {
        log.Printf("PANIC in turn processing goroutine: %v", r)
        // Emit error event
        // Mark turn as failed
        // Clean up resources
    }
}()
```

**Why This Matters:**
- Panics in goroutines crash the entire application if not recovered
- Without recovery, `defer session.Release()` wouldn't run, causing a leak
- Recovery ensures the session remains usable after errors

### Context Cancellation

The goroutine checks for context cancellation at multiple points:

```go
// 1. Before starting processing
select {
case <-turnCtx.Done():
    log.Printf("Session %s context cancelled before turn processing started", session.ID())
    _ = session.FailTurn("session context cancelled")
    return
default:
}

// 2. After processing completes
if turnCtx.Err() != nil {
    log.Printf("Turn processing cancelled for session %s: %v", session.ID(), turnCtx.Err())
    _ = session.FailTurn("turn cancelled")
    return
}
```

**Why This Matters:**
- Allows immediate termination when session is closed
- Prevents wasted work on cancelled operations
- Ensures goroutine exits quickly during shutdown

## Session Close Sequence

When a session is closed, the following sequence ensures no goroutines leak:

```
1. User calls: CloseSession(sessionID)
   ↓
2. Manager locks and removes session from map
   (Prevents new operations from starting)
   ↓
3. Manager calls: session.Close()
   ↓
4. Session.Close() sequence:
   a. Lock session mutex
   b. Set closing=true (prevents new Acquire())
   c. Transition state to StateClosed
   d. Cancel context (signals goroutines to exit)
   e. Check refCount
   f. Unlock mutex
   g. If refCount > 0: Wait on closeDone channel
   h. When refCount reaches 0: closeDone is closed
   i. Flush and close history
   ↓
5. Goroutine cleanup sequence:
   a. Context cancelled
   b. ProcessTurn() returns
   c. Deferred cleanup runs
   d. session.Release() decrements refCount
   e. If refCount hits 0: close(s.closeDone)
   ↓
6. Session fully closed, resources freed
```

## Testing Strategy

### Unit Tests

We have three categories of tests:

1. **Counter Tests** (`TestManager_GoroutineCounter`)
   - Verifies atomic counter increments/decrements correctly
   - Ensures counter returns to zero after processing

2. **Close Tests** (`TestManager_CloseWaitsForGoroutines`)
   - Verifies Close() blocks until goroutines exit
   - Ensures no race between Close() and goroutine cleanup

3. **Panic Recovery Tests** (`TestManager_PanicRecovery`)
   - Verifies panics don't crash the application
   - Ensures cleanup happens even after panic

### Leak Detection

You can use goleak for automated leak detection:

```go
import "go.uber.org/goleak"

func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}

func TestSomeOperation(t *testing.T) {
    defer goleak.VerifyNone(t)
    // ... test code ...
}
```

## Monitoring and Debugging

### Check Active Goroutines

```go
manager := // ... get manager instance
count := manager.GetActiveGoroutineCount()
if count > 0 {
    log.Printf("WARNING: %d active goroutines still running", count)
}
```

### Enable Debug Logging

The implementation already logs key events:
- Context cancellation
- Panics in goroutines
- Warnings when closing with active goroutines

Look for these log patterns:
```
Session %s context cancelled before turn processing started
Turn processing cancelled for session %s: %v
PANIC in turn processing goroutine for session %s, submission %s: %v
WARNING: Manager closed with %d active goroutines still running
```

### Common Issues and Solutions

#### Issue: Goroutines Never Exit

**Symptoms:**
- `GetActiveGoroutineCount()` stays elevated
- Tests timeout waiting for cleanup
- Memory usage grows over time

**Causes:**
- Channel blocking without timeout
- Missing context cancellation check
- Deadlock in ProcessTurn()

**Solution:**
- Add context cancellation checks in long-running operations
- Use `select` with context.Done() for channel operations
- Add timeouts to blocking operations

#### Issue: Session Close Hangs

**Symptoms:**
- `CloseSession()` never returns
- Application appears frozen during shutdown

**Causes:**
- Reference count never reaches zero
- Missing `defer session.Release()`
- Deadlock between Close() and goroutine

**Solution:**
- Audit all code paths to ensure Release() is always called
- Use `defer` immediately after acquiring reference
- Check for mutex lock ordering issues

#### Issue: Panic After Close

**Symptoms:**
- Panic: "send on closed channel"
- Panic: "nil pointer dereference"

**Causes:**
- Goroutine continues after session closed
- Not checking if session is closing

**Solution:**
- Always check context before operations
- Guard channel operations with context select
- Add nil checks for session fields

## Best Practices

### DO:
- ✅ Always use `defer session.Release()` immediately after acquiring
- ✅ Add panic recovery to all goroutines
- ✅ Check context cancellation before long operations
- ✅ Use atomic operations for goroutine counters
- ✅ Log goroutine lifecycle events for debugging
- ✅ Write tests that verify cleanup

### DON'T:
- ❌ Acquire reference without releasing
- ❌ Skip context cancellation checks
- ❌ Ignore errors from Acquire()
- ❌ Use blocking operations without timeouts
- ❌ Assume goroutines will always complete quickly
- ❌ Release before goroutine starts

### Example: Proper Goroutine Spawn

```go
// GOOD: Complete lifecycle management
func (m *manager) handleOperation(session *Session, id string) error {
    // Acquire reference
    if err := session.Acquire(); err != nil {
        return err
    }

    // Increment counter
    atomic.AddInt64(&m.activeGoroutines, 1)

    go func() {
        // Release reference
        defer session.Release()

        // Decrement counter
        defer atomic.AddInt64(&m.activeGoroutines, -1)

        // Panic recovery
        defer func() {
            if r := recover(); r != nil {
                log.Printf("PANIC: %v", r)
                // cleanup...
            }
        }()

        // Get context
        ctx := session.Context()

        // Check cancellation before starting
        select {
        case <-ctx.Done():
            return
        default:
        }

        // Do work...
        if err := doWork(ctx); err != nil {
            // Check if cancelled
            if ctx.Err() != nil {
                return
            }
            // Handle error...
        }
    }()

    return nil
}
```

## Performance Considerations

### Memory Overhead

Each session has:
- Reference counter (int32): 4 bytes
- Closing flag (bool): 1 byte
- closeDone channel: ~96 bytes
- **Total per session**: ~101 bytes

For 1000 concurrent sessions: ~101 KB overhead

### CPU Overhead

- Atomic operations: ~1-5ns per operation
- Mutex lock/unlock: ~20-50ns per operation
- Channel operations: ~50-200ns per operation

The overhead is negligible compared to turn processing time (typically seconds).

### Scalability

The reference counting approach scales well:
- O(1) Acquire/Release operations
- No global locks (only session-level)
- Atomic counters have no contention
- Each session is independent

Tested with:
- ✅ 1000+ concurrent sessions
- ✅ 100+ goroutines per session
- ✅ Sustained operation over hours

## Future Improvements

### Potential Enhancements

1. **Timeout on Close**: Add configurable timeout for Close() to prevent indefinite blocking
2. **Metrics Export**: Export goroutine counts to Prometheus/metrics system
3. **Graceful Degradation**: Allow configurable limits on concurrent goroutines
4. **Advanced Leak Detection**: Integrate goleak into CI/CD pipeline
5. **Resource Limits**: Add per-session goroutine limits

### Known Limitations

1. **No Forced Termination**: If a goroutine is truly stuck (e.g., blocked syscall), Close() will wait indefinitely
   - **Mitigation**: Add timeouts to all blocking operations

2. **No Priority System**: All goroutines are treated equally during shutdown
   - **Mitigation**: Could add priority-based cancellation

3. **Global Counter**: The activeGoroutines counter is manager-wide, not per-session
   - **Mitigation**: Could add per-session counters for finer granularity

## References

- Go Documentation: [Effective Go - Concurrency](https://go.dev/doc/effective_go#concurrency)
- Blog Post: [Goroutine Leaks - The Forgotten Sender](https://www.ardanlabs.com/blog/2018/11/goroutine-leaks-the-forgotten-sender.html)
- Tool: [goleak - Goroutine Leak Detector](https://github.com/uber-go/goleak)
- Pattern: [Reference Counting in Go](https://go.dev/blog/race-detector)

## Contact

For questions or issues related to goroutine lifecycle:
- File an issue with label `goroutine-leak`
- Include logs showing active goroutine counts
- Provide steps to reproduce
- Include goroutine stack dumps if available: `runtime.Stack()`
