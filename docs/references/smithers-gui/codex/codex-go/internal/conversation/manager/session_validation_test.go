package manager

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateSessionID tests session ID validation against various attack vectors.
func TestValidateSessionID(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		wantErr   bool
		errReason string
	}{
		// Valid session IDs
		{
			name:      "valid alphanumeric",
			sessionID: "session123",
			wantErr:   false,
		},
		{
			name:      "valid with hyphens",
			sessionID: "session-123-abc",
			wantErr:   false,
		},
		{
			name:      "valid with underscores",
			sessionID: "session_123_abc",
			wantErr:   false,
		},
		{
			name:      "valid mixed",
			sessionID: "my-session_123",
			wantErr:   false,
		},
		{
			name:      "valid single character",
			sessionID: "a",
			wantErr:   false,
		},
		{
			name:      "valid max length",
			sessionID: strings.Repeat("a", MaxSessionIDLength),
			wantErr:   false,
		},

		// Path traversal attempts
		{
			name:      "path traversal with double dots",
			sessionID: "../etc/passwd",
			wantErr:   true,
			errReason: "path traversal",
		},
		{
			name:      "path traversal prefix",
			sessionID: "../../etc/passwd",
			wantErr:   true,
			errReason: "path traversal",
		},
		{
			name:      "path traversal suffix",
			sessionID: "session/..",
			wantErr:   true,
			errReason: "path traversal", // Caught by .. check before separator check
		},
		{
			name:      "path traversal middle",
			sessionID: "sess../ion",
			wantErr:   true,
			errReason: "path traversal",
		},
		{
			name:      "forward slash",
			sessionID: "session/test",
			wantErr:   true,
			errReason: "path separators",
		},
		{
			name:      "backslash",
			sessionID: "session\\test",
			wantErr:   true,
			errReason: "path separators",
		},
		{
			name:      "absolute unix path",
			sessionID: "/etc/passwd",
			wantErr:   true,
			errReason: "path separators",
		},
		{
			name:      "absolute windows path",
			sessionID: "C:\\Windows\\System32",
			wantErr:   true,
			errReason: "path separators",
		},

		// URL-encoded path traversal
		{
			name:      "url encoded dot",
			sessionID: "session%2e%2e",
			wantErr:   true,
			errReason: "URL-encoded",
		},
		{
			name:      "url encoded slash",
			sessionID: "session%2ftest",
			wantErr:   true,
			errReason: "URL-encoded",
		},
		{
			name:      "url encoded backslash",
			sessionID: "session%5ctest",
			wantErr:   true,
			errReason: "URL-encoded",
		},
		{
			name:      "mixed encoding traversal",
			sessionID: "..%2f..%2fetc",
			wantErr:   true,
			errReason: "path traversal",
		},
		{
			name:      "uppercase url encoding",
			sessionID: "session%2F",
			wantErr:   true,
			errReason: "URL-encoded",
		},

		// Unicode and encoding bypasses
		{
			name:      "fullwidth full stop",
			sessionID: "session\uff0e\uff0e",
			wantErr:   true,
			errReason: "fullwidth unicode",
		},
		{
			name:      "fullwidth solidus",
			sessionID: "session\uff0ftest",
			wantErr:   true,
			errReason: "fullwidth unicode",
		},

		// Control characters and null bytes
		{
			name:      "null byte",
			sessionID: "session\x00test",
			wantErr:   true,
			errReason: "null bytes",
		},
		{
			name:      "carriage return",
			sessionID: "session\rtest",
			wantErr:   true,
			errReason: "control characters",
		},
		{
			name:      "newline",
			sessionID: "session\ntest",
			wantErr:   true,
			errReason: "control characters",
		},
		{
			name:      "tab character",
			sessionID: "session\ttest",
			wantErr:   true,
			errReason: "control characters",
		},
		{
			name:      "bell character",
			sessionID: "session\x07test",
			wantErr:   true,
			errReason: "control characters",
		},

		// Reserved and special names
		{
			name:      "single dot",
			sessionID: ".",
			wantErr:   true,
			errReason: "invalid characters", // Caught by character allowlist
		},
		{
			name:      "double dot",
			sessionID: "..",
			wantErr:   true,
			errReason: "path traversal", // Caught by .. check
		},

		// Length constraints
		{
			name:      "empty string",
			sessionID: "",
			wantErr:   true,
			errReason: "length",
		},
		{
			name:      "too long",
			sessionID: strings.Repeat("a", MaxSessionIDLength+1),
			wantErr:   true,
			errReason: "length",
		},

		// Invalid characters
		{
			name:      "space character",
			sessionID: "session test",
			wantErr:   true,
			errReason: "invalid characters",
		},
		{
			name:      "special characters",
			sessionID: "session@test",
			wantErr:   true,
			errReason: "invalid characters",
		},
		{
			name:      "dot in middle",
			sessionID: "session.test",
			wantErr:   true,
			errReason: "invalid characters",
		},
		{
			name:      "colon",
			sessionID: "session:test",
			wantErr:   true,
			errReason: "invalid characters",
		},
		{
			name:      "semicolon",
			sessionID: "session;test",
			wantErr:   true,
			errReason: "invalid characters",
		},
		{
			name:      "asterisk",
			sessionID: "session*",
			wantErr:   true,
			errReason: "invalid characters",
		},
		{
			name:      "question mark",
			sessionID: "session?",
			wantErr:   true,
			errReason: "invalid characters",
		},
		{
			name:      "quote",
			sessionID: "session\"test",
			wantErr:   true,
			errReason: "invalid characters",
		},
		{
			name:      "less than",
			sessionID: "session<test",
			wantErr:   true,
			errReason: "invalid characters",
		},
		{
			name:      "greater than",
			sessionID: "session>test",
			wantErr:   true,
			errReason: "invalid characters",
		},
		{
			name:      "pipe",
			sessionID: "session|test",
			wantErr:   true,
			errReason: "invalid characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSessionID(tt.sessionID)

			if tt.wantErr {
				require.Error(t, err, "Expected error for session ID: %q", tt.sessionID)
				assert.IsType(t, &SessionIDValidationError{}, err, "Expected SessionIDValidationError")
				if tt.errReason != "" {
					assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.errReason),
						"Error message should mention: %s", tt.errReason)
				}
			} else {
				assert.NoError(t, err, "Expected no error for valid session ID: %q", tt.sessionID)
			}
		})
	}
}

