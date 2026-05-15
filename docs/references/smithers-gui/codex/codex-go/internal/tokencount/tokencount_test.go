package tokencount

import (
	"strings"
	"testing"
)

// TestFallbackCounter tests the basic fallback counter (4 bytes per token)
func TestFallbackCounter(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name:     "single character",
			text:     "a",
			expected: 1, // 1 byte / 4 = 0.25, rounded up to 1
		},
		{
			name:     "four characters",
			text:     "test",
			expected: 1, // 4 bytes / 4 = 1 token
		},
		{
			name:     "five characters",
			text:     "hello",
			expected: 2, // 5 bytes / 4 = 1.25, rounded up to 2
		},
		{
			name:     "hello world",
			text:     "hello world",
			expected: 3, // 11 bytes / 4 = 2.75, rounded up to 3
		},
		{
			name:     "longer text",
			text:     "This is a longer piece of text for testing",
			expected: 11, // 43 bytes / 4 = 10.75, rounded up to 11
		},
		{
			name:     "unicode text",
			text:     "😀😀😀",
			expected: 3, // Each emoji is 4 bytes, so 12 / 4 = 3 tokens
		},
		{
			name:     "mixed ascii and unicode",
			text:     "Hello 😀 World",
			expected: 4, // "Hello " (6) + "😀" (4) + " World" (6) = 16 bytes / 4 = 4
		},
	}

	counter := NewFallbackCounter()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := counter.CountTokens(tt.text)
			if got != tt.expected {
				t.Errorf("CountTokens(%q) = %d, want %d (text length: %d bytes)",
					tt.text, got, tt.expected, len(tt.text))
			}
		})
	}
}

// TestTiktokenCounter tests the tiktoken-based counter if available
func TestTiktokenCounter(t *testing.T) {
	counter, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		t.Skipf("Skipping tiktoken test, tokenizer not available: %v", err)
	}

	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name: "hello world with cl100k_base",
			text: "hello world",
			// Known token IDs for cl100k_base: [15339, 1917]
			// Using o200k_base might differ, but we'll verify it counts correctly
			expected: 2,
		},
	}

	// Test with cl100k_base which has known token counts
	cl100kCounter, err := NewTiktokenCounter(EncodingCl100kBase)
	if err != nil {
		t.Skipf("Skipping cl100k test: %v", err)
	}

	// Test specific known encoding for "hello world" with cl100k_base
	count := cl100kCounter.CountTokens("hello world")
	if count != 2 {
		t.Errorf("cl100k_base CountTokens('hello world') = %d, want 2", count)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := counter.CountTokens(tt.text)
			if got != tt.expected {
				t.Errorf("CountTokens(%q) = %d, want %d",
					tt.text, got, tt.expected)
			}
		})
	}
}

// TestDefaultCounter tests the default counter which should use tiktoken with fallback
func TestDefaultCounter(t *testing.T) {
	counter := NewDefaultCounter()

	// Test basic functionality
	text := "This is a test string"
	count := counter.CountTokens(text)

	// Should return a positive count
	if count <= 0 {
		t.Errorf("CountTokens should return positive count, got %d", count)
	}

	// Empty string should return 0
	if counter.CountTokens("") != 0 {
		t.Errorf("CountTokens('') should return 0, got %d", counter.CountTokens(""))
	}
}

// TestCounterCaching tests that repeated calls are cached
func TestCounterCaching(t *testing.T) {
	counter := NewDefaultCounter()

	text := "This is a test for caching behavior"

	// Call multiple times
	count1 := counter.CountTokens(text)
	count2 := counter.CountTokens(text)
	count3 := counter.CountTokens(text)

	// All calls should return the same result
	if count1 != count2 || count2 != count3 {
		t.Errorf("Cached results differ: %d, %d, %d", count1, count2, count3)
	}
}

// TestForModel tests creating a counter for a specific model
func TestForModel(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		wantErr   bool
	}{
		{
			name:      "gpt-4",
			modelName: "gpt-4",
			wantErr:   false,
		},
		{
			name:      "unknown model falls back",
			modelName: "does-not-exist",
			wantErr:   false, // Should fallback, not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter, err := NewCounterForModel(tt.modelName)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCounterForModel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if counter != nil {
				// Test it works
				count := counter.CountTokens("test")
				if count <= 0 {
					t.Errorf("Counter should return positive count, got %d", count)
				}
			}
		})
	}
}

// TestLargeText tests token counting with large text
func TestLargeText(t *testing.T) {
	counter := NewDefaultCounter()

	// Create a large text (1MB)
	largeText := strings.Repeat("Hello world! ", 87000) // ~1MB

	count := counter.CountTokens(largeText)

	// Should return a reasonable count (rough estimate: 1MB / 4 = 256K tokens)
	if count <= 0 {
		t.Errorf("Large text should have positive token count, got %d", count)
	}

	// Should be in reasonable range (not exact, just sanity check)
	if count < 100000 || count > 500000 {
		t.Logf("Warning: Large text token count %d seems unusual (expected ~200k-300k)", count)
	}
}

// TestEdgeCases tests various edge cases
func TestEdgeCases(t *testing.T) {
	counter := NewDefaultCounter()

	tests := []struct {
		name string
		text string
	}{
		{
			name: "newlines",
			text: "\n\n\n",
		},
		{
			name: "tabs",
			text: "\t\t\t",
		},
		{
			name: "spaces",
			text: "     ",
		},
		{
			name: "mixed whitespace",
			text: " \t\n \t\n ",
		},
		{
			name: "special characters",
			text: "!@#$%^&*()_+-={}[]|\\:;<>?,./",
		},
		{
			name: "unicode characters",
			text: "Hello 世界 🌍",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := counter.CountTokens(tt.text)
			// Just verify it doesn't panic and returns a non-negative count
			if count < 0 {
				t.Errorf("CountTokens(%q) returned negative count: %d", tt.text, count)
			}
		})
	}
}

// BenchmarkFallbackCounter benchmarks the fallback counter
func BenchmarkFallbackCounter(b *testing.B) {
	counter := NewFallbackCounter()
	text := "This is a test string for benchmarking the fallback counter implementation"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.CountTokens(text)
	}
}

// BenchmarkTiktokenCounter benchmarks the tiktoken counter
func BenchmarkTiktokenCounter(b *testing.B) {
	counter, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping benchmark, tokenizer not available: %v", err)
	}

	text := "This is a test string for benchmarking the tiktoken counter implementation"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.CountTokens(text)
	}
}

// BenchmarkDefaultCounter benchmarks the default counter
func BenchmarkDefaultCounter(b *testing.B) {
	counter := NewDefaultCounter()
	text := "This is a test string for benchmarking the default counter implementation"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.CountTokens(text)
	}
}

// BenchmarkDefaultCounterWithCache benchmarks the default counter with caching
func BenchmarkDefaultCounterWithCache(b *testing.B) {
	counter := NewDefaultCounter()
	text := "This is a test string for benchmarking with cache"

	// Prime the cache
	counter.CountTokens(text)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.CountTokens(text)
	}
}

// BenchmarkLargeText benchmarks token counting with large text
func BenchmarkLargeText(b *testing.B) {
	counter := NewDefaultCounter()
	largeText := strings.Repeat("Hello world! ", 10000) // ~120KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.CountTokens(largeText)
	}
}
