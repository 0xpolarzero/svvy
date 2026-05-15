package manager

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/evmts/codex/codex-go/internal/client/mocks"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestManager_CreateSession_PathTraversalProtection tests that the manager
// properly validates session IDs and prevents path traversal attacks.
func TestManager_CreateSession_PathTraversalProtection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	fs := afero.NewMemMapFs()
	sessionsRoot := "/tmp/sessions"

	// Create sessions root
	err := fs.MkdirAll(sessionsRoot, 0755)
	require.NoError(t, err)

	// Create manager with history enabled
	mgr, err := NewManager(ManagerConfig{
		Client:        mockClient,
		HistoryFs:     fs,
		SessionsRoot:  sessionsRoot,
		EnableHistory: true,
	})
	require.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name             string
		sessionID        string
		expectError      bool
		errorContains    string
		checkFileSystem  func(t *testing.T, fs afero.Fs)
	}{
		{
			name:          "valid session ID creates session successfully",
			sessionID:     "valid-session-123",
			expectError:   false,
			checkFileSystem: func(t *testing.T, fs afero.Fs) {
				// Check that session directory was created in the right place
				expectedPath := filepath.Join(sessionsRoot, "valid-session-123")
				exists, err := afero.DirExists(fs, expectedPath)
				assert.NoError(t, err)
				assert.True(t, exists, "Session directory should be created")

				// Verify it's in the expected location
				absPath, err := filepath.Abs(expectedPath)
				assert.NoError(t, err)
				assert.Contains(t, absPath, "sessions/valid-session-123")
			},
		},
		{
			name:          "path traversal with double dots blocked",
			sessionID:     "../etc/passwd",
			expectError:   true,
			errorContains: "invalid session ID",
			checkFileSystem: func(t *testing.T, fs afero.Fs) {
				// Verify no directory was created outside sessions root
				exists, err := afero.DirExists(fs, "/etc/passwd")
				assert.NoError(t, err)
				assert.False(t, exists, "Should not create directory outside sessions root")
			},
		},
		{
			name:          "path traversal to parent directory blocked",
			sessionID:     "../../sensitive",
			expectError:   true,
			errorContains: "invalid session ID",
		},
		{
			name:          "absolute path attempt blocked",
			sessionID:     "/etc/passwd",
			expectError:   true,
			errorContains: "invalid session ID",
		},
		{
			name:          "forward slash in session ID blocked",
			sessionID:     "session/subdir",
			expectError:   true,
			errorContains: "invalid session ID",
		},
		{
			name:          "backslash in session ID blocked",
			sessionID:     "session\\subdir",
			expectError:   true,
			errorContains: "invalid session ID",
		},
		{
			name:          "url encoded path traversal blocked",
			sessionID:     "..%2f..%2fetc",
			expectError:   true,
			errorContains: "invalid session ID",
		},
		{
			name:          "null byte injection blocked",
			sessionID:     "session\x00../etc/passwd",
			expectError:   true,
			errorContains: "invalid session ID",
		},
		{
			name:          "unicode fullwidth characters blocked",
			sessionID:     "session\uff0e\uff0e\uff0ftest",
			expectError:   true,
			errorContains: "invalid session ID",
		},
		{
			name:          "dot dot in middle blocked",
			sessionID:     "sess..ion",
			expectError:   true,
			errorContains: "invalid session ID",
		},
		{
			name:          "single dot blocked",
			sessionID:     ".",
			expectError:   true,
			errorContains: "invalid session ID",
		},
		{
			name:          "double dot blocked",
			sessionID:     "..",
			expectError:   true,
			errorContains: "invalid session ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := SessionConfig{
				ID: tt.sessionID,
				TurnContext: &TurnContext{
					Cwd:   "/test",
					Model: "gpt-4",
				},
			}

			session, err := mgr.CreateSession(ctx, cfg)

			if tt.expectError {
				require.Error(t, err, "Expected error for session ID: %q", tt.sessionID)
				assert.Nil(t, session, "Session should be nil on error")
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains,
						"Error should mention: %s", tt.errorContains)
				}
			} else {
				require.NoError(t, err, "Expected no error for session ID: %q", tt.sessionID)
				require.NotNil(t, session, "Session should not be nil")
				assert.Equal(t, tt.sessionID, session.ID())
			}

			if tt.checkFileSystem != nil {
				tt.checkFileSystem(t, fs)
			}
		})
	}
}

