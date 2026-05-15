package patch

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// FuzzyMatchConfig controls the behavior of fuzzy matching.
type FuzzyMatchConfig struct {
	// EnableUnicodeNorm enables Unicode normalization (NFC).
	EnableUnicodeNorm bool

	// EnableWhitespaceNorm enables whitespace normalization.
	EnableWhitespaceNorm bool

	// EnableLineEndingNorm enables line ending normalization (CRLF → LF).
	EnableLineEndingNorm bool

	// FuzzyThreshold is the minimum similarity score (0.0-1.0) required for a fuzzy match.
	// 1.0 means exact match, 0.0 means any match. Recommended: 0.8-0.9.
	FuzzyThreshold float64
}

// DefaultFuzzyConfig returns a sensible default configuration.
func DefaultFuzzyConfig() FuzzyMatchConfig {
	return FuzzyMatchConfig{
		EnableUnicodeNorm:    true,
		EnableWhitespaceNorm: true,
		EnableLineEndingNorm: true,
		FuzzyThreshold:       0.85,
	}
}

// StrictConfig returns a config for strict matching (no normalization).
func StrictConfig() FuzzyMatchConfig {
	return FuzzyMatchConfig{
		EnableUnicodeNorm:    false,
		EnableWhitespaceNorm: false,
		EnableLineEndingNorm: false,
		FuzzyThreshold:       1.0,
	}
}

// NormalizedConfig returns a config for normalized matching without fuzzy scoring.
func NormalizedConfig() FuzzyMatchConfig {
	return FuzzyMatchConfig{
		EnableUnicodeNorm:    true,
		EnableWhitespaceNorm: true,
		EnableLineEndingNorm: true,
		FuzzyThreshold:       1.0, // Exact match after normalization
	}
}

// normalizeString applies all enabled normalizations to a string.
func normalizeString(s string, config FuzzyMatchConfig) string {
	result := s

	// Line ending normalization (CRLF → LF)
	if config.EnableLineEndingNorm {
		result = strings.ReplaceAll(result, "\r\n", "\n")
		result = strings.ReplaceAll(result, "\r", "\n")
	}

	// Unicode normalization (NFC - Canonical Decomposition followed by Canonical Composition)
	if config.EnableUnicodeNorm {
		result = norm.NFC.String(result)
	}

	// Whitespace normalization
	if config.EnableWhitespaceNorm {
		result = normalizeWhitespace(result)
	}

	return result
}

// normalizeWhitespace collapses multiple spaces/tabs to single space and trims trailing whitespace.
func normalizeWhitespace(s string) string {
	// Normalize line by line to preserve line structure
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		// Trim trailing whitespace
		line = strings.TrimRight(line, " \t")

		// Collapse multiple spaces/tabs to single space
		var result strings.Builder
		result.Grow(len(line))
		lastWasSpace := false

		for _, r := range line {
			if unicode.IsSpace(r) {
				if !lastWasSpace {
					result.WriteRune(' ')
					lastWasSpace = true
				}
			} else {
				result.WriteRune(r)
				lastWasSpace = false
			}
		}

		lines[i] = result.String()
	}

	return strings.Join(lines, "\n")
}

// fuzzyMatchLine attempts to match expected against actual with the given config.
// Returns true if the match succeeds according to the configured threshold.
func fuzzyMatchLine(expected, actual string, config FuzzyMatchConfig) bool {
	// Normalize both strings
	normExpected := normalizeString(expected, config)
	normActual := normalizeString(actual, config)

	// If threshold is 1.0, require exact match after normalization
	if config.FuzzyThreshold >= 1.0 {
		return normExpected == normActual
	}

	// Calculate similarity score
	similarity := calculateSimilarity(normExpected, normActual)
	return similarity >= config.FuzzyThreshold
}

// calculateSimilarity computes a similarity score between two strings using a combination of metrics.
// Returns a value between 0.0 (completely different) and 1.0 (identical).
func calculateSimilarity(s1, s2 string) float64 {
	// Quick exact match check
	if s1 == s2 {
		return 1.0
	}

	// If either is empty, return 0
	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}

	// Use Levenshtein distance for similarity
	distance := levenshteinDistance(s1, s2)
	maxLen := max(utf8.RuneCountInString(s1), utf8.RuneCountInString(s2))

	// Convert distance to similarity score
	similarity := 1.0 - float64(distance)/float64(maxLen)

	// Ensure result is in [0, 1]
	if similarity < 0 {
		return 0
	}
	return similarity
}

