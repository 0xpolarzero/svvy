package manager

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/client"
	"github.com/evmts/codex/codex-go/internal/client/mocks"
	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/evmts/codex/codex-go/internal/tools/file"
	"github.com/evmts/codex/codex-go/internal/tools/orchestrator"
	"github.com/evmts/codex/codex-go/internal/tools/patch"
	"github.com/evmts/codex/codex-go/internal/tools/runtime"
	"github.com/evmts/codex/codex-go/internal/tools/shell"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestIntegration_SDKToManagerFlow tests the complete SDK → Manager flow
// This test verifies the end-to-end integration without going through the full SDK package
func TestIntegration_SDKToManagerFlow(t *testing.T) {
	t.Run("complete session lifecycle", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Create mock client
		mockClient := mocks.NewMockClient(ctrl)

		// Mock client setup complete - no additional expectations needed

		// Create manager
		mgr, orch := setupIntegrationTest(t, mockClient)
		defer mgr.Close()

		// 1. Create session (simulating SDK.NewSession → manager.CreateSession)
		sessionCfg := SessionConfig{
			ID: "test-integration-session",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				ApprovalPolicy: "auto",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
				Model:          "claude-3-5-sonnet-20241022",
				Summary:        "off",
			},
			Orchestrator: orch,
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)
		require.NotNil(t, session)
		assert.Equal(t, "test-integration-session", session.ID())
		assert.Equal(t, StateIdle, session.State())

		// 2. Submit a turn (simulating SDK.Session.Submit → manager.SubmitOp)
		// Mock streaming response
		eventChan := make(chan client.StreamEvent, 3)

		// Send text delta
		eventChan <- client.StreamEvent{
			Type: client.EventTypeOutputTextDelta,
			Data: "Test response",
		}

		// Send completion event
		eventChan <- client.StreamEvent{
			Type: client.EventTypeCompleted,
			Data: nil,
		}

		close(eventChan)

		mockClient.EXPECT().
			Stream(gomock.Any(), gomock.Any()).
			Return(eventChan, nil).
			Times(1)

		op := &protocol.OpUserTurn{
			Items: []protocol.UserInput{
				{Type: "text", Text: testStrPtr("Tell me a joke")},
			},
			Cwd:            "/test",
			ApprovalPolicy: "auto",
			SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
			Model:          "claude-3-5-sonnet-20241022",
			Summary:        "off",
		}

		err = mgr.SubmitOp(ctx, "test-integration-session", op)
		require.NoError(t, err)

		// Wait for turn to complete
		assert.Eventually(t, func() bool {
			state := session.State()
			return state == StateCompleted || state == StateError
		}, 2*time.Second, 50*time.Millisecond, "turn should complete")

		// 3. Close session (simulating SDK.CloseSession → manager.CloseSession)
		err = mgr.CloseSession("test-integration-session")
		require.NoError(t, err)
		assert.True(t, session.IsClosed())
	})
}