// TestValidateAndResolveSessionPath tests the complete path validation logic.
func TestValidateAndResolveSessionPath(t *testing.T) {
	// Create a temporary sessions root for testing
	sessionsRoot := "/tmp/test-sessions"
	if runtime.GOOS == "windows" {
		sessionsRoot = "C:\\temp\\test-sessions"
	}

	tests := []struct {
		name         string
		sessionID    string
		sessionsRoot string
		wantErr      bool
		checkPath    func(t *testing.T, path string)
	}{
		{
			name:         "valid session ID creates correct path",
			sessionID:    "session-123",
			sessionsRoot: sessionsRoot,
			wantErr:      false,
			checkPath: func(t *testing.T, path string) {
				expected := filepath.Join(sessionsRoot, "session-123")
				assert.Equal(t, filepath.Clean(expected), path)
			},
		},
		{
			name:         "path stays within sessions root",
			sessionID:    "valid-session",
			sessionsRoot: sessionsRoot,
			wantErr:      false,
			checkPath: func(t *testing.T, path string) {
				// Verify the path is under sessions root
				rel, err := filepath.Rel(sessionsRoot, path)
				require.NoError(t, err)
				assert.False(t, strings.HasPrefix(rel, ".."), "Path should not escape sessions root")
			},
		},
		{
			name:         "path traversal attempt rejected",
			sessionID:    "../etc/passwd",
			sessionsRoot: sessionsRoot,
			wantErr:      true,
		},
		{
			name:         "relative sessions root rejected",
			sessionID:    "session-123",
			sessionsRoot: "relative/path",
			wantErr:      true,
		},
		{
			name:         "invalid session ID rejected",
			sessionID:    "session/with/slashes",
			sessionsRoot: sessionsRoot,
			wantErr:      true,
		},
		{
			name:         "url encoded traversal rejected",
			sessionID:    "..%2fetc",
			sessionsRoot: sessionsRoot,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := ValidateAndResolveSessionPath(tt.sessionID, tt.sessionsRoot)

			if tt.wantErr {
				require.Error(t, err, "Expected error for session ID: %q", tt.sessionID)
			} else {
				require.NoError(t, err, "Expected no error for session ID: %q", tt.sessionID)
				require.NotEmpty(t, path, "Path should not be empty")
				if tt.checkPath != nil {
					tt.checkPath(t, path)
				}
			}
		})
	}
}

