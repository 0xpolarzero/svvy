package manager

import (
	"context"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/evmts/codex/codex-go/internal/client/mocks"
)

func TestNewSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("valid configuration", func(t *testing.T) {
		cfg := SessionConfig{
			ID:     "test-session-1",
			Client: mockClient,
			TurnContext: &TurnContext{
				Cwd:            "/test",
				ApprovalPolicy: "auto",
				Model:          "gpt-4",
				Summary:        "off",
			},
		}

		session, err := NewSession(cfg)
		require.NoError(t, err)
		require.NotNil(t, session)

		assert.Equal(t, "test-session-1", session.ID())
		assert.Equal(t, StateIdle, session.State())
		assert.False(t, session.IsClosed())
		assert.True(t, session.CanAcceptTurn())
	})

	t.Run("missing ID", func(t *testing.T) {
		cfg := SessionConfig{
			Client: mockClient,
			TurnContext: &TurnContext{
				Cwd:   "/test",
				Model: "gpt-4",
			},
		}

		session, err := NewSession(cfg)
		require.Error(t, err)
		assert.Nil(t, session)
		assert.Contains(t, err.Error(), "session ID is required")
	})

	t.Run("missing client", func(t *testing.T) {
		cfg := SessionConfig{
			ID: "test-session-1",
			TurnContext: &TurnContext{
				Cwd:   "/test",
				Model: "gpt-4",
			},
		}

		session, err := NewSession(cfg)
		require.Error(t, err)
		assert.Nil(t, session)
		assert.Contains(t, err.Error(), "client is required")
	})

	t.Run("missing turn context", func(t *testing.T) {
		cfg := SessionConfig{
			ID:     "test-session-1",
			Client: mockClient,
		}

		session, err := NewSession(cfg)
		require.Error(t, err)
		assert.Nil(t, session)
		assert.Contains(t, err.Error(), "turn context is required")
	})
}

func TestSession_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("close idle session", func(t *testing.T) {
		session := createTestSession(t, mockClient)

		err := session.Close()
		require.NoError(t, err)

		assert.True(t, session.IsClosed())
		assert.Equal(t, StateClosed, session.State())
		assert.False(t, session.CanAcceptTurn())
	})

	t.Run("close already closed session", func(t *testing.T) {
		session := createTestSession(t, mockClient)

		err := session.Close()
		require.NoError(t, err)

		// Try to close again
		err = session.Close()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already closed")
	})

	t.Run("context cancelled on close", func(t *testing.T) {
		session := createTestSession(t, mockClient)

		ctx := session.Context()
		select {
		case <-ctx.Done():
			t.Fatal("context should not be cancelled yet")
		default:
		}

		err := session.Close()
		require.NoError(t, err)

		// Context should be cancelled
		select {
		case <-ctx.Done():
			// Expected
		case <-time.After(100 * time.Millisecond):
			t.Fatal("context should be cancelled after close")
		}
	})
}

func TestSession_SubmitTurn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("submit turn from idle", func(t *testing.T) {
		session := createTestSession(t, mockClient)
		ctx := context.Background()

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

		submissionID, err := session.SubmitTurn(ctx, op)
		require.NoError(t, err)
		assert.NotEmpty(t, submissionID)

		assert.Equal(t, StateProcessingTurn, session.State())
		assert.False(t, session.CanAcceptTurn())

		// Check turn context was updated
		turnCtx := session.GetTurnContext()
		assert.Equal(t, "/test", turnCtx.Cwd)
		assert.Equal(t, "auto", turnCtx.ApprovalPolicy)
		assert.Equal(t, "gpt-4", turnCtx.Model)
	})

	t.Run("cannot submit turn while processing", func(t *testing.T) {
		session := createTestSession(t, mockClient)
		ctx := context.Background()

		op := &protocol.OpUserTurn{
			Items:          []protocol.UserInput{{Type: "text", Text: strPtr("msg1")}},
			Cwd:            "/test",
			ApprovalPolicy: "auto",
			SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
			Model:          "gpt-4",
			Summary:        "off",
		}

		// Submit first turn
		_, err := session.SubmitTurn(ctx, op)
		require.NoError(t, err)

		// Try to submit second turn (should fail)
		_, err = session.SubmitTurn(ctx, op)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot accept turn")
	})

	t.Run("can submit turn after completion", func(t *testing.T) {
		session := createTestSession(t, mockClient)
		ctx := context.Background()

		op := &protocol.OpUserTurn{
			Items:          []protocol.UserInput{{Type: "text", Text: strPtr("msg")}},
			Cwd:            "/test",
			ApprovalPolicy: "auto",
			SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
			Model:          "gpt-4",
			Summary:        "off",
		}

		// Submit turn
		_, err := session.SubmitTurn(ctx, op)
		require.NoError(t, err)

		// Complete turn
		err = session.CompleteTurn()
		require.NoError(t, err)

		// Reset to idle
		err = session.ResetToIdle()
		require.NoError(t, err)

		// Submit another turn (should succeed)
		_, err = session.SubmitTurn(ctx, op)
		require.NoError(t, err)
	})
}

