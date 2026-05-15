package errors

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Test sentinel errors
func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"ErrCancelled", ErrCancelled, "operation cancelled"},
		{"ErrNotFound", ErrNotFound, "not found"},
		{"ErrTimeout", ErrTimeout, "timeout"},
		{"ErrInterrupted", ErrInterrupted, "interrupted (Ctrl-C). Something went wrong? Hit `/feedback` to report the issue."},
		{"ErrSpawn", ErrSpawn, "spawn failed: child stdout/stderr not captured"},
		{"ErrContextWindowExceeded", ErrContextWindowExceeded, "Codex ran out of room in the model's context window. Start a new conversation or clear earlier history before retrying."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Test error wrapping and Is
func TestErrorWrapping(t *testing.T) {
	base := fmt.Errorf("base error")
	wrapped := fmt.Errorf("wrapped: %w", base)

	if !errors.Is(wrapped, base) {
		t.Error("errors.Is should detect wrapped error")
	}

	// Test wrapping with our sentinel errors
	wrappedCancelled := fmt.Errorf("operation failed: %w", ErrCancelled)
	if !errors.Is(wrappedCancelled, ErrCancelled) {
		t.Error("errors.Is should detect wrapped ErrCancelled")
	}
}

// Test ConnectionError
func TestConnectionError(t *testing.T) {
	tests := []struct {
		name    string
		err     *ConnectionError
		wantMsg string
	}{
		{
			name:    "basic connection error",
			err:     NewConnectionError(fmt.Errorf("connection refused")),
			wantMsg: "connection failed: connection refused",
		},
		{
			name:    "with context",
			err:     &ConnectionError{Err: fmt.Errorf("timeout"), Context: "connecting to API"},
			wantMsg: "connection failed: connecting to API: timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

// Test ConnectionError unwrapping
func TestConnectionErrorUnwrap(t *testing.T) {
	base := fmt.Errorf("network error")
	connErr := NewConnectionError(base)

	if !errors.Is(connErr, base) {
		t.Error("errors.Is should detect wrapped error in ConnectionError")
	}

	var target *ConnectionError
	if !errors.As(connErr, &target) {
		t.Error("errors.As should work with ConnectionError")
	}
}

// Test SandboxError
func TestSandboxError(t *testing.T) {
	tests := []struct {
		name    string
		err     *SandboxError
		wantMsg string
	}{
		{
			name: "denied with exit code",
			err: &SandboxError{
				Type:     SandboxDenied,
				ExitCode: 1,
				Stdout:   "output",
				Stderr:   "error output",
			},
			wantMsg: "sandbox denied exec error, exit code: 1, stdout: output, stderr: error output",
		},
		{
			name: "timeout",
			err: &SandboxError{
				Type:     SandboxTimeout,
				ExitCode: 0,
				Duration: 5 * time.Second,
			},
			wantMsg: "command timed out after 5s",
		},
		{
			name: "signal",
			err: &SandboxError{
				Type:   SandboxSignal,
				Signal: 9,
			},
			wantMsg: "command was killed by signal 9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

// Test UnexpectedStatusError
func TestUnexpectedStatusError(t *testing.T) {
	tests := []struct {
		name    string
		err     *UnexpectedStatusError
		wantMsg string
	}{
		{
			name: "without request ID",
			err: &UnexpectedStatusError{
				Status: http.StatusBadRequest,
				Body:   "invalid request",
			},
			wantMsg: "unexpected status 400: invalid request",
		},
		{
			name: "with request ID",
			err: &UnexpectedStatusError{
				Status:    http.StatusInternalServerError,
				Body:      "server error",
				RequestID: "req-123",
			},
			wantMsg: "unexpected status 500: server error, request id: req-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

// Test ResponseStreamError
func TestResponseStreamError(t *testing.T) {
	tests := []struct {
		name    string
		err     *ResponseStreamError
		wantMsg string
	}{
		{
			name: "without request ID",
			err: &ResponseStreamError{
				Err: fmt.Errorf("stream closed"),
			},
			wantMsg: "error while reading the server response: stream closed",
		},
		{
			name: "with request ID",
			err: &ResponseStreamError{
				Err:       fmt.Errorf("connection lost"),
				RequestID: "req-456",
			},
			wantMsg: "error while reading the server response: connection lost, request id: req-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

// Test ResponseStreamError unwrapping
func TestResponseStreamErrorUnwrap(t *testing.T) {
	base := fmt.Errorf("stream error")
	streamErr := &ResponseStreamError{Err: base}

	if !errors.Is(streamErr, base) {
		t.Error("errors.Is should detect wrapped error in ResponseStreamError")
	}
}

// Test RetryLimitError
func TestRetryLimitError(t *testing.T) {
	tests := []struct {
		name    string
		err     *RetryLimitError
		wantMsg string
	}{
		{
			name: "without request ID",
			err: &RetryLimitError{
				Status: http.StatusTooManyRequests,
			},
			wantMsg: "exceeded retry limit, last status: 429",
		},
		{
			name: "with request ID",
			err: &RetryLimitError{
				Status:    http.StatusServiceUnavailable,
				RequestID: "req-789",
			},
			wantMsg: "exceeded retry limit, last status: 503, request id: req-789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

// Test EnvVarError
func TestEnvVarError(t *testing.T) {
	tests := []struct {
		name         string
		varName      string
		instructions string
		wantMsg      string
	}{
		{
			name:    "without instructions",
			varName: "API_KEY",
			wantMsg: "missing environment variable: `API_KEY`",
		},
		{
			name:         "with instructions",
			varName:      "TOKEN",
			instructions: "Set TOKEN=your_token",
			wantMsg:      "missing environment variable: `TOKEN`. Set TOKEN=your_token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewEnvVarError(tt.varName, tt.instructions)
			if got := err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

// Test UsageLimitError
func TestUsageLimitError(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		err      *UsageLimitError
		wantMsg  string
		checkMsg bool
	}{
		{
			name: "no plan type",
			err: &UsageLimitError{
				PlanType: "",
			},
			wantMsg: "you've hit your usage limit. Try again later.",
		},
		{
			name: "free plan",
			err: &UsageLimitError{
				PlanType: "free",
			},
			checkMsg: true, // Contains "Upgrade to Plus"
		},
		{
			name: "with reset time",
			err: &UsageLimitError{
				PlanType: "pro",
				ResetsAt: now.Add(5 * time.Minute),
			},
			checkMsg: true, // Contains "Try again in"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if tt.checkMsg {
				if len(got) == 0 {
					t.Error("Error() should return non-empty message")
				}
			} else if got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

// Test StreamError
func TestStreamError(t *testing.T) {
	tests := []struct {
		name       string
		message    string
		retryDelay time.Duration
		wantMsg    string
	}{
		{
			name:    "without retry delay",
			message: "connection lost",
			wantMsg: "stream disconnected before completion: connection lost",
		},
		{
			name:       "with retry delay",
			message:    "temporary error",
			retryDelay: 5 * time.Second,
			wantMsg:    "stream disconnected before completion: temporary error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewStreamError(tt.message, tt.retryDelay)
			if got := err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
			if err.RetryDelay != tt.retryDelay {
				t.Errorf("RetryDelay = %v, want %v", err.RetryDelay, tt.retryDelay)
			}
		})
	}
}

// Test ConversationNotFoundError
func TestConversationNotFoundError(t *testing.T) {
	err := NewConversationNotFoundError("conv-123")
	want := "no conversation with id: conv-123"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if err.ConversationID != "conv-123" {
		t.Errorf("ConversationID = %q, want %q", err.ConversationID, "conv-123")
	}
}

// Test UnsupportedOperationError
func TestUnsupportedOperationError(t *testing.T) {
	err := NewUnsupportedOperationError("operation X")
	want := "unsupported operation: operation X"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// Test FatalError
func TestFatalError(t *testing.T) {
	err := NewFatalError("critical failure")
	want := "fatal error: critical failure"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// Test error type assertions with errors.As
func TestErrorsAs(t *testing.T) {
	t.Run("ConnectionError", func(t *testing.T) {
		err := NewConnectionError(fmt.Errorf("test"))
		var target *ConnectionError
		if !errors.As(err, &target) {
			t.Error("errors.As should detect ConnectionError")
		}
	})

	t.Run("SandboxError", func(t *testing.T) {
		err := &SandboxError{Type: SandboxDenied}
		var target *SandboxError
		if !errors.As(err, &target) {
			t.Error("errors.As should detect SandboxError")
		}
	})

	t.Run("UnexpectedStatusError", func(t *testing.T) {
		err := &UnexpectedStatusError{Status: http.StatusBadRequest}
		var target *UnexpectedStatusError
		if !errors.As(err, &target) {
			t.Error("errors.As should detect UnexpectedStatusError")
		}
	})

	t.Run("wrapped error", func(t *testing.T) {
		err := fmt.Errorf("wrapped: %w", NewConnectionError(fmt.Errorf("test")))
		var target *ConnectionError
		if !errors.As(err, &target) {
			t.Error("errors.As should detect wrapped ConnectionError")
		}
	})
}

// Test IsConnectionError helper
func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "direct ConnectionError",
			err:  NewConnectionError(fmt.Errorf("test")),
			want: true,
		},
		{
			name: "wrapped ConnectionError",
			err:  fmt.Errorf("wrapped: %w", NewConnectionError(fmt.Errorf("test"))),
			want: true,
		},
		{
			name: "other error",
			err:  fmt.Errorf("regular error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConnectionError(tt.err); got != tt.want {
				t.Errorf("IsConnectionError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test IsSandboxError helper
func TestIsSandboxError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "direct SandboxError",
			err:  &SandboxError{Type: SandboxDenied},
			want: true,
		},
		{
			name: "wrapped SandboxError",
			err:  fmt.Errorf("wrapped: %w", &SandboxError{Type: SandboxTimeout}),
			want: true,
		},
		{
			name: "other error",
			err:  fmt.Errorf("regular error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSandboxError(tt.err); got != tt.want {
				t.Errorf("IsSandboxError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test error context preservation
func TestErrorContextPreservation(t *testing.T) {
	// Create a chain of wrapped errors
	base := fmt.Errorf("base error")
	wrapped1 := fmt.Errorf("layer 1: %w", base)
	wrapped2 := fmt.Errorf("layer 2: %w", wrapped1)

	// Verify errors.Is works through the chain
	if !errors.Is(wrapped2, base) {
		t.Error("errors.Is should detect base error through multiple wraps")
	}

	// Test with our custom types
	connErr := NewConnectionError(base)
	wrappedConn := fmt.Errorf("connection issue: %w", connErr)

	if !errors.Is(wrappedConn, base) {
		t.Error("errors.Is should detect base error through custom type")
	}

	var ce *ConnectionError
	if !errors.As(wrappedConn, &ce) {
		t.Error("errors.As should detect ConnectionError through wrap")
	}
}

// Test helper functions for common error creation
func TestHelperFunctions(t *testing.T) {
	t.Run("NewConnectionError", func(t *testing.T) {
		err := NewConnectionError(fmt.Errorf("test"))
		if err == nil {
			t.Error("NewConnectionError should not return nil")
		}
		// Type is already known from function signature
		if err.Err == nil {
			t.Error("ConnectionError should have underlying error")
		}
	})

	t.Run("NewEnvVarError", func(t *testing.T) {
		err := NewEnvVarError("VAR", "")
		if err == nil {
			t.Error("NewEnvVarError should not return nil")
		}
		// Type is already known from function signature
		if err.VarName != "VAR" {
			t.Error("EnvVarError should have correct VarName")
		}
	})

	t.Run("NewStreamError", func(t *testing.T) {
		err := NewStreamError("test", 0)
		if err == nil {
			t.Error("NewStreamError should not return nil")
		}
		// Type is already known from function signature
		if err.Message != "test" {
			t.Error("StreamError should have correct Message")
		}
	})
}
