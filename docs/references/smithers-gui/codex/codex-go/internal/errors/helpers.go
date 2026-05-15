package errors

import (
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileErrorType represents the type of file operation error
type FileErrorType int

const (
	// FileErrorNotFound indicates a file was not found
	FileErrorNotFound FileErrorType = iota
	// FileErrorPermission indicates a permission error
	FileErrorPermission
	// FileErrorIsDirectory indicates the path is a directory when a file was expected
	FileErrorIsDirectory
	// FileErrorAlreadyExists indicates the file already exists
	FileErrorAlreadyExists
	// FileErrorInvalidPath indicates an invalid or malformed path
	FileErrorInvalidPath
	// FileErrorBinary indicates the file is binary and cannot be read as text
	FileErrorBinary
	// FileErrorTooLarge indicates the file is too large to process
	FileErrorTooLarge
	// FileErrorReadOnly indicates the file is read-only
	FileErrorReadOnly
)

// FileError represents a file operation error with context
type FileError struct {
	Type      FileErrorType
	Path      string
	Operation string // e.g., "read", "write", "delete"
	Message   string
	Err       error
}

func (e *FileError) Error() string {
	var builder strings.Builder

	// Start with operation context
	if e.Operation != "" {
		builder.WriteString(fmt.Sprintf("failed to %s file", e.Operation))
	} else {
		builder.WriteString("file operation failed")
	}

	// Add path if available
	if e.Path != "" {
		builder.WriteString(fmt.Sprintf(" '%s'", e.Path))
	}

	// Add specific error message
	if e.Message != "" {
		builder.WriteString(": ")
		builder.WriteString(e.Message)
	}

	// Add underlying error if available
	if e.Err != nil {
		builder.WriteString(fmt.Sprintf(" (%v)", e.Err))
	}

	// Add suggestions based on error type
	suggestion := e.getSuggestion()
	if suggestion != "" {
		builder.WriteString(". ")
		builder.WriteString(suggestion)
	}

	return builder.String()
}

func (e *FileError) Unwrap() error {
	return e.Err
}

func (e *FileError) getSuggestion() string {
	switch e.Type {
	case FileErrorNotFound:
		return "Verify the path exists and is spelled correctly"
	case FileErrorPermission:
		return "Check file permissions or try running with appropriate privileges"
	case FileErrorIsDirectory:
		return "Provide a file path, not a directory"
	case FileErrorBinary:
		return "Use a binary file reader or convert to text format"
	case FileErrorTooLarge:
		return "Consider processing the file in chunks or using a streaming reader"
	case FileErrorReadOnly:
		return "The file system or file is read-only"
	default:
		return ""
	}
}

// NewFileError creates a new FileError with automatic type detection
func NewFileError(operation, path string, err error) *FileError {
	fileErr := &FileError{
		Operation: operation,
		Path:      path,
		Err:       err,
	}

	// Try to detect error type from underlying error
	if os.IsNotExist(err) {
		fileErr.Type = FileErrorNotFound
		fileErr.Message = "file does not exist"
	} else if os.IsPermission(err) {
		fileErr.Type = FileErrorPermission
		fileErr.Message = "permission denied"
	} else if os.IsExist(err) {
		fileErr.Type = FileErrorAlreadyExists
		fileErr.Message = "file already exists"
	} else {
		fileErr.Message = "operation failed"
	}

	return fileErr
}

// NewFileNotFoundError creates a specific not found error
func NewFileNotFoundError(path, operation string) *FileError {
	return &FileError{
		Type:      FileErrorNotFound,
		Path:      path,
		Operation: operation,
		Message:   "file does not exist",
	}
}

// NewPermissionError creates a specific permission error
func NewPermissionError(path, operation string) *FileError {
	return &FileError{
		Type:      FileErrorPermission,
		Path:      path,
		Operation: operation,
		Message:   "permission denied",
	}
}

// NewDirectoryError creates an error for when a directory was found instead of a file
func NewDirectoryError(path, operation string) *FileError {
	return &FileError{
		Type:      FileErrorIsDirectory,
		Path:      path,
		Operation: operation,
		Message:   "path is a directory, not a file",
	}
}

// NewBinaryFileError creates an error for binary files
func NewBinaryFileError(path string, size int64) *FileError {
	return &FileError{
		Type:      FileErrorBinary,
		Path:      path,
		Operation: "read",
		Message:   fmt.Sprintf("file appears to be binary (%s)", formatBytes(size)),
	}
}

// ValidationError represents a validation error with context
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
	Err     error
}

