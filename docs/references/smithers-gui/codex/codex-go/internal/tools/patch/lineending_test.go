package patch

import (
	"testing"
)

func TestDetectLineEnding(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected LineEndingStyle
		lfCount  int
		crlfCount int
	}{
		{
			name:     "empty content",
			content:  []byte{},
			expected: LineEndingEmpty,
			lfCount:  0,
			crlfCount: 0,
		},
		{
			name:     "LF only",
			content:  []byte("line1\nline2\nline3\n"),
			expected: LineEndingLF,
			lfCount:  3,
			crlfCount: 0,
		},
		{
			name:     "CRLF only",
			content:  []byte("line1\r\nline2\r\nline3\r\n"),
			expected: LineEndingCRLF,
			lfCount:  0,
			crlfCount: 3,
		},
		{
			name:     "mixed CRLF and LF",
			content:  []byte("line1\r\nline2\nline3\r\n"),
			expected: LineEndingMixed,
			lfCount:  1,
			crlfCount: 2,
		},
		{
			name:     "no line endings",
			content:  []byte("single line with no newline"),
			expected: LineEndingEmpty,
			lfCount:  0,
			crlfCount: 0,
		},
		{
			name:     "trailing LF",
			content:  []byte("line1\nline2\n"),
			expected: LineEndingLF,
			lfCount:  2,
			crlfCount: 0,
		},
		{
			name:     "trailing CRLF",
			content:  []byte("line1\r\nline2\r\n"),
			expected: LineEndingCRLF,
			lfCount:  0,
			crlfCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := DetectLineEnding(tt.content)
			if info.Style != tt.expected {
				t.Errorf("DetectLineEnding() style = %v, want %v", info.Style, tt.expected)
			}
			if info.LFCount != tt.lfCount {
				t.Errorf("DetectLineEnding() LFCount = %d, want %d", info.LFCount, tt.lfCount)
			}
			if info.CRLFCount != tt.crlfCount {
				t.Errorf("DetectLineEnding() CRLFCount = %d, want %d", info.CRLFCount, tt.crlfCount)
			}
		})
	}
}

func TestNormalizeToLF(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected []byte
	}{
		{
			name:     "CRLF to LF",
			content:  []byte("line1\r\nline2\r\nline3\r\n"),
			expected: []byte("line1\nline2\nline3\n"),
		},
		{
			name:     "already LF",
			content:  []byte("line1\nline2\nline3\n"),
			expected: []byte("line1\nline2\nline3\n"),
		},
		{
			name:     "mixed endings",
			content:  []byte("line1\r\nline2\nline3\r\n"),
			expected: []byte("line1\nline2\nline3\n"),
		},
		{
			name:     "empty content",
			content:  []byte{},
			expected: []byte{},
		},
		{
			name:     "no line endings",
			content:  []byte("single line"),
			expected: []byte("single line"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeToLF(tt.content)
			if string(result) != string(tt.expected) {
				t.Errorf("NormalizeToLF() = %q, want %q", string(result), string(tt.expected))
			}
		})
	}
}

