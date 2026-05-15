package manager

import (
    "context"
    "testing"
    "time"

    "github.com/evmts/codex/codex-go/internal/client"
    "github.com/evmts/codex/codex-go/internal/protocol"
    "github.com/evmts/codex/codex-go/internal/tools/orchestrator"
    "github.com/evmts/codex/codex-go/internal/tools/runtime"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// mockClientAlwaysToolCall returns a tool call every stream to drive multi-turn.
type mockClientAlwaysToolCall struct{}

func (m *mockClientAlwaysToolCall) Stream(ctx context.Context, req *client.ChatCompletionRequest) (<-chan client.StreamEvent, error) {
    ch := make(chan client.StreamEvent, 4)
    go func() {
        defer close(ch)
        ch <- client.StreamEvent{Type: client.EventTypeCreated, Data: map[string]interface{}{"id": "r", "role": "assistant"}}
        ch <- client.StreamEvent{Type: client.EventTypeOutputItemDone, Data: map[string]interface{}{
            "tool_calls": []client.ToolCall{
                {ID: "c", Type: "function", Function: &client.FunctionCall{Name: "shell", Arguments: `{"command":"echo ok"}`}},
            },
        }}
        ch <- client.StreamEvent{Type: client.EventTypeCompleted, Data: &client.CompletedEvent{ResponseID: "r", TokenUsage: &client.TokenUsage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}}}
    }()
    return ch, nil
}
func (m *mockClientAlwaysToolCall) Complete(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) {
    return &client.ChatCompletionResponse{}, nil
}
func (m *mockClientAlwaysToolCall) GetModelContextWindow() int64 { return 200000 }
func (m *mockClientAlwaysToolCall) GetAutoCompactTokenLimit() int64 { return 0 }

// mockSimpleStreamingClient emits a single completion with usage and no tool calls.
type mockSimpleStreamingClient struct{ usage client.TokenUsage }

func (m *mockSimpleStreamingClient) Stream(ctx context.Context, req *client.ChatCompletionRequest) (<-chan client.StreamEvent, error) {
    ch := make(chan client.StreamEvent, 2)
    go func() {
        defer close(ch)
        ch <- client.StreamEvent{Type: client.EventTypeOutputTextDelta, Data: "hello"}
        ch <- client.StreamEvent{Type: client.EventTypeCompleted, Data: &client.CompletedEvent{ResponseID: "r1", TokenUsage: &m.usage}}
    }()
    return ch, nil
}
func (m *mockSimpleStreamingClient) Complete(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) {
    return &client.ChatCompletionResponse{}, nil
}
func (m *mockSimpleStreamingClient) GetModelContextWindow() int64 { return 200000 }
func (m *mockSimpleStreamingClient) GetAutoCompactTokenLimit() int64 { return 0 }

// mockToolOK returns a successful response without executing anything.
type mockToolOK struct{}

func (m *mockToolOK) Name() string { return "shell" }
func (m *mockToolOK) Execute(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
    ok := true
    return &runtime.ToolResponse{Content: "ok", Success: &ok}, nil
}
func (m *mockToolOK) ApprovalKey(req *runtime.ToolRequest) string { return "k" }
func (m *mockToolOK) NeedsInitialApproval(*runtime.ToolRequest, runtime.ApprovalPolicy, runtime.SandboxPolicy) bool {
    return false
}
func (m *mockToolOK) NeedsRetryApproval(runtime.ApprovalPolicy) bool { return false }
func (m *mockToolOK) SandboxPreference() runtime.SandboxPreference { return runtime.SandboxAuto }
func (m *mockToolOK) EscalateOnFailure() bool { return false }
func (m *mockToolOK) WantsEscalatedFirstAttempt(*runtime.ToolRequest) bool { return false }
func (m *mockToolOK) SupportsParallel() bool { return true }
func (m *mockToolOK) SandboxRetryData(*runtime.ToolRequest) *runtime.SandboxRetryData { return &runtime.SandboxRetryData{Command: []string{"sh", "-c", "echo ok"}} }

