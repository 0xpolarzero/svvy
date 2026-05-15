package manager

import (
    "context"
    "testing"
    "time"

    "github.com/evmts/codex/codex-go/internal/client/mocks"
    "github.com/evmts/codex/codex-go/internal/protocol"
    "github.com/evmts/codex/codex-go/internal/tools/runtime"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.uber.org/mock/gomock"
)

func TestSessionApprovalHandler_AutoPolicySkipsApproval(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockClient := mocks.NewMockClient(ctrl)

    // Create session in processing state via SubmitTurn
    sess := createTestSession(t, mockClient)
    op := &protocol.OpUserTurn{
        Items:          []protocol.UserInput{{Type: "text", Text: strPtr("hello")}},
        Cwd:            ".",
        ApprovalPolicy: "auto",
        SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
        Model:          "gpt-4",
    }
    submissionID, err := sess.SubmitTurn(context.Background(), op)
    require.NoError(t, err)

    // Collect events
    var gotApprovalEvent bool
    handler := func(ctx context.Context, e *protocol.Event) error {
        if _, ok := e.Msg.(*protocol.EventToolCallApprovalNeeded); ok {
            gotApprovalEvent = true
        }
        return nil
    }
    sess.eventHandlers = append(sess.eventHandlers, handler)

    // Create approval handler
    sah := NewSessionApprovalHandler(sess, submissionID)

    // Invoke HandleApproval with any request; expect immediate approval without events
    req := &runtime.ApprovalRequest{CallID: "call-1", ToolName: "shell", Command: []string{"echo", "hi"}}
    decision, err := sah.HandleApproval(context.Background(), req)
    require.NoError(t, err)
    assert.Equal(t, runtime.ApprovalApprovedForSession, decision)
    assert.False(t, gotApprovalEvent, "auto policy should not emit approval needed event")
    // State remains processing
    assert.Equal(t, StateProcessingTurn, sess.State())
}

func TestSessionApprovalHandler_ManualApprovalFlow(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockClient := mocks.NewMockClient(ctrl)

    sess := createTestSession(t, mockClient)
    op := &protocol.OpUserTurn{
        Items:          []protocol.UserInput{{Type: "text", Text: strPtr("hello")}},
        Cwd:            ".",
        ApprovalPolicy: "manual",
        SandboxPolicy:  protocol.SandboxPolicy{Mode: "workspace-write"},
        Model:          "gpt-4",
    }
    submissionID, err := sess.SubmitTurn(context.Background(), op)
    require.NoError(t, err)

    // Capture approval-needed event
    approvalCh := make(chan *protocol.EventToolCallApprovalNeeded, 1)
    handler := func(ctx context.Context, e *protocol.Event) error {
        if msg, ok := e.Msg.(*protocol.EventToolCallApprovalNeeded); ok {
            select { case approvalCh <- msg: default: }
        }
        return nil
    }
    sess.eventHandlers = append(sess.eventHandlers, handler)

    sah := NewSessionApprovalHandler(sess, submissionID)

    // Run HandleApproval in goroutine (it should block until decision)
    ctx := context.Background()
    req := &runtime.ApprovalRequest{CallID: "call-2", ToolName: "shell", Command: []string{"sh", "-c", "echo hi"}, WorkingDirectory: "."}
    done := make(chan struct{})
    var decision runtime.ApprovalDecision
    var herr error
    go func() {
        decision, herr = sah.HandleApproval(ctx, req)
        close(done)
    }()

    // Expect approval-needed event and state awaiting approval
    select {
    case <-approvalCh:
        assert.Equal(t, StateAwaitingApproval, sess.State())
    case <-time.After(time.Second):
        t.Fatal("timeout waiting for approval-needed event")
    }

    // Send decision
    require.NoError(t, sah.NotifyApprovalDecision("call-2", runtime.ApprovalApproved))

    <-done
    require.NoError(t, herr)
    assert.Equal(t, runtime.ApprovalApproved, decision)
}

func TestSessionApprovalHandler_ContextCancellation(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockClient := mocks.NewMockClient(ctrl)
    sess := createTestSession(t, mockClient)
    op := &protocol.OpUserTurn{
        Items:          []protocol.UserInput{{Type: "text", Text: strPtr("hi")}},
        Cwd:            ".",
        ApprovalPolicy: "manual",
        SandboxPolicy:  protocol.SandboxPolicy{Mode: "workspace-write"},
        Model:          "gpt-4",
    }
    submissionID, err := sess.SubmitTurn(context.Background(), op)
    require.NoError(t, err)

    sah := NewSessionApprovalHandler(sess, submissionID)
    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    req := &runtime.ApprovalRequest{CallID: "call-3", ToolName: "shell"}
    decision, err := sah.HandleApproval(ctx, req)
    require.Error(t, err)
    assert.Equal(t, runtime.ApprovalDenied, decision)
}

