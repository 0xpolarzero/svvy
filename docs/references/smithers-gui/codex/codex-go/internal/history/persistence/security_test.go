package persistence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/evmts/codex/codex-go/test"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateSafePath tests the ValidateSafePath function
func TestValidateSafePath(t *testing.T) {
	tests := []struct {
		name            string
		path            string
		requireAbsolute bool
		wantErr         bool
		errType         error
	}{
		{
			name:            "valid absolute path",
			path:            "/sessions/test-session",
			requireAbsolute: true,
			wantErr:         false,
		},
		{
			name:            "valid relative path when not required absolute",
			path:            "sessions/test-session",
			requireAbsolute: false,
			wantErr:         false,
		},
		{
			name:            "relative path when absolute required",
			path:            "sessions/test-session",
			requireAbsolute: true,
			wantErr:         true,
			errType:         ErrInvalidPath,
		},
		{
			name:            "empty path",
			path:            "",
			requireAbsolute: false,
			wantErr:         true,
			errType:         ErrEmptyPath,
		},
		{
			name:            "path traversal attempt with ..",
			path:            "/sessions/../etc/passwd",
			requireAbsolute: true,
			wantErr:         true,
			errType:         ErrPathTraversal,
		},
		{
			name:            "path traversal in middle",
			path:            "/sessions/test/../admin",
			requireAbsolute: true,
			wantErr:         true,
			errType:         ErrPathTraversal,
		},
		{
			name:            "path with spaces",
			path:            "/sessions/test session",
			requireAbsolute: true,
			wantErr:         false,
		},
		{
			name:            "path with hyphens and underscores",
			path:            "/sessions/test-session_123",
			requireAbsolute: true,
			wantErr:         false,
		},
		{
			name:            "path with dots in filename",
			path:            "/sessions/session.test",
			requireAbsolute: true,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSafePath(tt.path, tt.requireAbsolute)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestEnsureWithinRoot tests the EnsureWithinRoot function
func TestEnsureWithinRoot(t *testing.T) {
	tests := []struct {
		name       string
		rootPath   string
		targetPath string
		wantErr    bool
		errType    error
	}{
		{
			name:       "target within root",
			rootPath:   "/sessions",
			targetPath: "/sessions/test-session",
			wantErr:    false,
		},
		{
			name:       "target deep within root",
			rootPath:   "/sessions",
			targetPath: "/sessions/2024/01/test-session",
			wantErr:    false,
		},
		{
			name:       "target outside root",
			rootPath:   "/sessions",
			targetPath: "/etc/passwd",
			wantErr:    true,
			errType:    ErrPathTraversal,
		},
		{
			name:       "target traverses outside root",
			rootPath:   "/sessions",
			targetPath: "/sessions/../etc/passwd",
			wantErr:    true,
			errType:    ErrPathTraversal,
		},
		{
			name:       "target equals root",
			rootPath:   "/sessions",
			targetPath: "/sessions",
			wantErr:    false,
		},
		{
			name:       "relative root path",
			rootPath:   "sessions",
			targetPath: "/sessions/test",
			wantErr:    true,
			errType:    ErrInvalidPath,
		},
		{
			name:       "relative target path",
			rootPath:   "/sessions",
			targetPath: "test-session",
			wantErr:    true,
			errType:    ErrInvalidPath,
		},
		{
			name:       "similar prefix but different directory",
			rootPath:   "/sessions",
			targetPath: "/sessions-admin/test",
			wantErr:    true,
			errType:    ErrPathTraversal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnsureWithinRoot(tt.rootPath, tt.targetPath)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateSessionDir tests the validateSessionDir function
func TestValidateSessionDir(t *testing.T) {
	fs := test.NewMemFS(t)

	// Create a test directory
	err := fs.MkdirAll("/sessions/existing-session", 0700)
	require.NoError(t, err)

	// Create a file (not a directory)
	err = afero.WriteFile(fs, "/sessions/file-not-dir", []byte("test"), 0600)
	require.NoError(t, err)

	tests := []struct {
		name          string
		sessionDir    string
		wantErr       bool
		errType       error
		wantSessionID string
	}{
		{
			name:          "valid new session directory",
			sessionDir:    "/sessions/test-session",
			wantErr:       false,
			wantSessionID: "test-session",
		},
		{
			name:          "valid existing session directory",
			sessionDir:    "/sessions/existing-session",
			wantErr:       false,
			wantSessionID: "existing-session",
		},
		{
			name:       "empty path",
			sessionDir: "",
			wantErr:    true,
			errType:    ErrEmptyPath,
		},
		{
			name:       "root directory",
			sessionDir: "/",
			wantErr:    true,
			errType:    ErrInvalidPath,
		},
		{
			name:       "current directory",
			sessionDir: ".",
			wantErr:    true,
			errType:    ErrInvalidPath,
		},
		{
			name:       "path traversal attempt",
			sessionDir: "/sessions/../etc/passwd",
			wantErr:    true,
			errType:    ErrPathTraversal,
		},
		{
			name:       "relative path",
			sessionDir: "sessions/test",
			wantErr:    true,
			errType:    ErrInvalidPath,
		},
		{
			name:       "path with trailing slash",
			sessionDir: "/sessions/test-session/",
			wantErr:    false,
			wantSessionID: "test-session",
		},
		{
			name:       "nested path",
			sessionDir: "/sessions/2024/test-session",
			wantErr:    false,
			wantSessionID: "test-session",
		},
		{
			name:       "path exists but is a file",
			sessionDir: "/sessions/file-not-dir",
			wantErr:    true,
			errType:    ErrInvalidPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanPath, sessionID, err := validateSessionDir(fs, tt.sessionDir)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, cleanPath)
				assert.Equal(t, tt.wantSessionID, sessionID)
			}
		})
	}
}

// TestNewHistoryPersistence_PathTraversalAttacks tests security against path traversal
func TestNewHistoryPersistence_PathTraversalAttacks(t *testing.T) {
	fs := test.NewMemFS(t)

	attacks := []struct {
		name        string
		sessionDir  string
		description string
	}{
		{
			name:        "parent directory traversal",
			sessionDir:  "/sessions/../etc/passwd",
			description: "Attempt to access parent directory",
		},
		{
			name:        "multiple parent traversal",
			sessionDir:  "/sessions/../../etc/passwd",
			description: "Multiple parent directory traversal",
		},
		{
			name:        "encoded parent traversal",
			sessionDir:  "/sessions/%2e%2e/etc/passwd",
			description: "URL-encoded parent directory",
		},
		{
			name:        "backslash traversal",
			sessionDir:  "/sessions\\..\\etc\\passwd",
			description: "Backslash-based traversal",
		},
		{
			name:        "empty string",
			sessionDir:  "",
			description: "Empty path",
		},
		{
			name:        "root directory",
			sessionDir:  "/",
			description: "Root directory",
		},
		{
			name:        "relative path",
			sessionDir:  "sessions/test",
			description: "Relative path instead of absolute",
		},
	}

	for _, attack := range attacks {
		t.Run(attack.name, func(t *testing.T) {
			hp, err := NewHistoryPersistence(fs, attack.sessionDir)
			assert.Error(t, err, "Expected error for: %s", attack.description)
			assert.Nil(t, hp, "Expected nil persistence for: %s", attack.description)
		})
	}
}

// TestNewHistoryPersistence_SymlinkAttack tests protection against symlink attacks
func TestNewHistoryPersistence_SymlinkAttack(t *testing.T) {
	// Note: This test may not work on all filesystems (especially in-memory)
	// but demonstrates the intended behavior
	fs := afero.NewOsFs()
	tempDir := t.TempDir()

	// Create a directory outside the sessions root
	outsideDir := filepath.Join(tempDir, "outside")
	err := fs.MkdirAll(outsideDir, 0700)
	require.NoError(t, err)

	// Create a sessions directory
	sessionsDir := filepath.Join(tempDir, "sessions")
	err = fs.MkdirAll(sessionsDir, 0700)
	require.NoError(t, err)

	// Create a symlink inside sessions that points outside
	symlinkPath := filepath.Join(sessionsDir, "symlink-attack")
	err = os.Symlink(outsideDir, symlinkPath)
	if err != nil {
		t.Skip("Filesystem doesn't support symlinks")
	}

	// Attempt to create persistence with the symlink
	hp, err := NewHistoryPersistence(fs, symlinkPath)
	assert.Error(t, err, "Expected error for symlink")
	assert.ErrorIs(t, err, ErrInvalidPath)
	assert.Nil(t, hp)
}

// TestGetSessionDir_Security tests GetSessionDir with malicious inputs
func TestGetSessionDir_Security(t *testing.T) {
	tests := []struct {
		name         string
		sessionsRoot string
		sessionID    string
		wantErr      bool
		errType      error
	}{
		{
			name:         "valid session",
			sessionsRoot: "/sessions",
			sessionID:    "test-session",
			wantErr:      false,
		},
		{
			name:         "empty sessions root",
			sessionsRoot: "",
			sessionID:    "test",
			wantErr:      true,
			errType:      ErrEmptyPath,
		},
		{
			name:         "empty session ID",
			sessionsRoot: "/sessions",
			sessionID:    "",
			wantErr:      true,
			errType:      ErrEmptyPath,
		},
		{
			name:         "session ID with forward slash",
			sessionsRoot: "/sessions",
			sessionID:    "../etc/passwd",
			wantErr:      true,
			errType:      ErrInvalidPath,
		},
		{
			name:         "session ID with backslash",
			sessionsRoot: "/sessions",
			sessionID:    "..\\etc\\passwd",
			wantErr:      true,
			errType:      ErrInvalidPath,
		},
		{
			name:         "session ID with parent directory",
			sessionsRoot: "/sessions",
			sessionID:    "..",
			wantErr:      true,
			errType:      ErrPathTraversal,
		},
		{
			name:         "session ID with embedded slash",
			sessionsRoot: "/sessions",
			sessionID:    "test/admin",
			wantErr:      true,
			errType:      ErrInvalidPath,
		},
		{
			name:         "valid session with hyphens and underscores",
			sessionsRoot: "/sessions",
			sessionID:    "test-session_123",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := GetSessionDir(tt.sessionsRoot, tt.sessionID)
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, path)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, path)
				assert.Contains(t, path, tt.sessionID)
			}
		})
	}
}

