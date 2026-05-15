package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/evmts/codex/codex-go/internal/filesearch"
)

// FileSearchPopup manages the state and rendering of the file search autocomplete popup
type FileSearchPopup struct {
	// Query is the current search query (without the @ symbol)
	query string
	// PendingQuery is the latest query that hasn't been searched yet
	pendingQuery string
	// Waiting indicates if a search is currently in progress
	waiting bool
	// Matches are the current search results
	matches []filesearch.FileMatch
	// SelectedIndex is the currently selected match
	selectedIndex int
	// MaxResults is the maximum number of results to display
	maxResults int
}

// NewFileSearchPopup creates a new file search popup
func NewFileSearchPopup() *FileSearchPopup {
	return &FileSearchPopup{
		query:         "",
		pendingQuery:  "",
		waiting:       false,
		matches:       []filesearch.FileMatch{},
		selectedIndex: 0,
		maxResults:    8, // Match Rust implementation
	}
}

// SetQuery updates the query and marks the popup as waiting for results
func (p *FileSearchPopup) SetQuery(query string) {
	p.pendingQuery = query
	p.waiting = true
	p.selectedIndex = 0
}

// SetMatches updates the matches and clears the waiting state
func (p *FileSearchPopup) SetMatches(query string, matches []filesearch.FileMatch) {
	// Only update if this matches our pending query
	if query == p.pendingQuery {
		p.query = query
		p.matches = matches
		p.waiting = false
		p.selectedIndex = 0

		// Limit to maxResults
		if len(p.matches) > p.maxResults {
			p.matches = p.matches[:p.maxResults]
		}
	}
}

// SetEmptyPrompt sets an empty state with a helpful message
func (p *FileSearchPopup) SetEmptyPrompt() {
	p.query = ""
	p.matches = []filesearch.FileMatch{}
	p.waiting = false
	p.selectedIndex = 0
}

// MoveUp moves the selection up in the list
func (p *FileSearchPopup) MoveUp() {
	if p.selectedIndex > 0 {
		p.selectedIndex--
	}
}

// MoveDown moves the selection down in the list
func (p *FileSearchPopup) MoveDown() {
	if len(p.matches) > 0 && p.selectedIndex < len(p.matches)-1 {
		p.selectedIndex++
	}
}

// SelectedMatch returns the currently selected file match, or empty string if none
func (p *FileSearchPopup) SelectedMatch() string {
	if len(p.matches) > 0 && p.selectedIndex >= 0 && p.selectedIndex < len(p.matches) {
		return p.matches[p.selectedIndex].Path
	}
	return ""
}

// HasMatches returns true if there are any matches to display
func (p *FileSearchPopup) HasMatches() bool {
	return len(p.matches) > 0
}

// IsWaiting returns true if a search is in progress
func (p *FileSearchPopup) IsWaiting() bool {
	return p.waiting
}

// Query returns the current query
func (p *FileSearchPopup) Query() string {
	return p.query
}

// Render renders the popup as a string with styling
func (p *FileSearchPopup) Render() string {
	var b strings.Builder

	// Header with query and status
	if p.waiting {
		b.WriteString(popupHeaderStyle.Render(fmt.Sprintf("Searching: %s…", p.pendingQuery)))
	} else if p.query == "" {
		b.WriteString(popupHeaderStyle.Render("Type a filename to search"))
	} else if len(p.matches) == 0 {
		b.WriteString(popupHeaderStyle.Render(fmt.Sprintf("No matches for: %s", p.query)))
	} else {
		b.WriteString(popupHeaderStyle.Render(fmt.Sprintf("Files matching: %s", p.query)))
	}
	b.WriteString("\n")

	// Render matches
	if len(p.matches) > 0 {
		for i, match := range p.matches {
			var style lipgloss.Style
			var prefix string

			if i == p.selectedIndex {
				style = selectedFileStyle
				prefix = "▸ "
			} else {
				style = fileItemStyle
				prefix = "  "
			}

			// Highlight matched characters if indices are available
			displayPath := match.Path
			if len(match.MatchIndices) > 0 {
				displayPath = p.highlightMatches(match.Path, match.MatchIndices)
			}

			b.WriteString(style.Render(fmt.Sprintf("%s%s", prefix, displayPath)))
			b.WriteString("\n")
		}
	} else if !p.waiting && p.query != "" {
		b.WriteString(popupHintStyle.Render("  No files found"))
		b.WriteString("\n")
	}

	// Footer with hints
	if len(p.matches) > 0 {
		b.WriteString("\n")
		b.WriteString(popupHintStyle.Render("↑↓ navigate • Tab/Enter select • Esc dismiss"))
	} else if p.query == "" {
		b.WriteString("\n")
		b.WriteString(popupHintStyle.Render("Example: @src/main.go"))
	}

	return popupBoxStyle.Render(b.String())
}

// highlightMatches highlights the matched characters in the path
func (p *FileSearchPopup) highlightMatches(path string, indices []int) string {
	if len(indices) == 0 {
		return path
	}

	// Create a map of indices for quick lookup
	matchMap := make(map[int]bool)
	for _, idx := range indices {
		matchMap[idx] = true
	}

	var result strings.Builder
	for i, ch := range path {
		if matchMap[i] {
			// Highlight matched character
			result.WriteString(highlightStyle.Render(string(ch)))
		} else {
			result.WriteString(string(ch))
		}
	}

	return result.String()
}

// Styles for the file search popup
var (
	popupBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Width(60)

	popupHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("99")).
				Bold(true)

	fileItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	selectedFileStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("170")).
				Bold(true)

	popupHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)

	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")).
			Bold(true)
)
