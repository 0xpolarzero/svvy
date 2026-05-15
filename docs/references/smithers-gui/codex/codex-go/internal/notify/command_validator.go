package notify

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Allowed script directories (can be configured)
var allowedScriptDirs = []string{
	"~/.codex/scripts",
	"/usr/local/lib/codex/scripts",
}

// validateNotificationCommand validates a notification command for security
func validateNotificationCommand(command string) error {
	if command == "" {
		return fmt.Errorf("command cannot be empty")
	}

	// Check for null bytes
	if strings.Contains(command, "\x00") {
		return fmt.Errorf("command contains null byte")
	}

	// Prevent shell metacharacters in entire command
	dangerousChars := []string{";", "&&", "||", "|", ">", "<", "`", "$", "(", ")"}
	for _, char := range dangerousChars {
		if strings.Contains(command, char) {
			return fmt.Errorf("command contains shell metacharacter: %s", char)
		}
	}

	// Parse command to get executable path
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("command is empty after parsing")
	}

	executable := parts[0]

	// Prevent path traversal
	if strings.Contains(executable, "..") {
		return fmt.Errorf("command contains path traversal")
	}

	// Resolve executable path
	var execPath string
	if filepath.IsAbs(executable) {
		execPath = executable
	} else {
		// Try to find in PATH
		path, err := exec.LookPath(executable)
		if err != nil {
			return fmt.Errorf("command not found in PATH: %s", executable)
		}
		execPath = path
	}

	// Ensure it exists and is executable
	info, err := os.Stat(execPath)
	if err != nil {
		return fmt.Errorf("cannot access command: %w", err)
	}

	// Check if executable
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("command is not executable: %s", execPath)
	}

	// Optionally validate against allowed directories
	// This is commented out as it may be too restrictive
	/*
	allowed := false
	absExecPath, _ := filepath.Abs(execPath)
	for _, allowedDir := range allowedScriptDirs {
		expandedDir := os.ExpandEnv(allowedDir)
		if strings.HasPrefix(absExecPath, expandedDir) {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("command not in allowed script directories: %s", execPath)
	}
	*/

	return nil
}