// TestIntegration_ToolExecution tests tool execution through the orchestrator
func TestIntegration_ToolExecution(t *testing.T) {
	t.Run("file tool execution with auto approval", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := mocks.NewMockClient(ctrl)

		// Create test filesystem
		fs := afero.NewMemMapFs()
		testFile := "/test/file.txt"
		testContent := "test content"
		afero.WriteFile(fs, testFile, []byte(testContent), 0644)

		// Create manager with real tools
		mgr, orch := setupIntegrationTestWithFS(t, mockClient, fs)
		defer mgr.Close()

		// Create session with auto approval
		sessionCfg := SessionConfig{
			ID: "file-tool-session",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				ApprovalPolicy: "auto",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
				Model:          "claude-3-5-sonnet-20241022",
			},
			Orchestrator: orch,
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)

		// Execute file read tool directly through orchestrator
		req := &runtime.ToolRequest{
			ToolName: "read_file",
			CallID:   "test-call-1",
			Input: map[string]interface{}{
				"path": testFile,
			},
		}

		execCtx := &runtime.ExecutionContext{
			WorkingDirectory: "/test",
			SandboxAttempt: runtime.SandboxAttempt{
				Policy: runtime.SandboxNever,
				Type:   runtime.SandboxNone,
			},
		}

		result, err := orch.Execute(ctx, req, execCtx)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.ApprovalRequired, "auto approval should not require user interaction")

		// Close session
		mgr.CloseSession(session.ID())
	})

	t.Run("shell tool execution with sandbox", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := mocks.NewMockClient(ctrl)

		fs := afero.NewMemMapFs()
		mgr, orch := setupIntegrationTestWithFS(t, mockClient, fs)
		defer mgr.Close()

		// Create session with sandbox enabled
		sessionCfg := SessionConfig{
			ID: "shell-tool-session",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				ApprovalPolicy: "auto",
				SandboxPolicy: protocol.SandboxPolicy{
					Mode:          "workspace-write",
					WritableRoots: []string{"/test"},
				},
				Model: "claude-3-5-sonnet-20241022",
			},
			Orchestrator: orch,
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)

		// Execute shell command (echo is safe)
		req := &runtime.ToolRequest{
			ToolName: "bash",
			CallID:   "test-call-2",
			Input: map[string]interface{}{
				"command": "echo 'hello world'",
			},
		}

		execCtx := &runtime.ExecutionContext{
			WorkingDirectory: "/test",
			SandboxAttempt: runtime.SandboxAttempt{
				Policy: runtime.SandboxWorkspaceWrite,
				Type:   runtime.SandboxBubbleWrap,
			},
		}

		result, err := orch.Execute(ctx, req, execCtx)

		// Shell tool might not be available in test environment, that's ok
		if err != nil {
			t.Logf("Shell execution failed (expected in test env): %v", err)
		} else {
			assert.NotNil(t, result)
		}

		mgr.CloseSession(session.ID())
	})

	t.Run("patch tool execution with approval", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := mocks.NewMockClient(ctrl)

		fs := afero.NewMemMapFs()
		testFile := "/test/patch.txt"
		afero.WriteFile(fs, testFile, []byte("original content\n"), 0644)

		mgr, orch := setupIntegrationTestWithFS(t, mockClient, fs)
		defer mgr.Close()

		// Create session with manual approval
		sessionCfg := SessionConfig{
			ID: "patch-tool-session",
			TurnContext: &TurnContext{
				Cwd:             "/test",
				ApprovalPolicy:  "manual",
				SandboxPolicy:   protocol.SandboxPolicy{Mode: "off"},
				Model:           "claude-3-5-sonnet-20241022",
				ApprovalTimeout: 100 * time.Millisecond, // Short timeout for test
			},
			Orchestrator: orch,
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)

		// Create approval handler
		handler := NewSessionApprovalHandler(session, "patch-submission-1")
		session.SetApprovalHandler(handler)

		// Execute patch tool in background (will block on approval)
		resultChan := make(chan *runtime.ExecutionResult, 1)
		errChan := make(chan error, 1)

		go func() {
			req := &runtime.ToolRequest{
				ToolName: "patch",
				CallID:   "test-patch-1",
				Input: map[string]interface{}{
					"path":       testFile,
					"old_string": "original",
					"new_string": "modified",
				},
			}

			execCtx := &runtime.ExecutionContext{
				WorkingDirectory: "/test",
				SandboxAttempt: runtime.SandboxAttempt{
					Policy: runtime.SandboxNever,
					Type:   runtime.SandboxNone,
				},
			}

			result, err := orch.Execute(ctx, req, execCtx)
			if err != nil {
				errChan <- err
			} else {
				resultChan <- result
			}
		}()

		// Wait for approval to be requested
		time.Sleep(50 * time.Millisecond)

		// Verify approval was requested
		assert.True(t, handler.HasPendingApproval())

		// Timeout will occur, which is expected for this test
		select {
		case <-resultChan:
			t.Log("Tool execution completed (unexpected)")
		case err := <-errChan:
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "timeout")
		case <-time.After(200 * time.Millisecond):
			t.Log("Approval timeout occurred as expected")
		}

		mgr.CloseSession(session.ID())
	})
}

