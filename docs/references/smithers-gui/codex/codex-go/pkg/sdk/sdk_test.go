package sdk

import (
	"testing"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
	"github.com/evmts/codex/codex-go/pkg/sdk/client"
	"github.com/evmts/codex/codex-go/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid options with client",
			opts: Options{
				Client: mustCreateTestClient(t),
			},
			wantErr: false,
		},
		{
			name:    "missing client",
			opts:    Options{},
			wantErr: true,
			errMsg:  "client is required",
		},
		{
			name: "with custom tools",
			opts: Options{
				Client: mustCreateTestClient(t),
				Tools:  []runtime.ToolRuntime{},
			},
			wantErr: false,
		},
		{
			name: "with history enabled",
			opts: Options{
				Client:        mustCreateTestClient(t),
				EnableHistory: true,
				HistoryPath:   test.TempDir(t) + "/history.jsonl",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			codex, err := New(tt.opts)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, codex)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, codex)
			}
		})
	}
}

func TestNewSession(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	tests := []struct {
		name    string
		opts    SessionOptions
		wantErr bool
		errMsg  string
	}{
		{
			name: "basic session",
			opts: SessionOptions{
				SystemPrompt: "You are a helpful assistant.",
			},
			wantErr: false,
		},
		{
			name: "streaming session",
			opts: SessionOptions{
				SystemPrompt: "You are a helpful assistant.",
				Streaming:    true,
			},
			wantErr: false,
		},
		{
			name: "with approval callback",
			opts: SessionOptions{
				SystemPrompt: "You are a helpful assistant.",
				OnToolApproval: func(toolName, operation string) bool {
					return true
				},
			},
			wantErr: false,
		},
		{
			name: "with custom working directory",
			opts: SessionOptions{
				SystemPrompt:     "You are a helpful assistant.",
				WorkingDirectory: "/tmp",
			},
			wantErr: false,
		},
		{
			name: "with approval policy",
			opts: SessionOptions{
				SystemPrompt:   "You are a helpful assistant.",
				ApprovalPolicy: "always",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := codex.NewSession(ctx, tt.opts)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, session)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, session)
				assert.NotEmpty(t, session.ID())
			}
		})
	}
}

func TestClose(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	// Create a session
	session, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
	})
	require.NoError(t, err)
	require.NotNil(t, session)

	// Close the SDK
	err = codex.Close()
	require.NoError(t, err)

	// Verify session is closed
	assert.True(t, session.IsClosed())
}

func TestGetSession(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	// Create a session
	session1, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
	})
	require.NoError(t, err)
	require.NotNil(t, session1)

	// Retrieve the session
	session2, err := codex.GetSession(session1.ID())
	require.NoError(t, err)
	assert.Equal(t, session1.ID(), session2.ID())

	// Try to get non-existent session
	_, err = codex.GetSession("nonexistent-id")
	require.Error(t, err)
}

func TestListSessions(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	// Initially no sessions
	sessions := codex.ListSessions()
	assert.Empty(t, sessions)

	// Create some sessions
	session1, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt: "Session 1",
	})
	require.NoError(t, err)

	session2, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt: "Session 2",
	})
	require.NoError(t, err)

	// List should return both
	sessions = codex.ListSessions()
	assert.Len(t, sessions, 2)
	assert.Contains(t, sessions, session1.ID())
	assert.Contains(t, sessions, session2.ID())
}

func TestCloseSession(t *testing.T) {
	codex := mustCreateTestSDK(t)
	ctx := test.LongContext(t)

	// Create a session
	session, err := codex.NewSession(ctx, SessionOptions{
		SystemPrompt: "You are a helpful assistant.",
	})
	require.NoError(t, err)
	sessionID := session.ID()

	// Close the specific session
	err = codex.CloseSession(sessionID)
	require.NoError(t, err)

	// Session should no longer be retrievable
	_, err = codex.GetSession(sessionID)
	require.Error(t, err)

	// Try to close again should error
	err = codex.CloseSession(sessionID)
	require.Error(t, err)
}

// Helper functions

func mustCreateTestClient(t *testing.T) *client.Client {
	t.Helper()

	c, err := client.New(client.Options{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "test-key",
		Model:   "gpt-4",
	})
	require.NoError(t, err)
	return c
}

func mustCreateTestSDK(t *testing.T) *SDK {
	t.Helper()

	codex, err := New(Options{
		Client: mustCreateTestClient(t),
	})
	require.NoError(t, err)
	return codex
}
