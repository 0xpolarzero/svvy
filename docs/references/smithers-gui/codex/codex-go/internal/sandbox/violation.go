// Package sandbox provides violation detection for sandbox policy violations.
//
// This module detects and reports sandbox violations by analyzing command output
// and exit codes from sandboxed execution environments.
package sandbox

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ViolationType represents the type of sandbox violation.
type ViolationType string

const (
	// ViolationTypeFileSystem indicates a filesystem access violation (e.g., write to read-only path)
	ViolationTypeFileSystem ViolationType = "filesystem"
	// ViolationTypeNetwork indicates a network access violation
	ViolationTypeNetwork ViolationType = "network"
	// ViolationTypeSyscall indicates a blocked system call
	ViolationTypeSyscall ViolationType = "syscall"
	// ViolationTypeUnknown indicates an unclassified violation
	ViolationTypeUnknown ViolationType = "unknown"
)

// Violation represents a detected sandbox policy violation.
type Violation struct {
	// Type is the category of violation
	Type ViolationType
	// Operation describes what was attempted (e.g., "write", "connect")
	Operation string
	// Path is the filesystem path involved (if applicable)
	Path *string
	// Syscall is the system call that was blocked (if applicable)
	Syscall *string
	// ErrorMessage is the raw error message from the sandbox
	ErrorMessage string
	// ExitCode is the command exit code
	ExitCode int
	// Timestamp when the violation was detected
	Timestamp time.Time
}

// SandboxViolationDetector detects sandbox violations from command execution results.
type SandboxViolationDetector struct {
	// sandboxType identifies the sandbox mechanism (native, docker, kubernetes)
	sandboxType string
}

// NewViolationDetector creates a new violation detector for the given sandbox type.
func NewViolationDetector(sandboxType string) *SandboxViolationDetector {
	return &SandboxViolationDetector{
		sandboxType: sandboxType,
	}
}

// DetectViolation analyzes command execution results for sandbox violations.
// Returns nil if no violation is detected.
//
// Detection strategy (based on Rust implementation):
//  1. Quick reject: exit code 0 or known non-sandbox exit codes (2, 126, 127)
//  2. Keyword scan: Look for sandbox-related error messages in output
//  3. Signal detection: Check for SIGSYS (seccomp), EACCES/EPERM (landlock)
//  4. Parse context: Extract operation, path, or syscall from error messages
func (d *SandboxViolationDetector) DetectViolation(result *Result) *Violation {
	// Skip if command succeeded
	if result.ExitCode == 0 {
		return nil
	}

	// Quick reject: well-known non-sandbox exit codes
	// 2: misuse of shell builtins
	// 126: permission denied (general, not sandbox-specific)
	// 127: command not found
	quickRejectExitCodes := []int{2, 126, 127}
	for _, code := range quickRejectExitCodes {
		if result.ExitCode == code && !d.hasKeywordInOutput(result) {
			return nil
		}
	}

	// Combine output for analysis
	combinedOutput := result.Stderr + "\n" + result.Stdout

	// Check for sandbox-specific violations
	if violation := d.detectByKeywords(combinedOutput, result.ExitCode); violation != nil {
		return violation
	}

	// Check for signal-based violations (seccomp SIGSYS = 128 + 31 = 159)
	if result.ExitCode == 159 {
		return &Violation{
			Type:         ViolationTypeSyscall,
			Operation:    "syscall_blocked",
			ErrorMessage: "Process terminated by SIGSYS (seccomp violation)",
			ExitCode:     result.ExitCode,
			Timestamp:    time.Now(),
		}
	}

	return nil
}

// hasKeywordInOutput checks if any sandbox keywords appear in the output.
func (d *SandboxViolationDetector) hasKeywordInOutput(result *Result) bool {
	keywords := []string{
		"operation not permitted",
		"permission denied",
		"read-only file system",
		"seccomp",
		"sandbox",
		"landlock",
		"failed to write file",
		"seatbelt",
		"eacces",
		"eperm",
	}

	combinedOutput := strings.ToLower(result.Stderr + " " + result.Stdout)
	for _, keyword := range keywords {
		if strings.Contains(combinedOutput, keyword) {
			return true
		}
	}
	return false
}

