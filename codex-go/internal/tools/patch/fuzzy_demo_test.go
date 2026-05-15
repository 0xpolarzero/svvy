package patch

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSuccessRateComparison compares strict vs fuzzy matching success rates
func TestSuccessRateComparison(t *testing.T) {
	fmt.Println("\n=== Success Rate Comparison: Strict vs Fuzzy ===")

	// Real-world scenarios that fail with strict matching
	scenarios := []struct {
		expected string
		actual   string
		scenario string
	}{
		{"return a+b;", "return a + b;", "formatter added spaces"},
		{"line1", "line1   ", "trailing whitespace"},
		{"func(x)", "func(x)\t", "trailing tab"},
		{"\tif (x) {", "    if (x) {", "tabs vs spaces"},
		{"café", "cafe\u0301", "unicode normalization"},
		{"line\nbreak", "line\r\nbreak", "CRLF vs LF"},
		{"const X=10;", "const  X = 10 ;", "inconsistent spacing"},
		{"//comment", "// comment", "space after //"},
	}

	strictSuccesses := 0
	fuzzySuccesses := 0

	fmt.Println("Testing real-world formatting differences:")

	for i, scenario := range scenarios {
		// Try strict matching
		strictMatch := scenario.expected == scenario.actual

		// Try fuzzy matching
		fuzzyMatch, _ := tryMatchWithFallback(scenario.expected, scenario.actual, DefaultFuzzyConfig())

		if strictMatch {
			strictSuccesses++
		}
		if fuzzyMatch {
			fuzzySuccesses++
		}

		status := "✗"
		if fuzzyMatch {
			status = "✓"
		}

		fmt.Printf("%s #%d %s\n", status, i+1, scenario.scenario)
		fmt.Printf("   Expected: %q\n", scenario.expected)
		fmt.Printf("   Actual:   %q\n", scenario.actual)
		fmt.Printf("   Strict: %v, Fuzzy: %v\n\n", strictMatch, fuzzyMatch)
	}

	strictRate := float64(strictSuccesses) / float64(len(scenarios)) * 100
	fuzzyRate := float64(fuzzySuccesses) / float64(len(scenarios)) * 100
	improvement := fuzzyRate - strictRate

	fmt.Println("Results:")
	fmt.Printf("  Strict Matching: %d/%d (%.1f%%)\n", strictSuccesses, len(scenarios), strictRate)
	fmt.Printf("  Fuzzy Matching:  %d/%d (%.1f%%)\n", fuzzySuccesses, len(scenarios), fuzzyRate)
	fmt.Printf("  Improvement:     +%.1f%%\n", improvement)

	assert.Greater(t, fuzzySuccesses, strictSuccesses, "Fuzzy matching should succeed more often")
	fmt.Println()
}
