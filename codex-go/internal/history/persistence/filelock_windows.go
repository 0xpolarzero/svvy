//go:build windows

package persistence

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

const (
	// Lock flags for LockFileEx
	lockfileFailImmediately = 0x00000001
	lockfileExclusiveLock   = 0x00000002
	lockfileSharedLock      = 0x00000000

	// Windows error codes
	errorLockViolation = 33 // ERROR_LOCK_VIOLATION
)

// windowsFileLock implements FileLock using LockFileEx() on Windows.
type windowsFileLock struct {
	file   *os.File
	locked bool
}

// newFileLock creates a new FileLock for the given file.
func newFileLock(file *os.File) FileLock {
	return &windowsFileLock{
		file:   file,
		locked: false,
	}
}

// lockFileEx calls the Windows LockFileEx API.
func lockFileEx(handle syscall.Handle, flags, reserved, lockLow, lockHigh uint32, overlapped *syscall.Overlapped) error {
	r1, _, err := procLockFileEx.Call(
		uintptr(handle),
		uintptr(flags),
		uintptr(reserved),
		uintptr(lockLow),
		uintptr(lockHigh),
		uintptr(unsafe.Pointer(overlapped)),
	)
	if r1 == 0 {
		return err
	}
	return nil
}

// unlockFileEx calls the Windows UnlockFileEx API.
func unlockFileEx(handle syscall.Handle, reserved, lockLow, lockHigh uint32, overlapped *syscall.Overlapped) error {
	r1, _, err := procUnlockFileEx.Call(
		uintptr(handle),
		uintptr(reserved),
		uintptr(lockLow),
		uintptr(lockHigh),
		uintptr(unsafe.Pointer(overlapped)),
	)
	if r1 == 0 {
		return err
	}
	return nil
}

// Lock acquires an exclusive (write) lock on the file.
func (l *windowsFileLock) Lock(timeout time.Duration) error {
	if l.file == nil {
		return &LockError{
			Operation: "lock",
			Path:      "<nil>",
			Timeout:   timeout,
			Err:       errors.New("file is nil"),
		}
	}

	handle := syscall.Handle(l.file.Fd())
	overlapped := &syscall.Overlapped{}

	deadline := time.Now().Add(timeout)
	for {
		// Try to acquire exclusive lock
		err := lockFileEx(handle, lockfileExclusiveLock|lockfileFailImmediately, 0, 1, 0, overlapped)
		if err == nil {
			l.locked = true
			return nil
		}

		// Check if it's a "lock violation" error (someone else has the lock)
		if errno, ok := err.(syscall.Errno); !ok || errno != errorLockViolation {
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
func (l *windowsFileLock) LockShared(timeout time.Duration) error {
	if l.file == nil {
		return &LockError{
			Operation: "lock_shared",
			Path:      "<nil>",
			Timeout:   timeout,
			Err:       errors.New("file is nil"),
		}
	}

	handle := syscall.Handle(l.file.Fd())
	overlapped := &syscall.Overlapped{}

	deadline := time.Now().Add(timeout)
	for {
		// Try to acquire shared lock
		err := lockFileEx(handle, lockfileSharedLock|lockfileFailImmediately, 0, 1, 0, overlapped)
		if err == nil {
			l.locked = true
			return nil
		}

		// Check if it's a "lock violation" error
		if errno, ok := err.(syscall.Errno); !ok || errno != errorLockViolation {
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
func (l *windowsFileLock) Unlock() error {
	if l.file == nil || !l.locked {
		return nil
	}

	handle := syscall.Handle(l.file.Fd())
	overlapped := &syscall.Overlapped{}

	err := unlockFileEx(handle, 0, 1, 0, overlapped)
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
func (l *windowsFileLock) TryLock() (bool, error) {
	if l.file == nil {
		return false, &LockError{
			Operation: "try_lock",
			Path:      "<nil>",
			Err:       errors.New("file is nil"),
		}
	}

	handle := syscall.Handle(l.file.Fd())
	overlapped := &syscall.Overlapped{}

	err := lockFileEx(handle, lockfileExclusiveLock|lockfileFailImmediately, 0, 1, 0, overlapped)
	if err == nil {
		l.locked = true
		return true, nil
	}

	// If it's a lock violation, the lock is held by another process
	if errno, ok := err.(syscall.Errno); ok && errno == errorLockViolation {
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
func (l *windowsFileLock) TryLockShared() (bool, error) {
	if l.file == nil {
		return false, &LockError{
			Operation: "try_lock_shared",
			Path:      "<nil>",
			Err:       errors.New("file is nil"),
		}
	}

	handle := syscall.Handle(l.file.Fd())
	overlapped := &syscall.Overlapped{}

	err := lockFileEx(handle, lockfileSharedLock|lockfileFailImmediately, 0, 1, 0, overlapped)
	if err == nil {
		l.locked = true
		return true, nil
	}

	// If it's a lock violation, the lock is held by another process
	if errno, ok := err.(syscall.Errno); ok && errno == errorLockViolation {
		return false, nil
	}

	// Other errors are real errors
	return false, &LockError{
		Operation: "try_lock_shared",
		Path:      l.file.Name(),
		Err:       err,
	}
}
