package errors

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// Test FileError
func TestFileError(t *testing.T) {
	tests := []struct {
		name        string
		err         *FileError
		wantContain []string
	}{
		{
			name: "not found error",
			err: &FileError{
				Type:      FileErrorNotFound,
				Path:      "/path/to/file.txt",
				Operation: "read",
				Message:   "file does not exist",
			},
			wantContain: []string{"read", "/path/to/file.txt", "does not exist", "Verify the path"},
		},
		{
			name: "permission error",
			err: &FileError{
				Type:      FileErrorPermission,
				Path:      "/protected/file",
				Operation: "write",
				Message:   "permission denied",
			},
			wantContain: []string{"write", "/protected/file", "permission denied", "Check file permissions"},
		},
		{
			name: "directory error",
			err: &FileError{
				Type:      FileErrorIsDirectory,
				Path:      "/some/dir",
				Operation: "read",
				Message:   "path is a directory",
			},
			wantContain: []string{"read", "/some/dir", "directory", "Provide a file path"},
		},
		{
			name: "binary file error",
			err: &FileError{
				Type:      FileErrorBinary,
				Path:      "image.png",
				Operation: "read",
				Message:   "file is binary",
			},
			wantContain: []string{"read", "image.png", "binary", "binary file reader"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("Error() = %q, should contain %q", got, want)
				}
			}
		})
	}
}

// Test FileError helper constructors
func TestFileErrorHelpers(t *testing.T) {
	t.Run("NewFileError with os.ErrNotExist", func(t *testing.T) {
		err := NewFileError("read", "/test/file", os.ErrNotExist)
		if err.Type != FileErrorNotFound {
			t.Errorf("Type = %v, want %v", err.Type, FileErrorNotFound)
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("Error message should contain 'does not exist'")
		}
	})

	t.Run("NewFileError with permission error", func(t *testing.T) {
		err := NewFileError("write", "/test/file", os.ErrPermission)
		if err.Type != FileErrorPermission {
			t.Errorf("Type = %v, want %v", err.Type, FileErrorPermission)
		}
		if !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("Error message should contain 'permission denied'")
		}
	})

	t.Run("NewFileNotFoundError", func(t *testing.T) {
		err := NewFileNotFoundError("/missing", "read")
		if err.Type != FileErrorNotFound {
			t.Errorf("Type = %v, want %v", err.Type, FileErrorNotFound)
		}
		if err.Path != "/missing" {
			t.Errorf("Path = %q, want %q", err.Path, "/missing")
		}
	})

	t.Run("NewPermissionError", func(t *testing.T) {
		err := NewPermissionError("/protected", "write")
		if err.Type != FileErrorPermission {
			t.Errorf("Type = %v, want %v", err.Type, FileErrorPermission)
		}
	})

	t.Run("NewDirectoryError", func(t *testing.T) {
		err := NewDirectoryError("/dir", "read")
		if err.Type != FileErrorIsDirectory {
			t.Errorf("Type = %v, want %v", err.Type, FileErrorIsDirectory)
		}
	})

	t.Run("NewBinaryFileError", func(t *testing.T) {
		err := NewBinaryFileError("image.png", 1024*1024)
		if err.Type != FileErrorBinary {
			t.Errorf("Type = %v, want %v", err.Type, FileErrorBinary)
		}
		if !strings.Contains(err.Error(), "1.00 MB") {
			t.Errorf("Error should contain formatted size")
		}
	})
}

// Test ValidationError
func TestValidationError(t *testing.T) {
	tests := []struct {
		name        string
		err         *ValidationError
		wantContain []string
	}{
		{
			name: "basic validation error",
			err: &ValidationError{
				Field:   "username",
				Message: "must not be empty",
			},
			wantContain: []string{"validation error", "username", "must not be empty"},
		},
		{
			name: "with value",
			err: &ValidationError{
				Field:   "age",
				Value:   -5,
				Message: "must be positive",
			},
			wantContain: []string{"validation error", "age", "-5", "positive"},
		},
		{
			name: "with underlying error",
			err: &ValidationError{
				Field:   "email",
				Message: "invalid format",
				Err:     fmt.Errorf("missing @ symbol"),
			},
			wantContain: []string{"validation error", "email", "invalid format", "missing @ symbol"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("Error() = %q, should contain %q", got, want)
				}
			}
		})
	}
}

