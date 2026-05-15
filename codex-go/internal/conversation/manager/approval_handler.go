package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/evmts/codex/codex-go/internal/tools/orchestrator"
	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// SessionApprovalHandler creates an approval handler that integrates with the Session state machine.
// It bridges orchestrator approval requests to Session.RequestApproval and waits for user response.
type SessionApprovalHandler struct {
	session      *Session
	submissionID string
	mu           sync.Mutex
	pendingReqs  map[string]*approvalRequest
}

// approvalRequest tracks a pending approval request and its response channel.
type approvalRequest struct {
	req      *runtime.ApprovalRequest
	respChan chan runtime.ApprovalDecision
	errChan  chan error
}

// NewSessionApprovalHandler creates a new approval handler for a session and turn.
func NewSessionApprovalHandler(session *Session, submissionID string) *SessionApprovalHandler {
	return &SessionApprovalHandler{
		session:      session,
		submissionID: submissionID,
		pendingReqs:  make(map[string]*approvalRequest),
	}
}

// HandleApproval implements the orchestrator.ApprovalHandler function signature.
// It transitions the session to StateAwaitingApproval, emits an approval event,
// and blocks until the user responds via SubmitApproval.
func (h *SessionApprovalHandler) HandleApproval(ctx context.Context, req *runtime.ApprovalRequest) (runtime.ApprovalDecision, error) {
	// Check approval policy - auto-approve if policy is "auto"
	turnCtx := h.session.GetTurnContext()
	if turnCtx.ApprovalPolicy == "auto" {
		return runtime.ApprovalApprovedForSession, nil
	}

	// Check if approval handler was incorrectly called with "never" policy
	// "never" policy means the approval handler should not be invoked at all.
	// If we reach here with "never" policy, it's a configuration error or bug.
	// We should not blindly approve - instead return an error to surface the issue.
	if turnCtx.ApprovalPolicy == "never" {
		return runtime.ApprovalDenied, fmt.Errorf("approval handler invoked with 'never' policy - operations should proceed without approval")
	}

	// For manual and semi-auto policies, we need to request approval

	// Check semi-auto policy - only require approval for exec/patch operations
	if turnCtx.ApprovalPolicy == "semi-auto" {
		// For semi-auto, only require approval for retries or if the command is potentially dangerous
		if !req.IsRetry && (req.Command == nil || len(req.Command) == 0) {
			// Non-command tools in semi-auto are auto-approved
			return runtime.ApprovalApprovedForSession, nil
		}
		if !req.IsRetry && req.Command != nil && runtime.IsKnownSafeCommand(req.Command) {
			// Known safe commands are auto-approved in semi-auto
			return runtime.ApprovalApprovedForSession, nil
		}
	}

    // Create approval request tracking
    approvalReq := &approvalRequest{
        req:      req,
        respChan: make(chan runtime.ApprovalDecision, 1),
        errChan:  make(chan error, 1),
    }

    h.mu.Lock()
    // Reject concurrent approval requests to avoid ambiguous state
    if len(h.pendingReqs) > 0 {
        h.mu.Unlock()
        return runtime.ApprovalDenied, fmt.Errorf("another approval request is already pending")
    }
    h.pendingReqs[req.CallID] = approvalReq
    h.mu.Unlock()

	// Transition session to awaiting approval state
	approvalType := ApprovalTypeExec
	if req.ToolName == "patch" || req.ToolName == "edit_file" {
		approvalType = ApprovalTypePatch
	}

	if err := h.session.RequestApproval(h.submissionID, approvalType); err != nil {
		h.mu.Lock()
		delete(h.pendingReqs, req.CallID)
		h.mu.Unlock()
		return runtime.ApprovalDenied, fmt.Errorf("failed to request approval: %w", err)
	}

	// Emit approval needed event
	if err := h.emitApprovalNeededEvent(ctx, req); err != nil {
		// Try to clean up state
		h.mu.Lock()
		delete(h.pendingReqs, req.CallID)
		h.mu.Unlock()
		// Attempt to transition back, but don't fail if we can't
		_ = h.session.SubmitApproval(ctx, h.submissionID, "deny")
		return runtime.ApprovalDenied, fmt.Errorf("failed to emit approval event: %w", err)
	}

	// Set up timeout for approval
	approvalTimeout := turnCtx.ApprovalTimeout
	if approvalTimeout <= 0 {
		approvalTimeout = 5 * time.Minute // Default to 5 minutes
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, approvalTimeout)
	defer cancel()

	// Wait for approval decision, timeout, or context cancellation
	select {
	case <-timeoutCtx.Done():
		h.mu.Lock()
		delete(h.pendingReqs, req.CallID)
		h.mu.Unlock()
		// Check if it was a timeout or parent context cancellation
		if ctx.Err() != nil {
			return runtime.ApprovalDenied, ctx.Err()
		}
		return runtime.ApprovalDenied, fmt.Errorf("approval request timed out after %v", approvalTimeout)

	case err := <-approvalReq.errChan:
		h.mu.Lock()
		delete(h.pendingReqs, req.CallID)
		h.mu.Unlock()
		return runtime.ApprovalDenied, err

	case decision := <-approvalReq.respChan:
		h.mu.Lock()
		delete(h.pendingReqs, req.CallID)
		h.mu.Unlock()
		return decision, nil
	}
}

