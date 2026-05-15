package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// ApprovalManager handles approval requests and caching for tool execution.
// It provides a higher-level abstraction over the approval cache and handler.
type ApprovalManager struct {
	cache   runtime.ApprovalCache
	handler ApprovalHandler
}

// NewApprovalManager creates a new approval manager.
func NewApprovalManager(cache runtime.ApprovalCache, handler ApprovalHandler) *ApprovalManager {
	return &ApprovalManager{
		cache:   cache,
		handler: handler,
	}
}

// RequestApproval requests user approval for a tool operation.
// It constructs an approval request with all necessary context and delegates
// to the approval handler. The decision is returned for caching at a higher level.
func (a *ApprovalManager) RequestApproval(
	ctx context.Context,
	tool runtime.ToolRuntime,
	req *runtime.ToolRequest,
	sandboxAttempt *runtime.SandboxAttempt,
	isRetry bool,
	retryReason string,
) (runtime.ApprovalDecision, error) {
	// Build approval request
	approvalReq := &runtime.ApprovalRequest{
		CallID:           req.CallID,
		ToolName:         tool.Name(),
		WorkingDirectory: req.WorkingDirectory,
		IsRetry:          isRetry,
		RetryReason:      retryReason,
	}

	// Extract command if available (for shell/exec tools)
	if retryData := tool.SandboxRetryData(req); retryData != nil {
		approvalReq.Command = retryData.Command
	}

	// Extract justification from metadata
	if justification, ok := req.Metadata["justification"].(string); ok {
		approvalReq.Justification = justification
	}

	// Add risk assessment for retry approvals
	if isRetry {
		approvalReq.Risk = a.assessRisk(tool, req, sandboxAttempt)
	}

	// Call the approval handler
	decision, err := a.handler(ctx, approvalReq)
	if err != nil {
		return runtime.ApprovalDenied, &runtime.ToolError{
			Kind:    runtime.ErrorInternal,
			Message: fmt.Sprintf("approval handler error: %v", err),
			Cause:   err,
		}
	}

	return decision, nil
}

// CheckCachedApproval checks if there's a cached approval for the given tool and request.
func (a *ApprovalManager) CheckCachedApproval(tool runtime.ToolRuntime, req *runtime.ToolRequest) *runtime.ApprovalDecision {
	approvalKey := tool.ApprovalKey(req)
	return a.cache.Get(approvalKey)
}

// CacheApproval stores an approval decision in the cache.
func (a *ApprovalManager) CacheApproval(tool runtime.ToolRuntime, req *runtime.ToolRequest, decision runtime.ApprovalDecision) {
	approvalKey := tool.ApprovalKey(req)
	a.cache.Put(approvalKey, decision)
}

// assessRisk performs a security risk assessment for a command.
// This is used to help users make informed approval decisions.
func (a *ApprovalManager) assessRisk(
	tool runtime.ToolRuntime,
	req *runtime.ToolRequest,
	sandboxAttempt *runtime.SandboxAttempt,
) *runtime.RiskAssessment {
	assessment := &runtime.RiskAssessment{
		Level:   runtime.RiskMedium,
		Reasons: []string{},
	}

	// Get command if available
	retryData := tool.SandboxRetryData(req)
	if retryData == nil || len(retryData.Command) == 0 {
		assessment.Level = runtime.RiskLow
		return assessment
	}

	command := retryData.Command

	// Check for dangerous commands
	if runtime.IsDangerousCommand(command) {
		assessment.Level = runtime.RiskHigh
		assessment.Reasons = append(assessment.Reasons, "potentially destructive command")
	}

	// Check for system path access
	workDir := req.WorkingDirectory
	if len(workDir) > 0 && (workDir[0] == '/' && workDir != "/workspace") {
		if workDir == "/" || workDir == "/etc" || workDir == "/usr" || workDir == "/bin" {
			assessment.Level = runtime.RiskHigh
			assessment.Reasons = append(assessment.Reasons, "accesses system directories")
		}
	}

	// Check for network access requests
	if sandboxAttempt != nil && sandboxAttempt.NetworkEnabled {
		assessment.Reasons = append(assessment.Reasons, "network access enabled")
		if assessment.Level < runtime.RiskMedium {
			assessment.Level = runtime.RiskMedium
		}
	}

	// Check for write operations in arguments
	if containsWriteOperations(command) {
		assessment.Reasons = append(assessment.Reasons, "performs write operations")
		if assessment.Level < runtime.RiskMedium {
			assessment.Level = runtime.RiskMedium
		}
	}

	// Add mitigation information
	assessment.Mitigation = a.buildMitigation(assessment.Level)

	return assessment
}