// Test ValidationError helpers
func TestValidationErrorHelpers(t *testing.T) {
	t.Run("NewValidationError", func(t *testing.T) {
		err := NewValidationError("field1", "is required")
		if err.Field != "field1" {
			t.Errorf("Field = %q, want %q", err.Field, "field1")
		}
		if err.Message != "is required" {
			t.Errorf("Message = %q, want %q", err.Message, "is required")
		}
	})

	t.Run("NewValidationErrorWithValue", func(t *testing.T) {
		err := NewValidationErrorWithValue("port", 99999, "out of range")
		if err.Field != "port" {
			t.Errorf("Field = %q, want %q", err.Field, "port")
		}
		if err.Value != 99999 {
			t.Errorf("Value = %v, want %v", err.Value, 99999)
		}
	})
}

// Test ConfigError
func TestConfigError(t *testing.T) {
	tests := []struct {
		name        string
		err         *ConfigError
		wantContain []string
	}{
		{
			name: "basic config error",
			err: &ConfigError{
				Field:   "model",
				Message: "cannot be empty",
			},
			wantContain: []string{"configuration error", "model", "cannot be empty"},
		},
		{
			name: "with config path",
			err: &ConfigError{
				ConfigPath: "/etc/config.toml",
				Field:      "api_key",
				Message:    "invalid format",
			},
			wantContain: []string{"configuration error", "/etc/config.toml", "api_key", "invalid format"},
		},
		{
			name: "with suggestion",
			err: &ConfigError{
				Field:      "timeout",
				Message:    "must be positive",
				Suggestion: "Set timeout to a value greater than 0",
			},
			wantContain: []string{"configuration error", "timeout", "must be positive", "Suggestion", "greater than 0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("Error() = %q, should contain %q", got, want)
				}
			}
		})
	}
}

// Test ConfigError helpers
func TestConfigErrorHelpers(t *testing.T) {
	t.Run("NewConfigError", func(t *testing.T) {
		err := NewConfigError("database", "connection failed")
		if err.Field != "database" {
			t.Errorf("Field = %q, want %q", err.Field, "database")
		}
	})

	t.Run("NewConfigErrorWithSuggestion", func(t *testing.T) {
		err := NewConfigErrorWithSuggestion("port", "invalid", "Use a value between 1-65535")
		if err.Suggestion != "Use a value between 1-65535" {
			t.Errorf("Suggestion = %q, want suggestion about port range", err.Suggestion)
		}
	})
}

// Test PathError
func TestPathError(t *testing.T) {
	tests := []struct {
		name        string
		err         *PathError
		wantContain []string
	}{
		{
			name: "path traversal error",
			err: &PathError{
				Path:         "/workspace/../etc/passwd",
				Workspace:    "/workspace",
				Reason:       "path traversal detected",
				SecurityNote: "attempting to access files outside the workspace",
			},
			wantContain: []string{"path error", "traversal", "Security", "outside the workspace"},
		},
		{
			name: "sensitive path error",
			err: &PathError{
				Path:         "/etc/shadow",
				Operation:    "write",
				Reason:       "sensitive system path",
				SecurityNote: "modifying system paths is not allowed",
			},
			wantContain: []string{"path error", "sensitive", "write", "Security", "not allowed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("Error() = %q, should contain %q", got, want)
				}
			}
		})
	}
}

// Test PathError helpers
func TestPathErrorHelpers(t *testing.T) {
	t.Run("NewPathTraversalError", func(t *testing.T) {
		err := NewPathTraversalError("../../etc/passwd", "/workspace")
		if !strings.Contains(err.Reason, "traversal") {
			t.Errorf("Reason should mention traversal")
		}
		if !strings.Contains(err.SecurityNote, "outside the workspace") {
			t.Errorf("SecurityNote should mention workspace boundary")
		}
	})

	t.Run("NewSensitivePathError", func(t *testing.T) {
		err := NewSensitivePathError("/etc/shadow", "write")
		if !strings.Contains(err.Reason, "sensitive") {
			t.Errorf("Reason should mention sensitive")
		}
		if err.Operation != "write" {
			t.Errorf("Operation = %q, want %q", err.Operation, "write")
		}
	})
}

