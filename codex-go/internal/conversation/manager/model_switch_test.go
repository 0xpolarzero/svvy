package manager

import (
	"context"
	"testing"

	"github.com/evmts/codex/codex-go/internal/client/mocks"
	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestManager_SwitchModel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("switch to valid model", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		// Create a session with initial model
		sessionCfg := SessionConfig{
			ID: "test-session",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				ApprovalPolicy: "auto",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
				Model:          "gpt-5-codex",
			},
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)
		require.NotNil(t, session)

		// Verify initial model
		turnCtx := session.GetTurnContext()
		assert.Equal(t, "gpt-5-codex", turnCtx.Model)

		// Switch to a different model
		err = mgr.SwitchModel(ctx, "test-session", "gpt-5")
		require.NoError(t, err)

		// Verify model was updated
		turnCtx = session.GetTurnContext()
		assert.Equal(t, "gpt-5", turnCtx.Model)
	})

	t.Run("switch to default model with empty string", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		// Create a session
		sessionCfg := SessionConfig{
			ID: "test-session",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				ApprovalPolicy: "auto",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
				Model:          "gpt-5",
			},
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)

		// Switch to default (empty string resolves to default)
		err = mgr.SwitchModel(ctx, "test-session", "")
		require.NoError(t, err)

		// Should now have default model (gpt-5-codex)
		turnCtx := session.GetTurnContext()
		assert.Equal(t, "gpt-5-codex", turnCtx.Model)
	})

	t.Run("switch to nonexistent model", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		// Create a session
		sessionCfg := SessionConfig{
			ID: "test-session",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				ApprovalPolicy: "auto",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
				Model:          "gpt-5-codex",
			},
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)

		// Get original model
		originalModel := session.GetTurnContext().Model

		// Try to switch to invalid model
		err = mgr.SwitchModel(ctx, "test-session", "invalid-model")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid model")

		// Model should remain unchanged
		turnCtx := session.GetTurnContext()
		assert.Equal(t, originalModel, turnCtx.Model)
	})

	t.Run("switch model on nonexistent session", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		err := mgr.SwitchModel(ctx, "nonexistent-session", "gpt-5")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("model persists across turns", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		// Create session
		sessionCfg := SessionConfig{
			ID: "test-session",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				ApprovalPolicy: "auto",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
				Model:          "gpt-5-codex",
			},
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)

		// Switch model
		err = mgr.SwitchModel(ctx, "test-session", "gpt-5")
		require.NoError(t, err)

		// Simulate turn context usage - model should persist
		turnCtx := session.GetTurnContext()
		assert.Equal(t, "gpt-5", turnCtx.Model)

		// Even after getting turn context multiple times
		turnCtx2 := session.GetTurnContext()
		assert.Equal(t, "gpt-5", turnCtx2.Model)
	})

	t.Run("switch between multiple models", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		// Create session
		sessionCfg := SessionConfig{
			ID: "test-session",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				ApprovalPolicy: "auto",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
				Model:          "gpt-5-codex",
			},
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)

		// Switch to gpt-5
		err = mgr.SwitchModel(ctx, "test-session", "gpt-5")
		require.NoError(t, err)
		assert.Equal(t, "gpt-5", session.GetTurnContext().Model)

		// Switch back to gpt-5-codex
		err = mgr.SwitchModel(ctx, "test-session", "gpt-5-codex")
		require.NoError(t, err)
		assert.Equal(t, "gpt-5-codex", session.GetTurnContext().Model)

		// Switch to gpt-5 again
		err = mgr.SwitchModel(ctx, "test-session", "gpt-5")
		require.NoError(t, err)
		assert.Equal(t, "gpt-5", session.GetTurnContext().Model)
	})

	t.Run("model switch emits session configured event", func(t *testing.T) {
		mgr := createTestManager(t, mockClient)
		ctx := context.Background()

		// Track emitted events
		var emittedEvents []*protocol.Event
		eventHandler := func(ctx context.Context, event *protocol.Event) error {
			emittedEvents = append(emittedEvents, event)
			return nil
		}

		// Create session with event handler
		sessionCfg := SessionConfig{
			ID: "test-session",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				ApprovalPolicy: "auto",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
				Model:          "gpt-5-codex",
			},
			EventHandlers: []EventHandler{eventHandler},
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)

		// Clear initial events (from session creation)
		emittedEvents = nil

		// Switch model
		err = mgr.SwitchModel(ctx, "test-session", "gpt-5")
		require.NoError(t, err)

		// Verify session configured event was emitted
		require.NotEmpty(t, emittedEvents)

		// Find the session configured event
		var sessionConfiguredEvent *protocol.EventSessionConfigured
		for _, event := range emittedEvents {
			if evt, ok := event.Msg.(*protocol.EventSessionConfigured); ok {
				sessionConfiguredEvent = evt
				break
			}
		}

		require.NotNil(t, sessionConfiguredEvent, "expected EventSessionConfigured to be emitted")
		assert.Equal(t, "gpt-5", sessionConfiguredEvent.Model)
		assert.Equal(t, "test-session", sessionConfiguredEvent.SessionID)

		// Verify model actually changed
		assert.Equal(t, "gpt-5", session.GetTurnContext().Model)
	})
}

func TestModelSwitchingWithMultipleSessions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	mgr := createTestManager(t, mockClient)
	ctx := context.Background()

	// Create multiple sessions with different models
	session1Cfg := SessionConfig{
		ID: "session-1",
		TurnContext: &TurnContext{
			Cwd:            "/test",
			ApprovalPolicy: "auto",
			SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
			Model:          "gpt-5-codex",
		},
	}

	session2Cfg := SessionConfig{
		ID: "session-2",
		TurnContext: &TurnContext{
			Cwd:            "/test",
			ApprovalPolicy: "auto",
			SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
			Model:          "gpt-5",
		},
	}

	session1, err := mgr.CreateSession(ctx, session1Cfg)
	require.NoError(t, err)

	session2, err := mgr.CreateSession(ctx, session2Cfg)
	require.NoError(t, err)

	// Verify initial models
	assert.Equal(t, "gpt-5-codex", session1.GetTurnContext().Model)
	assert.Equal(t, "gpt-5", session2.GetTurnContext().Model)

	// Switch session1 to gpt-5
	err = mgr.SwitchModel(ctx, "session-1", "gpt-5")
	require.NoError(t, err)

	// Switch session2 to gpt-5-codex
	err = mgr.SwitchModel(ctx, "session-2", "gpt-5-codex")
	require.NoError(t, err)

	// Verify models switched
	assert.Equal(t, "gpt-5", session1.GetTurnContext().Model)
	assert.Equal(t, "gpt-5-codex", session2.GetTurnContext().Model)

	// Verify they remain independent
	err = mgr.SwitchModel(ctx, "session-1", "gpt-5-codex")
	require.NoError(t, err)

	assert.Equal(t, "gpt-5-codex", session1.GetTurnContext().Model)
	assert.Equal(t, "gpt-5-codex", session2.GetTurnContext().Model) // Unchanged
}
