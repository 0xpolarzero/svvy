package persistence

import (
	"testing"

	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/evmts/codex/codex-go/test"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHistoryPersistence(t *testing.T) {
	fs := test.NewMemFS(t)

	hp, err := NewHistoryPersistence(fs, "/sessions/test-session")
	require.NoError(t, err)
	assert.NotNil(t, hp)
	defer hp.Close()

	// Verify session directory was created
	exists, err := afero.DirExists(fs, "/sessions/test-session")
	require.NoError(t, err)
	assert.True(t, exists)

	// Verify history file path
	assert.Equal(t, "/sessions/test-session/history.jsonl", hp.HistoryPath())
}

func TestHistoryPersistenceRecordSubmission(t *testing.T) {
	fs := test.NewMemFS(t)
	hp, err := NewHistoryPersistence(fs, "/sessions/test")
	require.NoError(t, err)
	defer hp.Close()

	submission := &protocol.Submission{
		ID: "sub-1",
		Op: &protocol.OpUserTurn{
			Items: []protocol.UserInput{
				{Type: "text", Text: stringPtr("hello")},
			},
			Cwd:            "/test",
			ApprovalPolicy: "auto",
			SandboxPolicy: protocol.SandboxPolicy{
				Mode: "unrestricted",
			},
			Model:   "claude-3-5-sonnet-20241022",
			Summary: "auto",
		},
	}

	err = hp.RecordSubmission(submission)
	require.NoError(t, err)

	// Verify written to file
	data, err := afero.ReadFile(fs, hp.HistoryPath())
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"sub-1"`)
}

func TestHistoryPersistenceRecordEvent(t *testing.T) {
	fs := test.NewMemFS(t)
	hp, err := NewHistoryPersistence(fs, "/sessions/test")
	require.NoError(t, err)
	defer hp.Close()

	event := &protocol.Event{
		ID:  "evt-1",
		Msg: &protocol.EventAgentMessage{Message: "Hello"},
	}

	err = hp.RecordEvent(event)
	require.NoError(t, err)

	// Verify written to file
	data, err := afero.ReadFile(fs, hp.HistoryPath())
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"evt-1"`)
	assert.Contains(t, string(data), "Hello")
}

func TestHistoryPersistenceLoadHistory(t *testing.T) {
	fs := test.NewMemFS(t)
	hp, err := NewHistoryPersistence(fs, "/sessions/test")
	require.NoError(t, err)

	// Record some items
	items := []interface{}{
		&protocol.Submission{ID: "1", Op: &protocol.OpInterrupt{}},
		&protocol.Event{ID: "2", Msg: &protocol.EventError{Message: "e1"}},
		&protocol.Submission{ID: "3", Op: &protocol.OpShutdown{}},
		&protocol.Event{ID: "4", Msg: &protocol.EventTaskStarted{}},
	}

	for _, item := range items {
		switch v := item.(type) {
		case *protocol.Submission:
			err := hp.RecordSubmission(v)
			require.NoError(t, err)
		case *protocol.Event:
			err := hp.RecordEvent(v)
			require.NoError(t, err)
		}
	}

	hp.Close()

	// Load in new instance
	hp2, err := NewHistoryPersistence(fs, "/sessions/test")
	require.NoError(t, err)
	defer hp2.Close()

	submissions, events, err := hp2.LoadHistory()
	require.NoError(t, err)

	assert.Len(t, submissions, 2)
	assert.Len(t, events, 2)

	assert.Equal(t, "1", submissions[0].ID)
	assert.Equal(t, "3", submissions[1].ID)
	assert.Equal(t, "2", events[0].ID)
	assert.Equal(t, "4", events[1].ID)
}

func TestHistoryPersistenceLoadHistoryEmpty(t *testing.T) {
	fs := test.NewMemFS(t)
	hp, err := NewHistoryPersistence(fs, "/sessions/test")
	require.NoError(t, err)
	defer hp.Close()

	submissions, events, err := hp.LoadHistory()
	require.NoError(t, err)

	assert.Empty(t, submissions)
	assert.Empty(t, events)
}

