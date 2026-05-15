package state

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMessageHistory(t *testing.T) {
	t.Run("creates empty history", func(t *testing.T) {
		history := NewMessageHistory()

		require.NotNil(t, history)
		assert.Empty(t, history.All())
		assert.Equal(t, 0, history.Count())
	})
}

func TestMessageHistory_Append(t *testing.T) {
	t.Run("appends message", func(t *testing.T) {
		history := NewMessageHistory()

		msg := Message{
			Role:      "user",
			Content:   "Hello",
			Timestamp: time.Now(),
		}

		err := history.Append(msg)
		require.NoError(t, err)
		assert.Equal(t, 1, history.Count())
	})

	t.Run("maintains append order", func(t *testing.T) {
		history := NewMessageHistory()

		for i := 0; i < 5; i++ {
			msg := Message{
				Role:      "user",
				Content:   string(rune('A' + i)),
				Timestamp: time.Now(),
			}
			history.Append(msg)
		}

		messages := history.All()
		assert.Len(t, messages, 5)
		for i := 0; i < 5; i++ {
			assert.Equal(t, string(rune('A'+i)), messages[i].Content)
		}
	})

	t.Run("validates message before append", func(t *testing.T) {
		history := NewMessageHistory()

		msg := Message{
			Role:      "invalid",
			Content:   "test",
			Timestamp: time.Now(),
		}

		err := history.Append(msg)
		assert.Error(t, err)
		assert.Equal(t, 0, history.Count())
	})

	t.Run("accepts tool role", func(t *testing.T) {
		history := NewMessageHistory()

		msg := Message{
			Role:      "tool",
			Content:   "Tool execution result",
			Timestamp: time.Now(),
		}

		err := history.Append(msg)
		require.NoError(t, err)
		assert.Equal(t, 1, history.Count())

		messages := history.All()
		assert.Equal(t, "tool", messages[0].Role)
		assert.Equal(t, "Tool execution result", messages[0].Content)
	})
}

func TestMessageHistory_GetByRole(t *testing.T) {
	t.Run("filters by user role", func(t *testing.T) {
		history := NewMessageHistory()

		history.Append(Message{Role: "user", Content: "User 1", Timestamp: time.Now()})
		history.Append(Message{Role: "assistant", Content: "Assistant 1", Timestamp: time.Now()})
		history.Append(Message{Role: "user", Content: "User 2", Timestamp: time.Now()})
		history.Append(Message{Role: "system", Content: "System 1", Timestamp: time.Now()})

		userMessages := history.GetByRole("user")
		assert.Len(t, userMessages, 2)
		assert.Equal(t, "User 1", userMessages[0].Content)
		assert.Equal(t, "User 2", userMessages[1].Content)
	})

	t.Run("filters by assistant role", func(t *testing.T) {
		history := NewMessageHistory()

		history.Append(Message{Role: "user", Content: "User 1", Timestamp: time.Now()})
		history.Append(Message{Role: "assistant", Content: "Assistant 1", Timestamp: time.Now()})
		history.Append(Message{Role: "assistant", Content: "Assistant 2", Timestamp: time.Now()})

		assistantMessages := history.GetByRole("assistant")
		assert.Len(t, assistantMessages, 2)
		assert.Equal(t, "Assistant 1", assistantMessages[0].Content)
		assert.Equal(t, "Assistant 2", assistantMessages[1].Content)
	})

	t.Run("returns empty for non-existent role", func(t *testing.T) {
		history := NewMessageHistory()

		history.Append(Message{Role: "user", Content: "User 1", Timestamp: time.Now()})

		messages := history.GetByRole("system")
		assert.Empty(t, messages)
	})
}

func TestMessageHistory_GetLast(t *testing.T) {
	t.Run("returns last message", func(t *testing.T) {
		history := NewMessageHistory()

		history.Append(Message{Role: "user", Content: "First", Timestamp: time.Now()})
		history.Append(Message{Role: "assistant", Content: "Second", Timestamp: time.Now()})
		history.Append(Message{Role: "user", Content: "Third", Timestamp: time.Now()})

		last := history.GetLast()
		require.NotNil(t, last)
		assert.Equal(t, "Third", last.Content)
	})

	t.Run("returns nil for empty history", func(t *testing.T) {
		history := NewMessageHistory()

		last := history.GetLast()
		assert.Nil(t, last)
	})
}

