# File Locking Strategy for History Persistence

## Overview

The history persistence layer uses advisory file locking to prevent data corruption when multiple processes access the same history file. This document describes the locking strategy, implementation details, and usage guidelines.

## Problem Statement

Without file locking, concurrent writes to the same history file from multiple processes can result in:
- Interleaved writes that corrupt JSON structure
- Race conditions where partial writes are visible to readers
- Data loss from overwritten buffers
- Non-deterministic file corruption

While the in-process mutex (`sync.Mutex`) prevents corruption within a single process, it provides no protection across multiple processes that may share the same history file.

## Solution: Advisory File Locking

We implement cross-process synchronization using advisory file locks:

### Lock Types

1. **Exclusive (Write) Lock**: Acquired by writers before write operations
   - Only one process can hold an exclusive lock at a time
   - Blocks other exclusive and shared locks
   - Used by `HistoryWriter.Append()` and `HistoryWriter.Flush()`

2. **Shared (Read) Lock**: Acquired by readers for consistent reads
   - Multiple processes can hold shared locks simultaneously
   - Blocks exclusive locks
   - Used by `HistoryReader.ReadAll()`

### Platform Implementation

#### Unix/Linux/macOS (POSIX)
- Uses `flock()` system call
- File: `filelock_unix.go` (build tag: `//go:build unix`)
- Lock types:
  - `LOCK_EX` - Exclusive lock
  - `LOCK_SH` - Shared lock
  - `LOCK_UN` - Unlock
  - `LOCK_NB` - Non-blocking flag

#### Windows
- Uses `LockFileEx()` and `UnlockFileEx()` Win32 API
- File: `filelock_windows.go` (build tag: `//go:build windows`)
- Lock flags:
  - `LOCKFILE_EXCLUSIVE_LOCK` - Exclusive lock
  - `LOCKFILE_SHARED_LOCK` - Shared lock (default)
  - `LOCKFILE_FAIL_IMMEDIATELY` - Non-blocking

## Implementation Details

### HistoryWriter

```go
type HistoryWriter struct {
    fs       afero.Fs
    path     string
    file     afero.File
    writer   *bufio.Writer
    mu       sync.Mutex    // In-process synchronization
    fileLock FileLock      // Cross-process synchronization
    closed   bool
}
```

**Locking Sequence in Append():**
1. Acquire in-process mutex (`mu.Lock()`)
2. Acquire exclusive file lock (`fileLock.Lock()`)
3. Marshal data
4. Write to buffer
5. Flush buffer
6. Sync to disk
7. Release file lock (`fileLock.Unlock()`)
8. Release in-process mutex (`mu.Unlock()`)

The file lock is held only for the duration of the write operation to minimize lock contention.

### HistoryReader

```go
type HistoryReader struct {
    fs       afero.Fs
    path     string
    file     afero.File
    scanner  *bufio.Scanner
    fileLock FileLock      // For consistent reads
    position int64
    closed   bool
}
```

**Locking Sequence in ReadAll():**
1. Acquire shared file lock (`fileLock.LockShared()`)
2. Read all entries
3. Release shared lock (`fileLock.Unlock()`)

Multiple readers can read simultaneously (shared lock), but writers are blocked during reads.

### Lock Timeout

All lock operations have a configurable timeout:
- Default: `DefaultLockTimeout = 5 seconds`
- Prevents indefinite blocking on stale locks
- Returns `LockError` on timeout

**Retry Logic:**
```go
deadline := time.Now().Add(timeout)
for {
    err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
    if err == nil {
        return nil  // Lock acquired
    }
    if time.Now().After(deadline) {
        return timeout error
    }
    time.Sleep(10 * time.Millisecond)  // Retry interval
}
```

## Filesystem Compatibility

### OS Filesystem (afero.OsFs)
- **File locking enabled**: Full cross-process protection
- The `file` is cast to `*os.File` and used to create a `FileLock`
- All lock operations are performed using OS-level APIs

### In-Memory/Mock Filesystems (afero.MemMapFs, etc.)
- **File locking disabled**: Gracefully degrades to in-process mutex only
- `fileLock` is `nil` when the file is not an `*os.File`
- Lock operations are skipped: `if w.fileLock != nil { ... }`
- This allows tests to use memory filesystems without modification

## Usage Examples

### Basic Writer Usage

```go
// Using OS filesystem - file locking enabled
fs := afero.NewOsFs()
writer, err := NewHistoryWriter(fs, "/path/to/history.jsonl")
if err != nil {
    return err
}
defer writer.Close()

// Each Append automatically acquires/releases locks
err = writer.Append(&protocol.Submission{
    ID: "test-1",
    Op: &protocol.OpInterrupt{},
})
```

### Concurrent Multi-Process Writes

