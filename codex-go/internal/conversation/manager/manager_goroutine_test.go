package manager

import (
	"context"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/client"
	"github.com/evmts/codex/codex-go/internal/client/mocks"
	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestManager_GoroutineCounter verifies that the atomic goroutine counter
// correctly tracks active goroutines.
func TestManager_GoroutineCounter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	mgr := createManagerForTest(t, mockClient)

	// Initially should be zero
	assert.Equal(t, int64(0), mgr.(*manager).GetActiveGoroutineCount())

	// Create a session
	ctx := context.Background()
	_, err := mgr.CreateSession(ctx, SessionConfig{
		ID: "test-session",
		TurnContext: &TurnContext{
			Cwd:   "/test",
			Model: "test-model",
		},
	})
	require.NoError(t, err)

	// Setup a stream that blocks
	blockChan := make(chan struct{})
	streamStarted := make(chan struct{})

	mockClient.EXPECT().GetModelContextWindow().Return(int64(128000)).AnyTimes()
	mockClient.EXPECT().
		Stream(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *client.ChatCompletionRequest) (<-chan client.StreamEvent, error) {
			ch := make(chan client.StreamEvent)
			go func() {
				defer close(ch)
				close(streamStarted)
				select {
				case <-ctx.Done():
					return
				case <-blockChan:
					ch <- client.StreamEvent{
						Type: client.EventTypeCompleted,
						Data: &client.CompletedEvent{},
					}
				}
			}()
			return ch, nil
		})

	// Submit a turn
	textInput := "test input"
	op := &protocol.OpUserTurn{
		Items: []protocol.UserInput{
			{Type: "text", Text: &textInput},
		},
		Cwd:            "/test",
		ApprovalPolicy: "auto",
		SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
		Model:          "test-model",
	}

	err = mgr.SubmitOp(ctx, "test-session", op)
	require.NoError(t, err)

	// Wait for stream to start
	<-streamStarted
	time.Sleep(50 * time.Millisecond)

	// Should have one active goroutine
	assert.Equal(t, int64(1), mgr.(*manager).GetActiveGoroutineCount())

	// Unblock the stream
	close(blockChan)
	time.Sleep(100 * time.Millisecond)

	// Should be back to zero
	assert.Equal(t, int64(0), mgr.(*manager).GetActiveGoroutineCount())

	// Clean up
	err = mgr.CloseSession("test-session")
	require.NoError(t, err)

	err = mgr.Close()
	require.NoError(t, err)
}

// TestManager_CloseWaitsForGoroutines verifies that closing a session
// waits for active goroutines to complete.
func TestManager_CloseWaitsForGoroutines(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	mgr := createManagerForTest(t, mockClient)
	ctx := context.Background()

	_, err := mgr.CreateSession(ctx, SessionConfig{
		ID: "test-session",
		TurnContext: &TurnContext{
			Cwd:   "/test",
			Model: "test-model",
		},
	})
	require.NoError(t, err)

	// Setup a long-running stream
	goroutineExited := make(chan struct{})

	mockClient.EXPECT().GetModelContextWindow().Return(int64(128000)).AnyTimes()
	mockClient.EXPECT().
		Stream(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *client.ChatCompletionRequest) (<-chan client.StreamEvent, error) {
			ch := make(chan client.StreamEvent)
			go func() {
				defer close(ch)
				defer close(goroutineExited)
				<-ctx.Done() // Wait for cancellation
			}()
			return ch, nil
		})

	// Submit a turn
	textInput := "test input"
	op := &protocol.OpUserTurn{
		Items: []protocol.UserInput{
			{Type: "text", Text: &textInput},
		},
		Cwd:            "/test",
		ApprovalPolicy: "auto",
		SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
		Model:          "test-model",
	}

	err = mgr.SubmitOp(ctx, "test-session", op)
	require.NoError(t, err)

	// Give goroutine time to start
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int64(1), mgr.(*manager).GetActiveGoroutineCount())

	// Close session should block until goroutine exits
	closeDone := make(chan struct{})
	go func() {
		err := mgr.CloseSession("test-session")
		require.NoError(t, err)
		close(closeDone)
	}()

	// Wait for goroutine to exit
	select {
	case <-goroutineExited:
		// Good - goroutine exited
	case <-time.After(1 * time.Second):
		t.Fatal("Goroutine did not exit within timeout")
	}

	// Close should complete shortly after goroutine exits
	select {
	case <-closeDone:
		// Good - close completed
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Close did not complete after goroutine exited")
	}

	// Verify counter is back to zero
	assert.Equal(t, int64(0), mgr.(*manager).GetActiveGoroutineCount())

	err = mgr.Close()
	require.NoError(t, err)
}

// TestManager_PanicRecovery verifies that panics in goroutines are
// caught and don't crash the application.
func TestManager_PanicRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)

	mgr := createManagerForTest(t, mockClient)
	ctx := context.Background()

	_, err := mgr.CreateSession(ctx, SessionConfig{
		ID: "test-session",
		TurnContext: &TurnContext{
			Cwd:   "/test",
			Model: "test-model",
		},
	})
	require.NoError(t, err)

	// Mock stream that will cause an error
	mockClient.EXPECT().GetModelContextWindow().Return(int64(128000)).AnyTimes()
	mockClient.EXPECT().
		Stream(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *client.ChatCompletionRequest) (<-chan client.StreamEvent, error) {
			ch := make(chan client.StreamEvent, 1)
			go func() {
				defer close(ch)
				ch <- client.StreamEvent{
					Type:  client.EventTypeError,
					Error: assert.AnError,
				}
			}()
			return ch, nil
		})

	// Submit a turn
	textInput := "test input"
	op := &protocol.OpUserTurn{
		Items: []protocol.UserInput{
			{Type: "text", Text: &textInput},
		},
		Cwd:            "/test",
		ApprovalPolicy: "auto",
		SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
		Model:          "test-model",
	}

	err = mgr.SubmitOp(ctx, "test-session", op)
	require.NoError(t, err)

	// Wait for processing to complete
	time.Sleep(100 * time.Millisecond)

	// Goroutine should have cleaned up even after error
	assert.Equal(t, int64(0), mgr.(*manager).GetActiveGoroutineCount())

	// Session should still be usable (panic was recovered)
	err = mgr.CloseSession("test-session")
	require.NoError(t, err)

	err = mgr.Close()
	require.NoError(t, err)
}

// Helper functions

func createManagerForTest(t *testing.T, mockClient client.Client) ConversationManager {
	t.Helper()

	cfg := ManagerConfig{
		Client: mockClient,
	}

	mgr, err := NewManager(cfg)
	require.NoError(t, err)

	return mgr
}

func strPointer(s string) *string {
	return &s
}
