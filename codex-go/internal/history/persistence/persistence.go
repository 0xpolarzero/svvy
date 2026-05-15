// Package persistence provides history persistence functionality for Codex sessions.
//
// This package handles reading and writing session history to disk in JSONL format,
// with support for rollouts (timestamped snapshots), atomic writes, and fsync for
// durability.
//
// Key features:
// - Append-only JSONL format (one Op/Event per line)
// - Atomic writes with fsync for durability
// - Rollout support (history.jsonl.1234567890 snapshots)
// - Session directory management (~/.codex/sessions/)
// - Resume from history (replay Ops/Events)
// - Concurrent read safety
// - Uses afero.Fs abstraction for testing
package persistence

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/spf13/afero"
)

var (
	// ErrInvalidPath indicates a path validation error
	ErrInvalidPath = errors.New("invalid path")
	// ErrPathTraversal indicates a path traversal attempt
	ErrPathTraversal = errors.New("path traversal detected")
	// ErrEmptyPath indicates an empty path was provided
	ErrEmptyPath = errors.New("path cannot be empty")
	// ErrClosed indicates the persistence instance is closed
	ErrClosed = errors.New("history persistence is closed")
)

// ValidateSafePath validates that a path is safe for use.
// It checks for:
// - Empty paths
// - Path traversal attempts (..)
// - Absolute paths (when required)
// - Valid characters
func ValidateSafePath(path string, requireAbsolute bool) error {
	if path == "" {
		return fmt.Errorf("%w: path is empty", ErrEmptyPath)
	}

	// Check for path traversal attempts BEFORE cleaning
	// This catches both "/sessions/../etc" and "sessions/../etc"
	if strings.Contains(path, "..") {
		return fmt.Errorf("%w: path contains '..'", ErrPathTraversal)
	}

	// Clean the path to normalize it
	cleaned := filepath.Clean(path)

	// Double-check after cleaning (filepath.Clean should not introduce ..)
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("%w: path contains '..' after normalization", ErrPathTraversal)
	}

	// Check if absolute path is required
	if requireAbsolute && !filepath.IsAbs(cleaned) {
		return fmt.Errorf("%w: path must be absolute: %q", ErrInvalidPath, path)
	}

	// Validate the path doesn't contain suspicious characters
	// Allow alphanumeric, hyphens, underscores, periods, and path separators
	for _, r := range cleaned {
		if !(r == filepath.Separator || r == '/' || r == '.' || r == '-' || r == '_' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			// Allow spaces in paths but nothing else suspicious
			if r != ' ' && r != ':' {
				return fmt.Errorf("%w: path contains invalid character: %q", ErrInvalidPath, r)
			}
		}
	}

	return nil
}

// EnsureWithinRoot validates that targetPath is within rootPath.
// This prevents path traversal attacks where an attacker tries to access
// files outside the intended directory tree.
func EnsureWithinRoot(rootPath, targetPath string) error {
	// Clean both paths
	cleanRoot := filepath.Clean(rootPath)
	cleanTarget := filepath.Clean(targetPath)

	// Make both absolute for comparison
	if !filepath.IsAbs(cleanRoot) {
		return fmt.Errorf("%w: root path must be absolute", ErrInvalidPath)
	}

	if !filepath.IsAbs(cleanTarget) {
		return fmt.Errorf("%w: target path must be absolute", ErrInvalidPath)
	}

	// Check if target starts with root
	rel, err := filepath.Rel(cleanRoot, cleanTarget)
	if err != nil {
		return fmt.Errorf("%w: failed to compute relative path: %v", ErrInvalidPath, err)
	}

	// If the relative path starts with "..", it's outside the root
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("%w: path is outside root directory", ErrPathTraversal)
	}

	return nil
}

