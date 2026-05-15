package persistence

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/spf13/afero"
)

// HistoryReader provides reading from a history file in JSONL format.
// It supports both line-by-line reading and batch reading.
// When using the OS filesystem, it acquires shared (read) locks to coordinate
// with writers and prevent reading partially written data.
type HistoryReader struct {
	fs       afero.Fs
	path     string
	file     afero.File
	scanner  *bufio.Scanner
	fileLock FileLock
	position int64
	closed   bool
}

// NewHistoryReader creates a new HistoryReader for the given path.
// File locking is enabled when using the OS filesystem to coordinate with writers.
func NewHistoryReader(fs afero.Fs, path string) (*HistoryReader, error) {
	file, err := fs.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", path, err)
	}

	// Create file lock if we have an *os.File (i.e., using real OS filesystem)
	// For in-memory or mock filesystems, fileLock will be nil and locking is skipped
	var fileLock FileLock
	if osFile, ok := file.(*os.File); ok {
		fileLock = newFileLock(osFile)
	}

	return &HistoryReader{
		fs:       fs,
		path:     path,
		file:     file,
		scanner:  bufio.NewScanner(file),
		fileLock: fileLock,
		position: 0,
		closed:   false,
	}, nil
}

// ReadNext reads the next Submission or Event from the file.
// Returns (submission, nil, nil) for submissions, (nil, event, nil) for events,
// or (nil, nil, error) on failure or EOF.
func (r *HistoryReader) ReadNext() (*protocol.Submission, *protocol.Event, error) {
	if r.closed {
		return nil, nil, fmt.Errorf("reader is closed")
	}

	for r.scanner.Scan() {
		line := r.scanner.Bytes()
		r.position += int64(len(line)) + 1 // +1 for newline

		// Skip empty lines
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		// Parse the line
		sub, evt, err := UnmarshalHistoryLine(line)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse line at position %d: %w", r.position, err)
		}

		return sub, evt, nil
	}

	if err := r.scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("scanner error: %w", err)
	}

	return nil, nil, io.EOF
}

// ReadAll reads all Submissions and Events from the file.
// Returns separate slices for submissions and events.
// When using the OS filesystem, this method acquires a shared lock for the duration
// of the read operation to ensure consistent data.
func (r *HistoryReader) ReadAll() ([]*protocol.Submission, []*protocol.Event, error) {
	// Acquire shared lock if available (skipped for non-OS filesystems)
	// This ensures that no writer can modify the file while we're reading
	if r.fileLock != nil {
		if err := r.fileLock.LockShared(DefaultLockTimeout); err != nil {
			return nil, nil, fmt.Errorf("failed to acquire shared lock: %w", err)
		}
		defer r.fileLock.Unlock()
	}

	var submissions []*protocol.Submission
	var events []*protocol.Event

	for {
		sub, evt, err := r.ReadNext()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}

		if sub != nil {
			submissions = append(submissions, sub)
		}
		if evt != nil {
			events = append(events, evt)
		}
	}

	return submissions, events, nil
}

// Position returns the current byte position in the file.
func (r *HistoryReader) Position() int64 {
	return r.position
}

// Close closes the underlying file.
// It ensures any held file locks are released before closing.
func (r *HistoryReader) Close() error {
	if r.closed {
		return nil
	}

	r.closed = true

	// Unlock file before closing (defensive, in case a lock is held)
	if r.fileLock != nil {
		_ = r.fileLock.Unlock()
	}

	if err := r.file.Close(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	return nil
}

// Path returns the file path of this reader.
func (r *HistoryReader) Path() string {
	return r.path
}
