package state

import (
	"fmt"
	"sync"
	"time"
)

// PolicyOptions configures policy enforcement behavior.
type PolicyOptions struct {
	RequireToolApproval  bool
	MaxTokensPerTurn     int64
	MaxMessagesInHistory int
	AllowedTools         []string
	BlockedTools         []string
	DangerousTools       []string
}

// Policy defines conversation policy constraints.
type Policy struct {
	RequireToolApproval  bool
	MaxTokensPerTurn     int64
	MaxMessagesInHistory int
	AllowedTools         []string
	BlockedTools         []string
	DangerousTools       []string
	customValidators     []func(string, map[string]interface{}) error
}

// NewPolicy creates a new policy with default settings.
func NewPolicy() *Policy {
	return &Policy{
		RequireToolApproval:  true,
		MaxTokensPerTurn:     100000,
		MaxMessagesInHistory: 100,
		AllowedTools:         []string{},
		BlockedTools:         []string{},
		DangerousTools:       []string{},
		customValidators:     make([]func(string, map[string]interface{}) error, 0),
	}
}

// NewPolicyWithOptions creates a new policy with custom options.
func NewPolicyWithOptions(opts PolicyOptions) *Policy {
	return &Policy{
		RequireToolApproval:  opts.RequireToolApproval,
		MaxTokensPerTurn:     opts.MaxTokensPerTurn,
		MaxMessagesInHistory: opts.MaxMessagesInHistory,
		AllowedTools:         opts.AllowedTools,
		BlockedTools:         opts.BlockedTools,
		DangerousTools:       opts.DangerousTools,
		customValidators:     make([]func(string, map[string]interface{}) error, 0),
	}
}

// ValidateToolCall validates a tool call against policy constraints.
func (p *Policy) ValidateToolCall(toolName string, args map[string]interface{}) error {
	// Check blocked tools first
	for _, blocked := range p.BlockedTools {
		if toolName == blocked {
			return fmt.Errorf("tool %s is blocked by policy", toolName)
		}
	}

	// Check allowed tools if list is not empty
	if len(p.AllowedTools) > 0 {
		allowed := false
		for _, allowedTool := range p.AllowedTools {
			if toolName == allowedTool {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("tool %s is not in allowed list", toolName)
		}
	}

	// Run custom validators
	for _, validator := range p.customValidators {
		if err := validator(toolName, args); err != nil {
			return err
		}
	}

	return nil
}

// ValidateTokenUsage validates token usage against policy limits.
func (p *Policy) ValidateTokenUsage(usage TokenUsage) error {
	if p.MaxTokensPerTurn <= 0 {
		return nil // No limit
	}

	if usage.TotalTokens > p.MaxTokensPerTurn {
		return fmt.Errorf("token usage %d exceeds maximum %d", usage.TotalTokens, p.MaxTokensPerTurn)
	}

	return nil
}

// ValidateMessageHistory validates message history against policy limits.
func (p *Policy) ValidateMessageHistory(history *MessageHistory) error {
	if p.MaxMessagesInHistory <= 0 {
		return nil // No limit
	}

	count := history.Count()
	if count > p.MaxMessagesInHistory {
		return fmt.Errorf("message history count %d exceeds maximum %d", count, p.MaxMessagesInHistory)
	}

	return nil
}

// ShouldApproveToolCall determines if a tool call requires approval.
func (p *Policy) ShouldApproveToolCall(toolName string) bool {
	// Always require approval for dangerous tools
	for _, dangerous := range p.DangerousTools {
		if toolName == dangerous {
			return true
		}
	}

	return p.RequireToolApproval
}

// AddCustomValidator adds a custom validation function.
func (p *Policy) AddCustomValidator(validator func(string, map[string]interface{}) error) {
	p.customValidators = append(p.customValidators, validator)
}

// Clone creates a deep copy of the policy.
func (p *Policy) Clone() *Policy {
	allowedTools := make([]string, len(p.AllowedTools))
	copy(allowedTools, p.AllowedTools)

	blockedTools := make([]string, len(p.BlockedTools))
	copy(blockedTools, p.BlockedTools)

	dangerousTools := make([]string, len(p.DangerousTools))
	copy(dangerousTools, p.DangerousTools)

	return &Policy{
		RequireToolApproval:  p.RequireToolApproval,
		MaxTokensPerTurn:     p.MaxTokensPerTurn,
		MaxMessagesInHistory: p.MaxMessagesInHistory,
		AllowedTools:         allowedTools,
		BlockedTools:         blockedTools,
		DangerousTools:       dangerousTools,
		customValidators:     p.customValidators, // Share validators (functions are immutable)
	}
}

// PolicyViolation represents a policy violation.
type PolicyViolation struct {
	Type      string
	Message   string
	Severity  string
	Details   map[string]interface{}
	Timestamp time.Time
}

// PolicyEnforcer enforces policy rules and tracks violations.
type PolicyEnforcer struct {
	mu         sync.RWMutex
	policy     *Policy
	violations []PolicyViolation
}

// NewPolicyEnforcer creates a new policy enforcer.
func NewPolicyEnforcer(policy *Policy) *PolicyEnforcer {
	return &PolicyEnforcer{
		policy:     policy,
		violations: make([]PolicyViolation, 0),
	}
}

// Policy returns the enforcer's policy.
func (e *PolicyEnforcer) Policy() *Policy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.policy
}

// EnforceToolCall enforces tool call policy.
func (e *PolicyEnforcer) EnforceToolCall(toolName string, args map[string]interface{}) error {
	err := e.policy.ValidateToolCall(toolName, args)
	if err != nil {
		e.recordViolation(PolicyViolation{
			Type:     "tool_call",
			Message:  err.Error(),
			Severity: "error",
			Details: map[string]interface{}{
				"tool": toolName,
				"args": args,
			},
			Timestamp: time.Now(),
		})
		return err
	}
	return nil
}

// EnforceTokenUsage enforces token usage policy.
func (e *PolicyEnforcer) EnforceTokenUsage(usage TokenUsage) error {
	err := e.policy.ValidateTokenUsage(usage)
	if err != nil {
		e.recordViolation(PolicyViolation{
			Type:     "token_limit",
			Message:  err.Error(),
			Severity: "error",
			Details: map[string]interface{}{
				"used":  usage.TotalTokens,
				"limit": e.policy.MaxTokensPerTurn,
			},
			Timestamp: time.Now(),
		})
		return err
	}
	return nil
}

// EnforceMessageHistory enforces message history policy.
func (e *PolicyEnforcer) EnforceMessageHistory(history *MessageHistory) error {
	err := e.policy.ValidateMessageHistory(history)
	if err != nil {
		e.recordViolation(PolicyViolation{
			Type:     "message_history",
			Message:  err.Error(),
			Severity: "error",
			Details: map[string]interface{}{
				"count": history.Count(),
				"limit": e.policy.MaxMessagesInHistory,
			},
			Timestamp: time.Now(),
		})
		return err
	}
	return nil
}

// recordViolation records a policy violation.
func (e *PolicyEnforcer) recordViolation(violation PolicyViolation) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.violations = append(e.violations, violation)
}

// Violations returns all recorded violations.
func (e *PolicyEnforcer) Violations() []PolicyViolation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	violations := make([]PolicyViolation, len(e.violations))
	copy(violations, e.violations)
	return violations
}

// ClearViolations clears all recorded violations.
func (e *PolicyEnforcer) ClearViolations() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.violations = make([]PolicyViolation, 0)
}
