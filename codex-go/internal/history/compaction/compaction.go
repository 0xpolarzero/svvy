// Package compaction provides history compaction for managing conversation context.
//
// This package implements token-based history compaction with multiple strategies:
//   - Token-based truncation (drop oldest, sliding window, importance-based)
//   - LLM-based summarization for old messages
//   - Policy-driven preservation of important messages
//   - Async compaction support
//
// Example usage:
//
//	compactor := compaction.NewCompactor(compaction.CompactorConfig{
//	    Client:       apiClient,
//	    TokenCounter: tokencount.NewDefaultCounter(),
//	    MaxTokens:    100000,
//	    Strategy:     compaction.StrategyCompress,
//	    Policy:       compaction.NewDefaultPolicy(),
//	})
//
//	compactedMessages, err := compactor.Compact(ctx, messages)
package compaction

import (
	"context"
	"fmt"
	"sync"

	"github.com/evmts/codex/codex-go/internal/client"
)

// Compactor is the main coordinator for history compaction operations.
// It combines truncation, summarization, and policy enforcement to
// reduce message history while preserving important context.
type Compactor struct {
	// Client for API calls (summarization)
	Client client.Client

	// TokenCounter for counting message tokens
	TokenCounter TokenCounter

	// MaxTokens is the target token budget
	MaxTokens int

	// AutoCompactThreshold triggers automatic compaction
	// When 0, auto-compaction is disabled
	AutoCompactThreshold int

	// Strategy determines the compaction approach
	Strategy Strategy

	// Policy defines preservation rules
	Policy *Policy

	// Async enables background compaction
	Async bool

	// summarizer for LLM-based compression
	summarizer *Summarizer

	// truncator for token-based truncation
	truncator *Truncator

	// mu protects concurrent access
	mu sync.RWMutex

	// stats tracks compaction metrics
	stats CompactionStats
}

// CompactorConfig holds configuration for creating a Compactor.
type CompactorConfig struct {
	// Client for API calls
	Client client.Client

	// TokenCounter for counting tokens
	TokenCounter TokenCounter

	// MaxTokens is the target token budget
	MaxTokens int

	// AutoCompactThreshold for automatic compaction trigger
	// Set to 0 to disable auto-compaction
	AutoCompactThreshold int

	// Strategy for compaction
	Strategy Strategy

	// Policy for message preservation
	Policy *Policy

	// Async enables background compaction
	Async bool

	// SummarizerConfig for LLM summarization
	SummarizerConfig *SummarizerConfig

	// AllowPartialTruncation permits truncating individual messages
	AllowPartialTruncation bool
}

// SummarizerConfig holds summarizer-specific configuration.
type SummarizerConfig struct {
	// BatchSize for batched summarization
	BatchSize int

	// Model override for summarization
	Model string

	// SystemPrompt for summarization
	SystemPrompt string

	// MaxSummaryTokens limits summary length
	MaxSummaryTokens int
}

// CompactionStats tracks metrics about compaction operations.
type CompactionStats struct {
	// TotalCompactions is the number of compaction operations
	TotalCompactions int

	// TotalTokensSaved is the cumulative tokens removed
	TotalTokensSaved int

	// TotalMessagesSaved is the cumulative messages removed
	TotalMessagesSaved int

	// LastCompactionTokens is tokens in last compaction
	LastCompactionTokens int

	// StrategyUsage counts usage of each strategy
	StrategyUsage map[Strategy]int
}