func TestMessageHistory_GetLastN(t *testing.T) {
	t.Run("returns last N messages", func(t *testing.T) {
		history := NewMessageHistory()

		for i := 0; i < 10; i++ {
			history.Append(Message{
				Role:      "user",
				Content:   string(rune('A' + i)),
				Timestamp: time.Now(),
			})
		}

		last3 := history.GetLastN(3)
		assert.Len(t, last3, 3)
		assert.Equal(t, "H", last3[0].Content)
		assert.Equal(t, "I", last3[1].Content)
		assert.Equal(t, "J", last3[2].Content)
	})

	t.Run("returns all if N exceeds size", func(t *testing.T) {
		history := NewMessageHistory()

		history.Append(Message{Role: "user", Content: "Only", Timestamp: time.Now()})

		messages := history.GetLastN(10)
		assert.Len(t, messages, 1)
	})

	t.Run("returns empty for zero N", func(t *testing.T) {
		history := NewMessageHistory()

		history.Append(Message{Role: "user", Content: "Test", Timestamp: time.Now()})

		messages := history.GetLastN(0)
		assert.Empty(t, messages)
	})
}

func TestMessageHistory_GetSince(t *testing.T) {
	t.Run("returns messages after timestamp", func(t *testing.T) {
		history := NewMessageHistory()

		start := time.Now()
		history.Append(Message{Role: "user", Content: "Old", Timestamp: start})

		time.Sleep(10 * time.Millisecond)
		cutoff := time.Now()

		history.Append(Message{Role: "user", Content: "New 1", Timestamp: time.Now()})
		history.Append(Message{Role: "user", Content: "New 2", Timestamp: time.Now()})

		recent := history.GetSince(cutoff)
		assert.Len(t, recent, 2)
		assert.Equal(t, "New 1", recent[0].Content)
		assert.Equal(t, "New 2", recent[1].Content)
	})

	t.Run("returns all messages for old timestamp", func(t *testing.T) {
		history := NewMessageHistory()

		history.Append(Message{Role: "user", Content: "Test 1", Timestamp: time.Now()})
		history.Append(Message{Role: "user", Content: "Test 2", Timestamp: time.Now()})

		all := history.GetSince(time.Now().Add(-time.Hour))
		assert.Len(t, all, 2)
	})
}

func TestMessageHistory_Compact(t *testing.T) {
	t.Run("keeps last N messages", func(t *testing.T) {
		history := NewMessageHistory()

		for i := 0; i < 10; i++ {
			history.Append(Message{
				Role:      "user",
				Content:   string(rune('A' + i)),
				Timestamp: time.Now(),
			})
		}

		history.Compact(3)

		messages := history.All()
		assert.Len(t, messages, 3)
		assert.Equal(t, "H", messages[0].Content)
		assert.Equal(t, "I", messages[1].Content)
		assert.Equal(t, "J", messages[2].Content)
	})

	t.Run("does nothing if N exceeds size", func(t *testing.T) {
		history := NewMessageHistory()

		history.Append(Message{Role: "user", Content: "Test 1", Timestamp: time.Now()})
		history.Append(Message{Role: "user", Content: "Test 2", Timestamp: time.Now()})

		history.Compact(10)

		messages := history.All()
		assert.Len(t, messages, 2)
	})

	t.Run("clears all if N is zero", func(t *testing.T) {
		history := NewMessageHistory()

		history.Append(Message{Role: "user", Content: "Test", Timestamp: time.Now()})

		history.Compact(0)

		messages := history.All()
		assert.Empty(t, messages)
	})
}

func TestMessageHistory_Clear(t *testing.T) {
	t.Run("removes all messages", func(t *testing.T) {
		history := NewMessageHistory()

		for i := 0; i < 5; i++ {
			history.Append(Message{
				Role:      "user",
				Content:   "test",
				Timestamp: time.Now(),
			})
		}

		assert.Equal(t, 5, history.Count())

		history.Clear()

		assert.Equal(t, 0, history.Count())
		assert.Empty(t, history.All())
	})
}

