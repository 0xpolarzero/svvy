package manager

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/evmts/codex/codex-go/internal/client"
	"github.com/evmts/codex/codex-go/internal/config"
	"github.com/evmts/codex/codex-go/internal/history/persistence"
	"github.com/evmts/codex/codex-go/internal/models"
	"github.com/evmts/codex/codex-go/internal/notify"
	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/evmts/codex/codex-go/internal/tools/orchestrator"
	"github.com/evmts/codex/codex-go/internal/tools/runtime"
	"github.com/spf13/afero"
)

// ConversationManager is the main interface for managing conversation sessions.
// It coordinates session lifecycle, turn processing, and event handling.
type ConversationManager interface {
	// CreateSession creates a new conversation session
	CreateSession(ctx context.Context, cfg SessionConfig) (*Session, error)

	// GetSession retrieves an existing session by ID
	GetSession(sessionID string) (*Session, error)

	// AcquireSession retrieves a session and increments its reference count
	// Caller MUST call session.Release() when done
	AcquireSession(sessionID string) (*Session, error)

	// ListSessions returns all active session IDs
	ListSessions() []string

	// CloseSession closes a session and removes it from the manager
	CloseSession(sessionID string) error

	// SubmitOp submits an operation to a session for processing
	SubmitOp(ctx context.Context, sessionID string, op protocol.Op) error

	// ResumeSession resumes an existing session from history if available
	ResumeSession(ctx context.Context, sessionID string) (*Session, error)

	// SwitchModel switches the model for an existing session
	SwitchModel(ctx context.Context, sessionID string, modelID string) error

	// Close closes all sessions and shuts down the manager
	Close() error
}

// manager implements ConversationManager.
type manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	client   client.Client
	orch     *orchestrator.Orchestrator
	notifier *notify.Notifier

	// History persistence settings
	historyFs     afero.Fs
	sessionsRoot  string
	enableHistory bool

	// Goroutine lifecycle management
	activeGoroutines int64 // Atomic counter for active goroutines across all sessions

	// History persistence interface for session state management
	historyStore HistoryStore
}

// ManagerConfig contains configuration for the conversation manager.
type ManagerConfig struct {
	Client       client.Client
	Orchestrator *orchestrator.Orchestrator
	// History settings
	HistoryFs     afero.Fs
	SessionsRoot  string
	EnableHistory bool
	// History storage backend (optional, defaults to FilesystemHistoryStore if history enabled)
	HistoryStore HistoryStore
	// Notification settings
	NotifyConfig *config.NotifyConfig
}

// NewManager creates a new conversation manager.
func NewManager(cfg ManagerConfig) (ConversationManager, error) {
	if cfg.Client == nil {
		return nil, fmt.Errorf("client is required")
	}

	// Initialize notifier if configuration is provided
	var notifier *notify.Notifier
	if cfg.NotifyConfig != nil {
		notifier = notify.NewNotifier(convertNotifyConfig(cfg.NotifyConfig))
	}

	// Initialize history store if history is enabled
	var historyStore HistoryStore
	if cfg.EnableHistory {
		if cfg.HistoryStore != nil {
			// Use provided history store
			historyStore = cfg.HistoryStore
		} else if cfg.HistoryFs != nil && cfg.SessionsRoot != "" {
			// Default to filesystem history store
			store, err := NewFilesystemHistoryStore(cfg.HistoryFs, cfg.SessionsRoot)
			if err != nil {
				return nil, fmt.Errorf("failed to create history store: %w", err)
			}
			historyStore = store
		}
	}

	return &manager{
		sessions:      make(map[string]*Session),
		client:        cfg.Client,
		orch:          cfg.Orchestrator,
		notifier:      notifier,
		historyFs:     cfg.HistoryFs,
		sessionsRoot:  cfg.SessionsRoot,
		enableHistory: cfg.EnableHistory,
		historyStore:  historyStore,
	}, nil
}