// NewCompactor creates a new compactor with the given configuration.
func NewCompactor(config CompactorConfig) *Compactor {
	if config.Policy == nil {
		config.Policy = NewDefaultPolicy()
	}

	c := &Compactor{
		Client:               config.Client,
		TokenCounter:         config.TokenCounter,
		MaxTokens:            config.MaxTokens,
		AutoCompactThreshold: config.AutoCompactThreshold,
		Strategy:             config.Strategy,
		Policy:               config.Policy,
		Async:                config.Async,
		stats: CompactionStats{
			StrategyUsage: make(map[Strategy]int),
		},
	}

	// Initialize summarizer if client provided
	if config.Client != nil {
		c.summarizer = NewSummarizer(config.Client)
		c.summarizer.Policy = config.Policy

		if config.SummarizerConfig != nil {
			if config.SummarizerConfig.BatchSize > 0 {
				c.summarizer.BatchSize = config.SummarizerConfig.BatchSize
			}
			if config.SummarizerConfig.Model != "" {
				c.summarizer.Model = config.SummarizerConfig.Model
			}
			if config.SummarizerConfig.SystemPrompt != "" {
				c.summarizer.SystemPrompt = config.SummarizerConfig.SystemPrompt
			}
			if config.SummarizerConfig.MaxSummaryTokens > 0 {
				c.summarizer.MaxSummaryTokens = config.SummarizerConfig.MaxSummaryTokens
			}
		}
	}

	// Initialize truncator
	c.truncator = &Truncator{
		TokenCounter:           config.TokenCounter,
		MaxTokens:              config.MaxTokens,
		Strategy:               config.Strategy,
		Policy:                 config.Policy,
		AllowPartialTruncation: config.AllowPartialTruncation,
	}

	return c
}

// Compact performs history compaction on the given messages.
// Returns compacted messages or error if compaction fails.
func (c *Compactor) Compact(ctx context.Context, messages []client.Message) ([]client.Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(messages) == 0 {
		return messages, nil
	}

	// Check if compaction is needed
	currentTokens := c.countTokens(messages)
	if currentTokens <= c.MaxTokens {
		return messages, nil
	}

	// Track statistics
	c.stats.TotalCompactions++
	c.stats.StrategyUsage[c.Strategy]++

	// Apply compaction strategy
	var result []client.Message
	var err error

	switch c.Strategy {
	case StrategyDropOldest:
		result, err = c.compactDropOldest(ctx, messages)
	case StrategySlidingWindow:
		result, err = c.compactSlidingWindow(ctx, messages)
	case StrategyCompress:
		result, err = c.compactCompress(ctx, messages)
	case StrategyImportanceBased:
		result, err = c.compactImportanceBased(ctx, messages)
	default:
		result, err = c.compactDropOldest(ctx, messages)
	}

	if err != nil {
		return nil, fmt.Errorf("compaction failed: %w", err)
	}

	// Update statistics
	resultTokens := c.countTokens(result)
	c.stats.TotalTokensSaved += (currentTokens - resultTokens)
	c.stats.TotalMessagesSaved += (len(messages) - len(result))
	c.stats.LastCompactionTokens = resultTokens

	return result, nil
}

// CompactAsync performs compaction asynchronously and returns a channel for results.
func (c *Compactor) CompactAsync(ctx context.Context, messages []client.Message) <-chan CompactionResult {
	resultChan := make(chan CompactionResult, 1)

	go func() {
		defer close(resultChan)

		compacted, err := c.Compact(ctx, messages)
		resultChan <- CompactionResult{
			Messages: compacted,
			Error:    err,
		}
	}()

	return resultChan
}

// CompactionResult holds the result of an async compaction.
type CompactionResult struct {
	Messages []client.Message
	Error    error
}

// ShouldCompact determines if messages exceed the auto-compact threshold.
func (c *Compactor) ShouldCompact(messages []client.Message) bool {
	if c.AutoCompactThreshold == 0 {
		return false
	}

	currentTokens := c.countTokens(messages)
	return currentTokens > c.AutoCompactThreshold
}

// compactDropOldest removes oldest messages to fit budget.
func (c *Compactor) compactDropOldest(ctx context.Context, messages []client.Message) ([]client.Message, error) {
	return c.truncator.Truncate(messages)
}

// compactSlidingWindow keeps only recent messages that fit in budget.
func (c *Compactor) compactSlidingWindow(ctx context.Context, messages []client.Message) ([]client.Message, error) {
	// Temporarily switch truncator strategy
	originalStrategy := c.truncator.Strategy
	c.truncator.Strategy = StrategySlidingWindow
	defer func() { c.truncator.Strategy = originalStrategy }()

	return c.truncator.Truncate(messages)
}