func TestSession_Interrupt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("interrupt processing turn", func(t *testing.T) {
		session := createTestSession(t, mockClient)
		ctx := context.Background()

		// Start a turn
		op := createTestUserTurn()
		_, err := session.SubmitTurn(ctx, op)
		require.NoError(t, err)

		// Interrupt
		err = session.SubmitInterrupt(ctx)
		require.NoError(t, err)

		assert.Equal(t, StateInterrupted, session.State())
		assert.True(t, session.CanAcceptTurn())
	})

	t.Run("interrupt while awaiting approval", func(t *testing.T) {
		session := createTestSession(t, mockClient)
		ctx := context.Background()

		// Start a turn and request approval
		op := createTestUserTurn()
		submissionID, err := session.SubmitTurn(ctx, op)
		require.NoError(t, err)

		err = session.RequestApproval(submissionID, ApprovalTypeExec)
		require.NoError(t, err)

		// Interrupt
		err = session.SubmitInterrupt(ctx)
		require.NoError(t, err)

		assert.Equal(t, StateInterrupted, session.State())
	})

	t.Run("cannot interrupt from idle", func(t *testing.T) {
		session := createTestSession(t, mockClient)
		ctx := context.Background()

		err := session.SubmitInterrupt(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot interrupt")
	})
}

func TestSession_Approval(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("request and approve", func(t *testing.T) {
		session := createTestSession(t, mockClient)
		ctx := context.Background()

		// Start a turn
		op := createTestUserTurn()
		submissionID, err := session.SubmitTurn(ctx, op)
		require.NoError(t, err)

		// Request approval
		err = session.RequestApproval(submissionID, ApprovalTypeExec)
		require.NoError(t, err)

		assert.Equal(t, StateAwaitingApproval, session.State())
		assert.NotNil(t, session.GetPendingApproval())

		// Approve
		err = session.SubmitApproval(ctx, submissionID, "approve")
		require.NoError(t, err)

		assert.Equal(t, StateProcessingTurn, session.State())
		assert.Nil(t, session.GetPendingApproval())
	})

	t.Run("request and reject", func(t *testing.T) {
		session := createTestSession(t, mockClient)
		ctx := context.Background()

		// Start a turn
		op := createTestUserTurn()
		submissionID, err := session.SubmitTurn(ctx, op)
		require.NoError(t, err)

		// Request approval
		err = session.RequestApproval(submissionID, ApprovalTypePatch)
		require.NoError(t, err)

		// Reject
		err = session.SubmitApproval(ctx, submissionID, "reject")
		require.NoError(t, err)

		assert.Equal(t, StateInterrupted, session.State())
		assert.Nil(t, session.GetPendingApproval())
	})

	t.Run("cannot request approval from idle", func(t *testing.T) {
		session := createTestSession(t, mockClient)

		err := session.RequestApproval("test-id", ApprovalTypeExec)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot request approval")
	})

	t.Run("cannot submit approval when not awaiting", func(t *testing.T) {
		session := createTestSession(t, mockClient)
		ctx := context.Background()

		err := session.SubmitApproval(ctx, "test-id", "approve")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not awaiting approval")
	})

	t.Run("submission ID mismatch", func(t *testing.T) {
		session := createTestSession(t, mockClient)
		ctx := context.Background()

		// Start a turn and request approval
		op := createTestUserTurn()
		submissionID, err := session.SubmitTurn(ctx, op)
		require.NoError(t, err)

		err = session.RequestApproval(submissionID, ApprovalTypeExec)
		require.NoError(t, err)

		// Submit approval with wrong ID
		err = session.SubmitApproval(ctx, "wrong-id", "approve")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "submission ID mismatch")
	})
}

func TestSession_CompleteTurn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("complete turn from processing", func(t *testing.T) {
		session := createTestSession(t, mockClient)
		ctx := context.Background()

		// Start a turn
		op := createTestUserTurn()
		_, err := session.SubmitTurn(ctx, op)
		require.NoError(t, err)

		// Complete
		err = session.CompleteTurn()
		require.NoError(t, err)

		assert.Equal(t, StateCompleted, session.State())
		assert.True(t, session.CanAcceptTurn())
	})

	t.Run("cannot complete from idle", func(t *testing.T) {
		session := createTestSession(t, mockClient)

		err := session.CompleteTurn()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot complete turn")
	})
}

func TestSession_FailTurn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("fail turn with error message", func(t *testing.T) {
		session := createTestSession(t, mockClient)
		ctx := context.Background()

		// Start a turn
		op := createTestUserTurn()
		_, err := session.SubmitTurn(ctx, op)
		require.NoError(t, err)

		// Fail
		errMsg := "test error"
		err = session.FailTurn(errMsg)
		require.NoError(t, err)

		assert.Equal(t, StateError, session.State())
		assert.True(t, session.CanAcceptTurn())
	})
}

