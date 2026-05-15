package manager

import (
	"context"
	"testing"

	"github.com/evmts/codex/codex-go/internal/client"
	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// BenchmarkReconstructStateFromHistory benchmarks state reconstruction from history.
func BenchmarkReconstructStateFromHistory(b *testing.B) {
	// Create sample submissions and events
	submissions := []*protocol.Submission{
		{
			ID: "turn-1",
			Op: &protocol.OpUserTurn{
				Items: []protocol.UserInput{
					{Type: "text", Text: stringPtr("test message 1")},
				},
				Cwd:            ".",
				ApprovalPolicy: "auto",
				Model:          "gpt-4",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
			},
		},
		{
			ID: "turn-2",
			Op: &protocol.OpUserTurn{
				Items: []protocol.UserInput{
					{Type: "text", Text: stringPtr("test message 2")},
				},
				Cwd:            ".",
				ApprovalPolicy: "auto",
				Model:          "gpt-4",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
			},
		},
	}

	events := []*protocol.Event{
		{ID: "turn-1", Msg: &protocol.EventTaskStarted{}},
		{ID: "turn-1", Msg: &protocol.EventAgentMessageDelta{Delta: "Hello "}},
		{ID: "turn-1", Msg: &protocol.EventAgentMessageDelta{Delta: "World"}},
		{ID: "turn-1", Msg: &protocol.EventTokenCount{Info: &protocol.TokenUsageInfo{
			TotalTokenUsage: protocol.TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
		}}},
		{ID: "turn-1", Msg: &protocol.EventTaskComplete{LastAgentMessage: stringPtr("Hello World")}},
		{ID: "turn-2", Msg: &protocol.EventTaskStarted{}},
		{ID: "turn-2", Msg: &protocol.EventAgentMessageDelta{Delta: "Second "}},
		{ID: "turn-2", Msg: &protocol.EventAgentMessageDelta{Delta: "response"}},
		{ID: "turn-2", Msg: &protocol.EventTokenCount{Info: &protocol.TokenUsageInfo{
			TotalTokenUsage: protocol.TokenUsage{InputTokens: 20, OutputTokens: 10, TotalTokens: 30},
		}}},
		{ID: "turn-2", Msg: &protocol.EventTaskComplete{LastAgentMessage: stringPtr("Second response")}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ReconstructStateFromHistory(submissions, events)
	}
}

// BenchmarkMultiTurnHistoryReconstruction benchmarks reconstruction with multiple turns.
func BenchmarkMultiTurnHistoryReconstruction(b *testing.B) {
	// Create sample data for 10 turns
	submissions := make([]*protocol.Submission, 0, 10)
	events := make([]*protocol.Event, 0, 50)

	for i := 0; i < 10; i++ {
		turnID := stringPtr("turn-" + string(rune('0'+i)))
		submissions = append(submissions, &protocol.Submission{
			ID: *turnID,
			Op: &protocol.OpUserTurn{
				Items: []protocol.UserInput{
					{Type: "text", Text: stringPtr("test message")},
				},
				Cwd:            ".",
				ApprovalPolicy: "auto",
				Model:          "gpt-4",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
			},
		})

		events = append(events,
			&protocol.Event{ID: *turnID, Msg: &protocol.EventTaskStarted{}},
			&protocol.Event{ID: *turnID, Msg: &protocol.EventAgentMessageDelta{Delta: "Hello "}},
			&protocol.Event{ID: *turnID, Msg: &protocol.EventAgentMessageDelta{Delta: "World"}},
			&protocol.Event{ID: *turnID, Msg: &protocol.EventTokenCount{Info: &protocol.TokenUsageInfo{
				TotalTokenUsage: protocol.TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
			}}},
			&protocol.Event{ID: *turnID, Msg: &protocol.EventTaskComplete{LastAgentMessage: stringPtr("Hello World")}},
		)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ReconstructStateFromHistory(submissions, events)
	}
}

// BenchmarkConversationCompaction benchmarks conversation history compaction.
func BenchmarkConversationCompaction(b *testing.B) {
	// Create a large conversation history
	messages := make([]client.Message, 1000)
	for i := 0; i < 1000; i++ {
		if i%2 == 0 {
			messages[i] = client.NewUserMessage("test message")
		} else {
			messages[i] = client.Message{Role: "assistant", Content: "test response"}
		}
	}

	// Create mock session and turn processor
	mockClient := &mockClientAlwaysToolCall{}
	sess, _ := NewSession(SessionConfig{
		ID:     "bench-session",
		Client: mockClient,
		TurnContext: &TurnContext{
			Cwd:            ".",
			ApprovalPolicy: "auto",
			SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
			Model:          "gpt-4",
		},
	})

	tp := NewTurnProcessor(sess)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tp.compactConversationIfNeeded(messages)
	}
}

// BenchmarkApprovalHandlerConcurrency benchmarks approval handler under concurrent requests.
func BenchmarkApprovalHandlerConcurrency(b *testing.B) {
	mockClient := &mockClientAlwaysToolCall{}
	sess, _ := NewSession(SessionConfig{
		ID:     "bench-session",
		Client: mockClient,
		TurnContext: &TurnContext{
			Cwd:            ".",
			ApprovalPolicy: "auto", // Auto-approve for fast benchmarking
			SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
			Model:          "gpt-4",
		},
	})

	// Transition to processing state
	_, _ = sess.SubmitTurn(context.Background(), &protocol.OpUserTurn{
		Items:          []protocol.UserInput{{Type: "text", Text: stringPtr("test")}},
		Cwd:            ".",
		ApprovalPolicy: "auto",
		SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
		Model:          "gpt-4",
	})

	sah := NewSessionApprovalHandler(sess, "bench-sub")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := &runtime.ApprovalRequest{
				CallID:   "bench-call",
				ToolName: "shell",
				Command:  []string{"echo", "test"},
			}
			_, _ = sah.HandleApproval(context.Background(), req)
		}
	})
}

// BenchmarkTokenUsageUpdate benchmarks token usage updates during streaming.
func BenchmarkTokenUsageUpdate(b *testing.B) {
	mockClient := &mockSimpleStreamingClient{
		usage: client.TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
	}
	sess, _ := NewSession(SessionConfig{
		ID:     "bench-session",
		Client: mockClient,
		TurnContext: &TurnContext{
			Cwd:            ".",
			ApprovalPolicy: "auto",
			SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
			Model:          "gpt-4",
		},
	})

	usage := &protocol.TokenUsage{
		InputTokens:  10,
		OutputTokens: 5,
		TotalTokens:  15,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sess.UpdateTokenUsage(usage)
	}
}

func stringPtr(s string) *string {
	return &s
}
