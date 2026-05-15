package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/evmts/codex/codex-go/internal/conversation/manager"
	"github.com/evmts/codex/codex-go/internal/filesearch"
	"github.com/evmts/codex/codex-go/internal/input"
	"github.com/evmts/codex/codex-go/internal/protocol"
)

// Model represents the TUI application state
type Model struct {
	// Core state
	viewMode        ViewMode
	conversationMgr manager.ConversationManager
	keys            KeyMap

	// Session management
	sessions       []string
	selectedIdx    int
	currentSession *manager.Session

	// Conversation state
	messages      []Message
	streamingText string
	inputText     textinput.Model

	// Tool approval state
	pendingTool      *PendingToolApproval
	toolApprovalChan chan bool

	// File search state
	fileSearchPopup     *FileSearchPopup
	fileSearchManager   *filesearch.SearchManager
	fileSearchResultCh  chan filesearch.SearchResultMsg
	activeAtToken       string
	dismissedAtToken    string
	workingDir          string

	// Status
	model       string
	totalTokens int
	err         error

	// UI state
	width  int
	height int
	ready  bool

	// Event handling
	eventChan chan *protocol.Event
}

// PendingToolApproval represents a tool waiting for approval
type PendingToolApproval struct {
	ToolName   string
	Parameters map[string]interface{}
	RiskLevel  string
}