```go
// Process 1
writer1, _ := NewHistoryWriter(fs, "/shared/history.jsonl")
writer1.Append(item1)  // Acquires exclusive lock
writer1.Close()

// Process 2 (concurrent)
writer2, _ := NewHistoryWriter(fs, "/shared/history.jsonl")
writer2.Append(item2)  // Waits for Process 1's lock to release
writer2.Close()
```

### Reader/Writer Coordination

```go
// Writer (Process 1)
writer.Append(item)  // Exclusive lock held during write

// Reader (Process 2) - concurrent
reader.ReadAll()  // Shared lock - blocks until write completes

// Multiple concurrent readers are allowed
reader1.ReadAll()  // Shared lock
reader2.ReadAll()  // Shared lock - can run simultaneously with reader1
```

## Error Handling

### Lock Timeout

```go
err := writer.Append(item)
if err != nil {
    var lockErr *LockError
    if errors.As(err, &lockErr) {
        // Handle lock timeout
        log.Printf("Failed to acquire lock: %v", lockErr)
        // Retry or fail gracefully
    }
}
```

### Lock Errors

`LockError` provides detailed context:
```go
type LockError struct {
    Operation string        // "lock", "lock_shared", "unlock"
    Path      string        // File path
    Timeout   time.Duration // Timeout duration
    Err       error         // Underlying error
}
```

## Testing

### Unit Tests
- `TestFileLockExclusive`: Verifies exclusive locks block concurrent access
- `TestFileLockShared`: Verifies shared locks allow concurrent reads
- `TestFileLockSharedBlocksExclusive`: Verifies read/write coordination
- `TestFileLockTryLock`: Tests non-blocking lock attempts

### Integration Tests
- `TestHistoryWriterConcurrentFileWrites`: Multiple goroutines writing
- `TestHistoryWriterMultiProcessSimulation`: Simulated multi-process access
- `TestMultiProcessRealSubProcess`: Real multi-process test (requires `go` command)

### Benchmarks
- `BenchmarkFileLockExclusive`: Lock acquisition overhead
- `BenchmarkHistoryWriterWithLocking`: End-to-end write performance

## Performance Considerations

### Lock Granularity
- Locks are held only during the actual I/O operation
- In-process mutex is held for the entire operation (marshal + write)
- File lock is held for write + flush + sync only

### Lock Contention
- Typical lock hold time: < 1ms for small writes
- Timeout of 5 seconds handles brief contention
- Retry interval of 10ms balances CPU usage and responsiveness

### Benchmarks (approximate)
```
BenchmarkFileLockExclusive        50000     ~30 μs/op
BenchmarkHistoryWriterWithLocking 10000    ~150 μs/op
```

Lock overhead is minimal compared to I/O operations.

## Limitations

### Advisory Locking
- File locks are **advisory**, not mandatory
- Non-cooperating processes can still corrupt the file
- All processes must use the `HistoryWriter`/`HistoryReader` API

### Network Filesystems
- Lock behavior on NFS, SMB, etc. may vary
- Some network filesystems don't support proper file locking
- Test thoroughly in your deployment environment

### Lock Lifetime
- Locks are automatically released when the file descriptor is closed
- If a process crashes while holding a lock, the OS releases it automatically
- No risk of permanent lock file starvation

## Troubleshooting

### "Timeout waiting for lock" errors
- **Cause**: Another process is holding the lock longer than expected
- **Solution**:
  - Increase timeout if legitimate long operations
  - Check for deadlocks or processes not releasing locks
  - Ensure `Close()` is called (use `defer`)

### Tests passing with MemMapFs but failing with OsFs
- **Cause**: File locking exposes race conditions that in-process mutex doesn't
- **Solution**: Review concurrent access patterns, ensure proper locking

### Lock contention in high-throughput scenarios
- **Cause**: Multiple processes writing to the same file frequently
- **Solution**:
  - Consider per-process history files with merge step
  - Batch writes to reduce lock acquisitions
  - Use write buffering (already implemented)

## Best Practices

1. **Always use `defer Close()`** to ensure locks are released:
   ```go
   writer, err := NewHistoryWriter(fs, path)
   if err != nil {
       return err
   }
   defer writer.Close()  // Ensures lock release
   ```

2. **Handle lock errors gracefully**:
   - Implement retry logic for transient failures
   - Log lock timeouts for debugging
   - Consider graceful degradation if locking fails

3. **Minimize lock hold time**:
   - Don't perform expensive operations while holding locks
   - Marshal data before acquiring locks if possible

4. **Test with real filesystem**:
   - In-memory tests don't exercise locking
   - Use `afero.NewOsFs()` in integration tests

5. **Monitor lock contention**:
   - Log slow lock acquisitions
   - Alert on frequent timeouts
   - Consider file-per-process if contention is high

## References

- POSIX `flock()`: https://man7.org/linux/man-pages/man2/flock.2.html
- Windows `LockFileEx()`: https://docs.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex
- Go `syscall` package: https://pkg.go.dev/syscall