// Test APIError
func TestAPIError(t *testing.T) {
	tests := []struct {
		name        string
		err         *APIError
		wantContain []string
	}{
		{
			name: "basic API error",
			err: &APIError{
				Endpoint:   "/api/v1/users",
				StatusCode: 404,
				Message:    "not found",
			},
			wantContain: []string{"API error", "/api/v1/users", "404", "not found"},
		},
		{
			name: "with retry information",
			err: &APIError{
				Endpoint:   "/api/v1/data",
				StatusCode: 503,
				Message:    "service unavailable",
				CanRetry:   true,
				RetryAfter: "30 seconds",
			},
			wantContain: []string{"API error", "503", "service unavailable", "retried", "30 seconds"},
		},
		{
			name: "with request ID and suggestion",
			err: &APIError{
				Endpoint:   "/api/v1/submit",
				StatusCode: 401,
				RequestID:  "req-xyz",
				Message:    "unauthorized",
				Suggestion: "Check your API key",
			},
			wantContain: []string{"API error", "401", "unauthorized", "req-xyz", "Check your API key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("Error() = %q, should contain %q", got, want)
				}
			}
		})
	}
}

// Test APIError helpers
func TestAPIErrorHelpers(t *testing.T) {
	t.Run("NewAPIError", func(t *testing.T) {
		err := NewAPIError("/api/test", 500, "internal error")
		if err.StatusCode != 500 {
			t.Errorf("StatusCode = %d, want %d", err.StatusCode, 500)
		}
	})

	t.Run("NewAPIErrorWithRetry", func(t *testing.T) {
		err := NewAPIErrorWithRetry("/api/test", 429, "rate limited", "60 seconds")
		if !err.CanRetry {
			t.Error("CanRetry should be true")
		}
		if err.RetryAfter != "60 seconds" {
			t.Errorf("RetryAfter = %q, want %q", err.RetryAfter, "60 seconds")
		}
	})
}

// Test ToolExecutionError
func TestToolExecutionError(t *testing.T) {
	tests := []struct {
		name        string
		err         *ToolExecutionError
		wantContain []string
	}{
		{
			name: "basic tool error",
			err: &ToolExecutionError{
				ToolName: "grep",
				ExitCode: 1,
				Stderr:   "pattern not found",
			},
			wantContain: []string{"grep", "execution failed", "exit code: 1", "pattern not found"},
		},
		{
			name: "with suggestion",
			err: &ToolExecutionError{
				ToolName:   "compile",
				ExitCode:   2,
				Stderr:     "syntax error",
				Suggestion: "Check your source code syntax",
			},
			wantContain: []string{"compile", "exit code: 2", "syntax error", "Suggestion", "syntax"},
		},
		{
			name: "with long stderr",
			err: &ToolExecutionError{
				ToolName: "test",
				ExitCode: 1,
				Stderr:   strings.Repeat("error line\n", 50), // Long output
			},
			wantContain: []string{"test", "execution failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("Error() = %q, should contain %q", got, want)
				}
			}
			// Verify stderr is truncated if too long
			if len(tt.err.Stderr) > 200 && len(got) > 500 {
				if !strings.Contains(got, "...") {
					t.Error("Long stderr should be truncated with ...")
				}
			}
		})
	}
}

// Test ToolExecutionError helper
func TestToolExecutionErrorHelper(t *testing.T) {
	err := NewToolExecutionError("make", 2, "build failed")
	if err.ToolName != "make" {
		t.Errorf("ToolName = %q, want %q", err.ToolName, "make")
	}
	if err.ExitCode != 2 {
		t.Errorf("ExitCode = %d, want %d", err.ExitCode, 2)
	}
}

