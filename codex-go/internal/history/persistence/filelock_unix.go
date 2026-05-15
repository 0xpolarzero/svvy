//go:build unix

package persistence

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

// unixFileLock implements FileLock using flock() system call on Unix-like systems.
type unixFileLock struct {
	file   *os.File
	locked bool
}

// newFileLock creates a new FileLock for the given file.
// The file must be opened before calling this function.
func newFileLock(file *os.File) FileLock {
	return &unixFileLock{
		file:   file,
		locked: false,
	}
}

// Lock acquires an exclusive (write) lock on the file.
func (l *unixFileLock) Lock(timeout time.Duration) error {
	if l.file == nil {
		return &LockError{
			Operation: "lock",
			Path:      "<nil>",
			Timeout:   timeout,
			Err:       errors.New("file is nil"),
		}
	}

	deadline := time.Now().Add(timeout)
	for {
		// Try to acquire exclusive lock (LOCK_EX)
		err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			l.locked = true
			return nil
		}

		// If the error is not "would block", return it
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return &LockError{
				Operation: "lock",
				Path:      l.file.Name(),
				Timeout:   timeout,
				Err:       err,
			}
		}

		// Check if we've exceeded the timeout
		if time.Now().After(deadline) {
			return &LockError{
				Operation: "lock",
				Path:      l.file.Name(),
				Timeout:   timeout,
				Err:       fmt.Errorf("timeout waiting for exclusive lock"),
			}
		}

		// Wait a bit before retrying
		time.Sleep(10 * time.Millisecond)
	}
}

// LockShared acquires a shared (read) lock on the file.
func (l *unixFileLock) LockShared(timeout time.Duration) error {
	if l.file == nil {
		return &LockError{
			Operation: "lock_shared",
			Path:      "<nil>",
			Timeout:   timeout,
			Err:       errors.New("file is nil"),
		}
	}

	deadline := time.Now().Add(timeout)
	for {
		// Try to acquire shared lock (LOCK_SH)
		err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
		if err == nil {
			l.locked = true
			return nil
		}

		// If the error is not "would block", return it
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return &LockError{
				Operation: "lock_shared",
				Path:      l.file.Name(),
				Timeout:   timeout,
				Err:       err,
			}
		}

		// Check if we've exceeded the timeout
		if time.Now().After(deadline) {
			return &LockError{
				Operation: "lock_shared",
				Path:      l.file.Name(),
				Timeout:   timeout,
				Err:       fmt.Errorf("timeout waiting for shared lock"),
			}
		}

		// Wait a bit before retrying
		time.Sleep(10 * time.Millisecond)
	}
}

// Unlock releases the lock.
func (l *unixFileLock) Unlock() error {
	if l.file == nil || !l.locked {
		return nil
	}

	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	if err != nil {
		return &LockError{
			Operation: "unlock",
			Path:      l.file.Name(),
			Err:       err,
		}
	}

	l.locked = false
	return nil
}

// TryLock attempts to acquire an exclusive lock without blocking.
func (l *unixFileLock) TryLock() (bool, error) {
	if l.file == nil {
		return false, &LockError{
			Operation: "try_lock",
			Path:      "<nil>",
			Err:       errors.New("file is nil"),
		}
	}

	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		l.locked = true
		return true, nil
	}

	// If the error is "would block", the lock is held by another process
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return false, nil
	}

	// Other errors are real errors
	return false, &LockError{
		Operation: "try_lock",
		Path:      l.file.Name(),
		Err:       err,
	}
}

// TryLockShared attempts to acquire a shared lock without blocking.
func (l *unixFileLock) TryLockShared() (bool, error) {
	if l.file == nil {
		return false, &LockError{
			Operation: "try_lock_shared",
			Path:      "<nil>",
			Err:       errors.New("file is nil"),
		}
	}

	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
	if err == nil {
		l.locked = true
		return true, nil
	}

	// If the error is "would block", the lock is held by another process
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return false, nil
	}

	// Other errors are real errors
	return false, &LockError{
		Operation: "try_lock_shared",
		Path:      l.file.Name(),
		Err:       err,
	}
}