// NewModel creates a new TUI model
func NewModel(mgr manager.ConversationManager) Model {
	ti := textinput.New()
	ti.Placeholder = "Type your message..."
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 80

	// Get working directory for file search
	workingDir, _ := os.Getwd()

	// Initialize file search components
	fileSearchResultCh := make(chan filesearch.SearchResultMsg, 10)

	// Create searcher with default options
	searcher, _ := filesearch.NewSearcher(workingDir, filesearch.DefaultSearchOptions())

	// Create search manager
	searchManager := filesearch.NewSearchManager(searcher, fileSearchResultCh)

	return Model{
		viewMode:            ViewModeSessionList,
		conversationMgr:     mgr,
		keys:                DefaultKeyMap(),
		sessions:            []string{},
		selectedIdx:         0,
		messages:            []Message{},
		inputText:           ti,
		model:               "claude-3-5-sonnet-20241022",
		totalTokens:         0,
		width:               80,
		height:              24,
		toolApprovalChan:    make(chan bool, 1),
		eventChan:           make(chan *protocol.Event, 100),
		fileSearchPopup:     nil, // Created on demand
		fileSearchManager:   searchManager,
		fileSearchResultCh:  fileSearchResultCh,
		workingDir:          workingDir,
		activeAtToken:       "",
		dismissedAtToken:    "",
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.loadSessions(), m.waitForEvent(), m.waitForFileSearchResult())
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case sessionsLoadedMsg:
		m.sessions = msg.sessions
		return m, nil

	case sessionCreatedMsg:
		m.sessions = append(m.sessions, msg.sessionID)
		m.selectedIdx = len(m.sessions) - 1
		return m, nil

	// Protocol event handlers
	case taskStartedMsg:
		// Clear streaming text for new response
		m.streamingText = ""
		return m, m.waitForEvent() // Continue polling

	case textDeltaMsg:
		// Sanitize and append streaming text delta to prevent ANSI injection
		sanitizedDelta := SanitizeContent(msg.delta)
		m.streamingText += sanitizedDelta
		return m, m.waitForEvent() // Continue polling

	case reasoningDeltaMsg:
		// Could display reasoning in a separate area
		// For now, just continue polling
		return m, m.waitForEvent()

	case tokenCountMsg:
		// Update token display
		m.totalTokens = msg.totalTokens
		return m, m.waitForEvent() // Continue polling

	case commandBeginMsg:
		// Show tool execution started
		// Could add to a command log
		return m, m.waitForEvent()

	case commandOutputMsg:
		// Show command output
		// Could append to command log
		return m, m.waitForEvent()

	case commandEndMsg:
		// Show command completed
		// Could display exit code and output
		return m, m.waitForEvent()

	case taskCompleteMsg:
		// Task complete - finalize the message
		finalText := m.streamingText
		if finalText == "" && msg.finalMessage != "" {
			finalText = SanitizeContent(msg.finalMessage)
		}
		if finalText != "" {
			m.messages = append(m.messages, Message{
				Role:    "assistant",
				Content: finalText,
			})
		}
		m.streamingText = ""
		// Re-enable input
		m.inputText.Focus()
		return m, m.waitForEvent() // Continue polling for next events

	case errorEventMsg:
		// Display error
		m.err = fmt.Errorf("%s", msg.errorMsg)
		m.streamingText = ""
		m.inputText.Focus() // Re-enable input on error
		return m, m.waitForEvent()

	case fileSearchResultMsg:
		// Update file search popup with results
		if m.fileSearchPopup != nil {
			m.fileSearchPopup.SetMatches(msg.query, msg.matches)
		}
		// Continue polling for more results
		return m, m.waitForFileSearchResult()

	case tea.KeyMsg:
		// Handle file search popup keys first
		if m.fileSearchPopup != nil && m.viewMode == ViewModeConversation {
			switch msg.String() {
			case "up":
				m.fileSearchPopup.MoveUp()
				return m, nil

			case "down":
				m.fileSearchPopup.MoveDown()
				return m, nil

			case "tab":
				// Select file from popup
				selectedPath := m.fileSearchPopup.SelectedMatch()
				if selectedPath != "" {
					m.insertFileReference(selectedPath)
					m.fileSearchPopup = nil
					m.activeAtToken = ""
					m.dismissedAtToken = ""
				}
				return m, nil

			case "enter":
				// Check if we have a selection in popup
				if m.fileSearchPopup.HasMatches() {
					selectedPath := m.fileSearchPopup.SelectedMatch()
					if selectedPath != "" {
						m.insertFileReference(selectedPath)
						m.fileSearchPopup = nil
						m.activeAtToken = ""
						m.dismissedAtToken = ""
						return m, nil
					}
				}
				// If no selection, fall through to normal enter handling

			case "esc":
				// Dismiss popup and remember this token
				m.dismissedAtToken = m.activeAtToken
				m.fileSearchPopup = nil
				m.activeAtToken = ""
				return m, nil
			}
		}

		// Normal key handling
		switch msg.String() {
		case "ctrl+c", "q":
			if m.viewMode == ViewModeSessionList || m.pendingTool == nil {
				return m, tea.Quit
			}

		case "n":
			if m.viewMode == ViewModeSessionList {
				return m, m.createNewSession()
			}

		case "enter":
			if m.viewMode == ViewModeSessionList {
				// Select session
				if len(m.sessions) > 0 && m.selectedIdx < len(m.sessions) {
					m.currentSession = m.getSession(m.sessions[m.selectedIdx])
					m.viewMode = ViewModeConversation
					m.loadMessages()
				}
			} else if m.viewMode == ViewModeConversation && m.pendingTool == nil {
				// Submit message
				if m.inputText.Value() != "" {
					return m, m.submitMessage()
				}
			}

		case "a":
			if m.pendingTool != nil {
				m.approveTool()
				m.pendingTool = nil
				m.viewMode = ViewModeConversation
			}

		case "d":
			if m.pendingTool != nil {
				m.denyTool()
				m.pendingTool = nil
				m.viewMode = ViewModeConversation
			}

		case "up", "k":
			if m.viewMode == ViewModeSessionList && m.selectedIdx > 0 {
				m.selectedIdx--
			}

		case "down", "j":
			if m.viewMode == ViewModeSessionList && m.selectedIdx < len(m.sessions)-1 {
				m.selectedIdx++
			}
		}

	case streamingMsg:
		// Sanitize legacy streaming text to prevent ANSI injection
		m.streamingText += SanitizeContent(msg.text)
		return m, waitForStreaming(msg.done)

	case streamingDoneMsg:
		m.messages = append(m.messages, Message{
			Role:    "assistant",
			Content: m.streamingText,
		})
		m.streamingText = ""
		return m, nil

	case toolApprovalMsg:
		m.pendingTool = &PendingToolApproval{
			ToolName:   msg.toolName,
			Parameters: msg.params,
			RiskLevel:  msg.riskLevel,
		}
		m.viewMode = ViewModeToolApproval
		return m, nil

	case errorMsg:
		m.err = msg.err
		return m, nil
	}

	// Update text input if in conversation mode
	if m.viewMode == ViewModeConversation && m.pendingTool == nil {
		oldValue := m.inputText.Value()
		m.inputText, cmd = m.inputText.Update(msg)
		newValue := m.inputText.Value()

		// Check if input changed and sync file search popup
		if oldValue != newValue {
			m.syncFileSearchPopup()
		}

		return m, cmd
	}

	return m, nil
}

