// Package state provides conversation state tracking with thread-safe operations.
//
// This package implements immutable state updates, context accumulation,
// tool call lifecycle tracking, and policy enforcement for Codex conversations.
package state

import (
	"fmt"
	"sync"
	"time"
)

// ToolCallStatus represents the lifecycle status of a tool call.
type ToolCallStatus string

const (
	// ToolCallPending indicates the tool call has been requested but not approved.
	ToolCallPending ToolCallStatus = "pending"
	// ToolCallApproved indicates the tool call has been approved for execution.
	ToolCallApproved ToolCallStatus = "approved"
	// ToolCallExecuted indicates the tool call has been executed.
	ToolCallExecuted ToolCallStatus = "executed"
	// ToolCallRejected indicates the tool call was rejected.
	ToolCallRejected ToolCallStatus = "rejected"
)

// Message represents a conversation message.
type Message struct {
	Role      string
	Content   string
	Reasoning string    // Optional reasoning/thinking content from the model
	Timestamp time.Time
	ID        string
}

// ToolCall represents a tool call in the conversation.
type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]interface{}
	Status    ToolCallStatus
	Result    interface{}
	Error     string
	Timestamp time.Time
}

// TokenUsage tracks token usage for the conversation.
type TokenUsage struct {
	InputTokens           int64
	CachedInputTokens     int64
	OutputTokens          int64
	ReasoningOutputTokens int64
	TotalTokens           int64
}

// SessionMetadata contains configuration and metadata about the session.
type SessionMetadata struct {
	SessionID        string
	Model            string
	Provider         string
	ApprovalPolicy   string
	SandboxMode      string
	MaxTurns         int
	ApprovalTimeout  time.Duration
	ReasoningEffort  *string
	HistoryLogID     uint64
	HistoryEntryCount int
}

// ConversationState tracks the complete state of a conversation.
// It is thread-safe and supports immutable snapshots.
type ConversationState struct {
	mu sync.RWMutex

	messages        []Message
	toolCalls       map[string]*ToolCall
	tokenUsage      []TokenUsage
	sessionMetadata *SessionMetadata
	plan            interface{} // Current plan/todo list (map[string]interface{})

	CreatedAt time.Time
	UpdatedAt time.Time
}

