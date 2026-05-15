package sdk_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/evmts/codex/codex-go/pkg/sdk"
	"github.com/evmts/codex/codex-go/pkg/sdk/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHistoryPersistence tests that conversation history is persisted correctly.
func TestHistoryPersistence(t *testing.T) {
	// Create temporary directory for history
	tmpDir, err := os.MkdirTemp("", "codex-history-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create client with mock API key
	c, err := client.New(&client.Config{
		APIKey:  "test-key",
		BaseURL: "http://localhost:8080", // Mock endpoint
	})
	require.NoError(t, err)

	// Create SDK with history enabled
	s, err := sdk.New(sdk.Options{
		Client:        c,
		EnableHistory: true,
		HistoryPath:   tmpDir,
	})
	require.NoError(t, err)
	defer s.Close()

	ctx := context.Background()

	// Create a session
	session, err := s.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "You are a helpful assistant",
		Model:        "claude-3-5-sonnet-20241022",
	})
	require.NoError(t, err)

	sessionID := session.ID()
	assert.NotEmpty(t, sessionID)

	// Close the session to ensure history is written
	err = s.CloseSession(sessionID)
	require.NoError(t, err)

	// Check that history directory was created
	sessionDir := filepath.Join(tmpDir, sessionID)
	_, err = os.Stat(sessionDir)
	assert.NoError(t, err, "session directory should exist")

	// Check that history file exists
	historyFile := filepath.Join(sessionDir, "history.jsonl")
	_, err = os.Stat(historyFile)
	assert.NoError(t, err, "history file should exist")
}

// TestSessionResumption tests that sessions can be resumed from history.
func TestSessionResumption(t *testing.T) {
	// Create temporary directory for history
	tmpDir, err := os.MkdirTemp("", "codex-resume-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create client
	c, err := client.New(&client.Config{
		APIKey:  "test-key",
		BaseURL: "http://localhost:8080",
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Create first SDK instance with history
	sdk1, err := sdk.New(sdk.Options{
		Client:        c,
		EnableHistory: true,
		HistoryPath:   tmpDir,
	})
	require.NoError(t, err)

	// Create a session
	session1, err := sdk1.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt:     "You are a helpful assistant",
		Model:            "claude-3-5-sonnet-20241022",
		WorkingDirectory: "/tmp",
		ApprovalPolicy:   "auto",
	})
	require.NoError(t, err)

	sessionID := session1.ID()

	// Close the session
	err = sdk1.CloseSession(sessionID)
	require.NoError(t, err)

	// Close first SDK
	err = sdk1.Close()
	require.NoError(t, err)

	// Create second SDK instance (simulating restart)
	sdk2, err := sdk.New(sdk.Options{
		Client:        c,
		EnableHistory: true,
		HistoryPath:   tmpDir,
	})
	require.NoError(t, err)
	defer sdk2.Close()

	// Resume the session
	session2, err := sdk2.ResumeSession(ctx, sessionID)
	require.NoError(t, err)
	assert.Equal(t, sessionID, session2.ID())
	assert.False(t, session2.IsClosed())
}

// TestListPersistedSessions tests listing of persisted sessions.
func TestListPersistedSessions(t *testing.T) {
	// Create temporary directory for history
	tmpDir, err := os.MkdirTemp("", "codex-list-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create client
	c, err := client.New(&client.Config{
		APIKey:  "test-key",
		BaseURL: "http://localhost:8080",
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Create SDK with history
	s, err := sdk.New(sdk.Options{
		Client:        c,
		EnableHistory: true,
		HistoryPath:   tmpDir,
	})
	require.NoError(t, err)
	defer s.Close()

	// Create multiple sessions
	session1, err := s.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "Assistant 1",
	})
	require.NoError(t, err)

	session2, err := s.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "Assistant 2",
	})
	require.NoError(t, err)

	// Close sessions to persist history
	err = s.CloseSession(session1.ID())
	require.NoError(t, err)
	err = s.CloseSession(session2.ID())
	require.NoError(t, err)

	// List persisted sessions
	sessions, err := s.ListPersistedSessions()
	require.NoError(t, err)
	assert.Len(t, sessions, 2)
	assert.Contains(t, sessions, session1.ID())
	assert.Contains(t, sessions, session2.ID())
}

