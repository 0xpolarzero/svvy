package state

import (
	"sync"
	"time"
)

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	CallID    string
	Name      string
	Output    interface{}
	Error     string
	Duration  time.Duration
	Timestamp time.Time
}

// TurnContext represents the context for a single conversation turn.
// It accumulates user input, tool results, and system messages.
type TurnContext struct {
	UserID         string
	UserInput      string
	ToolResults    []ToolResult
	SystemMessages []string
	Metadata       map[string]interface{}
	StartTime      time.Time
	EndTime        time.Time
	complete       bool
}

// NewTurnContext creates a new turn context.
func NewTurnContext(userID, userInput string) *TurnContext {
	return &TurnContext{
		UserID:         userID,
		UserInput:      userInput,
		ToolResults:    make([]ToolResult, 0),
		SystemMessages: make([]string, 0),
		Metadata:       make(map[string]interface{}),
		StartTime:      time.Now(),
	}
}

// AddToolResult adds a tool execution result to the context.
func (c *TurnContext) AddToolResult(result ToolResult) {
	if result.Timestamp.IsZero() {
		result.Timestamp = time.Now()
	}
	c.ToolResults = append(c.ToolResults, result)
}

// AddSystemMessage adds a system message to the context.
func (c *TurnContext) AddSystemMessage(message string) {
	c.SystemMessages = append(c.SystemMessages, message)
}

// SetMetadata sets a metadata value.
func (c *TurnContext) SetMetadata(key string, value interface{}) {
	c.Metadata[key] = value
}

// GetMetadata retrieves a metadata value.
func (c *TurnContext) GetMetadata(key string) (interface{}, bool) {
	value, exists := c.Metadata[key]
	return value, exists
}

// Complete marks the turn context as complete.
func (c *TurnContext) Complete() {
	if !c.complete {
		c.EndTime = time.Now()
		c.complete = true
	}
}

// IsComplete returns whether the turn context is complete.
func (c *TurnContext) IsComplete() bool {
	return c.complete
}

// Duration returns the duration of the turn.
// Returns 0 if the turn is not yet complete.
func (c *TurnContext) Duration() time.Duration {
	if !c.complete {
		return 0
	}
	return c.EndTime.Sub(c.StartTime)
}

// ContextHistory maintains a history of turn contexts.
// It is thread-safe.
type ContextHistory struct {
	mu       sync.RWMutex
	contexts []*TurnContext
}

// NewContextHistory creates a new context history.
func NewContextHistory() *ContextHistory {
	return &ContextHistory{
		contexts: make([]*TurnContext, 0),
	}
}

// Add adds a context to the history.
func (h *ContextHistory) Add(ctx *TurnContext) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.contexts = append(h.contexts, ctx)
}

// Contexts returns a copy of all contexts.
func (h *ContextHistory) Contexts() []*TurnContext {
	h.mu.RLock()
	defer h.mu.RUnlock()

	contexts := make([]*TurnContext, len(h.contexts))
	copy(contexts, h.contexts)
	return contexts
}

// Latest returns the most recent context.
// Returns nil if the history is empty.
func (h *ContextHistory) Latest() *TurnContext {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.contexts) == 0 {
		return nil
	}

	return h.contexts[len(h.contexts)-1]
}

// Since returns all contexts that started after the given time.
func (h *ContextHistory) Since(t time.Time) []*TurnContext {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []*TurnContext
	for _, ctx := range h.contexts {
		if ctx.StartTime.After(t) {
			result = append(result, ctx)
		}
	}

	return result
}

// Limit returns the last N contexts.
// If N exceeds the history size, returns all contexts.
func (h *ContextHistory) Limit(n int) []*TurnContext {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if n <= 0 {
		return []*TurnContext{}
	}

	if n >= len(h.contexts) {
		contexts := make([]*TurnContext, len(h.contexts))
		copy(contexts, h.contexts)
		return contexts
	}

	start := len(h.contexts) - n
	contexts := make([]*TurnContext, n)
	copy(contexts, h.contexts[start:])
	return contexts
}

// Clear removes all contexts from the history.
func (h *ContextHistory) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.contexts = make([]*TurnContext, 0)
}