func (e *ValidationError) Error() string {
	var builder strings.Builder

	builder.WriteString("validation error")

	if e.Field != "" {
		builder.WriteString(fmt.Sprintf(" for field '%s'", e.Field))
	}

	if e.Value != nil {
		builder.WriteString(fmt.Sprintf(" (value: %v)", e.Value))
	}

	builder.WriteString(": ")
	builder.WriteString(e.Message)

	if e.Err != nil {
		builder.WriteString(fmt.Sprintf(" (%v)", e.Err))
	}

	return builder.String()
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// NewValidationErrorWithValue creates a validation error with the invalid value
func NewValidationErrorWithValue(field string, value interface{}, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// ConfigError represents a configuration error
type ConfigError struct {
	ConfigPath string
	Field      string
	Message    string
	Suggestion string
	Err        error
}

func (e *ConfigError) Error() string {
	var builder strings.Builder

	builder.WriteString("configuration error")

	if e.ConfigPath != "" {
		builder.WriteString(fmt.Sprintf(" in %s", e.ConfigPath))
	}

	if e.Field != "" {
		builder.WriteString(fmt.Sprintf(" [%s]", e.Field))
	}

	builder.WriteString(": ")
	builder.WriteString(e.Message)

	if e.Err != nil {
		builder.WriteString(fmt.Sprintf(" (%v)", e.Err))
	}

	if e.Suggestion != "" {
		builder.WriteString(". Suggestion: ")
		builder.WriteString(e.Suggestion)
	}

	return builder.String()
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// NewConfigError creates a new configuration error
func NewConfigError(field, message string) *ConfigError {
	return &ConfigError{
		Field:   field,
		Message: message,
	}
}

// NewConfigErrorWithSuggestion creates a configuration error with a suggestion
func NewConfigErrorWithSuggestion(field, message, suggestion string) *ConfigError {
	return &ConfigError{
		Field:      field,
		Message:    message,
		Suggestion: suggestion,
	}
}

// PathError represents a path-related error with security context
type PathError struct {
	Path         string
	Workspace    string
	Operation    string
	Reason       string
	SecurityNote string
	Err          error
}

func (e *PathError) Error() string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("path error: %s", e.Reason))

	if e.Path != "" {
		builder.WriteString(fmt.Sprintf(" (path: %s)", e.Path))
	}

	if e.Workspace != "" {
		relPath, err := filepath.Rel(e.Workspace, e.Path)
		if err == nil && !strings.HasPrefix(relPath, "..") {
			builder.WriteString(fmt.Sprintf(" [relative: %s]", relPath))
		}
	}

	if e.Operation != "" {
		builder.WriteString(fmt.Sprintf(" during %s operation", e.Operation))
	}

	if e.SecurityNote != "" {
		builder.WriteString(fmt.Sprintf(". Security: %s", e.SecurityNote))
	}

	if e.Err != nil {
		builder.WriteString(fmt.Sprintf(" (%v)", e.Err))
	}

	return builder.String()
}

func (e *PathError) Unwrap() error {
	return e.Err
}

// NewPathTraversalError creates an error for path traversal attempts
func NewPathTraversalError(path, workspace string) *PathError {
	return &PathError{
		Path:         path,
		Workspace:    workspace,
		Reason:       "path traversal detected",
		SecurityNote: "attempting to access files outside the workspace is not allowed",
	}
}

// NewSensitivePathError creates an error for sensitive path access
func NewSensitivePathError(path, operation string) *PathError {
	return &PathError{
		Path:         path,
		Operation:    operation,
		Reason:       "sensitive system path",
		SecurityNote: "modifying system paths is not allowed for security reasons",
	}
}

// APIError represents an API-related error with retry information
type APIError struct {
	Endpoint      string
	StatusCode    int
	RequestID     string
	Message       string
	RetryAfter    string
	Suggestion    string
	CanRetry      bool
	Err           error
}

func (e *APIError) Error() string {
	var builder strings.Builder

	builder.WriteString("API error")

	if e.Endpoint != "" {
		builder.WriteString(fmt.Sprintf(" [%s]", e.Endpoint))
	}

	if e.StatusCode > 0 {
		builder.WriteString(fmt.Sprintf(" (status: %d)", e.StatusCode))
	}

	builder.WriteString(": ")
	builder.WriteString(e.Message)

	if e.RequestID != "" {
		builder.WriteString(fmt.Sprintf(" [request_id: %s]", e.RequestID))
	}

	if e.CanRetry {
		builder.WriteString(". This request can be retried")
		if e.RetryAfter != "" {
			builder.WriteString(fmt.Sprintf(" after %s", e.RetryAfter))
		}
	}

	if e.Suggestion != "" {
		builder.WriteString(". ")
		builder.WriteString(e.Suggestion)
	}

	if e.Err != nil {
		builder.WriteString(fmt.Sprintf(" (underlying error: %v)", e.Err))
	}

	return builder.String()
}

func (e *APIError) Unwrap() error {
	return e.Err
}

// NewAPIError creates a new API error
func NewAPIError(endpoint string, statusCode int, message string) *APIError {
	return &APIError{
		Endpoint:   endpoint,
		StatusCode: statusCode,
		Message:    message,
	}
}

// NewAPIErrorWithRetry creates an API error that can be retried
func NewAPIErrorWithRetry(endpoint string, statusCode int, message string, retryAfter string) *APIError {
	return &APIError{
		Endpoint:   endpoint,
		StatusCode: statusCode,
		Message:    message,
		CanRetry:   true,
		RetryAfter: retryAfter,
	}
}

// ToolExecutionError represents an error during tool execution
type ToolExecutionError struct {
	ToolName   string
	Args       string
	ExitCode   int
	Stdout     string
	Stderr     string
	Duration   string
	Suggestion string
	Err        error
}

func (e *ToolExecutionError) Error() string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("tool '%s' execution failed", e.ToolName))

	if e.ExitCode != 0 {
		builder.WriteString(fmt.Sprintf(" (exit code: %d)", e.ExitCode))
	}

	if e.Duration != "" {
		builder.WriteString(fmt.Sprintf(" after %s", e.Duration))
	}

	// Include stderr if available and not too long
	if e.Stderr != "" {
		stderr := e.Stderr
		if len(stderr) > 200 {
			stderr = stderr[:200] + "..."
		}
		builder.WriteString(fmt.Sprintf("\nstderr: %s", stderr))
	}

	if e.Suggestion != "" {
		builder.WriteString(fmt.Sprintf("\nSuggestion: %s", e.Suggestion))
	}

	if e.Err != nil {
		builder.WriteString(fmt.Sprintf("\nUnderlying error: %v", e.Err))
	}

	return builder.String()
}

