package state

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConversationState(t *testing.T) {
	t.Run("creates state with empty history", func(t *testing.T) {
		state := NewConversationState()

		require.NotNil(t, state)
		assert.Empty(t, state.Messages())
		assert.Empty(t, state.ToolCalls())
		assert.NotNil(t, state.CreatedAt)
		assert.NotNil(t, state.UpdatedAt)
	})

	t.Run("is thread-safe", func(t *testing.T) {
		state := NewConversationState()

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = state.Messages()
			}()
		}
		wg.Wait()
	})
}

func TestConversationState_AddMessage(t *testing.T) {
	t.Run("adds user message", func(t *testing.T) {
		state := NewConversationState()

		msg := Message{
			Role:      "user",
			Content:   "Hello",
			Timestamp: time.Now(),
		}

		err := state.AddMessage(msg)
		require.NoError(t, err)

		messages := state.Messages()
		require.Len(t, messages, 1)
		assert.Equal(t, "user", messages[0].Role)
		assert.Equal(t, "Hello", messages[0].Content)
	})

	t.Run("adds assistant message", func(t *testing.T) {
		state := NewConversationState()

		msg := Message{
			Role:      "assistant",
			Content:   "Hi there!",
			Timestamp: time.Now(),
		}

		err := state.AddMessage(msg)
		require.NoError(t, err)

		messages := state.Messages()
		require.Len(t, messages, 1)
		assert.Equal(t, "assistant", messages[0].Role)
	})

	t.Run("maintains message order", func(t *testing.T) {
		state := NewConversationState()

		for i := 0; i < 5; i++ {
			msg := Message{
				Role:      "user",
				Content:   string(rune('A' + i)),
				Timestamp: time.Now(),
			}
			err := state.AddMessage(msg)
			require.NoError(t, err)
		}

		messages := state.Messages()
		require.Len(t, messages, 5)
		for i := 0; i < 5; i++ {
			assert.Equal(t, string(rune('A'+i)), messages[i].Content)
		}
	})

	t.Run("validates message role", func(t *testing.T) {
		state := NewConversationState()

		msg := Message{
			Role:      "invalid",
			Content:   "test",
			Timestamp: time.Now(),
		}

		err := state.AddMessage(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid role")
	})

	t.Run("validates empty content", func(t *testing.T) {
		state := NewConversationState()

		msg := Message{
			Role:      "user",
			Content:   "",
			Timestamp: time.Now(),
		}

		err := state.AddMessage(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty content")
	})

	t.Run("accepts tool role", func(t *testing.T) {
		state := NewConversationState()

		msg := Message{
			Role:      "tool",
			Content:   "Tool execution result",
			Timestamp: time.Now(),
		}

		err := state.AddMessage(msg)
		require.NoError(t, err)

		messages := state.Messages()
		require.Len(t, messages, 1)
		assert.Equal(t, "tool", messages[0].Role)
		assert.Equal(t, "Tool execution result", messages[0].Content)
	})
}

func TestConversationState_AddToolCall(t *testing.T) {
	t.Run("adds pending tool call", func(t *testing.T) {
		state := NewConversationState()

		toolCall := ToolCall{
			ID:        "call_1",
			Name:      "read_file",
			Arguments: map[string]interface{}{"path": "/test.txt"},
			Status:    ToolCallPending,
		}

		err := state.AddToolCall(toolCall)
		require.NoError(t, err)

		calls := state.ToolCalls()
		require.Len(t, calls, 1)
		assert.Equal(t, "call_1", calls[0].ID)
		assert.Equal(t, ToolCallPending, calls[0].Status)
	})

	t.Run("validates tool call ID", func(t *testing.T) {
		state := NewConversationState()

		toolCall := ToolCall{
			ID:        "",
			Name:      "read_file",
			Arguments: map[string]interface{}{},
			Status:    ToolCallPending,
		}

		err := state.AddToolCall(toolCall)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty ID")
	})

	t.Run("validates tool call name", func(t *testing.T) {
		state := NewConversationState()

		toolCall := ToolCall{
			ID:        "call_1",
			Name:      "",
			Arguments: map[string]interface{}{},
			Status:    ToolCallPending,
		}

		err := state.AddToolCall(toolCall)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty name")
	})
}

func TestConversationState_UpdateToolCallStatus(t *testing.T) {
	t.Run("updates from pending to approved", func(t *testing.T) {
		state := NewConversationState()

		toolCall := ToolCall{
			ID:        "call_1",
			Name:      "read_file",
			Arguments: map[string]interface{}{},
			Status:    ToolCallPending,
		}

		err := state.AddToolCall(toolCall)
		require.NoError(t, err)

		err = state.UpdateToolCallStatus("call_1", ToolCallApproved)
		require.NoError(t, err)

		calls := state.ToolCalls()
		require.Len(t, calls, 1)
		assert.Equal(t, ToolCallApproved, calls[0].Status)
	})

	t.Run("updates from approved to executed", func(t *testing.T) {
		state := NewConversationState()

		toolCall := ToolCall{
			ID:        "call_1",
			Name:      "read_file",
			Arguments: map[string]interface{}{},
			Status:    ToolCallApproved,
		}

		err := state.AddToolCall(toolCall)
		require.NoError(t, err)

		err = state.UpdateToolCallStatus("call_1", ToolCallExecuted)
		require.NoError(t, err)

		calls := state.ToolCalls()
		assert.Equal(t, ToolCallExecuted, calls[0].Status)
	})

	t.Run("returns error for non-existent tool call", func(t *testing.T) {
		state := NewConversationState()

		err := state.UpdateToolCallStatus("nonexistent", ToolCallApproved)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("validates status transition", func(t *testing.T) {
		state := NewConversationState()

		toolCall := ToolCall{
			ID:        "call_1",
			Name:      "read_file",
			Arguments: map[string]interface{}{},
			Status:    ToolCallExecuted,
		}

		err := state.AddToolCall(toolCall)
		require.NoError(t, err)

		// Can't go from executed back to pending
		err = state.UpdateToolCallStatus("call_1", ToolCallPending)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status transition")
	})
}

func TestConversationState_TokenUsage(t *testing.T) {
	t.Run("tracks token usage per turn", func(t *testing.T) {
		state := NewConversationState()

		usage := TokenUsage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		}

		state.AddTokenUsage(usage)

		total := state.TotalTokenUsage()
		assert.Equal(t, int64(100), total.InputTokens)
		assert.Equal(t, int64(50), total.OutputTokens)
		assert.Equal(t, int64(150), total.TotalTokens)
	})

	t.Run("accumulates token usage across turns", func(t *testing.T) {
		state := NewConversationState()

		usage1 := TokenUsage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		}
		state.AddTokenUsage(usage1)

		usage2 := TokenUsage{
			InputTokens:  200,
			OutputTokens: 100,
			TotalTokens:  300,
		}
		state.AddTokenUsage(usage2)

		total := state.TotalTokenUsage()
		assert.Equal(t, int64(300), total.InputTokens)
		assert.Equal(t, int64(150), total.OutputTokens)
		assert.Equal(t, int64(450), total.TotalTokens)
	})
}

func TestConversationState_Snapshot(t *testing.T) {
	t.Run("creates immutable snapshot", func(t *testing.T) {
		state := NewConversationState()

		msg := Message{
			Role:      "user",
			Content:   "Hello",
			Timestamp: time.Now(),
		}
		err := state.AddMessage(msg)
		require.NoError(t, err)

		snapshot := state.Snapshot()
		require.NotNil(t, snapshot)
		assert.Len(t, snapshot.Messages, 1)

		// Add another message
		msg2 := Message{
			Role:      "assistant",
			Content:   "Hi",
			Timestamp: time.Now(),
		}
		err = state.AddMessage(msg2)
		require.NoError(t, err)

		// Snapshot should be unchanged
		assert.Len(t, snapshot.Messages, 1)
		assert.Len(t, state.Messages(), 2)
	})

	t.Run("includes all state fields", func(t *testing.T) {
		state := NewConversationState()

		msg := Message{
			Role:      "user",
			Content:   "test",
			Timestamp: time.Now(),
		}
		state.AddMessage(msg)

		toolCall := ToolCall{
			ID:        "call_1",
			Name:      "test_tool",
			Arguments: map[string]interface{}{},
			Status:    ToolCallPending,
		}
		state.AddToolCall(toolCall)

		usage := TokenUsage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		}
		state.AddTokenUsage(usage)

		snapshot := state.Snapshot()
		assert.Len(t, snapshot.Messages, 1)
		assert.Len(t, snapshot.ToolCalls, 1)
		assert.Equal(t, int64(150), snapshot.TotalTokens.TotalTokens)
	})
}

func TestConversationState_Serialization(t *testing.T) {
	t.Run("marshals to JSON", func(t *testing.T) {
		state := NewConversationState()

		msg := Message{
			Role:      "user",
			Content:   "test",
			Timestamp: time.Now(),
		}
		state.AddMessage(msg)

		snapshot := state.Snapshot()
		data, err := json.Marshal(snapshot)
		require.NoError(t, err)
		assert.NotEmpty(t, data)
	})

	t.Run("unmarshals from JSON", func(t *testing.T) {
		state := NewConversationState()

		msg := Message{
			Role:      "user",
			Content:   "test",
			Timestamp: time.Now(),
		}
		state.AddMessage(msg)

		snapshot := state.Snapshot()
		data, err := json.Marshal(snapshot)
		require.NoError(t, err)

		var restored StateSnapshot
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.Len(t, restored.Messages, 1)
		assert.Equal(t, "user", restored.Messages[0].Role)
		assert.Equal(t, "test", restored.Messages[0].Content)
	})

	t.Run("round-trips complex state", func(t *testing.T) {
		state := NewConversationState()

		// Add messages
		state.AddMessage(Message{
			Role:      "user",
			Content:   "Hello",
			Timestamp: time.Now(),
		})
		state.AddMessage(Message{
			Role:      "assistant",
			Content:   "Hi",
			Timestamp: time.Now(),
		})

		// Add tool calls
		state.AddToolCall(ToolCall{
			ID:        "call_1",
			Name:      "read_file",
			Arguments: map[string]interface{}{"path": "/test.txt"},
			Status:    ToolCallExecuted,
		})

		// Add token usage
		state.AddTokenUsage(TokenUsage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		})

		snapshot := state.Snapshot()
		data, err := json.Marshal(snapshot)
		require.NoError(t, err)

		var restored StateSnapshot
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.Len(t, restored.Messages, 2)
		assert.Len(t, restored.ToolCalls, 1)
		assert.Equal(t, int64(150), restored.TotalTokens.TotalTokens)
	})
}

