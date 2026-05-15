// Package protocol defines the core protocol types for Codex sessions.
//
// This package provides the types and interfaces for communication between
// a Codex client and agent using a Submission Queue (SQ) / Event Queue (EQ) pattern.
// The protocol enables asynchronous communication for AI-assisted coding sessions.
package protocol

import (
	"encoding/json"
	"fmt"
)

// Constants for user input parsing
const (
	UserInstructionsOpenTag    = "<user_instructions>"
	UserInstructionsCloseTag   = "</user_instructions>"
	EnvironmentContextOpenTag  = "<environment_context>"
	EnvironmentContextCloseTag = "</environment_context>"
	UserMessageBegin           = "## My request for Codex:"
)

// Constants for UserInput types
const (
	UserInputTypeText     = "text"
	UserInputTypeImageURL = "image_url"
	UserInputTypePath     = "path"
)

// Decision values for approval responses
const (
	DecisionApproved           = "approved"
	DecisionApprovedForSession = "approved_for_session"
	DecisionDenied             = "denied"
	DecisionAbort              = "abort"
)

// ApprovalPolicy values
const (
	ApprovalPolicyAuto   = "auto"
	ApprovalPolicyAlways = "always"
	ApprovalPolicyNever  = "never"
)

// SandboxPolicy mode values
const (
	SandboxModeReadOnly         = "read-only"
	SandboxModeWorkspaceWrite   = "workspace-write"
	SandboxModeDangerFullAccess = "danger-full-access"
)

// Summary values
const (
	SummaryNone   = "none"
	SummaryLow    = "low"
	SummaryMedium = "medium"
	SummaryHigh   = "high"
)

// Submission represents a request from the user to the agent.
// It wraps an Op with a unique ID for correlation with events.
type Submission struct {
	// ID is the unique identifier for this submission to correlate with events
	ID string `json:"id"`
	// Op is the operation to perform
	Op Op `json:"op"`
}

// Op represents a submission operation from the user.
// This is the primary interface for all user requests.
type Op interface {
	OpType() string
	isOp()
}

// Op type implementations

// OpInterrupt aborts the current task.
// The server sends an EventMsg of type TurnAborted in response.
type OpInterrupt struct{}

func (o *OpInterrupt) OpType() string { return "interrupt" }
func (o *OpInterrupt) isOp()          {}
func (o *OpInterrupt) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": "interrupt",
	})
}

// OpUserInput represents input from the user.
type OpUserInput struct {
	Items []UserInput `json:"items"`
}

func (o *OpUserInput) OpType() string { return "user_input" }
func (o *OpUserInput) isOp()          {}
func (o *OpUserInput) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":  "user_input",
		"items": o.Items,
	})
}

// OpUserTurn is similar to OpUserInput but contains additional context
// required for a turn of a Codex conversation.
type OpUserTurn struct {
	// Items contains user input items
	Items []UserInput `json:"items"`
	// Cwd is the working directory to use with the sandbox policy and tool calls
	Cwd string `json:"cwd"`
	// ApprovalPolicy determines when to ask for command approval
	ApprovalPolicy string `json:"approval_policy"`
	// SandboxPolicy determines execution restrictions for model shell commands
	SandboxPolicy SandboxPolicy `json:"sandbox_policy"`
	// Model is a valid model slug for the model client
	Model string `json:"model"`
	// Effort is the reasoning effort (only honored for reasoning-capable models)
	Effort *string `json:"effort,omitempty"`
	// Summary controls reasoning summary preference
	Summary string `json:"summary"`
	// FinalOutputJSONSchema is the JSON schema for the final assistant message
	FinalOutputJSONSchema interface{} `json:"final_output_json_schema,omitempty"`
}

func (o *OpUserTurn) OpType() string { return "user_turn" }
func (o *OpUserTurn) isOp()          {}
func (o *OpUserTurn) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"type":            "user_turn",
		"items":           o.Items,
		"cwd":             o.Cwd,
		"approval_policy": o.ApprovalPolicy,
		"sandbox_policy":  o.SandboxPolicy,
		"model":           o.Model,
		"summary":         o.Summary,
	}
	if o.Effort != nil {
		m["effort"] = *o.Effort
	}
	if o.FinalOutputJSONSchema != nil {
		m["final_output_json_schema"] = o.FinalOutputJSONSchema
	}
	return json.Marshal(m)
}

// Validate checks if the OpUserTurn is valid
func (o *OpUserTurn) Validate() error {
	if len(o.Items) == 0 {
		return fmt.Errorf("items cannot be empty")
	}
	if len(o.Items) > 100 {
		return fmt.Errorf("items exceeds maximum of 100")
	}
	for i, item := range o.Items {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("item %d: %w", i, err)
		}
	}
	if o.Cwd == "" {
		return fmt.Errorf("cwd cannot be empty")
	}
	if o.ApprovalPolicy != "" && o.ApprovalPolicy != ApprovalPolicyAuto &&
		o.ApprovalPolicy != ApprovalPolicyAlways && o.ApprovalPolicy != ApprovalPolicyNever {
		return fmt.Errorf("invalid approval_policy: %s", o.ApprovalPolicy)
	}
	if err := o.SandboxPolicy.Validate(); err != nil {
		return fmt.Errorf("sandbox_policy: %w", err)
	}
	if o.Model == "" {
		return fmt.Errorf("model cannot be empty")
	}
	if o.Summary != "" && o.Summary != SummaryNone && o.Summary != SummaryLow &&
		o.Summary != SummaryMedium && o.Summary != SummaryHigh {
		return fmt.Errorf("invalid summary: %s", o.Summary)
	}
	return nil
}

// OpOverrideTurnContext overrides parts of the persistent turn context for subsequent turns.
// All fields are optional; when omitted, the existing value is preserved.
type OpOverrideTurnContext struct {
	Cwd            *string        `json:"cwd,omitempty"`
	ApprovalPolicy *string        `json:"approval_policy,omitempty"`
	SandboxPolicy  *SandboxPolicy `json:"sandbox_policy,omitempty"`
	Model          *string        `json:"model,omitempty"`
	Effort         *string        `json:"effort,omitempty"`
	Summary        *string        `json:"summary,omitempty"`
}