// TestGetSessionHistoryPath_Security tests GetSessionHistoryPath with malicious inputs
func TestGetSessionHistoryPath_Security(t *testing.T) {
	tests := []struct {
		name         string
		sessionsRoot string
		sessionID    string
		wantErr      bool
		errType      error
	}{
		{
			name:         "valid session",
			sessionsRoot: "/sessions",
			sessionID:    "test-session",
			wantErr:      false,
		},
		{
			name:         "path traversal in session ID",
			sessionsRoot: "/sessions",
			sessionID:    "../etc/passwd",
			wantErr:      true,
			errType:      ErrInvalidPath,
		},
		{
			name:         "empty inputs",
			sessionsRoot: "",
			sessionID:    "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := GetSessionHistoryPath(tt.sessionsRoot, tt.sessionID)
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, path)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, path)
				assert.Contains(t, path, "history.jsonl")
			}
		})
	}
}

// TestHistoryPersistence_ClosedOperations tests that operations fail after Close
func TestHistoryPersistence_ClosedOperations(t *testing.T) {
	fs := test.NewMemFS(t)
	hp, err := NewHistoryPersistence(fs, "/sessions/test")
	require.NoError(t, err)

	// Close the persistence
	err = hp.Close()
	require.NoError(t, err)

	// Try operations on closed instance
	t.Run("RecordSubmission after close", func(t *testing.T) {
		err := hp.RecordSubmission(nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrClosed)
	})

	t.Run("RecordEvent after close", func(t *testing.T) {
		err := hp.RecordEvent(nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrClosed)
	})

	t.Run("Flush after close", func(t *testing.T) {
		err := hp.Flush()
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrClosed)
	})

	t.Run("CreateRollout after close", func(t *testing.T) {
		_, err := hp.CreateRollout()
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrClosed)
	})

	t.Run("Multiple Close calls are safe", func(t *testing.T) {
		err := hp.Close()
		assert.NoError(t, err, "Multiple Close calls should not error")
	})
}

