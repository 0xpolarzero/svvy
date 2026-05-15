package sdk

import (
	"context"
	"fmt"

	"github.com/evmts/codex/codex-go/internal/conversation/manager"
	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/google/uuid"
)

// newSessionWithManager creates a new session integrated with the conversation manager.
// This is called by NewSession after validation.
func (s *SDK) newSessionWithManager(ctx context.Context, opts SessionOptions) (*Session, error) {
	// Generate session ID if not provided
	sessionID := opts.ConversationID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// Apply defaults for unset options
	opts = applySessionDefaults(opts)

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
		managerSession:   mgrSession,
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

	return session, nil
}

// closeSessionWithManager closes a session and coordinates with the manager.
func (s *SDK) closeSessionWithManager(sessionID string) error {
	s.mu.Lock()
	session, ok := s.sessions[sessionID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Remove from SDK immediately to prevent new operations
	delete(s.sessions, sessionID)
	s.mu.Unlock()

	// Close the session (outside lock to avoid deadlock)
	if err := session.close(); err != nil {
		// Continue with manager cleanup even if session close fails
		_ = err
	}

	// Close manager session
	if err := s.manager.CloseSession(sessionID); err != nil {
		return fmt.Errorf("failed to close manager session: %w", err)
	}

	return nil
}
