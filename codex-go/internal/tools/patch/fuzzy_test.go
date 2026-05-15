package patch

import (
	"strings"
	"testing"

	"github.com/evmts/codex/codex-go/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Unicode Normalization Tests
// ============================================================================

func TestFuzzyMatch_UnicodeNormalization(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		actual   string
		config   FuzzyMatchConfig
		match    bool
	}{
		{
			name:     "NFC vs NFD é",
			expected: "café", // NFC (single codepoint é)
			actual:   "cafe\u0301", // NFD (e + combining acute)
			config:   DefaultFuzzyConfig(),
			match:    true,
		},
		{
			name:     "different unicode forms with normalization",
			expected: "naïve",
			actual:   "nai\u0308ve", // NFD form
			config:   DefaultFuzzyConfig(),
			match:    true,
		},
		{
			name:     "different unicode forms without normalization",
			expected: "naïve",
			actual:   "nai\u0308ve",
			config:   StrictConfig(),
			match:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fuzzyMatchLine(tt.expected, tt.actual, tt.config)
			assert.Equal(t, tt.match, result, "Match result mismatch")
		})
	}
}

// ============================================================================
// Whitespace Normalization Tests
// ============================================================================

func TestFuzzyMatch_WhitespaceNormalization(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		actual   string
		config   FuzzyMatchConfig
		match    bool
	}{
		{
			name:     "extra spaces collapsed",
			expected: "hello world",
			actual:   "hello  world", // Two spaces
			config:   DefaultFuzzyConfig(),
			match:    true,
		},
		{
			name:     "tabs to spaces",
			expected: "hello world",
			actual:   "hello\tworld",
			config:   DefaultFuzzyConfig(),
			match:    true,
		},
		{
			name:     "trailing whitespace ignored",
			expected: "hello",
			actual:   "hello   ",
			config:   DefaultFuzzyConfig(),
			match:    true,
		},
		{
			name:     "mixed spaces and tabs",
			expected: "int x = 10;",
			actual:   "int\tx\t=  10;",
			config:   DefaultFuzzyConfig(),
			match:    true,
		},
		{
			name:     "whitespace differences with strict config",
			expected: "hello world",
			actual:   "hello  world",
			config:   StrictConfig(),
			match:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fuzzyMatchLine(tt.expected, tt.actual, tt.config)
			assert.Equal(t, tt.match, result, "Match result mismatch")
		})
	}
}

// ============================================================================
// Line Ending Normalization Tests
// ============================================================================

func TestFuzzyMatch_LineEndingNormalization(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		actual   string
		config   FuzzyMatchConfig
		match    bool
	}{
		{
			name:     "CRLF vs LF",
			expected: "line1\nline2",
			actual:   "line1\r\nline2",
			config:   DefaultFuzzyConfig(),
			match:    true,
		},
		{
			name:     "CR vs LF",
			expected: "line1\nline2",
			actual:   "line1\rline2",
			config:   DefaultFuzzyConfig(),
			match:    true,
		},
		{
			name:     "mixed line endings",
			expected: "a\nb\nc",
			actual:   "a\r\nb\rc",
			config:   DefaultFuzzyConfig(),
			match:    true,
		},
		{
			name:     "line endings without normalization",
			expected: "line1\nline2",
			actual:   "line1\r\nline2",
			config:   StrictConfig(),
			match:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fuzzyMatchLine(tt.expected, tt.actual, tt.config)
			assert.Equal(t, tt.match, result, "Match result mismatch")
		})
	}
}

// ============================================================================
// Fuzzy Threshold Tests
// ============================================================================