// Test WrapWithContext
func TestWrapWithContext(t *testing.T) {
	t.Run("wrap non-nil error", func(t *testing.T) {
		base := fmt.Errorf("base error")
		wrapped := WrapWithContext(base, "additional context")
		if wrapped == nil {
			t.Error("WrapWithContext should not return nil for non-nil error")
		}
		if !strings.Contains(wrapped.Error(), "additional context") {
			t.Error("Wrapped error should contain context")
		}
		if !strings.Contains(wrapped.Error(), "base error") {
			t.Error("Wrapped error should contain base error")
		}
	})

	t.Run("wrap nil error", func(t *testing.T) {
		wrapped := WrapWithContext(nil, "context")
		if wrapped != nil {
			t.Error("WrapWithContext should return nil for nil error")
		}
	})
}

// Test WrapWithContextf
func TestWrapWithContextf(t *testing.T) {
	t.Run("wrap with formatted context", func(t *testing.T) {
		base := fmt.Errorf("base error")
		wrapped := WrapWithContextf(base, "operation %s failed on %s", "read", "file.txt")
		if !strings.Contains(wrapped.Error(), "operation read failed on file.txt") {
			t.Error("Wrapped error should contain formatted context")
		}
	})

	t.Run("wrap nil error", func(t *testing.T) {
		wrapped := WrapWithContextf(nil, "context %s", "test")
		if wrapped != nil {
			t.Error("WrapWithContextf should return nil for nil error")
		}
	})
}

// Test formatBytes
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"bytes", 500, "500 B"},
		{"kilobytes", 2048, "2.00 KB"},
		{"megabytes", 5*1024*1024, "5.00 MB"},
		{"gigabytes", 3*1024*1024*1024, "3.00 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

// Test IsRetryable
func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "retryable API error",
			err:  &APIError{CanRetry: true},
			want: true,
		},
		{
			name: "non-retryable API error",
			err:  &APIError{CanRetry: false},
			want: false,
		},
		{
			name: "connection error",
			err:  NewConnectionError(fmt.Errorf("test")),
			want: true,
		},
		{
			name: "timeout error",
			err:  ErrTimeout,
			want: true,
		},
		{
			name: "regular error",
			err:  fmt.Errorf("regular error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryable(tt.err)
			if got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test error unwrapping
func TestErrorUnwrapping(t *testing.T) {
	t.Run("FileError unwrap", func(t *testing.T) {
		base := fmt.Errorf("underlying error")
		fileErr := &FileError{Err: base}
		if !AsError(fileErr, &base) {
			t.Error("FileError should preserve underlying error for unwrapping")
		}
	})

	t.Run("ValidationError unwrap", func(t *testing.T) {
		base := fmt.Errorf("validation failed")
		valErr := &ValidationError{Err: base}
		if !AsError(valErr, &base) {
			t.Error("ValidationError should preserve underlying error for unwrapping")
		}
	})

	t.Run("ConfigError unwrap", func(t *testing.T) {
		base := fmt.Errorf("config parse error")
		cfgErr := &ConfigError{Err: base}
		if !AsError(cfgErr, &base) {
			t.Error("ConfigError should preserve underlying error for unwrapping")
		}
	})

	t.Run("PathError unwrap", func(t *testing.T) {
		base := fmt.Errorf("path error")
		pathErr := &PathError{Err: base}
		if !AsError(pathErr, &base) {
			t.Error("PathError should preserve underlying error for unwrapping")
		}
	})

	t.Run("APIError unwrap", func(t *testing.T) {
		base := fmt.Errorf("api error")
		apiErr := &APIError{Err: base}
		if !AsError(apiErr, &base) {
			t.Error("APIError should preserve underlying error for unwrapping")
		}
	})

	t.Run("ToolExecutionError unwrap", func(t *testing.T) {
		base := fmt.Errorf("tool error")
		toolErr := &ToolExecutionError{Err: base}
		if !AsError(toolErr, &base) {
			t.Error("ToolExecutionError should preserve underlying error for unwrapping")
		}
	})
}