// CreateSession creates a new conversation session.
func (m *manager) CreateSession(ctx context.Context, cfg SessionConfig) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// SECURITY: Validate session ID to prevent path traversal attacks
	if err := ValidateSessionID(cfg.ID); err != nil {
		// Log security violation
		log.Printf("SECURITY WARNING: Invalid session ID rejected: %v", err)
		return nil, fmt.Errorf("invalid session ID: %w", err)
	}

	// Check if session already exists
	if _, exists := m.sessions[cfg.ID]; exists {
		return nil, fmt.Errorf("session %s already exists", cfg.ID)
	}

	// Use manager's client if not provided
	if cfg.Client == nil {
		cfg.Client = m.client
	}

	// Use manager's orchestrator if not provided
	if cfg.Orchestrator == nil {
		cfg.Orchestrator = m.orch
	}

	// Set up history if enabled
	if m.enableHistory && m.historyFs != nil && m.sessionsRoot != "" {
		// SECURITY: Use validated path construction to prevent directory traversal
		sessionDir, err := ValidateAndResolveSessionPath(cfg.ID, m.sessionsRoot)
		if err != nil {
			// Log security violation
			log.Printf("SECURITY WARNING: Session path validation failed for ID %q: %v",
				sanitizeSessionIDForLog(cfg.ID), err)
			return nil, fmt.Errorf("failed to resolve session path: %w", err)
		}

		hp, err := persistence.NewHistoryPersistence(m.historyFs, sessionDir)
		if err != nil {
			return nil, fmt.Errorf("failed to set up history persistence: %w", err)
		}
		cfg.History = hp
	}

	// Create the session
	session, err := NewSession(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Store the session
	m.sessions[cfg.ID] = session

	// Emit session configured event
	if err := session.EmitSessionConfigured(ctx); err != nil {
		// Log error but don't fail session creation
		// The session is still usable even if the event fails to emit
		_ = err
	}

	// Save initial session state if history store is available
	if m.historyStore != nil {
		state := session.ExtractState()
		if err := m.historyStore.SaveSession(ctx, state); err != nil {
			// Log error but don't fail session creation
			// The session is still usable even if state save fails
			log.Printf("WARNING: Failed to save initial session state for %s: %v", cfg.ID, err)
		}
	}

	return session, nil
}

// GetSession retrieves an existing session by ID.
// Note: This method does NOT increment the reference count.
// For operations that need to use the session beyond the current call,
// use AcquireSession instead.
func (m *manager) GetSession(sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	return session, nil
}

// AcquireSession retrieves a session and increments its reference count.
// This prevents the session from being deleted while in use.
// The caller MUST call session.Release() when done to prevent leaks.
// Returns an error if the session doesn't exist or is closing.
//
// Thread-Safety: This method is safe for concurrent use and prevents
// the TOCTOU race condition between GetSession and CloseSession.
func (m *manager) AcquireSession(sessionID string) (*Session, error) {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	// Acquire reference - this may fail if session is closing
	if err := session.Acquire(); err != nil {
		return nil, err
	}

	return session, nil
}

// ListSessions returns all active session IDs.
func (m *manager) ListSessions() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}

	return ids
}

