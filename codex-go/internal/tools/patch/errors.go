package patch

import (
	"fmt"
)

// PatchError represents an error that occurred during patch operations.
type PatchError struct {
	Kind    PatchErrorKind
	Message string
	File    string
	Line    int
	Cause   error
}

// Error implements the error interface.
func (e *PatchError) Error() string {
	if e.File != "" && e.Line > 0 {
		return fmt.Sprintf("%s at %s:%d: %s", e.Kind, e.File, e.Line, e.Message)
	}
	if e.File != "" {
		return fmt.Sprintf("%s in %s: %s", e.Kind, e.File, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

// Unwrap returns the underlying cause for error wrapping.
func (e *PatchError) Unwrap() error {
	return e.Cause
}

// PatchErrorKind categorizes different types of patch errors.
type PatchErrorKind string

const (
	// ErrorParse indicates the patch format is invalid.
	ErrorParse PatchErrorKind = "parse error"

	// ErrorConflict indicates the patch cannot be applied due to content mismatch.
	ErrorConflict PatchErrorKind = "conflict"

	// ErrorPathTraversal indicates an attempt to access files outside the root.
	ErrorPathTraversal PatchErrorKind = "path traversal"

	// ErrorFileNotFound indicates a file required for patching doesn't exist.
	ErrorFileNotFound PatchErrorKind = "file not found"

	// ErrorIO indicates an I/O error occurred during file operations.
	ErrorIO PatchErrorKind = "I/O error"

	// ErrorInvalidHunk indicates a hunk is malformed or has invalid line numbers.
	ErrorInvalidHunk PatchErrorKind = "invalid hunk"
)

// NewPatchError creates a new PatchError with the specified kind and message.
func NewPatchError(kind PatchErrorKind, message string) *PatchError {
	return &PatchError{
		Kind:    kind,
		Message: message,
	}
}

// NewPatchErrorWithFile creates a PatchError with file context.
func NewPatchErrorWithFile(kind PatchErrorKind, file, message string) *PatchError {
	return &PatchError{
		Kind:    kind,
		File:    file,
		Message: message,
	}
}

// NewPatchErrorWithCause creates a PatchError wrapping an underlying error.
func NewPatchErrorWithCause(kind PatchErrorKind, message string, cause error) *PatchError {
	return &PatchError{
		Kind:    kind,
		Message: message,
		Cause:   cause,
	}
}

// NewPatchErrorWithFileAndCause creates a PatchError with file context and underlying error.
func NewPatchErrorWithFileAndCause(kind PatchErrorKind, file, message string, cause error) *PatchError {
	return &PatchError{
		Kind:    kind,
		File:    file,
		Message: message,
		Cause:   cause,
	}
}
