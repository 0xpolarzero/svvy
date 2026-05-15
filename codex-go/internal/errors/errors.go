// Package errors provides error types and utilities for Codex Go.
// This package implements error types matching codex-rs/core/src/error.rs
// and follows Go 1.13+ error wrapping idioms.
package errors

import (
	"errors"
	"fmt"
	"time"
)

// Sentinel errors for common cases
var (
	// ErrCancelled indicates an operation was cancelled
	ErrCancelled = errors.New("operation cancelled")

	// ErrNotFound indicates a resource was not found
	ErrNotFound = errors.New("not found")

	// ErrTimeout indicates a timeout occurred
	ErrTimeout = errors.New("timeout")

	// ErrInterrupted indicates the operation was interrupted by user (Ctrl-C)
	ErrInterrupted = errors.New("interrupted (Ctrl-C). Something went wrong? Hit `/feedback` to report the issue.")

	// ErrSpawn indicates a child process failed to spawn
	ErrSpawn = errors.New("spawn failed: child stdout/stderr not captured")

	// ErrContextWindowExceeded indicates the model's context window was exceeded
	ErrContextWindowExceeded = errors.New("Codex ran out of room in the model's context window. Start a new conversation or clear earlier history before retrying.")

	// ErrSessionConfiguredNotFirstEvent indicates session.configured was not the first event
	ErrSessionConfiguredNotFirstEvent = errors.New("session configured event was not the first event in the stream")

	// ErrUsageNotIncluded indicates the plan doesn't include Codex usage
	ErrUsageNotIncluded = errors.New("To use Codex with your ChatGPT plan, upgrade to Plus: https://openai.com/chatgpt/pricing.")

	// ErrInternalServerError indicates a server-side error
	ErrInternalServerError = errors.New("We're currently experiencing high demand, which may cause temporary errors.")

	// ErrInternalAgentDied indicates the agent loop died unexpectedly
	ErrInternalAgentDied = errors.New("internal error; agent loop died unexpectedly")

	// ErrTurnAborted indicates a turn was aborted
	ErrTurnAborted = errors.New("turn aborted. Something went wrong? Hit `/feedback` to report the issue.")
)

// ConnectionError represents a connection failure
type ConnectionError struct {
	Err     error  // The underlying error
	Context string // Optional context about where the connection failed
}

func (e *ConnectionError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("connection failed: %s: %v", e.Context, e.Err)
	}
	return fmt.Sprintf("connection failed: %v", e.Err)
}

func (e *ConnectionError) Unwrap() error {
	return e.Err
}

// NewConnectionError creates a new ConnectionError
func NewConnectionError(err error) *ConnectionError {
	return &ConnectionError{Err: err}
}

// IsConnectionError checks if an error is or wraps a ConnectionError
func IsConnectionError(err error) bool {
	var ce *ConnectionError
	return errors.As(err, &ce)
}

// SandboxErrorType represents the type of sandbox error
type SandboxErrorType int

const (
	// SandboxDenied indicates the sandbox denied execution
	SandboxDenied SandboxErrorType = iota
	// SandboxTimeout indicates the command timed out
	SandboxTimeout
	// SandboxSignal indicates the command was killed by a signal
	SandboxSignal
)

// SandboxError represents an error from sandbox execution
type SandboxError struct {
	Type     SandboxErrorType
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
	Signal   int
}

func (e *SandboxError) Error() string {
	switch e.Type {
	case SandboxDenied:
		return fmt.Sprintf("sandbox denied exec error, exit code: %d, stdout: %s, stderr: %s",
			e.ExitCode, e.Stdout, e.Stderr)
	case SandboxTimeout:
		return fmt.Sprintf("command timed out after %v", e.Duration)
	case SandboxSignal:
		return fmt.Sprintf("command was killed by signal %d", e.Signal)
	default:
		return "sandbox error"
	}
}

// IsSandboxError checks if an error is or wraps a SandboxError
func IsSandboxError(err error) bool {
	var se *SandboxError
	return errors.As(err, &se)
}

// UnexpectedStatusError represents an unexpected HTTP status code
type UnexpectedStatusError struct {
	Status    int    // HTTP status code
	Body      string // Response body
	RequestID string // Optional request ID for debugging
}

func (e *UnexpectedStatusError) Error() string {
	msg := fmt.Sprintf("unexpected status %d: %s", e.Status, e.Body)
	if e.RequestID != "" {
		msg += fmt.Sprintf(", request id: %s", e.RequestID)
	}
	return msg
}

// ResponseStreamError represents an error while reading a server response stream
type ResponseStreamError struct {
	Err       error  // The underlying error
	RequestID string // Optional request ID for debugging
}

func (e *ResponseStreamError) Error() string {
	msg := fmt.Sprintf("error while reading the server response: %v", e.Err)
	if e.RequestID != "" {
		msg += fmt.Sprintf(", request id: %s", e.RequestID)
	}
	return msg
}

func (e *ResponseStreamError) Unwrap() error {
	return e.Err
}

// RetryLimitError represents exceeding the retry limit
type RetryLimitError struct {
	Status    int    // Last HTTP status code
	RequestID string // Optional request ID for debugging
}