// levenshteinDistance calculates the Levenshtein distance between two strings.
// This is the minimum number of single-character edits (insertions, deletions, or substitutions)
// required to change one string into the other.
func levenshteinDistance(s1, s2 string) int {
	// Convert to rune slices for proper Unicode handling
	r1 := []rune(s1)
	r2 := []rune(s2)

	len1 := len(r1)
	len2 := len(r2)

	// Create a 2D matrix for dynamic programming
	// Optimize space by using two rows instead of full matrix
	if len1 == 0 {
		return len2
	}
	if len2 == 0 {
		return len1
	}

	// Use rolling array optimization
	previousRow := make([]int, len2+1)
	currentRow := make([]int, len2+1)

	// Initialize first row
	for j := 0; j <= len2; j++ {
		previousRow[j] = j
	}

	// Calculate distances
	for i := 1; i <= len1; i++ {
		currentRow[0] = i

		for j := 1; j <= len2; j++ {
			cost := 1
			if r1[i-1] == r2[j-1] {
				cost = 0
			}

			currentRow[j] = min(
				min(currentRow[j-1]+1, previousRow[j]+1), // Insertion and Deletion
				previousRow[j-1]+cost,                     // Substitution
			)
		}

		// Swap rows
		previousRow, currentRow = currentRow, previousRow
	}

	return previousRow[len2]
}

// findFuzzyMatch attempts to find the best matching line in the actual content
// for the expected line, starting from the hint line number.
// Returns the line index if found, or -1 if no suitable match is found.
func findFuzzyMatch(expected string, actualLines []string, hintLine int, config FuzzyMatchConfig, maxOffset int) int {
	if hintLine < 0 || hintLine >= len(actualLines) {
		return -1
	}

	// Try exact match at hint location first
	if fuzzyMatchLine(expected, actualLines[hintLine], config) {
		return hintLine
	}

	// Search in a window around the hint line
	bestScore := 0.0
	bestIndex := -1

	// Calculate search window
	searchStart := max(0, hintLine-maxOffset)
	searchEnd := min(len(actualLines), hintLine+maxOffset+1)

	for i := searchStart; i < searchEnd; i++ {
		// Calculate similarity
		normExpected := normalizeString(expected, config)
		normActual := normalizeString(actualLines[i], config)
		similarity := calculateSimilarity(normExpected, normActual)

		// Update best match if this is better
		if similarity > bestScore && similarity >= config.FuzzyThreshold {
			bestScore = similarity
			bestIndex = i
		}
	}

	return bestIndex
}

// MatchStrategy represents the approach used for matching.
type MatchStrategy int

const (
	// MatchStrategyStrict uses byte-level exact matching.
	MatchStrategyStrict MatchStrategy = iota

	// MatchStrategyNormalized uses exact matching after normalization.
	MatchStrategyNormalized

	// MatchStrategyFuzzy uses fuzzy matching with similarity scoring.
	MatchStrategyFuzzy
)

// String returns the name of the match strategy.
func (m MatchStrategy) String() string {
	switch m {
	case MatchStrategyStrict:
		return "strict"
	case MatchStrategyNormalized:
		return "normalized"
	case MatchStrategyFuzzy:
		return "fuzzy"
	default:
		return "unknown"
	}
}

// FuzzyMatchResult contains information about a fuzzy match attempt.
type FuzzyMatchResult struct {
	// Matched indicates whether a match was found.
	Matched bool

	// Strategy is the strategy that succeeded.
	Strategy MatchStrategy

	// LineOffset is the offset from the expected line (0 means exact position).
	LineOffset int

	// Similarity is the similarity score (1.0 for exact match).
	Similarity float64
}

// tryMatchWithFallback attempts to match a line using multiple strategies:
// 1. Strict byte-level matching
// 2. Normalized matching (after Unicode/whitespace/line ending normalization)
// 3. Fuzzy matching with similarity scoring
func tryMatchWithFallback(expected, actual string, config FuzzyMatchConfig) (bool, MatchStrategy) {
	// Strategy 1: Strict matching
	if expected == actual {
		return true, MatchStrategyStrict
	}

	// Strategy 2: Normalized matching
	normConfig := NormalizedConfig()
	if fuzzyMatchLine(expected, actual, normConfig) {
		return true, MatchStrategyNormalized
	}

	// Strategy 3: Fuzzy matching
	if fuzzyMatchLine(expected, actual, config) {
		return true, MatchStrategyFuzzy
	}

	return false, MatchStrategyStrict
}

