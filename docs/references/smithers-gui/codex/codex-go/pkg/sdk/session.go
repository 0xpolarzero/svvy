package sdk

import (
	"context"
	"fmt"
	"sync"
)

// Session represents a conversation session with the AI.
// It maintains the conversation history and handles message submission.
type Session struct {
	sdk              *SDK
	systemPrompt     string
	streaming        bool
	onToolApproval   func(toolName, operation string) bool
	approvalPolicy   string
	sandboxPolicy    string
	workingDirectory string
	model            string
	conversationID   string
	messages         []*Message
	mu               sync.RWMutex
	closed           bool
}

// Message represents a single message in the conversation.
type Message struct {
	// Role is the message role: "system", "user", "assistant", "tool"
	Role string

	// Content is the message content
	Content string

	// ToolCalls contains any tool invocations (for assistant messages)
	ToolCalls []ToolCall

	// ToolCallID is the ID of the tool call this message responds to (for tool messages)
	ToolCallID string
}

// ToolCall represents a tool invocation by the assistant.
type ToolCall struct {
	// ID is the unique identifier for this tool call
	ID string

	// Name is the name of the tool being called
	Name string

	// Arguments are the JSON-encoded arguments for the tool
	Arguments string

	// Result is the result of executing the tool
	Result string

	// Error is any error that occurred during tool execution
	Error string
}

// Response represents the AI's response to a user message.
type Response struct {
	// Content is the text response from the AI
	Content string

	// ToolCalls contains any tools the AI wants to execute
	ToolCalls []ToolCall

	// FinishReason indicates why the response ended
	// Values: "stop", "length", "tool_calls", "content_filter"
	FinishReason string

	// TokenUsage contains token consumption information
	TokenUsage TokenUsage
}

// TokenUsage tracks token consumption for a request/response.
type TokenUsage struct {
	// InputTokens is the number of tokens in the prompt
	InputTokens int64

	// OutputTokens is the number of tokens in the completion
	OutputTokens int64

	// TotalTokens is the sum of input and output tokens
	TotalTokens int64
}

// StreamEvent represents a streaming response event.
type StreamEvent struct {
	// Type indicates the event type
	Type string

	// Delta contains incremental content updates
	Delta string

	// ToolCall contains incremental tool call information
	ToolCall *ToolCall

	// Done indicates this is the final event
	Done bool

	// Error contains any error that occurred
	Error error

	// Response contains the final response (only when Done=true)
	Response *Response
}

// ID returns the session's unique identifier.
func (s *Session) ID() string {
	return s.conversationID
}

// IsClosed returns whether the session has been closed.
func (s *Session) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

// History returns a copy of the conversation history.
func (s *Session) History() []*Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a deep copy to prevent external modification
	history := make([]*Message, len(s.messages))
	for i, msg := range s.messages {
		// Create a copy of each message
		msgCopy := &Message{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		// Copy tool calls if present
		if len(msg.ToolCalls) > 0 {
			msgCopy.ToolCalls = make([]ToolCall, len(msg.ToolCalls))
			copy(msgCopy.ToolCalls, msg.ToolCalls)
		}
		history[i] = msgCopy
	}
	return history
}

// Submit sends a message and waits for the complete response.
// This is a blocking call suitable for non-streaming use cases.
//
// The method:
// 1. Validates the message is not empty
// 2. Creates a protocol.OpUserTurn with the message
// 3. Submits it to the conversation manager via SubmitOp()
// 4. Waits for completion events from the manager
// 5. Extracts the assistant's response and token usage
// 6. Returns the complete response
//
// Context cancellation is respected throughout the process.
func (s *Session) Submit(ctx context.Context, message string) (*Response, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, fmt.Errorf("session is closed")
	}
	if s.streaming {
		s.mu.Unlock()
		return nil, fmt.Errorf("session is configured for streaming; use SubmitStream instead")
	}
	s.mu.Unlock()

	// Validate message
	if message == "" {
		return nil, fmt.Errorf("message cannot be empty")
	}

	// Add user message to history
	userMsg := &Message{
		Role:    "user",
		Content: message,
	}
	s.addMessage(userMsg)

	// Submit to manager and collect streaming response
	streamCh, err := s.submitInternal(ctx, message)
	if err != nil {
		return nil, err
	}

	// Collect all events and build final response
	var response Response
	var contentBuilder string

	for event := range streamCh {
		if event.Error != nil {
			return nil, event.Error
		}

		if event.Delta != "" {
			contentBuilder += event.Delta
		}

		if event.Done && event.Response != nil {
			response = *event.Response
			// Use accumulated content if response content is empty
			if response.Content == "" {
				response.Content = contentBuilder
			}
		}
	}

	// Add assistant message to history
	assistantMsg := &Message{
		Role:    "assistant",
		Content: response.Content,
	}
	s.addMessage(assistantMsg)

	return &response, nil
}

// SubmitStream sends a message and returns a channel for streaming responses.
// The channel will be closed when the response is complete or an error occurs.
func (s *Session) SubmitStream(ctx context.Context, message string) (<-chan StreamEvent, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, fmt.Errorf("session is closed")
	}
	if !s.streaming {
		s.mu.Unlock()
		return nil, fmt.Errorf("session is not configured for streaming; use Submit instead")
	}
	s.mu.Unlock()

	// Add user message to history
	userMsg := &Message{
		Role:    "user",
		Content: message,
	}
	s.addMessage(userMsg)

	// Use shared internal implementation
	return s.submitInternal(ctx, message)
}

// close closes the session and releases resources.
func (s *Session) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("session already closed")
	}

	s.closed = true
	return nil
}

// addMessage adds a message to the session history.
func (s *Session) addMessage(msg *Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
}
