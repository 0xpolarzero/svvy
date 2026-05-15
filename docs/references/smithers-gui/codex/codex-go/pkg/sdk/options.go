package sdk

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
	"github.com/evmts/codex/codex-go/pkg/sdk/client"
)

// Valid approval policy values
const (
	ApprovalPolicyAuto   = "auto"
	ApprovalPolicyManual = "manual"
	ApprovalPolicyNever  = "never"
)

// Valid sandbox policy mode values
const (
	SandboxPolicyOff              = "off"
	SandboxPolicyReadOnly         = "read-only"
	SandboxPolicyWorkspaceWrite   = "workspace-write"
	SandboxPolicyNative           = "native"
	SandboxPolicyDangerFullAccess = "danger-full-access"
)

// Options configures the SDK.
type Options struct {
	// Client is the OpenAI-compatible client for API requests.
	// Use client.New(), client.FromEnv(), or client.FromConfig() to create one.
	Client *client.Client

	// Tools is the list of tools available to the agent.
	// If nil, a default set of tools will be registered.
	Tools []runtime.ToolRuntime

	// ToolRegistry allows providing a pre-configured tool registry.
	// If provided, Tools will be ignored.
	ToolRegistry *runtime.ToolRegistry

	// EnableHistory enables conversation history persistence.
	// When true, conversations will be saved to disk.
	EnableHistory bool

	// HistoryPath is the path where conversation history is stored.
	// Only used when EnableHistory is true.
	// Defaults to ~/.codex/history.jsonl
	HistoryPath string
}

// SessionOptions configures a conversation session.
type SessionOptions struct {
	// SystemPrompt is the initial system message for the session.
	SystemPrompt string

	// Streaming enables streaming responses.
	// When true, use SubmitStream() instead of Submit().
	Streaming bool

	// OnToolApproval is called when a tool requires user approval.
	// If nil, tools will be auto-approved based on the approval policy.
	// Return true to approve, false to deny.
	OnToolApproval func(toolName, operation string) bool

	// ApprovalPolicy controls when tools require approval.
	// Valid values: "auto" (default), "manual", "never"
	// Use the ApprovalPolicy* constants for type safety.
	ApprovalPolicy string

	// SandboxPolicy controls tool sandboxing.
	// Valid values: "off", "read-only", "workspace-write", "native" (default), "danger-full-access"
	// Use the SandboxPolicy* constants for type safety.
	SandboxPolicy string

	// WorkingDirectory is the initial working directory for the session.
	WorkingDirectory string

	// Model overrides the default model for this session.
	Model string

	// ConversationID for resuming existing conversations.
	// If empty, a new conversation is started.
	ConversationID string
}

// Validate checks if the Options are valid and returns an error if not.
func (o *Options) Validate() error {
	if o.Client == nil {
		return fmt.Errorf("client is required")
	}

	// Validate tool configuration - both Tools and ToolRegistry should not be set
	if o.ToolRegistry != nil && len(o.Tools) > 0 {
		return fmt.Errorf("cannot specify both ToolRegistry and Tools; use one or the other")
	}

	// Validate history configuration
	if o.EnableHistory && o.HistoryPath != "" {
		if err := validatePath(o.HistoryPath, "history path"); err != nil {
			return err
		}
	}

	return nil
}

// Validate checks if the SessionOptions are valid and returns an error if not.
func (so *SessionOptions) Validate() error {
	// Validate ApprovalPolicy if provided
	if so.ApprovalPolicy != "" {
		if err := validateApprovalPolicy(so.ApprovalPolicy); err != nil {
			return err
		}

		// If approval policy is manual, OnToolApproval callback is required
		if so.ApprovalPolicy == ApprovalPolicyManual && so.OnToolApproval == nil {
			return fmt.Errorf("OnToolApproval callback is required when ApprovalPolicy is 'manual'")
		}
	}

	// Validate SandboxPolicy if provided
	if so.SandboxPolicy != "" {
		if err := validateSandboxPolicy(so.SandboxPolicy); err != nil {
			return err
		}
	}

	// Validate WorkingDirectory if provided
	if so.WorkingDirectory != "" {
		if !filepath.IsAbs(so.WorkingDirectory) {
			return fmt.Errorf("WorkingDirectory must be an absolute path, got: %s", so.WorkingDirectory)
		}

		// Check if it exists and is a directory
		info, err := os.Stat(so.WorkingDirectory)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("WorkingDirectory does not exist: %s", so.WorkingDirectory)
			}
			return fmt.Errorf("WorkingDirectory validation failed: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("WorkingDirectory is not a directory: %s", so.WorkingDirectory)
		}

		// Security check: prevent using dangerous system directories
		if err := validateNotSystemDirectory(so.WorkingDirectory, "WorkingDirectory"); err != nil {
			return err
		}
	}

	// Validate Model if provided
	if so.Model != "" {
		if err := validateModel(so.Model); err != nil {
			return err
		}
	}

	// Validate SystemPrompt length is reasonable (prevent extremely large prompts)
	if len(so.SystemPrompt) > 100000 {
		return fmt.Errorf("SystemPrompt is too large: %d characters (max 100000)", len(so.SystemPrompt))
	}

	return nil
}