// compactImportanceBased removes least important messages first.
func (c *Compactor) compactImportanceBased(ctx context.Context, messages []client.Message) ([]client.Message, error) {
	// Temporarily switch truncator strategy
	originalStrategy := c.truncator.Strategy
	c.truncator.Strategy = StrategyImportanceBased
	defer func() { c.truncator.Strategy = originalStrategy }()

	return c.truncator.Truncate(messages)
}

// compactCompress uses LLM to summarize old messages.
func (c *Compactor) compactCompress(ctx context.Context, messages []client.Message) ([]client.Message, error) {
	if c.summarizer == nil {
		return nil, fmt.Errorf("compress strategy requires a client for summarization")
	}

	// Separate preserved and summarizable messages
	preserved := c.Policy.PreserveMessages(messages)
	preservedMap := make(map[int]bool)

	for _, pMsg := range preserved {
		for i, msg := range messages {
			if messagesEqual(msg, pMsg) {
				preservedMap[i] = true
				break
			}
		}
	}

	// Split messages into preserved and to-summarize
	toSummarize := make([]client.Message, 0)
	result := make([]client.Message, 0, len(messages))

	// First, add system messages
	for i, msg := range messages {
		if msg.Role == "system" && preservedMap[i] {
			result = append(result, msg)
		}
	}

	// Collect messages to summarize
	for i, msg := range messages {
		if !preservedMap[i] && c.Policy.CanSummarize(msg) {
			toSummarize = append(toSummarize, msg)
		}
	}

	// Summarize if we have messages
	if len(toSummarize) > 0 {
		summaryMsg, err := c.summarizer.SummarizeToMessage(ctx, toSummarize)
		if err != nil {
			// Fall back to truncation if summarization fails
			return c.compactDropOldest(ctx, messages)
		}
		result = append(result, summaryMsg)
	}

	// Add preserved non-system messages
	for i, msg := range messages {
		if preservedMap[i] && msg.Role != "system" {
			result = append(result, msg)
		}
	}

	// If still over budget, apply truncation to summary
	if c.countTokens(result) > c.MaxTokens {
		return c.truncator.Truncate(result)
	}

	return result, nil
}

// CompactIfNeeded performs compaction only if threshold is exceeded.
func (c *Compactor) CompactIfNeeded(ctx context.Context, messages []client.Message) ([]client.Message, bool, error) {
	if !c.ShouldCompact(messages) {
		return messages, false, nil
	}

	compacted, err := c.Compact(ctx, messages)
	if err != nil {
		return nil, false, err
	}

	return compacted, true, nil
}

// GetStats returns current compaction statistics.
func (c *Compactor) GetStats() CompactionStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to avoid race conditions
	stats := c.stats
	stats.StrategyUsage = make(map[Strategy]int, len(c.stats.StrategyUsage))
	for k, v := range c.stats.StrategyUsage {
		stats.StrategyUsage[k] = v
	}

	return stats
}

// ResetStats clears compaction statistics.
func (c *Compactor) ResetStats() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stats = CompactionStats{
		StrategyUsage: make(map[Strategy]int),
	}
}

// EstimateCompaction estimates the result of compaction without performing it.
type CompactionEstimate struct {
	// CurrentTokens is the current token count
	CurrentTokens int

	// EstimatedTokens is the expected token count after compaction
	EstimatedTokens int

	// TokenSavings is the estimated reduction
	TokenSavings int

	// CurrentMessages is the current message count
	CurrentMessages int

	// EstimatedMessages is the expected message count after compaction
	EstimatedMessages int

	// WillCompact indicates if compaction is needed
	WillCompact bool

	// Strategy that would be used
	Strategy Strategy
}

