package patch

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseUnifiedDiff_SizeLimits(t *testing.T) {
	tests := []struct {
		name    string
		diff    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "oversized patch",
			diff:    strings.Repeat("a", MaxPatchSize+1),
			wantErr: true,
			errMsg:  "exceeds maximum size",
		},
		{
			name:    "too many hunks",
			diff:    createPatchWithManyHunks(MaxHunks + 1),
			wantErr: true,
			errMsg:  "exceeds maximum",
		},
		{
			name:    "too many lines per hunk",
			diff:    createPatchWithManyLines(MaxLinesPerHunk + 1),
			wantErr: true,
			errMsg:  "exceeds maximum",
		},
		{
			name:    "valid patch at max size",
			diff:    createValidPatch(100),
			wantErr: false,
		},
		{
			name:    "valid patch with max hunks",
			diff:    createPatchWithManyHunks(MaxHunks),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseUnifiedDiff(tt.diff)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseUnifiedDiff() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing '%s', got '%v'", tt.errMsg, err)
			}
		})
	}
}

func TestValidatePatchPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid relative path",
			path:    "src/file.go",
			wantErr: false,
		},
		{
			name:    "valid nested path",
			path:    "internal/tools/patch/parser.go",
			wantErr: false,
		},
		{
			name:    "empty path allowed (dev/null)",
			path:    "",
			wantErr: false,
		},
		// Note: Path traversal is NOT validated here - it's validated at apply time
		// where AllowOutsideRoot flag is available
		{
			name:    "path traversal allowed in parser (validated at apply time)",
			path:    "../../../etc/passwd",
			wantErr: false, // Parser allows it, apply.go validates based on AllowOutsideRoot
		},
		{
			name:    "path traversal in middle allowed in parser",
			path:    "src/../../../etc/passwd",
			wantErr: false, // Parser allows it, apply.go validates based on AllowOutsideRoot
		},
		{
			name:    "absolute path unix",
			path:    "/etc/passwd",
			wantErr: true,
			errMsg:  "absolute paths not allowed",
		},
		// Note: Windows paths like C:\ are only detected as absolute on Windows
		// On Unix systems, filepath.IsAbs won't catch them, so we skip this test
		// {
		// 	name:    "absolute path windows",
		// 	path:    "C:\\Windows\\System32",
		// 	wantErr: true,
		// 	errMsg:  "absolute paths not allowed",
		// },
		{
			name:    "null byte in path",
			path:    "file\x00.txt",
			wantErr: true,
			errMsg:  "null byte",
		},
		{
			name:    "oversized path",
			path:    strings.Repeat("a", MaxFilePathLength+1),
			wantErr: true,
			errMsg:  "exceeds maximum length",
		},
		{
			name:    "path at max length",
			path:    strings.Repeat("a", MaxFilePathLength),
			wantErr: false,
		},
		{
			name:    "path with dots but not traversal",
			path:    "src/file.with.dots.go",
			wantErr: false,
		},
		{
			name:    "hidden file",
			path:    ".config/settings.json",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePatchPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePatchPath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%v'", tt.errMsg, err)
				}
			}
		})
	}
}

func TestValidateLineNumber(t *testing.T) {
	tests := []struct {
		name      string
		lineNum   int
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid line number",
			lineNum:   42,
			fieldName: "start",
			wantErr:   false,
		},
		{
			name:      "line number zero",
			lineNum:   0,
			fieldName: "start",
			wantErr:   false,
		},
		{
			name:      "negative line number",
			lineNum:   -1,
			fieldName: "start",
			wantErr:   true,
			errMsg:    "cannot be negative",
		},
		{
			name:      "large negative line number",
			lineNum:   -1000,
			fieldName: "start",
			wantErr:   true,
			errMsg:    "cannot be negative",
		},
		{
			name:      "overflow line number",
			lineNum:   20_000_000,
			fieldName: "start",
			wantErr:   true,
			errMsg:    "exceeds reasonable maximum",
		},
		{
			name:      "max reasonable line number",
			lineNum:   10_000_000,
			fieldName: "start",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLineNumber(tt.lineNum, tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateLineNumber() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%v'", tt.errMsg, err)
				}
			}
		})
	}
}

func TestValidateLineCount(t *testing.T) {
	tests := []struct {
		name      string
		count     int
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid line count",
			count:     100,
			fieldName: "line count",
			wantErr:   false,
		},
		{
			name:      "zero line count",
			count:     0,
			fieldName: "line count",
			wantErr:   false,
		},
		{
			name:      "negative line count",
			count:     -5,
			fieldName: "line count",
			wantErr:   true,
			errMsg:    "cannot be negative",
		},
		{
			name:      "exceeds max lines per hunk",
			count:     MaxLinesPerHunk + 1,
			fieldName: "line count",
			wantErr:   true,
			errMsg:    "exceeds maximum",
		},
		{
			name:      "at max lines per hunk",
			count:     MaxLinesPerHunk,
			fieldName: "line count",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLineCount(tt.count, tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateLineCount() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%v'", tt.errMsg, err)
				}
			}
		})
	}
}