// View renders the current view
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var content string

	switch m.viewMode {
	case ViewModeSessionList:
		content = RenderSessionList(m.sessions, m.selectedIdx)

	case ViewModeConversation:
		content = RenderConversation(
			m.getCurrentSessionID(),
			m.messages,
			m.streamingText,
			m.inputText.View(),
		)

	case ViewModeToolApproval:
		conversationView := RenderConversation(
			m.getCurrentSessionID(),
			m.messages,
			m.streamingText,
			m.inputText.View(),
		)
		toolPanel := RenderToolApproval(
			m.pendingTool.ToolName,
			m.pendingTool.Parameters,
			m.pendingTool.RiskLevel,
		)
		content = conversationView + "\n" + toolPanel
	}

	// Add error if present
	if m.err != nil {
		content += "\n\n" + RenderError(m.err)
	}

	// Overlay file search popup if active
	if m.fileSearchPopup != nil && m.viewMode == ViewModeConversation {
		content += "\n\n" + m.fileSearchPopup.Render()
	}

	// Add status bar
	var modeStr string
	switch m.viewMode {
	case ViewModeSessionList:
		modeStr = "session-list"
	case ViewModeConversation:
		modeStr = "conversation"
	case ViewModeToolApproval:
		modeStr = "tool-approval"
	default:
		modeStr = "unknown"
	}
	statusBar := RenderStatusBar(m.model, m.totalTokens, modeStr, m.width)

	// Add help
	help := RenderHelp()

	return content + "\n" + statusBar + "\n" + help
}

// Helper methods

func (m *Model) createNewSession() tea.Cmd {
	return func() tea.Msg {
		// Generate a new session ID
		sessionID := fmt.Sprintf("session-%d", len(m.sessions)+1)

		// Create event handler that sends events to the TUI's event channel
		eventHandler := func(ctx context.Context, event *protocol.Event) error {
			// Non-blocking send to avoid deadlocks
			select {
			case m.eventChan <- event:
			default:
				// Channel full, skip event (shouldn't happen with large buffer)
			}
			return nil
		}

		// Get absolute path for current directory
		// Tools require absolute paths for working directory
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "." // Fallback to relative path
		}

		// Create session
		ctx := context.Background()
		_, err = m.conversationMgr.CreateSession(ctx, manager.SessionConfig{
			ID:     sessionID,
			Client: nil, // Uses manager's default client
			TurnContext: &manager.TurnContext{
				Cwd:            cwd,
				ApprovalPolicy: "auto",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
				Model:          m.model,
			},
			EventHandlers: []manager.EventHandler{eventHandler},
		})

		if err != nil {
			return errorMsg{err: err}
		}

		return sessionCreatedMsg{sessionID: sessionID}
	}
}

func (m *Model) getSession(sessionID string) *manager.Session {
	session, err := m.conversationMgr.GetSession(sessionID)
	if err != nil {
		m.err = err
		return nil
	}
	return session
}

func (m *Model) loadMessages() {
	// In a real implementation, load message history
	// For now, start with empty messages
	m.messages = []Message{}
}

func (m *Model) getCurrentSessionID() string {
	if len(m.sessions) > 0 && m.selectedIdx < len(m.sessions) {
		return m.sessions[m.selectedIdx]
	}
	return "no session"
}

