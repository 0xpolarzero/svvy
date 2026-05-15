package persistence

import (
	"fmt"
	"time"
)

// FileLock represents a file lock that can be acquired and released.
// It supports both exclusive (write) and shared (read) locks for cross-process synchronization.
//
// The file locking mechanism used here is advisory, meaning that all processes must cooperate
// by using the same locking protocol. The lock does not prevent non-cooperating processes
// from modifying the file.
//
// Implementation details:
// - Uses flock() on Unix-like systems (Linux, macOS, BSD)
// - Uses LockFileEx() on Windows
// - Supports lock timeouts to prevent indefinite blocking
// - Automatically releases locks when the file is closed
type FileLock interface {
	// Lock acquires an exclusive (write) lock on the file.
	// If the lock cannot be acquired within the timeout, returns an error.
	// An exclusive lock prevents other processes from acquiring either shared or exclusive locks.
	Lock(timeout time.Duration) error

	// LockShared acquires a shared (read) lock on the file.
	// If the lock cannot be acquired within the timeout, returns an error.
	// Multiple processes can hold shared locks simultaneously, but no exclusive lock can be acquired.
	LockShared(timeout time.Duration) error

	// Unlock releases the lock.
	// It is safe to call Unlock multiple times or on an unlocked file.
	Unlock() error

	// TryLock attempts to acquire an exclusive lock without blocking.
	// Returns true if the lock was acquired, false otherwise.
	TryLock() (bool, error)

	// TryLockShared attempts to acquire a shared lock without blocking.
	// Returns true if the lock was acquired, false otherwise.
	TryLockShared() (bool, error)
}

// LockError represents an error that occurred during lock operations.
type LockError struct {
	Operation string        // The operation that failed (e.g., "lock", "unlock")
	Path      string        // The file path
	Timeout   time.Duration // The timeout duration (if applicable)
	Err       error         // The underlying error
}

func (e *LockError) Error() string {
	if e.Timeout > 0 {
		return fmt.Sprintf("failed to %s file %s within %v: %v", e.Operation, e.Path, e.Timeout, e.Err)
	}
	return fmt.Sprintf("failed to %s file %s: %v", e.Operation, e.Path, e.Err)
}

func (e *LockError) Unwrap() error {
	return e.Err
}

// DefaultLockTimeout is the default timeout for lock operations.
// This value balances responsiveness with the need to handle temporary lock contention.
const DefaultLockTimeout = 5 * time.Second