// TestNewHistoryPersistence_PermissionValidation tests permission checking
func TestNewHistoryPersistence_PermissionValidation(t *testing.T) {
	fs := test.NewMemFS(t)

	// Create a directory with wrong permissions
	err := fs.MkdirAll("/sessions/wrong-perms", 0755)
	require.NoError(t, err)

	// NewHistoryPersistence should fix the permissions
	hp, err := NewHistoryPersistence(fs, "/sessions/wrong-perms")
	require.NoError(t, err)
	defer hp.Close()

	// Verify permissions were corrected
	info, err := fs.Stat("/sessions/wrong-perms")
	require.NoError(t, err)
	assert.Equal(t, SensitiveDirMode, info.Mode().Perm())
}

// TestNewHistoryPersistence_EdgeCases tests edge cases in path handling
func TestNewHistoryPersistence_EdgeCases(t *testing.T) {
	fs := test.NewMemFS(t)

	tests := []struct {
		name       string
		sessionDir string
		wantErr    bool
		setup      func() error
	}{
		{
			name:       "path with trailing slash",
			sessionDir: "/sessions/test/",
			wantErr:    false,
		},
		{
			name:       "path with multiple trailing slashes",
			sessionDir: "/sessions/test///",
			wantErr:    false,
		},
		{
			name:       "path with spaces",
			sessionDir: "/sessions/test session",
			wantErr:    false,
		},
		{
			name:       "path with unicode characters",
			sessionDir: "/sessions/test-🔒",
			wantErr:    true, // Invalid characters
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				err := tt.setup()
				require.NoError(t, err)
			}

			hp, err := NewHistoryPersistence(fs, tt.sessionDir)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, hp)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, hp)
				hp.Close()
			}
		})
	}
}
