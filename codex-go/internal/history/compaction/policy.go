package compaction

import (
	"github.com/evmts/codex/codex-go/internal/client"
)

// Policy defines rules for which messages to preserve during compaction.
// It implements importance scoring and preservation rules to ensure critical
// context is maintained while reducing token usage.
type Policy struct {
	// PreserveSystemMessages ensures system prompts are never removed
	PreserveSystemMessages bool

	// PreserveRecentTurns keeps the N most recent conversation turns
	// A turn is a user message + assistant response pair
	PreserveRecentTurns int

	// PreserveToolCalls ensures messages with tool calls are kept
	PreserveToolCalls bool

	// MinImportanceScore is the threshold for keeping messages (0.0 to 1.0)
	MinImportanceScore float64

	// PreserveReasoningContent keeps assistant reasoning/thinking
	PreserveReasoningContent bool
}

// NewDefaultPolicy creates a policy with sensible defaults for most use cases.
func NewDefaultPolicy() *Policy {
	return &Policy{
		PreserveSystemMessages:   true,
		PreserveRecentTurns:      3, // Keep last 3 turns (6 messages)
		PreserveToolCalls:        true,
		MinImportanceScore:       0.3,
		PreserveReasoningContent: false, // Reasoning can be large, drop by default
	}
}

// NewAggressivePolicy creates a policy that removes more aggressively.
func NewAggressivePolicy() *Policy {
	return &Policy{
		PreserveSystemMessages:   true,
		PreserveRecentTurns:      1, // Only keep last turn
		PreserveToolCalls:        false,
		MinImportanceScore:       0.5,
		PreserveReasoningContent: false,
	}
}

// NewConservativePolicy creates a policy that preserves more history.
func NewConservativePolicy() *Policy {
	return &Policy{
		PreserveSystemMessages:   true,
		PreserveRecentTurns:      10, // Keep last 10 turns
		PreserveToolCalls:        true,
		MinImportanceScore:       0.1,
		PreserveReasoningContent: true,
	}
}

// PreserveMessages returns messages that should be preserved according to policy.
// This is used to identify "anchor" messages that must not be removed.
func (p *Policy) PreserveMessages(messages []client.Message) []client.Message {
	if len(messages) == 0 {
		return messages
	}

	preserved := make([]client.Message, 0, len(messages))

	// Always preserve system messages if configured
	if p.PreserveSystemMessages {
		for _, msg := range messages {
			if msg.Role == "system" {
				preserved = append(preserved, msg)
			}
		}
	}

	// Preserve recent turns
	if p.PreserveRecentTurns > 0 {
		recentMessages := p.getRecentMessages(messages, p.PreserveRecentTurns)
		preserved = append(preserved, recentMessages...)
	}

	// Preserve tool calls if configured
	if p.PreserveToolCalls {
		for _, msg := range messages {
			if len(msg.ToolCalls) > 0 || msg.ToolCallID != "" {
				// Check if not already in preserved list
				if !containsMessage(preserved, msg) {
					preserved = append(preserved, msg)
				}
			}
		}
	}

	return preserved
}

// GetImportanceScore calculates an importance score for a message (0.0 to 1.0).
// Higher scores indicate messages that should be preserved during compaction.
func (p *Policy) GetImportanceScore(msg client.Message) float64 {
	score := 0.5 // Base score for regular messages

	// System messages have maximum importance
	if msg.Role == "system" {
		return 1.0
	}

	// Tool-related messages have high importance
	if len(msg.ToolCalls) > 0 {
		score = 0.9
	}
	if msg.ToolCallID != "" {
		score = 0.85
	}

	// Messages with reasoning content
	if msg.Reasoning != "" {
		if p.PreserveReasoningContent {
			score = 0.8
		} else {
			score = 0.4 // Lower score if we don't preserve reasoning
		}
	}

	// User messages slightly more important than assistant (they define intent)
	if msg.Role == "user" {
		score += 0.1
	}

	// Long messages may contain important context
	if content, ok := msg.Content.(string); ok {
		if len(content) > 500 {
			score += 0.05
		}
	}

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// ShouldPreserve determines if a message should be preserved based on policy.
func (p *Policy) ShouldPreserve(msg client.Message, position MessagePosition) bool {
	// System messages always preserved if configured
	if p.PreserveSystemMessages && msg.Role == "system" {
		return true
	}

	// Recent messages always preserved if configured
	if position.IsRecent && position.TurnsFromEnd <= p.PreserveRecentTurns {
		return true
	}

	// Tool-related messages preserved if configured
	if p.PreserveToolCalls && (len(msg.ToolCalls) > 0 || msg.ToolCallID != "") {
		return true
	}

	// Check importance score
	score := p.GetImportanceScore(msg)
	return score >= p.MinImportanceScore
}

// CanSummarize determines if a message is suitable for summarization.
// Some messages (like tool calls) need exact content and can't be summarized.
func (p *Policy) CanSummarize(msg client.Message) bool {
	// System messages should not be summarized
	if msg.Role == "system" {
		return false
	}

	// Tool calls need exact structure
	if len(msg.ToolCalls) > 0 || msg.ToolCallID != "" {
		return false
	}

	// Only text content can be summarized
	if _, ok := msg.Content.(string); !ok {
		return false
	}

	return true
}

// MessagePosition provides context about a message's position in the conversation.
type MessagePosition struct {
	// Index is the position in the message array
	Index int

	// TurnsFromEnd is how many turns from the end (0 = most recent)
	TurnsFromEnd int

	// IsRecent indicates if this is in the recent message window
	IsRecent bool

	// IsSystemMessage indicates if this is a system message
	IsSystemMessage bool

	// PairIndex is the turn pair this belongs to (user+assistant)
	PairIndex int
}

// getRecentMessages extracts the most recent N turns from messages.
// A turn is considered a user message followed by assistant response.
func (p *Policy) getRecentMessages(messages []client.Message, turns int) []client.Message {
	if len(messages) == 0 || turns == 0 {
		return nil
	}

	// Count turns from the end
	turnCount := 0
	startIdx := len(messages)

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]

		// Skip system messages in turn counting
		if msg.Role == "system" {
			continue
		}

		// Count user messages as turn boundaries
		if msg.Role == "user" {
			turnCount++
			if turnCount > turns {
				startIdx = i + 1
				break
			}
		}
	}

	if startIdx >= len(messages) {
		return nil
	}

	// Extract recent messages (excluding system messages already preserved)
	recent := make([]client.Message, 0, len(messages)-startIdx)
	for i := startIdx; i < len(messages); i++ {
		if messages[i].Role != "system" || !p.PreserveSystemMessages {
			recent = append(recent, messages[i])
		}
	}

	return recent
}