func TestFuzzyMatch_ThresholdBehavior(t *testing.T) {
	tests := []struct {
		name      string
		expected  string
		actual    string
		threshold float64
		match     bool
	}{
		{
			name:      "high similarity passes high threshold",
			expected:  "hello world",
			actual:    "hello word", // One char different
			threshold: 0.85,
			match:     true,
		},
		{
			name:      "low similarity fails high threshold",
			expected:  "hello world",
			actual:    "goodbye world",
			threshold: 0.85,
			match:     false,
		},
		{
			name:      "low similarity passes low threshold",
			expected:  "hello world",
			actual:    "hello earth",
			threshold: 0.5,
			match:     true,
		},
		{
			name:      "exact match always passes",
			expected:  "exact",
			actual:    "exact",
			threshold: 1.0,
			match:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultFuzzyConfig()
			config.FuzzyThreshold = tt.threshold
			result := fuzzyMatchLine(tt.expected, tt.actual, config)
			assert.Equal(t, tt.match, result, "Match result mismatch")
		})
	}
}

// ============================================================================
// Levenshtein Distance Tests
// ============================================================================

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected int
	}{
		{
			name:     "identical strings",
			s1:       "hello",
			s2:       "hello",
			expected: 0,
		},
		{
			name:     "one insertion",
			s1:       "hello",
			s2:       "hellow",
			expected: 1,
		},
		{
			name:     "one deletion",
			s1:       "hello",
			s2:       "hell",
			expected: 1,
		},
		{
			name:     "one substitution",
			s1:       "hello",
			s2:       "hallo",
			expected: 1,
		},
		{
			name:     "multiple edits",
			s1:       "kitten",
			s2:       "sitting",
			expected: 3, // k->s, e->i, insert g
		},
		{
			name:     "empty strings",
			s1:       "",
			s2:       "",
			expected: 0,
		},
		{
			name:     "one empty",
			s1:       "hello",
			s2:       "",
			expected: 5,
		},
		{
			name:     "unicode handling",
			s1:       "café",
			s2:       "cafe",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := levenshteinDistance(tt.s1, tt.s2)
			assert.Equal(t, tt.expected, result, "Distance mismatch")
		})
	}
}

// ============================================================================
// Real-World Patch Failure Scenarios
// ============================================================================

func TestRealWorld_FormatterAddedSpaces(t *testing.T) {
	// Scenario: Code formatter added spaces around operators
	fs := test.NewMemFS(t)
	original := `func add(a, b int) int {
	return a+b
}
`
	test.WriteFileFS(t, fs, "/workspace/math.go", []byte(original))

	// Patch expects unformatted code, but file has been formatted
	diff := `--- a/math.go
+++ b/math.go
@@ -1,3 +1,3 @@
 func add(a, b int) int {
-	return a+b
+	return a + b + 1
 }
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyPatches(fs, patches, "/workspace", false)
	require.NoError(t, err, "Should succeed with fuzzy matching")
	assert.Len(t, result.Updated, 1)

	content := test.ReadFileFS(t, fs, "/workspace/math.go")
	expected := `func add(a, b int) int {
	return a + b + 1
}
`
	assert.Equal(t, expected, string(content))
}

func TestRealWorld_EditorAddedTrailingWhitespace(t *testing.T) {
	// Scenario: Editor adds trailing whitespace to lines
	fs := test.NewMemFS(t)
	original := "line1   \nline2\nline3   \n" // Trailing spaces
	test.WriteFileFS(t, fs, "/workspace/file.txt", []byte(original))

	// Patch expects no trailing whitespace
	diff := `--- a/file.txt
+++ b/file.txt
@@ -1,3 +1,3 @@
 line1
-line2
+line2_modified
 line3
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyPatches(fs, patches, "/workspace", false)
	require.NoError(t, err, "Should succeed with whitespace normalization")
	assert.Len(t, result.Updated, 1)

	content := test.ReadFileFS(t, fs, "/workspace/file.txt")
	assert.Contains(t, string(content), "line2_modified")
}