func (o *OpOverrideTurnContext) OpType() string { return "override_turn_context" }
func (o *OpOverrideTurnContext) isOp()          {}
func (o *OpOverrideTurnContext) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":            "override_turn_context",
		"cwd":             o.Cwd,
		"approval_policy": o.ApprovalPolicy,
		"sandbox_policy":  o.SandboxPolicy,
		"model":           o.Model,
		"effort":          o.Effort,
		"summary":         o.Summary,
	})
}

// OpExecApproval approves a command execution.
type OpExecApproval struct {
	// ID is the submission ID being approved
	ID string `json:"id"`
	// Decision is the user's decision in response to the request
	Decision string `json:"decision"`
}

func (o *OpExecApproval) OpType() string { return "exec_approval" }
func (o *OpExecApproval) isOp()          {}
func (o *OpExecApproval) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":     "exec_approval",
		"id":       o.ID,
		"decision": o.Decision,
	})
}

// Validate checks if the OpExecApproval is valid
func (o *OpExecApproval) Validate() error {
	if o.ID == "" {
		return fmt.Errorf("id cannot be empty")
	}
	if o.Decision != DecisionApproved && o.Decision != DecisionApprovedForSession &&
		o.Decision != DecisionDenied && o.Decision != DecisionAbort {
		return fmt.Errorf("invalid decision: %s", o.Decision)
	}
	return nil
}

// OpPatchApproval approves a code patch.
type OpPatchApproval struct {
	// ID is the submission ID being approved
	ID string `json:"id"`
	// Decision is the user's decision in response to the request
	Decision string `json:"decision"`
}

func (o *OpPatchApproval) OpType() string { return "patch_approval" }
func (o *OpPatchApproval) isOp()          {}
func (o *OpPatchApproval) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":     "patch_approval",
		"id":       o.ID,
		"decision": o.Decision,
	})
}

// Validate checks if the OpPatchApproval is valid
func (o *OpPatchApproval) Validate() error {
	if o.ID == "" {
		return fmt.Errorf("id cannot be empty")
	}
	if o.Decision != DecisionApproved && o.Decision != DecisionApprovedForSession &&
		o.Decision != DecisionDenied && o.Decision != DecisionAbort {
		return fmt.Errorf("invalid decision: %s", o.Decision)
	}
	return nil
}

// OpAddToHistory appends an entry to the persistent cross-session message history.
type OpAddToHistory struct {
	Text string `json:"text"`
}

func (o *OpAddToHistory) OpType() string { return "add_to_history" }
func (o *OpAddToHistory) isOp()          {}
func (o *OpAddToHistory) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": "add_to_history",
		"text": o.Text,
	})
}

// OpGetHistoryEntryRequest requests a single history entry.
type OpGetHistoryEntryRequest struct {
	Offset uint64 `json:"offset"`
	LogID  uint64 `json:"log_id"`
}

func (o *OpGetHistoryEntryRequest) OpType() string { return "get_history_entry_request" }
func (o *OpGetHistoryEntryRequest) isOp()          {}
func (o *OpGetHistoryEntryRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":   "get_history_entry_request",
		"offset": o.Offset,
		"log_id": o.LogID,
	})
}

// OpGetPath requests the full in-memory conversation transcript.
type OpGetPath struct{}

func (o *OpGetPath) OpType() string { return "get_path" }
func (o *OpGetPath) isOp()          {}
func (o *OpGetPath) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": "get_path",
	})
}

// OpListMcpTools requests the list of MCP tools available.
type OpListMcpTools struct{}

func (o *OpListMcpTools) OpType() string { return "list_mcp_tools" }
func (o *OpListMcpTools) isOp()          {}
func (o *OpListMcpTools) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": "list_mcp_tools",
	})
}

// OpListCustomPrompts requests the list of available custom prompts.
type OpListCustomPrompts struct{}

func (o *OpListCustomPrompts) OpType() string { return "list_custom_prompts" }
func (o *OpListCustomPrompts) isOp()          {}
func (o *OpListCustomPrompts) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": "list_custom_prompts",
	})
}

// OpCompact requests the agent to summarize the current conversation context.
type OpCompact struct{}

func (o *OpCompact) OpType() string { return "compact" }
func (o *OpCompact) isOp()          {}
func (o *OpCompact) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": "compact",
	})
}

// OpReview requests a code review from the agent.
type OpReview struct {
	ReviewRequest ReviewRequest `json:"review_request"`
}

func (o *OpReview) OpType() string { return "review" }
func (o *OpReview) isOp()          {}
func (o *OpReview) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":           "review",
		"review_request": o.ReviewRequest,
	})
}

// OpShutdown requests to shut down the codex instance.
type OpShutdown struct{}

func (o *OpShutdown) OpType() string { return "shutdown" }
func (o *OpShutdown) isOp()          {}
func (o *OpShutdown) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": "shutdown",
	})
}

