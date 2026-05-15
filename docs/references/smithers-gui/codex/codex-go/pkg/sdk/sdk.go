// Package sdk provides a high-level, ergonomic API for using Codex programmatically.
// It wraps internal packages (client, conversation manager, tools orchestrator) and provides
// a simple interface for creating AI-powered coding sessions with streaming, tool execution,
// and conversation persistence.
package sdk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/evmts/codex/codex-go/internal/conversation/manager"
	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/evmts/codex/codex-go/internal/tools/file"
	"github.com/evmts/codex/codex-go/internal/tools/orchestrator"
	"github.com/evmts/codex/codex-go/internal/tools/patch"
	"github.com/evmts/codex/codex-go/internal/tools/runtime"
	"github.com/evmts/codex/codex-go/internal/tools/shell"
	"github.com/evmts/codex/codex-go/pkg/sdk/client"
	"github.com/spf13/afero"
)

// SDK is the main entry point for the Codex SDK.
// It manages conversation sessions and provides access to the AI model.
type SDK struct {
	client        *client.Client
	manager       manager.ConversationManager
	toolRegistry  *runtime.ToolRegistry
	orchestrator  *orchestrator.Orchestrator
	approvalCache runtime.ApprovalCache
	enableHistory bool
	historyPath   string
	mu            sync.RWMutex
	sessions      map[string]*Session
}

// New creates a new Codex SDK instance with the given options.
// The client is required; other options are optional with sensible defaults.
func New(opts Options) (*SDK, error) {
	// Validate options
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	// Create or use provided tool registry
	var toolRegistry *runtime.ToolRegistry
	if opts.ToolRegistry != nil {
		toolRegistry = opts.ToolRegistry
	} else {
		toolRegistry = runtime.NewToolRegistry()

		// Register tools
		if len(opts.Tools) > 0 {
			// Use provided tools
			for _, tool := range opts.Tools {
				toolRegistry.Register(tool)
			}
		} else {
			// Register default tools
			fs := afero.NewOsFs()
			toolRegistry.Register(patch.NewPatchTool(fs))
			toolRegistry.Register(file.NewReadTool(fs))
			toolRegistry.Register(file.NewWriteTool(fs))
			toolRegistry.Register(file.NewListTool(fs))
			toolRegistry.Register(file.NewGrepTool(fs))
			toolRegistry.Register(shell.NewShellTool())
		}
	}

	// Create approval cache
	approvalCache := runtime.NewMemoryApprovalCache()

	// Create orchestrator
	orch := orchestrator.NewOrchestrator(
		toolRegistry,
		approvalCache,
		nil, // Approval handler will be set per-session
	)

	// Set up history configuration
	var historyFs afero.Fs
	var sessionsRoot string
	if opts.EnableHistory {
		historyFs = afero.NewOsFs()

		// Determine sessions root path
		if opts.HistoryPath != "" {
			sessionsRoot = opts.HistoryPath
		} else {
			// Default to ~/.codex/sessions
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get home directory: %w", err)
			}
			sessionsRoot = filepath.Join(homeDir, ".codex", "sessions")
		}
	}

	// Create conversation manager
	mgr, err := manager.NewManager(manager.ManagerConfig{
		Client:        opts.Client.Internal(),
		Orchestrator:  orch,
		HistoryFs:     historyFs,
		SessionsRoot:  sessionsRoot,
		EnableHistory: opts.EnableHistory,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation manager: %w", err)
	}

	return &SDK{
		client:        opts.Client,
		manager:       mgr,
		toolRegistry:  toolRegistry,
		orchestrator:  orch,
		approvalCache: approvalCache,
		enableHistory: opts.EnableHistory,
		historyPath:   sessionsRoot,
		sessions:      make(map[string]*Session),
	}, nil
}

// NewSession creates a new conversation session with the given options.
// Sessions are isolated and can run concurrently.
func (s *SDK) NewSession(ctx context.Context, opts SessionOptions) (*Session, error) {
	// Apply default values before validation
	if opts.ApprovalPolicy == "" {
		opts.ApprovalPolicy = "auto"
	}
	if opts.SandboxPolicy == "" {
		opts.SandboxPolicy = "native"
	}
	if opts.WorkingDirectory == "" {
		opts.WorkingDirectory = "."
	}

	// Validate session options
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid session options: %w", err)
	}

	// Generate session ID if not provided
	sessionID := opts.ConversationID
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	// Create turn context for manager session
	turnContext := &manager.TurnContext{
		Cwd:            opts.WorkingDirectory,
		ApprovalPolicy: opts.ApprovalPolicy,
		SandboxPolicy: protocol.SandboxPolicy{
			Mode: opts.SandboxPolicy,
		},
		Model: opts.Model,
	}

	// Create session with conversation manager
	mgrSession, err := s.manager.CreateSession(ctx, manager.SessionConfig{
		ID:           sessionID,
		Client:       s.client.Internal(),
		TurnContext:  turnContext,
		Orchestrator: s.orchestrator,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create manager session: %w", err)
	}

	// Create SDK session wrapper
	session := &Session{
		sdk:              s,
		mgrSession:       mgrSession,
		systemPrompt:     opts.SystemPrompt,
		streaming:        opts.Streaming,
		onToolApproval:   opts.OnToolApproval,
		approvalPolicy:   opts.ApprovalPolicy,
		sandboxPolicy:    opts.SandboxPolicy,
		workingDirectory: opts.WorkingDirectory,
		model:            opts.Model,
		conversationID:   sessionID,
		messages:         make([]*Message, 0),
	}

	// Store session
	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	return session, nil
}