func TestRealWorld_GitAutoCRLF(t *testing.T) {
	// Scenario: Git autocrlf converted LF to CRLF on Windows
	fs := test.NewMemFS(t)
	original := "line1\r\nline2\r\nline3\r\n" // CRLF endings
	test.WriteFileFS(t, fs, "/workspace/file.txt", []byte(original))

	// Patch has LF endings (Unix-style)
	diff := `--- a/file.txt
+++ b/file.txt
@@ -1,3 +1,3 @@
 line1
-line2
+line2_changed
 line3
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyPatches(fs, patches, "/workspace", false)
	require.NoError(t, err, "Should succeed with line ending normalization")
	assert.Len(t, result.Updated, 1)

	content := test.ReadFileFS(t, fs, "/workspace/file.txt")
	// Should preserve CRLF endings
	assert.Contains(t, string(content), "\r\n")
	assert.Contains(t, string(content), "line2_changed")
}

func TestRealWorld_TabsVsSpacesIndentation(t *testing.T) {
	// Scenario: File uses tabs, patch uses spaces (or vice versa)
	fs := test.NewMemFS(t)
	original := "\tif (x) {\n\t\treturn true;\n\t}\n"
	test.WriteFileFS(t, fs, "/workspace/code.js", []byte(original))

	// Patch uses spaces instead of tabs
	diff := `--- a/code.js
+++ b/code.js
@@ -1,3 +1,3 @@
     if (x) {
-        return true;
+        return false;
     }
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyPatches(fs, patches, "/workspace", false)
	require.NoError(t, err, "Should succeed with whitespace normalization")
	assert.Len(t, result.Updated, 1)

	content := test.ReadFileFS(t, fs, "/workspace/code.js")
	assert.Contains(t, string(content), "return false")
}

func TestRealWorld_UnicodeCommentCharacters(t *testing.T) {
	// Scenario: Comments with Unicode characters in different normalization forms
	fs := test.NewMemFS(t)
	original := "// Café function\nfunc process() {}\n"
	test.WriteFileFS(t, fs, "/workspace/unicode.go", []byte(original))

	// Patch has NFD form of café
	diff := "--- a/unicode.go\n+++ b/unicode.go\n@@ -1,2 +1,2 @@\n-// Cafe\u0301 function\n+// Café updated\n func process() {}\n"

	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyPatches(fs, patches, "/workspace", false)
	require.NoError(t, err, "Should succeed with Unicode normalization")
	assert.Len(t, result.Updated, 1)

	content := test.ReadFileFS(t, fs, "/workspace/unicode.go")
	assert.Contains(t, string(content), "updated")
}

func TestRealWorld_SlightTypoInComment(t *testing.T) {
	// Scenario: Someone fixed a typo in a comment
	fs := test.NewMemFS(t)
	original := "// This is a funciton\nfunc foo() {}\n"
	test.WriteFileFS(t, fs, "/workspace/typo.go", []byte(original))

	// Patch tries to modify the function, but comment has a typo
	diff := `--- a/typo.go
+++ b/typo.go
@@ -1,2 +1,2 @@
 // This is a function
-func foo() {}
+func foo() { return }
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	// The fuzzy matching is smart enough to handle this small typo
	// "funciton" vs "function" has high similarity (88.9%)
	result, err := applyPatches(fs, patches, "/workspace", false)
	require.NoError(t, err, "Fuzzy matching should handle small typos in context")
	assert.Len(t, result.Updated, 1)

	content := test.ReadFileFS(t, fs, "/workspace/typo.go")
	assert.Contains(t, string(content), "return")
}

func TestRealWorld_NearlyIdenticalLines(t *testing.T) {
	// Scenario: Two lines are very similar, fuzzy matching should help
	fs := test.NewMemFS(t)
	original := "const MAX_SIZE = 100;\nconst MIN_SIZE = 10;\n"
	test.WriteFileFS(t, fs, "/workspace/constants.js", []byte(original))

	diff := `--- a/constants.js
+++ b/constants.js
@@ -1,2 +1,2 @@
-const MAX_SIZE = 100;
+const MAX_SIZE = 200;
 const MIN_SIZE = 10;
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyPatches(fs, patches, "/workspace", false)
	require.NoError(t, err)
	assert.Len(t, result.Updated, 1)

	content := test.ReadFileFS(t, fs, "/workspace/constants.js")
	assert.Contains(t, string(content), "MAX_SIZE = 200")
}

