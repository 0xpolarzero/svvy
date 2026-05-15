package shell

import (
	"encoding/base64"
	"unicode/utf8"
)

// IsBinaryData detects if a byte slice contains binary (non-UTF-8) data.
// It checks for:
// 1. Null bytes (common in binary data)
// 2. Invalid UTF-8 sequences
// 3. High ratio of non-printable characters
func IsBinaryData(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// Quick check: null bytes are a strong indicator of binary data
	for _, b := range data {
		if b == 0 {
			return true
		}
	}

	// Check if the data is valid UTF-8
	if !utf8.Valid(data) {
		return true
	}

	// Additional heuristic: check for high ratio of control characters
	// (excluding common ones like tab, newline, carriage return)
	controlCount := 0
	for _, b := range data {
		if b < 32 && b != '\t' && b != '\n' && b != '\r' {
			controlCount++
		}
	}

	// If more than 30% are control characters, consider it binary
	if len(data) > 0 && float64(controlCount)/float64(len(data)) > 0.3 {
		return true
	}

	return false
}

// EncodeChunk encodes a byte slice for transmission.
// If the data is binary, it returns base64-encoded string and true.
// If the data is valid UTF-8 text, it returns the raw string and false.
func EncodeChunk(data []byte) (encoded string, isBinary bool) {
	if IsBinaryData(data) {
		return base64.StdEncoding.EncodeToString(data), true
	}
	return string(data), false
}

// DecodeChunk decodes a chunk based on whether it's binary.
// If isBinary is true, it base64-decodes the string.
// Otherwise, it returns the string as bytes.
func DecodeChunk(chunk string, isBinary bool) ([]byte, error) {
	if isBinary {
		return base64.StdEncoding.DecodeString(chunk)
	}
	return []byte(chunk), nil
}