// Helper functions

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// calculateMatchQuality computes a quality metric for a match.
// Higher values indicate better matches.
func calculateMatchQuality(expected, actual string, strategy MatchStrategy) float64 {
	switch strategy {
	case MatchStrategyStrict:
		if expected == actual {
			return 1.0
		}
		return 0.0
	case MatchStrategyNormalized:
		normConfig := NormalizedConfig()
		normExpected := normalizeString(expected, normConfig)
		normActual := normalizeString(actual, normConfig)
		if normExpected == normActual {
			return 0.95 // Slightly lower than strict
		}
		return 0.0
	case MatchStrategyFuzzy:
		config := DefaultFuzzyConfig()
		normExpected := normalizeString(expected, config)
		normActual := normalizeString(actual, config)
		return calculateSimilarity(normExpected, normActual)
	default:
		return 0.0
	}
}

// NormalizeDiff normalizes an entire diff string for better matching.
// This is useful for pre-processing diffs before parsing.
func NormalizeDiff(diff string, config FuzzyMatchConfig) string {
	// Don't normalize the diff structure markers (---, +++, @@)
	lines := strings.Split(diff, "\n")
	result := make([]string, len(lines))

	for i, line := range lines {
		// Skip normalizing diff markers and hunk headers
		if strings.HasPrefix(line, "---") ||
			strings.HasPrefix(line, "+++") ||
			strings.HasPrefix(line, "@@") ||
			strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "index ") {
			result[i] = line
			continue
		}

		// For content lines (starting with space, +, -), normalize the content part
		if len(line) > 0 {
			prefix := line[0:1]
			content := line[1:]
			if prefix == " " || prefix == "+" || prefix == "-" {
				result[i] = prefix + normalizeString(content, config)
				continue
			}
		}

		// For other lines, normalize as-is
		result[i] = normalizeString(line, config)
	}

	return strings.Join(result, "\n")
}

// CompareWithMetrics compares two strings and returns detailed metrics.
type ComparisonMetrics struct {
	Exact           bool
	NormalizedEqual bool
	LevenshteinDist int
	Similarity      float64
	CharDifferences int
	WhitespaceDiff  bool
	UnicodeFormDiff bool
	LineEndingDiff  bool
}

// Compare performs a detailed comparison of two strings.
func Compare(s1, s2 string) ComparisonMetrics {
	metrics := ComparisonMetrics{
		Exact: s1 == s2,
	}

	// Check various normalizations
	norm1 := norm.NFC.String(s1)
	norm2 := norm.NFC.String(s2)
	metrics.UnicodeFormDiff = (norm1 != norm2) && (s1 == norm1 || s2 == norm2)

	ws1 := normalizeWhitespace(s1)
	ws2 := normalizeWhitespace(s2)
	metrics.WhitespaceDiff = (ws1 != ws2) && (s1 != ws1 || s2 != ws2)

	le1 := strings.ReplaceAll(strings.ReplaceAll(s1, "\r\n", "\n"), "\r", "\n")
	le2 := strings.ReplaceAll(strings.ReplaceAll(s2, "\r\n", "\n"), "\r", "\n")
	metrics.LineEndingDiff = (le1 != le2) && (s1 != le1 || s2 != le2)

	// Full normalization check
	config := DefaultFuzzyConfig()
	normFull1 := normalizeString(s1, config)
	normFull2 := normalizeString(s2, config)
	metrics.NormalizedEqual = normFull1 == normFull2

	// Calculate distance and similarity
	metrics.LevenshteinDist = levenshteinDistance(s1, s2)
	metrics.Similarity = calculateSimilarity(s1, s2)

	// Character differences
	runes1 := []rune(s1)
	runes2 := []rune(s2)
	maxLen := int(math.Max(float64(len(runes1)), float64(len(runes2))))
	minLen := int(math.Min(float64(len(runes1)), float64(len(runes2))))

	diff := 0
	for i := 0; i < minLen; i++ {
		if runes1[i] != runes2[i] {
			diff++
		}
	}
	diff += maxLen - minLen
	metrics.CharDifferences = diff

	return metrics
}