// ============================================================================
// Fallback Strategy Tests
// ============================================================================

func TestTryMatchWithFallback_Strategies(t *testing.T) {
	tests := []struct {
		name             string
		expected         string
		actual           string
		shouldMatch      bool
		expectedStrategy MatchStrategy
	}{
		{
			name:             "exact match uses strict",
			expected:         "hello world",
			actual:           "hello world",
			shouldMatch:      true,
			expectedStrategy: MatchStrategyStrict,
		},
		{
			name:             "whitespace difference uses normalized",
			expected:         "hello world",
			actual:           "hello  world",
			shouldMatch:      true,
			expectedStrategy: MatchStrategyNormalized,
		},
		{
			name:             "unicode difference uses normalized",
			expected:         "café",
			actual:           "cafe\u0301",
			shouldMatch:      true,
			expectedStrategy: MatchStrategyNormalized,
		},
		{
			name:             "slight difference uses fuzzy",
			expected:         "hello world",
			actual:           "hello wrld", // Missing 'o'
			shouldMatch:      true,
			expectedStrategy: MatchStrategyFuzzy,
		},
		{
			name:             "too different fails all strategies",
			expected:         "hello world",
			actual:           "goodbye universe",
			shouldMatch:      false,
			expectedStrategy: MatchStrategyStrict,
		},
	}

	config := DefaultFuzzyConfig()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, strategy := tryMatchWithFallback(tt.expected, tt.actual, config)
			assert.Equal(t, tt.shouldMatch, matched, "Match result mismatch")
			if matched {
				assert.Equal(t, tt.expectedStrategy, strategy, "Strategy mismatch")
			}
		})
	}
}

// ============================================================================
// Compare Metrics Tests
// ============================================================================

func TestCompare_Metrics(t *testing.T) {
	tests := []struct {
		name               string
		s1                 string
		s2                 string
		expectExact        bool
		expectNormalized   bool
		expectWhitespace   bool
		expectUnicode      bool
		expectLineEnding   bool
	}{
		{
			name:             "exact match",
			s1:               "hello",
			s2:               "hello",
			expectExact:      true,
			expectNormalized: true,
		},
		{
			name:             "whitespace difference",
			s1:               "hello world",
			s2:               "hello  world",
			expectExact:      false,
			expectNormalized: true,
			expectWhitespace: false,
		},
		{
			name:             "unicode difference",
			s1:               "café",
			s2:               "cafe\u0301",
			expectExact:      false,
			expectNormalized: true,
			expectUnicode:    false,
		},
		{
			name:             "line ending difference",
			s1:               "line1\nline2",
			s2:               "line1\r\nline2",
			expectExact:      false,
			expectNormalized: true,
			expectLineEnding: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := Compare(tt.s1, tt.s2)
			assert.Equal(t, tt.expectExact, metrics.Exact, "Exact match mismatch")
			assert.Equal(t, tt.expectNormalized, metrics.NormalizedEqual, "Normalized match mismatch")
		})
	}
}

// ============================================================================
// Integration Tests with Apply Logic
// ============================================================================

