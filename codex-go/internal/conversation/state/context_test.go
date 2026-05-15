package state

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTurnContext(t *testing.T) {
	t.Run("creates context with user input", func(t *testing.T) {
		ctx := NewTurnContext("user_123", "Hello, world!")

		require.NotNil(t, ctx)
		assert.Equal(t, "user_123", ctx.UserID)
		assert.Equal(t, "Hello, world!", ctx.UserInput)
		assert.NotZero(t, ctx.StartTime)
		assert.Empty(t, ctx.ToolResults)
		assert.Empty(t, ctx.SystemMessages)
	})

	t.Run("creates unique contexts", func(t *testing.T) {
		ctx1 := NewTurnContext("user_1", "input 1")
		time.Sleep(time.Millisecond)
		ctx2 := NewTurnContext("user_2", "input 2")

		assert.NotEqual(t, ctx1.StartTime, ctx2.StartTime)
		assert.NotEqual(t, ctx1.UserID, ctx2.UserID)
	})
}

func TestTurnContext_AddToolResult(t *testing.T) {
	t.Run("adds tool result", func(t *testing.T) {
		ctx := NewTurnContext("user_123", "test")

		result := ToolResult{
			CallID: "call_1",
			Name:   "read_file",
			Output: "file contents",
		}

		ctx.AddToolResult(result)

		assert.Len(t, ctx.ToolResults, 1)
		assert.Equal(t, "call_1", ctx.ToolResults[0].CallID)
	})

	t.Run("maintains tool result order", func(t *testing.T) {
		ctx := NewTurnContext("user_123", "test")

		for i := 0; i < 5; i++ {
			result := ToolResult{
				CallID: string(rune('a' + i)),
				Name:   "test",
				Output: i,
			}
			ctx.AddToolResult(result)
		}

		assert.Len(t, ctx.ToolResults, 5)
		for i := 0; i < 5; i++ {
			assert.Equal(t, string(rune('a'+i)), ctx.ToolResults[i].CallID)
		}
	})

	t.Run("accumulates multiple results", func(t *testing.T) {
		ctx := NewTurnContext("user_123", "test")

		result1 := ToolResult{CallID: "call_1", Name: "tool1", Output: "result1"}
		result2 := ToolResult{CallID: "call_2", Name: "tool2", Output: "result2"}

		ctx.AddToolResult(result1)
		ctx.AddToolResult(result2)

		assert.Len(t, ctx.ToolResults, 2)
	})
}

func TestTurnContext_AddSystemMessage(t *testing.T) {
	t.Run("adds system message", func(t *testing.T) {
		ctx := NewTurnContext("user_123", "test")

		ctx.AddSystemMessage("System notification")

		assert.Len(t, ctx.SystemMessages, 1)
		assert.Equal(t, "System notification", ctx.SystemMessages[0])
	})

	t.Run("maintains system message order", func(t *testing.T) {
		ctx := NewTurnContext("user_123", "test")

		ctx.AddSystemMessage("First")
		ctx.AddSystemMessage("Second")
		ctx.AddSystemMessage("Third")

		assert.Len(t, ctx.SystemMessages, 3)
		assert.Equal(t, "First", ctx.SystemMessages[0])
		assert.Equal(t, "Second", ctx.SystemMessages[1])
		assert.Equal(t, "Third", ctx.SystemMessages[2])
	})
}

func TestTurnContext_SetMetadata(t *testing.T) {
	t.Run("sets and gets metadata", func(t *testing.T) {
		ctx := NewTurnContext("user_123", "test")

		ctx.SetMetadata("model", "claude-3")
		ctx.SetMetadata("temperature", 0.7)

		model, exists := ctx.GetMetadata("model")
		assert.True(t, exists)
		assert.Equal(t, "claude-3", model)

		temp, exists := ctx.GetMetadata("temperature")
		assert.True(t, exists)
		assert.Equal(t, 0.7, temp)
	})

	t.Run("returns false for non-existent metadata", func(t *testing.T) {
		ctx := NewTurnContext("user_123", "test")

		value, exists := ctx.GetMetadata("nonexistent")
		assert.False(t, exists)
		assert.Nil(t, value)
	})

	t.Run("overwrites existing metadata", func(t *testing.T) {
		ctx := NewTurnContext("user_123", "test")

		ctx.SetMetadata("key", "value1")
		ctx.SetMetadata("key", "value2")

		value, exists := ctx.GetMetadata("key")
		assert.True(t, exists)
		assert.Equal(t, "value2", value)
	})
}