// CalculatePositions calculates position metadata for all messages.
func (p *Policy) CalculatePositions(messages []client.Message) []MessagePosition {
	positions := make([]MessagePosition, len(messages))

	// Count total turns
	turnCount := 0
	for _, msg := range messages {
		if msg.Role == "user" {
			turnCount++
		}
	}

	currentTurn := 0
	for i, msg := range messages {
		pos := MessagePosition{
			Index:           i,
			IsSystemMessage: msg.Role == "system",
		}

		if msg.Role == "user" {
			currentTurn++
		}

		pos.PairIndex = currentTurn
		pos.TurnsFromEnd = turnCount - currentTurn
		pos.IsRecent = pos.TurnsFromEnd <= p.PreserveRecentTurns

		positions[i] = pos
	}

	return positions
}

// containsMessage checks if a message is already in the slice.
// Uses pointer comparison and role/content matching.
func containsMessage(messages []client.Message, target client.Message) bool {
	for _, msg := range messages {
		// Simple equality check based on role and content
		if msg.Role == target.Role {
			// Compare content if both are strings
			msgContent, msgOk := msg.Content.(string)
			targetContent, targetOk := target.Content.(string)

			if msgOk && targetOk && msgContent == targetContent {
				return true
			}

			// Compare tool call IDs if present
			if msg.ToolCallID != "" && msg.ToolCallID == target.ToolCallID {
				return true
			}

			// Compare tool calls if present
			if len(msg.ToolCalls) > 0 && len(target.ToolCalls) > 0 {
				if msg.ToolCalls[0].ID == target.ToolCalls[0].ID {
					return true
				}
			}
		}
	}
	return false
}

// EstimateTokenSavings estimates how many tokens could be saved by compaction.
func (p *Policy) EstimateTokenSavings(messages []client.Message, counter TokenCounter) (canRemove, mustKeep int) {
	positions := p.CalculatePositions(messages)

	for i, msg := range messages {
		tokens := countMessageTokens(msg, counter)

		if p.ShouldPreserve(msg, positions[i]) {
			mustKeep += tokens
		} else {
			canRemove += tokens
		}
	}

	return canRemove, mustKeep
}

// TokenCounter is an interface for counting tokens in text.
// This matches the tokencount package interface.
type TokenCounter interface {
	CountTokens(text string) int
}

// countMessageTokens counts tokens in a message, handling different content types.
func countMessageTokens(msg client.Message, counter TokenCounter) int {
	tokens := 0

	// Count role tokens (approximately 1 token per role)
	tokens += 1

	// Count content tokens
	switch content := msg.Content.(type) {
	case string:
		tokens += counter.CountTokens(content)
	case []client.ContentItem:
		for _, item := range content {
			tokens += counter.CountTokens(item.Text)
		}
	}

	// Count reasoning tokens
	if msg.Reasoning != "" {
		tokens += counter.CountTokens(msg.Reasoning)
	}

	// Tool calls add overhead
	if len(msg.ToolCalls) > 0 {
		tokens += len(msg.ToolCalls) * 10 // Approximate overhead per tool call
		for _, tc := range msg.ToolCalls {
			if tc.Function != nil {
				tokens += counter.CountTokens(tc.Function.Arguments)
			}
		}
	}

	return tokens
}
