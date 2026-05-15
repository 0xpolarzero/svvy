// Package tokencount provides token counting functionality with fallback support.
// It matches the behavior of codex-rs/codex-utils-tokenizer, providing:
// - Primary tokenizer using tiktoken (o200k_base or cl100k_base)
// - Fallback heuristic: 4 bytes per token
// - Model-specific tokenizer selection
// - Caching for repeated calls
package tokencount

import (
	"fmt"

	"github.com/pkoukk/tiktoken-go"
)

// EncodingKind represents the supported tiktoken encodings
type EncodingKind string

const (
	// EncodingO200kBase is the o200k_base encoding (default, used by GPT-4o and newer models)
	EncodingO200kBase EncodingKind = "o200k_base"
	// EncodingCl100kBase is the cl100k_base encoding (used by GPT-4, GPT-3.5-turbo)
	EncodingCl100kBase EncodingKind = "cl100k_base"
)

// TokenCounter is the interface for counting tokens in text
type TokenCounter interface {
	// CountTokens returns the number of tokens in the given text
	CountTokens(text string) int
}

// fallbackCounter implements a simple 4 bytes per token heuristic
type fallbackCounter struct{}

// NewFallbackCounter creates a counter that uses the 4 bytes per token heuristic.
// This matches the fallback behavior in codex-rs/core/src/truncate.rs:
// `(text.len() as u64).div_ceil(4)`
func NewFallbackCounter() TokenCounter {
	return &fallbackCounter{}
}

// CountTokens implements the 4 bytes per token heuristic.
// Uses ceiling division to match Rust's div_ceil behavior.
func (f *fallbackCounter) CountTokens(text string) int {
	byteLen := len(text)
	if byteLen == 0 {
		return 0
	}
	// Ceiling division: (n + d - 1) / d
	return (byteLen + 3) / 4
}

// tiktokenCounter wraps a tiktoken encoder for accurate token counting
type tiktokenCounter struct {
	encoder *tiktoken.Tiktoken
	cache   *LRUCache
}

// NewTiktokenCounter creates a counter using the specified tiktoken encoding.
// Returns an error if the encoding cannot be loaded.
// Uses an LRU cache with 10,000 entries by default for better memory management.
func NewTiktokenCounter(kind EncodingKind) (TokenCounter, error) {
	encoder, err := tiktoken.GetEncoding(string(kind))
	if err != nil {
		return nil, fmt.Errorf("failed to load encoding %s: %w", kind, err)
	}

	return &tiktokenCounter{
		encoder: encoder,
		cache:   NewLRUCache(10000), // Default cache size
	}, nil
}

// NewTiktokenCounterWithCache creates a counter with a custom cache size.
func NewTiktokenCounterWithCache(kind EncodingKind, cacheSize int) (TokenCounter, error) {
	encoder, err := tiktoken.GetEncoding(string(kind))
	if err != nil {
		return nil, fmt.Errorf("failed to load encoding %s: %w", kind, err)
	}

	return &tiktokenCounter{
		encoder: encoder,
		cache:   NewLRUCache(cacheSize),
	}, nil
}

// CountTokens counts tokens using tiktoken encoding with LRU caching
func (t *tiktokenCounter) CountTokens(text string) int {
	// Check cache first (O(1) with LRU)
	if count, ok := t.cache.Get(text); ok {
		return count
	}

	// Encode and count tokens
	tokens := t.encoder.Encode(text, nil, nil)
	count := len(tokens)

	// Cache the result
	t.cache.Put(text, count)

	return count
}

// hybridCounter tries tiktoken first, falls back to heuristic if unavailable
type hybridCounter struct {
	primary  TokenCounter
	fallback TokenCounter
}

// NewDefaultCounter creates a counter that tries to use o200k_base tiktoken encoding,
// falling back to the 4 bytes per token heuristic if tiktoken is unavailable.
// This matches the behavior in codex-rs/core/src/truncate.rs.
func NewDefaultCounter() TokenCounter {
	primary, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		// If tiktoken fails, use fallback only
		return NewFallbackCounter()
	}

	return &hybridCounter{
		primary:  primary,
		fallback: NewFallbackCounter(),
	}
}

// CountTokens delegates to the primary counter if available
func (h *hybridCounter) CountTokens(text string) int {
	return h.primary.CountTokens(text)
}

// NewCounterForModel creates a counter for a specific model name.
// It attempts to use tiktoken's model-to-encoding mapping, falling back to
// o200k_base for unknown models. This matches the behavior of
// codex-rs/utils/tokenizer::Tokenizer::for_model.
func NewCounterForModel(modelName string) (TokenCounter, error) {
	// Try to get encoding for the specific model
	encoder, err := tiktoken.EncodingForModel(modelName)
	if err != nil {
		// Model not found, fall back to o200k_base (default for newer models)
		return NewTiktokenCounter(EncodingO200kBase)
	}

	return &tiktokenCounter{
		encoder: encoder,
		cache:   NewLRUCache(10000),
	}, nil
}

// CountTokensWithFallback is a convenience function that counts tokens with automatic fallback.
// It tries tiktoken first, then uses the 4 bytes per token heuristic if unavailable.
func CountTokensWithFallback(text string) int {
	counter := NewDefaultCounter()
	return counter.CountTokens(text)
}
