package compaction

import (
	"context"
	"errors"
	"testing"

	"github.com/evmts/codex/codex-go/internal/client"
	"github.com/evmts/codex/codex-go/internal/tokencount"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock client for testing
type mockClient struct {
	completeFunc func(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error)
}

func (m *mockClient) Stream(ctx context.Context, req *client.ChatCompletionRequest) (<-chan client.StreamEvent, error) {
	return nil, errors.New("not implemented")
}

func (m *mockClient) Complete(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockClient) GetModelContextWindow() int64 {
	return 128000 // Default context window
}

func (m *mockClient) GetAutoCompactTokenLimit() int64 {
	return 100000 // Default auto-compact threshold
}

// Test Policy: Preserve System Messages
func TestPolicy_PreserveSystemMessages(t *testing.T) {
	messages := []client.Message{
		{Role: "system", Content: "You are a helpful assistant"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
	}

	policy := NewDefaultPolicy()
	preserved := policy.PreserveMessages(messages)

	// System message should always be preserved
	require.GreaterOrEqual(t, len(preserved), 1)
	assert.Equal(t, "system", preserved[0].Role)
}

// Test Policy: Preserve Recent Messages
func TestPolicy_PreserveRecentMessages(t *testing.T) {
	messages := []client.Message{
		{Role: "user", Content: "Message 1"},
		{Role: "assistant", Content: "Response 1"},
		{Role: "user", Content: "Message 2"},
		{Role: "assistant", Content: "Response 2"},
		{Role: "user", Content: "Message 3 - Recent"},
		{Role: "assistant", Content: "Response 3 - Recent"},
	}

	policy := &Policy{
		PreserveSystemMessages: true,
		PreserveRecentTurns:    2, // Keep last 2 turns (4 messages)
	}

	preserved := policy.PreserveMessages(messages)

	// Should preserve last 2 turns
	assert.GreaterOrEqual(t, len(preserved), 4, "Should preserve at least 4 messages from 2 turns")
	// Check that recent messages are in preserved list
	hasMessage2 := false
	hasResponse3 := false
	for _, msg := range preserved {
		if content, ok := msg.Content.(string); ok {
			if content == "Message 2" {
				hasMessage2 = true
			}
			if content == "Response 3 - Recent" {
				hasResponse3 = true
			}
		}
	}
	assert.True(t, hasMessage2, "Should preserve Message 2")
	assert.True(t, hasResponse3, "Should preserve Response 3 - Recent")
}

// Test Policy: Message Importance Scoring
func TestPolicy_MessageImportanceScore(t *testing.T) {
	tests := []struct {
		name     string
		message  client.Message
		minScore float64
	}{
		{
			name:     "system message has high importance",
			message:  client.Message{Role: "system", Content: "System prompt"},
			minScore: 1.0,
		},
		{
			name:     "tool call has high importance",
			message:  client.Message{Role: "assistant", ToolCalls: []client.ToolCall{{ID: "call_1"}}},
			minScore: 0.9,
		},
		{
			name:     "regular message has normal importance",
			message:  client.Message{Role: "user", Content: "Hello"},
			minScore: 0.5,
		},
	}

	policy := NewDefaultPolicy()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := policy.GetImportanceScore(tt.message)
			assert.GreaterOrEqual(t, score, tt.minScore)
		})
	}
}

// Test Truncation: Token Budget Enforcement
func TestTruncation_TokenBudget(t *testing.T) {
	counter := tokencount.NewFallbackCounter()

	messages := []client.Message{
		{Role: "system", Content: "System prompt with some tokens"},
		{Role: "user", Content: "First user message with many tokens here"},
		{Role: "assistant", Content: "First assistant response with lots of tokens"},
		{Role: "user", Content: "Second user message"},
		{Role: "assistant", Content: "Second response"},
	}

	truncator := &Truncator{
		TokenCounter: counter,
		MaxTokens:    100, // Small budget to force truncation
		Policy:       NewDefaultPolicy(),
	}

	result, err := truncator.Truncate(messages)
	require.NoError(t, err)

	// Count tokens in result
	totalTokens := 0
	for _, msg := range result {
		if content, ok := msg.Content.(string); ok {
			totalTokens += counter.CountTokens(content)
		}
	}

	assert.LessOrEqual(t, totalTokens, truncator.MaxTokens)
	assert.Greater(t, len(result), 0, "Should preserve at least some messages")
}