// UnmarshalJSON implements custom unmarshaling for Submission
func (s *Submission) UnmarshalJSON(data []byte) error {
	var temp struct {
		ID string          `json:"id"`
		Op json.RawMessage `json:"op"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	s.ID = temp.ID

	// Parse the op based on its type field
	var typeCheck struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(temp.Op, &typeCheck); err != nil {
		return err
	}

	op, err := unmarshalOp(typeCheck.Type, temp.Op)
	if err != nil {
		return fmt.Errorf("failed to unmarshal op of type %s: %w", typeCheck.Type, err)
	}
	s.Op = op
	return nil
}

func unmarshalOp(opType string, data []byte) (Op, error) {
	switch opType {
	case "interrupt":
		return &OpInterrupt{}, nil
	case "user_input":
		var op OpUserInput
		if err := json.Unmarshal(data, &op); err != nil {
			return nil, err
		}
		return &op, nil
	case "user_turn":
		var op OpUserTurn
		if err := json.Unmarshal(data, &op); err != nil {
			return nil, err
		}
		return &op, nil
	case "override_turn_context":
		var op OpOverrideTurnContext
		if err := json.Unmarshal(data, &op); err != nil {
			return nil, err
		}
		return &op, nil
	case "exec_approval":
		var op OpExecApproval
		if err := json.Unmarshal(data, &op); err != nil {
			return nil, err
		}
		return &op, nil
	case "patch_approval":
		var op OpPatchApproval
		if err := json.Unmarshal(data, &op); err != nil {
			return nil, err
		}
		return &op, nil
	case "add_to_history":
		var op OpAddToHistory
		if err := json.Unmarshal(data, &op); err != nil {
			return nil, err
		}
		return &op, nil
	case "get_history_entry_request":
		var op OpGetHistoryEntryRequest
		if err := json.Unmarshal(data, &op); err != nil {
			return nil, err
		}
		return &op, nil
	case "get_path":
		return &OpGetPath{}, nil
	case "list_mcp_tools":
		return &OpListMcpTools{}, nil
	case "list_custom_prompts":
		return &OpListCustomPrompts{}, nil
	case "compact":
		return &OpCompact{}, nil
	case "review":
		var op OpReview
		if err := json.Unmarshal(data, &op); err != nil {
			return nil, err
		}
		return &op, nil
	case "shutdown":
		return &OpShutdown{}, nil
	default:
		return nil, fmt.Errorf("unknown op type: %s", opType)
	}
}

// Event represents an event from the agent to the client.
type Event struct {
	// ID is the submission ID that this event is correlated with
	ID string `json:"id"`
	// Msg is the event message payload
	Msg EventMsg `json:"msg"`
}

// EventMsg is the interface for all event message types.
type EventMsg interface {
	EventType() string
	isEventMsg()
}

// Event type implementations

// EventError represents an error while executing a submission.
type EventError struct {
	Message string `json:"message"`
}

func (e *EventError) EventType() string { return "error" }
func (e *EventError) isEventMsg()       {}
func (e *EventError) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":    "error",
		"message": e.Message,
	})
}

// EventTaskStarted indicates the agent has started a task.
type EventTaskStarted struct {
	ModelContextWindow *int64 `json:"model_context_window,omitempty"`
}

func (e *EventTaskStarted) EventType() string { return "task_started" }
func (e *EventTaskStarted) isEventMsg()       {}
func (e *EventTaskStarted) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"type": "task_started",
	}
	if e.ModelContextWindow != nil {
		m["model_context_window"] = *e.ModelContextWindow
	}
	return json.Marshal(m)
}

// EventTaskComplete indicates the agent has completed all actions.
type EventTaskComplete struct {
	LastAgentMessage *string `json:"last_agent_message,omitempty"`
}

func (e *EventTaskComplete) EventType() string { return "task_complete" }
func (e *EventTaskComplete) isEventMsg()       {}
func (e *EventTaskComplete) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"type": "task_complete",
	}
	if e.LastAgentMessage != nil {
		m["last_agent_message"] = *e.LastAgentMessage
	}
	return json.Marshal(m)
}

// EventTokenCount provides usage updates for the current session.
type EventTokenCount struct {
	Info       *TokenUsageInfo    `json:"info,omitempty"`
	RateLimits *RateLimitSnapshot `json:"rate_limits,omitempty"`
}

func (e *EventTokenCount) EventType() string { return "token_count" }
func (e *EventTokenCount) isEventMsg()       {}
func (e *EventTokenCount) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":        "token_count",
		"info":        e.Info,
		"rate_limits": e.RateLimits,
	})
}

// EventAgentMessage represents agent text output.
type EventAgentMessage struct {
	Message string `json:"message"`
}

func (e *EventAgentMessage) EventType() string { return "agent_message" }
func (e *EventAgentMessage) isEventMsg()       {}
func (e *EventAgentMessage) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":    "agent_message",
		"message": e.Message,
	})
}

// EventUserMessage represents user/system input message.
type EventUserMessage struct {
	Message string    `json:"message"`
	Images  *[]string `json:"images,omitempty"`
}

func (e *EventUserMessage) EventType() string { return "user_message" }
func (e *EventUserMessage) isEventMsg()       {}
func (e *EventUserMessage) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"type":    "user_message",
		"message": e.Message,
	}
	if e.Images != nil {
		m["images"] = *e.Images
	}
	return json.Marshal(m)
}

// EventAgentMessageDelta represents an agent text output delta.
type EventAgentMessageDelta struct {
	Delta string `json:"delta"`
}

func (e *EventAgentMessageDelta) EventType() string { return "agent_message_delta" }
func (e *EventAgentMessageDelta) isEventMsg()       {}
func (e *EventAgentMessageDelta) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":  "agent_message_delta",
		"delta": e.Delta,
	})
}

// EventAgentReasoning represents reasoning output from the agent.
type EventAgentReasoning struct {
	Text string `json:"text"`
}

func (e *EventAgentReasoning) EventType() string { return "agent_reasoning" }
func (e *EventAgentReasoning) isEventMsg()       {}
func (e *EventAgentReasoning) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": "agent_reasoning",
		"text": e.Text,
	})
}

// EventAgentReasoningDelta represents a reasoning delta from the agent.
type EventAgentReasoningDelta struct {
	Delta string `json:"delta"`
}

func (e *EventAgentReasoningDelta) EventType() string { return "agent_reasoning_delta" }
func (e *EventAgentReasoningDelta) isEventMsg()       {}
func (e *EventAgentReasoningDelta) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":  "agent_reasoning_delta",
		"delta": e.Delta,
	})
}

// EventExecCommandBegin indicates the server is about to execute a command.
type EventExecCommandBegin struct {
	CallID    string        `json:"call_id"`
	Command   []string      `json:"command"`
	Cwd       string        `json:"cwd"`
	ParsedCmd []interface{} `json:"parsed_cmd"`
}

func (e *EventExecCommandBegin) EventType() string { return "exec_command_begin" }
func (e *EventExecCommandBegin) isEventMsg()       {}
func (e *EventExecCommandBegin) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":       "exec_command_begin",
		"call_id":    e.CallID,
		"command":    e.Command,
		"cwd":        e.Cwd,
		"parsed_cmd": e.ParsedCmd,
	})
}

// EventExecCommandOutputDelta represents incremental output from a running command.
// Chunk contains base64-encoded binary data to prevent corruption during JSON serialization.
// IsBinary indicates whether the chunk contains binary data that was base64-encoded.
type EventExecCommandOutputDelta struct {
	CallID   string `json:"call_id"`
	Stream   string `json:"stream"`
	Chunk    string `json:"chunk"`     // base64 encoded if binary, raw UTF-8 if text
	IsBinary bool   `json:"is_binary"` // true if chunk is base64-encoded binary data
}

func (e *EventExecCommandOutputDelta) EventType() string { return "exec_command_output_delta" }
func (e *EventExecCommandOutputDelta) isEventMsg()       {}
func (e *EventExecCommandOutputDelta) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":      "exec_command_output_delta",
		"call_id":   e.CallID,
		"stream":    e.Stream,
		"chunk":     e.Chunk,
		"is_binary": e.IsBinary,
	})
}

// EventExecCommandEnd indicates command execution has finished.
type EventExecCommandEnd struct {
	CallID           string `json:"call_id"`
	Stdout           string `json:"stdout"`
	Stderr           string `json:"stderr"`
	AggregatedOutput string `json:"aggregated_output"`
	ExitCode         int    `json:"exit_code"`
	Duration         string `json:"duration"`
	FormattedOutput  string `json:"formatted_output"`
}

func (e *EventExecCommandEnd) EventType() string { return "exec_command_end" }
func (e *EventExecCommandEnd) isEventMsg()       {}
func (e *EventExecCommandEnd) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":              "exec_command_end",
		"call_id":           e.CallID,
		"stdout":            e.Stdout,
		"stderr":            e.Stderr,
		"aggregated_output": e.AggregatedOutput,
		"exit_code":         e.ExitCode,
		"duration":          e.Duration,
		"formatted_output":  e.FormattedOutput,
	})
}

// EventToolCallApprovalNeeded indicates that user approval is required before proceeding.
type EventToolCallApprovalNeeded struct {
	CallID            string   `json:"call_id"`
	ToolName          string   `json:"tool_name"`
	Command           []string `json:"command,omitempty"`
	WorkingDirectory  string   `json:"working_directory"`
	Justification     string   `json:"justification,omitempty"`
	IsRetry           bool     `json:"is_retry"`
	RetryReason       string   `json:"retry_reason,omitempty"`
	RiskLevel         string   `json:"risk_level,omitempty"`
	RiskReasons       []string `json:"risk_reasons,omitempty"`
	RiskMitigation    string   `json:"risk_mitigation,omitempty"`
}

func (e *EventToolCallApprovalNeeded) EventType() string { return "tool_call_approval_needed" }
func (e *EventToolCallApprovalNeeded) isEventMsg()       {}
func (e *EventToolCallApprovalNeeded) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"type":              "tool_call_approval_needed",
		"call_id":           e.CallID,
		"tool_name":         e.ToolName,
		"working_directory": e.WorkingDirectory,
		"is_retry":          e.IsRetry,
	}
	if len(e.Command) > 0 {
		m["command"] = e.Command
	}
	if e.Justification != "" {
		m["justification"] = e.Justification
	}
	if e.RetryReason != "" {
		m["retry_reason"] = e.RetryReason
	}
	if e.RiskLevel != "" {
		m["risk_level"] = e.RiskLevel
	}
	if len(e.RiskReasons) > 0 {
		m["risk_reasons"] = e.RiskReasons
	}
	if e.RiskMitigation != "" {
		m["risk_mitigation"] = e.RiskMitigation
	}
	return json.Marshal(m)
}

// EventShutdownComplete indicates the agent is shutting down.
type EventShutdownComplete struct{}

func (e *EventShutdownComplete) EventType() string { return "shutdown_complete" }
func (e *EventShutdownComplete) isEventMsg()       {}
func (e *EventShutdownComplete) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": "shutdown_complete",
	})
}

// EventAgentReasoningRawContent represents raw chain-of-thought from the agent.
type EventAgentReasoningRawContent struct {
	Text string `json:"text"`
}

func (e *EventAgentReasoningRawContent) EventType() string { return "agent_reasoning_raw_content" }
func (e *EventAgentReasoningRawContent) isEventMsg()       {}
func (e *EventAgentReasoningRawContent) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": "agent_reasoning_raw_content",
		"text": e.Text,
	})
}

// EventAgentReasoningRawContentDelta represents a raw reasoning content delta from the agent.
type EventAgentReasoningRawContentDelta struct {
	Delta string `json:"delta"`
}

func (e *EventAgentReasoningRawContentDelta) EventType() string {
	return "agent_reasoning_raw_content_delta"
}
func (e *EventAgentReasoningRawContentDelta) isEventMsg() {}
func (e *EventAgentReasoningRawContentDelta) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":  "agent_reasoning_raw_content_delta",
		"delta": e.Delta,
	})
}

// EventAgentReasoningSectionBreak signals when the model begins a new reasoning summary section.
type EventAgentReasoningSectionBreak struct{}

func (e *EventAgentReasoningSectionBreak) EventType() string { return "agent_reasoning_section_break" }
func (e *EventAgentReasoningSectionBreak) isEventMsg()       {}
func (e *EventAgentReasoningSectionBreak) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": "agent_reasoning_section_break",
	})
}

// EventSessionConfigured acknowledges the client's configure message.
type EventSessionConfigured struct {
	SessionID         string   `json:"session_id"`
	Model             string   `json:"model"`
	ReasoningEffort   *string  `json:"reasoning_effort,omitempty"`
	HistoryLogID      uint64   `json:"history_log_id"`
	HistoryEntryCount int      `json:"history_entry_count"`
	InitialMessages   []string `json:"initial_messages,omitempty"`
	RolloutPath       string   `json:"rollout_path"`
}

func (e *EventSessionConfigured) EventType() string { return "session_configured" }
func (e *EventSessionConfigured) isEventMsg()       {}
func (e *EventSessionConfigured) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"type":                "session_configured",
		"session_id":          e.SessionID,
		"model":               e.Model,
		"history_log_id":      e.HistoryLogID,
		"history_entry_count": e.HistoryEntryCount,
		"rollout_path":        e.RolloutPath,
	}
	if e.ReasoningEffort != nil {
		m["reasoning_effort"] = *e.ReasoningEffort
	}
	if len(e.InitialMessages) > 0 {
		m["initial_messages"] = e.InitialMessages
	}
	return json.Marshal(m)
}

// McpInvocation represents an MCP tool invocation.
type McpInvocation struct {
	Server    string      `json:"server"`
	Tool      string      `json:"tool"`
	Arguments interface{} `json:"arguments,omitempty"`
}

// EventMcpToolCallBegin indicates the start of an MCP tool call.
type EventMcpToolCallBegin struct {
	CallID     string        `json:"call_id"`
	Invocation McpInvocation `json:"invocation"`
}

func (e *EventMcpToolCallBegin) EventType() string { return "mcp_tool_call_begin" }
func (e *EventMcpToolCallBegin) isEventMsg()       {}
func (e *EventMcpToolCallBegin) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":       "mcp_tool_call_begin",
		"call_id":    e.CallID,
		"invocation": e.Invocation,
	})
}

// EventMcpToolCallEnd indicates the completion of an MCP tool call.
type EventMcpToolCallEnd struct {
	CallID     string        `json:"call_id"`
	Invocation McpInvocation `json:"invocation"`
	Duration   string        `json:"duration"`
	Result     interface{}   `json:"result"`
}

func (e *EventMcpToolCallEnd) EventType() string { return "mcp_tool_call_end" }
func (e *EventMcpToolCallEnd) isEventMsg()       {}
func (e *EventMcpToolCallEnd) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":       "mcp_tool_call_end",
		"call_id":    e.CallID,
		"invocation": e.Invocation,
		"duration":   e.Duration,
		"result":     e.Result,
	})
}

// EventWebSearchBegin indicates the start of a web search operation.
type EventWebSearchBegin struct {
	CallID string `json:"call_id"`
}

func (e *EventWebSearchBegin) EventType() string { return "web_search_begin" }
func (e *EventWebSearchBegin) isEventMsg()       {}
func (e *EventWebSearchBegin) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":    "web_search_begin",
		"call_id": e.CallID,
	})
}

// EventWebSearchEnd indicates the completion of a web search operation.
type EventWebSearchEnd struct {
	CallID string `json:"call_id"`
	Query  string `json:"query"`
}

func (e *EventWebSearchEnd) EventType() string { return "web_search_end" }
func (e *EventWebSearchEnd) isEventMsg()       {}
func (e *EventWebSearchEnd) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":    "web_search_end",
		"call_id": e.CallID,
		"query":   e.Query,
	})
}

// EventViewImageToolCall indicates the agent attached a local image via view_image tool.
type EventViewImageToolCall struct {
	CallID string `json:"call_id"`
	Path   string `json:"path"`
}

func (e *EventViewImageToolCall) EventType() string { return "view_image_tool_call" }
func (e *EventViewImageToolCall) isEventMsg()       {}
func (e *EventViewImageToolCall) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":    "view_image_tool_call",
		"call_id": e.CallID,
		"path":    e.Path,
	})
}

// EventBackgroundEvent represents a background event message.
type EventBackgroundEvent struct {
	Message string `json:"message"`
}

func (e *EventBackgroundEvent) EventType() string { return "background_event" }
func (e *EventBackgroundEvent) isEventMsg()       {}
func (e *EventBackgroundEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":    "background_event",
		"message": e.Message,
	})
}

// EventResourceListChanged indicates that the list of resources has changed on an MCP server.
type EventResourceListChanged struct {
	ServerName string `json:"server_name"`
}

func (e *EventResourceListChanged) EventType() string { return "resource_list_changed" }
func (e *EventResourceListChanged) isEventMsg()       {}
func (e *EventResourceListChanged) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":        "resource_list_changed",
		"server_name": e.ServerName,
	})
}

// EventStreamError indicates a model stream error or disconnect.
type EventStreamError struct {
	Message string `json:"message"`
}

func (e *EventStreamError) EventType() string { return "stream_error" }
func (e *EventStreamError) isEventMsg()       {}
func (e *EventStreamError) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":    "stream_error",
		"message": e.Message,
	})
}

// EventPatchApplyBegin indicates the agent is about to apply a code patch.
type EventPatchApplyBegin struct {
	CallID       string                 `json:"call_id"`
	AutoApproved bool                   `json:"auto_approved"`
	Changes      map[string]interface{} `json:"changes"`
}

func (e *EventPatchApplyBegin) EventType() string { return "patch_apply_begin" }
func (e *EventPatchApplyBegin) isEventMsg()       {}
func (e *EventPatchApplyBegin) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":          "patch_apply_begin",
		"call_id":       e.CallID,
		"auto_approved": e.AutoApproved,
		"changes":       e.Changes,
	})
}

// EventPatchApplyEnd indicates that a patch application has finished.
type EventPatchApplyEnd struct {
	CallID  string `json:"call_id"`
	Stdout  string `json:"stdout"`
	Stderr  string `json:"stderr"`
	Success bool   `json:"success"`
}

func (e *EventPatchApplyEnd) EventType() string { return "patch_apply_end" }
func (e *EventPatchApplyEnd) isEventMsg()       {}
func (e *EventPatchApplyEnd) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":    "patch_apply_end",
		"call_id": e.CallID,
		"stdout":  e.Stdout,
		"stderr":  e.Stderr,
		"success": e.Success,
	})
}

// EventTurnDiff represents a unified diff for the turn.
type EventTurnDiff struct {
	UnifiedDiff string `json:"unified_diff"`
}

func (e *EventTurnDiff) EventType() string { return "turn_diff" }
func (e *EventTurnDiff) isEventMsg()       {}
func (e *EventTurnDiff) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":         "turn_diff",
		"unified_diff": e.UnifiedDiff,
	})
}

// EventGetHistoryEntryResponse is the response to GetHistoryEntryRequest.
type EventGetHistoryEntryResponse struct {
	Offset uint64      `json:"offset"`
	LogID  uint64      `json:"log_id"`
	Entry  interface{} `json:"entry,omitempty"`
}

func (e *EventGetHistoryEntryResponse) EventType() string { return "get_history_entry_response" }
func (e *EventGetHistoryEntryResponse) isEventMsg()       {}
func (e *EventGetHistoryEntryResponse) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"type":   "get_history_entry_response",
		"offset": e.Offset,
		"log_id": e.LogID,
	}
	if e.Entry != nil {
		m["entry"] = e.Entry
	}
	return json.Marshal(m)
}

// EventMcpListToolsResponse is the list of MCP tools available to the agent.
type EventMcpListToolsResponse struct {
	Tools             map[string]interface{} `json:"tools"`
	Resources         map[string]interface{} `json:"resources"`
	ResourceTemplates map[string]interface{} `json:"resource_templates"`
	AuthStatuses      map[string]string      `json:"auth_statuses"`
}

func (e *EventMcpListToolsResponse) EventType() string { return "mcp_list_tools_response" }
func (e *EventMcpListToolsResponse) isEventMsg()       {}
func (e *EventMcpListToolsResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":               "mcp_list_tools_response",
		"tools":              e.Tools,
		"resources":          e.Resources,
		"resource_templates": e.ResourceTemplates,
		"auth_statuses":      e.AuthStatuses,
	})
}

// EventListCustomPromptsResponse is the list of custom prompts available to the agent.
type EventListCustomPromptsResponse struct {
	CustomPrompts []interface{} `json:"custom_prompts"`
}

func (e *EventListCustomPromptsResponse) EventType() string { return "list_custom_prompts_response" }
func (e *EventListCustomPromptsResponse) isEventMsg()       {}
func (e *EventListCustomPromptsResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":           "list_custom_prompts_response",
		"custom_prompts": e.CustomPrompts,
	})
}

// EventPlanUpdate represents an update to the plan.
type EventPlanUpdate struct {
	Plan interface{} `json:"plan"`
}

func (e *EventPlanUpdate) EventType() string { return "plan_update" }
func (e *EventPlanUpdate) isEventMsg()       {}
func (e *EventPlanUpdate) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": "plan_update",
		"plan": e.Plan,
	})
}

// EventTurnAborted indicates a turn was aborted.
type EventTurnAborted struct {
	Reason string `json:"reason"`
}

func (e *EventTurnAborted) EventType() string { return "turn_aborted" }
func (e *EventTurnAborted) isEventMsg()       {}
func (e *EventTurnAborted) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":   "turn_aborted",
		"reason": e.Reason,
	})
}

// EventConversationPath is the response containing the current session's in-memory transcript.
type EventConversationPath struct {
	ConversationID string `json:"conversation_id"`
	Path           string `json:"path"`
}

func (e *EventConversationPath) EventType() string { return "conversation_path" }
func (e *EventConversationPath) isEventMsg()       {}
func (e *EventConversationPath) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":            "conversation_path",
		"conversation_id": e.ConversationID,
		"path":            e.Path,
	})
}

// EventEnteredReviewMode indicates the agent entered review mode.
type EventEnteredReviewMode struct {
	Prompt         string `json:"prompt"`
	UserFacingHint string `json:"user_facing_hint"`
}

func (e *EventEnteredReviewMode) EventType() string { return "entered_review_mode" }
func (e *EventEnteredReviewMode) isEventMsg()       {}
func (e *EventEnteredReviewMode) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":             "entered_review_mode",
		"prompt":           e.Prompt,
		"user_facing_hint": e.UserFacingHint,
	})
}

// EventExitedReviewMode indicates the agent exited review mode.
type EventExitedReviewMode struct {
	ReviewOutput interface{} `json:"review_output,omitempty"`
}

func (e *EventExitedReviewMode) EventType() string { return "exited_review_mode" }
func (e *EventExitedReviewMode) isEventMsg()       {}
func (e *EventExitedReviewMode) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"type": "exited_review_mode",
	}
	if e.ReviewOutput != nil {
		m["review_output"] = e.ReviewOutput
	}
	return json.Marshal(m)
}

// EventRawResponseItem represents a raw response item.
type EventRawResponseItem struct {
	Item interface{} `json:"item"`
}

func (e *EventRawResponseItem) EventType() string { return "raw_response_item" }
func (e *EventRawResponseItem) isEventMsg()       {}
func (e *EventRawResponseItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": "raw_response_item",
		"item": e.Item,
	})
}

// EventItemStarted indicates an item has started.
type EventItemStarted struct {
	ThreadID string      `json:"thread_id"`
	TurnID   string      `json:"turn_id"`
	Item     interface{} `json:"item"`
}

func (e *EventItemStarted) EventType() string { return "item_started" }
func (e *EventItemStarted) isEventMsg()       {}
func (e *EventItemStarted) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":      "item_started",
		"thread_id": e.ThreadID,
		"turn_id":   e.TurnID,
		"item":      e.Item,
	})
}

// EventItemCompleted indicates an item has completed.
type EventItemCompleted struct {
	ThreadID string      `json:"thread_id"`
	TurnID   string      `json:"turn_id"`
	Item     interface{} `json:"item"`
}

func (e *EventItemCompleted) EventType() string { return "item_completed" }
func (e *EventItemCompleted) isEventMsg()       {}
func (e *EventItemCompleted) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":      "item_completed",
		"thread_id": e.ThreadID,
		"turn_id":   e.TurnID,
		"item":      e.Item,
	})
}

// EventSandboxViolation reports a sandbox policy violation.
type EventSandboxViolation struct {
	CallID       string  `json:"call_id"`
	SandboxType  string  `json:"sandbox_type"`
	Operation    string  `json:"operation"`
	Path         *string `json:"path,omitempty"`
	Syscall      *string `json:"syscall,omitempty"`
	ErrorMessage string  `json:"error_message"`
	ExitCode     int     `json:"exit_code"`
	Timestamp    string  `json:"timestamp"`
}

func (e *EventSandboxViolation) EventType() string { return "sandbox_violation" }
func (e *EventSandboxViolation) isEventMsg()       {}
func (e *EventSandboxViolation) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"type":          "sandbox_violation",
		"call_id":       e.CallID,
		"sandbox_type":  e.SandboxType,
		"operation":     e.Operation,
		"error_message": e.ErrorMessage,
		"exit_code":     e.ExitCode,
		"timestamp":     e.Timestamp,
	}
	if e.Path != nil {
		m["path"] = *e.Path
	}
	if e.Syscall != nil {
		m["syscall"] = *e.Syscall
	}
	return json.Marshal(m)
}

// EventOperationStarted indicates a long-running operation has started.
type EventOperationStarted struct {
	Operation         string  `json:"operation"`
	EstimatedDuration *int64  `json:"estimated_duration_ms,omitempty"`
	Total             *int64  `json:"total,omitempty"`
	Message           string  `json:"message,omitempty"`
}

func (e *EventOperationStarted) EventType() string { return "operation_started" }
func (e *EventOperationStarted) isEventMsg()       {}
func (e *EventOperationStarted) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"type":      "operation_started",
		"operation": e.Operation,
	}
	if e.EstimatedDuration != nil {
		m["estimated_duration_ms"] = *e.EstimatedDuration
	}
	if e.Total != nil {
		m["total"] = *e.Total
	}
	if e.Message != "" {
		m["message"] = e.Message
	}
	return json.Marshal(m)
}

// EventOperationProgress reports progress for a long-running operation.
type EventOperationProgress struct {
	Operation  string   `json:"operation"`
	Current    int64    `json:"current"`
	Total      *int64   `json:"total,omitempty"`
	Percentage *float64 `json:"percentage,omitempty"`
	Message    string   `json:"message,omitempty"`
}

func (e *EventOperationProgress) EventType() string { return "operation_progress" }
func (e *EventOperationProgress) isEventMsg()       {}
func (e *EventOperationProgress) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"type":      "operation_progress",
		"operation": e.Operation,
		"current":   e.Current,
	}
	if e.Total != nil {
		m["total"] = *e.Total
	}
	if e.Percentage != nil {
		m["percentage"] = *e.Percentage
	}
	if e.Message != "" {
		m["message"] = e.Message
	}
	return json.Marshal(m)
}

// EventOperationCompleted indicates a long-running operation has finished.
type EventOperationCompleted struct {
	Operation      string  `json:"operation"`
	DurationMs     int64   `json:"duration_ms"`
	Status         string  `json:"status"`
	Message        string  `json:"message,omitempty"`
	ProcessedCount *int64  `json:"processed_count,omitempty"`
}

func (e *EventOperationCompleted) EventType() string { return "operation_completed" }
func (e *EventOperationCompleted) isEventMsg()       {}
func (e *EventOperationCompleted) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"type":        "operation_completed",
		"operation":   e.Operation,
		"duration_ms": e.DurationMs,
		"status":      e.Status,
	}
	if e.Message != "" {
		m["message"] = e.Message
	}
	if e.ProcessedCount != nil {
		m["processed_count"] = *e.ProcessedCount
	}
	return json.Marshal(m)
}

// UnmarshalJSON implements custom unmarshaling for Event
func (e *Event) UnmarshalJSON(data []byte) error {
	var temp struct {
		ID  string          `json:"id"`
		Msg json.RawMessage `json:"msg"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	e.ID = temp.ID

	// Parse the msg based on its type field
	var typeCheck struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(temp.Msg, &typeCheck); err != nil {
		return err
	}

	msg, err := unmarshalEventMsg(typeCheck.Type, temp.Msg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal event msg of type %s: %w", typeCheck.Type, err)
	}
	e.Msg = msg
	return nil
}

func unmarshalEventMsg(eventType string, data []byte) (EventMsg, error) {
	switch eventType {
	case "error":
		var evt EventError
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "task_started":
		var evt EventTaskStarted
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "task_complete":
		var evt EventTaskComplete
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "token_count":
		var evt EventTokenCount
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "agent_message":
		var evt EventAgentMessage
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "user_message":
		var evt EventUserMessage
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "agent_message_delta":
		var evt EventAgentMessageDelta
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "agent_reasoning":
		var evt EventAgentReasoning
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "agent_reasoning_delta":
		var evt EventAgentReasoningDelta
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "exec_command_begin":
		var evt EventExecCommandBegin
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "exec_command_output_delta":
		var evt EventExecCommandOutputDelta
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "exec_command_end":
		var evt EventExecCommandEnd
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "tool_call_approval_needed":
		var evt EventToolCallApprovalNeeded
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "shutdown_complete":
		return &EventShutdownComplete{}, nil
	case "agent_reasoning_raw_content":
		var evt EventAgentReasoningRawContent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "agent_reasoning_raw_content_delta":
		var evt EventAgentReasoningRawContentDelta
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "agent_reasoning_section_break":
		return &EventAgentReasoningSectionBreak{}, nil
	case "session_configured":
		var evt EventSessionConfigured
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "mcp_tool_call_begin":
		var evt EventMcpToolCallBegin
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "mcp_tool_call_end":
		var evt EventMcpToolCallEnd
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "web_search_begin":
		var evt EventWebSearchBegin
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "web_search_end":
		var evt EventWebSearchEnd
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "view_image_tool_call":
		var evt EventViewImageToolCall
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "background_event":
		var evt EventBackgroundEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "resource_list_changed":
		var evt EventResourceListChanged
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "stream_error":
		var evt EventStreamError
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "patch_apply_begin":
		var evt EventPatchApplyBegin
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "patch_apply_end":
		var evt EventPatchApplyEnd
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "turn_diff":
		var evt EventTurnDiff
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "get_history_entry_response":
		var evt EventGetHistoryEntryResponse
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "mcp_list_tools_response":
		var evt EventMcpListToolsResponse
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "list_custom_prompts_response":
		var evt EventListCustomPromptsResponse
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "plan_update":
		var evt EventPlanUpdate
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "turn_aborted":
		var evt EventTurnAborted
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "conversation_path":
		var evt EventConversationPath
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "entered_review_mode":
		var evt EventEnteredReviewMode
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "exited_review_mode":
		var evt EventExitedReviewMode
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "raw_response_item":
		var evt EventRawResponseItem
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "item_started":
		var evt EventItemStarted
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "item_completed":
		var evt EventItemCompleted
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "sandbox_violation":
		var evt EventSandboxViolation
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "operation_started":
		var evt EventOperationStarted
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "operation_progress":
		var evt EventOperationProgress
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	case "operation_completed":
		var evt EventOperationCompleted
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, err
		}
		return &evt, nil
	default:
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}
}

