package patch

import (
	"strings"
	"unicode"
)

// ContextMatchConfig defines the configuration for context-based patch seeking.
type ContextMatchConfig struct {
	// SearchWindowSize is the number of lines to search before and after the expected position.
	// A window of 50 means we search from (expected-50) to (expected+50).
	SearchWindowSize int
}

// DefaultContextMatchConfig returns the default configuration for context matching.
func DefaultContextMatchConfig() ContextMatchConfig {
	return ContextMatchConfig{
		SearchWindowSize: 50, // Search within ±50 lines of expected position
	}
}

// ContextMatchResult represents the result of a context search operation.
type ContextMatchResult struct {
	// Found indicates whether a match was found.
	Found bool
	// StartLine is the 0-based line index where the pattern starts (if found).
	StartLine int
	// MatchType indicates which matching strategy succeeded.
	MatchType MatchType
}

// MatchType indicates which matching strategy was used.
type MatchType int

const (
	// MatchExact indicates an exact byte-for-byte match.
	MatchExact MatchType = iota
	// MatchTrimEnd indicates a match ignoring trailing whitespace.
	MatchTrimEnd
	// MatchTrimBoth indicates a match ignoring leading and trailing whitespace.
	MatchTrimBoth
	// MatchNormalized indicates a match after normalizing Unicode punctuation.
	MatchNormalized
)

func (mt MatchType) String() string {
	switch mt {
	case MatchExact:
		return "exact"
	case MatchTrimEnd:
		return "trim_end"
	case MatchTrimBoth:
		return "trim_both"
	case MatchNormalized:
		return "normalized"
	default:
		return "unknown"
	}
}

// seekSequence attempts to find the sequence of pattern lines within lines beginning
// at or after start. Returns the starting index of the match or -1 if not found.
// Matches are attempted with decreasing strictness:
//  1. Exact match
//  2. Ignoring trailing whitespace
//  3. Ignoring leading and trailing whitespace
//  4. After normalizing Unicode punctuation to ASCII equivalents
//
// When eof is true, we first try starting at the end-of-file (so that patterns
// intended to match file endings are applied at the end), and fall back to
// searching from start if needed.
//
// Special cases handled defensively:
//   - Empty pattern → returns start (no-op match)
//   - pattern length > lines length → returns -1 (cannot match)
func seekSequence(lines []string, pattern []string, start int, eof bool) ContextMatchResult {
	// Empty pattern matches at the start position
	if len(pattern) == 0 {
		return ContextMatchResult{Found: true, StartLine: start, MatchType: MatchExact}
	}

	// When the pattern is longer than the available input there is no possible match
	if len(pattern) > len(lines) {
		return ContextMatchResult{Found: false}
	}

	// Determine search start position
	searchStart := start
	if eof && len(lines) >= len(pattern) {
		searchStart = len(lines) - len(pattern)
	}

	// Try exact match first
	if result := tryExactMatch(lines, pattern, searchStart); result.Found {
		return result
	}

	// Try match ignoring trailing whitespace
	if result := tryTrimEndMatch(lines, pattern, searchStart); result.Found {
		return result
	}

	// Try match ignoring leading and trailing whitespace
	if result := tryTrimBothMatch(lines, pattern, searchStart); result.Found {
		return result
	}

	// Try match after normalizing Unicode punctuation
	if result := tryNormalizedMatch(lines, pattern, searchStart); result.Found {
		return result
	}

	return ContextMatchResult{Found: false}
}