// CloseSession closes a session and removes it from the manager.
// This method waits for all active operations on the session to complete
// before removing it, preventing use-after-free races.
//
// Thread-Safety: Safe for concurrent use. Will block until all operations
// using the session have called Release().
func (m *manager) CloseSession(sessionID string) error {
	// First, remove the session from the map so no new operations can find it
	m.mu.Lock()
	session, exists := m.sessions[sessionID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Remove from manager immediately to prevent new operations
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	// Now close the session - this will wait for active operations to complete
	// This happens outside the manager lock to avoid deadlocks
	if err := session.Close(); err != nil {
		return fmt.Errorf("failed to close session: %w", err)
	}

	return nil
}

// SubmitOp submits an operation to a session for processing.
// Thread-Safety: This method uses reference counting to prevent race conditions
// with CloseSession. The session reference is held during the entire operation.
func (m *manager) SubmitOp(ctx context.Context, sessionID string, op protocol.Op) error {
	// Acquire the session with reference counting
	// This prevents the session from being deleted while we're using it
	session, err := m.AcquireSession(sessionID)
	if err != nil {
		return err
	}

	// Route based on operation type
	switch o := op.(type) {
	case *protocol.OpUserInput:
		// Transform user input into a full user turn using the session's current turn context
		turnCtx := session.GetTurnContext()
		turn := &protocol.OpUserTurn{
			Items:          o.Items,
			Cwd:            turnCtx.Cwd,
			ApprovalPolicy: turnCtx.ApprovalPolicy,
			SandboxPolicy:  turnCtx.SandboxPolicy,
			Model:          turnCtx.Model,
			Effort:         turnCtx.Effort,
			Summary:        turnCtx.Summary,
		}
		return m.handleUserTurn(ctx, session, turn)
	case *protocol.OpUserTurn:
		return m.handleUserTurn(ctx, session, o)

	case *protocol.OpInterrupt:
		defer session.Release()
		return m.handleInterrupt(ctx, session, o)

	case *protocol.OpExecApproval:
		defer session.Release()
		return m.handleExecApproval(ctx, session, o)

	case *protocol.OpPatchApproval:
		defer session.Release()
		return m.handlePatchApproval(ctx, session, o)

	case *protocol.OpOverrideTurnContext:
		defer session.Release()
		return m.handleOverrideTurnContext(ctx, session, o)

	default:
		session.Release()
		return fmt.Errorf("unsupported operation type: %s", op.OpType())
	}
}

// handleUserTurn processes a user turn submission.
//
// Goroutine Lifecycle:
// This function spawns a background goroutine to process the turn asynchronously.
// The goroutine lifecycle is carefully managed to prevent leaks:
//
// 1. Reference Counting: The session reference (acquired in SubmitOp) is transferred
//    to the goroutine, which releases it when done via defer.
//
// 2. Panic Recovery: The goroutine includes panic recovery to prevent crashes and
//    ensure cleanup happens even if ProcessTurn panics.
//
// 3. Context Cancellation: The goroutine checks for context cancellation before and
//    after processing to allow quick termination when the session is closed.
//
// 4. Atomic Counter: The manager tracks active goroutines via atomic counter for
//    monitoring and debugging (see GetActiveGoroutineCount).
//
// Thread-Safety:
// - The session reference prevents the session from being closed while processing
// - Session.Close() will block until this goroutine exits (via reference counting)
// - Context cancellation provides a signal to exit quickly
//
// See GOROUTINE_LIFECYCLE.md for detailed documentation.
func (m *manager) handleUserTurn(ctx context.Context, session *Session, op *protocol.OpUserTurn) error {
	// Check if session can accept the turn
	if !session.CanAcceptTurn() {
		session.Release()
		return fmt.Errorf("session %s cannot accept turn in state %s", session.ID(), session.State())
	}

	// Submit the turn to the session
	submissionID, err := session.SubmitTurn(ctx, op)
	if err != nil {
		session.Release()
		return fmt.Errorf("failed to submit turn: %w", err)
	}

	// Record submission to history if enabled
	session.RecordSubmission(&protocol.Submission{ID: submissionID, Op: op})

	// Increment active goroutine counter for metrics
	atomic.AddInt64(&m.activeGoroutines, 1)

	// Process the turn in a goroutine
	// The goroutine takes ownership of the session reference
	go func() {
		// Ensure session reference is released when goroutine completes
		defer session.Release()

		// Decrement active goroutine counter
		defer atomic.AddInt64(&m.activeGoroutines, -1)

		// Panic recovery to prevent crashes and resource leaks
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC in turn processing goroutine for session %s, submission %s: %v",
					session.ID(), submissionID, r)

				// Attempt to emit error event
				processor := NewTurnProcessorWithApprovalHandler(session, submissionID)
				_ = processor.emitError(context.Background(), submissionID,
					fmt.Sprintf("Internal error: %v", r))

				// Mark turn as failed
				_ = session.FailTurn(fmt.Sprintf("panic: %v", r))

				// Clean up approval handler
				session.ClearApprovalHandler()
			}
		}()

		// Use session context for cancellation
		turnCtx := session.Context()

		// Check if context is already cancelled before starting
		select {
		case <-turnCtx.Done():
			log.Printf("Session %s context cancelled before turn processing started", session.ID())
			_ = session.FailTurn("session context cancelled")
			return
		default:
		}

		processor := NewTurnProcessorWithApprovalHandler(session, submissionID)

		// Ensure cleanup happens regardless of outcome
		defer func() {
			if processor.approvalHandler != nil {
				processor.approvalHandler.CancelAllPending()
			}
			session.ClearApprovalHandler()
		}()

		if err := processor.ProcessTurn(turnCtx, submissionID, op); err != nil {
			// Check if error is due to context cancellation
			if turnCtx.Err() != nil {
				log.Printf("Turn processing cancelled for session %s: %v", session.ID(), turnCtx.Err())
				_ = session.FailTurn("turn cancelled")
				return
			}

			// Mark turn as failed
			_ = session.FailTurn(err.Error()) // nolint:errcheck

			// Emit error event
			_ = processor.emitError(turnCtx, submissionID, err.Error()) // nolint:errcheck

			// Send error notification
			if m.notifier != nil {
				_ = m.notifier.NotifyTurnError(turnCtx, session.ID(), submissionID, err.Error())
			}
			return
		}

		// Mark turn as completed
		if err := session.CompleteTurn(); err != nil {
			_ = session.FailTurn(err.Error()) // nolint:errcheck
			// Send error notification
			if m.notifier != nil {
				_ = m.notifier.NotifyTurnError(turnCtx, session.ID(), submissionID, err.Error())
			}
		} else {
			// Send completion notification
			if m.notifier != nil {
				lastMsg := session.GetLastAgentMessage()
				_ = m.notifier.NotifyTurnComplete(turnCtx, session.ID(), submissionID, lastMsg)
			}
		}

		// Save session state after turn completion if history store is available
		if m.historyStore != nil {
			state := session.ExtractState()
			if err := m.historyStore.SaveSession(context.Background(), state); err != nil {
				// Log error but don't fail the turn
				log.Printf("WARNING: Failed to save session state after turn for %s: %v", session.ID(), err)
			}
		}
	}()

	return nil
}