// Supporting types

// UserInput represents user input in various forms.
type UserInput struct {
	Type     string  `json:"type"`
	Text     *string `json:"text,omitempty"`
	ImageURL *string `json:"image_url,omitempty"`
	Path     *string `json:"path,omitempty"`
}

const (
	MaxUserInputTextSize = 1 * 1024 * 1024 // 1MB
	MaxUserInputPathSize = 256 * 1024      // 256KB
)

// Validate checks if the UserInput is valid
func (u *UserInput) Validate() error {
	switch u.Type {
	case UserInputTypeText:
		if u.Text == nil {
			return fmt.Errorf("text is required for type 'text'")
		}
		if len(*u.Text) > MaxUserInputTextSize {
			return fmt.Errorf("text exceeds maximum size of %d bytes", MaxUserInputTextSize)
		}
	case UserInputTypeImageURL:
		if u.ImageURL == nil {
			return fmt.Errorf("image_url is required for type 'image_url'")
		}
		if len(*u.ImageURL) > MaxUserInputPathSize {
			return fmt.Errorf("image_url exceeds maximum size of %d bytes", MaxUserInputPathSize)
		}
	case UserInputTypePath:
		if u.Path == nil {
			return fmt.Errorf("path is required for type 'path'")
		}
		if len(*u.Path) > MaxUserInputPathSize {
			return fmt.Errorf("path exceeds maximum size of %d bytes", MaxUserInputPathSize)
		}
	default:
		return fmt.Errorf("invalid type: %s", u.Type)
	}
	return nil
}

