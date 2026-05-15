package input

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidationOptions controls file reference validation behavior
type ValidationOptions struct {
	// MaxFileSize is the maximum allowed file size in bytes (0 = no limit)
	MaxFileSize int64
	// AllowedExtensions is a whitelist of file extensions (nil = all allowed)
	AllowedExtensions []string
	// WorkingDirectory is the root directory for relative paths
	WorkingDirectory string
	// AllowAbsolutePaths allows references to files outside working directory
	AllowAbsolutePaths bool
}

// DefaultValidationOptions returns sensible defaults for file validation
func DefaultValidationOptions() ValidationOptions {
	return ValidationOptions{
		MaxFileSize:        10 * 1024 * 1024, // 10 MB
		AllowedExtensions:  nil,               // Allow all extensions
		WorkingDirectory:   "",                // Will use os.Getwd()
		AllowAbsolutePaths: true,              // Allow absolute paths by default
	}
}

// ValidateFileReference performs comprehensive validation on a file reference
func ValidateFileReference(ref FileReference, opts ValidationOptions) error {
	// Check file exists
	info, err := os.Stat(ref.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", ref.DisplayName)
		}
		return fmt.Errorf("cannot access file %s: %w", ref.DisplayName, err)
	}

	// Ensure it's a regular file
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", ref.DisplayName)
	}

	// Check file size
	if opts.MaxFileSize > 0 && info.Size() > opts.MaxFileSize {
		return fmt.Errorf("file %s exceeds maximum size of %d bytes (actual: %d bytes)",
			ref.DisplayName, opts.MaxFileSize, info.Size())
	}

	// Check extension whitelist
	if len(opts.AllowedExtensions) > 0 {
		ext := strings.ToLower(filepath.Ext(ref.Path))
		allowed := false
		for _, allowedExt := range opts.AllowedExtensions {
			if ext == strings.ToLower(allowedExt) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("file extension %s not allowed for file: %s", ext, ref.DisplayName)
		}
	}

	// Check if absolute paths are allowed
	if !opts.AllowAbsolutePaths && filepath.IsAbs(ref.Path) {
		if opts.WorkingDirectory != "" {
			// Check if path is within working directory
			relPath, err := filepath.Rel(opts.WorkingDirectory, ref.Path)
			if err != nil || strings.HasPrefix(relPath, "..") {
				return fmt.Errorf("absolute paths outside working directory not allowed: %s", ref.DisplayName)
			}
		}
	}

	// Check read permissions
	file, err := os.Open(ref.Path)
	if err != nil {
		return fmt.Errorf("cannot open file %s: %w", ref.DisplayName, err)
	}
	file.Close()

	return nil
}

// ValidateAllReferences validates all file references in a parse result
func ValidateAllReferences(result *ParseResult, opts ValidationOptions) []error {
	var errors []error

	for _, ref := range result.FileReferences {
		if err := ValidateFileReference(ref, opts); err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}

// ReadFileContent reads and returns the content of a file reference
func ReadFileContent(ref FileReference) (string, error) {
	content, err := os.ReadFile(ref.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", ref.DisplayName, err)
	}
	return string(content), nil
}

// FormatFileForLLM formats a file reference with its content for LLM consumption
func FormatFileForLLM(ref FileReference, content string) string {
	return fmt.Sprintf("[File: %s]\n%s\n[End File: %s]", ref.DisplayName, content, ref.DisplayName)
}
