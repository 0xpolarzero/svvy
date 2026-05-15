package compaction

import (
	"fmt"
	"sort"

	"github.com/evmts/codex/codex-go/internal/client"
)

// Strategy defines the approach for compacting message history.
type Strategy string

const (
	// StrategyDropOldest removes oldest messages first while preserving policy rules
	StrategyDropOldest Strategy = "drop_oldest"

	// StrategySlidingWindow keeps a sliding window of recent messages
	StrategySlidingWindow Strategy = "sliding_window"

	// StrategyCompress uses LLM to summarize old messages
	StrategyCompress Strategy = "compress"

	// StrategyImportanceBased removes messages with lowest importance scores
	StrategyImportanceBased Strategy = "importance_based"
)

// Truncator handles token-based truncation of message history.
// It removes or reduces messages to fit within a token budget while
// respecting policy constraints.
type Truncator struct {
	// TokenCounter for counting message tokens
	TokenCounter TokenCounter

	// MaxTokens is the target token budget
	MaxTokens int

	// Strategy determines how messages are removed
	Strategy Strategy

	// Policy defines which messages must be preserved
	Policy *Policy

	// AllowPartialTruncation permits truncating individual messages
	AllowPartialTruncation bool
}

// TruncationResult contains the results of a truncation operation.
type TruncationResult struct {
	// Messages is the truncated message list
	Messages []client.Message

	// TokensRemoved is how many tokens were removed
	TokensRemoved int

	// TokensRemaining is the token count after truncation
	TokensRemaining int

	// MessagesRemoved is how many messages were removed
	MessagesRemoved int

	// Strategy used for this truncation
	Strategy Strategy
}

// Truncate reduces message history to fit within the token budget.
// Returns truncated messages or error if truncation fails.
func (t *Truncator) Truncate(messages []client.Message) ([]client.Message, error) {
	if len(messages) == 0 {
		return messages, nil
	}

	// Calculate current token usage
	currentTokens := t.countTotalTokens(messages)

	// If already under budget, no truncation needed
	if currentTokens <= t.MaxTokens {
		return messages, nil
	}

	// Apply truncation strategy
	var result []client.Message
	var err error

	switch t.Strategy {
	case StrategyDropOldest:
		result, err = t.truncateDropOldest(messages, currentTokens)
	case StrategySlidingWindow:
		result, err = t.truncateSlidingWindow(messages, currentTokens)
	case StrategyImportanceBased:
		result, err = t.truncateByImportance(messages, currentTokens)
	default:
		// Default to drop oldest
		result, err = t.truncateDropOldest(messages, currentTokens)
	}

	if err != nil {
		return nil, fmt.Errorf("truncation failed: %w", err)
	}

	return result, nil
}

// TruncateWithResult performs truncation and returns detailed results.
func (t *Truncator) TruncateWithResult(messages []client.Message) (*TruncationResult, error) {
	originalTokens := t.countTotalTokens(messages)
	originalCount := len(messages)

	truncated, err := t.Truncate(messages)
	if err != nil {
		return nil, err
	}

	remainingTokens := t.countTotalTokens(truncated)

	return &TruncationResult{
		Messages:        truncated,
		TokensRemoved:   originalTokens - remainingTokens,
		TokensRemaining: remainingTokens,
		MessagesRemoved: originalCount - len(truncated),
		Strategy:        t.Strategy,
	}, nil
}

// truncateDropOldest removes messages from oldest to newest until under budget.
func (t *Truncator) truncateDropOldest(messages []client.Message, currentTokens int) ([]client.Message, error) {
	// Get preserved message indices
	preserved := t.getPreservedIndices(messages)

	result := make([]client.Message, 0, len(messages))
	tokensUsed := 0
	budget := t.MaxTokens

	// First pass: add all preserved messages
	for i, msg := range messages {
		if preserved[i] {
			tokens := countMessageTokens(msg, t.TokenCounter)
			result = append(result, msg)
			tokensUsed += tokens
		}
	}

	// If preserved messages exceed budget, we have a problem
	if tokensUsed > budget {
		return t.handleBudgetExceeded(messages, preserved)
	}

	// Second pass: add non-preserved messages from newest to oldest
	for i := len(messages) - 1; i >= 0; i-- {
		if !preserved[i] {
			msg := messages[i]
			tokens := countMessageTokens(msg, t.TokenCounter)

			if tokensUsed+tokens <= budget {
				// Insert in correct position
				result = insertAtCorrectPosition(result, msg, i, messages)
				tokensUsed += tokens
			}
		}
	}

	return result, nil
}