// SandboxPolicy determines execution restrictions for model shell commands.
type SandboxPolicy struct {
	Mode                string   `json:"mode"`
	WritableRoots       []string `json:"writable_roots"`
	NetworkAccess       bool     `json:"network_access"`
	ExcludeTmpdirEnvVar bool     `json:"exclude_tmpdir_env_var"`
	ExcludeSlashTmp     bool     `json:"exclude_slash_tmp"`
}

// MarshalJSON implements custom marshaling for SandboxPolicy
func (s SandboxPolicy) MarshalJSON() ([]byte, error) {
	// For workspace-write mode, always include all fields
	if s.Mode == SandboxModeWorkspaceWrite {
		type Alias SandboxPolicy
		return json.Marshal(&struct {
			*Alias
		}{
			Alias: (*Alias)(&s),
		})
	}
	// For other modes, only include the mode
	return json.Marshal(map[string]interface{}{
		"mode": s.Mode,
	})
}

// Validate checks if the SandboxPolicy is valid
func (s *SandboxPolicy) Validate() error {
	if s.Mode != SandboxModeReadOnly && s.Mode != SandboxModeWorkspaceWrite &&
		s.Mode != SandboxModeDangerFullAccess {
		return fmt.Errorf("invalid mode: %s", s.Mode)
	}
	if s.Mode == SandboxModeWorkspaceWrite {
		// Validate WritableRoots if provided
		for i, root := range s.WritableRoots {
			if root == "" {
				return fmt.Errorf("writable_roots[%d] cannot be empty", i)
			}
		}
	}
	return nil
}