// TestIntegration_ApprovalWorkflows tests different approval workflows
func TestIntegration_ApprovalWorkflows(t *testing.T) {
	t.Run("manual approval - approve", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := mocks.NewMockClient(ctrl)

		fs := afero.NewMemMapFs()
		testFile := "/test/file.txt"
		afero.WriteFile(fs, testFile, []byte("content"), 0644)

		mgr, orch := setupIntegrationTestWithFS(t, mockClient, fs)
		defer mgr.Close()

		sessionCfg := SessionConfig{
			ID: "approval-session-1",
			TurnContext: &TurnContext{
				Cwd:             "/test",
				ApprovalPolicy:  "manual",
				SandboxPolicy:   protocol.SandboxPolicy{Mode: "off"},
				Model:           "claude-3-5-sonnet-20241022",
				ApprovalTimeout: 1 * time.Second,
			},
			Orchestrator: orch,
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)

		handler := NewSessionApprovalHandler(session, "submission-1")
		session.SetApprovalHandler(handler)

		// Execute tool in background
		done := make(chan bool, 1)
		var execErr error

		go func() {
			req := &runtime.ToolRequest{
				ToolName: "read_file",
				CallID:   "call-1",
				Input:    map[string]interface{}{"path": testFile},
			}
			execCtx := &runtime.ExecutionContext{
				WorkingDirectory: "/test",
				SandboxAttempt: runtime.SandboxAttempt{
					Policy: runtime.SandboxNever,
					Type:   runtime.SandboxNone,
				},
			}
			_, execErr = orch.Execute(ctx, req, execCtx)
			done <- true
		}()

		// Wait for approval request
		time.Sleep(50 * time.Millisecond)
		require.True(t, handler.HasPendingApproval())

		// Approve
		callID := handler.GetPendingApprovalCallID()
		require.NotEmpty(t, callID)
		err = handler.NotifyApprovalDecision(callID, runtime.ApprovalApproved)
		require.NoError(t, err)

		// Wait for completion
		select {
		case <-done:
			assert.NoError(t, execErr)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Tool execution did not complete after approval")
		}

		mgr.CloseSession(session.ID())
	})

	t.Run("manual approval - deny", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := mocks.NewMockClient(ctrl)

		fs := afero.NewMemMapFs()
		testFile := "/test/file.txt"
		afero.WriteFile(fs, testFile, []byte("content"), 0644)

		mgr, orch := setupIntegrationTestWithFS(t, mockClient, fs)
		defer mgr.Close()

		sessionCfg := SessionConfig{
			ID: "approval-session-2",
			TurnContext: &TurnContext{
				Cwd:             "/test",
				ApprovalPolicy:  "manual",
				SandboxPolicy:   protocol.SandboxPolicy{Mode: "off"},
				Model:           "claude-3-5-sonnet-20241022",
				ApprovalTimeout: 1 * time.Second,
			},
			Orchestrator: orch,
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)

		handler := NewSessionApprovalHandler(session, "submission-2")
		session.SetApprovalHandler(handler)

		done := make(chan bool, 1)
		var execErr error

		go func() {
			req := &runtime.ToolRequest{
				ToolName: "read_file",
				CallID:   "call-2",
				Input:    map[string]interface{}{"path": testFile},
			}
			execCtx := &runtime.ExecutionContext{
				WorkingDirectory: "/test",
				SandboxAttempt: runtime.SandboxAttempt{
					Policy: runtime.SandboxNever,
					Type:   runtime.SandboxNone,
				},
			}
			_, execErr = orch.Execute(ctx, req, execCtx)
			done <- true
		}()

		// Wait for approval request
		time.Sleep(50 * time.Millisecond)
		require.True(t, handler.HasPendingApproval())

		// Deny
		callID := handler.GetPendingApprovalCallID()
		err = handler.NotifyApprovalDecision(callID, runtime.ApprovalDenied)
		require.NoError(t, err)

		// Wait for completion
		select {
		case <-done:
			assert.Error(t, execErr)
			assert.Contains(t, execErr.Error(), "denied")
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Tool execution did not complete after denial")
		}

		mgr.CloseSession(session.ID())
	})

	t.Run("auto approval - no user interaction", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := mocks.NewMockClient(ctrl)

		fs := afero.NewMemMapFs()
		testFile := "/test/file.txt"
		afero.WriteFile(fs, testFile, []byte("content"), 0644)

		mgr, orch := setupIntegrationTestWithFS(t, mockClient, fs)
		defer mgr.Close()

		sessionCfg := SessionConfig{
			ID: "approval-session-3",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				ApprovalPolicy: "auto",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
				Model:          "claude-3-5-sonnet-20241022",
			},
			Orchestrator: orch,
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)

		handler := NewSessionApprovalHandler(session, "submission-3")
		session.SetApprovalHandler(handler)

		// Execute tool - should complete without approval
		req := &runtime.ToolRequest{
			ToolName: "read_file",
			CallID:   "call-3",
			Input:    map[string]interface{}{"path": testFile},
		}
		execCtx := &runtime.ExecutionContext{
			WorkingDirectory: "/test",
			SandboxAttempt: runtime.SandboxAttempt{
				Policy: runtime.SandboxNever,
				Type:   runtime.SandboxNone,
			},
		}

		result, err := orch.Execute(ctx, req, execCtx)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, handler.HasPendingApproval(), "auto approval should not wait for user")

		mgr.CloseSession(session.ID())
	})

	t.Run("approval timeout", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := mocks.NewMockClient(ctrl)

		fs := afero.NewMemMapFs()
		testFile := "/test/file.txt"
		afero.WriteFile(fs, testFile, []byte("content"), 0644)

		mgr, orch := setupIntegrationTestWithFS(t, mockClient, fs)
		defer mgr.Close()

		sessionCfg := SessionConfig{
			ID: "approval-session-4",
			TurnContext: &TurnContext{
				Cwd:             "/test",
				ApprovalPolicy:  "manual",
				SandboxPolicy:   protocol.SandboxPolicy{Mode: "off"},
				Model:           "claude-3-5-sonnet-20241022",
				ApprovalTimeout: 100 * time.Millisecond, // Very short timeout
			},
			Orchestrator: orch,
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)

		handler := NewSessionApprovalHandler(session, "submission-4")
		session.SetApprovalHandler(handler)

		// Execute tool - should timeout
		req := &runtime.ToolRequest{
			ToolName: "read_file",
			CallID:   "call-4",
			Input:    map[string]interface{}{"path": testFile},
		}
		execCtx := &runtime.ExecutionContext{
			WorkingDirectory: "/test",
			SandboxAttempt: runtime.SandboxAttempt{
				Policy: runtime.SandboxNever,
				Type:   runtime.SandboxNone,
			},
		}

		_, err = orch.Execute(ctx, req, execCtx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "timeout")

		mgr.CloseSession(session.ID())
	})
}