// validateSessionDir performs comprehensive validation of a session directory path.
// It checks for security issues and returns the cleaned path and extracted session ID.
func validateSessionDir(fs afero.Fs, sessionDir string) (cleanPath string, sessionID string, err error) {
	// Basic path validation
	if err := ValidateSafePath(sessionDir, true); err != nil {
		return "", "", fmt.Errorf("invalid session directory: %w", err)
	}

	// Clean the path
	cleanPath = filepath.Clean(sessionDir)

	// Extract and validate session ID
	sessionID = filepath.Base(cleanPath)
	if sessionID == "" || sessionID == "." || sessionID == "/" || sessionID == string(filepath.Separator) {
		return "", "", fmt.Errorf("%w: cannot extract valid session ID from %q", ErrInvalidPath, sessionDir)
	}

	// Session ID should not contain path separators
	if strings.ContainsAny(sessionID, `/\`) {
		return "", "", fmt.Errorf("%w: session ID cannot contain path separators: %q", ErrInvalidPath, sessionID)
	}

	// Check if path exists and verify it's not a symlink
	// Try Lstater first for symlink detection, fall back to Stat
	var info os.FileInfo
	if lstater, ok := fs.(afero.Lstater); ok {
		info, _, err = lstater.LstatIfPossible(cleanPath)
	} else {
		info, err = fs.Stat(cleanPath)
	}

	if err != nil {
		// If it doesn't exist, that's ok - we'll create it
		if os.IsNotExist(err) {
			return cleanPath, sessionID, nil
		}
		return "", "", fmt.Errorf("failed to stat session directory: %w", err)
	}

	// If it exists, verify it's a directory and not a symlink
	if info.Mode()&os.ModeSymlink != 0 {
		return "", "", fmt.Errorf("%w: session directory is a symlink", ErrInvalidPath)
	}

	if !info.IsDir() {
		return "", "", fmt.Errorf("%w: session path exists but is not a directory", ErrInvalidPath)
	}

	return cleanPath, sessionID, nil
}

// HistoryPersistence manages persistence of session history to disk.
// It provides high-level operations for recording and loading history,
// managing rollouts, and accessing session metadata.
//
// Thread-safety: Recording methods are thread-safe. However, Close() should
// be coordinated with active writes by the caller to prevent errors.
type HistoryPersistence struct {
	mu         sync.RWMutex
	fs         afero.Fs
	sessionDir string
	sessionID  string
	writer     *HistoryWriter
	closed     bool
}

// NewHistoryPersistence creates a new HistoryPersistence for the given session directory.
//
// The session directory must be an absolute path to the session (e.g., ~/.codex/sessions/session-id).
// It creates the directory if it doesn't exist and opens the history file for writing in append mode,
// preserving any existing history.
//
// Security:
// - Validates the session directory path to prevent path traversal attacks
// - Rejects paths containing ".." or other suspicious patterns
// - Verifies symlinks are not used (prevents symlink attacks)
// - Creates directories with SensitiveDirMode (0700) to protect session data
// - Creates history files with SensitiveFileMode (0600) for owner-only access
//
// Multiple instances should NOT be created for the same session directory as this
// could lead to data corruption despite file locking.
//
// Thread-safety: Recording methods (RecordSubmission, RecordEvent) are thread-safe within
// a single instance. Close/Flush operations should be coordinated with writes by the caller.
func NewHistoryPersistence(fs afero.Fs, sessionDir string) (*HistoryPersistence, error) {
	// Validate and clean the session directory path
	cleanPath, sessionID, err := validateSessionDir(fs, sessionDir)
	if err != nil {
		return nil, fmt.Errorf("invalid session directory: %w", err)
	}

	// Create session directory if it doesn't exist
	// Use 0700 to ensure only the owner can access session data
	if err := fs.MkdirAll(cleanPath, SensitiveDirMode); err != nil {
		return nil, fmt.Errorf("failed to create session directory %q: %w", sessionID, err)
	}

	// Verify directory permissions if it already existed
	info, err := fs.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat session directory: %w", err)
	}

	// Check if permissions are correct, try to fix if not
	if info.Mode().Perm() != SensitiveDirMode {
		if err := fs.Chmod(cleanPath, SensitiveDirMode); err != nil {
			return nil, fmt.Errorf("session directory has incorrect permissions (%v) and cannot be fixed: %w",
				info.Mode().Perm(), err)
		}
	}

	// Open history writer
	historyPath := filepath.Join(cleanPath, "history.jsonl")
	writer, err := NewHistoryWriter(fs, historyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create history writer for session %q: %w", sessionID, err)
	}

	return &HistoryPersistence{
		fs:         fs,
		sessionDir: cleanPath,
		sessionID:  sessionID,
		writer:     writer,
		closed:     false,
	}, nil
}

// RecordSubmission appends a Submission to the history file.
// Returns ErrClosed if the persistence instance has been closed.
func (hp *HistoryPersistence) RecordSubmission(submission *protocol.Submission) error {
	hp.mu.RLock()
	defer hp.mu.RUnlock()

	if hp.closed {
		return fmt.Errorf("%w: cannot record submission", ErrClosed)
	}

	return hp.writer.Append(submission)
}

// RecordEvent appends an Event to the history file.
// Returns ErrClosed if the persistence instance has been closed.
func (hp *HistoryPersistence) RecordEvent(event *protocol.Event) error {
	hp.mu.RLock()
	defer hp.mu.RUnlock()

	if hp.closed {
		return fmt.Errorf("%w: cannot record event", ErrClosed)
	}

	return hp.writer.Append(event)
}

// LoadHistory reads all Submissions and Events from the history file.
// Returns empty slices (never nil) if the history file doesn't exist, allowing
// callers to distinguish between "no history yet" and "error reading history".
//
// Errors returned:
//   - File system errors (permissions, I/O errors)
//   - JSON unmarshaling errors if history file is corrupted
//   - Lock acquisition errors
func (hp *HistoryPersistence) LoadHistory() ([]*protocol.Submission, []*protocol.Event, error) {
	hp.mu.RLock()
	defer hp.mu.RUnlock()

	reader, err := NewHistoryReader(hp.fs, hp.HistoryPath())
	if err != nil {
		// If file doesn't exist, return empty history
		if os.IsNotExist(err) || !fileExists(hp.fs, hp.HistoryPath()) {
			return []*protocol.Submission{}, []*protocol.Event{}, nil
		}
		return nil, nil, fmt.Errorf("failed to open history file: %w", err)
	}
	defer reader.Close()

	return reader.ReadAll()
}

// CreateRollout creates a timestamped snapshot of the current history file.
// The rollout file is named {basename}.{nanosecond-timestamp}, for example:
//   history.jsonl.1698765432000000000
//
// Rollout files are useful for:
//   - Creating restore points before potentially destructive operations
//   - Archiving historical conversation states
//   - Debugging by examining past states
//
// The file is created with the same security permissions (0600) as the original
// history file. Any buffered writes are flushed before creating the rollout.
//
// Returns the full path to the created rollout file.
func (hp *HistoryPersistence) CreateRollout() (string, error) {
	hp.mu.RLock()
	defer hp.mu.RUnlock()

	if hp.closed {
		return "", fmt.Errorf("%w: cannot create rollout", ErrClosed)
	}

	// Flush any buffered data first
	if err := hp.writer.Flush(); err != nil {
		return "", fmt.Errorf("failed to flush history for session %s before rollout: %w", hp.sessionID, err)
	}

	rolloutPath, err := CreateRollout(hp.fs, hp.HistoryPath())
	if err != nil {
		return "", fmt.Errorf("failed to create rollout for session %s: %w", hp.sessionID, err)
	}
	return rolloutPath, nil
}

// ListRollouts returns a list of all rollout files for this session,
// sorted by timestamp (oldest first).
func (hp *HistoryPersistence) ListRollouts() ([]string, error) {
	return ListRollouts(hp.fs, hp.HistoryPath())
}

// CleanupOldRollouts keeps only the most recent N rollouts and deletes the rest.
func (hp *HistoryPersistence) CleanupOldRollouts(keepCount int) error {
	return CleanupOldRollouts(hp.fs, hp.HistoryPath(), keepCount)
}

// Flush flushes any buffered data to disk.
// Returns ErrClosed if the persistence instance has been closed.
func (hp *HistoryPersistence) Flush() error {
	hp.mu.RLock()
	defer hp.mu.RUnlock()

	if hp.closed {
		return fmt.Errorf("%w: cannot flush", ErrClosed)
	}

	return hp.writer.Flush()
}

// Close flushes any buffered data and closes the history file.
// Once closed, all recording operations will return ErrClosed.
// Multiple calls to Close() are safe and will only close once.
func (hp *HistoryPersistence) Close() error {
	hp.mu.Lock()
	defer hp.mu.Unlock()

	if hp.closed {
		return nil
	}

	hp.closed = true
	return hp.writer.Close()
}

// SessionID returns the session ID.
func (hp *HistoryPersistence) SessionID() string {
	return hp.sessionID
}

// SessionDir returns the session directory path.
func (hp *HistoryPersistence) SessionDir() string {
	return hp.sessionDir
}

// HistoryPath returns the full path to the history file.
func (hp *HistoryPersistence) HistoryPath() string {
	return hp.writer.Path()
}

// GetSessionDir returns the session directory path for a given session ID.
// This function performs basic validation to prevent path traversal attacks.
//
// Returns error if:
//   - sessionID is empty
//   - sessionID contains path separators (/, \)
//   - sessionID contains ".." (path traversal)
//   - sessionsRoot is empty
func GetSessionDir(sessionsRoot, sessionID string) (string, error) {
	if sessionsRoot == "" {
		return "", fmt.Errorf("%w: sessions root cannot be empty", ErrEmptyPath)
	}

	if sessionID == "" {
		return "", fmt.Errorf("%w: session ID cannot be empty", ErrEmptyPath)
	}

	// Validate session ID doesn't contain path separators
	if strings.ContainsAny(sessionID, `/\`) {
		return "", fmt.Errorf("%w: session ID cannot contain path separators: %q", ErrInvalidPath, sessionID)
	}

	// Validate against path traversal
	if strings.Contains(sessionID, "..") {
		return "", fmt.Errorf("%w: session ID cannot contain '..'", ErrPathTraversal)
	}

	sessionDir := filepath.Join(sessionsRoot, sessionID)

	// Verify the constructed path is within the sessions root
	if err := EnsureWithinRoot(sessionsRoot, sessionDir); err != nil {
		return "", fmt.Errorf("session directory validation failed: %w", err)
	}

	return sessionDir, nil
}

// GetSessionHistoryPath returns the full path to the history file for a given session ID.
// This function performs validation to prevent path traversal attacks.
//
// Returns error if:
//   - sessionID is invalid (see GetSessionDir)
//   - sessionsRoot is empty
func GetSessionHistoryPath(sessionsRoot, sessionID string) (string, error) {
	sessionDir, err := GetSessionDir(sessionsRoot, sessionID)
	if err != nil {
		return "", err
	}
	return filepath.Join(sessionDir, "history.jsonl"), nil
}

// fileExists checks if a file exists.
func fileExists(fs afero.Fs, path string) bool {
	exists, err := afero.Exists(fs, path)
	return err == nil && exists
}