func TestSessionApprovalHandler_ConcurrentRequestsError(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockClient := mocks.NewMockClient(ctrl)
    sess := createTestSession(t, mockClient)
    op := &protocol.OpUserTurn{
        Items:          []protocol.UserInput{{Type: "text", Text: strPtr("hi")}},
        Cwd:            ".",
        ApprovalPolicy: "manual",
        SandboxPolicy:  protocol.SandboxPolicy{Mode: "workspace-write"},
        Model:          "gpt-4",
    }
    submissionID, err := sess.SubmitTurn(context.Background(), op)
    require.NoError(t, err)

    sah := NewSessionApprovalHandler(sess, submissionID)

    // First request starts and blocks
    ctx1, cancel1 := context.WithCancel(context.Background())
    defer cancel1()
    req1 := &runtime.ApprovalRequest{CallID: "call-a", ToolName: "shell"}
    started := make(chan struct{})
    go func() {
        close(started)
        _, _ = sah.HandleApproval(ctx1, req1)
    }()
    <-started

    // Second concurrent request should error immediately
    req2 := &runtime.ApprovalRequest{CallID: "call-b", ToolName: "shell"}
    _, err = sah.HandleApproval(context.Background(), req2)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "already pending")
}

func TestSessionApprovalHandler_ApprovalTimeout(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockClient := mocks.NewMockClient(ctrl)
    sess := createTestSession(t, mockClient)

    // Set a short approval timeout for testing (100ms)
    op := &protocol.OpUserTurn{
        Items:          []protocol.UserInput{{Type: "text", Text: strPtr("hi")}},
        Cwd:            ".",
        ApprovalPolicy: "manual",
        SandboxPolicy:  protocol.SandboxPolicy{Mode: "workspace-write"},
        Model:          "gpt-4",
    }
    submissionID, err := sess.SubmitTurn(context.Background(), op)
    require.NoError(t, err)

    // Update turn context with short timeout
    sess.turnContext.ApprovalTimeout = 100 * time.Millisecond

    sah := NewSessionApprovalHandler(sess, submissionID)

    // Run HandleApproval - it should timeout without user response
    req := &runtime.ApprovalRequest{CallID: "call-timeout", ToolName: "shell", Command: []string{"sh", "-c", "rm -rf /"}}
    startTime := time.Now()
    decision, err := sah.HandleApproval(context.Background(), req)
    elapsed := time.Since(startTime)

    // Should have timed out
    require.Error(t, err)
    assert.Contains(t, err.Error(), "timed out")
    assert.Equal(t, runtime.ApprovalDenied, decision)

    // Should have taken approximately the timeout duration (allow some variance)
    assert.Greater(t, elapsed, 80*time.Millisecond, "timeout should be at least 80ms")
    assert.Less(t, elapsed, 200*time.Millisecond, "timeout should be less than 200ms")
}

func TestSessionApprovalHandler_DefaultApprovalTimeout(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockClient := mocks.NewMockClient(ctrl)
    sess := createTestSession(t, mockClient)

    // Use default timeout (0 = 5 minutes) but we'll cancel before that
    op := &protocol.OpUserTurn{
        Items:          []protocol.UserInput{{Type: "text", Text: strPtr("hi")}},
        Cwd:            ".",
        ApprovalPolicy: "manual",
        SandboxPolicy:  protocol.SandboxPolicy{Mode: "workspace-write"},
        Model:          "gpt-4",
    }
    submissionID, err := sess.SubmitTurn(context.Background(), op)
    require.NoError(t, err)

    // Leave ApprovalTimeout as 0 (should default to 5 minutes)
    sess.turnContext.ApprovalTimeout = 0

    sah := NewSessionApprovalHandler(sess, submissionID)

    // Run HandleApproval with a parent context that times out quickly
    ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel()

    req := &runtime.ApprovalRequest{CallID: "call-default", ToolName: "shell"}
    _, err = sah.HandleApproval(ctx, req)

    // Should have been cancelled by parent context, not the 5-minute default
    require.Error(t, err)
    assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestSessionApprovalHandler_NeverPolicyRejectsInvocation(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockClient := mocks.NewMockClient(ctrl)

    // Create session with "never" approval policy
    sess := createTestSession(t, mockClient)
    op := &protocol.OpUserTurn{
        Items:          []protocol.UserInput{{Type: "text", Text: strPtr("hello")}},
        Cwd:            ".",
        ApprovalPolicy: "never",
        SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
        Model:          "gpt-4",
    }
    submissionID, err := sess.SubmitTurn(context.Background(), op)
    require.NoError(t, err)

    // Collect events to ensure no approval-needed event is emitted
    var gotApprovalEvent bool
    handler := func(ctx context.Context, e *protocol.Event) error {
        if _, ok := e.Msg.(*protocol.EventToolCallApprovalNeeded); ok {
            gotApprovalEvent = true
        }
        return nil
    }
    sess.eventHandlers = append(sess.eventHandlers, handler)

    // Create approval handler
    sah := NewSessionApprovalHandler(sess, submissionID)

    // Invoke HandleApproval with any request
    // With "never" policy, the approval handler should NOT have been called at all
    // Since it was called anyway (defensive check), it should return an error
    req := &runtime.ApprovalRequest{CallID: "call-never", ToolName: "shell", Command: []string{"rm", "-rf", "/"}}
    decision, err := sah.HandleApproval(context.Background(), req)

    // Expect error since approval handler shouldn't be invoked with "never" policy
    require.Error(t, err)
    assert.Contains(t, err.Error(), "never")
    assert.Equal(t, runtime.ApprovalDenied, decision)
    assert.False(t, gotApprovalEvent, "never policy should not emit approval needed event")
    // State should remain processing (not transition to awaiting approval)
    assert.Equal(t, StateProcessingTurn, sess.State())
}