// Estimate calculates the expected result of compaction without executing it.
func (c *Compactor) Estimate(messages []client.Message) *CompactionEstimate {
	c.mu.RLock()
	defer c.mu.RUnlock()

	currentTokens := c.countTokens(messages)

	estimate := &CompactionEstimate{
		CurrentTokens:   currentTokens,
		CurrentMessages: len(messages),
		WillCompact:     currentTokens > c.MaxTokens,
		Strategy:        c.Strategy,
	}

	if !estimate.WillCompact {
		estimate.EstimatedTokens = currentTokens
		estimate.EstimatedMessages = len(messages)
		estimate.TokenSavings = 0
		return estimate
	}

	// Estimate based on strategy
	switch c.Strategy {
	case StrategyDropOldest, StrategySlidingWindow, StrategyImportanceBased:
		// Estimate truncation
		canRemove, mustKeep := c.Policy.EstimateTokenSavings(messages, c.TokenCounter)
		estimate.EstimatedTokens = mustKeep
		estimate.TokenSavings = canRemove

		// Estimate message count (rough approximation)
		if mustKeep > 0 {
			keepRatio := float64(mustKeep) / float64(currentTokens)
			estimate.EstimatedMessages = int(float64(len(messages)) * keepRatio)
		}

	case StrategyCompress:
		// Estimate summarization
		if c.summarizer != nil {
			summaryLength := c.summarizer.EstimateSummaryLength(messages, c.TokenCounter)
			_, mustKeep := c.Policy.EstimateTokenSavings(messages, c.TokenCounter)
			estimate.EstimatedTokens = mustKeep + summaryLength
			estimate.TokenSavings = currentTokens - estimate.EstimatedTokens

			// Compressed to fewer messages (preserved + 1 summary)
			preserved := c.Policy.PreserveMessages(messages)
			estimate.EstimatedMessages = len(preserved) + 1
		}
	}

	return estimate
}

// countTokens counts total tokens in messages.
func (c *Compactor) countTokens(messages []client.Message) int {
	total := 0
	for _, msg := range messages {
		total += countMessageTokens(msg, c.TokenCounter)
	}
	return total
}

// SetMaxTokens updates the maximum token budget.
func (c *Compactor) SetMaxTokens(maxTokens int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.MaxTokens = maxTokens
	c.truncator.MaxTokens = maxTokens
}

// GetMaxTokens returns the current token budget.
func (c *Compactor) GetMaxTokens() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.MaxTokens
}

// SetStrategy updates the compaction strategy.
func (c *Compactor) SetStrategy(strategy Strategy) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Strategy = strategy
	c.truncator.Strategy = strategy
}

// GetStrategy returns the current compaction strategy.
func (c *Compactor) GetStrategy() Strategy {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.Strategy
}

// SetPolicy updates the compaction policy.
func (c *Compactor) SetPolicy(policy *Policy) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Policy = policy
	c.truncator.Policy = policy
	if c.summarizer != nil {
		c.summarizer.Policy = policy
	}
}

// GetPolicy returns the current compaction policy.
func (c *Compactor) GetPolicy() *Policy {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.Policy
}

// SetAutoCompactThreshold updates the auto-compaction threshold.
func (c *Compactor) SetAutoCompactThreshold(threshold int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.AutoCompactThreshold = threshold
}

// GetAutoCompactThreshold returns the current auto-compaction threshold.
func (c *Compactor) GetAutoCompactThreshold() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.AutoCompactThreshold
}

// ValidateConfig checks if the compactor configuration is valid.
func ValidateConfig(config CompactorConfig) error {
	if config.TokenCounter == nil {
		return fmt.Errorf("TokenCounter is required")
	}

	if config.MaxTokens <= 0 {
		return fmt.Errorf("MaxTokens must be positive")
	}

	if config.Strategy == StrategyCompress && config.Client == nil {
		return fmt.Errorf("Client is required for compress strategy")
	}

	if config.AutoCompactThreshold < 0 {
		return fmt.Errorf("AutoCompactThreshold cannot be negative")
	}

	if config.AutoCompactThreshold > 0 && config.AutoCompactThreshold <= config.MaxTokens {
		return fmt.Errorf("AutoCompactThreshold must be greater than MaxTokens")
	}

	return nil
}