// handleInterrupt processes an interrupt operation.
func (m *manager) handleInterrupt(ctx context.Context, session *Session, op *protocol.OpInterrupt) error {
	// Generate submission ID for tracking
	submissionID := fmt.Sprintf("interrupt_%s_%d", session.ID(), time.Now().UnixNano())

	// Record submission to history if enabled
	session.RecordSubmission(&protocol.Submission{ID: submissionID, Op: op})

	// Send abort notification
	if m.notifier != nil {
		_ = m.notifier.NotifyTurnAborted(ctx, session.ID(), submissionID, "User interrupted turn")
	}

	return session.SubmitInterrupt(ctx)
}

// handleExecApproval processes an exec approval operation.
func (m *manager) handleExecApproval(ctx context.Context, session *Session, op *protocol.OpExecApproval) error {
	// Record submission to history if enabled
	session.RecordSubmission(&protocol.Submission{ID: op.ID, Op: op})

	// First submit to session state machine
	if err := session.SubmitApproval(ctx, op.ID, op.Decision); err != nil {
		return err
	}

	// Then notify the approval handler if one is active
	if handler := session.GetApprovalHandler(); handler != nil {
		// Parse the decision string
		decision, err := parseApprovalDecision(op.Decision)
		if err != nil {
			// Try to get the pending call ID and notify with denial
			if callID := handler.GetPendingApprovalCallID(); callID != "" {
				_ = handler.NotifyApprovalDecision(callID, runtime.ApprovalDenied)
			}
			return err
		}

		// Get the pending approval call ID and notify
		if callID := handler.GetPendingApprovalCallID(); callID != "" {
			return handler.NotifyApprovalDecision(callID, decision)
		}
	}

	return nil
}

// handlePatchApproval processes a patch approval operation.
func (m *manager) handlePatchApproval(ctx context.Context, session *Session, op *protocol.OpPatchApproval) error {
	// Record submission to history if enabled
	session.RecordSubmission(&protocol.Submission{ID: op.ID, Op: op})

	// First submit to session state machine
	if err := session.SubmitApproval(ctx, op.ID, op.Decision); err != nil {
		return err
	}

	// Then notify the approval handler if one is active
	if handler := session.GetApprovalHandler(); handler != nil {
		// Parse the decision string
		decision, err := parseApprovalDecision(op.Decision)
		if err != nil {
			// Try to get the pending call ID and notify with denial
			if callID := handler.GetPendingApprovalCallID(); callID != "" {
				_ = handler.NotifyApprovalDecision(callID, runtime.ApprovalDenied)
			}
			return err
		}

		// Get the pending approval call ID and notify
		if callID := handler.GetPendingApprovalCallID(); callID != "" {
			return handler.NotifyApprovalDecision(callID, decision)
		}
	}

	return nil
}

// handleOverrideTurnContext processes a turn context override.
func (m *manager) handleOverrideTurnContext(ctx context.Context, session *Session, op *protocol.OpOverrideTurnContext) error {
	// Generate submission ID for tracking
	submissionID := fmt.Sprintf("override_%s_%d", session.ID(), time.Now().UnixNano())

	// Record submission to history if enabled
	session.RecordSubmission(&protocol.Submission{ID: submissionID, Op: op})

	return session.UpdateTurnContext(op)
}