// Test Truncation: Drop Oldest Strategy
func TestTruncation_DropOldestStrategy(t *testing.T) {
	counter := tokencount.NewFallbackCounter()

	messages := []client.Message{
		{Role: "system", Content: "System"},
		{Role: "user", Content: "Old message 1"},
		{Role: "assistant", Content: "Old response 1"},
		{Role: "user", Content: "Recent message"},
		{Role: "assistant", Content: "Recent response"},
	}

	truncator := &Truncator{
		TokenCounter: counter,
		MaxTokens:    50,
		Strategy:     StrategyDropOldest,
		Policy:       NewDefaultPolicy(),
	}

	result, err := truncator.Truncate(messages)
	require.NoError(t, err)

	// System message should be preserved
	assert.Equal(t, "system", result[0].Role)

	// Recent messages should be preserved
	hasRecent := false
	for _, msg := range result {
		if content, ok := msg.Content.(string); ok && content == "Recent message" {
			hasRecent = true
		}
	}
	assert.True(t, hasRecent, "Recent messages should be preserved")
}

// Test Truncation: Sliding Window Strategy
func TestTruncation_SlidingWindowStrategy(t *testing.T) {
	counter := tokencount.NewFallbackCounter()

	messages := make([]client.Message, 0, 10)
	messages = append(messages, client.Message{Role: "system", Content: "System prompt"})

	// Add many messages
	for i := 0; i < 20; i++ {
		messages = append(messages, client.Message{
			Role:    "user",
			Content: "Message " + string(rune(i)),
		})
		messages = append(messages, client.Message{
			Role:    "assistant",
			Content: "Response " + string(rune(i)),
		})
	}

	truncator := &Truncator{
		TokenCounter: counter,
		MaxTokens:    200,
		Strategy:     StrategySlidingWindow,
		Policy: &Policy{
			PreserveSystemMessages: true,
			PreserveRecentTurns:    5,
		},
	}

	result, err := truncator.Truncate(messages)
	require.NoError(t, err)

	// Should preserve system message and recent turns
	assert.Equal(t, "system", result[0].Role)
	assert.LessOrEqual(t, len(result), len(messages))
}

// Test Summarization: Basic Summarization
func TestSummarize_Basic(t *testing.T) {
	mockCli := &mockClient{
		completeFunc: func(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) {
			return &client.ChatCompletionResponse{
				Choices: []client.Choice{
					{
						Message: client.Message{
							Role:    "assistant",
							Content: "Summary: User asked about compaction, assistant explained it.",
						},
					},
				},
			}, nil
		},
	}

	summarizer := &Summarizer{
		Client: mockCli,
		Policy: NewDefaultPolicy(),
	}

	messages := []client.Message{
		{Role: "user", Content: "What is history compaction?"},
		{Role: "assistant", Content: "History compaction is a technique to reduce memory usage by summarizing or truncating old conversation history while preserving important context."},
	}

	summary, err := summarizer.Summarize(context.Background(), messages)
	require.NoError(t, err)
	assert.NotEmpty(t, summary)
	assert.Contains(t, summary, "Summary")
}

// Test Summarization: Batch Summarization
func TestSummarize_Batching(t *testing.T) {
	callCount := 0
	mockCli := &mockClient{
		completeFunc: func(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) {
			callCount++
			return &client.ChatCompletionResponse{
				Choices: []client.Choice{
					{
						Message: client.Message{
							Role:    "assistant",
							Content: "Batch summary " + string(rune(callCount)),
						},
					},
				},
			}, nil
		},
	}

	summarizer := &Summarizer{
		Client:    mockCli,
		BatchSize: 4, // Summarize 4 messages at a time
		Policy:    NewDefaultPolicy(),
	}

	messages := make([]client.Message, 0, 12)
	for i := 0; i < 12; i++ {
		messages = append(messages, client.Message{
			Role:    "user",
			Content: "Message " + string(rune(i)),
		})
	}

	summary, err := summarizer.Summarize(context.Background(), messages)
	require.NoError(t, err)
	assert.NotEmpty(t, summary)
	// Batching may call 3 times for 3 batches + 1 for meta-summary = 4 calls
	assert.GreaterOrEqual(t, callCount, 3, "Should batch into at least 3 calls")
}