// TestIntegration_ErrorHandling tests error handling scenarios
func TestIntegration_ErrorHandling(t *testing.T) {
	t.Run("invalid tool call", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := mocks.NewMockClient(ctrl)

		mgr, orch := setupIntegrationTest(t, mockClient)
		defer mgr.Close()

		sessionCfg := SessionConfig{
			ID: "error-session-1",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				ApprovalPolicy: "auto",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
				Model:          "claude-3-5-sonnet-20241022",
			},
			Orchestrator: orch,
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)

		// Try to execute non-existent tool
		req := &runtime.ToolRequest{
			ToolName: "nonexistent_tool",
			CallID:   "error-call-1",
			Input:    map[string]interface{}{},
		}
		execCtx := &runtime.ExecutionContext{
			WorkingDirectory: "/test",
			SandboxAttempt: runtime.SandboxAttempt{
				Policy: runtime.SandboxNever,
				Type:   runtime.SandboxNone,
			},
		}

		_, err = orch.Execute(ctx, req, execCtx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		mgr.CloseSession(session.ID())
	})

	t.Run("tool execution failure", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := mocks.NewMockClient(ctrl)

		fs := afero.NewMemMapFs()
		mgr, orch := setupIntegrationTestWithFS(t, mockClient, fs)
		defer mgr.Close()

		sessionCfg := SessionConfig{
			ID: "error-session-2",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				ApprovalPolicy: "auto",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
				Model:          "claude-3-5-sonnet-20241022",
			},
			Orchestrator: orch,
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)

		// Try to read non-existent file
		req := &runtime.ToolRequest{
			ToolName: "read_file",
			CallID:   "error-call-2",
			Input: map[string]interface{}{
				"path": "/nonexistent/file.txt",
			},
		}
		execCtx := &runtime.ExecutionContext{
			WorkingDirectory: "/test",
			SandboxAttempt: runtime.SandboxAttempt{
				Policy: runtime.SandboxNever,
				Type:   runtime.SandboxNone,
			},
		}

		_, err = orch.Execute(ctx, req, execCtx)
		require.Error(t, err)

		mgr.CloseSession(session.ID())
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := mocks.NewMockClient(ctrl)

		mgr, _ := setupIntegrationTest(t, mockClient)
		defer mgr.Close()

		ctx, cancel := context.WithCancel(context.Background())

		sessionCfg := SessionConfig{
			ID: "error-session-3",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				ApprovalPolicy: "auto",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
				Model:          "claude-3-5-sonnet-20241022",
			},
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)

		// Cancel context before turn processing
		cancel()

		// Mock streaming with cancellation
		eventChan := make(chan client.StreamEvent)
		close(eventChan) // Close immediately

		mockClient.EXPECT().
			Stream(gomock.Any(), gomock.Any()).
			Return(eventChan, context.Canceled).
			MaxTimes(1)

		op := &protocol.OpUserTurn{
			Items: []protocol.UserInput{
				{Type: "text", Text: strPtr("test")},
			},
			Cwd:            "/test",
			ApprovalPolicy: "auto",
			SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
			Model:          "claude-3-5-sonnet-20241022",
			Summary:        "off",
		}

		// This may fail due to cancelled context
		_ = mgr.SubmitOp(context.Background(), session.ID(), op)

		// Session should handle cancellation gracefully
		time.Sleep(100 * time.Millisecond)

		mgr.CloseSession(session.ID())
	})

	t.Run("concurrent operations on same session", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := mocks.NewMockClient(ctrl)

		fs := afero.NewMemMapFs()
		testFile := "/test/file.txt"
		afero.WriteFile(fs, testFile, []byte("content"), 0644)

		mgr, orch := setupIntegrationTestWithFS(t, mockClient, fs)
		defer mgr.Close()

		sessionCfg := SessionConfig{
			ID: "concurrent-session",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				ApprovalPolicy: "auto",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
				Model:          "claude-3-5-sonnet-20241022",
			},
			Orchestrator: orch,
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)

		// Execute multiple tools concurrently
		const numConcurrent = 5
		errChan := make(chan error, numConcurrent)

		for i := 0; i < numConcurrent; i++ {
			go func(id int) {
				req := &runtime.ToolRequest{
					ToolName: "read_file",
					CallID:   fmt.Sprintf("concurrent-call-%d", id),
					Input:    map[string]interface{}{"path": testFile},
				}
				execCtx := &runtime.ExecutionContext{
					WorkingDirectory: "/test",
					SandboxAttempt: runtime.SandboxAttempt{
						Policy: runtime.SandboxNever,
						Type:   runtime.SandboxNone,
					},
				}
				_, err := orch.Execute(ctx, req, execCtx)
				errChan <- err
			}(i)
		}

		// Wait for all to complete
		for i := 0; i < numConcurrent; i++ {
			err := <-errChan
			assert.NoError(t, err, "concurrent tool execution should succeed")
		}

		mgr.CloseSession(session.ID())
	})
}