// TestDeleteSession tests deletion of persisted session history.
func TestDeleteSession(t *testing.T) {
	// Create temporary directory for history
	tmpDir, err := os.MkdirTemp("", "codex-delete-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create client
	c, err := client.New(&client.Config{
		APIKey:  "test-key",
		BaseURL: "http://localhost:8080",
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Create SDK with history
	s, err := sdk.New(sdk.Options{
		Client:        c,
		EnableHistory: true,
		HistoryPath:   tmpDir,
	})
	require.NoError(t, err)
	defer s.Close()

	// Create a session
	session, err := s.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "Test assistant",
	})
	require.NoError(t, err)

	sessionID := session.ID()

	// Close session to persist
	err = s.CloseSession(sessionID)
	require.NoError(t, err)

	// Verify session exists
	sessionDir := filepath.Join(tmpDir, sessionID)
	_, err = os.Stat(sessionDir)
	require.NoError(t, err)

	// Delete the session
	err = s.DeleteSession(sessionID)
	require.NoError(t, err)

	// Verify session was deleted
	_, err = os.Stat(sessionDir)
	assert.True(t, os.IsNotExist(err), "session directory should not exist after deletion")
}

// TestGetSessionMetadata tests retrieving session metadata.
func TestGetSessionMetadata(t *testing.T) {
	// Create temporary directory for history
	tmpDir, err := os.MkdirTemp("", "codex-metadata-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create client
	c, err := client.New(&client.Config{
		APIKey:  "test-key",
		BaseURL: "http://localhost:8080",
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Create SDK with history
	s, err := sdk.New(sdk.Options{
		Client:        c,
		EnableHistory: true,
		HistoryPath:   tmpDir,
	})
	require.NoError(t, err)
	defer s.Close()

	// Create a session
	session, err := s.NewSession(ctx, sdk.SessionOptions{
		SystemPrompt: "Test assistant",
	})
	require.NoError(t, err)

	sessionID := session.ID()

	// Close session to persist
	err = s.CloseSession(sessionID)
	require.NoError(t, err)

	// Get metadata
	metadata, err := s.GetSessionMetadata(sessionID)
	require.NoError(t, err)
	assert.Equal(t, sessionID, metadata.SessionID)
	assert.NotEmpty(t, metadata.Path)
	assert.NotEmpty(t, metadata.LastModified)
}

// TestHistoryWithoutPersistence tests that history operations fail when persistence is disabled.
func TestHistoryWithoutPersistence(t *testing.T) {
	// Create client
	c, err := client.New(&client.Config{
		APIKey:  "test-key",
		BaseURL: "http://localhost:8080",
	})
	require.NoError(t, err)

	// Create SDK WITHOUT history enabled
	s, err := sdk.New(sdk.Options{
		Client:        c,
		EnableHistory: false, // History disabled
	})
	require.NoError(t, err)
	defer s.Close()

	ctx := context.Background()

	// Try to resume a session - should fail
	_, err = s.ResumeSession(ctx, "test-session")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "history persistence is not enabled")

	// Try to list persisted sessions - should fail
	_, err = s.ListPersistedSessions()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "history persistence is not enabled")

	// Try to delete a session - should fail
	err = s.DeleteSession("test-session")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "history persistence is not enabled")

	// Try to get metadata - should fail
	_, err = s.GetSessionMetadata("test-session")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "history persistence is not enabled")
}

// TestMultipleSessionsHistory tests that multiple sessions can be persisted concurrently.
func TestMultipleSessionsHistory(t *testing.T) {
	// Create temporary directory for history
	tmpDir, err := os.MkdirTemp("", "codex-multiple-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create client
	c, err := client.New(&client.Config{
		APIKey:  "test-key",
		BaseURL: "http://localhost:8080",
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Create SDK with history
	s, err := sdk.New(sdk.Options{
		Client:        c,
		EnableHistory: true,
		HistoryPath:   tmpDir,
	})
	require.NoError(t, err)
	defer s.Close()

	// Create multiple sessions
	numSessions := 5
	sessionIDs := make([]string, numSessions)

	for i := 0; i < numSessions; i++ {
		session, err := s.NewSession(ctx, sdk.SessionOptions{
			SystemPrompt: fmt.Sprintf("Assistant %d", i),
		})
		require.NoError(t, err)
		sessionIDs[i] = session.ID()

		// Close immediately to persist
		err = s.CloseSession(session.ID())
		require.NoError(t, err)
	}

	// Verify all sessions are persisted
	sessions, err := s.ListPersistedSessions()
	require.NoError(t, err)
	assert.Len(t, sessions, numSessions)

	for _, id := range sessionIDs {
		assert.Contains(t, sessions, id)

		// Verify each session can be resumed
		resumed, err := s.ResumeSession(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, id, resumed.ID())
	}
}
