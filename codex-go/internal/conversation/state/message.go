package state

import (
	"fmt"
	"sync"
	"time"
)

// MessageHistory manages a list of conversation messages.
// It is thread-safe and supports filtering, compaction, and serialization.
type MessageHistory struct {
	mu       sync.RWMutex
	messages []Message
}

// NewMessageHistory creates a new message history.
func NewMessageHistory() *MessageHistory {
	return &MessageHistory{
		messages: make([]Message, 0),
	}
}

// Append adds a message to the history.
// This method is thread-safe and validates the message before adding.
func (h *MessageHistory) Append(msg Message) error {
	// Validate message
	if msg.Role != "user" && msg.Role != "assistant" && msg.Role != "system" && msg.Role != "tool" {
		return fmt.Errorf("invalid role: %s", msg.Role)
	}
	if msg.Content == "" {
		return fmt.Errorf("empty content not allowed")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = append(h.messages, msg)
	return nil
}

// All returns a copy of all messages.
// This method is thread-safe.
func (h *MessageHistory) All() []Message {
	h.mu.RLock()
	defer h.mu.RUnlock()

	messages := make([]Message, len(h.messages))
	copy(messages, h.messages)
	return messages
}

// Count returns the number of messages in the history.
// This method is thread-safe.
func (h *MessageHistory) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.messages)
}

// GetByRole returns all messages with the specified role.
// This method is thread-safe.
func (h *MessageHistory) GetByRole(role string) []Message {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var filtered []Message
	for _, msg := range h.messages {
		if msg.Role == role {
			filtered = append(filtered, msg)
		}
	}

	return filtered
}

// GetLast returns the most recent message.
// Returns nil if the history is empty.
// This method is thread-safe.
func (h *MessageHistory) GetLast() *Message {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.messages) == 0 {
		return nil
	}

	last := h.messages[len(h.messages)-1]
	return &last
}

// GetLastN returns the last N messages.
// If N exceeds the history size, returns all messages.
// This method is thread-safe.
func (h *MessageHistory) GetLastN(n int) []Message {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if n <= 0 {
		return []Message{}
	}

	if n >= len(h.messages) {
		messages := make([]Message, len(h.messages))
		copy(messages, h.messages)
		return messages
	}

	start := len(h.messages) - n
	messages := make([]Message, n)
	copy(messages, h.messages[start:])
	return messages
}

// GetSince returns all messages after the specified timestamp.
// This method is thread-safe.
func (h *MessageHistory) GetSince(t time.Time) []Message {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var filtered []Message
	for _, msg := range h.messages {
		if msg.Timestamp.After(t) {
			filtered = append(filtered, msg)
		}
	}

	return filtered
}

// Compact reduces the history to the last N messages.
// This is useful for managing memory and token limits.
// This method is thread-safe.
func (h *MessageHistory) Compact(keepLast int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if keepLast <= 0 {
		h.messages = make([]Message, 0)
		return
	}

	if keepLast >= len(h.messages) {
		return
	}

	start := len(h.messages) - keepLast
	h.messages = h.messages[start:]
}

// Clear removes all messages from the history.
// This method is thread-safe.
func (h *MessageHistory) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = make([]Message, 0)
}