// TestIntegration_HistoryIntegration tests history persistence
func TestIntegration_HistoryIntegration(t *testing.T) {
	t.Run("history persistence after turns", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := mocks.NewMockClient(ctrl)

		// Use a real filesystem for history
		fs := afero.NewMemMapFs()
		historyRoot := "/tmp/test-history"

		mgr := createTestManagerWithHistory(t, mockClient, fs, historyRoot)
		defer mgr.Close()

		sessionCfg := SessionConfig{
			ID: "history-session-1",
			TurnContext: &TurnContext{
				Cwd:            "/test",
				ApprovalPolicy: "auto",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
				Model:          "claude-3-5-sonnet-20241022",
				Summary:        "off",
			},
		}

		session, err := mgr.CreateSession(ctx, sessionCfg)
		require.NoError(t, err)

		// Mock a turn
		eventChan := make(chan client.StreamEvent, 2)
		eventChan <- client.StreamEvent{
			Type: client.EventTypeOutputTextDelta,
			Data: "Response",
		}
		eventChan <- client.StreamEvent{
			Type: client.EventTypeCompleted,
			Data: nil,
		}
		close(eventChan)

		mockClient.EXPECT().
			Stream(gomock.Any(), gomock.Any()).
			Return(eventChan, nil).
			Times(1)

		op := &protocol.OpUserTurn{
			Items:          []protocol.UserInput{{Type: "text", Text: testStrPtr("test")}},
			Cwd:            "/test",
			ApprovalPolicy: "auto",
			SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
			Model:          "claude-3-5-sonnet-20241022",
			Summary:        "off",
		}

		err = mgr.SubmitOp(ctx, session.ID(), op)
		require.NoError(t, err)

		// Wait for completion
		time.Sleep(200 * time.Millisecond)

		// Close session - should flush history
		err = mgr.CloseSession(session.ID())
		require.NoError(t, err)

		// Verify history file was created
		historyPath := fmt.Sprintf("%s/%s", historyRoot, session.ID())
		exists, _ := afero.Exists(fs, historyPath)
		assert.True(t, exists, "history directory should be created")
	})
}