func (e *ToolExecutionError) Unwrap() error {
	return e.Err
}

// NewToolExecutionError creates a new tool execution error
func NewToolExecutionError(toolName string, exitCode int, stderr string) *ToolExecutionError {
	return &ToolExecutionError{
		ToolName: toolName,
		ExitCode: exitCode,
		Stderr:   stderr,
	}
}

// WrapWithContext wraps an error with additional context
func WrapWithContext(err error, context string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}

// WrapWithContextf wraps an error with formatted context
func WrapWithContextf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	context := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s: %w", context, err)
}

// formatBytes formats a byte count into a human-readable string
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// IsRetryable checks if an error indicates a retryable condition
func IsRetryable(err error) bool {
	// Check for specific error types that are retryable
	var apiErr *APIError
	if AsError(err, &apiErr) {
		return apiErr.CanRetry
	}

	var connErr *ConnectionError
	if AsError(err, &connErr) {
		return true // Connection errors are typically retryable
	}

	// Check for specific sentinel errors
	if IsError(err, ErrTimeout) {
		return true
	}

	return false
}

// AsError is a convenience wrapper around errors.As from stdlib
func AsError(err error, target interface{}) bool {
	return stderrors.As(err, target)
}

// IsError is a convenience wrapper around errors.Is from stdlib
func IsError(err, target error) bool {
	return stderrors.Is(err, target)
}
