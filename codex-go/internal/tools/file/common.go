package file

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// validatePath ensures a path is within the workspace and returns the absolute path.
// This prevents path traversal attacks by checking that the resolved path is within
// the base directory.
func validatePath(base, path string) (string, error) {
	// Clean the base path
	base = filepath.Clean(base)
	if !filepath.IsAbs(base) {
		return "", fmt.Errorf("base path must be absolute: %s", base)
	}

	// If path is relative, join with base
	var fullPath string
	if filepath.IsAbs(path) {
		fullPath = filepath.Clean(path)
	} else {
		fullPath = filepath.Clean(filepath.Join(base, path))
	}

	// Check if the full path is within the base directory
	// Use Rel to check if we can construct a relative path from base to fullPath
	// that doesn't escape the base directory
	rel, err := filepath.Rel(base, fullPath)
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	// If the relative path starts with "..", it's trying to escape
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("path is outside workspace: %s", path)
	}

	return fullPath, nil
}

// isBinaryFile checks if a file appears to be binary by looking for null bytes
// and non-UTF8 sequences in the first chunk of data.
func isBinaryFile(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// Check for null bytes (strong indicator of binary)
	if bytes.Contains(data, []byte{0}) {
		return true
	}

	// Check if the data is valid UTF-8
	if !utf8.Valid(data) {
		return true
	}

	// Count control characters (excluding common whitespace)
	controlCount := 0
	sampleSize := len(data)
	if sampleSize > 8192 {
		sampleSize = 8192 // Check first 8KB
	}

	for i := 0; i < sampleSize; i++ {
		b := data[i]
		// Control characters except tab, newline, carriage return
		if b < 32 && b != 9 && b != 10 && b != 13 {
			controlCount++
		}
	}

	// If more than 30% control characters, likely binary
	threshold := sampleSize * 30 / 100
	return controlCount > threshold
}

// formatFileSize formats a file size in bytes to a human-readable string.
func formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.1f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.1f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// isHiddenFile checks if a filename is hidden (starts with .)
func isHiddenFile(name string) bool {
	return strings.HasPrefix(name, ".")
}

// matchPattern checks if a name matches a glob pattern.
// Returns true if pattern is empty or if name matches the pattern.
func matchPattern(pattern, name string) (bool, error) {
	if pattern == "" {
		return true, nil
	}
	return filepath.Match(pattern, name)
}

// (unused helper removed)
