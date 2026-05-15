package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestOpSerialization tests that Op types serialize/deserialize correctly
func TestOpSerialization(t *testing.T) {
	tests := []struct {
		name     string
		op       Op
		wantJSON string
	}{
		{
			name: "Interrupt",
			op:   &OpInterrupt{},
			wantJSON: `{
				"type": "interrupt"
			}`,
		},
		{
			name: "UserInput",
			op: &OpUserInput{
				Items: []UserInput{
					{Type: "text", Text: strPtr("Hello")},
				},
			},
			wantJSON: `{
				"type": "user_input",
				"items": [
					{"type": "text", "text": "Hello"}
				]
			}`,
		},
		{
			name: "UserTurn",
			op: &OpUserTurn{
				Items: []UserInput{
					{Type: "text", Text: strPtr("Hello world")},
				},
				Cwd:                   "/test/path",
				ApprovalPolicy:        "on-request",
				SandboxPolicy:         SandboxPolicy{Mode: "read-only"},
				Model:                 "claude-3-5-sonnet-20241022",
				Summary:               "auto",
				Effort:                strPtr("medium"),
				FinalOutputJSONSchema: nil,
			},
			wantJSON: `{
				"type": "user_turn",
				"items": [{"type": "text", "text": "Hello world"}],
				"cwd": "/test/path",
				"approval_policy": "on-request",
				"sandbox_policy": {"mode": "read-only"},
				"model": "claude-3-5-sonnet-20241022",
				"summary": "auto",
				"effort": "medium"
			}`,
		},
		{
			name: "ExecApproval",
			op: &OpExecApproval{
				ID:       "req-123",
				Decision: "approved",
			},
			wantJSON: `{
				"type": "exec_approval",
				"id": "req-123",
				"decision": "approved"
			}`,
		},
		{
			name: "PatchApproval",
			op: &OpPatchApproval{
				ID:       "req-456",
				Decision: "denied",
			},
			wantJSON: `{
				"type": "patch_approval",
				"id": "req-456",
				"decision": "denied"
			}`,
		},
		{
			name: "Shutdown",
			op:   &OpShutdown{},
			wantJSON: `{
				"type": "shutdown"
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			got, err := json.Marshal(tt.op)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			// Normalize JSON for comparison
			var gotNorm, wantNorm interface{}
			if err := json.Unmarshal(got, &gotNorm); err != nil {
				t.Fatalf("Unmarshal got error = %v", err)
			}
			if err := json.Unmarshal([]byte(tt.wantJSON), &wantNorm); err != nil {
				t.Fatalf("Unmarshal want error = %v", err)
			}

			gotJSON, _ := json.Marshal(gotNorm)
			wantJSON, _ := json.Marshal(wantNorm)

			if string(gotJSON) != string(wantJSON) {
				t.Errorf("Marshal() = %s, want %s", string(gotJSON), string(wantJSON))
			}

			// Test unmarshaling back - need to determine type first
			var typeCheck struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(got, &typeCheck); err != nil {
				t.Fatalf("Type check unmarshal error = %v", err)
			}

			// Create appropriate concrete type
			var unmarshaled Op
			switch typeCheck.Type {
			case "interrupt":
				unmarshaled = &OpInterrupt{}
			case "user_input":
				unmarshaled = &OpUserInput{}
			case "user_turn":
				unmarshaled = &OpUserTurn{}
			case "exec_approval":
				unmarshaled = &OpExecApproval{}
			case "patch_approval":
				unmarshaled = &OpPatchApproval{}
			case "shutdown":
				unmarshaled = &OpShutdown{}
			default:
				t.Fatalf("Unknown op type: %s", typeCheck.Type)
			}

			if err := json.Unmarshal(got, unmarshaled); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			// Marshal again to compare
			got2, err := json.Marshal(unmarshaled)
			if err != nil {
				t.Fatalf("Marshal() after Unmarshal() error = %v", err)
			}

			if string(got) != string(got2) {
				t.Errorf("Round-trip failed: original = %s, after = %s", string(got), string(got2))
			}
		})
	}
}

// TestEventMsgSerialization tests that EventMsg types serialize/deserialize correctly
func TestEventMsgSerialization(t *testing.T) {
	tests := []struct {
		name     string
		event    EventMsg
		wantJSON string
	}{
		{
			name: "AgentMessage",
			event: &EventAgentMessage{
				Message: "Hello from agent",
			},
			wantJSON: `{
				"type": "agent_message",
				"message": "Hello from agent"
			}`,
		},
		{
			name: "TaskStarted",
			event: &EventTaskStarted{
				ModelContextWindow: int64Ptr(200000),
			},
			wantJSON: `{
				"type": "task_started",
				"model_context_window": 200000
			}`,
		},
		{
			name: "TaskComplete",
			event: &EventTaskComplete{
				LastAgentMessage: strPtr("Task finished"),
			},
			wantJSON: `{
				"type": "task_complete",
				"last_agent_message": "Task finished"
			}`,
		},
		{
			name: "Error",
			event: &EventError{
				Message: "Something went wrong",
			},
			wantJSON: `{
				"type": "error",
				"message": "Something went wrong"
			}`,
		},
		{
			name: "ExecCommandBegin",
			event: &EventExecCommandBegin{
				CallID:    "call-123",
				Command:   []string{"ls", "-la"},
				Cwd:       "/test/path",
				ParsedCmd: []interface{}{},
			},
			wantJSON: `{
				"type": "exec_command_begin",
				"call_id": "call-123",
				"command": ["ls", "-la"],
				"cwd": "/test/path",
				"parsed_cmd": []
			}`,
		},
		{
			name: "ExecCommandEnd",
			event: &EventExecCommandEnd{
				CallID:           "call-123",
				Stdout:           "output",
				Stderr:           "error",
				AggregatedOutput: "output\nerror",
				ExitCode:         0,
				Duration:         "1.5s",
				FormattedOutput:  "output",
			},
			wantJSON: `{
				"type": "exec_command_end",
				"call_id": "call-123",
				"stdout": "output",
				"stderr": "error",
				"aggregated_output": "output\nerror",
				"exit_code": 0,
				"duration": "1.5s",
				"formatted_output": "output"
			}`,
		},
		{
			name: "AgentReasoningRawContent",
			event: &EventAgentReasoningRawContent{
				Text: "Raw reasoning content",
			},
			wantJSON: `{
				"type": "agent_reasoning_raw_content",
				"text": "Raw reasoning content"
			}`,
		},
		{
			name: "AgentReasoningRawContentDelta",
			event: &EventAgentReasoningRawContentDelta{
				Delta: "reasoning delta",
			},
			wantJSON: `{
				"type": "agent_reasoning_raw_content_delta",
				"delta": "reasoning delta"
			}`,
		},
		{
			name:  "AgentReasoningSectionBreak",
			event: &EventAgentReasoningSectionBreak{},
			wantJSON: `{
				"type": "agent_reasoning_section_break"
			}`,
		},
		{
			name: "SessionConfigured",
			event: &EventSessionConfigured{
				SessionID:         "session-123",
				Model:             "claude-3-5-sonnet-20241022",
				HistoryLogID:      12345,
				HistoryEntryCount: 10,
				RolloutPath:       "/path/to/rollout",
			},
			wantJSON: `{
				"type": "session_configured",
				"session_id": "session-123",
				"model": "claude-3-5-sonnet-20241022",
				"history_log_id": 12345,
				"history_entry_count": 10,
				"rollout_path": "/path/to/rollout"
			}`,
		},
		{
			name: "McpToolCallBegin",
			event: &EventMcpToolCallBegin{
				CallID: "mcp-call-123",
				Invocation: McpInvocation{
					Server: "test-server",
					Tool:   "test-tool",
				},
			},
			wantJSON: `{
				"type": "mcp_tool_call_begin",
				"call_id": "mcp-call-123",
				"invocation": {
					"server": "test-server",
					"tool": "test-tool"
				}
			}`,
		},
		{
			name: "McpToolCallEnd",
			event: &EventMcpToolCallEnd{
				CallID: "mcp-call-123",
				Invocation: McpInvocation{
					Server: "test-server",
					Tool:   "test-tool",
				},
				Duration: "2.5s",
				Result:   map[string]interface{}{"status": "success"},
			},
			wantJSON: `{
				"type": "mcp_tool_call_end",
				"call_id": "mcp-call-123",
				"invocation": {
					"server": "test-server",
					"tool": "test-tool"
				},
				"duration": "2.5s",
				"result": {"status": "success"}
			}`,
		},
		{
			name: "WebSearchBegin",
			event: &EventWebSearchBegin{
				CallID: "search-123",
			},
			wantJSON: `{
				"type": "web_search_begin",
				"call_id": "search-123"
			}`,
		},
		{
			name: "WebSearchEnd",
			event: &EventWebSearchEnd{
				CallID: "search-123",
				Query:  "golang best practices",
			},
			wantJSON: `{
				"type": "web_search_end",
				"call_id": "search-123",
				"query": "golang best practices"
			}`,
		},
		{
			name: "ViewImageToolCall",
			event: &EventViewImageToolCall{
				CallID: "image-123",
				Path:   "/path/to/image.png",
			},
			wantJSON: `{
				"type": "view_image_tool_call",
				"call_id": "image-123",
				"path": "/path/to/image.png"
			}`,
		},
		{
			name: "BackgroundEvent",
			event: &EventBackgroundEvent{
				Message: "Background processing started",
			},
			wantJSON: `{
				"type": "background_event",
				"message": "Background processing started"
			}`,
		},
		{
			name: "ResourceListChanged",
			event: &EventResourceListChanged{
				ServerName: "test-server",
			},
			wantJSON: `{
				"type": "resource_list_changed",
				"server_name": "test-server"
			}`,
		},
		{
			name: "StreamError",
			event: &EventStreamError{
				Message: "Connection lost",
			},
			wantJSON: `{
				"type": "stream_error",
				"message": "Connection lost"
			}`,
		},
		{
			name: "PatchApplyBegin",
			event: &EventPatchApplyBegin{
				CallID:       "patch-123",
				AutoApproved: true,
				Changes:      map[string]interface{}{"file.go": "update"},
			},
			wantJSON: `{
				"type": "patch_apply_begin",
				"call_id": "patch-123",
				"auto_approved": true,
				"changes": {"file.go": "update"}
			}`,
		},
		{
			name: "PatchApplyEnd",
			event: &EventPatchApplyEnd{
				CallID:  "patch-123",
				Stdout:  "Applied successfully",
				Stderr:  "",
				Success: true,
			},
			wantJSON: `{
				"type": "patch_apply_end",
				"call_id": "patch-123",
				"stdout": "Applied successfully",
				"stderr": "",
				"success": true
			}`,
		},
		{
			name: "TurnDiff",
			event: &EventTurnDiff{
				UnifiedDiff: "diff --git a/file.go b/file.go\n...",
			},
			wantJSON: `{
				"type": "turn_diff",
				"unified_diff": "diff --git a/file.go b/file.go\n..."
			}`,
		},
		{
			name: "GetHistoryEntryResponse",
			event: &EventGetHistoryEntryResponse{
				Offset: 5,
				LogID:  12345,
				Entry:  map[string]string{"text": "history entry"},
			},
			wantJSON: `{
				"type": "get_history_entry_response",
				"offset": 5,
				"log_id": 12345,
				"entry": {"text": "history entry"}
			}`,
		},
		{
			name: "McpListToolsResponse",
			event: &EventMcpListToolsResponse{
				Tools:             map[string]interface{}{"tool1": "description"},
				Resources:         map[string]interface{}{"res1": "data"},
				ResourceTemplates: map[string]interface{}{"tmpl1": "template"},
				AuthStatuses:      map[string]string{"server1": "authenticated"},
			},
			wantJSON: `{
				"type": "mcp_list_tools_response",
				"tools": {"tool1": "description"},
				"resources": {"res1": "data"},
				"resource_templates": {"tmpl1": "template"},
				"auth_statuses": {"server1": "authenticated"}
			}`,
		},
		{
			name: "ListCustomPromptsResponse",
			event: &EventListCustomPromptsResponse{
				CustomPrompts: []interface{}{"prompt1", "prompt2"},
			},
			wantJSON: `{
				"type": "list_custom_prompts_response",
				"custom_prompts": ["prompt1", "prompt2"]
			}`,
		},
		{
			name: "PlanUpdate",
			event: &EventPlanUpdate{
				Plan: map[string]string{"step1": "complete"},
			},
			wantJSON: `{
				"type": "plan_update",
				"plan": {"step1": "complete"}
			}`,
		},
		{
			name: "TurnAborted",
			event: &EventTurnAborted{
				Reason: "interrupted",
			},
			wantJSON: `{
				"type": "turn_aborted",
				"reason": "interrupted"
			}`,
		},
		{
			name: "ConversationPath",
			event: &EventConversationPath{
				ConversationID: "conv-123",
				Path:           "/path/to/conversation",
			},
			wantJSON: `{
				"type": "conversation_path",
				"conversation_id": "conv-123",
				"path": "/path/to/conversation"
			}`,
		},
		{
			name: "EnteredReviewMode",
			event: &EventEnteredReviewMode{
				Prompt:         "Review this code",
				UserFacingHint: "Starting code review",
			},
			wantJSON: `{
				"type": "entered_review_mode",
				"prompt": "Review this code",
				"user_facing_hint": "Starting code review"
			}`,
		},
		{
			name: "ExitedReviewMode",
			event: &EventExitedReviewMode{
				ReviewOutput: map[string]string{"status": "completed"},
			},
			wantJSON: `{
				"type": "exited_review_mode",
				"review_output": {"status": "completed"}
			}`,
		},
		{
			name: "RawResponseItem",
			event: &EventRawResponseItem{
				Item: map[string]string{"type": "message"},
			},
			wantJSON: `{
				"type": "raw_response_item",
				"item": {"type": "message"}
			}`,
		},
		{
			name: "ItemStarted",
			event: &EventItemStarted{
				ThreadID: "thread-123",
				TurnID:   "turn-456",
				Item:     map[string]string{"name": "task"},
			},
			wantJSON: `{
				"type": "item_started",
				"thread_id": "thread-123",
				"turn_id": "turn-456",
				"item": {"name": "task"}
			}`,
		},
		{
			name: "ItemCompleted",
			event: &EventItemCompleted{
				ThreadID: "thread-123",
				TurnID:   "turn-456",
				Item:     map[string]string{"name": "task"},
			},
			wantJSON: `{
				"type": "item_completed",
				"thread_id": "thread-123",
				"turn_id": "turn-456",
				"item": {"name": "task"}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			got, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			// Normalize JSON for comparison
			var gotNorm, wantNorm interface{}
			if err := json.Unmarshal(got, &gotNorm); err != nil {
				t.Fatalf("Unmarshal got error = %v", err)
			}
			if err := json.Unmarshal([]byte(tt.wantJSON), &wantNorm); err != nil {
				t.Fatalf("Unmarshal want error = %v", err)
			}

			gotJSON, _ := json.Marshal(gotNorm)
			wantJSON, _ := json.Marshal(wantNorm)

			if string(gotJSON) != string(wantJSON) {
				t.Errorf("Marshal() = %s, want %s", string(gotJSON), string(wantJSON))
			}

			// Test unmarshaling back - need to determine type first
			var typeCheck struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(got, &typeCheck); err != nil {
				t.Fatalf("Type check unmarshal error = %v", err)
			}

			// Create appropriate concrete type
			var unmarshaled EventMsg
			switch typeCheck.Type {
			case "error":
				unmarshaled = &EventError{}
			case "task_started":
				unmarshaled = &EventTaskStarted{}
			case "task_complete":
				unmarshaled = &EventTaskComplete{}
			case "token_count":
				unmarshaled = &EventTokenCount{}
			case "agent_message":
				unmarshaled = &EventAgentMessage{}
			case "user_message":
				unmarshaled = &EventUserMessage{}
			case "agent_message_delta":
				unmarshaled = &EventAgentMessageDelta{}
			case "agent_reasoning":
				unmarshaled = &EventAgentReasoning{}
			case "agent_reasoning_delta":
				unmarshaled = &EventAgentReasoningDelta{}
			case "exec_command_begin":
				unmarshaled = &EventExecCommandBegin{}
			case "exec_command_output_delta":
				unmarshaled = &EventExecCommandOutputDelta{}
			case "exec_command_end":
				unmarshaled = &EventExecCommandEnd{}
			case "agent_reasoning_raw_content":
				unmarshaled = &EventAgentReasoningRawContent{}
			case "agent_reasoning_raw_content_delta":
				unmarshaled = &EventAgentReasoningRawContentDelta{}
			case "agent_reasoning_section_break":
				unmarshaled = &EventAgentReasoningSectionBreak{}
			case "session_configured":
				unmarshaled = &EventSessionConfigured{}
			case "mcp_tool_call_begin":
				unmarshaled = &EventMcpToolCallBegin{}
			case "mcp_tool_call_end":
				unmarshaled = &EventMcpToolCallEnd{}
			case "web_search_begin":
				unmarshaled = &EventWebSearchBegin{}
			case "web_search_end":
				unmarshaled = &EventWebSearchEnd{}
			case "view_image_tool_call":
				unmarshaled = &EventViewImageToolCall{}
			case "background_event":
				unmarshaled = &EventBackgroundEvent{}
			case "resource_list_changed":
				unmarshaled = &EventResourceListChanged{}
			case "stream_error":
				unmarshaled = &EventStreamError{}
			case "patch_apply_begin":
				unmarshaled = &EventPatchApplyBegin{}
			case "patch_apply_end":
				unmarshaled = &EventPatchApplyEnd{}
			case "turn_diff":
				unmarshaled = &EventTurnDiff{}
			case "get_history_entry_response":
				unmarshaled = &EventGetHistoryEntryResponse{}
			case "mcp_list_tools_response":
				unmarshaled = &EventMcpListToolsResponse{}
			case "list_custom_prompts_response":
				unmarshaled = &EventListCustomPromptsResponse{}
			case "plan_update":
				unmarshaled = &EventPlanUpdate{}
			case "turn_aborted":
				unmarshaled = &EventTurnAborted{}
			case "conversation_path":
				unmarshaled = &EventConversationPath{}
			case "entered_review_mode":
				unmarshaled = &EventEnteredReviewMode{}
			case "exited_review_mode":
				unmarshaled = &EventExitedReviewMode{}
			case "raw_response_item":
				unmarshaled = &EventRawResponseItem{}
			case "item_started":
				unmarshaled = &EventItemStarted{}
			case "item_completed":
				unmarshaled = &EventItemCompleted{}
			default:
				t.Fatalf("Unknown event type: %s", typeCheck.Type)
			}

			if err := json.Unmarshal(got, unmarshaled); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			// Marshal again to compare
			got2, err := json.Marshal(unmarshaled)
			if err != nil {
				t.Fatalf("Marshal() after Unmarshal() error = %v", err)
			}

			if string(got) != string(got2) {
				t.Errorf("Round-trip failed: original = %s, after = %s", string(got), string(got2))
			}
		})
	}
}