// StateSnapshot represents an immutable snapshot of conversation state.
type StateSnapshot struct {
	Messages    []Message
	ToolCalls   []ToolCall
	TotalTokens TokenUsage
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewConversationState creates a new conversation state.
func NewConversationState() *ConversationState {
	now := time.Now()
	return &ConversationState{
		messages:   make([]Message, 0),
		toolCalls:  make(map[string]*ToolCall),
		tokenUsage: make([]TokenUsage, 0),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// AddMessage adds a message to the conversation.
// This method is thread-safe.
func (s *ConversationState) AddMessage(msg Message) error {
	// Validate message
	if msg.Role != "user" && msg.Role != "assistant" && msg.Role != "system" && msg.Role != "tool" {
		return fmt.Errorf("invalid role: %s", msg.Role)
	}
	if msg.Content == "" {
		return fmt.Errorf("empty content not allowed")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, msg)
	s.UpdatedAt = time.Now()

	return nil
}

// Messages returns a copy of all messages.
// This method is thread-safe.
func (s *ConversationState) Messages() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modifications
	messages := make([]Message, len(s.messages))
	copy(messages, s.messages)
	return messages
}

// AddToolCall adds a tool call to the conversation.
// This method is thread-safe.
func (s *ConversationState) AddToolCall(call ToolCall) error {
	// Validate tool call
	if call.ID == "" {
		return fmt.Errorf("empty ID not allowed")
	}
	if call.Name == "" {
		return fmt.Errorf("empty name not allowed")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Store a copy
	callCopy := call
	if callCopy.Timestamp.IsZero() {
		callCopy.Timestamp = time.Now()
	}
	s.toolCalls[call.ID] = &callCopy
	s.UpdatedAt = time.Now()

	return nil
}

// UpdateToolCallStatus updates the status of a tool call.
// This method is thread-safe and validates status transitions.
func (s *ConversationState) UpdateToolCallStatus(id string, status ToolCallStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	call, exists := s.toolCalls[id]
	if !exists {
		return fmt.Errorf("tool call %s not found", id)
	}

	// Validate status transition
	if !isValidStatusTransition(call.Status, status) {
		return fmt.Errorf("invalid status transition from %s to %s", call.Status, status)
	}

	call.Status = status
	s.UpdatedAt = time.Now()

	return nil
}

// isValidStatusTransition checks if a status transition is valid.
func isValidStatusTransition(from, to ToolCallStatus) bool {
	// Define valid transitions
	validTransitions := map[ToolCallStatus][]ToolCallStatus{
		ToolCallPending:  {ToolCallApproved, ToolCallRejected},
		ToolCallApproved: {ToolCallExecuted},
		ToolCallExecuted: {},
		ToolCallRejected: {},
	}

	allowed, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, status := range allowed {
		if status == to {
			return true
		}
	}

	return false
}

// ToolCalls returns a copy of all tool calls.
// This method is thread-safe.
func (s *ConversationState) ToolCalls() []ToolCall {
	s.mu.RLock()
	defer s.mu.RUnlock()

	calls := make([]ToolCall, 0, len(s.toolCalls))
	for _, call := range s.toolCalls {
		calls = append(calls, *call)
	}

	return calls
}

// AddTokenUsage adds token usage to the conversation.
// This method is thread-safe.
func (s *ConversationState) AddTokenUsage(usage TokenUsage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokenUsage = append(s.tokenUsage, usage)
	s.UpdatedAt = time.Now()
}

// TotalTokenUsage returns the total token usage across all turns.
// This method is thread-safe.
func (s *ConversationState) TotalTokenUsage() TokenUsage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := TokenUsage{}
	for _, usage := range s.tokenUsage {
		total.InputTokens += usage.InputTokens
		total.CachedInputTokens += usage.CachedInputTokens
		total.OutputTokens += usage.OutputTokens
		total.ReasoningOutputTokens += usage.ReasoningOutputTokens
		total.TotalTokens += usage.TotalTokens
	}

	return total
}

// Snapshot creates an immutable snapshot of the current state.
// This method is thread-safe.
func (s *ConversationState) Snapshot() StateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Copy messages
	messages := make([]Message, len(s.messages))
	copy(messages, s.messages)

	// Copy tool calls
	toolCalls := make([]ToolCall, 0, len(s.toolCalls))
	for _, call := range s.toolCalls {
		toolCalls = append(toolCalls, *call)
	}

	return StateSnapshot{
		Messages:    messages,
		ToolCalls:   toolCalls,
		TotalTokens: s.totalTokenUsageUnsafe(),
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

// totalTokenUsageUnsafe returns total token usage without locking.
// Must be called with lock held.
func (s *ConversationState) totalTokenUsageUnsafe() TokenUsage {
	total := TokenUsage{}
	for _, usage := range s.tokenUsage {
		total.InputTokens += usage.InputTokens
		total.CachedInputTokens += usage.CachedInputTokens
		total.OutputTokens += usage.OutputTokens
		total.ReasoningOutputTokens += usage.ReasoningOutputTokens
		total.TotalTokens += usage.TotalTokens
	}
	return total
}

// Clear removes all messages and tool calls from the state.
// This method is thread-safe.
func (s *ConversationState) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = make([]Message, 0)
	s.toolCalls = make(map[string]*ToolCall)
	s.tokenUsage = make([]TokenUsage, 0)
	s.UpdatedAt = time.Now()
}

// SetSessionMetadata sets the session metadata.
// This method is thread-safe.
func (s *ConversationState) SetSessionMetadata(metadata *SessionMetadata) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessionMetadata = metadata
	s.UpdatedAt = time.Now()
}

// GetSessionMetadata returns a copy of the session metadata.
// This method is thread-safe.
func (s *ConversationState) GetSessionMetadata() *SessionMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.sessionMetadata == nil {
		return nil
	}

	// Return a copy to prevent external modifications
	metadata := *s.sessionMetadata
	return &metadata
}

// UpdatePlan updates the current plan/todo list.
// This method is thread-safe.
func (s *ConversationState) UpdatePlan(plan interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.plan = plan
	s.UpdatedAt = time.Now()
}

// GetPlan returns the current plan/todo list.
// This method is thread-safe.
func (s *ConversationState) GetPlan() interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.plan
}

// ClearPlan removes the current plan.
// This method is thread-safe.
func (s *ConversationState) ClearPlan() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.plan = nil
	s.UpdatedAt = time.Now()
}
