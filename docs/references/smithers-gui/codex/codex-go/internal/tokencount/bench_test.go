package tokencount

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// Benchmark utilities

// generateText creates test text of approximately the given byte size
func generateText(size int) string {
	words := []string{"hello", "world", "test", "benchmark", "performance", "optimization", "token", "count"}
	var builder strings.Builder
	builder.Grow(size)

	for builder.Len() < size {
		builder.WriteString(words[rand.Intn(len(words))])
		builder.WriteString(" ")
	}
	return builder.String()
}

// generateUniqueTexts creates N unique text samples
func generateUniqueTexts(count, size int) []string {
	texts := make([]string, count)
	for i := 0; i < count; i++ {
		texts[i] = fmt.Sprintf("%s-%d", generateText(size), i)
	}
	return texts
}

// generateRepeatedTexts creates N texts with some repetition (simulates real usage)
func generateRepeatedTexts(count, uniqueCount, size int) []string {
	unique := generateUniqueTexts(uniqueCount, size)
	texts := make([]string, count)
	for i := 0; i < count; i++ {
		texts[i] = unique[i%uniqueCount]
	}
	return texts
}

// Baseline Benchmarks

func BenchmarkFallbackCounter_Short(b *testing.B) {
	counter := NewFallbackCounter()
	text := "This is a short test string"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.CountTokens(text)
	}
}

func BenchmarkFallbackCounter_Medium(b *testing.B) {
	counter := NewFallbackCounter()
	text := strings.Repeat("This is a medium length test string for benchmarking. ", 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.CountTokens(text)
	}
}

func BenchmarkFallbackCounter_Large(b *testing.B) {
	counter := NewFallbackCounter()
	text := strings.Repeat("This is a large test string for benchmarking. ", 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.CountTokens(text)
	}
}

// Tiktoken Benchmarks - Basic

func BenchmarkTiktoken_Short_NoCaching(b *testing.B) {
	counter, err := NewTiktokenCounterWithCache(EncodingO200kBase, 0) // No cache
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	texts := generateUniqueTexts(b.N, 50)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		counter.CountTokens(texts[i%len(texts)])
	}
}

func BenchmarkTiktoken_Short_WithLRUCache(b *testing.B) {
	counter, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	texts := generateRepeatedTexts(b.N, 100, 50)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		counter.CountTokens(texts[i%len(texts)])
	}
}

func BenchmarkTiktoken_Medium_WithCache(b *testing.B) {
	counter, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	text := strings.Repeat("This is a medium length test string. ", 20)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		counter.CountTokens(text)
	}
}

func BenchmarkTiktoken_Large_WithCache(b *testing.B) {
	counter, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	text := strings.Repeat("This is a large test string for benchmarking performance. ", 500)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		counter.CountTokens(text)
	}
}

// Cache Performance Benchmarks

func BenchmarkLRUCache_Get_Hit(b *testing.B) {
	cache := NewLRUCache(1000)
	cache.Put("test", 42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get("test")
	}
}

func BenchmarkLRUCache_Get_Miss(b *testing.B) {
	cache := NewLRUCache(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get("nonexistent")
	}
}

func BenchmarkLRUCache_Put(b *testing.B) {
	cache := NewLRUCache(1000)
	texts := generateUniqueTexts(b.N, 50)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Put(texts[i%len(texts)], i)
	}
}

func BenchmarkLRUCache_Put_WithEviction(b *testing.B) {
	cache := NewLRUCache(100) // Small cache to trigger evictions
	texts := generateUniqueTexts(1000, 50)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Put(texts[i%len(texts)], i)
	}
}

// CachedCounter Benchmarks

func BenchmarkCachedCounter_HighHitRate(b *testing.B) {
	base, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	counter := NewCachedCounter(base, 1000)
	texts := generateRepeatedTexts(1000, 50, 100) // High repetition = high hit rate

	// Prime cache
	for _, text := range texts[:50] {
		counter.CountTokens(text)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.CountTokens(texts[i%len(texts)])
	}
}

func BenchmarkCachedCounter_LowHitRate(b *testing.B) {
	base, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	counter := NewCachedCounter(base, 1000)
	texts := generateUniqueTexts(b.N, 100) // All unique = low hit rate

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.CountTokens(texts[i%len(texts)])
	}
}

// BatchCounter Benchmarks

func BenchmarkBatchCounter_Small(b *testing.B) {
	base, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	counter := NewBatchCounter(base, 1000)
	texts := generateRepeatedTexts(10, 5, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.CountTokensBatch(texts)
	}
}

func BenchmarkBatchCounter_Medium(b *testing.B) {
	base, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	counter := NewBatchCounter(base, 1000)
	texts := generateRepeatedTexts(100, 20, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.CountTokensBatch(texts)
	}
}

func BenchmarkBatchCounter_Large(b *testing.B) {
	base, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	counter := NewBatchCounter(base, 5000)
	texts := generateRepeatedTexts(500, 100, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.CountTokensBatch(texts)
	}
}