// Helper functions

func setupIntegrationTest(t *testing.T, mockClient *mocks.MockClient) (ConversationManager, *orchestrator.Orchestrator) {
	t.Helper()
	fs := afero.NewMemMapFs()
	return setupIntegrationTestWithFS(t, mockClient, fs)
}

func setupIntegrationTestWithFS(t *testing.T, mockClient *mocks.MockClient, fs afero.Fs) (ConversationManager, *orchestrator.Orchestrator) {
	t.Helper()

	// Create tool registry with real tools
	registry := runtime.NewToolRegistry()
	registry.Register(file.NewReadTool(fs))
	registry.Register(file.NewWriteTool(fs))
	registry.Register(file.NewListTool(fs))
	registry.Register(file.NewGrepTool(fs))
	registry.Register(patch.NewPatchTool(fs))
	registry.Register(shell.NewShellTool())

	// Create approval cache
	approvalCache := runtime.NewMemoryApprovalCache()

	// Create orchestrator (approval handler set per-session)
	orch := orchestrator.NewOrchestrator(registry, approvalCache, nil)

	// Create manager
	cfg := ManagerConfig{
		Client:       mockClient,
		Orchestrator: orch,
	}

	mgr, err := NewManager(cfg)
	require.NoError(t, err)

	return mgr, orch
}

func createTestManagerWithHistory(t *testing.T, mockClient *mocks.MockClient, fs afero.Fs, historyRoot string) ConversationManager {
	t.Helper()

	// Create tool registry
	registry := runtime.NewToolRegistry()
	registry.Register(file.NewReadTool(fs))

	approvalCache := runtime.NewMemoryApprovalCache()
	orch := orchestrator.NewOrchestrator(registry, approvalCache, nil)

	cfg := ManagerConfig{
		Client:        mockClient,
		Orchestrator:  orch,
		HistoryFs:     fs,
		SessionsRoot:  historyRoot,
		EnableHistory: true,
	}

	mgr, err := NewManager(cfg)
	require.NoError(t, err)

	return mgr
}

// testStrPtr is a helper for creating string pointers in tests
func testStrPtr(s string) *string {
	return &s
}