// validateApprovalPolicy checks if the approval policy string is valid.
func validateApprovalPolicy(policy string) error {
	validPolicies := map[string]bool{
		ApprovalPolicyAuto:   true,
		ApprovalPolicyManual: true,
		ApprovalPolicyNever:  true,
	}

	if !validPolicies[policy] {
		return fmt.Errorf("invalid ApprovalPolicy: %q (valid values: %q, %q, %q)",
			policy, ApprovalPolicyAuto, ApprovalPolicyManual, ApprovalPolicyNever)
	}

	return nil
}

// validateSandboxPolicy checks if the sandbox policy string is valid.
func validateSandboxPolicy(policy string) error {
	validPolicies := map[string]bool{
		SandboxPolicyOff:              true,
		SandboxPolicyReadOnly:         true,
		SandboxPolicyWorkspaceWrite:   true,
		SandboxPolicyNative:           true,
		SandboxPolicyDangerFullAccess: true,
	}

	if !validPolicies[policy] {
		return fmt.Errorf("invalid SandboxPolicy: %q (valid values: %q, %q, %q, %q, %q)",
			policy, SandboxPolicyOff, SandboxPolicyReadOnly, SandboxPolicyWorkspaceWrite,
			SandboxPolicyNative, SandboxPolicyDangerFullAccess)
	}

	return nil
}

// validateModel checks if the model name is reasonable.
// This performs basic format validation without checking if the model exists.
func validateModel(model string) error {
	if model == "" {
		return fmt.Errorf("model cannot be empty")
	}

	// Basic sanity check: model should be alphanumeric with hyphens, dots, or underscores
	// Examples: gpt-4, gpt-5-codex, claude-3-opus-20240229
	for _, ch := range model {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '-' || ch == '.' || ch == '_') {
			return fmt.Errorf("model name contains invalid character: %q in %q", ch, model)
		}
	}

	// Prevent excessively long model names
	if len(model) > 100 {
		return fmt.Errorf("model name is too long: %d characters (max 100)", len(model))
	}

	return nil
}

// validatePath checks if a path is valid and safe.
func validatePath(path, pathType string) error {
	if path == "" {
		return fmt.Errorf("%s cannot be empty", pathType)
	}

	// Must be absolute path
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%s must be an absolute path, got: %s", pathType, path)
	}

	// Security check: prevent using dangerous system directories
	if err := validateNotSystemDirectory(path, pathType); err != nil {
		return err
	}

	// Check if parent directory exists
	dir := filepath.Dir(path)
	if info, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s parent directory does not exist: %s", pathType, dir)
		}
		return fmt.Errorf("%s parent directory validation failed: %w", pathType, err)
	} else if !info.IsDir() {
		return fmt.Errorf("%s parent is not a directory: %s", pathType, dir)
	}

	// Check write permissions by attempting to create a test file
	testFile := filepath.Join(dir, fmt.Sprintf(".codex_test_write_%d", os.Getpid()))
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("%s directory is not writable: %w", pathType, err)
	}
	f.Close()
	os.Remove(testFile)

	return nil
}

// validateNotSystemDirectory prevents using dangerous system directories.
func validateNotSystemDirectory(path, pathType string) error {
	// Normalize path for comparison
	path = filepath.Clean(path)

	// List of dangerous directories
	dangerousDirs := []string{
		"/",
		"/bin",
		"/boot",
		"/dev",
		"/etc",
		"/lib",
		"/lib64",
		"/proc",
		"/root",
		"/sbin",
		"/sys",
		"/usr",
		"/var",
	}

	for _, dangerousDir := range dangerousDirs {
		if path == dangerousDir || strings.HasPrefix(path, dangerousDir+string(filepath.Separator)) {
			return fmt.Errorf("%s cannot be in system directory %s: %s", pathType, dangerousDir, path)
		}
	}

	return nil
}
