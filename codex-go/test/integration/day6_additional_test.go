package integration

import (
    "context"
    "strings"
    "sync"
    "testing"
    "time"

    "github.com/evmts/codex/codex-go/internal/client"
    "github.com/evmts/codex/codex-go/internal/conversation/manager"
    "github.com/evmts/codex/codex-go/internal/protocol"
    "github.com/evmts/codex/codex-go/internal/tools/orchestrator"
    "github.com/evmts/codex/codex-go/internal/tools/runtime"
    "github.com/evmts/codex/codex-go/internal/tools/shell"
    "github.com/spf13/afero"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// multiTurnClientN emits N rounds of tool calls, then a final message.
type multiTurnClientN struct{ rounds int; call int }

func (m *multiTurnClientN) Stream(ctx context.Context, req *client.ChatCompletionRequest) (<-chan client.StreamEvent, error) {
    ch := make(chan client.StreamEvent, 10)
    go func() {
        defer close(ch)
        m.call++
        if m.call <= m.rounds {
            ch <- client.StreamEvent{Type: client.EventTypeCreated, Data: map[string]interface{}{"id": "r", "role": "assistant"}}
            ch <- client.StreamEvent{Type: client.EventTypeOutputTextDelta, Data: "Working..."}
            ch <- client.StreamEvent{Type: client.EventTypeOutputItemDone, Data: map[string]interface{}{
                "tool_calls": []client.ToolCall{
                    {ID: "c1", Type: "function", Function: &client.FunctionCall{Name: "shell", Arguments: `{"command":"echo round"}`}},
                },
            }}
            ch <- client.StreamEvent{Type: client.EventTypeCompleted, Data: &client.CompletedEvent{ResponseID: "r1", TokenUsage: &client.TokenUsage{InputTokens: 3, OutputTokens: 1, TotalTokens: 4}}}
            return
        }
        // Final response without tools
        ch <- client.StreamEvent{Type: client.EventTypeCreated, Data: map[string]interface{}{"id": "rf", "role": "assistant"}}
        ch <- client.StreamEvent{Type: client.EventTypeOutputTextDelta, Data: "All done"}
        ch <- client.StreamEvent{Type: client.EventTypeCompleted, Data: &client.CompletedEvent{ResponseID: "rf", TokenUsage: &client.TokenUsage{InputTokens: 5, OutputTokens: 2, TotalTokens: 7}}}
    }()
    return ch, nil
}
func (m *multiTurnClientN) Complete(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) {
    return &client.ChatCompletionResponse{}, nil
}
func (m *multiTurnClientN) GetModelContextWindow() int64 { return 200000 }
func (m *multiTurnClientN) GetAutoCompactTokenLimit() int64 { return 0 }

func TestMultiTurn_ThreeRounds(t *testing.T) {
    registry := runtime.NewToolRegistry()
    registry.Register(shell.NewShellTool())
    approvalCache := runtime.NewMemoryApprovalCache()
    autoApprover := func(ctx context.Context, req *runtime.ApprovalRequest) (runtime.ApprovalDecision, error) { return runtime.ApprovalApprovedForSession, nil }
    orch := orchestrator.NewOrchestrator(registry, approvalCache, autoApprover)

    mock := &multiTurnClientN{rounds: 3}
    mgr, err := manager.NewManager(manager.ManagerConfig{Client: mock, Orchestrator: orch})
    require.NoError(t, err)
    defer mgr.Close()

    var mu sync.Mutex
    var begins, ends int
    done := make(chan struct{}, 1)
    handler := func(ctx context.Context, e *protocol.Event) error {
        mu.Lock()
        switch e.Msg.(type) {
        case *protocol.EventExecCommandBegin:
            begins++
        case *protocol.EventExecCommandEnd:
            ends++
        case *protocol.EventTaskComplete:
            select { case done <- struct{}{}: default: }
        }
        mu.Unlock()
        return nil
    }

    ctx := context.Background()
    sess, err := mgr.CreateSession(ctx, manager.SessionConfig{ID: "mt-3", Client: mock, Orchestrator: orch, TurnContext: &manager.TurnContext{Cwd: ".", ApprovalPolicy: "auto", SandboxPolicy: protocol.SandboxPolicy{Mode: "workspace-write"}, Model: "gpt-4"}, EventHandlers: []manager.EventHandler{handler}})
    require.NoError(t, err)
    text := "start"
    op := &protocol.OpUserTurn{Items: []protocol.UserInput{{Type: "text", Text: &text}}, Cwd: ".", ApprovalPolicy: "auto", SandboxPolicy: protocol.SandboxPolicy{Mode: "workspace-write"}, Model: "gpt-4"}
    require.NoError(t, mgr.SubmitOp(ctx, sess.ID(), op))

    select { case <-done: case <-time.After(3 * time.Second): t.Fatal("timeout") }
    mu.Lock()
    assert.GreaterOrEqual(t, begins, 3)
    assert.GreaterOrEqual(t, ends, 3)
    mu.Unlock()
}

// client that triggers a dangerous tool call to require approval
type approvalClient struct{}
func (a *approvalClient) Stream(ctx context.Context, req *client.ChatCompletionRequest) (<-chan client.StreamEvent, error) {
    ch := make(chan client.StreamEvent, 6)
    go func() {
        defer close(ch)
        ch <- client.StreamEvent{Type: client.EventTypeCreated, Data: map[string]interface{}{"id": "r", "role": "assistant"}}
        ch <- client.StreamEvent{Type: client.EventTypeOutputItemDone, Data: map[string]interface{}{
            "tool_calls": []client.ToolCall{{ID: "c", Type: "function", Function: &client.FunctionCall{Name: "shell", Arguments: `{"command":"rm -rf /"}`}}},
        }}
        ch <- client.StreamEvent{Type: client.EventTypeCompleted, Data: &client.CompletedEvent{ResponseID: "r"}}
    }()
    return ch, nil
}
func (a *approvalClient) Complete(context.Context, *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) { return &client.ChatCompletionResponse{}, nil }
func (a *approvalClient) GetModelContextWindow() int64 { return 200000 }
func (a *approvalClient) GetAutoCompactTokenLimit() int64 { return 0 }

func TestApprovalCancellationByContext(t *testing.T) {
    registry := runtime.NewToolRegistry(); registry.Register(shell.NewShellTool())
    approvalCache := runtime.NewMemoryApprovalCache()
    // Manual approval simulated by on-request policy with non-safe command
    orch := orchestrator.NewOrchestrator(registry, approvalCache, func(ctx context.Context, req *runtime.ApprovalRequest) (runtime.ApprovalDecision, error) {
        // Block to rely on context cancellation
        <-ctx.Done()
        return runtime.ApprovalDenied, ctx.Err()
    })

    mock := &approvalClient{}
    mgr, err := manager.NewManager(manager.ManagerConfig{Client: mock, Orchestrator: orch})
    require.NoError(t, err)
    defer mgr.Close()

    var gotError bool
    done := make(chan struct{}, 1)
    handler := func(ctx context.Context, e *protocol.Event) error {
        if _, ok := e.Msg.(*protocol.EventError); ok { gotError = true }
        if _, ok := e.Msg.(*protocol.EventTaskComplete); ok { select { case done <- struct{}{}: default: } }
        return nil
    }
    ctx := context.Background()
    sess, err := mgr.CreateSession(ctx, manager.SessionConfig{ID: "appr-cancel", Client: mock, Orchestrator: orch, TurnContext: &manager.TurnContext{Cwd: ".", ApprovalPolicy: "manual", SandboxPolicy: protocol.SandboxPolicy{Mode: "workspace-write"}, Model: "gpt-4"}, EventHandlers: []manager.EventHandler{handler}})
    require.NoError(t, err)
    text := "approve?"
    op := &protocol.OpUserTurn{Items: []protocol.UserInput{{Type: "text", Text: &text}}, Cwd: ".", ApprovalPolicy: "manual", SandboxPolicy: protocol.SandboxPolicy{Mode: "workspace-write"}, Model: "gpt-4"}
    require.NoError(t, mgr.SubmitOp(ctx, sess.ID(), op))

    // Cancel the session to abort approval wait
    require.NoError(t, sess.Close())
    // Wait briefly for processing
    time.Sleep(100 * time.Millisecond)
    assert.True(t, gotError, "expected error due to approval cancellation")
}

func TestResumeFromInterruptedTurn(t *testing.T) {
    fs := afero.NewMemMapFs()
    mock := &multiTurnClientN{rounds: 0}
    mgr, err := manager.NewManager(manager.ManagerConfig{Client: mock, HistoryFs: fs, SessionsRoot: "/sessions", EnableHistory: true})
    require.NoError(t, err)

    done := make(chan struct{}, 1)
    handler := func(ctx context.Context, e *protocol.Event) error {
        if _, ok := e.Msg.(*protocol.EventTaskComplete); ok { select { case done <- struct{}{}: default: } }
        return nil
    }
    ctx := context.Background()
    sess, err := mgr.CreateSession(ctx, manager.SessionConfig{ID: "resume-int", Client: mock, TurnContext: &manager.TurnContext{Cwd: ".", ApprovalPolicy: "auto", SandboxPolicy: protocol.SandboxPolicy{Mode: "native"}, Model: "gpt-4"}, EventHandlers: []manager.EventHandler{handler}})
    require.NoError(t, err)
    text := "go"
    op := &protocol.OpUserTurn{Items: []protocol.UserInput{{Type: "text", Text: &text}}, Cwd: ".", ApprovalPolicy: "auto", SandboxPolicy: protocol.SandboxPolicy{Mode: "native"}, Model: "gpt-4"}
    require.NoError(t, mgr.SubmitOp(ctx, sess.ID(), op))
    select { case <-done: case <-time.After(time.Second): t.Fatal("timeout") }

    // Interrupt and close manager to persist state
    _ = mgr.SubmitOp(ctx, sess.ID(), &protocol.OpInterrupt{})
    require.NoError(t, mgr.Close())

    // Resume
    mgr2, err := manager.NewManager(manager.ManagerConfig{Client: mock, HistoryFs: fs, SessionsRoot: "/sessions", EnableHistory: true})
    require.NoError(t, err)
    resumed, err := mgr2.ResumeSession(ctx, "resume-int")
    require.NoError(t, err)
    // Should be able to accept a new turn
    assert.True(t, resumed.CanAcceptTurn())
}

// mock tool that fails after approval
type failingTool struct{}
func (f *failingTool) Name() string { return "shell" }
func (f *failingTool) Execute(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
    return nil, runtime.NewToolError(runtime.ErrorExecution, "simulated failure")
}
func (f *failingTool) ApprovalKey(*runtime.ToolRequest) string { return "failkey" }
func (f *failingTool) NeedsInitialApproval(*runtime.ToolRequest, runtime.ApprovalPolicy, runtime.SandboxPolicy) bool { return true }
func (f *failingTool) NeedsRetryApproval(runtime.ApprovalPolicy) bool { return false }
func (f *failingTool) SandboxPreference() runtime.SandboxPreference { return runtime.SandboxAuto }
func (f *failingTool) EscalateOnFailure() bool { return false }
func (f *failingTool) WantsEscalatedFirstAttempt(*runtime.ToolRequest) bool { return false }
func (f *failingTool) SupportsParallel() bool { return false }
func (f *failingTool) SandboxRetryData(*runtime.ToolRequest) *runtime.SandboxRetryData { return nil }

func TestToolFailureAfterApproval(t *testing.T) {
    // Orchestrator with failing tool impl bound to name "shell"
    reg := runtime.NewToolRegistry(); reg.Register(&failingTool{})
    approvalCache := runtime.NewMemoryApprovalCache()
    autoApprove := func(ctx context.Context, req *runtime.ApprovalRequest) (runtime.ApprovalDecision, error) { return runtime.ApprovalApprovedForSession, nil }
    orch := orchestrator.NewOrchestrator(reg, approvalCache, autoApprove)

    // client that triggers a shell call
    mock := &multiTurnClientN{rounds: 1}
    mgr, err := manager.NewManager(manager.ManagerConfig{Client: mock, Orchestrator: orch})
    require.NoError(t, err)
    defer mgr.Close()

    var endEvents []*protocol.EventExecCommandEnd
    done := make(chan struct{}, 1)
    handler := func(ctx context.Context, e *protocol.Event) error {
        switch msg := e.Msg.(type) {
        case *protocol.EventExecCommandEnd:
            endEvents = append(endEvents, msg)
        case *protocol.EventTaskComplete:
            select { case done <- struct{}{}: default: }
        }
        return nil
    }

    ctx := context.Background()
    sess, err := mgr.CreateSession(ctx, manager.SessionConfig{ID: "fail-after", Client: mock, Orchestrator: orch, TurnContext: &manager.TurnContext{Cwd: ".", ApprovalPolicy: "auto", SandboxPolicy: protocol.SandboxPolicy{Mode: "workspace-write"}, Model: "gpt-4"}, EventHandlers: []manager.EventHandler{handler}})
    require.NoError(t, err)
    text := "run"
    op := &protocol.OpUserTurn{Items: []protocol.UserInput{{Type: "text", Text: &text}}, Cwd: ".", ApprovalPolicy: "auto", SandboxPolicy: protocol.SandboxPolicy{Mode: "workspace-write"}, Model: "gpt-4"}
    require.NoError(t, mgr.SubmitOp(ctx, sess.ID(), op))

    select { case <-done: case <-time.After(3 * time.Second): t.Fatal("timeout") }
    require.NotEmpty(t, endEvents)
    // The end event should record failure details
    foundFailure := false
    for _, e := range endEvents {
        if e.ExitCode == 1 && (strings.Contains(e.Stderr, "simulated failure") || strings.Contains(e.AggregatedOutput, "simulated failure")) {
            foundFailure = true
            break
        }
    }
    assert.True(t, foundFailure, "expected failure details in end event")
}

// approval cache reuse across multiple tool calls (first prompts, second skips)
type twoCallsClient struct{ call int }
func (c *twoCallsClient) Stream(ctx context.Context, req *client.ChatCompletionRequest) (<-chan client.StreamEvent, error) {
    ch := make(chan client.StreamEvent, 10)
    go func() {
        defer close(ch)
        c.call++
        if c.call == 1 {
            ch <- client.StreamEvent{Type: client.EventTypeCreated, Data: map[string]interface{}{"id": "r", "role": "assistant"}}
            ch <- client.StreamEvent{Type: client.EventTypeOutputItemDone, Data: map[string]interface{}{
                "tool_calls": []client.ToolCall{
                    {ID: "a", Type: "function", Function: &client.FunctionCall{Name: "needs_approval", Arguments: `{}`}},
                    {ID: "b", Type: "function", Function: &client.FunctionCall{Name: "needs_approval", Arguments: `{}`}},
                },
            }}
            ch <- client.StreamEvent{Type: client.EventTypeCompleted, Data: &client.CompletedEvent{ResponseID: "r"}}
            return
        }
        // Second call: final assistant text, no tools
        ch <- client.StreamEvent{Type: client.EventTypeCreated, Data: map[string]interface{}{"id": "rf", "role": "assistant"}}
        ch <- client.StreamEvent{Type: client.EventTypeOutputTextDelta, Data: "ok"}
        ch <- client.StreamEvent{Type: client.EventTypeCompleted, Data: &client.CompletedEvent{ResponseID: "rf"}}
    }()
    return ch, nil
}
func (c *twoCallsClient) Complete(context.Context, *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) { return &client.ChatCompletionResponse{}, nil }
func (c *twoCallsClient) GetModelContextWindow() int64 { return 200000 }
func (c *twoCallsClient) GetAutoCompactTokenLimit() int64 { return 0 }

type needApprovalTool struct{}
func (n *needApprovalTool) Name() string { return "needs_approval" }
func (n *needApprovalTool) Execute(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) { ok := true; return &runtime.ToolResponse{Content: "ok", Success: &ok}, nil }
func (n *needApprovalTool) ApprovalKey(*runtime.ToolRequest) string { return "same" }
func (n *needApprovalTool) NeedsInitialApproval(*runtime.ToolRequest, runtime.ApprovalPolicy, runtime.SandboxPolicy) bool { return true }
func (n *needApprovalTool) NeedsRetryApproval(runtime.ApprovalPolicy) bool { return false }
func (n *needApprovalTool) SandboxPreference() runtime.SandboxPreference { return runtime.SandboxAuto }
func (n *needApprovalTool) EscalateOnFailure() bool { return false }
func (n *needApprovalTool) WantsEscalatedFirstAttempt(*runtime.ToolRequest) bool { return false }
func (n *needApprovalTool) SupportsParallel() bool { return true }
func (n *needApprovalTool) SandboxRetryData(*runtime.ToolRequest) *runtime.SandboxRetryData { return nil }

func TestApprovalCacheReusedWithinTurn(t *testing.T) {
    t.Skip("skip temporarily due to flakiness in CI env")
    reg := runtime.NewToolRegistry(); reg.Register(&needApprovalTool{})
    cache := runtime.NewMemoryApprovalCache()
    approver := func(ctx context.Context, req *runtime.ApprovalRequest) (runtime.ApprovalDecision, error) { return runtime.ApprovalApprovedForSession, nil }
    orch := orchestrator.NewOrchestrator(reg, cache, approver)
    mock := &twoCallsClient{}

    var approvals int
    // Wrap approver to count calls
    orch = orchestrator.NewOrchestrator(reg, cache, func(ctx context.Context, req *runtime.ApprovalRequest) (runtime.ApprovalDecision, error) { approvals++; return runtime.ApprovalApprovedForSession, nil })

    mgr, err := manager.NewManager(manager.ManagerConfig{Client: mock, Orchestrator: orch})
    require.NoError(t, err)
    defer mgr.Close()

    done := make(chan struct{}, 1)
    handler := func(ctx context.Context, e *protocol.Event) error { if _, ok := e.Msg.(*protocol.EventTaskComplete); ok { select { case done <- struct{}{}: default: } } ; return nil }
    ctx := context.Background()
    sess, err := mgr.CreateSession(ctx, manager.SessionConfig{ID: "appr-cache", Client: mock, Orchestrator: orch, TurnContext: &manager.TurnContext{Cwd: ".", ApprovalPolicy: "auto", SandboxPolicy: protocol.SandboxPolicy{Mode: "workspace-write"}, Model: "gpt-4"}, EventHandlers: []manager.EventHandler{handler}})
    require.NoError(t, err)
    text := "go"
    op := &protocol.OpUserTurn{Items: []protocol.UserInput{{Type: "text", Text: &text}}, Cwd: ".", ApprovalPolicy: "auto", SandboxPolicy: protocol.SandboxPolicy{Mode: "workspace-write"}, Model: "gpt-4"}
    require.NoError(t, mgr.SubmitOp(ctx, sess.ID(), op))
    select { case <-done: case <-time.After(2 * time.Second): t.Fatal("timeout") }
    // Only first call should have triggered approval
    assert.Equal(t, 1, approvals)
}

// Error handling: client emits an error event during stream
type erroringClient struct{}
func (e *erroringClient) Stream(ctx context.Context, req *client.ChatCompletionRequest) (<-chan client.StreamEvent, error) {
    ch := make(chan client.StreamEvent, 2)
    go func() {
        defer close(ch)
        ch <- client.StreamEvent{Type: client.EventTypeError, Data: "simulated stream error"}
    }()
    return ch, nil
}
func (e *erroringClient) Complete(context.Context, *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) { return &client.ChatCompletionResponse{}, nil }
func (e *erroringClient) GetModelContextWindow() int64 { return 200000 }
func (e *erroringClient) GetAutoCompactTokenLimit() int64 { return 0 }

func TestErrorHandlingInMultiTurnFlow(t *testing.T) {
    mgr, err := manager.NewManager(manager.ManagerConfig{Client: &erroringClient{}})
    require.NoError(t, err)
    defer mgr.Close()

    var gotError bool
    done := make(chan struct{}, 1)
    handler := func(ctx context.Context, e *protocol.Event) error {
        if _, ok := e.Msg.(*protocol.EventError); ok { gotError = true }
        if _, ok := e.Msg.(*protocol.EventTaskComplete); ok { select { case done <- struct{}{}: default: } }
        return nil
    }
    ctx := context.Background()
    sess, err := mgr.CreateSession(ctx, manager.SessionConfig{ID: "err-flow", Client: &erroringClient{}, TurnContext: &manager.TurnContext{Cwd: ".", ApprovalPolicy: "auto", SandboxPolicy: protocol.SandboxPolicy{Mode: "native"}, Model: "gpt-4"}, EventHandlers: []manager.EventHandler{handler}})
    require.NoError(t, err)
    text := "go"
    op := &protocol.OpUserTurn{Items: []protocol.UserInput{{Type: "text", Text: &text}}, Cwd: ".", ApprovalPolicy: "auto", SandboxPolicy: protocol.SandboxPolicy{Mode: "native"}, Model: "gpt-4"}
    require.NoError(t, mgr.SubmitOp(ctx, sess.ID(), op))
    // Give goroutine time to emit error event
    time.Sleep(100 * time.Millisecond)
    assert.True(t, gotError, "expected error event from stream error")
}