// seekSequenceWithWindow searches for a pattern within a configurable window
// around the expected position, using multiple matching strategies.
func seekSequenceWithWindow(lines []string, pattern []string, expectedStart int, config ContextMatchConfig, eof bool) ContextMatchResult {
	if len(pattern) == 0 {
		return ContextMatchResult{Found: true, StartLine: expectedStart, MatchType: MatchExact}
	}

	if len(pattern) > len(lines) {
		return ContextMatchResult{Found: false}
	}

	// Calculate search window bounds
	windowStart := expectedStart - config.SearchWindowSize
	if windowStart < 0 {
		windowStart = 0
	}

	windowEnd := expectedStart + config.SearchWindowSize
	maxEnd := len(lines) - len(pattern) + 1
	if windowEnd > maxEnd {
		windowEnd = maxEnd
	}

	// Special handling for EOF patterns
	var searchStart int
	if eof && len(lines) >= len(pattern) {
		// For EOF patterns, prioritize searching from the end
		searchStart = len(lines) - len(pattern)
		if searchStart < windowStart {
			searchStart = windowStart
		}
	} else {
		searchStart = windowStart
	}

	// Try each matching strategy in order of strictness
	strategies := []struct {
		name  MatchType
		match func([]string, []string, int, int) ContextMatchResult
	}{
		{MatchExact, tryExactMatchInWindow},
		{MatchTrimEnd, tryTrimEndMatchInWindow},
		{MatchTrimBoth, tryTrimBothMatchInWindow},
		{MatchNormalized, tryNormalizedMatchInWindow},
	}

	for _, strategy := range strategies {
		if result := strategy.match(lines, pattern, searchStart, windowEnd); result.Found {
			return result
		}
	}

	return ContextMatchResult{Found: false}
}

// tryExactMatch attempts an exact byte-for-byte match.
func tryExactMatch(lines []string, pattern []string, searchStart int) ContextMatchResult {
	maxStart := len(lines) - len(pattern) + 1
	for i := searchStart; i < maxStart; i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if lines[i+j] != pattern[j] {
				match = false
				break
			}
		}
		if match {
			return ContextMatchResult{Found: true, StartLine: i, MatchType: MatchExact}
		}
	}
	return ContextMatchResult{Found: false}
}

// tryTrimEndMatch attempts a match ignoring trailing whitespace.
func tryTrimEndMatch(lines []string, pattern []string, searchStart int) ContextMatchResult {
	maxStart := len(lines) - len(pattern) + 1
	for i := searchStart; i < maxStart; i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if strings.TrimRight(lines[i+j], " \t") != strings.TrimRight(pattern[j], " \t") {
				match = false
				break
			}
		}
		if match {
			return ContextMatchResult{Found: true, StartLine: i, MatchType: MatchTrimEnd}
		}
	}
	return ContextMatchResult{Found: false}
}

// tryTrimBothMatch attempts a match ignoring leading and trailing whitespace.
func tryTrimBothMatch(lines []string, pattern []string, searchStart int) ContextMatchResult {
	maxStart := len(lines) - len(pattern) + 1
	for i := searchStart; i < maxStart; i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if strings.TrimSpace(lines[i+j]) != strings.TrimSpace(pattern[j]) {
				match = false
				break
			}
		}
		if match {
			return ContextMatchResult{Found: true, StartLine: i, MatchType: MatchTrimBoth}
		}
	}
	return ContextMatchResult{Found: false}
}

// tryNormalizedMatch attempts a match after normalizing Unicode punctuation.
func tryNormalizedMatch(lines []string, pattern []string, searchStart int) ContextMatchResult {
	maxStart := len(lines) - len(pattern) + 1
	for i := searchStart; i < maxStart; i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if normalizeUnicode(lines[i+j]) != normalizeUnicode(pattern[j]) {
				match = false
				break
			}
		}
		if match {
			return ContextMatchResult{Found: true, StartLine: i, MatchType: MatchNormalized}
		}
	}
	return ContextMatchResult{Found: false}
}

// Window-based matching functions

func tryExactMatchInWindow(lines []string, pattern []string, windowStart, windowEnd int) ContextMatchResult {
	for i := windowStart; i < windowEnd; i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if lines[i+j] != pattern[j] {
				match = false
				break
			}
		}
		if match {
			return ContextMatchResult{Found: true, StartLine: i, MatchType: MatchExact}
		}
	}
	return ContextMatchResult{Found: false}
}