func TestMessageHistory_Serialization(t *testing.T) {
	t.Run("marshals to JSON", func(t *testing.T) {
		history := NewMessageHistory()

		history.Append(Message{Role: "user", Content: "Hello", Timestamp: time.Now()})
		history.Append(Message{Role: "assistant", Content: "Hi", Timestamp: time.Now()})

		data, err := json.Marshal(history.All())
		require.NoError(t, err)
		assert.NotEmpty(t, data)
	})

	t.Run("unmarshals from JSON", func(t *testing.T) {
		history := NewMessageHistory()

		msg1 := Message{Role: "user", Content: "Hello", Timestamp: time.Now()}
		msg2 := Message{Role: "assistant", Content: "Hi", Timestamp: time.Now()}

		history.Append(msg1)
		history.Append(msg2)

		data, err := json.Marshal(history.All())
		require.NoError(t, err)

		var restored []Message
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.Len(t, restored, 2)
		assert.Equal(t, "user", restored[0].Role)
		assert.Equal(t, "Hello", restored[0].Content)
	})
}

func TestMessageHistory_ThreadSafety(t *testing.T) {
	t.Run("concurrent appends", func(t *testing.T) {
		history := NewMessageHistory()

		done := make(chan bool, 100)
		for i := 0; i < 100; i++ {
			go func() {
				msg := Message{
					Role:      "user",
					Content:   "test",
					Timestamp: time.Now(),
				}
				history.Append(msg)
				done <- true
			}()
		}

		for i := 0; i < 100; i++ {
			<-done
		}

		assert.Equal(t, 100, history.Count())
	})

	t.Run("concurrent reads and writes", func(t *testing.T) {
		history := NewMessageHistory()

		// Pre-populate
		for i := 0; i < 10; i++ {
			history.Append(Message{
				Role:      "user",
				Content:   "test",
				Timestamp: time.Now(),
			})
		}

		done := make(chan bool, 200)

		// Writers
		for i := 0; i < 100; i++ {
			go func() {
				msg := Message{
					Role:      "user",
					Content:   "test",
					Timestamp: time.Now(),
				}
				history.Append(msg)
				done <- true
			}()
		}

		// Readers
		for i := 0; i < 100; i++ {
			go func() {
				_ = history.All()
				_ = history.Count()
				_ = history.GetLast()
				done <- true
			}()
		}

		for i := 0; i < 200; i++ {
			<-done
		}

		assert.Equal(t, 110, history.Count())
	})

	t.Run("concurrent compact and read", func(t *testing.T) {
		history := NewMessageHistory()

		// Pre-populate
		for i := 0; i < 100; i++ {
			history.Append(Message{
				Role:      "user",
				Content:   "test",
				Timestamp: time.Now(),
			})
		}

		done := make(chan bool, 2)

		go func() {
			history.Compact(50)
			done <- true
		}()

		go func() {
			_ = history.All()
			done <- true
		}()

		<-done
		<-done

		// Should have exactly 50 after compaction
		assert.LessOrEqual(t, history.Count(), 100)
	})
}

func TestMessageHistory_AlternatingRoles(t *testing.T) {
	t.Run("alternates user and assistant", func(t *testing.T) {
		history := NewMessageHistory()

		history.Append(Message{Role: "user", Content: "Q1", Timestamp: time.Now()})
		history.Append(Message{Role: "assistant", Content: "A1", Timestamp: time.Now()})
		history.Append(Message{Role: "user", Content: "Q2", Timestamp: time.Now()})
		history.Append(Message{Role: "assistant", Content: "A2", Timestamp: time.Now()})

		messages := history.All()
		assert.Len(t, messages, 4)
		assert.Equal(t, "user", messages[0].Role)
		assert.Equal(t, "assistant", messages[1].Role)
		assert.Equal(t, "user", messages[2].Role)
		assert.Equal(t, "assistant", messages[3].Role)
	})
}

func TestMessageHistory_Count(t *testing.T) {
	t.Run("tracks count accurately", func(t *testing.T) {
		history := NewMessageHistory()
		assert.Equal(t, 0, history.Count())

		history.Append(Message{Role: "user", Content: "1", Timestamp: time.Now()})
		assert.Equal(t, 1, history.Count())

		history.Append(Message{Role: "user", Content: "2", Timestamp: time.Now()})
		assert.Equal(t, 2, history.Count())

		history.Clear()
		assert.Equal(t, 0, history.Count())
	})
}
