package patch

import (
	"bytes"
	"strings"
)

// LineEndingStyle represents the type of line endings used in a file.
type LineEndingStyle int

const (
	// LineEndingLF represents Unix-style line endings (\n).
	LineEndingLF LineEndingStyle = iota

	// LineEndingCRLF represents Windows-style line endings (\r\n).
	LineEndingCRLF

	// LineEndingMixed represents files with inconsistent line endings.
	LineEndingMixed

	// LineEndingEmpty represents an empty file or file with no line breaks.
	LineEndingEmpty
)

// String returns a human-readable representation of the line ending style.
func (s LineEndingStyle) String() string {
	switch s {
	case LineEndingLF:
		return "LF"
	case LineEndingCRLF:
		return "CRLF"
	case LineEndingMixed:
		return "Mixed"
	case LineEndingEmpty:
		return "Empty"
	default:
		return "Unknown"
	}
}

// LineEndingInfo contains information about line endings in content.
type LineEndingInfo struct {
	Style     LineEndingStyle
	LFCount   int
	CRLFCount int
}

// DetectLineEnding analyzes content and determines its line ending style.
// It counts both LF and CRLF occurrences to detect the dominant style.
func DetectLineEnding(content []byte) LineEndingInfo {
	if len(content) == 0 {
		return LineEndingInfo{Style: LineEndingEmpty}
	}

	lfCount := 0
	crlfCount := 0

	// Scan through content to count line endings
	i := 0
	for i < len(content) {
		if content[i] == '\r' && i+1 < len(content) && content[i+1] == '\n' {
			// Found CRLF
			crlfCount++
			i += 2
		} else if content[i] == '\n' {
			// Found standalone LF
			lfCount++
			i++
		} else {
			i++
		}
	}

	// Determine style based on counts
	if crlfCount == 0 && lfCount == 0 {
		return LineEndingInfo{
			Style:     LineEndingEmpty,
			LFCount:   0,
			CRLFCount: 0,
		}
	}

	if crlfCount > 0 && lfCount == 0 {
		return LineEndingInfo{
			Style:     LineEndingCRLF,
			LFCount:   0,
			CRLFCount: crlfCount,
		}
	}

	if crlfCount == 0 && lfCount > 0 {
		return LineEndingInfo{
			Style:     LineEndingLF,
			LFCount:   lfCount,
			CRLFCount: 0,
		}
	}

	// Mixed line endings
	return LineEndingInfo{
		Style:     LineEndingMixed,
		LFCount:   lfCount,
		CRLFCount: crlfCount,
	}
}

// NormalizeToLF converts all line endings in content to LF (\n).
// This is used internally for patch operations to ensure consistent matching.
func NormalizeToLF(content []byte) []byte {
	// Replace CRLF with LF
	return bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
}

// NormalizeStringToLF converts all line endings in a string to LF (\n).
func NormalizeStringToLF(content string) string {
	// Replace CRLF with LF
	return strings.ReplaceAll(content, "\r\n", "\n")
}

// ConvertLineEndings converts content from LF to the specified line ending style.
// This is used after patch application to restore the original line ending format.
func ConvertLineEndings(content []byte, style LineEndingStyle) []byte {
	switch style {
	case LineEndingCRLF:
		// First normalize to LF to handle any existing CRLF
		normalized := NormalizeToLF(content)
		// Then convert LF to CRLF
		return bytes.ReplaceAll(normalized, []byte("\n"), []byte("\r\n"))
	case LineEndingLF, LineEndingEmpty, LineEndingMixed:
		// For LF, Empty, or Mixed, just ensure CRLF is converted to LF
		return NormalizeToLF(content)
	default:
		return content
	}
}

// ConvertStringLineEndings converts string content from LF to the specified line ending style.
func ConvertStringLineEndings(content string, style LineEndingStyle) string {
	switch style {
	case LineEndingCRLF:
		// First normalize to LF to handle any existing CRLF
		normalized := NormalizeStringToLF(content)
		// Then convert LF to CRLF
		return strings.ReplaceAll(normalized, "\n", "\r\n")
	case LineEndingLF, LineEndingEmpty, LineEndingMixed:
		// For LF, Empty, or Mixed, just ensure CRLF is converted to LF
		return NormalizeStringToLF(content)
	default:
		return content
	}
}

// SplitLinesNormalized splits content into lines after normalizing to LF.
// This ensures consistent behavior regardless of input line ending style.
func SplitLinesNormalized(content string) []string {
	normalized := NormalizeStringToLF(content)
	return strings.Split(normalized, "\n")
}

// PreprocessPatchContent normalizes patch content for reliable application.
// This ensures that patches can be applied regardless of Git autocrlf settings.
func PreprocessPatchContent(patchContent string) string {
	// Normalize patch content to LF for consistent parsing
	return NormalizeStringToLF(patchContent)
}