func (e *RetryLimitError) Error() string {
	msg := fmt.Sprintf("exceeded retry limit, last status: %d", e.Status)
	if e.RequestID != "" {
		msg += fmt.Sprintf(", request id: %s", e.RequestID)
	}
	return msg
}

// EnvVarError represents a missing environment variable
type EnvVarError struct {
	VarName      string // Name of the missing variable
	Instructions string // Optional instructions for setting the variable
}

func (e *EnvVarError) Error() string {
	msg := fmt.Sprintf("missing environment variable: `%s`", e.VarName)
	if e.Instructions != "" {
		msg += ". " + e.Instructions
	}
	return msg
}

// NewEnvVarError creates a new EnvVarError
func NewEnvVarError(varName, instructions string) *EnvVarError {
	return &EnvVarError{
		VarName:      varName,
		Instructions: instructions,
	}
}

// UsageLimitError represents hitting a usage limit
type UsageLimitError struct {
	PlanType string    // Type of plan (free, plus, pro, etc.)
	ResetsAt time.Time // When the limit resets
}

func (e *UsageLimitError) Error() string {
	baseMsg := "you've hit your usage limit"

	switch e.PlanType {
	case "free":
		return baseMsg + ". Upgrade to Plus to continue using Codex (https://openai.com/chatgpt/pricing)."
	case "plus":
		suffix := e.formatResetSuffix(" or try again")
		return fmt.Sprintf("%s. Upgrade to Pro (https://openai.com/chatgpt/pricing)%s.", baseMsg, suffix)
	case "team", "business":
		suffix := e.formatResetSuffix(" or try again")
		return fmt.Sprintf("%s. To get more access now, send a request to your admin%s.", baseMsg, suffix)
	case "pro", "enterprise", "edu":
		suffix := e.formatResetSuffix(". Try again")
		return fmt.Sprintf("%s%s.", baseMsg, suffix)
	default:
		suffix := e.formatResetSuffix(". Try again")
		return fmt.Sprintf("%s%s.", baseMsg, suffix)
	}
}

func (e *UsageLimitError) formatResetSuffix(prefix string) string {
	if e.ResetsAt.IsZero() {
		return prefix + " later"
	}

	remaining := time.Until(e.ResetsAt)
	if remaining <= 0 {
		return prefix + " now"
	}

	duration := formatDuration(remaining)
	return prefix + " in " + duration
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "less than a minute"
	}

	totalSecs := int(d.Seconds())
	days := totalSecs / 86400
	hours := (totalSecs % 86400) / 3600
	minutes := (totalSecs % 3600) / 60

	var parts []string
	if days > 0 {
		unit := "day"
		if days > 1 {
			unit = "days"
		}
		parts = append(parts, fmt.Sprintf("%d %s", days, unit))
	}
	if hours > 0 {
		unit := "hour"
		if hours > 1 {
			unit = "hours"
		}
		parts = append(parts, fmt.Sprintf("%d %s", hours, unit))
	}
	if minutes > 0 {
		unit := "minute"
		if minutes > 1 {
			unit = "minutes"
		}
		parts = append(parts, fmt.Sprintf("%d %s", minutes, unit))
	}

	if len(parts) == 0 {
		return "less than a minute"
	}

	result := ""
	for i, part := range parts {
		if i > 0 {
			result += " "
		}
		result += part
	}
	return result
}

// StreamError represents a stream disconnection before completion
type StreamError struct {
	Message    string        // Error message
	RetryDelay time.Duration // Optional delay before retrying
}

func (e *StreamError) Error() string {
	return fmt.Sprintf("stream disconnected before completion: %s", e.Message)
}

// NewStreamError creates a new StreamError
func NewStreamError(message string, retryDelay time.Duration) *StreamError {
	return &StreamError{
		Message:    message,
		RetryDelay: retryDelay,
	}
}

// ConversationNotFoundError represents a conversation that doesn't exist
type ConversationNotFoundError struct {
	ConversationID string
}

func (e *ConversationNotFoundError) Error() string {
	return fmt.Sprintf("no conversation with id: %s", e.ConversationID)
}

// NewConversationNotFoundError creates a new ConversationNotFoundError
func NewConversationNotFoundError(id string) *ConversationNotFoundError {
	return &ConversationNotFoundError{ConversationID: id}
}

// UnsupportedOperationError represents an unsupported operation
type UnsupportedOperationError struct {
	Operation string
}

func (e *UnsupportedOperationError) Error() string {
	return fmt.Sprintf("unsupported operation: %s", e.Operation)
}

// NewUnsupportedOperationError creates a new UnsupportedOperationError
func NewUnsupportedOperationError(operation string) *UnsupportedOperationError {
	return &UnsupportedOperationError{Operation: operation}
}

// FatalError represents a fatal error that should stop execution
type FatalError struct {
	Message string
}

func (e *FatalError) Error() string {
	return fmt.Sprintf("fatal error: %s", e.Message)
}

// NewFatalError creates a new FatalError
func NewFatalError(message string) *FatalError {
	return &FatalError{Message: message}
}

// IsCancelled checks if an error represents a cancellation
func IsCancelled(err error) bool {
	return errors.Is(err, ErrCancelled)
}

// IsTimeout checks if an error represents a timeout
func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout)
}

// IsNotFound checks if an error represents a not found condition
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}