func TestTurnProcessor_MaxTurnLimit(t *testing.T) {
    // Orchestrator with mock tool
    reg := runtime.NewToolRegistry()
    reg.Register(&mockToolOK{})
    orch := orchestrator.NewOrchestrator(reg, runtime.NewMemoryApprovalCache(), func(ctx context.Context, req *runtime.ApprovalRequest) (runtime.ApprovalDecision, error) {
        return runtime.ApprovalApprovedForSession, nil
    })

    // Session with mock client
    sess, err := NewSession(SessionConfig{ID: "s-max", Client: &mockClientAlwaysToolCall{}, TurnContext: &TurnContext{Cwd: ".", ApprovalPolicy: "auto", SandboxPolicy: protocol.SandboxPolicy{Mode: "workspace-write"}, Model: "gpt-4"}, Orchestrator: orch})
    require.NoError(t, err)

    // Move to processing state
    ctx := context.Background()
    text := "go"
    op := &protocol.OpUserTurn{Items: []protocol.UserInput{{Type: "text", Text: &text}}, Cwd: ".", ApprovalPolicy: "auto", SandboxPolicy: protocol.SandboxPolicy{Mode: "workspace-write"}, Model: "gpt-4"}
    submissionID, err := sess.SubmitTurn(ctx, op)
    require.NoError(t, err)

    // Process the turn directly
    tp := NewTurnProcessor(sess)
    err = tp.ProcessTurn(ctx, submissionID, op)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "maximum multi-turn iterations")
}

func TestTurnProcessor_TokenUsageAccumulation(t *testing.T) {
    sess, err := NewSession(SessionConfig{ID: "s-usage", Client: &mockSimpleStreamingClient{usage: client.TokenUsage{InputTokens: 11, OutputTokens: 7, TotalTokens: 18}}, TurnContext: &TurnContext{Cwd: ".", ApprovalPolicy: "auto", SandboxPolicy: protocol.SandboxPolicy{Mode: "native"}, Model: "gpt-4"}})
    require.NoError(t, err)

    ctx := context.Background()
    text := "hello"
    op := &protocol.OpUserTurn{Items: []protocol.UserInput{{Type: "text", Text: &text}}, Cwd: ".", ApprovalPolicy: "auto", SandboxPolicy: protocol.SandboxPolicy{Mode: "native"}, Model: "gpt-4"}
    submissionID, err := sess.SubmitTurn(ctx, op)
    require.NoError(t, err)

    tp := NewTurnProcessor(sess)
    require.NoError(t, tp.ProcessTurn(ctx, submissionID, op))

    // Allow async event handling to update token usage
    time.Sleep(10 * time.Millisecond)
    tu := sess.GetTokenUsage()
    assert.Equal(t, int64(11), tu.InputTokens)
    assert.Equal(t, int64(7), tu.OutputTokens)
    assert.Equal(t, int64(18), tu.TotalTokens)
}

func TestTurnProcessor_ConfigurableMaxTurnLimit(t *testing.T) {
    // Orchestrator with mock tool
    reg := runtime.NewToolRegistry()
    reg.Register(&mockToolOK{})
    orch := orchestrator.NewOrchestrator(reg, runtime.NewMemoryApprovalCache(), func(ctx context.Context, req *runtime.ApprovalRequest) (runtime.ApprovalDecision, error) {
        return runtime.ApprovalApprovedForSession, nil
    })

    tests := []struct {
        name      string
        maxTurns  int
        wantError bool
        errorMsg  string
    }{
        {
            name:      "custom limit of 3",
            maxTurns:  3,
            wantError: true,
            errorMsg:  "maximum multi-turn iterations exceeded: 3",
        },
        {
            name:      "custom limit of 5",
            maxTurns:  5,
            wantError: true,
            errorMsg:  "maximum multi-turn iterations exceeded: 5",
        },
        {
            name:      "zero uses default of 10",
            maxTurns:  0,
            wantError: true,
            errorMsg:  "maximum multi-turn iterations exceeded: 10",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Session with custom max turns
            sess, err := NewSession(SessionConfig{
                ID:     "s-config-" + tt.name,
                Client: &mockClientAlwaysToolCall{},
                TurnContext: &TurnContext{
                    Cwd:            ".",
                    ApprovalPolicy: "auto",
                    SandboxPolicy:  protocol.SandboxPolicy{Mode: "workspace-write"},
                    Model:          "gpt-4",
                    MaxTurns:       tt.maxTurns,
                },
                Orchestrator: orch,
            })
            require.NoError(t, err)

            ctx := context.Background()
            text := "go"
            op := &protocol.OpUserTurn{Items: []protocol.UserInput{{Type: "text", Text: &text}}, Cwd: ".", ApprovalPolicy: "auto", SandboxPolicy: protocol.SandboxPolicy{Mode: "workspace-write"}, Model: "gpt-4"}
            submissionID, err := sess.SubmitTurn(ctx, op)
            require.NoError(t, err)

            tp := NewTurnProcessor(sess)
            err = tp.ProcessTurn(ctx, submissionID, op)

            if tt.wantError {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.errorMsg)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