// truncateSlidingWindow keeps only the most recent N turns that fit in budget.
func (t *Truncator) truncateSlidingWindow(messages []client.Message, currentTokens int) ([]client.Message, error) {
	result := make([]client.Message, 0, len(messages))
	tokensUsed := 0
	budget := t.MaxTokens

	// Always include system messages at the start
	systemMessages := make([]client.Message, 0)
	nonSystemMessages := make([]client.Message, 0)

	for _, msg := range messages {
		if msg.Role == "system" && t.Policy.PreserveSystemMessages {
			systemMessages = append(systemMessages, msg)
		} else {
			nonSystemMessages = append(nonSystemMessages, msg)
		}
	}

	// Add system messages
	for _, msg := range systemMessages {
		tokens := countMessageTokens(msg, t.TokenCounter)
		result = append(result, msg)
		tokensUsed += tokens
	}

	// Add recent messages from end until budget exhausted
	for i := len(nonSystemMessages) - 1; i >= 0; i-- {
		msg := nonSystemMessages[i]
		tokens := countMessageTokens(msg, t.TokenCounter)

		if tokensUsed+tokens <= budget {
			// Prepend to maintain order
			result = append([]client.Message{msg}, result...)
			tokensUsed += tokens
		} else {
			break
		}
	}

	return result, nil
}

// truncateByImportance removes messages with lowest importance scores first.
func (t *Truncator) truncateByImportance(messages []client.Message, currentTokens int) ([]client.Message, error) {
	positions := t.Policy.CalculatePositions(messages)

	// Create scored messages
	type scoredMessage struct {
		message client.Message
		score   float64
		index   int
		tokens  int
	}

	scored := make([]scoredMessage, 0, len(messages))
	for i, msg := range messages {
		sm := scoredMessage{
			message: msg,
			score:   t.Policy.GetImportanceScore(msg),
			index:   i,
			tokens:  countMessageTokens(msg, t.TokenCounter),
		}

		// Boost score for preserved messages
		if t.Policy.ShouldPreserve(msg, positions[i]) {
			sm.score = 1.0
		}

		scored = append(scored, sm)
	}

	// Sort by score (highest first)
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			// If scores equal, prefer more recent (higher index)
			return scored[i].index > scored[j].index
		}
		return scored[i].score > scored[j].score
	})

	// Take messages until budget exhausted
	result := make([]client.Message, 0, len(messages))
	tokensUsed := 0
	budget := t.MaxTokens

	for _, sm := range scored {
		if tokensUsed+sm.tokens <= budget {
			result = append(result, sm.message)
			tokensUsed += sm.tokens
		}
	}

	// Sort result by original index to maintain order
	sort.Slice(result, func(i, j int) bool {
		return indexOf(messages, result[i]) < indexOf(messages, result[j])
	})

	return result, nil
}

// getPreservedIndices returns a map of message indices that must be preserved.
func (t *Truncator) getPreservedIndices(messages []client.Message) map[int]bool {
	preserved := make(map[int]bool)
	positions := t.Policy.CalculatePositions(messages)

	for i, msg := range messages {
		if t.Policy.ShouldPreserve(msg, positions[i]) {
			preserved[i] = true
		}
	}

	return preserved
}

// handleBudgetExceeded handles cases where preserved messages exceed budget.
// This is a critical situation - we try to truncate system messages if allowed.
func (t *Truncator) handleBudgetExceeded(messages []client.Message, preserved map[int]bool) ([]client.Message, error) {
	// If partial truncation is allowed, try to truncate long messages
	if t.AllowPartialTruncation {
		return t.truncatePartially(messages, preserved)
	}

	// Otherwise, keep only system messages and most recent message
	result := make([]client.Message, 0)

	// Add system messages
	for i, msg := range messages {
		if msg.Role == "system" && preserved[i] {
			result = append(result, msg)
		}
	}

	// Add last message if it fits
	if len(messages) > 0 {
		lastMsg := messages[len(messages)-1]
		tokens := t.countTotalTokens(result) + countMessageTokens(lastMsg, t.TokenCounter)
		if tokens <= t.MaxTokens {
			result = append(result, lastMsg)
		}
	}

	return result, nil
}