// detectByKeywords analyzes output for sandbox keywords and extracts violation details.
func (d *SandboxViolationDetector) detectByKeywords(output string, exitCode int) *Violation {
	lowerOutput := strings.ToLower(output)

	// Seatbelt (macOS) violations
	if strings.Contains(lowerOutput, "seatbelt") || strings.Contains(lowerOutput, "sandbox-exec") {
		return d.parseSeatbeltViolation(output, exitCode)
	}

	// Landlock violations (typically EACCES/EPERM)
	if strings.Contains(lowerOutput, "landlock") ||
		strings.Contains(lowerOutput, "eacces") ||
		strings.Contains(lowerOutput, "eperm") {
		return d.parseLandlockViolation(output, exitCode)
	}

	// Seccomp violations
	if strings.Contains(lowerOutput, "seccomp") {
		return d.parseSeccompViolation(output, exitCode)
	}

	// Generic filesystem violations
	if strings.Contains(lowerOutput, "read-only file system") ||
		strings.Contains(lowerOutput, "failed to write file") {
		return d.parseFilesystemViolation(output, exitCode)
	}

	// Generic permission violations
	if strings.Contains(lowerOutput, "operation not permitted") ||
		strings.Contains(lowerOutput, "permission denied") {
		return d.parseGenericViolation(output, exitCode)
	}

	return nil
}

// parseSeatbeltViolation extracts details from macOS Seatbelt error messages.
func (d *SandboxViolationDetector) parseSeatbeltViolation(output string, exitCode int) *Violation {
	violation := &Violation{
		Type:         ViolationTypeFileSystem,
		Operation:    "seatbelt_denied",
		ErrorMessage: extractErrorLine(output),
		ExitCode:     exitCode,
		Timestamp:    time.Now(),
	}

	// Try to extract path from common patterns
	// Example: "sandbox-exec: /path/to/file: Operation not permitted"
	if path := extractPath(output); path != "" {
		violation.Path = &path
	}

	// Determine operation type
	if strings.Contains(strings.ToLower(output), "write") ||
		strings.Contains(strings.ToLower(output), "create") {
		violation.Operation = "write"
	} else if strings.Contains(strings.ToLower(output), "read") {
		violation.Operation = "read"
	}

	return violation
}

// parseLandlockViolation extracts details from Landlock error messages.
func (d *SandboxViolationDetector) parseLandlockViolation(output string, exitCode int) *Violation {
	violation := &Violation{
		Type:         ViolationTypeFileSystem,
		Operation:    "landlock_denied",
		ErrorMessage: extractErrorLine(output),
		ExitCode:     exitCode,
		Timestamp:    time.Now(),
	}

	// Extract path from error messages
	if path := extractPath(output); path != "" {
		violation.Path = &path
	}

	// Determine operation from error codes
	if strings.Contains(strings.ToLower(output), "eacces") {
		violation.Operation = "access_denied"
	} else if strings.Contains(strings.ToLower(output), "eperm") {
		violation.Operation = "operation_not_permitted"
	}

	return violation
}

// parseSeccompViolation extracts details from seccomp error messages.
func (d *SandboxViolationDetector) parseSeccompViolation(output string, exitCode int) *Violation {
	violation := &Violation{
		Type:         ViolationTypeSyscall,
		Operation:    "syscall_blocked",
		ErrorMessage: extractErrorLine(output),
		ExitCode:     exitCode,
		Timestamp:    time.Now(),
	}

	// Try to extract syscall name
	if syscall := extractSyscall(output); syscall != "" {
		violation.Syscall = &syscall
	}

	return violation
}

// parseFilesystemViolation extracts details from generic filesystem errors.
func (d *SandboxViolationDetector) parseFilesystemViolation(output string, exitCode int) *Violation {
	violation := &Violation{
		Type:         ViolationTypeFileSystem,
		Operation:    "write",
		ErrorMessage: extractErrorLine(output),
		ExitCode:     exitCode,
		Timestamp:    time.Now(),
	}

	if path := extractPath(output); path != "" {
		violation.Path = &path
	}

	return violation
}