func TestParseUnifiedDiff_InvalidPaths(t *testing.T) {
	tests := []struct {
		name    string
		diff    string
		wantErr bool
		errMsg  string
	}{
		// Note: Path traversal (..) is allowed in parser - validated at apply time
		{
			name: "path traversal in original (allowed by parser)",
			diff: `--- ../../../etc/passwd
+++ b/file.txt
@@ -1,1 +1,1 @@
-old
+new
`,
			wantErr: false, // Parser allows, apply.go validates based on AllowOutsideRoot
		},
		{
			name: "path traversal in new (allowed by parser)",
			diff: `--- a/file.txt
+++ ../../../etc/passwd
@@ -1,1 +1,1 @@
-old
+new
`,
			wantErr: false, // Parser allows, apply.go validates based on AllowOutsideRoot
		},
		{
			name: "absolute path in original",
			diff: `--- /etc/passwd
+++ b/file.txt
@@ -1,1 +1,1 @@
-old
+new
`,
			wantErr: true,
			errMsg:  "absolute paths not allowed",
		},
		{
			name: "null byte in path",
			diff: "--- a/file\x00.txt\n+++ b/file.txt\n@@ -1,1 +1,1 @@\n-old\n+new\n",
			wantErr: true,
			errMsg:  "null byte",
		},
		{
			name: "valid dev/null original",
			diff: `--- /dev/null
+++ b/newfile.txt
@@ -0,0 +1,1 @@
+new file
`,
			wantErr: false,
		},
		{
			name: "valid dev/null new",
			diff: `--- a/oldfile.txt
+++ /dev/null
@@ -1,1 +0,0 @@
-deleted file
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseUnifiedDiff(tt.diff)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseUnifiedDiff() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%v'", tt.errMsg, err)
				}
			}
		})
	}
}

func TestParseUnifiedDiff_InvalidLineNumbers(t *testing.T) {
	tests := []struct {
		name    string
		diff    string
		wantErr bool
		errMsg  string
	}{
		// Note: Negative line numbers won't match the regex, so they'll fail with
		// "hunk header without file header" or "patch has no hunks"
		// These tests verify the parser doesn't crash on malformed input
		{
			name: "malformed hunk header - negative original start",
			diff: `--- a/file.txt
+++ b/file.txt
@@ --1,1 +1,1 @@
-old
+new
`,
			wantErr: true,
			errMsg:  "patch", // Will fail with "patch 0 has no hunks"
		},
		{
			name: "malformed hunk header - negative new start",
			diff: `--- a/file.txt
+++ b/file.txt
@@ -1,1 +-1,1 @@
-old
+new
`,
			wantErr: true,
			errMsg:  "patch", // Will fail with "patch 0 has no hunks"
		},
		{
			name: "valid zero start for add",
			diff: `--- /dev/null
+++ b/file.txt
@@ -0,0 +1,1 @@
+new
`,
			wantErr: false,
		},
		{
			name: "valid zero start for delete",
			diff: `--- a/file.txt
+++ /dev/null
@@ -1,1 +0,0 @@
-old
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseUnifiedDiff(tt.diff)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseUnifiedDiff() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%v'", tt.errMsg, err)
				}
			}
		})
	}
}

func TestParseUnifiedDiff_TotalLineLimit(t *testing.T) {
	// Create a patch that exceeds total line limit
	var sb strings.Builder
	sb.WriteString("--- a/file.txt\n")
	sb.WriteString("+++ b/file.txt\n")

	// Create multiple hunks that exceed MaxTotalLines when combined
	numHunks := 100
	linesPerHunk := (MaxTotalLines / numHunks) + 100 // Exceed the limit

	for i := 0; i < numHunks; i++ {
		sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n",
			i*linesPerHunk+1, linesPerHunk,
			i*linesPerHunk+1, linesPerHunk))
		for j := 0; j < linesPerHunk; j++ {
			sb.WriteString(" context line\n")
		}
	}

	_, err := parseUnifiedDiff(sb.String())
	if err == nil {
		t.Error("expected error for exceeding total line limit")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("expected error about exceeding maximum, got: %v", err)
	}
}