// TestSubmissionStructure tests the Submission wrapper
func TestSubmissionStructure(t *testing.T) {
	sub := Submission{
		ID: "sub-123",
		Op: &OpInterrupt{},
	}

	data, err := json.Marshal(sub)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var unmarshaled Submission
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if unmarshaled.ID != sub.ID {
		t.Errorf("ID = %v, want %v", unmarshaled.ID, sub.ID)
	}

	if unmarshaled.Op.OpType() != "interrupt" {
		t.Errorf("Op type = %v, want interrupt", unmarshaled.Op.OpType())
	}
}

// TestEventStructure tests the Event wrapper
func TestEventStructure(t *testing.T) {
	event := Event{
		ID: "evt-123",
		Msg: &EventAgentMessage{
			Message: "Hello",
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var unmarshaled Event
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if unmarshaled.ID != event.ID {
		t.Errorf("ID = %v, want %v", unmarshaled.ID, event.ID)
	}

	if unmarshaled.Msg.EventType() != "agent_message" {
		t.Errorf("Msg type = %v, want agent_message", unmarshaled.Msg.EventType())
	}
}

// TestTokenUsage tests token usage calculation helpers
func TestTokenUsage(t *testing.T) {
	usage := TokenUsage{
		InputTokens:           1000,
		CachedInputTokens:     500,
		OutputTokens:          200,
		ReasoningOutputTokens: 100,
		TotalTokens:           1200,
	}

	if usage.CachedInput() != 500 {
		t.Errorf("CachedInput() = %v, want 500", usage.CachedInput())
	}

	if usage.NonCachedInput() != 500 {
		t.Errorf("NonCachedInput() = %v, want 500", usage.NonCachedInput())
	}

	if usage.BlendedTotal() != 700 {
		t.Errorf("BlendedTotal() = %v, want 700", usage.BlendedTotal())
	}

	if usage.TokensInContextWindow() != 1100 {
		t.Errorf("TokensInContextWindow() = %v, want 1100", usage.TokensInContextWindow())
	}
}

// TestGoldenFiles tests against golden JSON files
func TestGoldenFiles(t *testing.T) {
	fixturesDir := filepath.Join("..", "..", "test", "testdata", "fixtures", "protocol")

	tests := []struct {
		name     string
		filename string
		target   interface{}
	}{
		{
			name:     "user_turn",
			filename: "user_turn.json",
			target:   &OpUserTurn{},
		},
		{
			name:     "exec_approval",
			filename: "exec_approval.json",
			target:   &OpExecApproval{},
		},
		{
			name:     "patch_approval",
			filename: "patch_approval.json",
			target:   &OpPatchApproval{},
		},
		{
			name:     "interrupt",
			filename: "interrupt.json",
			target:   &OpInterrupt{},
		},
		{
			name:     "event_agent_message",
			filename: "event_agent_message.json",
			target:   &EventAgentMessage{},
		},
		{
			name:     "event_task_started",
			filename: "event_task_started.json",
			target:   &EventTaskStarted{},
		},
		{
			name:     "event_exec_command_begin",
			filename: "event_exec_command_begin.json",
			target:   &EventExecCommandBegin{},
		},
		{
			name:     "event_exec_command_end",
			filename: "event_exec_command_end.json",
			target:   &EventExecCommandEndEvent{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(fixturesDir, tt.filename)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Skipf("Fixture file not found: %v", err)
				return
			}

			// Test that we can unmarshal the golden file
			if err := json.Unmarshal(data, tt.target); err != nil {
				t.Errorf("Unmarshal() error = %v", err)
			}

			// Test that we can marshal it back
			marshaled, err := json.Marshal(tt.target)
			if err != nil {
				t.Errorf("Marshal() error = %v", err)
			}

			// Verify round-trip
			var roundTrip interface{}
			if err := json.Unmarshal(marshaled, &roundTrip); err != nil {
				t.Errorf("Round-trip unmarshal error = %v", err)
			}
		})
	}
}

// TestResponseItems tests message and response item types
func TestResponseItems(t *testing.T) {
	tests := []struct {
		name string
		item ResponseItem
		want string
	}{
		{
			name: "Message",
			item: &ResponseItemMessage{
				Role: "assistant",
				Content: []ContentItem{
					{Type: "output_text", Text: strPtr("Hello")},
				},
			},
			want: `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello"}]}`,
		},
		{
			name: "Reasoning",
			item: &ResponseItemReasoning{
				Summary: []ReasoningSummary{
					{Type: "summary_text", Text: strPtr("Thinking...")},
				},
			},
			want: `{"type":"reasoning","summary":[{"type":"summary_text","text":"Thinking..."}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.item)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			var gotNorm, wantNorm interface{}
			json.Unmarshal(got, &gotNorm)
			json.Unmarshal([]byte(tt.want), &wantNorm)

			gotJSON, _ := json.Marshal(gotNorm)
			wantJSON, _ := json.Marshal(wantNorm)

			if string(gotJSON) != string(wantJSON) {
				t.Errorf("Marshal() = %s, want %s", string(gotJSON), string(wantJSON))
			}
		})
	}
}

// TestSandboxPolicy tests sandbox policy serialization
func TestSandboxPolicy(t *testing.T) {
	tests := []struct {
		name   string
		policy SandboxPolicy
		want   string
	}{
		{
			name:   "read-only",
			policy: SandboxPolicy{Mode: "read-only"},
			want:   `{"mode":"read-only"}`,
		},
		{
			name: "workspace-write",
			policy: SandboxPolicy{
				Mode:                "workspace-write",
				WritableRoots:       []string{},
				NetworkAccess:       false,
				ExcludeTmpdirEnvVar: false,
				ExcludeSlashTmp:     false,
			},
			want: `{"mode":"workspace-write","writable_roots":[],"network_access":false,"exclude_tmpdir_env_var":false,"exclude_slash_tmp":false}`,
		},
		{
			name:   "danger-full-access",
			policy: SandboxPolicy{Mode: "danger-full-access"},
			want:   `{"mode":"danger-full-access"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.policy)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			if string(got) != tt.want {
				t.Errorf("Marshal() = %s, want %s", string(got), tt.want)
			}

			var unmarshaled SandboxPolicy
			if err := json.Unmarshal(got, &unmarshaled); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
		})
	}
}