// parseGenericViolation handles generic permission errors.
func (d *SandboxViolationDetector) parseGenericViolation(output string, exitCode int) *Violation {
	violation := &Violation{
		Type:         ViolationTypeUnknown,
		Operation:    "permission_denied",
		ErrorMessage: extractErrorLine(output),
		ExitCode:     exitCode,
		Timestamp:    time.Now(),
	}

	if path := extractPath(output); path != "" {
		violation.Path = &path
		violation.Type = ViolationTypeFileSystem
	}

	return violation
}

// extractErrorLine extracts the first relevant error line from output.
func extractErrorLine(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && (strings.Contains(strings.ToLower(trimmed), "error") ||
			strings.Contains(strings.ToLower(trimmed), "denied") ||
			strings.Contains(strings.ToLower(trimmed), "permission") ||
			strings.Contains(strings.ToLower(trimmed), "sandbox")) {
			return trimmed
		}
	}
	// If no specific error line found, return first non-empty line
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return output
}

// extractPath attempts to extract a file path from error output.
func extractPath(output string) string {
	// Common patterns for path extraction
	patterns := []string{
		// "permission denied: /path/to/file"
		`(?:permission denied|operation not permitted|cannot access|failed to write)[:\s]+([/~][^\s:'"]+)`,
		// "/path/to/file: permission denied"
		`([/~][^\s:'"]+):\s*(?:permission denied|operation not permitted|cannot access)`,
		// "open /path/to/file: permission denied"
		`open\s+([/~][^\s:'"]+)`,
		// Generic path at start of line or after common prefixes
		`(?:^|\s)([/~][^\s:'"]{3,})`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(output); len(matches) > 1 {
			path := strings.TrimSpace(matches[1])
			// Validate path looks reasonable
			if len(path) > 2 && (strings.HasPrefix(path, "/") || strings.HasPrefix(path, "~")) {
				return path
			}
		}
	}

	return ""
}

// extractSyscall attempts to extract syscall name from error output.
func extractSyscall(output string) string {
	// Common patterns for syscall extraction
	patterns := []string{
		// "syscall: connect"
		`syscall:\s*(\w+)`,
		// "blocked syscall: connect"
		`blocked\s+syscall:\s*(\w+)`,
		// "seccomp: connect not allowed"
		`seccomp:\s*(\w+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(output); len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

// FormatViolation creates a human-readable description of the violation.
func (v *Violation) FormatViolation() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Sandbox violation detected: %s", v.Type))
	parts = append(parts, fmt.Sprintf("Operation: %s", v.Operation))

	if v.Path != nil {
		parts = append(parts, fmt.Sprintf("Path: %s", *v.Path))
	}
	if v.Syscall != nil {
		parts = append(parts, fmt.Sprintf("Syscall: %s", *v.Syscall))
	}

	parts = append(parts, fmt.Sprintf("Exit code: %d", v.ExitCode))
	parts = append(parts, fmt.Sprintf("Error: %s", v.ErrorMessage))

	return strings.Join(parts, "\n")
}

// ToProtocolEvent converts a Violation to a protocol EventSandboxViolation.
// This is a helper for emitting violation events through the protocol layer.
// The callID must be provided by the caller (typically from the tool execution context).
func (v *Violation) ToProtocolEvent(callID, sandboxType string) interface{} {
	// Import path would be: "github.com/evmts/codex/codex-go/internal/protocol"
	// But we return interface{} to avoid circular imports
	return struct {
		CallID       string  `json:"call_id"`
		SandboxType  string  `json:"sandbox_type"`
		Operation    string  `json:"operation"`
		Path         *string `json:"path,omitempty"`
		Syscall      *string `json:"syscall,omitempty"`
		ErrorMessage string  `json:"error_message"`
		ExitCode     int     `json:"exit_code"`
		Timestamp    string  `json:"timestamp"`
	}{
		CallID:       callID,
		SandboxType:  sandboxType,
		Operation:    v.Operation,
		Path:         v.Path,
		Syscall:      v.Syscall,
		ErrorMessage: v.ErrorMessage,
		ExitCode:     v.ExitCode,
		Timestamp:    v.Timestamp.Format(time.RFC3339),
	}
}