// truncatePartially truncates individual messages to fit budget.
func (t *Truncator) truncatePartially(messages []client.Message, preserved map[int]bool) ([]client.Message, error) {
	result := make([]client.Message, 0, len(messages))
	tokensUsed := 0
	budget := t.MaxTokens

	for i, msg := range messages {
		if !preserved[i] {
			continue
		}

		tokens := countMessageTokens(msg, t.TokenCounter)
		remaining := budget - tokensUsed

		if tokens <= remaining {
			result = append(result, msg)
			tokensUsed += tokens
		} else if remaining > 10 { // Only truncate if reasonable space left
			truncated := t.truncateMessage(msg, remaining)
			result = append(result, truncated)
			tokensUsed += countMessageTokens(truncated, t.TokenCounter)
		}
	}

	return result, nil
}

// truncateMessage truncates a single message to fit token budget.
func (t *Truncator) truncateMessage(msg client.Message, maxTokens int) client.Message {
	if content, ok := msg.Content.(string); ok {
		// Simple truncation: keep first portion of content
		// Estimate characters per token (rough average is 4)
		maxChars := maxTokens * 4

		if len(content) > maxChars {
			truncated := msg
			truncated.Content = content[:maxChars] + "... [truncated]"
			return truncated
		}
	}

	return msg
}

// countTotalTokens sums token counts across all messages.
func (t *Truncator) countTotalTokens(messages []client.Message) int {
	total := 0
	for _, msg := range messages {
		total += countMessageTokens(msg, t.TokenCounter)
	}
	return total
}

// EstimateTokensToRemove calculates how many tokens need removal.
func (t *Truncator) EstimateTokensToRemove(messages []client.Message) int {
	current := t.countTotalTokens(messages)
	if current <= t.MaxTokens {
		return 0
	}
	return current - t.MaxTokens
}

// CanTruncate checks if truncation is possible with current policy.
func (t *Truncator) CanTruncate(messages []client.Message) bool {
	preserved := t.getPreservedIndices(messages)
	preservedTokens := 0

	for i, msg := range messages {
		if preserved[i] {
			preservedTokens += countMessageTokens(msg, t.TokenCounter)
		}
	}

	// Can truncate if preserved messages fit in budget
	return preservedTokens <= t.MaxTokens
}

// Helper functions

// insertAtCorrectPosition inserts a message at its original position.
func insertAtCorrectPosition(result []client.Message, msg client.Message, originalIdx int, original []client.Message) []client.Message {
	// Find correct insertion point based on original order
	insertIdx := 0
	for i, m := range result {
		if indexOf(original, m) < originalIdx {
			insertIdx = i + 1
		}
	}

	// Insert at position
	result = append(result, client.Message{})
	copy(result[insertIdx+1:], result[insertIdx:])
	result[insertIdx] = msg

	return result
}

// indexOf finds the index of a message in the original slice.
func indexOf(messages []client.Message, target client.Message) int {
	for i, msg := range messages {
		if messagesEqual(msg, target) {
			return i
		}
	}
	return -1
}

// messagesEqual checks if two messages are equal.
func messagesEqual(a, b client.Message) bool {
	if a.Role != b.Role {
		return false
	}

	// Compare content
	aContent, aOk := a.Content.(string)
	bContent, bOk := b.Content.(string)

	if aOk && bOk {
		return aContent == bContent
	}

	// For non-string content, use pointer equality (not perfect but sufficient)
	return &a == &b
}

// GetStrategy returns the current truncation strategy.
func (t *Truncator) GetStrategy() Strategy {
	return t.Strategy
}

// SetStrategy updates the truncation strategy.
func (t *Truncator) SetStrategy(strategy Strategy) {
	t.Strategy = strategy
}

// GetTokenBudget returns the current token budget.
func (t *Truncator) GetTokenBudget() int {
	return t.MaxTokens
}

// SetTokenBudget updates the token budget.
func (t *Truncator) SetTokenBudget(tokens int) {
	t.MaxTokens = tokens
}