// NotifyApprovalDecision is called by the manager when a user submits an approval decision.
// It unblocks the waiting HandleApproval call.
func (h *SessionApprovalHandler) NotifyApprovalDecision(callID string, decision runtime.ApprovalDecision) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	req, exists := h.pendingReqs[callID]
	if !exists {
		return fmt.Errorf("no pending approval request for call ID: %s", callID)
	}

	// Send decision to waiting goroutine
	select {
	case req.respChan <- decision:
		return nil
	default:
		return fmt.Errorf("failed to send approval decision for call ID: %s", callID)
	}
}

// NotifyApprovalError is called when an error occurs during approval processing.
func (h *SessionApprovalHandler) NotifyApprovalError(callID string, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	req, exists := h.pendingReqs[callID]
	if !exists {
		return
	}

	// Send error to waiting goroutine
	select {
	case req.errChan <- err:
	default:
	}
}

// emitApprovalNeededEvent emits the protocol event for approval requests.
func (h *SessionApprovalHandler) emitApprovalNeededEvent(ctx context.Context, req *runtime.ApprovalRequest) error {
	event := &protocol.Event{
		ID: h.submissionID,
		Msg: &protocol.EventToolCallApprovalNeeded{
			CallID:           req.CallID,
			ToolName:         req.ToolName,
			Command:          req.Command,
			WorkingDirectory: req.WorkingDirectory,
			Justification:    req.Justification,
			IsRetry:          req.IsRetry,
			RetryReason:      req.RetryReason,
		},
	}

	// Add risk assessment if available
	if req.Risk != nil {
		evt := event.Msg.(*protocol.EventToolCallApprovalNeeded)
		evt.RiskLevel = formatRiskLevel(req.Risk.Level)
		evt.RiskReasons = req.Risk.Reasons
		evt.RiskMitigation = req.Risk.Mitigation
	}

	return h.session.EmitEvent(ctx, event)
}

// formatRiskLevel converts runtime.RiskLevel to a string for protocol events.
func formatRiskLevel(level runtime.RiskLevel) string {
	switch level {
	case runtime.RiskLow:
		return "low"
	case runtime.RiskMedium:
		return "medium"
	case runtime.RiskHigh:
		return "high"
	case runtime.RiskCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// GetPendingApprovalCallID returns the call ID of the current pending approval, if any.
// This is used to correlate user approval submissions with the correct tool call.
func (h *SessionApprovalHandler) GetPendingApprovalCallID() string {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Return the first (should be only one) pending call ID
	for callID := range h.pendingReqs {
		return callID
	}
	return ""
}

// HasPendingApproval returns true if there's a pending approval request.
func (h *SessionApprovalHandler) HasPendingApproval() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.pendingReqs) > 0
}

// CancelAllPending cancels all pending approval requests (e.g., on turn interruption).
func (h *SessionApprovalHandler) CancelAllPending() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for callID, req := range h.pendingReqs {
		select {
		case req.errChan <- fmt.Errorf("approval cancelled"):
		default:
		}
		delete(h.pendingReqs, callID)
	}
}

// CreateApprovalHandlerFunc creates an orchestrator.ApprovalHandler function from this handler.
func (h *SessionApprovalHandler) CreateApprovalHandlerFunc() orchestrator.ApprovalHandler {
	return func(ctx context.Context, req *runtime.ApprovalRequest) (runtime.ApprovalDecision, error) {
		return h.HandleApproval(ctx, req)
	}
}

// parseApprovalDecision converts a string decision to runtime.ApprovalDecision.
func parseApprovalDecision(decision string) (runtime.ApprovalDecision, error) {
	switch decision {
	case "approve", "approved", "yes":
		return runtime.ApprovalApproved, nil
	case "approve_session", "always", "all":
		return runtime.ApprovalApprovedForSession, nil
	case "deny", "denied", "no":
		return runtime.ApprovalDenied, nil
	case "abort", "cancel":
		return runtime.ApprovalAbort, nil
	default:
		return runtime.ApprovalDenied, fmt.Errorf("invalid approval decision: %s", decision)
	}
}

// WaitForApprovalWithTimeout is a helper that waits for approval with a timeout.
// It's not currently used but could be useful for future implementations.
func (h *SessionApprovalHandler) WaitForApprovalWithTimeout(ctx context.Context, callID string, timeout time.Duration) (runtime.ApprovalDecision, error) {
	h.mu.Lock()
	req, exists := h.pendingReqs[callID]
	h.mu.Unlock()

	if !exists {
		return runtime.ApprovalDenied, fmt.Errorf("no pending approval for call ID: %s", callID)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return runtime.ApprovalDenied, fmt.Errorf("approval timeout")
	case err := <-req.errChan:
		return runtime.ApprovalDenied, err
	case decision := <-req.respChan:
		return decision, nil
	}
}
