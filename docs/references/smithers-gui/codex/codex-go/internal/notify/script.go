package notify

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ScriptExecutor handles execution of notification scripts.
type ScriptExecutor struct {
	// Timeout is the maximum time a script can run
	Timeout time.Duration

	// Env contains additional environment variables to pass to scripts
	Env map[string]string
}

// NewScriptExecutor creates a new script executor with the given timeout.
func NewScriptExecutor(timeout time.Duration) *ScriptExecutor {
	return &ScriptExecutor{
		Timeout: timeout,
		Env:     make(map[string]string),
	}
}

// Execute runs a notification script with the given event context.
// The script is executed in the background with a timeout.
// Environment variables are set to provide event context to the script.
func (s *ScriptExecutor) Execute(ctx context.Context, command string, event *NotificationEvent) error {
	if command == "" {
		return fmt.Errorf("command is empty")
	}

	// Parse the command to extract the executable and arguments
	parts := parseCommand(command)
	if len(parts) == 0 {
		return fmt.Errorf("invalid command: %s", command)
	}

	// Create a timeout context
	execCtx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()

	// Create the command
	cmd := exec.CommandContext(execCtx, parts[0], parts[1:]...)

	// Set environment variables with event context
	cmd.Env = s.buildEnv(event)

	// Run the command in the background
	// We don't wait for output or check errors since this is fire-and-forget
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start script: %w", err)
	}

	// Launch a goroutine to wait for the command to complete
	go func() {
		// Wait for the command to complete or timeout
		_ = cmd.Wait()
	}()

	return nil
}

// buildEnv constructs the environment variables for the script execution.
// It includes both the system environment and event-specific variables.
func (s *ScriptExecutor) buildEnv(event *NotificationEvent) []string {
	// Start with the current environment
	env := os.Environ()

	// Add event-specific variables
	eventVars := map[string]string{
		"CODEX_EVENT_TYPE":     string(event.Type),
		"CODEX_SESSION_ID":     event.SessionID,
		"CODEX_TURN_ID":        event.TurnID,
		"CODEX_TIMESTAMP":      event.Timestamp.Format(time.RFC3339),
		"CODEX_STATUS":         event.Status,
		"CODEX_MESSAGE":        event.Message,
		"CODEX_ERROR_MESSAGE":  event.ErrorMessage,
	}

	// Add custom metadata as environment variables
	for key, value := range event.Metadata {
		envKey := "CODEX_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
		eventVars[envKey] = value
	}

	// Add configured environment variables
	for key, value := range s.Env {
		eventVars[key] = value
	}

	// Append to environment
	for key, value := range eventVars {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	return env
}

// parseCommand parses a command string into executable and arguments.
// It handles simple shell quoting but doesn't support complex shell syntax.
func parseCommand(command string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, ch := range command {
		switch {
		case (ch == '"' || ch == '\'') && !inQuote:
			inQuote = true
			quoteChar = ch
		case ch == quoteChar && inQuote:
			inQuote = false
			quoteChar = 0
		case ch == ' ' && !inQuote:
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}

	// Add the last part
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// SetEnv adds an environment variable to be passed to scripts.
func (s *ScriptExecutor) SetEnv(key, value string) {
	s.Env[key] = value
}