func TestNormalizeStringToLF(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "CRLF to LF",
			content:  "line1\r\nline2\r\nline3\r\n",
			expected: "line1\nline2\nline3\n",
		},
		{
			name:     "already LF",
			content:  "line1\nline2\nline3\n",
			expected: "line1\nline2\nline3\n",
		},
		{
			name:     "mixed endings",
			content:  "line1\r\nline2\nline3\r\n",
			expected: "line1\nline2\nline3\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeStringToLF(tt.content)
			if result != tt.expected {
				t.Errorf("NormalizeStringToLF() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConvertLineEndings(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		style    LineEndingStyle
		expected []byte
	}{
		{
			name:     "LF to CRLF",
			content:  []byte("line1\nline2\nline3\n"),
			style:    LineEndingCRLF,
			expected: []byte("line1\r\nline2\r\nline3\r\n"),
		},
		{
			name:     "CRLF to LF",
			content:  []byte("line1\r\nline2\r\nline3\r\n"),
			style:    LineEndingLF,
			expected: []byte("line1\nline2\nline3\n"),
		},
		{
			name:     "CRLF to CRLF (idempotent)",
			content:  []byte("line1\r\nline2\r\nline3\r\n"),
			style:    LineEndingCRLF,
			expected: []byte("line1\r\nline2\r\nline3\r\n"),
		},
		{
			name:     "LF to LF (idempotent)",
			content:  []byte("line1\nline2\nline3\n"),
			style:    LineEndingLF,
			expected: []byte("line1\nline2\nline3\n"),
		},
		{
			name:     "mixed to CRLF",
			content:  []byte("line1\r\nline2\nline3\r\n"),
			style:    LineEndingCRLF,
			expected: []byte("line1\r\nline2\r\nline3\r\n"),
		},
		{
			name:     "mixed to LF",
			content:  []byte("line1\r\nline2\nline3\r\n"),
			style:    LineEndingLF,
			expected: []byte("line1\nline2\nline3\n"),
		},
		{
			name:     "empty style preserves",
			content:  []byte("line1\nline2\n"),
			style:    LineEndingEmpty,
			expected: []byte("line1\nline2\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertLineEndings(tt.content, tt.style)
			if string(result) != string(tt.expected) {
				t.Errorf("ConvertLineEndings() = %q, want %q", string(result), string(tt.expected))
			}
		})
	}
}

func TestConvertStringLineEndings(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		style    LineEndingStyle
		expected string
	}{
		{
			name:     "LF to CRLF",
			content:  "line1\nline2\nline3\n",
			style:    LineEndingCRLF,
			expected: "line1\r\nline2\r\nline3\r\n",
		},
		{
			name:     "CRLF to LF",
			content:  "line1\r\nline2\r\nline3\r\n",
			style:    LineEndingLF,
			expected: "line1\nline2\nline3\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertStringLineEndings(tt.content, tt.style)
			if result != tt.expected {
				t.Errorf("ConvertStringLineEndings() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSplitLinesNormalized(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "LF endings",
			content:  "line1\nline2\nline3",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "CRLF endings",
			content:  "line1\r\nline2\r\nline3",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "mixed endings",
			content:  "line1\r\nline2\nline3",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "trailing newline LF",
			content:  "line1\nline2\n",
			expected: []string{"line1", "line2", ""},
		},
		{
			name:     "trailing newline CRLF",
			content:  "line1\r\nline2\r\n",
			expected: []string{"line1", "line2", ""},
		},
		{
			name:     "empty string",
			content:  "",
			expected: []string{""},
		},
		{
			name:     "single line no newline",
			content:  "line1",
			expected: []string{"line1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SplitLinesNormalized(tt.content)
			if len(result) != len(tt.expected) {
				t.Errorf("SplitLinesNormalized() length = %d, want %d", len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("SplitLinesNormalized()[%d] = %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestPreprocessPatchContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "patch with CRLF",
			content: "--- a/file.txt\r\n+++ b/file.txt\r\n@@ -1,3 +1,3 @@\r\n line1\r\n-line2\r\n+line2 modified\r\n line3\r\n",
			expected: "--- a/file.txt\n+++ b/file.txt\n@@ -1,3 +1,3 @@\n line1\n-line2\n+line2 modified\n line3\n",
		},
		{
			name: "patch with LF",
			content: "--- a/file.txt\n+++ b/file.txt\n@@ -1,3 +1,3 @@\n line1\n-line2\n+line2 modified\n line3\n",
			expected: "--- a/file.txt\n+++ b/file.txt\n@@ -1,3 +1,3 @@\n line1\n-line2\n+line2 modified\n line3\n",
		},
		{
			name: "patch with mixed endings",
			content: "--- a/file.txt\r\n+++ b/file.txt\n@@ -1,3 +1,3 @@\r\n line1\n-line2\r\n+line2 modified\n line3\n",
			expected: "--- a/file.txt\n+++ b/file.txt\n@@ -1,3 +1,3 @@\n line1\n-line2\n+line2 modified\n line3\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PreprocessPatchContent(tt.content)
			if result != tt.expected {
				t.Errorf("PreprocessPatchContent() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestLineEndingStyleString(t *testing.T) {
	tests := []struct {
		style    LineEndingStyle
		expected string
	}{
		{LineEndingLF, "LF"},
		{LineEndingCRLF, "CRLF"},
		{LineEndingMixed, "Mixed"},
		{LineEndingEmpty, "Empty"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.style.String()
			if result != tt.expected {
				t.Errorf("LineEndingStyle.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestRoundTripConversion tests that we can detect, normalize, and convert back correctly
func TestRoundTripConversion(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
	}{
		{
			name:    "CRLF file",
			content: []byte("line1\r\nline2\r\nline3\r\n"),
		},
		{
			name:    "LF file",
			content: []byte("line1\nline2\nline3\n"),
		},
		{
			name:    "complex content with CRLF",
			content: []byte("function main() {\r\n    console.log('hello');\r\n    return 0;\r\n}\r\n"),
		},
		{
			name:    "complex content with LF",
			content: []byte("function main() {\n    console.log('hello');\n    return 0;\n}\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Detect original line ending
			info := DetectLineEnding(tt.content)

			// Normalize to LF
			normalized := NormalizeToLF(tt.content)

			// Convert back to original style
			restored := ConvertLineEndings(normalized, info.Style)

			// Should match original
			if string(restored) != string(tt.content) {
				t.Errorf("Round trip failed: got %q, want %q", string(restored), string(tt.content))
			}
		})
	}
}
