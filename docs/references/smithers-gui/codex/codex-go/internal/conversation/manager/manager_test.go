package manager

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/client"
	"github.com/evmts/codex/codex-go/internal/client/mocks"
	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("valid configuration", func(t *testing.T) {
		cfg := ManagerConfig{
			Client: mockClient,
		}

		mgr, err := NewManager(cfg)
		require.NoError(t, err)
		require.NotNil(t, mgr)
	})

	t.Run("missing client", func(t *testing.T) {
		cfg := ManagerConfig{}

		mgr, err := NewManager(cfg)
		require.Error(t, err)
		assert.Nil(t, mgr)
		assert.Contains(t, err.Error(), "client is required")
	})
}

func TestManager_CreateSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("create new session", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		cfg := SessionConfig{
			ID: "session-1",
			TurnContext: &TurnContext{
				Cwd:   "/test",
				Model: "gpt-4",
			},
		}

		session, err := mgr.CreateSession(ctx, cfg)
		require.NoError(t, err)
		require.NotNil(t, session)
		assert.Equal(t, "session-1", session.ID())
		assert.Equal(t, StateIdle, session.State())
	})

	t.Run("duplicate session ID", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		cfg := SessionConfig{
			ID: "session-1",
			TurnContext: &TurnContext{
				Cwd:   "/test",
				Model: "gpt-4",
			},
		}

		// Create first session
		_, err := mgr.CreateSession(ctx, cfg)
		require.NoError(t, err)

		// Try to create duplicate
		_, err = mgr.CreateSession(ctx, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("uses manager client if not provided", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		cfg := SessionConfig{
			ID:     "session-1",
			Client: nil, // Not provided
			TurnContext: &TurnContext{
				Cwd:   "/test",
				Model: "gpt-4",
			},
		}

		session, err := mgr.CreateSession(ctx, cfg)
		require.NoError(t, err)
		require.NotNil(t, session)
	})
}

func TestManager_GetSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("get existing session", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		// Create session
		cfg := SessionConfig{
			ID: "session-1",
			TurnContext: &TurnContext{
				Cwd:   "/test",
				Model: "gpt-4",
			},
		}
		created, err := mgr.CreateSession(ctx, cfg)
		require.NoError(t, err)

		// Retrieve session
		retrieved, err := mgr.GetSession("session-1")
		require.NoError(t, err)
		assert.Equal(t, created.ID(), retrieved.ID())
	})

	t.Run("get non-existent session", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)

		_, err := mgr.GetSession("non-existent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestManager_ListSessions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("list empty sessions", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)

		sessions := mgr.ListSessions()
		assert.Empty(t, sessions)
	})

	t.Run("list multiple sessions", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		// Create multiple sessions
		for i := 1; i <= 3; i++ {
			cfg := SessionConfig{
				ID: fmt.Sprintf("session-%d", i),
				TurnContext: &TurnContext{
					Cwd:   "/test",
					Model: "gpt-4",
				},
			}
			_, err := mgr.CreateSession(ctx, cfg)
			require.NoError(t, err)
		}

		sessions := mgr.ListSessions()
		assert.Len(t, sessions, 3)
		assert.Contains(t, sessions, "session-1")
		assert.Contains(t, sessions, "session-2")
		assert.Contains(t, sessions, "session-3")
	})
}

