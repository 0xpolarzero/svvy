package persistence

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/afero"
)

// File permission constants for secure storage of sensitive conversation history.
// These permissions prevent unauthorized access to conversation data that may
// contain API keys, credentials, or other sensitive information.
const (
	// SensitiveFileMode (0600) allows only the owner to read/write sensitive files.
	// This prevents other users on the system from accessing conversation history
	// which may contain credentials, API keys, or confidential information.
	SensitiveFileMode os.FileMode = 0600

	// SensitiveDirMode (0700) allows only the owner to access session directories.
	// This ensures that session data and history files are protected from
	// unauthorized access by other users on multi-user systems.
	SensitiveDirMode os.FileMode = 0700
)

// HistoryWriter provides append-only writing to a history file in JSONL format.
// It is safe for concurrent use within a single process (via mutex) and across
// multiple processes (via file locking). All write operations acquire an exclusive
// file lock to ensure data integrity.
type HistoryWriter struct {
	fs       afero.Fs
	path     string
	file     afero.File
	writer   *bufio.Writer
	mu       sync.Mutex
	fileLock FileLock
	closed   bool
}

// NewHistoryWriter creates a new HistoryWriter for the given path.
// It creates parent directories if they don't exist and opens the file
// in append mode, creating it if necessary.
// Files are created with SensitiveFileMode (0600) to prevent unauthorized access.
//
// File locking is enabled when using the OS filesystem to prevent multi-process
// data corruption. When using in-memory or other non-OS filesystems (e.g., in tests),
// file locking is automatically disabled.
func NewHistoryWriter(fs afero.Fs, path string) (*HistoryWriter, error) {
	// Create parent directory if it doesn't exist
	// Use 0700 to ensure only the owner can access the session directory
	dir := filepath.Dir(path)
	if err := fs.MkdirAll(dir, SensitiveDirMode); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Open file in append mode (create if doesn't exist)
	// Use 0600 to prevent other users from reading sensitive conversation data
	file, err := fs.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, SensitiveFileMode)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", path, err)
	}

	// Create file lock if we have an *os.File (i.e., using real OS filesystem)
	// For in-memory or mock filesystems, fileLock will be nil and locking is skipped
	var fileLock FileLock
	if osFile, ok := file.(*os.File); ok {
		fileLock = newFileLock(osFile)
	}

	return &HistoryWriter{
		fs:       fs,
		path:     path,
		file:     file,
		writer:   bufio.NewWriter(file),
		fileLock: fileLock,
		closed:   false,
	}, nil
}

// Append writes a Submission or Event to the history file.
// Each item is written as a single JSON line followed by a newline.
// This method is thread-safe and multi-process safe.
//
// The method acquires both an in-process mutex and a cross-process file lock
// to prevent data corruption from concurrent writes. The file lock is held
// for the duration of the write operation.
func (w *HistoryWriter) Append(item interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	// Acquire exclusive file lock if available (skipped for non-OS filesystems)
	if w.fileLock != nil {
		if err := w.fileLock.Lock(DefaultLockTimeout); err != nil {
			return fmt.Errorf("failed to acquire file lock: %w", err)
		}
		defer w.fileLock.Unlock()
	}

	// Marshal the item
	data, err := MarshalHistoryLine(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	// Write the JSON line
	if _, err := w.writer.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	// Write newline
	if _, err := w.writer.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	// Flush to ensure data is written
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffer: %w", err)
	}

	// Sync to disk for durability (if supported by the filesystem)
	// Note: afero's memory filesystem doesn't support Sync, so we ignore errors
	if syncer, ok := w.file.(interface{ Sync() error }); ok {
		_ = syncer.Sync()
	}

	return nil
}

// Flush flushes any buffered data to the underlying file.
// This method is thread-safe and multi-process safe.
func (w *HistoryWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	// Acquire exclusive file lock if available (skipped for non-OS filesystems)
	if w.fileLock != nil {
		if err := w.fileLock.Lock(DefaultLockTimeout); err != nil {
			return fmt.Errorf("failed to acquire file lock: %w", err)
		}
		defer w.fileLock.Unlock()
	}

	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffer: %w", err)
	}

	// Sync to disk for durability
	if syncer, ok := w.file.(interface{ Sync() error }); ok {
		if err := syncer.Sync(); err != nil {
			return fmt.Errorf("failed to sync file: %w", err)
		}
	}

	return nil
}

// Close flushes any buffered data and closes the underlying file.
// This method is thread-safe and idempotent.
// It ensures any held file locks are released before closing the file.
func (w *HistoryWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true

	// Flush buffer
	if err := w.writer.Flush(); err != nil {
		// Best effort cleanup: unlock and close file
		if w.fileLock != nil {
			_ = w.fileLock.Unlock()
		}
		_ = w.file.Close()
		return fmt.Errorf("failed to flush buffer: %w", err)
	}

	// Unlock file before closing (if we happen to have a lock)
	// This is defensive - normally locks are released immediately after operations
	if w.fileLock != nil {
		_ = w.fileLock.Unlock()
	}

	// Close file
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	return nil
}

// Path returns the file path of this writer.
func (w *HistoryWriter) Path() string {
	return w.path
}
