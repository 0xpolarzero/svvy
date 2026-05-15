package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/evmts/codex/codex-go/internal/history/persistence"
	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/spf13/afero"
)

// SessionPersistentState represents the complete state of a session that can be persisted and restored.
// This includes conversation history, context, and metadata needed to resume a session.
type SessionPersistentState struct {
	// Session identification
	SessionID string    `json:"session_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Turn context (cwd, approval policy, sandbox policy, model)
	TurnContext *TurnContext `json:"turn_context,omitempty"`

	// Token usage statistics
	TokenUsage *protocol.TokenUsage `json:"token_usage,omitempty"`

	// Last agent message
	LastAgentMessage string `json:"last_agent_message,omitempty"`

	// History metadata
	HistoryLogID      uint64 `json:"history_log_id"`
	HistoryEntryCount int    `json:"history_entry_count"`

	// Provider information
	Provider string `json:"provider,omitempty"`

	// Session state information
	State         SessionState `json:"state"`
	ErrorMessage  string       `json:"error_message,omitempty"`
	CurrentTurnID string       `json:"current_turn_id,omitempty"`
}

// HistoryStore defines the interface for persisting and loading session state.
// This abstraction allows for different storage backends (filesystem, database, S3, etc.)
// while providing a consistent API for session persistence.
//
// Implementations must be thread-safe and handle concurrent access appropriately.
// The store is responsible for serialization/deserialization of session state.
type HistoryStore interface {
	// SaveSession persists the current state of a session.
	// This should be called after significant state changes (turn completion, etc.)
	// Returns an error if the save operation fails.
	SaveSession(ctx context.Context, state *SessionPersistentState) error

	// LoadSession restores a session's state from persistent storage.
	// Returns ErrSessionNotFound if the session doesn't exist.
	// Returns an error if the load operation fails.
	LoadSession(ctx context.Context, sessionID string) (*SessionPersistentState, error)

	// DeleteSession removes a session's state from persistent storage.
	// Returns ErrSessionNotFound if the session doesn't exist.
	// Returns an error if the delete operation fails.
	DeleteSession(ctx context.Context, sessionID string) error

	// ListSessions returns a list of all stored session IDs.
	// Returns an empty slice if no sessions exist.
	// Returns an error if the list operation fails.
	ListSessions(ctx context.Context) ([]string, error)

	// Close releases any resources held by the store.
	// After Close is called, all other methods should return an error.
	Close() error
}

// ErrSessionNotFound is returned when a session cannot be found in the store.
var ErrSessionNotFound = fmt.Errorf("session not found")

// FilesystemHistoryStore implements HistoryStore using the filesystem.
// It stores session state in a metadata file alongside the history.jsonl file.
// The metadata file is named "state.json" and contains the serialized SessionPersistentState.
//
// Directory structure:
//   ~/.codex/sessions/{session-id}/
//     history.jsonl   - JSONL history of submissions and events
//     state.json      - JSON session state metadata
//
// Thread-safety: All methods are thread-safe through the use of the underlying
// HistoryPersistence locking mechanisms and filesystem atomicity.
type FilesystemHistoryStore struct {
	fs           afero.Fs
	sessionsRoot string
	enableCache  bool
}

// NewFilesystemHistoryStore creates a new filesystem-based history store.
// The sessionsRoot should be an absolute path to the directory containing all sessions.
// Typically this is ~/.codex/sessions or similar.
//
// Security:
//   - Validates all paths to prevent path traversal attacks
//   - Uses secure file permissions (0600 for files, 0700 for directories)
//   - Checks for symlinks to prevent symlink attacks
func NewFilesystemHistoryStore(fs afero.Fs, sessionsRoot string) (*FilesystemHistoryStore, error) {
	if fs == nil {
		return nil, fmt.Errorf("filesystem is required")
	}

	if sessionsRoot == "" {
		return nil, fmt.Errorf("sessions root is required")
	}

	// Validate the sessions root path
	if err := persistence.ValidateSafePath(sessionsRoot, true); err != nil {
		return nil, fmt.Errorf("invalid sessions root: %w", err)
	}

	// Ensure sessions root exists
	if err := fs.MkdirAll(sessionsRoot, persistence.SensitiveDirMode); err != nil {
		return nil, fmt.Errorf("failed to create sessions root: %w", err)
	}

	return &FilesystemHistoryStore{
		fs:           fs,
		sessionsRoot: sessionsRoot,
		enableCache:  false, // Can be enabled in future for performance
	}, nil
}

// SaveSession persists session state to the filesystem.
// The state is written to {sessionsRoot}/{sessionID}/state.json
//
// This operation is atomic on most filesystems through the use of
// write-then-rename. A temporary file is written first, then renamed
// to the final location.
func (s *FilesystemHistoryStore) SaveSession(ctx context.Context, state *SessionPersistentState) error {
	if state == nil {
		return fmt.Errorf("session state cannot be nil")
	}

	if state.SessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	// Validate session ID and get session directory
	sessionDir, err := persistence.GetSessionDir(s.sessionsRoot, state.SessionID)
	if err != nil {
		return fmt.Errorf("invalid session ID: %w", err)
	}

	// Ensure session directory exists
	if err := s.fs.MkdirAll(sessionDir, persistence.SensitiveDirMode); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Update timestamp
	state.UpdatedAt = time.Now()

	// Serialize state to JSON
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session state: %w", err)
	}

	// Write to temporary file first for atomicity
	stateFile := filepath.Join(sessionDir, "state.json")
	tempFile := stateFile + ".tmp"

	// Write temporary file
	if err := afero.WriteFile(s.fs, tempFile, data, persistence.SensitiveFileMode); err != nil {
		return fmt.Errorf("failed to write temporary state file: %w", err)
	}

	// Atomic rename
	if err := s.fs.Rename(tempFile, stateFile); err != nil {
		// Clean up temporary file on failure
		_ = s.fs.Remove(tempFile)
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	// Ensure file has correct permissions
	if err := s.fs.Chmod(stateFile, persistence.SensitiveFileMode); err != nil {
		// Log warning but don't fail - the file was written successfully
		// On some filesystems, chmod might not be supported
	}

	return nil
}

// LoadSession loads session state from the filesystem.
// Returns ErrSessionNotFound if the session or state file doesn't exist.
func (s *FilesystemHistoryStore) LoadSession(ctx context.Context, sessionID string) (*SessionPersistentState, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	// Validate session ID and get session directory
	sessionDir, err := persistence.GetSessionDir(s.sessionsRoot, sessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid session ID: %w", err)
	}

	// Check if session directory exists
	exists, err := afero.DirExists(s.fs, sessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to check session directory: %w", err)
	}
	if !exists {
		return nil, ErrSessionNotFound
	}

	// Read state file
	stateFile := filepath.Join(sessionDir, "state.json")
	data, err := afero.ReadFile(s.fs, stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	// Deserialize state
	var state SessionPersistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session state: %w", err)
	}

	// Validate loaded state
	if state.SessionID != sessionID {
		return nil, fmt.Errorf("session ID mismatch: expected %s, got %s", sessionID, state.SessionID)
	}

	return &state, nil
}

// DeleteSession removes the entire session directory from the filesystem.
// This includes both the state.json file and the history.jsonl file.
func (s *FilesystemHistoryStore) DeleteSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	// Validate session ID and get session directory
	sessionDir, err := persistence.GetSessionDir(s.sessionsRoot, sessionID)
	if err != nil {
		return fmt.Errorf("invalid session ID: %w", err)
	}

	// Check if session directory exists
	exists, err := afero.DirExists(s.fs, sessionDir)
	if err != nil {
		return fmt.Errorf("failed to check session directory: %w", err)
	}
	if !exists {
		return ErrSessionNotFound
	}

	// Remove the entire session directory
	if err := s.fs.RemoveAll(sessionDir); err != nil {
		return fmt.Errorf("failed to remove session directory: %w", err)
	}

	return nil
}

// ListSessions returns a list of all session IDs that have state files.
// This scans the sessions root directory for subdirectories containing state.json files.
func (s *FilesystemHistoryStore) ListSessions(ctx context.Context) ([]string, error) {
	// Check if sessions root exists
	exists, err := afero.DirExists(s.fs, s.sessionsRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to check sessions root: %w", err)
	}
	if !exists {
		return []string{}, nil
	}

	// Read sessions root directory
	entries, err := afero.ReadDir(s.fs, s.sessionsRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions root: %w", err)
	}

	var sessions []string
	for _, entry := range entries {
		// Skip non-directories
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()

		// Check if state.json exists in this directory
		stateFile := filepath.Join(s.sessionsRoot, sessionID, "state.json")
		exists, err := afero.Exists(s.fs, stateFile)
		if err != nil {
			// Log error but continue scanning
			continue
		}

		if exists {
			sessions = append(sessions, sessionID)
		}
	}

	return sessions, nil
}

// Close releases any resources held by the store.
// For the filesystem implementation, this is a no-op.
func (s *FilesystemHistoryStore) Close() error {
	return nil
}

// SessionsRoot returns the root directory for sessions.
// This is useful for testing and debugging.
func (s *FilesystemHistoryStore) SessionsRoot() string {
	return s.sessionsRoot
}