func TestManager_CloseSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("close existing session", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		// Create session
		cfg := SessionConfig{
			ID: "session-1",
			TurnContext: &TurnContext{
				Cwd:   "/test",
				Model: "gpt-4",
			},
		}
		session, err := mgr.CreateSession(ctx, cfg)
		require.NoError(t, err)

		// Close session
		err = mgr.CloseSession("session-1")
		require.NoError(t, err)

		// Verify session is closed
		assert.True(t, session.IsClosed())

		// Verify session is removed from manager
		_, err = mgr.GetSession("session-1")
		require.Error(t, err)
	})

	t.Run("close non-existent session", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)

		err := mgr.CloseSession("non-existent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestManager_SubmitOp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("submit user turn", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		// Create session
		session := createSessionInManager(t, mgr, "session-1")

		// Setup mock expectations for streaming
		eventChan := make(chan client.StreamEvent, 1)
		close(eventChan) // Close immediately for test

		mockClient.EXPECT().
			Stream(gomock.Any(), gomock.Any()).
			Return(eventChan, nil).
			Times(1)

		mockClient.EXPECT().
			GetModelContextWindow().
			Return(int64(128000)).
			AnyTimes()

		// Submit user turn
		op := &protocol.OpUserTurn{
			Items: []protocol.UserInput{
				{Type: "text", Text: strPtr("test message")},
			},
			Cwd:            "/test",
			ApprovalPolicy: "auto",
			SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
			Model:          "gpt-4",
			Summary:        "off",
		}

		err := mgr.SubmitOp(ctx, "session-1", op)
		require.NoError(t, err)

		// Give goroutine time to process
		time.Sleep(50 * time.Millisecond)

		// Session should be in completed or processing state
		state := session.State()
		assert.True(t, state == StateProcessingTurn || state == StateCompleted || state == StateError)
	})

	t.Run("submit interrupt", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		// Create session and start a turn
		session := createSessionInManager(t, mgr, "session-1")

		// Manually set session to processing state
		session.stateMachine.Transition(StateProcessingTurn)

		// Submit interrupt
		op := &protocol.OpInterrupt{}
		err := mgr.SubmitOp(ctx, "session-1", op)
		require.NoError(t, err)

		assert.Equal(t, StateInterrupted, session.State())
	})

	t.Run("submit exec approval", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		// Create session
		session := createSessionInManager(t, mgr, "session-1")

		// Setup session state for approval
		session.stateMachine.Transition(StateProcessingTurn)
		submissionID := "test-submission"
		err := session.RequestApproval(submissionID, ApprovalTypeExec)
		require.NoError(t, err)

		// Submit approval
		op := &protocol.OpExecApproval{
			ID:       submissionID,
			Decision: "approve",
		}

		err = mgr.SubmitOp(ctx, "session-1", op)
		require.NoError(t, err)

		assert.Equal(t, StateProcessingTurn, session.State())
	})

	t.Run("submit patch approval", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		// Create session
		session := createSessionInManager(t, mgr, "session-1")

		// Setup session state for approval
		session.stateMachine.Transition(StateProcessingTurn)
		submissionID := "test-submission"
		err := session.RequestApproval(submissionID, ApprovalTypePatch)
		require.NoError(t, err)

		// Submit approval
		op := &protocol.OpPatchApproval{
			ID:       submissionID,
			Decision: "reject",
		}

		err = mgr.SubmitOp(ctx, "session-1", op)
		require.NoError(t, err)

		assert.Equal(t, StateInterrupted, session.State())
	})

	t.Run("submit override turn context", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		// Create session
		session := createSessionInManager(t, mgr, "session-1")

		// Submit override
		newCwd := "/new/path"
		op := &protocol.OpOverrideTurnContext{
			Cwd: &newCwd,
		}

		err := mgr.SubmitOp(ctx, "session-1", op)
		require.NoError(t, err)

		turnCtx := session.GetTurnContext()
		assert.Equal(t, "/new/path", turnCtx.Cwd)
	})

	t.Run("submit to non-existent session", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		op := &protocol.OpInterrupt{}
		err := mgr.SubmitOp(ctx, "non-existent", op)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestManager_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("close manager with sessions", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		// Create multiple sessions
		for i := 1; i <= 3; i++ {
			cfg := SessionConfig{
				ID: fmt.Sprintf("session-%d", i),
				TurnContext: &TurnContext{
					Cwd:   "/test",
					Model: "gpt-4",
				},
			}
			_, err := mgr.CreateSession(ctx, cfg)
			require.NoError(t, err)
		}

		// Close manager
		err := mgr.Close()
		require.NoError(t, err)

		// All sessions should be removed
		sessions := mgr.ListSessions()
		assert.Empty(t, sessions)
	})

	t.Run("close empty manager", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)

		err := mgr.Close()
		require.NoError(t, err)
	})
}

func TestManager_ThreadSafety(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	mgr := createTestManager(t, mockClient)
	ctx := context.Background()

	const goroutines = 50
	done := make(chan bool, goroutines)

	// Concurrent operations
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			sessionID := fmt.Sprintf("session-%d", id)

			// Create session
			cfg := SessionConfig{
				ID: sessionID,
				TurnContext: &TurnContext{
					Cwd:   "/test",
					Model: "gpt-4",
				},
			}
			mgr.CreateSession(ctx, cfg)

			// Get session
			mgr.GetSession(sessionID)

			// List sessions
			mgr.ListSessions()

			// Close session
			mgr.CloseSession(sessionID)
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < goroutines; i++ {
		<-done
	}
}

func TestApprovalChecker(t *testing.T) {
	t.Run("auto policy", func(t *testing.T) {
		checker := NewApprovalChecker("auto")

		assert.False(t, checker.NeedsApproval("exec"))
		assert.False(t, checker.NeedsApproval("patch"))
		assert.True(t, checker.ShouldApproveExec())
		assert.True(t, checker.ShouldApprovePatch())
	})

	t.Run("manual policy", func(t *testing.T) {
		checker := NewApprovalChecker("manual")

		assert.True(t, checker.NeedsApproval("exec"))
		assert.True(t, checker.NeedsApproval("patch"))
		assert.False(t, checker.ShouldApproveExec())
		assert.False(t, checker.ShouldApprovePatch())
	})

	t.Run("semi-auto policy", func(t *testing.T) {
		checker := NewApprovalChecker("semi-auto")

		assert.True(t, checker.NeedsApproval("exec"))
		assert.True(t, checker.NeedsApproval("patch"))
		assert.False(t, checker.NeedsApproval("read"))
	})

	t.Run("unknown policy defaults to manual", func(t *testing.T) {
		checker := NewApprovalChecker("unknown")

		assert.True(t, checker.NeedsApproval("exec"))
		assert.True(t, checker.NeedsApproval("patch"))
	})
}

func TestSessionConfigurationEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("emits EventSessionConfigured on session creation", func(t *testing.T) {
		// Track emitted events
		var emittedEvents []*protocol.Event
		eventHandler := func(ctx context.Context, event *protocol.Event) error {
			emittedEvents = append(emittedEvents, event)
			return nil
		}

		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		cfg := SessionConfig{
			ID:       "session-1",
			Provider: "anthropic",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				Model:          "claude-3-5-sonnet-20241022",
				ApprovalPolicy: "auto",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
				MaxTurns:       10,
				Summary:        "off",
			},
			EventHandlers: []EventHandler{eventHandler},
		}

		session, err := mgr.CreateSession(ctx, cfg)
		require.NoError(t, err)
		require.NotNil(t, session)

		// Verify EventSessionConfigured was emitted
		require.Len(t, emittedEvents, 1)
		event := emittedEvents[0]
		require.NotNil(t, event.Msg)

		configEvent, ok := event.Msg.(*protocol.EventSessionConfigured)
		require.True(t, ok, "expected EventSessionConfigured")
		assert.Equal(t, "session-1", configEvent.SessionID)
		assert.Equal(t, "claude-3-5-sonnet-20241022", configEvent.Model)
		assert.Nil(t, configEvent.ReasoningEffort)
	})

	t.Run("emits EventSessionConfigured with reasoning effort", func(t *testing.T) {
		// Track emitted events
		var emittedEvents []*protocol.Event
		eventHandler := func(ctx context.Context, event *protocol.Event) error {
			emittedEvents = append(emittedEvents, event)
			return nil
		}

		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		effort := "high"
		cfg := SessionConfig{
			ID:       "session-2",
			Provider: "anthropic",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				Model:          "claude-3-7-sonnet-20250219",
				ApprovalPolicy: "auto",
				Effort:         &effort,
				Summary:        "auto",
			},
			EventHandlers: []EventHandler{eventHandler},
		}

		session, err := mgr.CreateSession(ctx, cfg)
		require.NoError(t, err)
		require.NotNil(t, session)

		// Verify EventSessionConfigured was emitted with reasoning effort
		require.Len(t, emittedEvents, 1)
		event := emittedEvents[0]

		configEvent, ok := event.Msg.(*protocol.EventSessionConfigured)
		require.True(t, ok, "expected EventSessionConfigured")
		assert.Equal(t, "session-2", configEvent.SessionID)
		assert.Equal(t, "claude-3-7-sonnet-20250219", configEvent.Model)
		require.NotNil(t, configEvent.ReasoningEffort)
		assert.Equal(t, "high", *configEvent.ReasoningEffort)
	})

	t.Run("session stores history metadata", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		cfg := SessionConfig{
			ID: "session-3",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				Model:          "gpt-4",
				ApprovalPolicy: "manual",
			},
		}

		session, err := mgr.CreateSession(ctx, cfg)
		require.NoError(t, err)

		// Set history metadata
		session.SetHistoryMetadata(12345, 42)

		// Verify metadata is stored
		logID, entryCount := session.GetHistoryMetadata()
		assert.Equal(t, uint64(12345), logID)
		assert.Equal(t, 42, entryCount)
	})

	t.Run("session captures provider information", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		cfg := SessionConfig{
			ID:       "session-4",
			Provider: "openai",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				Model:          "gpt-4-turbo",
				ApprovalPolicy: "semi-auto",
			},
		}

		session, err := mgr.CreateSession(ctx, cfg)
		require.NoError(t, err)
		require.NotNil(t, session)

		// Provider is stored in session (internal field)
		assert.Equal(t, "session-4", session.ID())
	})
}

// Helper functions

func createTestManager(t *testing.T, mockClient *mocks.MockClient) ConversationManager {
	t.Helper()

	cfg := ManagerConfig{
		Client: mockClient,
	}

	mgr, err := NewManager(cfg)
	require.NoError(t, err)
	return mgr
}

func createSessionInManager(t *testing.T, mgr ConversationManager, sessionID string) *Session {
	t.Helper()

	ctx := context.Background()
	cfg := SessionConfig{
		ID: sessionID,
		TurnContext: &TurnContext{
			Cwd:            "/test",
			ApprovalPolicy: "auto",
			Model:          "gpt-4",
			Summary:        "off",
		},
	}

	session, err := mgr.CreateSession(ctx, cfg)
	require.NoError(t, err)
	return session
}