func TestParseUnifiedDiff_HunkValidation(t *testing.T) {
	// Note: This tests that validation runs successfully on well-formed patches
	// The actual line count validation is tested in the existing patch_test.go
	tests := []struct {
		name    string
		diff    string
		wantErr bool
	}{
		{
			name: "valid hunk - simple change",
			diff: `--- a/file.txt
+++ b/file.txt
@@ -1,1 +1,1 @@
-old
+new
`,
			wantErr: false,
		},
		{
			name: "valid hunk with context",
			diff: `--- a/file.txt
+++ b/file.txt
@@ -1,2 +1,2 @@
 context before
-old
+new
`,
			wantErr: false,
		},
		{
			name: "valid hunk - addition only",
			diff: `--- /dev/null
+++ b/file.txt
@@ -0,0 +1,1 @@
+new
`,
			wantErr: false,
		},
		{
			name: "valid hunk - deletion only",
			diff: `--- a/file.txt
+++ /dev/null
@@ -1,1 +0,0 @@
-old
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patches, err := parseUnifiedDiff(tt.diff)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("unexpected parse error: %v", err)
				}
				return
			}

			// Validate the patch
			for _, patch := range patches {
				err = validatePatch(&patch)
				if err != nil {
					break
				}
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Helper functions

func createPatchWithManyHunks(count int) string {
	var sb strings.Builder
	sb.WriteString("--- a/file.txt\n")
	sb.WriteString("+++ b/file.txt\n")
	for i := 0; i < count; i++ {
		sb.WriteString(fmt.Sprintf("@@ -%d,1 +%d,1 @@\n", i*10+1, i*10+1))
		sb.WriteString("-old line\n")
		sb.WriteString("+new line\n")
	}
	return sb.String()
}

func createPatchWithManyLines(lineCount int) string {
	var sb strings.Builder
	sb.WriteString("--- a/file.txt\n")
	sb.WriteString("+++ b/file.txt\n")
	sb.WriteString(fmt.Sprintf("@@ -1,%d +1,%d @@\n", lineCount, lineCount))
	for i := 0; i < lineCount; i++ {
		sb.WriteString(fmt.Sprintf(" context line %d\n", i))
	}
	return sb.String()
}

func createValidPatch(lines int) string {
	var sb strings.Builder
	sb.WriteString("--- a/file.txt\n")
	sb.WriteString("+++ b/file.txt\n")
	sb.WriteString(fmt.Sprintf("@@ -1,%d +1,%d @@\n", lines, lines))
	for i := 0; i < lines; i++ {
		sb.WriteString(fmt.Sprintf(" line %d\n", i))
	}
	return sb.String()
}

func TestResourceLimitConstants(t *testing.T) {
	// Verify constants are set to reasonable values
	if MaxPatchSize <= 0 {
		t.Error("MaxPatchSize must be positive")
	}
	if MaxHunks <= 0 {
		t.Error("MaxHunks must be positive")
	}
	if MaxLinesPerHunk <= 0 {
		t.Error("MaxLinesPerHunk must be positive")
	}
	if MaxTotalLines <= 0 {
		t.Error("MaxTotalLines must be positive")
	}
	if MaxFilePathLength <= 0 {
		t.Error("MaxFilePathLength must be positive")
	}

	// Verify reasonable relationships
	if MaxLinesPerHunk > MaxTotalLines {
		t.Error("MaxLinesPerHunk should not exceed MaxTotalLines")
	}

	t.Logf("Resource limits: PatchSize=%d, Hunks=%d, LinesPerHunk=%d, TotalLines=%d, PathLen=%d",
		MaxPatchSize, MaxHunks, MaxLinesPerHunk, MaxTotalLines, MaxFilePathLength)
}

func TestValidatePatchPath_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"single char", "a", false},
		{"single dot", ".", false},
		{"double dot alone", "..", false},     // Allowed by parser, validated at apply time
		{"slash double dot slash", "/../", true}, // Leading slash makes it absolute
		{"dot slash", "./file.txt", false},
		{"multiple slashes", "a//b//c", false}, // filepath.Clean handles this
		{"trailing slash", "dir/", false},
		{"space in name", "my file.txt", false},
		{"unicode", "файл.txt", false},
		{"emoji", "file😀.txt", false},
		{"very nested", "a/b/c/d/e/f/g/h/i/j/file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePatchPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePatchPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestParseUnifiedDiff_EmptyAndWhitespace(t *testing.T) {
	tests := []struct {
		name    string
		diff    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty string",
			diff:    "",
			wantErr: true,
			errMsg:  "empty diff",
		},
		{
			name:    "only whitespace",
			diff:    "   \n\n  \t  \n",
			wantErr: true,
			errMsg:  "empty diff",
		},
		{
			name:    "only newlines",
			diff:    "\n\n\n\n",
			wantErr: true,
			errMsg:  "empty diff",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseUnifiedDiff(tt.diff)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseUnifiedDiff() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing '%s', got '%v'", tt.errMsg, err)
			}
		})
	}
}