// ReviewRequest represents a code review request.
type ReviewRequest struct {
	Prompt         string `json:"prompt"`
	UserFacingHint string `json:"user_facing_hint"`
}

// TokenUsage represents token usage statistics.
type TokenUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
}

// CachedInput returns the number of cached input tokens.
func (t *TokenUsage) CachedInput() int64 {
	if t.CachedInputTokens < 0 {
		return 0
	}
	return t.CachedInputTokens
}

// NonCachedInput returns the number of non-cached input tokens.
func (t *TokenUsage) NonCachedInput() int64 {
	result := t.InputTokens - t.CachedInput()
	if result < 0 {
		return 0
	}
	return result
}

// BlendedTotal returns the primary count for display: non-cached input + output.
func (t *TokenUsage) BlendedTotal() int64 {
	result := t.NonCachedInput() + max(t.OutputTokens, 0)
	if result < 0 {
		return 0
	}
	return result
}

// TokensInContextWindow estimates tokens in context window.
func (t *TokenUsage) TokensInContextWindow() int64 {
	result := t.TotalTokens - t.ReasoningOutputTokens
	if result < 0 {
		return 0
	}
	return result
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// TokenUsageInfo contains usage information for a session.
type TokenUsageInfo struct {
	TotalTokenUsage    TokenUsage `json:"total_token_usage"`
	LastTokenUsage     TokenUsage `json:"last_token_usage"`
	ModelContextWindow *int64     `json:"model_context_window,omitempty"`
}

// RateLimitSnapshot represents rate limit information.
type RateLimitSnapshot struct {
	Primary   *RateLimitWindow `json:"primary,omitempty"`
	Secondary *RateLimitWindow `json:"secondary,omitempty"`
}

// RateLimitWindow represents a rate limit window.
type RateLimitWindow struct {
	UsedPercent   float64 `json:"used_percent"`
	WindowMinutes *int64  `json:"window_minutes,omitempty"`
	ResetsAt      *int64  `json:"resets_at,omitempty"`
}

// ResponseItem represents items in a response.
type ResponseItem interface {
	ResponseItemType() string
	isResponseItem()
}

// ResponseItemMessage represents a message in a response.
type ResponseItemMessage struct {
	ID      *string       `json:"id,omitempty"`
	Role    string        `json:"role"`
	Content []ContentItem `json:"content"`
}

func (r *ResponseItemMessage) ResponseItemType() string { return "message" }
func (r *ResponseItemMessage) isResponseItem()          {}
func (r *ResponseItemMessage) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":    "message",
		"role":    r.Role,
		"content": r.Content,
	})
}