func tryTrimEndMatchInWindow(lines []string, pattern []string, windowStart, windowEnd int) ContextMatchResult {
	for i := windowStart; i < windowEnd; i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if strings.TrimRight(lines[i+j], " \t") != strings.TrimRight(pattern[j], " \t") {
				match = false
				break
			}
		}
		if match {
			return ContextMatchResult{Found: true, StartLine: i, MatchType: MatchTrimEnd}
		}
	}
	return ContextMatchResult{Found: false}
}

func tryTrimBothMatchInWindow(lines []string, pattern []string, windowStart, windowEnd int) ContextMatchResult {
	for i := windowStart; i < windowEnd; i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if strings.TrimSpace(lines[i+j]) != strings.TrimSpace(pattern[j]) {
				match = false
				break
			}
		}
		if match {
			return ContextMatchResult{Found: true, StartLine: i, MatchType: MatchTrimBoth}
		}
	}
	return ContextMatchResult{Found: false}
}

func tryNormalizedMatchInWindow(lines []string, pattern []string, windowStart, windowEnd int) ContextMatchResult {
	for i := windowStart; i < windowEnd; i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if normalizeUnicode(lines[i+j]) != normalizeUnicode(pattern[j]) {
				match = false
				break
			}
		}
		if match {
			return ContextMatchResult{Found: true, StartLine: i, MatchType: MatchNormalized}
		}
	}
	return ContextMatchResult{Found: false}
}

// normalizeUnicode normalizes a string by:
//   - Trimming leading and trailing whitespace
//   - Converting various Unicode dashes to ASCII hyphen-minus
//   - Converting Unicode quotes to ASCII quotes
//   - Converting non-breaking and other special spaces to regular space
//
// This mirrors the fuzzy behaviour of `git apply` which ignores minor
// byte-level differences when locating context lines.
func normalizeUnicode(s string) string {
	var builder strings.Builder
	builder.Grow(len(s))

	for _, r := range strings.TrimSpace(s) {
		switch r {
		// Various dash/hyphen code-points → ASCII '-'
		case '\u2010', '\u2011', '\u2012', '\u2013', '\u2014', '\u2015', '\u2212':
			builder.WriteRune('-')
		// Fancy single quotes → '\''
		case '\u2018', '\u2019', '\u201A', '\u201B':
			builder.WriteRune('\'')
		// Fancy double quotes → '"'
		case '\u201C', '\u201D', '\u201E', '\u201F':
			builder.WriteRune('"')
		// Non-breaking space and other odd spaces → normal space
		case '\u00A0', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006',
			'\u2007', '\u2008', '\u2009', '\u200A', '\u202F', '\u205F', '\u3000':
			builder.WriteRune(' ')
		default:
			builder.WriteRune(r)
		}
	}

	return builder.String()
}

// calculateMatchScore computes a score for how good a match is.
// Higher scores indicate better matches. This is used when multiple
// potential matches exist within the search window.
func calculateMatchScore(result ContextMatchResult, expectedStart int) int {
	if !result.Found {
		return -1
	}

	// Base score depends on match type (stricter matches get higher scores)
	var baseScore int
	switch result.MatchType {
	case MatchExact:
		baseScore = 1000
	case MatchTrimEnd:
		baseScore = 750
	case MatchTrimBoth:
		baseScore = 500
	case MatchNormalized:
		baseScore = 250
	default:
		baseScore = 0
	}

	// Penalize matches that are farther from the expected position
	distance := abs(result.StartLine - expectedStart)
	distancePenalty := distance

	return baseScore - distancePenalty
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// findBestMatch finds the best match among multiple potential matches.
// It prioritizes matches closer to the expected position and stricter match types.
func findBestMatch(results []ContextMatchResult, expectedStart int) ContextMatchResult {
	if len(results) == 0 {
		return ContextMatchResult{Found: false}
	}

	bestResult := results[0]
	bestScore := calculateMatchScore(bestResult, expectedStart)

	for i := 1; i < len(results); i++ {
		score := calculateMatchScore(results[i], expectedStart)
		if score > bestScore {
			bestScore = score
			bestResult = results[i]
		}
	}

	return bestResult
}

// isWhitespace checks if a rune is a whitespace character.
func isWhitespace(r rune) bool {
	return unicode.IsSpace(r)
}