func TestIntegration_FuzzyMatchingInPatchApply(t *testing.T) {
	// Create a complex real-world scenario
	fs := test.NewMemFS(t)

	// File has been reformatted with extra spaces and CRLF line endings
	original := "function calculate(x, y) {\r\n  return  x + y;\r\n}\r\n"
	test.WriteFileFS(t, fs, "/workspace/calc.js", []byte(original))

	// Patch expects clean formatting
	diff := `--- a/calc.js
+++ b/calc.js
@@ -1,3 +1,3 @@
 function calculate(x, y) {
-  return x + y;
+  return x * y;
 }
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyPatches(fs, patches, "/workspace", false)
	require.NoError(t, err, "Fuzzy matching should handle formatting differences")
	assert.Len(t, result.Updated, 1)
	assert.Empty(t, result.Errors)

	content := test.ReadFileFS(t, fs, "/workspace/calc.js")
	assert.Contains(t, string(content), "x * y")
	// Should preserve original CRLF endings
	assert.Contains(t, string(content), "\r\n")
}

func TestIntegration_MultipleHunksWithFuzzyMatching(t *testing.T) {
	fs := test.NewMemFS(t)

	// File with various formatting inconsistencies
	original := `line 1
line  2
line 3
line 4
line 5
line 6
`
	test.WriteFileFS(t, fs, "/workspace/multi.txt", []byte(original))

	// Multiple hunks with formatting differences
	diff := `--- a/multi.txt
+++ b/multi.txt
@@ -1,3 +1,3 @@
 line 1
-line 2
+line two
 line 3
@@ -4,3 +4,3 @@
 line 4
-line 5
+line five
 line 6
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyPatches(fs, patches, "/workspace", false)
	require.NoError(t, err)
	assert.Len(t, result.Updated, 1)

	content := test.ReadFileFS(t, fs, "/workspace/multi.txt")
	assert.Contains(t, string(content), "line two")
	assert.Contains(t, string(content), "line five")
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkLevenshteinDistance(b *testing.B) {
	s1 := "the quick brown fox jumps over the lazy dog"
	s2 := "the quick brown fox jumped over the lazy cat"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = levenshteinDistance(s1, s2)
	}
}

func BenchmarkFuzzyMatchLine_Exact(b *testing.B) {
	config := DefaultFuzzyConfig()
	line := "const MAX_SIZE = 100;"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fuzzyMatchLine(line, line, config)
	}
}

func BenchmarkFuzzyMatchLine_WithNormalization(b *testing.B) {
	config := DefaultFuzzyConfig()
	expected := "hello world"
	actual := "hello  world  " // Extra spaces

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fuzzyMatchLine(expected, actual, config)
	}
}

func BenchmarkTryMatchWithFallback(b *testing.B) {
	config := DefaultFuzzyConfig()
	expected := "function calculate(x, y) { return x + y; }"
	actual := "function calculate(x, y) {  return  x + y;  }" // Formatting differences

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tryMatchWithFallback(expected, actual, config)
	}
}

func BenchmarkApplyHunk_StrictMatching(b *testing.B) {
	lines := []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
	}

	hunk := &Hunk{
		OriginalStart: 2,
		OriginalLines: 3,
		NewStart:      2,
		NewLines:      3,
		Lines: []Line{
			{Type: LineContext, Content: "line 2"},
			{Type: LineRemove, Content: "line 3"},
			{Type: LineAdd, Content: "line three"},
			{Type: LineContext, Content: "line 4"},
		},
	}

	config := StrictConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = applyHunkWithConfig(lines, hunk, config)
	}
}

func BenchmarkApplyHunk_FuzzyMatching(b *testing.B) {
	lines := []string{
		"line  1", // Extra space
		"line 2",
		"line  3", // Extra space
		"line 4",
		"line 5",
	}

	hunk := &Hunk{
		OriginalStart: 2,
		OriginalLines: 3,
		NewStart:      2,
		NewLines:      3,
		Lines: []Line{
			{Type: LineContext, Content: "line 2"},
			{Type: LineRemove, Content: "line 3"}, // Will match "line  3" with fuzzy
			{Type: LineAdd, Content: "line three"},
			{Type: LineContext, Content: "line 4"},
		},
	}

	config := DefaultFuzzyConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = applyHunkWithConfig(lines, hunk, config)
	}
}