// Helper functions
func strPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}

// TestDurationParsing tests that durations are properly handled
func TestDurationParsing(t *testing.T) {
	event := &EventExecCommandEnd{
		CallID:           "call-123",
		Stdout:           "",
		Stderr:           "",
		AggregatedOutput: "",
		ExitCode:         0,
		Duration:         "1.5s",
		FormattedOutput:  "",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var unmarshaled EventExecCommandEnd
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if unmarshaled.Duration != "1.5s" {
		t.Errorf("Duration = %v, want 1.5s", unmarshaled.Duration)
	}
}

// TestReviewDecision tests review decision enum
func TestReviewDecision(t *testing.T) {
	decisions := []string{"approved", "approved_for_session", "denied", "abort"}

	for _, d := range decisions {
		jsonStr := `{"type":"exec_approval","id":"test","decision":"` + d + `"}`
		var op OpExecApproval
		if err := json.Unmarshal([]byte(jsonStr), &op); err != nil {
			t.Errorf("Failed to unmarshal decision %s: %v", d, err)
		}
		if op.Decision != d {
			t.Errorf("Decision = %v, want %s", op.Decision, d)
		}
	}
}

// TestAskForApproval tests approval policy enum
func TestAskForApproval(t *testing.T) {
	policies := []string{"untrusted", "on-failure", "on-request", "never"}

	for _, p := range policies {
		op := OpUserTurn{
			Items:          []UserInput{{Type: "text", Text: strPtr("test")}},
			Cwd:            "/test",
			ApprovalPolicy: p,
			SandboxPolicy:  SandboxPolicy{Mode: "read-only"},
			Model:          "test-model",
			Summary:        "auto",
		}

		data, err := json.Marshal(op)
		if err != nil {
			t.Errorf("Marshal() error for policy %s: %v", p, err)
			continue
		}

		var unmarshaled OpUserTurn
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Errorf("Unmarshal() error for policy %s: %v", p, err)
			continue
		}

		if unmarshaled.ApprovalPolicy != p {
			t.Errorf("ApprovalPolicy = %v, want %s", unmarshaled.ApprovalPolicy, p)
		}
	}
}

// Benchmark tests
func BenchmarkOpMarshal(b *testing.B) {
	op := &OpUserTurn{
		Items: []UserInput{
			{Type: "text", Text: strPtr("Hello world")},
		},
		Cwd:            "/test/path",
		ApprovalPolicy: "on-request",
		SandboxPolicy:  SandboxPolicy{Mode: "read-only"},
		Model:          "claude-3-5-sonnet-20241022",
		Summary:        "auto",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(op)
	}
}

func BenchmarkEventMsgMarshal(b *testing.B) {
	event := &EventAgentMessage{
		Message: "Hello from agent",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(event)
	}
}