// ResponseItemReasoning represents reasoning in a response.
type ResponseItemReasoning struct {
	ID      string             `json:"id,omitempty"`
	Summary []ReasoningSummary `json:"summary"`
	Content []ReasoningContent `json:"content,omitempty"`
}

func (r *ResponseItemReasoning) ResponseItemType() string { return "reasoning" }
func (r *ResponseItemReasoning) isResponseItem()          {}
func (r *ResponseItemReasoning) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"type":    "reasoning",
		"summary": r.Summary,
	}
	if len(r.Content) > 0 {
		m["content"] = r.Content
	}
	return json.Marshal(m)
}

// ContentItem represents an item in message content.
type ContentItem struct {
	Type     string  `json:"type"`
	Text     *string `json:"text,omitempty"`
	ImageURL *string `json:"image_url,omitempty"`
}

// ReasoningSummary represents a reasoning summary item.
type ReasoningSummary struct {
	Type string  `json:"type"`
	Text *string `json:"text,omitempty"`
}

// ReasoningContent represents reasoning content.
type ReasoningContent struct {
	Type string  `json:"type"`
	Text *string `json:"text,omitempty"`
}

// EventExecCommandEndEvent is an alias for EventExecCommandEnd for test compatibility.
type EventExecCommandEndEvent = EventExecCommandEnd