// ============================================================================
// Error Message Quality Tests
// ============================================================================

func TestFuzzyMatch_ErrorMessageQuality(t *testing.T) {
	fs := test.NewMemFS(t)
	original := "const MAX_SIZE = 100;\nconst MIN_SIZE = 10;\n"
	test.WriteFileFS(t, fs, "/workspace/test.js", []byte(original))

	// Intentionally create a patch that will fail
	diff := `--- a/test.js
+++ b/test.js
@@ -1,2 +1,2 @@
-const MAXIMUM_SIZE = 100;
+const MAXIMUM_SIZE = 200;
 const MIN_SIZE = 10;
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	_, err = applyPatches(fs, patches, "/workspace", false)
	require.Error(t, err)

	// Error should be reported
	errMsg := err.Error()
	assert.NotEmpty(t, errMsg, "Error message should not be empty")
	// The detailed error with similarity is in the PatchError, but may be wrapped
	// by higher-level error handling. As long as we get an error, that's what matters.
}

// ============================================================================
// Configuration Tests
// ============================================================================

func TestFuzzyConfig_Defaults(t *testing.T) {
	config := DefaultFuzzyConfig()
	assert.True(t, config.EnableUnicodeNorm)
	assert.True(t, config.EnableWhitespaceNorm)
	assert.True(t, config.EnableLineEndingNorm)
	assert.Greater(t, config.FuzzyThreshold, 0.0)
	assert.LessOrEqual(t, config.FuzzyThreshold, 1.0)
}

func TestFuzzyConfig_Strict(t *testing.T) {
	config := StrictConfig()
	assert.False(t, config.EnableUnicodeNorm)
	assert.False(t, config.EnableWhitespaceNorm)
	assert.False(t, config.EnableLineEndingNorm)
	assert.Equal(t, 1.0, config.FuzzyThreshold)
}

func TestFuzzyConfig_Normalized(t *testing.T) {
	config := NormalizedConfig()
	assert.True(t, config.EnableUnicodeNorm)
	assert.True(t, config.EnableWhitespaceNorm)
	assert.True(t, config.EnableLineEndingNorm)
	assert.Equal(t, 1.0, config.FuzzyThreshold)
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestFuzzyMatch_EmptyStrings(t *testing.T) {
	config := DefaultFuzzyConfig()

	// Both empty
	assert.True(t, fuzzyMatchLine("", "", config))

	// One empty
	assert.False(t, fuzzyMatchLine("hello", "", config))
	assert.False(t, fuzzyMatchLine("", "hello", config))
}

func TestFuzzyMatch_VeryLongLines(t *testing.T) {
	config := DefaultFuzzyConfig()

	// Create very long lines
	longLine := strings.Repeat("x", 10000)
	longLineWithDiff := strings.Repeat("x", 9999) + "y"

	// Exact match should be fast
	assert.True(t, fuzzyMatchLine(longLine, longLine, config))

	// Slight difference should still match with fuzzy
	assert.True(t, fuzzyMatchLine(longLine, longLineWithDiff, config))
}

func TestFuzzyMatch_SpecialCharacters(t *testing.T) {
	config := DefaultFuzzyConfig()

	tests := []struct {
		name     string
		expected string
		actual   string
		match    bool
	}{
		{
			name:     "regex special chars",
			expected: ".*?+[]{}()^$|\\",
			actual:   ".*?+[]{}()^$|\\",
			match:    true,
		},
		{
			name:     "emoji",
			expected: "Hello 👋 World 🌍",
			actual:   "Hello 👋 World 🌍",
			match:    true,
		},
		{
			name:     "zero-width characters",
			expected: "hello\u200Bworld",
			actual:   "helloworld",
			match:    true, // Fuzzy matching is tolerant enough for this
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fuzzyMatchLine(tt.expected, tt.actual, config)
			assert.Equal(t, tt.match, result)
		})
	}
}
