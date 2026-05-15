// Package runtime provides the core abstraction for tool execution in Codex Go.
//
// The runtime package defines the ToolRuntime interface and supporting types that
// enable tools to execute under controlled conditions with approval workflows,
// sandboxing, streaming output, and error handling.
//
// Architecture:
//   - ToolRuntime: Core interface that all tools must implement
//   - ToolRegistry: Central registry for discovering and dispatching tools
//   - ToolContext: Execution context containing session, sandbox, and approval state
//   - Request/Response types: Standardized tool input/output
//   - Execution lifecycle: request → approval → sandbox selection → execution → streaming output
package runtime

import (
	"context"
	"io"
	"time"
)

// ToolRuntime defines the interface that all tool implementations must satisfy.
// It encapsulates the complete lifecycle of tool execution including approval,
// sandboxing, and output handling.
//
// A tool runtime is responsible for:
//   - Defining approval requirements (when user consent is needed)
//   - Declaring sandbox preferences (isolation requirements)
//   - Executing the tool logic under a given sandbox attempt
//   - Handling retry logic when sandbox restrictions cause failures
type ToolRuntime interface {
	// Name returns the unique identifier for this tool (e.g., "shell", "read_file").
	Name() string

	// Execute runs the tool with the given request under the specified execution context.
	// The context may be canceled if the user aborts or timeout occurs.
	// Returns a ToolResponse or error. Errors are wrapped as ToolError with context.
	Execute(ctx context.Context, req *ToolRequest, execCtx *ExecutionContext) (*ToolResponse, error)

	// ApprovalKey generates a unique key for caching approval decisions.
	// Tools with the same approval key (e.g., same command and working directory)
	// can reuse prior approval decisions within a session.
	ApprovalKey(req *ToolRequest) string

	// NeedsInitialApproval determines if approval is required before the first execution attempt.
	// Takes into account the approval policy, sandbox policy, and request characteristics
	// (e.g., known-safe commands might skip approval even with OnRequest policy).
	NeedsInitialApproval(req *ToolRequest, approvalPolicy ApprovalPolicy, sandboxPolicy SandboxPolicy) bool

	// NeedsRetryApproval determines if approval is required before retrying without sandbox
	// after a sandbox denial. Typically false for Never/OnRequest policies, true otherwise.
	NeedsRetryApproval(approvalPolicy ApprovalPolicy) bool

	// SandboxPreference indicates how this tool wants to interact with sandboxing.
	// Auto: use sandbox if available, can escalate on failure
	// Require: must run in sandbox
	// Forbid: must run without sandbox
	SandboxPreference() SandboxPreference

	// EscalateOnFailure returns true if the tool should retry without sandbox
	// when the initial sandboxed attempt fails with a permission denial.
	EscalateOnFailure() bool

	// WantsEscalatedFirstAttempt returns true if the request explicitly asks
	// for escalated permissions (bypass sandbox on first attempt).
	// Typically checks request-specific flags like "with_escalated_permissions".
	WantsEscalatedFirstAttempt(req *ToolRequest) bool

	// SupportsParallel returns true if multiple invocations of this tool
	// can safely execute concurrently.
	SupportsParallel() bool

	// SandboxRetryData extracts command metadata needed for re-running without sandbox.
	// Returns nil if this tool doesn't support sandbox retry (e.g., non-command tools).
	SandboxRetryData(req *ToolRequest) *SandboxRetryData
}

// ToolRegistry manages the collection of available tools and dispatches requests.
// It provides a central point for tool registration and discovery.
type ToolRegistry struct {
	tools map[string]ToolRuntime
}

// NewToolRegistry creates an empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]ToolRuntime),
	}
}

// Register adds a tool to the registry. If a tool with the same name
// already exists, it is overwritten.
func (r *ToolRegistry) Register(tool ToolRuntime) {
	r.tools[tool.Name()] = tool
}

// Get retrieves a tool by name. Returns nil if not found.
func (r *ToolRegistry) Get(name string) ToolRuntime {
	return r.tools[name]
}

