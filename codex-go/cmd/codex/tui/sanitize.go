package tui

import (
	"regexp"
	"strings"
)

// ANSI escape sequence patterns that could be used for terminal injection attacks
var (
	// CSI (Control Sequence Introducer) sequences: ESC [ ... (most common)
	// Matches: \x1b[<params><letter>
	// Examples: cursor movement, screen manipulation, mode changes
	csiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

	// OSC (Operating System Command) sequences: ESC ] ... BEL or ESC ] ... ST
	// Matches: \x1b]...\x07 or \x1b]...\x1b\\
	// Examples: window title changes, clipboard operations
	oscPattern = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)

	// DCS (Device Control String) sequences: ESC P ... ST
	// Matches: \x1bP...\x1b\\
	// Examples: terminal-specific control codes
	dcsPattern = regexp.MustCompile(`\x1bP[^\x1b]*\x1b\\`)

	// APC (Application Program Command) sequences: ESC _ ... ST
	// Matches: \x1b_...\x1b\\
	apcPattern = regexp.MustCompile(`\x1b_[^\x1b]*\x1b\\`)

	// PM (Privacy Message) sequences: ESC ^ ... ST
	// Matches: \x1b^...\x1b\\
	pmPattern = regexp.MustCompile(`\x1b\^[^\x1b]*\x1b\\`)

	// Simple escape sequences: ESC <letter>
	// Matches: \x1b[a-zA-Z]
	// Examples: charset selection, keypad mode
	simpleEscPattern = regexp.MustCompile(`\x1b[a-zA-Z]`)

	// C0 and C1 control characters (except common whitespace)
	// Matches control chars but preserves \t, \n, \r
	// These include dangerous chars like BEL (\x07), BS (\x08), etc.
	controlCharsPattern = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F-\x9F]`)
)

// SanitizeContent removes dangerous ANSI escape sequences from untrusted content
// while preserving safe formatting. This prevents terminal injection attacks.
//
// Security considerations:
// - Removes all CSI sequences (cursor movement, screen clearing, etc.)
// - Removes OSC sequences (window title changes, clipboard access, etc.)
// - Removes DCS, APC, PM sequences (device-specific commands)
// - Removes dangerous control characters
// - Preserves basic whitespace (tabs, newlines, carriage returns)
//
// Attack examples blocked:
// - \x1b]0;evil URL\x07 - Changes terminal title to phishing URL
// - \x1b[H\x1b[2J - Clears screen and moves cursor (hide attacks)
// - \x1b[?25l - Hides cursor (confuse user)
// - \x1b[1000;1000H - Moves cursor off-screen
// - \x1b[6n - Device status report (terminal fingerprinting)
// - \x1b]52;c;base64data\x07 - Clipboard manipulation
func SanitizeContent(content string) string {
	// Apply all sanitization patterns in order of specificity
	// Start with more specific patterns (longer sequences) first
	sanitized := content

	// 1. Remove OSC sequences (window title, clipboard, etc.)
	sanitized = oscPattern.ReplaceAllString(sanitized, "")

	// 2. Remove DCS sequences (device control)
	sanitized = dcsPattern.ReplaceAllString(sanitized, "")

	// 3. Remove APC sequences (application commands)
	sanitized = apcPattern.ReplaceAllString(sanitized, "")

	// 4. Remove PM sequences (privacy messages)
	sanitized = pmPattern.ReplaceAllString(sanitized, "")

	// 5. Remove CSI sequences (cursor movement, colors, etc.)
	// Note: This also removes color codes - if we want to preserve safe colors,
	// we'd need a more sophisticated allowlist approach
	sanitized = csiPattern.ReplaceAllString(sanitized, "")

	// 6. Remove simple escape sequences
	sanitized = simpleEscPattern.ReplaceAllString(sanitized, "")

	// 7. Remove dangerous control characters (but preserve whitespace)
	sanitized = controlCharsPattern.ReplaceAllString(sanitized, "")

	return sanitized
}

// SanitizeWithPlaceholder is like SanitizeContent but replaces removed sequences
// with a visible placeholder for debugging/visibility purposes.
func SanitizeWithPlaceholder(content string, placeholder string) string {
	sanitized := content

	// Replace each type of sequence with the placeholder
	sanitized = oscPattern.ReplaceAllString(sanitized, placeholder)
	sanitized = dcsPattern.ReplaceAllString(sanitized, placeholder)
	sanitized = apcPattern.ReplaceAllString(sanitized, placeholder)
	sanitized = pmPattern.ReplaceAllString(sanitized, placeholder)
	sanitized = csiPattern.ReplaceAllString(sanitized, placeholder)
	sanitized = simpleEscPattern.ReplaceAllString(sanitized, placeholder)
	sanitized = controlCharsPattern.ReplaceAllString(sanitized, placeholder)

	return sanitized
}

// StripAllANSI completely removes all ANSI sequences, even safe ones.
// This is the most aggressive sanitization for maximum security.
func StripAllANSI(content string) string {
	return SanitizeContent(content)
}

// ContainsANSI checks if the content contains any ANSI escape sequences.
// Useful for logging/alerting when malicious content is detected.
func ContainsANSI(content string) bool {
	return oscPattern.MatchString(content) ||
		dcsPattern.MatchString(content) ||
		apcPattern.MatchString(content) ||
		pmPattern.MatchString(content) ||
		csiPattern.MatchString(content) ||
		simpleEscPattern.MatchString(content) ||
		controlCharsPattern.MatchString(content)
}

// SanitizeStringSlice sanitizes all strings in a slice
func SanitizeStringSlice(items []string) []string {
	result := make([]string, len(items))
	for i, item := range items {
		result[i] = SanitizeContent(item)
	}
	return result
}

// SanitizeLines sanitizes content line by line, preserving line structure
func SanitizeLines(content string) string {
	lines := strings.Split(content, "\n")
	sanitizedLines := make([]string, len(lines))
	for i, line := range lines {
		sanitizedLines[i] = SanitizeContent(line)
	}
	return strings.Join(sanitizedLines, "\n")
}
