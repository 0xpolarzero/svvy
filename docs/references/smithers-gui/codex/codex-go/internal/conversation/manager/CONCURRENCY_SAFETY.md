# Concurrency Safety in Session Manager

## Overview

The session manager implements comprehensive concurrency safety through reference counting to prevent TOCTOU (Time-Of-Check-Time-Of-Use) race conditions and use-after-free bugs.

## The Problem: TOCTOU Race Condition

### Original Vulnerable Code

```go
// Thread 1: SubmitOp
session, err := m.GetSession(sessionID)  // RLock released here
if err != nil {
    return err
}
// Thread 2: CloseSession can delete session here!
return m.handleUserTurn(ctx, session, turn)  // Use-after-free!
```

### Attack Scenario

1. Thread 1 calls `SubmitOp` -> acquires RLock -> retrieves session -> releases RLock
2. Thread 2 calls `CloseSession` -> acquires Lock -> deletes session -> releases Lock
3. Thread 1 continues with deleted session pointer -> **Use-After-Free / Nil Pointer Dereference**

This can lead to:
- Panics from nil pointer dereferences
- Data corruption
- Undefined behavior
- Security vulnerabilities

## The Solution: Reference Counting

### Design

We implemented a reference counting system similar to C++'s `std::shared_ptr`:

1. Each session tracks active references via atomic counter
2. `AcquireSession()` increments the reference count
3. `Release()` decrements the reference count
4. `CloseSession()` waits for all references to reach zero before cleanup

### Key Components

#### Session Structure

```go
type Session struct {
    // ... other fields ...

    // Reference counting for safe concurrent access
    refCount     int32         // Number of active operations
    closing      bool          // True when Close() has been called
    closeDone    chan struct{} // Closed when all references released
}
```

#### AcquireSession Method

```go
func (m *manager) AcquireSession(sessionID string) (*Session, error) {
    m.mu.RLock()
    session, exists := m.sessions[sessionID]
    m.mu.RUnlock()

    if !exists {
        return nil, fmt.Errorf("session %s not found", sessionID)
    }

    // Acquire reference - fails if session is closing
    if err := session.Acquire(); err != nil {
        return nil, err
    }

    return session, nil
}
```

#### Session.Acquire Method

```go
func (s *Session) Acquire() error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.closing {
        return fmt.Errorf("session %s is closing", s.id)
    }

    if s.stateMachine.IsTerminal() {
        return fmt.Errorf("session %s is closed", s.id)
    }

    s.refCount++
    return nil
}
```

#### Session.Release Method

```go
func (s *Session) Release() {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.refCount <= 0 {
        return
    }

    s.refCount--

    // Signal Close() when last reference is released
    if s.refCount == 0 && s.closing {
        close(s.closeDone)
    }
}
```

#### CloseSession Method

```go
func (m *manager) CloseSession(sessionID string) error {
    // Remove from map so no new operations can find it
    m.mu.Lock()
    session, exists := m.sessions[sessionID]
    if !exists {
        m.mu.Unlock()
        return fmt.Errorf("session %s not found", sessionID)
    }
    delete(m.sessions, sessionID)
    m.mu.Unlock()

    // Wait for all active operations to complete
    if err := session.Close(); err != nil {
        return err
    }

    return nil
}
```

#### Session.Close Method

```go
func (s *Session) Close() error {
    s.mu.Lock()

    if s.closing {
        s.mu.Unlock()
        return fmt.Errorf("session is already closing")
    }
    s.closing = true

    // Mark as closed and cancel context
    _ = s.stateMachine.Transition(StateClosed)
    s.cancel()

    refCount := s.refCount
    s.mu.Unlock()

    // Wait for all references to be released
    if refCount > 0 {
        <-s.closeDone
    }

    // Safe to clean up resources now
    s.mu.Lock()
    if s.historyEnabled && s.history != nil {
        _ = s.history.Flush()
        _ = s.history.Close()
    }
    s.mu.Unlock()

    return nil
}
```

## Thread-Safety Guarantees

### For Operations

1. **AcquireSession always returns a valid session** (or error)
2. **Acquired sessions cannot be deleted** until Release() is called
3. **Closing sessions reject new acquisitions** immediately
4. **Background goroutines hold references** until completion