// List returns all registered tool names in unspecified order.
func (r *ToolRegistry) List() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// ToolRequest represents a tool invocation request from the AI model.
// It contains the tool name, arguments, and execution context.
type ToolRequest struct {
	// CallID uniquely identifies this tool call within the conversation turn.
	CallID string

	// ToolName is the name of the tool to invoke (e.g., "shell", "read_file").
	ToolName string

	// Arguments contains the tool-specific parameters as JSON string.
	// Each tool runtime is responsible for parsing its expected schema.
	Arguments string

	// WorkingDirectory is the directory in which the tool should execute.
	// For filesystem tools, this is the base path. For shell commands, this is the cwd.
	WorkingDirectory string

	// Timeout specifies the maximum execution duration. Zero means no timeout.
	Timeout time.Duration

	// Environment contains additional environment variables for command execution.
	// Only applicable to shell/exec tools.
	Environment map[string]string

	// Metadata holds tool-specific metadata that doesn't fit standard fields.
	// For example: escalated_permissions flag, justification text, etc.
	Metadata map[string]interface{}
}

// ToolResponse encapsulates the result of tool execution.
// It includes the output content, success status, and execution metadata.
type ToolResponse struct {
	// Content is the primary output text returned to the AI model.
	// For commands, this is typically stdout/stderr. For file operations,
	// this is file content or operation results.
	Content string

	// Success indicates whether the tool executed successfully.
	// Even failed commands can be "successful" if failure was expected.
	// Nil means success status is unknown or N/A.
	Success *bool

	// ExitCode is the command exit code for shell/exec tools.
	// Nil for non-command tools.
	ExitCode *int

	// ExecutionTime tracks how long the tool took to execute.
	ExecutionTime time.Duration

	// StreamedOutput indicates whether output was streamed during execution.
	StreamedOutput bool

	// Metadata contains tool-specific response metadata.
	Metadata map[string]interface{}
}

// ExecutionContext provides the runtime context for tool execution.
// It encapsulates session state, sandbox configuration, approval state,
// and output streaming capabilities.
type ExecutionContext struct {
	// SessionID uniquely identifies the user session.
	SessionID string

	// TurnID uniquely identifies the conversation turn.
	TurnID string

	// SandboxAttempt describes the current sandbox configuration.
	SandboxAttempt *SandboxAttempt

	// ApprovalCache provides access to cached approval decisions.
	ApprovalCache ApprovalCache

	// OutputWriter is an optional streaming writer for tool output.
	// If non-nil, tools should write incremental output here for real-time display.
	OutputWriter io.Writer

	// StartTime records when execution began (for timeout tracking).
	StartTime time.Time

	// AlreadyApproved indicates that approval was already granted for this request
	// (either earlier in this turn or cached from a previous turn).
	AlreadyApproved bool
}

// ApprovalCache stores and retrieves approval decisions within a session.
// This enables tools to skip re-prompting for identical operations.
type ApprovalCache interface {
	// Get retrieves a cached approval decision by key.
	// Returns nil if no decision is cached.
	Get(key string) *ApprovalDecision

	// Put stores an approval decision for the given key.
	Put(key string, decision ApprovalDecision)
}

// ApprovalDecision represents a user's response to an approval request.
type ApprovalDecision int

const (
	// ApprovalDenied means the user rejected the operation.
	ApprovalDenied ApprovalDecision = iota

	// ApprovalApproved means the user approved this specific operation.
	ApprovalApproved

	// ApprovalApprovedForSession means the user approved this operation
	// and all similar operations for the remainder of the session.
	ApprovalApprovedForSession

	// ApprovalAbort means the user wants to abort the entire turn.
	ApprovalAbort
)

// ApprovalPolicy defines when user approval is required for tool execution.
type ApprovalPolicy int

const (
	// ApprovalNever means tools execute without approval prompts.
	// Sandboxing still applies based on SandboxPolicy.
	ApprovalNever ApprovalPolicy = iota

	// ApprovalOnFailure means approval is only requested when a sandboxed
	// attempt fails and needs to retry without sandbox.
	ApprovalOnFailure

	// ApprovalOnRequest means approval is requested before execution
	// unless running in DangerFullAccess mode or the operation is known safe.
	ApprovalOnRequest

	// ApprovalUnlessTrusted means approval is always requested unless
	// the operation is explicitly trusted/safe.
	ApprovalUnlessTrusted
)