// SwitchModel switches the model for an existing session.
// This validates the model is available and updates the session configuration.
// The new model takes effect on the next turn submission.
func (m *manager) SwitchModel(ctx context.Context, sessionID string, modelID string) error {
	// Get the session
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	// Validate the new model exists
	model, err := models.ResolveModel(modelID)
	if err != nil {
		return fmt.Errorf("invalid model: %w", err)
	}

	// Update the turn context with new model
	override := &protocol.OpOverrideTurnContext{
		Model: &model.ID,
	}

	if err := session.UpdateTurnContext(override); err != nil {
		return fmt.Errorf("failed to update session model: %w", err)
	}

	// Emit session configured event with new model
	submissionID := fmt.Sprintf("switch_model_%s_%d", sessionID, time.Now().UnixNano())
	event := &protocol.Event{
		ID: submissionID,
		Msg: &protocol.EventSessionConfigured{
			SessionID:         sessionID,
			Model:             model.ID,
			HistoryLogID:      0,
			HistoryEntryCount: 0,
			RolloutPath:       "",
		},
	}

	if err := session.EmitEvent(ctx, event); err != nil {
		// Log but don't fail - the model switch succeeded
		_ = err
	}

	return nil
}

// Close closes all sessions and shuts down the manager.
func (m *manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error

	// Close all sessions
	for id, session := range m.sessions {
		if err := session.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close session %s: %w", id, err))
		}
	}

	// Clear sessions map
	m.sessions = make(map[string]*Session)

	// Log if there are still active goroutines (shouldn't happen if Close() worked correctly)
	remaining := atomic.LoadInt64(&m.activeGoroutines)
	if remaining > 0 {
		log.Printf("WARNING: Manager closed with %d active goroutines still running", remaining)
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing sessions: %v", errors)
	}

	return nil
}

// GetActiveGoroutineCount returns the number of active turn processing goroutines.
// This is useful for monitoring and debugging goroutine leaks.
func (m *manager) GetActiveGoroutineCount() int64 {
	return atomic.LoadInt64(&m.activeGoroutines)
}

