package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the keybindings for the TUI
type KeyMap struct {
	// Navigation
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding

	// Actions
	NewSession key.Binding
	Approve    key.Binding
	Deny       key.Binding
	Quit       key.Binding

	// Input
	Submit key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		// Navigation
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),

		// Actions
		NewSession: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new session"),
		),
		Approve: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "approve tool"),
		),
		Deny: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "deny tool"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),

		// Input
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "submit message"),
		),
	}
}

// ShortHelp returns a slice of keybindings to show in the help view
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.NewSession,
		k.Approve,
		k.Deny,
		k.Quit,
	}
}

// FullHelp returns all keybindings grouped by category
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter},           // Navigation
		{k.NewSession, k.Approve, k.Deny}, // Actions
		{k.Submit, k.Quit},                // Input/Control
	}
}