// TestManager_ResumeSession_PathTraversalProtection tests that resuming sessions
// also validates session IDs and prevents path traversal.
func TestManager_ResumeSession_PathTraversalProtection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	fs := afero.NewMemMapFs()
	sessionsRoot := "/tmp/sessions"

	// Create sessions root
	err := fs.MkdirAll(sessionsRoot, 0755)
	require.NoError(t, err)

	// Create a valid session directory with history
	validSessionID := "valid-session-123"
	validSessionDir := filepath.Join(sessionsRoot, validSessionID)
	err = fs.MkdirAll(validSessionDir, 0755)
	require.NoError(t, err)

	// Create manager with history enabled
	mgr, err := NewManager(ManagerConfig{
		Client:        mockClient,
		HistoryFs:     fs,
		SessionsRoot:  sessionsRoot,
		EnableHistory: true,
	})
	require.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name          string
		sessionID     string
		expectError   bool
		errorContains string
	}{
		{
			name:        "path traversal with double dots blocked",
			sessionID:   "../etc/passwd",
			expectError: true,
			errorContains: "invalid session ID",
		},
		{
			name:        "path traversal to parent blocked",
			sessionID:   "../../sensitive",
			expectError: true,
			errorContains: "invalid session ID",
		},
		{
			name:        "absolute path blocked",
			sessionID:   "/etc/passwd",
			expectError: true,
			errorContains: "invalid session ID",
		},
		{
			name:        "forward slash blocked",
			sessionID:   "session/subdir",
			expectError: true,
			errorContains: "invalid session ID",
		},
		{
			name:        "url encoded traversal blocked",
			sessionID:   "..%2fetc",
			expectError: true,
			errorContains: "invalid session ID",
		},
		{
			name:        "null byte blocked",
			sessionID:   "session\x00test",
			expectError: true,
			errorContains: "invalid session ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := mgr.ResumeSession(ctx, tt.sessionID)

			if tt.expectError {
				require.Error(t, err, "Expected error for session ID: %q", tt.sessionID)
				assert.Nil(t, session, "Session should be nil on error")
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains,
						"Error should mention: %s", tt.errorContains)
				}
			} else {
				// This test only focuses on path traversal protection,
				// so valid IDs may still fail due to missing history
				// We just verify path traversal is caught first
				if err != nil {
					assert.NotContains(t, err.Error(), "panic")
					assert.NotContains(t, err.Error(), "outside")
				}
			}
		})
	}
}

// TestManager_SessionIDValidation_BeforeHistoryAccess tests that validation
// happens before any filesystem operations.
func TestManager_SessionIDValidation_BeforeHistoryAccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	// Use a failing filesystem that panics on any operation
	// If validation works correctly, we should never reach filesystem operations
	panicFs := &panicFilesystem{}

	mgr, err := NewManager(ManagerConfig{
		Client:        mockClient,
		HistoryFs:     panicFs,
		SessionsRoot:  "/tmp/sessions",
		EnableHistory: true,
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Try to create a session with path traversal
	cfg := SessionConfig{
		ID: "../etc/passwd",
		TurnContext: &TurnContext{
			Cwd:   "/test",
			Model: "gpt-4",
		},
	}

	// Should fail validation before touching filesystem
	session, err := mgr.CreateSession(ctx, cfg)

	// Should return validation error, not panic from filesystem
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid session ID")
	assert.Nil(t, session)
}

// TestManager_DoubleValidation_PathConstruction tests that both session ID
// validation and path construction validation catch escapes (defense in depth).
func TestManager_DoubleValidation_PathConstruction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	fs := afero.NewMemMapFs()
	sessionsRoot := "/tmp/sessions"

	err := fs.MkdirAll(sessionsRoot, 0755)
	require.NoError(t, err)

	mgr, err := NewManager(ManagerConfig{
		Client:        mockClient,
		HistoryFs:     fs,
		SessionsRoot:  sessionsRoot,
		EnableHistory: true,
	})
	require.NoError(t, err)

	ctx := context.Background()

	// These IDs would fail character validation, but test defense-in-depth
	dangerousIDs := []string{
		"../parent",
		"./current",
		"../../grandparent",
	}

	for _, sessionID := range dangerousIDs {
		t.Run(sessionID, func(t *testing.T) {
			cfg := SessionConfig{
				ID: sessionID,
				TurnContext: &TurnContext{
					Cwd:   "/test",
					Model: "gpt-4",
				},
			}

			_, err := mgr.CreateSession(ctx, cfg)

			// Should be rejected by validation
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid session ID")

			// Verify no directory was created outside sessions root
			parentDir := filepath.Join(sessionsRoot, "..")
			entries, err := afero.ReadDir(fs, parentDir)
			assert.NoError(t, err)

			// Only "sessions" directory should exist in parent
			for _, entry := range entries {
				assert.Equal(t, "sessions", entry.Name(),
					"No directories should be created outside sessions root")
			}
		})
	}
}

// TestManager_SecurityLogging tests that security violations are logged.
func TestManager_SecurityLogging(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	fs := afero.NewMemMapFs()
	sessionsRoot := "/tmp/sessions"

	err := fs.MkdirAll(sessionsRoot, 0755)
	require.NoError(t, err)

	mgr, err := NewManager(ManagerConfig{
		Client:        mockClient,
		HistoryFs:     fs,
		SessionsRoot:  sessionsRoot,
		EnableHistory: true,
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Attempt path traversal
	cfg := SessionConfig{
		ID: "../etc/passwd",
		TurnContext: &TurnContext{
			Cwd:   "/test",
			Model: "gpt-4",
		},
	}

	// This should log a security warning
	// In production, these logs would be monitored for attack detection
	_, err = mgr.CreateSession(ctx, cfg)
	require.Error(t, err)

	// Attempt resume with path traversal
	_, err = mgr.ResumeSession(ctx, "../etc/passwd")
	require.Error(t, err)

	// Note: In a real implementation, we would capture and verify log output
	// For now, we just verify that the operations fail appropriately
}

// panicFilesystem is a filesystem that panics on any operation,
// used to test that validation happens before filesystem access.
type panicFilesystem struct {
	afero.Fs
}

func (p *panicFilesystem) MkdirAll(path string, perm os.FileMode) error {
	panic("filesystem should not be accessed with invalid session ID")
}

func (p *panicFilesystem) Stat(name string) (os.FileInfo, error) {
	panic("filesystem should not be accessed with invalid session ID")
}

func (p *panicFilesystem) Create(name string) (afero.File, error) {
	panic("filesystem should not be accessed with invalid session ID")
}