func BenchmarkBatchCounter_Total(b *testing.B) {
	base, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	counter := NewBatchCounter(base, 1000)
	texts := generateRepeatedTexts(100, 20, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.CountTokensTotal(texts)
	}
}

// IncrementalCounter Benchmarks

func BenchmarkIncrementalCounter_Delta(b *testing.B) {
	base, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	counter := NewIncrementalCounter(base, 1000)
	oldText := "This is the old text that will be replaced"
	newText := "This is the new text that replaces the old"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.CountDelta(oldText, newText)
	}
}

func BenchmarkIncrementalCounter_UpdateTotal(b *testing.B) {
	base, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	counter := NewIncrementalCounter(base, 1000)
	oldText := "Old message content"
	newText := "Updated message content with more text"

	b.ResetTimer()
	total := 1000
	for i := 0; i < b.N; i++ {
		total = counter.UpdateTotal(total, oldText, newText)
	}
}

// Comparison Benchmarks - Old vs New

func BenchmarkComparison_Sequential_Old(b *testing.B) {
	counter, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	texts := generateRepeatedTexts(100, 20, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, text := range texts {
			counter.CountTokens(text)
		}
	}
}

func BenchmarkComparison_Sequential_New(b *testing.B) {
	base, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	counter := NewCachedCounter(base, 1000)
	texts := generateRepeatedTexts(100, 20, 100)

	// Prime cache
	for _, text := range texts[:20] {
		counter.CountTokens(text)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, text := range texts {
			counter.CountTokens(text)
		}
	}
}

// Conversation Simulation Benchmarks

func BenchmarkConversationSimulation_Small(b *testing.B) {
	base, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	counter := NewCachedCounter(base, 1000)

	// Simulate a small conversation (10 messages, frequently re-counted)
	messages := generateRepeatedTexts(10, 8, 200)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total := 0
		for _, msg := range messages {
			total += counter.CountTokens(msg)
		}
	}
}

func BenchmarkConversationSimulation_Large(b *testing.B) {
	base, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	counter := NewCachedCounter(base, 5000)

	// Simulate a large conversation (100 messages)
	messages := generateRepeatedTexts(100, 50, 300)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total := 0
		for _, msg := range messages {
			total += counter.CountTokens(msg)
		}
	}
}

// Memory Allocation Benchmarks

func BenchmarkAllocation_StringConstruction(b *testing.B) {
	counter := NewFallbackCounter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate building strings (like JSON serialization)
		var builder strings.Builder
		builder.WriteString("message content here")
		text := builder.String()
		counter.CountTokens(text)
	}
}

func BenchmarkAllocation_DirectString(b *testing.B) {
	counter := NewFallbackCounter()
	text := "message content here"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.CountTokens(text)
	}
}

// Cache Size Impact Benchmarks

func BenchmarkCacheSize_100(b *testing.B) {
	benchmarkCacheSize(b, 100)
}

func BenchmarkCacheSize_1000(b *testing.B) {
	benchmarkCacheSize(b, 1000)
}

func BenchmarkCacheSize_10000(b *testing.B) {
	benchmarkCacheSize(b, 10000)
}

func benchmarkCacheSize(b *testing.B, cacheSize int) {
	base, err := NewTiktokenCounterWithCache(EncodingO200kBase, cacheSize)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	// Generate texts with working set slightly larger than cache
	texts := generateRepeatedTexts(cacheSize*2, cacheSize/2, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		base.CountTokens(texts[i%len(texts)])
	}
}

// Real-world scenario benchmarks

func BenchmarkRealWorld_MessageHistory(b *testing.B) {
	base, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	counter := NewCachedCounter(base, 1000)

	// Simulate message history with system message + conversation
	systemMsg := "You are a helpful AI assistant."
	userMessages := generateRepeatedTexts(50, 20, 150)
	assistantMessages := generateRepeatedTexts(50, 25, 300)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total := counter.CountTokens(systemMsg)
		for j := 0; j < len(userMessages); j++ {
			total += counter.CountTokens(userMessages[j])
			total += counter.CountTokens(assistantMessages[j])
		}
		_ = total
	}
}

func BenchmarkRealWorld_StreamingResponse(b *testing.B) {
	base, err := NewTiktokenCounter(EncodingO200kBase)
	if err != nil {
		b.Skipf("Skipping: %v", err)
	}

	counter := NewIncrementalCounter(base, 500)

	// Simulate streaming response building up token by token
	chunks := []string{
		"Hello",
		"Hello, how",
		"Hello, how can",
		"Hello, how can I",
		"Hello, how can I help",
		"Hello, how can I help you",
		"Hello, how can I help you today?",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total := 0
		for j := 1; j < len(chunks); j++ {
			total = counter.UpdateTotal(total, chunks[j-1], chunks[j])
		}
	}
}
