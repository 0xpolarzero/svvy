package tokencount_test

import (
	"fmt"

	"github.com/evmts/codex/codex-go/internal/tokencount"
)

// ExampleNewFallbackCounter demonstrates using the fallback counter (4 bytes per token)
func ExampleNewFallbackCounter() {
	counter := tokencount.NewFallbackCounter()
	text := "hello world"
	count := counter.CountTokens(text)
	fmt.Printf("Tokens: %d\n", count)
	// Output: Tokens: 3
}

// ExampleNewDefaultCounter demonstrates using the default counter with tiktoken
func ExampleNewDefaultCounter() {
	counter := tokencount.NewDefaultCounter()
	text := "hello world"
	count := counter.CountTokens(text)
	// Token count will vary by encoding, but should be around 2-3
	fmt.Printf("Tokens > 0: %v\n", count > 0)
	// Output: Tokens > 0: true
}

// ExampleNewTiktokenCounter demonstrates using a specific tiktoken encoding
func ExampleNewTiktokenCounter() {
	counter, err := tokencount.NewTiktokenCounter(tokencount.EncodingCl100kBase)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	text := "hello world"
	count := counter.CountTokens(text)
	fmt.Printf("Tokens: %d\n", count)
	// Output: Tokens: 2
}

// ExampleNewCounterForModel demonstrates creating a counter for a specific model
func ExampleNewCounterForModel() {
	counter, err := tokencount.NewCounterForModel("gpt-4")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	text := "The quick brown fox jumps over the lazy dog"
	count := counter.CountTokens(text)
	fmt.Printf("Tokens > 0: %v\n", count > 0)
	// Output: Tokens > 0: true
}

// ExampleCountTokensWithFallback demonstrates the convenience function
func ExampleCountTokensWithFallback() {
	text := "This is a test string"
	count := tokencount.CountTokensWithFallback(text)
	fmt.Printf("Tokens > 0: %v\n", count > 0)
	// Output: Tokens > 0: true
}