// ResumeSession loads an existing session from history if not in memory.
// It reconstructs the full conversation state including:
// - Conversation history (user/assistant/tool messages)
// - Token usage statistics
// - Turn context (cwd, approval policy, sandbox policy, model)
// - Session state validation
//
// If a HistoryStore is configured, it will attempt to load the session state
// from the store first for faster resume. Otherwise, it falls back to
// reconstructing state from the history.jsonl file.
func (m *manager) ResumeSession(ctx context.Context, sessionID string) (*Session, error) {
	// Check in-memory first
	if session, err := m.GetSession(sessionID); err == nil {
		return session, nil
	}

	// SECURITY: Validate session ID to prevent path traversal attacks
	if err := ValidateSessionID(sessionID); err != nil {
		// Log security violation
		log.Printf("SECURITY WARNING: Invalid session ID rejected in ResumeSession: %v", err)
		return nil, fmt.Errorf("invalid session ID: %w", err)
	}

	// Require history configuration
	if !m.enableHistory || m.historyFs == nil || m.sessionsRoot == "" {
		return nil, fmt.Errorf("history persistence not configured")
	}

	// SECURITY: Use validated path construction to prevent directory traversal
	sessionDir, err := ValidateAndResolveSessionPath(sessionID, m.sessionsRoot)
	if err != nil {
		// Log security violation
		log.Printf("SECURITY WARNING: Session path validation failed in ResumeSession for ID %q: %v",
			sanitizeSessionIDForLog(sessionID), err)
		return nil, fmt.Errorf("failed to resolve session path: %w", err)
	}

	hp, err := persistence.NewHistoryPersistence(m.historyFs, sessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open history: %w", err)
	}

	// Try to load session state from history store if available
	// This is faster than reconstructing from history.jsonl
	var turnCtx *TurnContext
	var tokenUsage *protocol.TokenUsage
	var lastAgentMessage string
	var historyLogID uint64
	var historyEntryCount int
	var provider string

	if m.historyStore != nil {
		savedState, err := m.historyStore.LoadSession(ctx, sessionID)
		if err == nil {
			// Successfully loaded state from store
			turnCtx = savedState.TurnContext
			tokenUsage = savedState.TokenUsage
			lastAgentMessage = savedState.LastAgentMessage
			historyLogID = savedState.HistoryLogID
			historyEntryCount = savedState.HistoryEntryCount
			provider = savedState.Provider
			log.Printf("Restored session %s from history store (updated at %s)", sessionID, savedState.UpdatedAt)
		} else if err != ErrSessionNotFound {
			// Log error but continue with reconstruction
			log.Printf("WARNING: Failed to load session state from store for %s: %v", sessionID, err)
		}
	}

	// If we didn't get state from the store, reconstruct from history
	if turnCtx == nil {
		// Load history entries (both submissions and events)
		submissions, events, err := hp.LoadHistory()
		if err != nil {
			return nil, fmt.Errorf("failed to load history: %w", err)
		}

		// Reconstruct complete session state from history
		reconstructed, err := ReconstructStateFromHistory(submissions, events)
		if err != nil {
			return nil, fmt.Errorf("failed to reconstruct state: %w", err)
		}

		// Validate reconstructed state
		if err := ValidateResumedState(reconstructed); err != nil {
			return nil, fmt.Errorf("invalid session state: %w", err)
		}

		// Use reconstructed state
		turnCtx = reconstructed.TurnContext
		tokenUsage = reconstructed.TokenUsage
		lastAgentMessage = reconstructed.LastAgentMessage
		historyEntryCount = reconstructed.CompletedTurns
	}

	// Ensure we have a valid turn context
	if turnCtx == nil {
		// Fallback to defaults if reconstruction failed
		turnCtx = &TurnContext{
			Cwd:            ".",
			ApprovalPolicy: "auto",
			SandboxPolicy:  protocol.SandboxPolicy{Mode: "native"},
			Model:          "",
		}
	}

	// Create new session with reconstructed context
	sess, err := NewSession(SessionConfig{
		ID:           sessionID,
		Client:       m.client,
		TurnContext:  turnCtx,
		Orchestrator: m.orch,
		History:      hp,
		Provider:     provider,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Restore session state
	if lastAgentMessage != "" {
		sess.SetLastAgentMessage(lastAgentMessage)
	}

	if tokenUsage != nil {
		sess.UpdateTokenUsage(tokenUsage)
	}

	// Store in manager
	m.mu.Lock()
	m.sessions[sessionID] = sess
	m.mu.Unlock()

	// Set history metadata
	sess.SetHistoryMetadata(historyLogID, historyEntryCount)

	// Emit session configured event for resumed session
	if err := sess.EmitSessionConfigured(ctx); err != nil {
		// Log error but don't fail session resume
		_ = err
	}

	return sess, nil
}

// convertNotifyConfig converts config.NotifyConfig to notify.Config.
func convertNotifyConfig(cfg *config.NotifyConfig) *notify.Config {
	if cfg == nil {
		return &notify.Config{}
	}

	notifyCfg := &notify.Config{}

	// Convert OnTurnComplete
	if cfg.OnTurnComplete != nil {
		notifyCfg.OnTurnComplete = &notify.NotificationConfig{
			Command: cfg.OnTurnComplete.Command,
			Enabled: cfg.OnTurnComplete.Enabled,
			Env:     cfg.OnTurnComplete.Env,
		}
	}

	// Convert OnError
	if cfg.OnError != nil {
		notifyCfg.OnError = &notify.NotificationConfig{
			Command: cfg.OnError.Command,
			Enabled: cfg.OnError.Enabled,
			Env:     cfg.OnError.Env,
		}
	}

	// Convert OnApprovalNeeded
	if cfg.OnApprovalNeeded != nil {
		notifyCfg.OnApprovalNeeded = &notify.NotificationConfig{
			Command: cfg.OnApprovalNeeded.Command,
			Enabled: cfg.OnApprovalNeeded.Enabled,
			Env:     cfg.OnApprovalNeeded.Env,
		}
	}

	// Convert OnTurnAborted
	if cfg.OnTurnAborted != nil {
		notifyCfg.OnTurnAborted = &notify.NotificationConfig{
			Command: cfg.OnTurnAborted.Command,
			Enabled: cfg.OnTurnAborted.Enabled,
			Env:     cfg.OnTurnAborted.Env,
		}
	}

	// Convert timeout
	if cfg.ScriptTimeoutSec != nil && *cfg.ScriptTimeoutSec > 0 {
		notifyCfg.ScriptTimeout = time.Duration(*cfg.ScriptTimeoutSec * float64(time.Second))
	}

	return notifyCfg
}