func TestConversationState_ThreadSafety(t *testing.T) {
	t.Run("concurrent message adds", func(t *testing.T) {
		state := NewConversationState()

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				msg := Message{
					Role:      "user",
					Content:   "test",
					Timestamp: time.Now(),
				}
				state.AddMessage(msg)
			}(i)
		}
		wg.Wait()

		messages := state.Messages()
		assert.Len(t, messages, 100)
	})

	t.Run("concurrent tool call updates", func(t *testing.T) {
		state := NewConversationState()

		// Add tool calls first
		for i := 0; i < 10; i++ {
			toolCall := ToolCall{
				ID:        string(rune('a' + i)),
				Name:      "test",
				Arguments: map[string]interface{}{},
				Status:    ToolCallPending,
			}
			state.AddToolCall(toolCall)
		}

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				state.UpdateToolCallStatus(string(rune('a'+n)), ToolCallApproved)
			}(i)
		}
		wg.Wait()

		calls := state.ToolCalls()
		assert.Len(t, calls, 10)
		for _, call := range calls {
			assert.Equal(t, ToolCallApproved, call.Status)
		}
	})

	t.Run("concurrent reads and writes", func(t *testing.T) {
		state := NewConversationState()

		var wg sync.WaitGroup

		// Writers
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				msg := Message{
					Role:      "user",
					Content:   "test",
					Timestamp: time.Now(),
				}
				state.AddMessage(msg)
			}()
		}

		// Readers
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = state.Messages()
				_ = state.ToolCalls()
			}()
		}

		wg.Wait()

		messages := state.Messages()
		assert.Len(t, messages, 50)
	})
}

func TestConversationState_Clear(t *testing.T) {
	t.Run("clears all messages and tool calls", func(t *testing.T) {
		state := NewConversationState()

		state.AddMessage(Message{
			Role:      "user",
			Content:   "test",
			Timestamp: time.Now(),
		})

		state.AddToolCall(ToolCall{
			ID:        "call_1",
			Name:      "test",
			Arguments: map[string]interface{}{},
			Status:    ToolCallPending,
		})

		state.Clear()

		assert.Empty(t, state.Messages())
		assert.Empty(t, state.ToolCalls())
	})

	t.Run("preserves timestamps", func(t *testing.T) {
		state := NewConversationState()

		createdAt := state.CreatedAt

		time.Sleep(10 * time.Millisecond)

		state.AddMessage(Message{
			Role:      "user",
			Content:   "test",
			Timestamp: time.Now(),
		})

		state.Clear()

		assert.Equal(t, createdAt, state.CreatedAt)
		assert.True(t, state.UpdatedAt.After(createdAt))
	})
}