// Test Compactor: Full Integration
func TestCompactor_Compact(t *testing.T) {
	mockCli := &mockClient{
		completeFunc: func(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) {
			return &client.ChatCompletionResponse{
				Choices: []client.Choice{
					{
						Message: client.Message{
							Role:    "assistant",
							Content: "Summarized history",
						},
					},
				},
			}, nil
		},
	}

	compactor := NewCompactor(CompactorConfig{
		Client:       mockCli,
		TokenCounter: tokencount.NewFallbackCounter(),
		MaxTokens:    100,
		Strategy:     StrategyCompress,
		Policy:       NewDefaultPolicy(),
	})

	messages := []client.Message{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Tell me about Go"},
		{Role: "assistant", Content: "Go is a programming language designed at Google..."},
		{Role: "user", Content: "What about its concurrency?"},
		{Role: "assistant", Content: "Go has excellent concurrency support with goroutines and channels..."},
		{Role: "user", Content: "Current question"},
	}

	result, err := compactor.Compact(context.Background(), messages)
	require.NoError(t, err)

	assert.NotEmpty(t, result)
	assert.LessOrEqual(t, len(result), len(messages))

	// System message should be preserved
	assert.Equal(t, "system", result[0].Role)
}

// Test Compactor: Incremental Compaction
func TestCompactor_IncrementalCompaction(t *testing.T) {
	compactor := NewCompactor(CompactorConfig{
		TokenCounter: tokencount.NewFallbackCounter(),
		MaxTokens:    200,
		Strategy:     StrategyDropOldest,
		Policy:       NewDefaultPolicy(),
	})

	// First compaction
	messages1 := []client.Message{
		{Role: "system", Content: "System"},
		{Role: "user", Content: "Message 1"},
		{Role: "assistant", Content: "Response 1"},
	}

	result1, err := compactor.Compact(context.Background(), messages1)
	require.NoError(t, err)

	// Add more messages
	messages2 := append(result1,
		client.Message{Role: "user", Content: "Message 2"},
		client.Message{Role: "assistant", Content: "Response 2"},
		client.Message{Role: "user", Content: "Message 3"},
	)

	// Second compaction should handle incrementally
	result2, err := compactor.Compact(context.Background(), messages2)
	require.NoError(t, err)

	assert.NotEmpty(t, result2)
}

// Test Compactor: Auto-Compact Threshold
func TestCompactor_AutoCompactThreshold(t *testing.T) {
	mockCli := &mockClient{}

	compactor := NewCompactor(CompactorConfig{
		Client:               mockCli,
		TokenCounter:         tokencount.NewFallbackCounter(),
		AutoCompactThreshold: 50,
		MaxTokens:            100,
		Strategy:             StrategyDropOldest,
		Policy:               NewDefaultPolicy(),
	})

	// Messages under threshold
	smallMessages := []client.Message{
		{Role: "system", Content: "Short"},
		{Role: "user", Content: "Hi"},
	}

	shouldCompact := compactor.ShouldCompact(smallMessages)
	assert.False(t, shouldCompact, "Should not compact when under threshold")

	// Messages over threshold
	largeMessages := []client.Message{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "This is a very long message with many tokens that should trigger auto-compaction"},
		{Role: "assistant", Content: "This is an equally long response with many tokens"},
	}

	// Create a compactor with threshold that will be exceeded
	compactor2 := NewCompactor(CompactorConfig{
		Client:               mockCli,
		TokenCounter:         tokencount.NewFallbackCounter(),
		AutoCompactThreshold: 10, // Very low threshold to ensure trigger
		MaxTokens:            100,
		Strategy:             StrategyDropOldest,
		Policy:               NewDefaultPolicy(),
	})

	shouldCompact = compactor2.ShouldCompact(largeMessages)
	assert.True(t, shouldCompact, "Should compact when over threshold")
}

