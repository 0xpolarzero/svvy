package manager

import (
    "testing"

    "github.com/evmts/codex/codex-go/internal/protocol"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestReconstructStateFromHistory_BasicFlow(t *testing.T) {
    // Build a simple history: user turn with tool exec, token count, and completion
    text := "Hello"
    op := &protocol.OpUserTurn{
        Items:          []protocol.UserInput{{Type: "text", Text: &text}},
        Cwd:            ".",
        ApprovalPolicy: "auto",
        SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
        Model:          "gpt-4",
    }
    submissions := []*protocol.Submission{{ID: "s1", Op: op}}

    usage := &protocol.TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15}
    events := []*protocol.Event{
        {ID: "s1", Msg: &protocol.EventTaskStarted{ModelContextWindow: int64Ptr(200000)}},
        {ID: "s1", Msg: &protocol.EventAgentMessageDelta{Delta: "Hi"}},
        {ID: "s1", Msg: &protocol.EventExecCommandBegin{CallID: "c1", Command: []string{"sh", "-c", "echo hi"}, Cwd: "."}},
        {ID: "s1", Msg: &protocol.EventExecCommandEnd{CallID: "c1", AggregatedOutput: "hi", ExitCode: 0}},
        {ID: "s1", Msg: &protocol.EventTokenCount{Info: &protocol.TokenUsageInfo{TotalTokenUsage: *usage}}},
        {ID: "s1", Msg: &protocol.EventTaskComplete{LastAgentMessage: strPtr("done")}},
    }

    state, err := ReconstructStateFromHistory(submissions, events)
    require.NoError(t, err)
    require.NotNil(t, state)

    // Validate reconstructed fields
    // LastAgentMessage uses accumulated deltas in current implementation
    assert.Equal(t, "Hi", state.LastAgentMessage)
    require.NotNil(t, state.TokenUsage)
    assert.Equal(t, usage.InputTokens, state.TokenUsage.InputTokens)
    assert.Equal(t, 1, state.CompletedTurns)
    assert.False(t, state.HasIncompleteTurn)

    // Messages should include at least user, assistant, and tool_result
    roles := make([]string, 0, len(state.Messages))
    for _, m := range state.Messages {
        roles = append(roles, m.Role)
    }
    assert.Contains(t, roles, "user")
    assert.Contains(t, roles, "assistant")
    assert.Contains(t, roles, "tool_result")

    require.NoError(t, ValidateResumedState(state))
}

func TestReconstructStateFromHistory_IncompleteTurn(t *testing.T) {
    text := "Hi"
    op := &protocol.OpUserTurn{Items: []protocol.UserInput{{Type: "text", Text: &text}}, Cwd: "."}
    submissions := []*protocol.Submission{{ID: "s2", Op: op}}
    // No completion event
    events := []*protocol.Event{{ID: "s2", Msg: &protocol.EventAgentMessageDelta{Delta: "Thinking"}}}

    state, err := ReconstructStateFromHistory(submissions, events)
    require.NoError(t, err)
    assert.True(t, state.HasIncompleteTurn)
    require.NoError(t, ValidateResumedState(state))
}

func TestReconstructStateFromHistory_InterruptedTurn(t *testing.T) {
    // User turn then interrupt op
    text := "Hi"
    submissions := []*protocol.Submission{
        {ID: "s3", Op: &protocol.OpUserTurn{Items: []protocol.UserInput{{Type: "text", Text: &text}}}},
        {ID: "s3", Op: &protocol.OpInterrupt{}},
    }
    events := []*protocol.Event{}

    state, err := ReconstructStateFromHistory(submissions, events)
    require.NoError(t, err)
    assert.Equal(t, "s3", state.InterruptedTurnID)
}

func TestValidateResumedState_Errors(t *testing.T) {
    require.Error(t, ValidateResumedState(nil))

    // Missing token usage
    st := &SessionReconstructedState{TurnContext: &TurnContext{Cwd: "."}, TokenUsage: nil}
    require.Error(t, ValidateResumedState(st))

    // Missing turn context
    st = &SessionReconstructedState{TurnContext: nil, TokenUsage: &protocol.TokenUsage{}}
    require.Error(t, ValidateResumedState(st))

    // Empty cwd
    st = &SessionReconstructedState{TurnContext: &TurnContext{Cwd: ""}, TokenUsage: &protocol.TokenUsage{}}
    require.Error(t, ValidateResumedState(st))
}

func int64Ptr(i int64) *int64 { return &i }