func TestSession_TurnContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("get initial turn context", func(t *testing.T) {
		session := createTestSession(t, mockClient)

		ctx := session.GetTurnContext()
		assert.Equal(t, "/test/workspace", ctx.Cwd)
		assert.Equal(t, "auto", ctx.ApprovalPolicy)
		assert.Equal(t, "gpt-4", ctx.Model)
	})

	t.Run("update turn context", func(t *testing.T) {
		session := createTestSession(t, mockClient)

		newCwd := "/new/path"
		newModel := "gpt-5"
		override := &protocol.OpOverrideTurnContext{
			Cwd:   &newCwd,
			Model: &newModel,
		}

		err := session.UpdateTurnContext(override)
		require.NoError(t, err)

		ctx := session.GetTurnContext()
		assert.Equal(t, "/new/path", ctx.Cwd)
		assert.Equal(t, "gpt-5", ctx.Model)
		assert.Equal(t, "auto", ctx.ApprovalPolicy) // Unchanged
	})
}

func TestSession_TokenUsage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("update and get token usage", func(t *testing.T) {
		session := createTestSession(t, mockClient)

		usage := &protocol.TokenUsage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		}

		session.UpdateTokenUsage(usage)

		retrieved := session.GetTokenUsage()
		assert.Equal(t, int64(100), retrieved.InputTokens)
		assert.Equal(t, int64(50), retrieved.OutputTokens)
		assert.Equal(t, int64(150), retrieved.TotalTokens)
	})
}

func TestSession_AgentMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("set and get agent message", func(t *testing.T) {
		session := createTestSession(t, mockClient)

		msg := "This is the agent response"
		session.SetLastAgentMessage(msg)

		retrieved := session.GetLastAgentMessage()
		assert.Equal(t, msg, retrieved)
	})
}

func TestSession_EventHandlers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	t.Run("emit event to handlers", func(t *testing.T) {
		var receivedEvents []*protocol.Event
		handler := func(ctx context.Context, event *protocol.Event) error {
			receivedEvents = append(receivedEvents, event)
			return nil
		}

		cfg := SessionConfig{
			ID:     "test-session",
			Client: mockClient,
			TurnContext: &TurnContext{
				Cwd:   "/test",
				Model: "gpt-4",
			},
			EventHandlers: []EventHandler{handler},
		}

		session, err := NewSession(cfg)
		require.NoError(t, err)

		ctx := context.Background()
		event := &protocol.Event{
			ID: "test-event",
			Msg: &protocol.EventAgentMessage{
				Message: "test message",
			},
		}

		err = session.EmitEvent(ctx, event)
		require.NoError(t, err)

		require.Len(t, receivedEvents, 1)
		assert.Equal(t, "test-event", receivedEvents[0].ID)
	})

	t.Run("multiple handlers", func(t *testing.T) {
		count1 := 0
		count2 := 0

		handler1 := func(ctx context.Context, event *protocol.Event) error {
			count1++
			return nil
		}
		handler2 := func(ctx context.Context, event *protocol.Event) error {
			count2++
			return nil
		}

		cfg := SessionConfig{
			ID:     "test-session",
			Client: mockClient,
			TurnContext: &TurnContext{
				Cwd:   "/test",
				Model: "gpt-4",
			},
			EventHandlers: []EventHandler{handler1, handler2},
		}

		session, err := NewSession(cfg)
		require.NoError(t, err)

		ctx := context.Background()
		event := &protocol.Event{
			ID:  "test-event",
			Msg: &protocol.EventAgentMessage{Message: "test"},
		}

		err = session.EmitEvent(ctx, event)
		require.NoError(t, err)

		assert.Equal(t, 1, count1)
		assert.Equal(t, 1, count2)
	})
}

func TestSession_ThreadSafety(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	session := createTestSession(t, mockClient)

	// Test concurrent access to various session methods
	const goroutines = 50
	done := make(chan bool, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Read operations
			_ = session.ID()
			_ = session.State()
			_ = session.GetTurnContext()
			_ = session.GetTokenUsage()
			_ = session.GetLastAgentMessage()
			_ = session.GetPendingApproval()

			// Write operations
			session.SetLastAgentMessage("test")
			session.UpdateTokenUsage(&protocol.TokenUsage{
				InputTokens:  int64(id),
				OutputTokens: int64(id * 2),
			})
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Session should still be in valid state
	assert.False(t, session.IsClosed())
}

// Helper functions

func createTestSession(t *testing.T, mockClient *mocks.MockClient) *Session {
	t.Helper()

	cfg := SessionConfig{
		ID:     "test-session",
		Client: mockClient,
		TurnContext: &TurnContext{
			Cwd:            "/test/workspace",
			ApprovalPolicy: "auto",
			SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
			Model:          "gpt-4",
			Summary:        "off",
		},
	}

	session, err := NewSession(cfg)
	require.NoError(t, err)
	return session
}

func createTestUserTurn() *protocol.OpUserTurn {
	return &protocol.OpUserTurn{
		Items: []protocol.UserInput{
			{Type: "text", Text: strPtr("test message")},
		},
		Cwd:            "/test",
		ApprovalPolicy: "auto",
		SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
		Model:          "gpt-4",
		Summary:        "off",
	}
}

func strPtr(s string) *string {
	return &s
}
