package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ViewMode represents the current view state of the TUI
type ViewMode int

const (
	ViewModeSessionList ViewMode = iota
	ViewModeConversation
	ViewModeToolApproval
)

// Styles for the TUI
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			MarginBottom(1)

	sessionItemStyle = lipgloss.NewStyle().
				PaddingLeft(2)

	selectedSessionStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("170")).
				Bold(true)

	messageUserStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Bold(true)

	messageAssistantStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("99"))

	messageSystemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				Italic(true)

	toolApprovalStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("214")).
				Padding(1, 2).
				MarginTop(1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Background(lipgloss.Color("235")).
			PaddingLeft(1).
			PaddingRight(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	streamingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")).
			Italic(true)
)

// RenderSessionList renders the session list view
func RenderSessionList(sessions []string, selectedIdx int) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Codex - Session List"))
	b.WriteString("\n\n")

	if len(sessions) == 0 {
		b.WriteString(messageSystemStyle.Render("No sessions yet. Press 'n' to create a new session."))
		b.WriteString("\n")
	} else {
		for i, session := range sessions {
			style := sessionItemStyle
			if i == selectedIdx {
				style = selectedSessionStyle
				b.WriteString(style.Render(fmt.Sprintf("▸ %s", session)))
			} else {
				b.WriteString(style.Render(fmt.Sprintf("  %s", session)))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

// RenderConversation renders the conversation view with messages
func RenderConversation(sessionID string, messages []Message, streamingText string, inputView string) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("Session: %s", sessionID)))
	b.WriteString("\n\n")

	// Render messages
	for _, msg := range messages {
		var style lipgloss.Style
		var prefix string

		switch msg.Role {
		case "user":
			style = messageUserStyle
			prefix = "You: "
		case "assistant":
			style = messageAssistantStyle
			prefix = "Assistant: "
		case "system":
			style = messageSystemStyle
			prefix = "System: "
		default:
			style = messageSystemStyle
			prefix = fmt.Sprintf("%s: ", msg.Role)
		}

		// Sanitize content to prevent ANSI escape sequence injection attacks
		sanitizedContent := SanitizeContent(msg.Content)
		b.WriteString(style.Render(prefix + sanitizedContent))
		b.WriteString("\n\n")
	}

	// Render streaming text if present
	if streamingText != "" {
		// Sanitize streaming text to prevent ANSI escape sequence injection attacks
		sanitizedStreamingText := SanitizeContent(streamingText)
		b.WriteString(streamingStyle.Render("Assistant: " + sanitizedStreamingText))
		b.WriteString(" ▌") // Cursor
		b.WriteString("\n\n")
	}

	// Render input box - use the actual textinput view
	b.WriteString(inputView)
	b.WriteString("\n")

	return b.String()
}

// RenderToolApproval renders the tool approval panel
func RenderToolApproval(toolName string, toolParams map[string]interface{}, riskLevel string) string {
	var b strings.Builder

	b.WriteString("Tool Approval Required\n\n")
	// Sanitize tool name and risk level to prevent injection
	b.WriteString(fmt.Sprintf("Tool: %s\n", SanitizeContent(toolName)))
	b.WriteString(fmt.Sprintf("Risk Level: %s\n\n", SanitizeContent(riskLevel)))
	b.WriteString("Parameters:\n")

	for key, value := range toolParams {
		// Sanitize both key and value to prevent injection
		sanitizedKey := SanitizeContent(key)
		sanitizedValue := SanitizeContent(fmt.Sprintf("%v", value))
		b.WriteString(fmt.Sprintf("  %s: %s\n", sanitizedKey, sanitizedValue))
	}

	b.WriteString("\n")
	b.WriteString("Press 'a' to approve, 'd' to deny")

	return toolApprovalStyle.Render(b.String())
}

// RenderStatusBar renders the status bar at the bottom
func RenderStatusBar(model string, tokens int, mode string, width int) string {
	status := fmt.Sprintf(" Model: %s | Tokens: %d | Mode: %s ", model, tokens, mode)
	return statusBarStyle.Width(width).Render(status)
}

// RenderHelp renders the help text
func RenderHelp() string {
	return helpStyle.Render("Press 'q' to quit, 'n' for new session, 'a' to approve, 'd' to deny")
}

// RenderError renders an error message
func RenderError(err error) string {
	return errorStyle.Render(fmt.Sprintf("Error: %v", err))
}

// Message represents a conversation message
type Message struct {
	Role    string
	Content string
}