// containsWriteOperations checks if a command contains write operations.
func containsWriteOperations(command []string) bool {
	if len(command) == 0 {
		return false
	}

	writeCommands := map[string]bool{
		"rm":      true,
		"mv":      true,
		"cp":      true,
		"mkdir":   true,
		"rmdir":   true,
		"touch":   true,
		"dd":      true,
		"tee":     true,
		"install": true,
	}

	program := command[0]
	if writeCommands[program] {
		return true
	}

	// Check for shell redirects
	for _, arg := range command {
		if len(arg) > 0 && (arg[0] == '>' || arg == ">>") {
			return true
		}
	}

	return false
}

// buildMitigation generates mitigation suggestions based on risk level.
func (a *ApprovalManager) buildMitigation(level runtime.RiskLevel) string {
	switch level {
	case runtime.RiskLow:
		return "This operation has minimal risk. It performs read-only actions."
	case runtime.RiskMedium:
		return "Sandbox restrictions would limit this operation to the workspace directory."
	case runtime.RiskHigh:
		return "Sandbox restrictions would prevent system modifications and network access."
	case runtime.RiskCritical:
		return "This operation could cause significant damage. Sandbox restrictions are strongly recommended."
	default:
		return "Sandbox restrictions provide protection against unintended modifications."
	}
}

// FormatApprovalRequest formats an approval request for display to the user.
// This is a helper function that tools can use to generate user-friendly prompts.
func FormatApprovalRequest(req *runtime.ApprovalRequest) string {
	var output string

	if req.IsRetry {
		output = fmt.Sprintf("Retry approval required for %s\n", req.ToolName)
		output += fmt.Sprintf("Reason: %s\n", req.RetryReason)
	} else {
		output = fmt.Sprintf("Approval required for %s\n", req.ToolName)
	}

	if len(req.Command) > 0 {
		output += fmt.Sprintf("Command: %v\n", req.Command)
	}

	output += fmt.Sprintf("Working directory: %s\n", req.WorkingDirectory)

	if req.Justification != "" {
		output += fmt.Sprintf("Justification: %s\n", req.Justification)
	}

	if req.Risk != nil {
		output += "\nRisk Assessment:\n"
		output += fmt.Sprintf("  Level: %s\n", formatRiskLevel(req.Risk.Level))
		if len(req.Risk.Reasons) > 0 {
			output += "  Concerns:\n"
			for _, reason := range req.Risk.Reasons {
				output += fmt.Sprintf("    - %s\n", reason)
			}
		}
		output += fmt.Sprintf("  Mitigation: %s\n", req.Risk.Mitigation)
	}

	return output
}

// formatRiskLevel converts a risk level to a human-readable string.
func formatRiskLevel(level runtime.RiskLevel) string {
	switch level {
	case runtime.RiskLow:
		return "Low"
	case runtime.RiskMedium:
		return "Medium"
	case runtime.RiskHigh:
		return "High"
	case runtime.RiskCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}

// ApprovalRequestToJSON serializes an approval request to JSON for logging/debugging.
func ApprovalRequestToJSON(req *runtime.ApprovalRequest) (string, error) {
	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ParseApprovalDecisionString converts a string to an ApprovalDecision.
// This is useful for parsing user input from CLI or API.
func ParseApprovalDecisionString(s string) (runtime.ApprovalDecision, error) {
	switch s {
	case "deny", "denied", "no":
		return runtime.ApprovalDenied, nil
	case "approve", "approved", "yes":
		return runtime.ApprovalApproved, nil
	case "approve_session", "always", "all":
		return runtime.ApprovalApprovedForSession, nil
	case "abort", "cancel":
		return runtime.ApprovalAbort, nil
	default:
		return runtime.ApprovalDenied, fmt.Errorf("invalid approval decision: %s", s)
	}
}