func (m *Model) submitMessage() tea.Cmd {
	userInput := m.inputText.Value()
	m.messages = append(m.messages, Message{
		Role:    "user",
		Content: userInput,
	})
	m.inputText.SetValue("")
	m.streamingText = ""

	// Blur input while processing (will be re-focused on completion)
	m.inputText.Blur()

	return func() tea.Msg {
		// Parse @ file references from user input
		ctx := context.Background()
		workingDir, _ := os.Getwd()
		parseResult, err := input.ParseFileReferences(userInput, workingDir)
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to parse file references: %w", err)}
		}

		// Build UserInput items
		items := []protocol.UserInput{}

		// Add processed text (with @ references replaced by placeholders)
		if parseResult.ProcessedText != "" {
			textPtr := &parseResult.ProcessedText
			items = append(items, protocol.UserInput{
				Type: protocol.UserInputTypeText,
				Text: textPtr,
			})
		}

		// Add file references
		for _, ref := range parseResult.FileReferences {
			pathPtr := &ref.Path
			items = append(items, protocol.UserInput{
				Type: protocol.UserInputTypePath,
				Path: pathPtr,
			})
		}

		// If no items were created (empty input), add the original text
		if len(items) == 0 {
			textPtr := &userInput
			items = append(items, protocol.UserInput{
				Type: protocol.UserInputTypeText,
				Text: textPtr,
			})
		}

		// Submit to conversation manager
		op := &protocol.OpUserInput{
			Items: items,
		}

		err = m.conversationMgr.SubmitOp(ctx, m.getCurrentSessionID(), op)
		if err != nil {
			return errorMsg{err: err}
		}

		// Events will be received through the event channel
		// and processed by waitForEvent() polling loop
		return nil
	}
}

func (m *Model) approveTool() {
	select {
	case m.toolApprovalChan <- true:
	default:
	}
}

func (m *Model) denyTool() {
	select {
	case m.toolApprovalChan <- false:
	default:
	}
}

// syncFileSearchPopup detects @ tokens and updates the file search popup
func (m *Model) syncFileSearchPopup() {
	inputValue := m.inputText.Value()
	cursorPos := m.inputText.Position()

	// Get current @ token at cursor
	token, _, hasAt := input.GetCurrentToken(inputValue, cursorPos)

	if !hasAt || token == "" {
		// No @ token, close popup if open
		if m.fileSearchPopup != nil {
			m.fileSearchPopup = nil
			m.activeAtToken = ""
		}
		return
	}

	// Check if this is the dismissed token
	if token == m.dismissedAtToken {
		return
	}

	// Create popup if it doesn't exist
	if m.fileSearchPopup == nil {
		m.fileSearchPopup = NewFileSearchPopup()
	}

	// If token changed, trigger search
	if token != m.activeAtToken {
		m.activeAtToken = token
		m.fileSearchPopup.SetQuery(token)

		// Trigger debounced search
		if m.fileSearchManager != nil {
			m.fileSearchManager.OnUserQuery(token)
		}
	}
}

// insertFileReference inserts a file path at the current @ token position
func (m *Model) insertFileReference(path string) {
	inputValue := m.inputText.Value()
	cursorPos := m.inputText.Position()

	// Get current @ token at cursor
	token, atPos, hasAt := input.GetCurrentToken(inputValue, cursorPos)
	if !hasAt {
		return
	}

	// Calculate the end position of the @ token
	tokenEnd := atPos + 1 + len(token) // @ + token length

	// Quote path if it contains spaces
	displayPath := path
	if strings.Contains(path, " ") {
		displayPath = `"` + path + `"`
	}

	// Build new input value
	newValue := inputValue[:atPos] + "@" + displayPath + inputValue[tokenEnd:]

	// Update input
	m.inputText.SetValue(newValue)

	// Set cursor after the inserted path
	newCursorPos := atPos + 1 + len(displayPath)
	m.inputText.SetCursor(newCursorPos)
}

// Message types for tea.Msg

type sessionsLoadedMsg struct {
	sessions []string
}

type sessionCreatedMsg struct {
	sessionID string
}

// Protocol event messages
type taskStartedMsg struct {
	contextWindow int64
}

type textDeltaMsg struct {
	delta        string
	submissionID string
}

type reasoningDeltaMsg struct {
	delta string
}

type tokenCountMsg struct {
	inputTokens  int
	outputTokens int
	totalTokens  int
}

type commandBeginMsg struct {
	callID   string
	toolName string
	command  []string
}

type commandOutputMsg struct {
	callID string
	output string
	stream string // "stdout" or "stderr"
}

type commandEndMsg struct {
	callID   string
	exitCode int
	output   string
}

type taskCompleteMsg struct {
	finalMessage string
}

type errorEventMsg struct {
	errorMsg string
}

// Legacy streaming messages (will be replaced by protocol events)
type streamingMsg struct {
	text string
	done chan bool
}

type streamingDoneMsg struct{}