func TestTurnContext_Complete(t *testing.T) {
	t.Run("marks context as complete", func(t *testing.T) {
		ctx := NewTurnContext("user_123", "test")

		assert.False(t, ctx.IsComplete())
		assert.Zero(t, ctx.EndTime)

		ctx.Complete()

		assert.True(t, ctx.IsComplete())
		assert.NotZero(t, ctx.EndTime)
		assert.True(t, ctx.EndTime.After(ctx.StartTime))
	})

	t.Run("idempotent completion", func(t *testing.T) {
		ctx := NewTurnContext("user_123", "test")

		ctx.Complete()
		firstEndTime := ctx.EndTime

		time.Sleep(time.Millisecond)
		ctx.Complete()

		assert.Equal(t, firstEndTime, ctx.EndTime)
	})
}

func TestTurnContext_Duration(t *testing.T) {
	t.Run("returns zero duration before completion", func(t *testing.T) {
		ctx := NewTurnContext("user_123", "test")

		duration := ctx.Duration()
		assert.Equal(t, time.Duration(0), duration)
	})

	t.Run("returns duration after completion", func(t *testing.T) {
		ctx := NewTurnContext("user_123", "test")

		time.Sleep(10 * time.Millisecond)
		ctx.Complete()

		duration := ctx.Duration()
		assert.True(t, duration > 0)
		assert.True(t, duration >= 10*time.Millisecond)
	})
}

func TestTurnContext_Serialization(t *testing.T) {
	t.Run("marshals to JSON", func(t *testing.T) {
		ctx := NewTurnContext("user_123", "test input")
		ctx.AddToolResult(ToolResult{
			CallID: "call_1",
			Name:   "test_tool",
			Output: "result",
		})
		ctx.AddSystemMessage("system msg")
		ctx.SetMetadata("key", "value")

		data, err := json.Marshal(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, data)
	})

	t.Run("unmarshals from JSON", func(t *testing.T) {
		ctx := NewTurnContext("user_123", "test input")
		ctx.AddToolResult(ToolResult{
			CallID: "call_1",
			Name:   "test_tool",
			Output: "result",
		})
		ctx.AddSystemMessage("system msg")

		data, err := json.Marshal(ctx)
		require.NoError(t, err)

		var restored TurnContext
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.Equal(t, ctx.UserID, restored.UserID)
		assert.Equal(t, ctx.UserInput, restored.UserInput)
		assert.Len(t, restored.ToolResults, 1)
		assert.Len(t, restored.SystemMessages, 1)
	})
}

func TestContextHistory(t *testing.T) {
	t.Run("creates empty history", func(t *testing.T) {
		history := NewContextHistory()

		require.NotNil(t, history)
		assert.Empty(t, history.Contexts())
	})

	t.Run("adds context to history", func(t *testing.T) {
		history := NewContextHistory()

		ctx := NewTurnContext("user_123", "test")
		history.Add(ctx)

		contexts := history.Contexts()
		assert.Len(t, contexts, 1)
		assert.Equal(t, "user_123", contexts[0].UserID)
	})

	t.Run("maintains context order", func(t *testing.T) {
		history := NewContextHistory()

		for i := 0; i < 5; i++ {
			ctx := NewTurnContext(string(rune('a'+i)), "test")
			history.Add(ctx)
		}

		contexts := history.Contexts()
		assert.Len(t, contexts, 5)
		for i := 0; i < 5; i++ {
			assert.Equal(t, string(rune('a'+i)), contexts[i].UserID)
		}
	})

	t.Run("gets latest context", func(t *testing.T) {
		history := NewContextHistory()

		ctx1 := NewTurnContext("user_1", "first")
		ctx2 := NewTurnContext("user_2", "second")
		ctx3 := NewTurnContext("user_3", "third")

		history.Add(ctx1)
		history.Add(ctx2)
		history.Add(ctx3)

		latest := history.Latest()
		require.NotNil(t, latest)
		assert.Equal(t, "user_3", latest.UserID)
	})

	t.Run("returns nil for empty history", func(t *testing.T) {
		history := NewContextHistory()

		latest := history.Latest()
		assert.Nil(t, latest)
	})
}