// TestPathTraversalDefenseInDepth tests that even if a session ID passes initial validation,
// the path resolution will catch any escapes.
func TestPathTraversalDefenseInDepth(t *testing.T) {
	sessionsRoot := "/tmp/test-sessions"
	if runtime.GOOS == "windows" {
		sessionsRoot = "C:\\temp\\test-sessions"
	}

	// These session IDs might theoretically pass some checks but should still be caught
	dangerousIDs := []string{
		// These all violate the character allowlist and will be rejected early
		"../sibling",
		"./current",
		"parent/../sibling",
	}

	for _, sessionID := range dangerousIDs {
		t.Run(sessionID, func(t *testing.T) {
			path, err := ValidateAndResolveSessionPath(sessionID, sessionsRoot)

			// Should be rejected either by ID validation or path validation
			require.Error(t, err, "Dangerous session ID should be rejected: %q", sessionID)
			assert.Empty(t, path, "Path should be empty on error")
		})
	}
}

// TestSessionIDValidationError tests the error type and formatting.
func TestSessionIDValidationError(t *testing.T) {
	err := &SessionIDValidationError{
		SessionID: "bad-session",
		Reason:    "contains invalid characters",
	}

	errMsg := err.Error()
	assert.Contains(t, errMsg, "bad-session")
	assert.Contains(t, errMsg, "invalid characters")
}

// TestSanitizeSessionIDForLog tests log sanitization prevents log injection.
func TestSanitizeSessionIDForLog(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		check     func(t *testing.T, sanitized string)
	}{
		{
			name:      "normal ID unchanged",
			sessionID: "session-123",
			check: func(t *testing.T, sanitized string) {
				assert.Equal(t, "session-123", sanitized)
			},
		},
		{
			name:      "control characters replaced",
			sessionID: "session\ntest\rinjection",
			check: func(t *testing.T, sanitized string) {
				assert.NotContains(t, sanitized, "\n")
				assert.NotContains(t, sanitized, "\r")
			},
		},
		{
			name:      "long ID truncated",
			sessionID: strings.Repeat("a", 100),
			check: func(t *testing.T, sanitized string) {
				assert.LessOrEqual(t, len(sanitized), 67) // 64 + "..."
			},
		},
		{
			name:      "null bytes removed",
			sessionID: "session\x00test",
			check: func(t *testing.T, sanitized string) {
				assert.NotContains(t, sanitized, "\x00")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := sanitizeSessionIDForLog(tt.sessionID)
			tt.check(t, sanitized)
		})
	}
}

// BenchmarkValidateSessionID benchmarks the validation performance.
func BenchmarkValidateSessionID(b *testing.B) {
	validID := "valid-session-123"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateSessionID(validID)
	}
}

// BenchmarkValidateSessionIDPathTraversal benchmarks validation with attack attempts.
func BenchmarkValidateSessionIDPathTraversal(b *testing.B) {
	attackID := "../../etc/passwd"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateSessionID(attackID)
	}
}