type toolApprovalMsg struct {
	toolName  string
	params    map[string]interface{}
	riskLevel string
}

type errorMsg struct {
	err error
}

type fileSearchResultMsg struct {
	query   string
	matches []filesearch.FileMatch
	err     error
}

// Commands

func (m *Model) loadSessions() tea.Cmd {
	return func() tea.Msg {
		sessions := m.conversationMgr.ListSessions()
		return sessionsLoadedMsg{sessions: sessions}
	}
}

// waitForEvent polls the event channel and converts protocol events to tea messages
func (m *Model) waitForEvent() tea.Cmd {
	return func() tea.Msg {
		event, ok := <-m.eventChan
		if !ok {
			// Channel closed
			return nil
		}
		return convertEventToMsg(event)
	}
}

// waitForFileSearchResult polls the file search result channel
func (m *Model) waitForFileSearchResult() tea.Cmd {
	return func() tea.Msg {
		select {
		case result := <-m.fileSearchResultCh:
			return fileSearchResultMsg{
				query:   result.Query,
				matches: result.Matches,
				err:     result.Error,
			}
		default:
			return nil
		}
	}
}

// convertEventToMsg converts a protocol.Event to a bubbletea message
func convertEventToMsg(event *protocol.Event) tea.Msg {
	if event == nil {
		return nil
	}

	switch msg := event.Msg.(type) {
	case *protocol.EventTaskStarted:
		var contextWindow int64
		if msg.ModelContextWindow != nil {
			contextWindow = *msg.ModelContextWindow
		}
		return taskStartedMsg{contextWindow: contextWindow}

	case *protocol.EventAgentMessageDelta:
		return textDeltaMsg{
			delta:        msg.Delta,
			submissionID: event.ID,
		}

	case *protocol.EventAgentReasoningDelta:
		return reasoningDeltaMsg{delta: msg.Delta}

	case *protocol.EventTokenCount:
		if msg.Info != nil {
			return tokenCountMsg{
				inputTokens:  int(msg.Info.TotalTokenUsage.InputTokens),
				outputTokens: int(msg.Info.TotalTokenUsage.OutputTokens),
				totalTokens:  int(msg.Info.TotalTokenUsage.TotalTokens),
			}
		}

	case *protocol.EventExecCommandBegin:
		// Extract tool name from command if available
		toolName := "command"
		if len(msg.Command) > 0 {
			toolName = msg.Command[0]
		}
		return commandBeginMsg{
			callID:   msg.CallID,
			toolName: toolName,
			command:  msg.Command,
		}

	case *protocol.EventExecCommandOutputDelta:
		return commandOutputMsg{
			callID: msg.CallID,
			output: string(msg.Chunk),
			stream: msg.Stream,
		}

	case *protocol.EventExecCommandEnd:
		return commandEndMsg{
			callID:   msg.CallID,
			exitCode: msg.ExitCode,
			output:   msg.AggregatedOutput,
		}

	case *protocol.EventToolCallApprovalNeeded:
		// Convert to existing toolApprovalMsg
		params := make(map[string]interface{})
		params["command"] = msg.Command
		params["working_directory"] = msg.WorkingDirectory
		return toolApprovalMsg{
			toolName:  msg.ToolName,
			params:    params,
			riskLevel: msg.RiskLevel,
		}

	case *protocol.EventTaskComplete:
		finalMsg := ""
		if msg.LastAgentMessage != nil {
			finalMsg = *msg.LastAgentMessage
		}
		return taskCompleteMsg{finalMessage: finalMsg}

	case *protocol.EventError:
		return errorEventMsg{errorMsg: msg.Message}
	}

	return nil
}

func waitForStreaming(done chan bool) tea.Cmd {
	return func() tea.Msg {
		<-done
		return streamingDoneMsg{}
	}
}

func simulateStreaming(text string) tea.Cmd {
	return func() tea.Msg {
		done := make(chan bool)

		// Split text into chunks for streaming effect
		chunks := strings.Split(text, " ")

		go func() {
			for range chunks {
				// In real impl, this would come from the streaming API
			}
			close(done)
		}()

		return streamingMsg{
			text: text,
			done: done,
		}
	}
}

// Run starts the TUI application
func Run(mgr manager.ConversationManager) error {
	p := tea.NewProgram(NewModel(mgr), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