// SandboxPolicy defines the system-wide sandboxing policy.
type SandboxPolicy int

const (
	// SandboxReadOnly allows read operations but blocks writes and network.
	SandboxReadOnly SandboxPolicy = iota

	// SandboxWorkspaceWrite allows writes within workspace but blocks network.
	SandboxWorkspaceWrite

	// SandboxDangerFullAccess disables sandboxing (for trusted environments).
	SandboxDangerFullAccess
)

// SandboxPreference indicates a tool's preferred sandbox behavior.
type SandboxPreference int

const (
	// SandboxAuto means use sandbox if available, escalate on failure if allowed.
	SandboxAuto SandboxPreference = iota

	// SandboxRequire means the tool must run in a sandbox.
	SandboxRequire

	// SandboxForbid means the tool must run without sandbox restrictions.
	SandboxForbid
)

// SandboxType describes the current sandbox configuration for an execution attempt.
type SandboxType int

const (
	// SandboxNone means no sandboxing is applied.
	SandboxNone SandboxType = iota

	// SandboxBubblewrap uses Linux bubblewrap for isolation.
	SandboxBubblewrap

	// SandboxDocker uses Docker container isolation.
	SandboxDocker

	// SandboxKubernetes uses Kubernetes pod isolation.
	SandboxKubernetes
)

// SandboxAttempt describes a specific execution attempt with sandbox configuration.
// It provides methods to transform command specs according to sandbox requirements.
type SandboxAttempt struct {
	// Type indicates which sandbox technology is in use (None, Bubblewrap, Docker).
	Type SandboxType

	// Policy is the system-wide sandbox policy.
	Policy SandboxPolicy

	// WorkingDirectory is the sandboxed working directory.
	WorkingDirectory string

	// SandboxRoot is the sandbox chroot/mount root (for bubblewrap/docker).
	SandboxRoot string

	// ReadOnlyPaths lists paths that should be mounted read-only.
	ReadOnlyPaths []string

	// ReadWritePaths lists paths that should be mounted read-write.
	ReadWritePaths []string

	// NetworkEnabled indicates if network access is permitted.
	NetworkEnabled bool
}

// SandboxRetryData captures the information needed to retry a command without sandbox.
// This is extracted from the tool request after a sandbox denial.
type SandboxRetryData struct {
	// Command is the full command array (program + args).
	Command []string

	// WorkingDirectory is where the command should execute.
	WorkingDirectory string
}

// ToolError represents an error that occurred during tool execution.
// It provides structured error information for proper handling and user display.
type ToolError struct {
	// Kind categorizes the error type.
	Kind ToolErrorKind

	// Message is the human-readable error description.
	Message string

	// Cause is the underlying error, if any.
	Cause error
}

// Error implements the error interface.
func (e *ToolError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Unwrap returns the underlying cause for error wrapping.
func (e *ToolError) Unwrap() error {
	return e.Cause
}

// ToolErrorKind categorizes different types of tool execution errors.
type ToolErrorKind int

const (
	// ErrorRejected means the user rejected the operation during approval.
	ErrorRejected ToolErrorKind = iota

	// ErrorSandboxDenied means the sandbox blocked the operation.
	ErrorSandboxDenied

	// ErrorTimeout means the operation exceeded its time limit.
	ErrorTimeout

	// ErrorExecution means the tool itself failed (command error, I/O error, etc.).
	ErrorExecution

	// ErrorInvalidArguments means the tool arguments were malformed.
	ErrorInvalidArguments

	// ErrorInternal means an internal error occurred in the runtime.
	ErrorInternal
)

// NewToolError creates a ToolError with the specified kind and message.
func NewToolError(kind ToolErrorKind, message string) *ToolError {
	return &ToolError{
		Kind:    kind,
		Message: message,
	}
}

// NewToolErrorWithCause creates a ToolError wrapping an underlying error.
func NewToolErrorWithCause(kind ToolErrorKind, message string, cause error) *ToolError {
	return &ToolError{
		Kind:    kind,
		Message: message,
		Cause:   cause,
	}
}
