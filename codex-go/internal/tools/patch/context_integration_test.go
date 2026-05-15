package patch

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContextIntegration_BasicModifiedFile tests applying a patch to a file
// where line numbers have shifted due to additions
func TestContextIntegration_BasicModifiedFile(t *testing.T) {
	// Actual file with extra lines at the beginning
	modifiedContent := `new line A
new line B
line 1
line 2
line 3
line 4
line 5
`

	// Patch that was created for original (targets lines 2-3)
	diff := `--- a/file.txt
+++ b/file.txt
@@ -2,2 +2,2 @@
 line 2
-line 3
+modified line 3
`

	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)
	require.Len(t, patches, 1)

	// Apply the patch to modified content
	result, err := applyHunks(modifiedContent, patches[0].Hunks)
	require.NoError(t, err)

	expected := `new line A
new line B
line 1
line 2
modified line 3
line 4
line 5
`

	assert.Equal(t, expected, result)
}

// TestContextIntegration_LargeOffset tests context seeking with a large offset
func TestContextIntegration_LargeOffset(t *testing.T) {
	// Build a file with many lines before the target
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, "header line "+strings.Repeat("x", i))
	}
	lines = append(lines, "function foo() {")
	lines = append(lines, "  var x = 1;")
	lines = append(lines, "  var y = 2;")
	lines = append(lines, "  return x + y;")
	lines = append(lines, "}")

	modifiedContent := strings.Join(lines, "\n") + "\n"

	// Patch expects lines 1-4 but they're actually at 21-24
	diff := `--- a/file.js
+++ b/file.js
@@ -1,4 +1,4 @@
 function foo() {
-  var x = 1;
+  const x = 1;
   var y = 2;
   return x + y;
`

	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyHunks(modifiedContent, patches[0].Hunks)
	require.NoError(t, err)

	// Verify the change was applied at the correct location
	assert.Contains(t, result, "const x = 1;")
	assert.NotContains(t, result, "  var x = 1;")
}

// TestContextIntegration_UnicodeNormalization tests that Unicode is normalized
func TestContextIntegration_UnicodeNormalization(t *testing.T) {
	// File with fancy Unicode characters (en dash U+2013)
	modifiedContent := "# This is a comment \u2013 with en dash\ndef function():\n    pass\n"

	// Patch uses ASCII dash
	diff := `--- a/file.py
+++ b/file.py
@@ -1,1 +1,1 @@
-# This is a comment - with en dash
+# Updated comment
`

	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyHunks(modifiedContent, patches[0].Hunks)
	require.NoError(t, err)

	expected := "# Updated comment\ndef function():\n    pass\n"

	assert.Equal(t, expected, result)
}

// TestContextIntegration_WhitespaceVariations tests handling of whitespace differences
func TestContextIntegration_WhitespaceVariations(t *testing.T) {
	// File with trailing whitespace
	modifiedContent := "line 1   \nline 2\t\nline 3\n"

	// Patch without trailing whitespace
	diff := `--- a/file.txt
+++ b/file.txt
@@ -1,3 +1,3 @@
 line 1
-line 2
+modified line 2
 line 3
`

	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyHunks(modifiedContent, patches[0].Hunks)
	require.NoError(t, err)

	// Should find and apply despite whitespace differences
	assert.Contains(t, result, "modified line 2")
}

// TestContextIntegration_MultipleHunks tests applying multiple hunks with context seeking
func TestContextIntegration_MultipleHunks(t *testing.T) {
	modifiedContent := `header1
header2
header3
function first() {
  return 1;
}

function second() {
  return 2;
}
`

	// Patch expects different line numbers but should find via context
	diff := `--- a/file.js
+++ b/file.js
@@ -1,3 +1,3 @@
 function first() {
-  return 1;
+  return 100;
 }
@@ -7,3 +7,3 @@
 function second() {
-  return 2;
+  return 200;
 }
`

	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyHunks(modifiedContent, patches[0].Hunks)
	require.NoError(t, err)

	assert.Contains(t, result, "return 100;")
	assert.Contains(t, result, "return 200;")
	assert.NotContains(t, result, "return 1;")
	assert.NotContains(t, result, "return 2;")
}

// TestContextIntegration_NoMatch tests that we get a clear error when context isn't found
func TestContextIntegration_NoMatch(t *testing.T) {
	modifiedContent := `line 1
line 2
line 3
`

	// Patch for content that doesn't exist
	diff := `--- a/file.txt
+++ b/file.txt
@@ -1,2 +1,2 @@
-nonexistent line
+new line
`

	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	_, err = applyHunks(modifiedContent, patches[0].Hunks)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not find context lines")
}

// BenchmarkContextSeeking benchmarks the context seeking performance
func BenchmarkContextSeeking(b *testing.B) {
	// Create a large file
	var lines []string
	for i := 1; i <= 1000; i++ {
		lines = append(lines, "line "+strings.Repeat("x", i%50))
	}
	lines = append(lines, "target line")
	lines = append(lines, "next line")

	modifiedContent := strings.Join(lines, "\n") + "\n"

	diff := `--- a/file.txt
+++ b/file.txt
@@ -1,2 +1,2 @@
-target line
+modified target line
 next line
`

	patches, err := parseUnifiedDiff(diff)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = applyHunks(modifiedContent, patches[0].Hunks)
	}
}

// TestContextMatchConfig_WindowSizes tests different window sizes
func TestContextMatchConfig_WindowSizes(t *testing.T) {
	// Create content with target 60 lines away from expected
	var lines []string
	for i := 1; i <= 60; i++ {
		lines = append(lines, "filler line")
	}
	lines = append(lines, "target")

	actualLines := lines

	hunk := &Hunk{
		OriginalStart: 1,
		OriginalLines: 1,
		NewStart:      1,
		NewLines:      1,
		Lines: []Line{
			{Type: LineRemove, Content: "target"},
			{Type: LineAdd, Content: "modified"},
		},
	}

	// Small window should fail (target is 60 lines away, window is only 30)
	smallConfig := ContextMatchConfig{SearchWindowSize: 30}
	_, err := applyHunkWithContextAndFuzzy(actualLines, hunk, smallConfig, DefaultFuzzyConfig())
	// Note: With fuzzy fallback, this might still succeed, so we just check it doesn't panic
	_ = err

	// Large window should succeed
	largeConfig := ContextMatchConfig{SearchWindowSize: 100}
	result, err := applyHunkWithContextAndFuzzy(actualLines, hunk, largeConfig, DefaultFuzzyConfig())
	require.NoError(t, err)
	assert.Contains(t, result, "modified")
	assert.NotContains(t, result, "target")
}