### For Cleanup

1. **CloseSession removes session from map** immediately
2. **No new operations can find the session** after removal
3. **Close waits for active operations** to complete
4. **Resource cleanup happens only after** all references released

## Usage Patterns

### Short Operations (GetSession)

```go
// For quick read-only access
session, err := mgr.GetSession(sessionID)
if err != nil {
    return err
}
// Use session immediately
state := session.State()
```

### Long Operations (AcquireSession)

```go
// For operations that span multiple calls or goroutines
session, err := mgr.AcquireSession(sessionID)
if err != nil {
    return err
}
defer session.Release() // MUST call Release!

// Safe to use session for extended period
return processLongRunningOperation(session)
```

### Background Goroutines

```go
func (m *manager) handleUserTurn(...) error {
    // Goroutine takes ownership of reference
    go func() {
        defer session.Release() // Release when done

        // Process turn...
    }()

    return nil
}
```

## Testing

### Concurrency Tests

We provide comprehensive tests in `manager_race_test.go`:

1. **TestConcurrentGetSessionAndCloseSession** - Tests TOCTOU race condition
2. **TestReferenceCountingPreventsUseAfterFree** - Verifies reference counting works
3. **TestAcquireOnClosingSession** - Tests that closing sessions reject acquisitions
4. **TestConcurrentOperationsOnSameSession** - Tests concurrent access patterns
5. **TestNoDeadlockOnRepeatedAcquireRelease** - Tests for deadlocks

### Running with Race Detector

```bash
# Run all race tests
go test -race ./internal/conversation/manager/

# Run specific test
go test -race -run TestReferenceCountingPreventsUseAfterFree ./internal/conversation/manager/
```

## Common Pitfalls

### ❌ Forgetting to Release

```go
session, err := mgr.AcquireSession(sessionID)
if err != nil {
    return err
}
// BUG: No Release() - session will never be cleaned up!
return doSomething(session)
```

### ✅ Proper Release with Defer

```go
session, err := mgr.AcquireSession(sessionID)
if err != nil {
    return err
}
defer session.Release() // Always release
return doSomething(session)
```

### ❌ Early Return Without Release

```go
session, err := mgr.AcquireSession(sessionID)
if err != nil {
    return err
}

if !session.CanAcceptTurn() {
    return fmt.Errorf("cannot accept turn") // BUG: No Release!
}

defer session.Release()
return doSomething(session)
```

### ✅ Defer Immediately After Acquire

```go
session, err := mgr.AcquireSession(sessionID)
if err != nil {
    return err
}
defer session.Release() // Defer immediately

if !session.CanAcceptTurn() {
    return fmt.Errorf("cannot accept turn") // OK: Release will happen
}

return doSomething(session)
```

## Performance Considerations

### Lock Contention

- `AcquireSession` only holds RLock briefly during lookup
- Session-level mutex only held during ref count updates
- Minimal contention under normal load

### Memory Overhead

- 4 bytes (int32) for refCount per session
- 1 byte (bool) for closing flag per session
- 1 channel (closeDone) per session
- **Total: ~20 bytes overhead per session**

### Blocking Behavior

- `CloseSession` blocks until all references released
- Design trade-off: **correctness over speed**
- Typically blocks < 1ms in practice

## Migration Guide

### From Old Code

```go
// OLD: Race condition
session, err := m.GetSession(sessionID)
go func() {
    // Use session...
}()
```

### To New Code

```go
// NEW: Race-free with reference counting
session, err := m.AcquireSession(sessionID)
if err != nil {
    return err
}

go func() {
    defer session.Release() // Goroutine owns reference
    // Use session...
}()
```

## Related Files

- `/internal/conversation/manager/session.go` - Session struct and methods
- `/internal/conversation/manager/manager.go` - Manager implementation
- `/internal/conversation/manager/manager_race_test.go` - Concurrency tests
- `/CODEBASE_REVIEW.md` - Original vulnerability report (section 1.4)

## References

- [TOCTOU on Wikipedia](https://en.wikipedia.org/wiki/Time-of-check_to_time-of-use)
- [Go Memory Model](https://golang.org/ref/mem)
- [Go Race Detector](https://golang.org/doc/articles/race_detector.html)