func TestHistoryPersistenceCreateRollout(t *testing.T) {
	fs := test.NewMemFS(t)
	hp, err := NewHistoryPersistence(fs, "/sessions/test")
	require.NoError(t, err)
	defer hp.Close()

	// Record some data
	err = hp.RecordSubmission(&protocol.Submission{ID: "1", Op: &protocol.OpInterrupt{}})
	require.NoError(t, err)

	// Create rollout
	rolloutPath, err := hp.CreateRollout()
	require.NoError(t, err)
	assert.NotEmpty(t, rolloutPath)

	// Verify rollout exists
	exists, err := afero.Exists(fs, rolloutPath)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestHistoryPersistenceListRollouts(t *testing.T) {
	fs := test.NewMemFS(t)
	hp, err := NewHistoryPersistence(fs, "/sessions/test")
	require.NoError(t, err)
	defer hp.Close()

	// Record some data
	err = hp.RecordSubmission(&protocol.Submission{ID: "1", Op: &protocol.OpInterrupt{}})
	require.NoError(t, err)

	// Create rollouts
	_, err = hp.CreateRollout()
	require.NoError(t, err)
	_, err = hp.CreateRollout()
	require.NoError(t, err)

	// List rollouts
	rollouts, err := hp.ListRollouts()
	require.NoError(t, err)
	assert.Len(t, rollouts, 2)
}

func TestHistoryPersistenceCleanupOldRollouts(t *testing.T) {
	fs := test.NewMemFS(t)
	hp, err := NewHistoryPersistence(fs, "/sessions/test")
	require.NoError(t, err)
	defer hp.Close()

	// Record data and create rollouts
	err = hp.RecordSubmission(&protocol.Submission{ID: "1", Op: &protocol.OpInterrupt{}})
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		_, err = hp.CreateRollout()
		require.NoError(t, err)
	}

	// Cleanup, keep 2
	err = hp.CleanupOldRollouts(2)
	require.NoError(t, err)

	// Verify only 2 remain
	rollouts, err := hp.ListRollouts()
	require.NoError(t, err)
	assert.Len(t, rollouts, 2)
}

func TestHistoryPersistenceSessionID(t *testing.T) {
	fs := test.NewMemFS(t)
	hp, err := NewHistoryPersistence(fs, "/sessions/my-session-123")
	require.NoError(t, err)
	defer hp.Close()

	assert.Equal(t, "my-session-123", hp.SessionID())
}

func TestHistoryPersistenceSessionDir(t *testing.T) {
	fs := test.NewMemFS(t)
	hp, err := NewHistoryPersistence(fs, "/sessions/test-session")
	require.NoError(t, err)
	defer hp.Close()

	assert.Equal(t, "/sessions/test-session", hp.SessionDir())
}

func TestHistoryPersistenceClose(t *testing.T) {
	fs := test.NewMemFS(t)
	hp, err := NewHistoryPersistence(fs, "/sessions/test")
	require.NoError(t, err)

	// Record data
	err = hp.RecordSubmission(&protocol.Submission{ID: "1", Op: &protocol.OpInterrupt{}})
	require.NoError(t, err)

	// Close
	err = hp.Close()
	require.NoError(t, err)

	// Data should be persisted
	data, err := afero.ReadFile(fs, hp.HistoryPath())
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"1"`)

	// Recording after close should error
	err = hp.RecordSubmission(&protocol.Submission{ID: "2", Op: &protocol.OpInterrupt{}})
	assert.Error(t, err)
}

func TestHistoryPersistenceFlush(t *testing.T) {
	fs := test.NewMemFS(t)
	hp, err := NewHistoryPersistence(fs, "/sessions/test")
	require.NoError(t, err)
	defer hp.Close()

	// Record data
	err = hp.RecordSubmission(&protocol.Submission{ID: "1", Op: &protocol.OpInterrupt{}})
	require.NoError(t, err)

	// Flush
	err = hp.Flush()
	require.NoError(t, err)

	// Data should be persisted
	data, err := afero.ReadFile(fs, hp.HistoryPath())
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"1"`)
}

