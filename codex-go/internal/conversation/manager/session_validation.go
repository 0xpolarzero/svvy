package manager

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// SessionIDValidationError represents a session ID validation error.
type SessionIDValidationError struct {
	SessionID string
	Reason    string
}

func (e *SessionIDValidationError) Error() string {
	return fmt.Sprintf("invalid session ID %q: %s", e.SessionID, e.Reason)
}

const (
	// MaxSessionIDLength is the maximum allowed length for a session ID.
	MaxSessionIDLength = 128
	// MinSessionIDLength is the minimum allowed length for a session ID.
	MinSessionIDLength = 1
)

// validSessionIDPattern defines the allowlist of safe characters for session IDs.
// Only alphanumeric, hyphens, and underscores are allowed.
var validSessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ValidateSessionID validates a session ID for security and correctness.
// It checks for path traversal attempts, invalid characters, and length constraints.
//
// Security checks include:
// - Path traversal patterns (.., /, \)
// - URL-encoded path traversal attempts
// - Unicode normalization attacks
// - Control characters and null bytes
// - Length constraints
// - Character allowlist (alphanumeric, hyphen, underscore only)
func ValidateSessionID(sessionID string) error {
	// Check for empty or too short ID
	if len(sessionID) < MinSessionIDLength {
		return &SessionIDValidationError{
			SessionID: sessionID,
			Reason:    fmt.Sprintf("length must be at least %d characters", MinSessionIDLength),
		}
	}

	// Check for too long ID (DoS prevention)
	if len(sessionID) > MaxSessionIDLength {
		return &SessionIDValidationError{
			SessionID: sessionID,
			Reason:    fmt.Sprintf("length must not exceed %d characters", MaxSessionIDLength),
		}
	}

	// Check for null bytes
	if strings.Contains(sessionID, "\x00") {
		return &SessionIDValidationError{
			SessionID: sessionID,
			Reason:    "contains null bytes",
		}
	}

	// Check for control characters
	for _, r := range sessionID {
		if unicode.IsControl(r) {
			return &SessionIDValidationError{
				SessionID: sessionID,
				Reason:    "contains control characters",
			}
		}
	}

	// Check for path traversal patterns - explicit dots and slashes
	if strings.Contains(sessionID, "..") {
		return &SessionIDValidationError{
			SessionID: sessionID,
			Reason:    "contains path traversal pattern (..)",
		}
	}

	if strings.ContainsAny(sessionID, "/\\") {
		return &SessionIDValidationError{
			SessionID: sessionID,
			Reason:    "contains path separators",
		}
	}

	// Check for URL-encoded path traversal attempts
	lowerID := strings.ToLower(sessionID)
	encodedPatterns := []string{
		"%2e",  // .
		"%2f",  // /
		"%5c",  // \
		"%00",  // null byte
	}
	for _, pattern := range encodedPatterns {
		if strings.Contains(lowerID, pattern) {
			return &SessionIDValidationError{
				SessionID: sessionID,
				Reason:    "contains URL-encoded path traversal or control characters",
			}
		}
	}

	// Check for unicode normalization attacks (e.g., fullwidth characters)
	// U+FF0E (fullwidth full stop) could normalize to '.'
	// U+FF0F (fullwidth solidus) could normalize to '/'
	for _, r := range sessionID {
		if r >= 0xFF00 && r <= 0xFFEF {
			return &SessionIDValidationError{
				SessionID: sessionID,
				Reason:    "contains fullwidth unicode characters that may normalize to path separators",
			}
		}
	}

	// Apply strict character allowlist - only alphanumeric, hyphen, and underscore
	if !validSessionIDPattern.MatchString(sessionID) {
		return &SessionIDValidationError{
			SessionID: sessionID,
			Reason:    "contains invalid characters (only alphanumeric, hyphen, and underscore allowed)",
		}
	}

	// Additional check: ensure it doesn't look like a special path
	if sessionID == "." || sessionID == ".." {
		return &SessionIDValidationError{
			SessionID: sessionID,
			Reason:    "is a reserved path component",
		}
	}

	return nil
}

// ValidateAndResolvSessionPath validates a session ID and constructs a safe path
// within the sessions root directory. It returns an error if the session ID is invalid
// or if the resolved path would escape the sessions root.
//
// This function provides defense-in-depth by validating both the session ID and
// the resulting path after construction.
func ValidateAndResolveSessionPath(sessionID, sessionsRoot string) (string, error) {
	// First, validate the session ID itself
	if err := ValidateSessionID(sessionID); err != nil {
		return "", err
	}

	// Validate sessions root is absolute
	if !filepath.IsAbs(sessionsRoot) {
		return "", fmt.Errorf("sessions root must be an absolute path: %s", sessionsRoot)
	}

	// Clean the sessions root
	cleanRoot := filepath.Clean(sessionsRoot)

	// Construct the session directory path
	sessionDir := filepath.Join(cleanRoot, sessionID)

	// Clean the constructed path
	cleanSessionDir := filepath.Clean(sessionDir)

	// Defense in depth: verify the path is still within sessions root
	// Use filepath.Rel to detect any escape attempt
	relPath, err := filepath.Rel(cleanRoot, cleanSessionDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve session path: %w", err)
	}

	// If the relative path starts with "..", it escaped the root
	if strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || relPath == ".." {
		return "", &SessionIDValidationError{
			SessionID: sessionID,
			Reason:    "resolved path escapes sessions root",
		}
	}

	// Verify the session directory is exactly one level deep
	// This ensures sessionID didn't contain hidden path separators
	if relPath != sessionID {
		return "", &SessionIDValidationError{
			SessionID: sessionID,
			Reason:    fmt.Sprintf("resolved path does not match expected structure (got %q)", relPath),
		}
	}

	return cleanSessionDir, nil
}

// sanitizeSessionIDForLog returns a safe version of the session ID for logging.
// It truncates long IDs and escapes control characters to prevent log injection.
func sanitizeSessionIDForLog(sessionID string) string {
	// Truncate if too long
	const maxLogLength = 64
	if len(sessionID) > maxLogLength {
		sessionID = sessionID[:maxLogLength] + "..."
	}

	// Escape control characters
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '?'
		}
		return r
	}, sessionID)
}