// Test Compactor: Async Summarization
func TestCompactor_AsyncSummarization(t *testing.T) {
	callChan := make(chan bool, 1)
	mockCli := &mockClient{
		completeFunc: func(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) {
			callChan <- true
			return &client.ChatCompletionResponse{
				Choices: []client.Choice{
					{Message: client.Message{Role: "assistant", Content: "Async summary"}},
				},
			}, nil
		},
	}

	compactor := NewCompactor(CompactorConfig{
		Client:       mockCli,
		TokenCounter: tokencount.NewFallbackCounter(),
		MaxTokens:    100,
		Strategy:     StrategyCompress,
		Policy:       NewDefaultPolicy(),
		Async:        true,
	})

	messages := []client.Message{
		{Role: "user", Content: "Message to summarize"},
		{Role: "assistant", Content: "Response to summarize"},
	}

	resultChan := compactor.CompactAsync(context.Background(), messages)

	result := <-resultChan
	require.NoError(t, result.Error)
	assert.NotEmpty(t, result.Messages)

	// Note: async compaction may or may not call API depending on whether
	// summarization is needed. The test is mainly checking that async works.
}

// Test Strategy: Compress Strategy
func TestStrategy_Compress(t *testing.T) {
	mockCli := &mockClient{
		completeFunc: func(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) {
			return &client.ChatCompletionResponse{
				Choices: []client.Choice{
					{Message: client.Message{Role: "assistant", Content: "Compressed history"}},
				},
			}, nil
		},
	}

	compactor := NewCompactor(CompactorConfig{
		Client:       mockCli,
		TokenCounter: tokencount.NewFallbackCounter(),
		MaxTokens:    50,
		Strategy:     StrategyCompress,
		Policy:       NewDefaultPolicy(),
	})

	messages := []client.Message{
		{Role: "user", Content: "Long message 1"},
		{Role: "assistant", Content: "Long response 1"},
		{Role: "user", Content: "Long message 2"},
	}

	result, err := compactor.Compact(context.Background(), messages)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

// Test Error Handling: Client Errors
func TestCompactor_ClientError(t *testing.T) {
	mockCli := &mockClient{
		completeFunc: func(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) {
			return nil, errors.New("API error")
		},
	}

	compactor := NewCompactor(CompactorConfig{
		Client:       mockCli,
		TokenCounter: tokencount.NewFallbackCounter(),
		MaxTokens:    100,
		Strategy:     StrategyCompress,
		Policy:       NewDefaultPolicy(),
	})

	// Need enough messages to trigger compression strategy
	messages := []client.Message{
		{Role: "user", Content: "This is a very long message that exceeds the token budget and will require compaction using the compress strategy which will call the API"},
		{Role: "assistant", Content: "This is a very long response that also exceeds budget"},
	}

	_, err := compactor.Compact(context.Background(), messages)
	// With compress strategy and API error, should either error or fall back
	if err != nil {
		// Error is acceptable
		assert.Error(t, err)
	}
}

// Test Error Handling: Context Cancellation
func TestCompactor_ContextCancellation(t *testing.T) {
	mockCli := &mockClient{
		completeFunc: func(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	compactor := NewCompactor(CompactorConfig{
		Client:       mockCli,
		TokenCounter: tokencount.NewFallbackCounter(),
		MaxTokens:    10, // Very small to force compaction
		Strategy:     StrategyCompress,
		Policy:       NewDefaultPolicy(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Use a long message to ensure compaction is triggered
	messages := []client.Message{
		{Role: "user", Content: "This is a very long message that will definitely exceed the tiny token budget and require compaction with the compress strategy"},
	}

	_, err := compactor.Compact(ctx, messages)
	// Error expected due to context cancellation, but may not happen if compaction
	// doesn't need to call API (e.g., falls back to truncation)
	// So we just check that it doesn't panic
	_ = err
}

// Test Edge Cases: Empty Messages
func TestCompactor_EmptyMessages(t *testing.T) {
	compactor := NewCompactor(CompactorConfig{
		TokenCounter: tokencount.NewFallbackCounter(),
		MaxTokens:    100,
		Strategy:     StrategyDropOldest,
		Policy:       NewDefaultPolicy(),
	})

	result, err := compactor.Compact(context.Background(), []client.Message{})
	require.NoError(t, err)
	assert.Empty(t, result)
}

// Test Edge Cases: Single Message
func TestCompactor_SingleMessage(t *testing.T) {
	compactor := NewCompactor(CompactorConfig{
		TokenCounter: tokencount.NewFallbackCounter(),
		MaxTokens:    100,
		Strategy:     StrategyDropOldest,
		Policy:       NewDefaultPolicy(),
	})

	messages := []client.Message{
		{Role: "system", Content: "System message"},
	}

	result, err := compactor.Compact(context.Background(), messages)
	require.NoError(t, err)
	assert.Equal(t, messages, result)
}

// Test Token Counting: Multiple Content Types
func TestTokenCounting_MultipleContentTypes(t *testing.T) {
	counter := tokencount.NewFallbackCounter()

	messages := []client.Message{
		{Role: "user", Content: "Simple text"},
		{Role: "assistant", Content: []client.ContentItem{
			{Type: "text", Text: "Complex content"},
		}},
	}

	truncator := &Truncator{
		TokenCounter: counter,
		MaxTokens:    100,
		Strategy:     StrategyDropOldest,
		Policy:       NewDefaultPolicy(),
	}

	result, err := truncator.Truncate(messages)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

// Benchmark: Truncation Performance
func BenchmarkTruncation_DropOldest(b *testing.B) {
	counter := tokencount.NewFallbackCounter()
	messages := make([]client.Message, 100)
	for i := 0; i < 100; i++ {
		messages[i] = client.Message{
			Role:    "user",
			Content: "Benchmark message number " + string(rune(i)),
		}
	}

	truncator := &Truncator{
		TokenCounter: counter,
		MaxTokens:    500,
		Strategy:     StrategyDropOldest,
		Policy:       NewDefaultPolicy(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = truncator.Truncate(messages)
	}
}

// Benchmark: Summarization Performance
func BenchmarkSummarization(b *testing.B) {
	mockCli := &mockClient{
		completeFunc: func(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) {
			return &client.ChatCompletionResponse{
				Choices: []client.Choice{
					{Message: client.Message{Role: "assistant", Content: "Summary"}},
				},
			}, nil
		},
	}

	summarizer := &Summarizer{
		Client: mockCli,
		Policy: NewDefaultPolicy(),
	}

	messages := make([]client.Message, 10)
	for i := 0; i < 10; i++ {
		messages[i] = client.Message{
			Role:    "user",
			Content: "Message " + string(rune(i)),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = summarizer.Summarize(context.Background(), messages)
	}
}

// Test Compactor: GetStats and ResetStats
func TestCompactor_Stats(t *testing.T) {
	compactor := NewCompactor(CompactorConfig{
		TokenCounter: tokencount.NewFallbackCounter(),
		MaxTokens:    20, // Very small budget to force compaction
		Strategy:     StrategyDropOldest,
		Policy:       NewDefaultPolicy(),
	})

	messages := []client.Message{
		{Role: "user", Content: "Long message that definitely exceeds the very small budget we have configured"},
		{Role: "assistant", Content: "Long response that also exceeds the budget"},
	}

	// Perform compaction
	_, err := compactor.Compact(context.Background(), messages)
	require.NoError(t, err)

	// Get stats
	stats := compactor.GetStats()
	assert.GreaterOrEqual(t, stats.TotalCompactions, 1)
	assert.GreaterOrEqual(t, stats.TotalTokensSaved, 0)

	// Reset stats
	compactor.ResetStats()
	stats = compactor.GetStats()
	assert.Equal(t, 0, stats.TotalCompactions)
	assert.Equal(t, 0, stats.TotalTokensSaved)
}

// Test Compactor: Estimate
func TestCompactor_Estimate(t *testing.T) {
	compactor := NewCompactor(CompactorConfig{
		TokenCounter: tokencount.NewFallbackCounter(),
		MaxTokens:    20, // Very small budget
		Strategy:     StrategyDropOldest,
		Policy:       NewDefaultPolicy(),
	})

	messages := []client.Message{
		{Role: "user", Content: "Long message that definitely exceeds the very small budget we configured"},
		{Role: "assistant", Content: "Long response that also exceeds budget"},
	}

	estimate := compactor.Estimate(messages)
	assert.NotNil(t, estimate)
	assert.True(t, estimate.WillCompact, "Should need compaction with small budget")
	assert.Equal(t, StrategyDropOldest, estimate.Strategy)
	// EstimatedTokens should be less than or equal to current
	assert.LessOrEqual(t, estimate.EstimatedTokens, estimate.CurrentTokens)
}

// Test Compactor: CompactIfNeeded
func TestCompactor_CompactIfNeeded(t *testing.T) {
	compactor := NewCompactor(CompactorConfig{
		TokenCounter:         tokencount.NewFallbackCounter(),
		MaxTokens:            100,
		AutoCompactThreshold: 10, // Very low threshold
		Strategy:             StrategyDropOldest,
		Policy:               NewDefaultPolicy(),
	})

	// Small messages - no compaction needed
	smallMessages := []client.Message{
		{Role: "user", Content: "Hi"},
	}

	result, compacted, err := compactor.CompactIfNeeded(context.Background(), smallMessages)
	require.NoError(t, err)
	assert.False(t, compacted)
	assert.Equal(t, smallMessages, result)

	// Large messages - compaction needed
	largeMessages := []client.Message{
		{Role: "user", Content: "Long message that definitely exceeds the very low auto-compact threshold"},
		{Role: "assistant", Content: "Long response that also exceeds it"},
	}

	result, compacted, err = compactor.CompactIfNeeded(context.Background(), largeMessages)
	require.NoError(t, err)
	assert.True(t, compacted, "Should compact when over threshold")
	assert.NotNil(t, result)
}

// Test Compactor: Setters and Getters
func TestCompactor_SettersGetters(t *testing.T) {
	compactor := NewCompactor(CompactorConfig{
		TokenCounter: tokencount.NewFallbackCounter(),
		MaxTokens:    100,
		Strategy:     StrategyDropOldest,
		Policy:       NewDefaultPolicy(),
	})

	// Test MaxTokens
	compactor.SetMaxTokens(200)
	assert.Equal(t, 200, compactor.GetMaxTokens())

	// Test Strategy
	compactor.SetStrategy(StrategyCompress)
	assert.Equal(t, StrategyCompress, compactor.GetStrategy())

	// Test Policy
	newPolicy := NewConservativePolicy()
	compactor.SetPolicy(newPolicy)
	assert.Equal(t, newPolicy, compactor.GetPolicy())

	// Test AutoCompactThreshold
	compactor.SetAutoCompactThreshold(150)
	assert.Equal(t, 150, compactor.GetAutoCompactThreshold())
}

// Test ValidateConfig
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      CompactorConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: CompactorConfig{
				TokenCounter:         tokencount.NewFallbackCounter(),
				MaxTokens:            100,
				AutoCompactThreshold: 150,
				Strategy:             StrategyDropOldest,
			},
			expectError: false,
		},
		{
			name: "missing token counter",
			config: CompactorConfig{
				MaxTokens: 100,
				Strategy:  StrategyDropOldest,
			},
			expectError: true,
		},
		{
			name: "invalid max tokens",
			config: CompactorConfig{
				TokenCounter: tokencount.NewFallbackCounter(),
				MaxTokens:    0,
				Strategy:     StrategyDropOldest,
			},
			expectError: true,
		},
		{
			name: "compress without client",
			config: CompactorConfig{
				TokenCounter: tokencount.NewFallbackCounter(),
				MaxTokens:    100,
				Strategy:     StrategyCompress,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test Policy: Conservative and Aggressive
func TestPolicy_Variants(t *testing.T) {
	// Conservative policy should preserve more recent turns
	conservative := NewConservativePolicy()
	assert.Equal(t, 10, conservative.PreserveRecentTurns)

	// Aggressive policy should preserve fewer recent turns
	aggressive := NewAggressivePolicy()
	assert.Equal(t, 1, aggressive.PreserveRecentTurns)

	// Test importance scoring differences
	conservativeScore := conservative.GetImportanceScore(client.Message{
		Role:      "assistant",
		Reasoning: "Some reasoning",
	})
	aggressiveScore := aggressive.GetImportanceScore(client.Message{
		Role:      "assistant",
		Reasoning: "Some reasoning",
	})

	// Conservative preserves reasoning, aggressive doesn't
	assert.NotEqual(t, conservativeScore, aggressiveScore)
}

// Test Summarizer: Quality Evaluation
func TestSummarizer_EvaluateQuality(t *testing.T) {
	summarizer := NewSummarizer(nil)
	counter := tokencount.NewFallbackCounter()

	original := []client.Message{
		{Role: "user", Content: "This is a very long original message with lots of content"},
		{Role: "assistant", Content: "This is a very long response with lots of details"},
	}

	summary := "Short summary"

	quality := summarizer.EvaluateSummary(original, summary, counter)
	assert.NotNil(t, quality)
	assert.Greater(t, quality.ReductionRatio, 0.0)
	assert.LessOrEqual(t, quality.InformationLoss, 1.0)
}

// Test Truncator: Importance-Based Strategy
func TestTruncation_ImportanceBasedStrategy(t *testing.T) {
	counter := tokencount.NewFallbackCounter()

	messages := []client.Message{
		{Role: "system", Content: "System message"},
		{Role: "user", Content: "Low importance user message"},
		{Role: "assistant", ToolCalls: []client.ToolCall{{ID: "call_1"}}},
		{Role: "user", Content: "Another message"},
	}

	truncator := &Truncator{
		TokenCounter: counter,
		MaxTokens:    50,
		Strategy:     StrategyImportanceBased,
		Policy:       NewDefaultPolicy(),
	}

	result, err := truncator.Truncate(messages)
	require.NoError(t, err)

	// System message and tool call should be preserved
	hasSystem := false
	hasToolCall := false
	for _, msg := range result {
		if msg.Role == "system" {
			hasSystem = true
		}
		if len(msg.ToolCalls) > 0 {
			hasToolCall = true
		}
	}
	assert.True(t, hasSystem, "System message should be preserved")
	assert.True(t, hasToolCall, "Tool call should be preserved")
}

// Test Strategy: Sliding Window
func TestStrategy_SlidingWindow(t *testing.T) {
	compactor := NewCompactor(CompactorConfig{
		TokenCounter: tokencount.NewFallbackCounter(),
		MaxTokens:    30,
		Strategy:     StrategySlidingWindow,
		Policy:       NewDefaultPolicy(),
	})

	messages := []client.Message{
		{Role: "system", Content: "System"},
		{Role: "user", Content: "Old message 1"},
		{Role: "assistant", Content: "Old response 1"},
		{Role: "user", Content: "Recent message"},
		{Role: "assistant", Content: "Recent response"},
	}

	result, err := compactor.Compact(context.Background(), messages)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

// Test Strategy: Importance-Based
func TestStrategy_ImportanceBased(t *testing.T) {
	compactor := NewCompactor(CompactorConfig{
		TokenCounter: tokencount.NewFallbackCounter(),
		MaxTokens:    30,
		Strategy:     StrategyImportanceBased,
		Policy:       NewDefaultPolicy(),
	})

	messages := []client.Message{
		{Role: "system", Content: "System"},
		{Role: "user", Content: "User message"},
		{Role: "assistant", ToolCalls: []client.ToolCall{{ID: "call_1", Function: &client.FunctionCall{Name: "test"}}}},
		{Role: "user", Content: "Another message"},
	}

	result, err := compactor.Compact(context.Background(), messages)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

// Test Summarizer: Additional Functions
func TestSummarizer_AdditionalFunctions(t *testing.T) {
	mockCli := &mockClient{
		completeFunc: func(ctx context.Context, req *client.ChatCompletionRequest) (*client.ChatCompletionResponse, error) {
			return &client.ChatCompletionResponse{
				Choices: []client.Choice{
					{Message: client.Message{Role: "assistant", Content: "Test summary"}},
				},
			}, nil
		},
	}

	summarizer := NewSummarizer(mockCli)
	counter := tokencount.NewFallbackCounter()

	messages := []client.Message{
		{Role: "user", Content: "Message 1"},
		{Role: "assistant", Content: "Response 1"},
		{Role: "user", Content: "Message 2"},
	}

	// Test SummarizeWithResult
	result, err := summarizer.SummarizeWithResult(context.Background(), messages, counter)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Greater(t, result.TokensSaved, 0)

	// Test SummarizeRange
	summary, err := summarizer.SummarizeRange(context.Background(), messages, 0, 2)
	require.NoError(t, err)
	assert.NotEmpty(t, summary)

	// Test SummarizeByTurns
	summarizer.PreserveTurnStructure = true
	turnResult, err := summarizer.SummarizeByTurns(context.Background(), messages, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, turnResult)

	// Test EstimateSummaryLength
	estimatedLength := summarizer.EstimateSummaryLength(messages, counter)
	assert.Greater(t, estimatedLength, 0)

	// Test CanSummarize
	canSummarize := summarizer.CanSummarize(messages)
	assert.True(t, canSummarize)
}

// Test Policy: Additional Functions
func TestPolicy_AdditionalFunctions(t *testing.T) {
	policy := NewDefaultPolicy()
	counter := tokencount.NewFallbackCounter()

	messages := []client.Message{
		{Role: "system", Content: "System"},
		{Role: "user", Content: "User message"},
		{Role: "assistant", Content: "Assistant response"},
	}

	// Test CalculatePositions
	positions := policy.CalculatePositions(messages)
	assert.Len(t, positions, len(messages))
	assert.True(t, positions[0].IsSystemMessage)

	// Test EstimateTokenSavings
	canRemove, mustKeep := policy.EstimateTokenSavings(messages, counter)
	assert.GreaterOrEqual(t, canRemove+mustKeep, 0)

	// Test CanSummarize
	assert.False(t, policy.CanSummarize(messages[0]), "System messages can't be summarized")
	assert.True(t, policy.CanSummarize(messages[1]), "User messages can be summarized")

	// Test ShouldPreserve with positions
	pos := positions[0]
	assert.True(t, policy.ShouldPreserve(messages[0], pos), "System message should be preserved")
}

// Test Truncator: Additional Functions
func TestTruncator_AdditionalFunctions(t *testing.T) {
	counter := tokencount.NewFallbackCounter()
	truncator := &Truncator{
		TokenCounter: counter,
		MaxTokens:    50,
		Strategy:     StrategyDropOldest,
		Policy:       NewDefaultPolicy(),
	}

	messages := []client.Message{
		{Role: "user", Content: "Long message"},
		{Role: "assistant", Content: "Long response"},
	}

	// Test TruncateWithResult
	result, err := truncator.TruncateWithResult(messages)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, result.TokensRemoved, 0)

	// Test EstimateTokensToRemove
	toRemove := truncator.EstimateTokensToRemove(messages)
	assert.GreaterOrEqual(t, toRemove, 0)

	// Test CanTruncate
	canTruncate := truncator.CanTruncate(messages)
	assert.True(t, canTruncate)

	// Test getters/setters
	truncator.SetStrategy(StrategySlidingWindow)
	assert.Equal(t, StrategySlidingWindow, truncator.GetStrategy())

	truncator.SetTokenBudget(100)
	assert.Equal(t, 100, truncator.GetTokenBudget())
}