func TestHistoryPersistenceConcurrentWrites(t *testing.T) {
	fs := test.NewMemFS(t)
	hp, err := NewHistoryPersistence(fs, "/sessions/test")
	require.NoError(t, err)
	defer hp.Close()

	// Concurrent writes
	done := make(chan bool, 20)

	for i := 0; i < 10; i++ {
		go func(id int) {
			sub := &protocol.Submission{
				ID: string(rune('a' + id)),
				Op: &protocol.OpInterrupt{},
			}
			err := hp.RecordSubmission(sub)
			assert.NoError(t, err)
			done <- true
		}(i)

		go func(id int) {
			evt := &protocol.Event{
				ID:  string(rune('A' + id)),
				Msg: &protocol.EventError{Message: "test"},
			}
			err := hp.RecordEvent(evt)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < 20; i++ {
		<-done
	}

	// Load and verify
	submissions, events, err := hp.LoadHistory()
	require.NoError(t, err)

	assert.Len(t, submissions, 10)
	assert.Len(t, events, 10)
}

func TestHistoryPersistenceResumeAfterRestart(t *testing.T) {
	fs := test.NewMemFS(t)

	// First session
	hp1, err := NewHistoryPersistence(fs, "/sessions/test")
	require.NoError(t, err)

	err = hp1.RecordSubmission(&protocol.Submission{ID: "1", Op: &protocol.OpInterrupt{}})
	require.NoError(t, err)
	err = hp1.RecordEvent(&protocol.Event{ID: "2", Msg: &protocol.EventError{Message: "e1"}})
	require.NoError(t, err)
	err = hp1.Close()
	require.NoError(t, err)

	// Second session (simulating restart)
	hp2, err := NewHistoryPersistence(fs, "/sessions/test")
	require.NoError(t, err)
	defer hp2.Close()

	// Continue recording
	err = hp2.RecordSubmission(&protocol.Submission{ID: "3", Op: &protocol.OpShutdown{}})
	require.NoError(t, err)

	// Load all
	submissions, events, err := hp2.LoadHistory()
	require.NoError(t, err)

	assert.Len(t, submissions, 2)
	assert.Len(t, events, 1)
	assert.Equal(t, "1", submissions[0].ID)
	assert.Equal(t, "3", submissions[1].ID)
	assert.Equal(t, "2", events[0].ID)
}

func TestGetSessionHistoryPath(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		wantPath  string
	}{
		{
			name:      "simple session id",
			sessionID: "test-123",
			wantPath:  "/sessions/test-123/history.jsonl",
		},
		{
			name:      "uuid session id",
			sessionID: "550e8400-e29b-41d4-a716-446655440000",
			wantPath:  "/sessions/550e8400-e29b-41d4-a716-446655440000/history.jsonl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := GetSessionHistoryPath("/sessions", tt.sessionID)
			require.NoError(t, err)
			assert.Equal(t, tt.wantPath, path)
		})
	}
}

func TestGetSessionDir(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		wantDir   string
	}{
		{
			name:      "simple session id",
			sessionID: "test-123",
			wantDir:   "/sessions/test-123",
		},
		{
			name:      "complex session id",
			sessionID: "my-session-2024-01-01",
			wantDir:   "/sessions/my-session-2024-01-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := GetSessionDir("/sessions", tt.sessionID)
			require.NoError(t, err)
			assert.Equal(t, tt.wantDir, dir)
		})
	}
}

func TestHistoryPersistenceMetadata(t *testing.T) {
	fs := test.NewMemFS(t)
	hp, err := NewHistoryPersistence(fs, "/sessions/test")
	require.NoError(t, err)
	defer hp.Close()

	// Verify we can get metadata
	assert.Equal(t, "test", hp.SessionID())
	assert.Equal(t, "/sessions/test", hp.SessionDir())
	assert.Equal(t, "/sessions/test/history.jsonl", hp.HistoryPath())
}

func TestHistoryPersistenceNestedSessionDir(t *testing.T) {
	fs := test.NewMemFS(t)

	// Use nested path
	hp, err := NewHistoryPersistence(fs, "/a/b/c/sessions/my-session")
	require.NoError(t, err)
	defer hp.Close()

	// Verify directory created
	exists, err := afero.DirExists(fs, "/a/b/c/sessions/my-session")
	require.NoError(t, err)
	assert.True(t, exists)

	// Record and verify
	err = hp.RecordSubmission(&protocol.Submission{ID: "1", Op: &protocol.OpInterrupt{}})
	require.NoError(t, err)

	data, err := afero.ReadFile(fs, "/a/b/c/sessions/my-session/history.jsonl")
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"1"`)
}