func TestContextHistory_Since(t *testing.T) {
	t.Run("returns contexts after timestamp", func(t *testing.T) {
		history := NewContextHistory()

		now := time.Now()
		ctx1 := NewTurnContext("user_1", "first")
		history.Add(ctx1)

		time.Sleep(10 * time.Millisecond)
		cutoff := time.Now()

		ctx2 := NewTurnContext("user_2", "second")
		history.Add(ctx2)

		ctx3 := NewTurnContext("user_3", "third")
		history.Add(ctx3)

		recent := history.Since(cutoff)
		assert.Len(t, recent, 2)
		assert.Equal(t, "user_2", recent[0].UserID)
		assert.Equal(t, "user_3", recent[1].UserID)

		all := history.Since(now.Add(-time.Hour))
		assert.Len(t, all, 3)
	})
}

func TestContextHistory_Limit(t *testing.T) {
	t.Run("returns last N contexts", func(t *testing.T) {
		history := NewContextHistory()

		for i := 0; i < 10; i++ {
			ctx := NewTurnContext(string(rune('a'+i)), "test")
			history.Add(ctx)
		}

		last3 := history.Limit(3)
		assert.Len(t, last3, 3)
		assert.Equal(t, "h", last3[0].UserID)
		assert.Equal(t, "i", last3[1].UserID)
		assert.Equal(t, "j", last3[2].UserID)
	})

	t.Run("returns all if limit exceeds size", func(t *testing.T) {
		history := NewContextHistory()

		ctx := NewTurnContext("user_1", "test")
		history.Add(ctx)

		limited := history.Limit(10)
		assert.Len(t, limited, 1)
	})

	t.Run("returns empty for zero limit", func(t *testing.T) {
		history := NewContextHistory()

		ctx := NewTurnContext("user_1", "test")
		history.Add(ctx)

		limited := history.Limit(0)
		assert.Empty(t, limited)
	})
}

func TestContextHistory_Clear(t *testing.T) {
	t.Run("clears all contexts", func(t *testing.T) {
		history := NewContextHistory()

		for i := 0; i < 5; i++ {
			ctx := NewTurnContext(string(rune('a'+i)), "test")
			history.Add(ctx)
		}

		assert.Len(t, history.Contexts(), 5)

		history.Clear()

		assert.Empty(t, history.Contexts())
		assert.Nil(t, history.Latest())
	})
}

func TestContextHistory_ThreadSafety(t *testing.T) {
	t.Run("concurrent adds", func(t *testing.T) {
		history := NewContextHistory()

		done := make(chan bool, 100)
		for i := 0; i < 100; i++ {
			go func(n int) {
				ctx := NewTurnContext(string(rune('a'+n%26)), "test")
				history.Add(ctx)
				done <- true
			}(i)
		}

		for i := 0; i < 100; i++ {
			<-done
		}

		contexts := history.Contexts()
		assert.Len(t, contexts, 100)
	})

	t.Run("concurrent reads and writes", func(t *testing.T) {
		history := NewContextHistory()

		// Pre-populate
		for i := 0; i < 10; i++ {
			ctx := NewTurnContext(string(rune('a'+i)), "test")
			history.Add(ctx)
		}

		done := make(chan bool, 200)

		// Writers
		for i := 0; i < 100; i++ {
			go func() {
				ctx := NewTurnContext("user", "test")
				history.Add(ctx)
				done <- true
			}()
		}

		// Readers
		for i := 0; i < 100; i++ {
			go func() {
				_ = history.Contexts()
				_ = history.Latest()
				done <- true
			}()
		}

		for i := 0; i < 200; i++ {
			<-done
		}

		contexts := history.Contexts()
		assert.Len(t, contexts, 110)
	})
}