// GetSession retrieves an existing session by ID.
func (s *SDK) GetSession(sessionID string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// ListSessions returns the IDs of all active sessions.
func (s *SDK) ListSessions() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}

	return ids
}

// CloseSession closes a specific session and removes it from the SDK.
func (s *SDK) CloseSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Close the session
	if err := session.close(); err != nil {
		return fmt.Errorf("failed to close session: %w", err)
	}

	// Remove from SDK
	delete(s.sessions, sessionID)

	return nil
}

// Close closes all sessions and shuts down the SDK.
// This should be called when the SDK is no longer needed to clean up resources.
func (s *SDK) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error

	// Close all sessions
	for id, session := range s.sessions {
		if err := session.close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close session %s: %w", id, err))
		}
	}

	// Close conversation manager
	if err := s.manager.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close conversation manager: %w", err))
	}

	// Clear sessions
	s.sessions = make(map[string]*Session)

	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}

	return nil
}

// ResumeSession resumes an existing session from history.
// This loads the session from disk and restores its state, allowing
// you to continue a previous conversation.
//
// If the session is already loaded in memory, it returns the existing session.
// If history persistence is not enabled, this returns an error.
func (s *SDK) ResumeSession(ctx context.Context, sessionID string) (*Session, error) {
	// Check if session is already in memory
	s.mu.RLock()
	if session, ok := s.sessions[sessionID]; ok {
		s.mu.RUnlock()
		return session, nil
	}
	s.mu.RUnlock()

	// Check if history is enabled
	if !s.enableHistory {
		return nil, fmt.Errorf("history persistence is not enabled")
	}

	// Resume session using manager
	mgrSession, err := s.manager.ResumeSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to resume session: %w", err)
	}

	// Get turn context from resumed session
	turnCtx := mgrSession.GetTurnContext()

	// Create SDK session wrapper
	session := &Session{
		sdk:              s,
		systemPrompt:     "", // System prompt is not persisted
		streaming:        false,
		approvalPolicy:   turnCtx.ApprovalPolicy,
		sandboxPolicy:    turnCtx.SandboxPolicy.Mode,
		workingDirectory: turnCtx.Cwd,
		model:            turnCtx.Model,
		conversationID:   sessionID,
		messages:         make([]*Message, 0),
	}

	// Store session
	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	return session, nil
}

// ListPersistedSessions returns the IDs of all sessions with persisted history.
// This only works if history persistence is enabled.
func (s *SDK) ListPersistedSessions() ([]string, error) {
	if !s.enableHistory {
		return nil, fmt.Errorf("history persistence is not enabled")
	}

	if s.historyPath == "" {
		return nil, fmt.Errorf("history path is not configured")
	}

	fs := afero.NewOsFs()

	// List directories in sessions root
	entries, err := afero.ReadDir(fs, s.historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessionIDs []string
	for _, entry := range entries {
		if entry.IsDir() {
			sessionIDs = append(sessionIDs, entry.Name())
		}
	}

	return sessionIDs, nil
}

// DeleteSession removes a session's persisted history from disk.
// This does NOT close an active session - use CloseSession for that.
// This only works if history persistence is enabled.
func (s *SDK) DeleteSession(sessionID string) error {
	if !s.enableHistory {
		return fmt.Errorf("history persistence is not enabled")
	}

	if s.historyPath == "" {
		return fmt.Errorf("history path is not configured")
	}

	fs := afero.NewOsFs()

	// Build session directory path
	sessionDir := filepath.Join(s.historyPath, sessionID)

	// Remove the entire session directory
	err := fs.RemoveAll(sessionDir)
	if err != nil {
		return fmt.Errorf("failed to delete session history: %w", err)
	}

	return nil
}

// SessionMetadata contains information about a persisted session.
type SessionMetadata struct {
	SessionID  string
	Path       string
	TurnCount  int
	MessageCount int
	LastModified string
}

// GetSessionMetadata returns metadata about a persisted session.
// This only works if history persistence is enabled.
func (s *SDK) GetSessionMetadata(sessionID string) (*SessionMetadata, error) {
	if !s.enableHistory {
		return nil, fmt.Errorf("history persistence is not enabled")
	}

	if s.historyPath == "" {
		return nil, fmt.Errorf("history path is not configured")
	}

	fs := afero.NewOsFs()

	// Build session directory path
	sessionDir := filepath.Join(s.historyPath, sessionID)

	// Check if session exists
	exists, err := afero.Exists(fs, sessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to check session existence: %w", err)
	}

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Get directory info for last modified time
	info, err := fs.Stat(sessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to stat session directory: %w", err)
	}

	metadata := &SessionMetadata{
		SessionID:    sessionID,
		Path:         sessionDir,
		LastModified: info.ModTime().Format("2006-01-02 15:04:05"),
	}

	// Try to load history to get counts
	// This is best-effort - if it fails, we still return basic metadata
	historyPath := filepath.Join(sessionDir, "history.jsonl")
	if exists, _ := afero.Exists(fs, historyPath); exists {
		// Use manager's history loading capabilities
		// For now, we'll just note that the history file exists
		// In a full implementation, we'd parse it to get accurate counts
		metadata.TurnCount = -1 // Unknown
		metadata.MessageCount = -1 // Unknown
	}

	return metadata, nil
}

// generateSessionID generates a unique session identifier.
var sessionIDCounter int64

func generateSessionID() string {
	// Use a counter to ensure unique IDs in tests
	sessionIDCounter++
	return fmt.Sprintf("session_%d", sessionIDCounter)
}
