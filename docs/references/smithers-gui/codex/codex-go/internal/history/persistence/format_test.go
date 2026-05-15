package persistence

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalHistoryLine(t *testing.T) {
	tests := []struct {
		name    string
		item    interface{}
		wantErr bool
	}{
		{
			name: "marshal submission",
			item: &protocol.Submission{
				ID: "test-id",
				Op: &protocol.OpInterrupt{},
			},
			wantErr: false,
		},
		{
			name: "marshal event",
			item: &protocol.Event{
				ID: "test-id",
				Msg: &protocol.EventError{
					Message: "test error",
				},
			},
			wantErr: false,
		},
		{
			name: "marshal op user turn",
			item: &protocol.Submission{
				ID: "turn-1",
				Op: &protocol.OpUserTurn{
					Items: []protocol.UserInput{
						{
							Type: "text",
							Text: stringPtr("hello"),
						},
					},
					Cwd:            "/test",
					ApprovalPolicy: "auto",
					SandboxPolicy: protocol.SandboxPolicy{
						Mode: "unrestricted",
					},
					Model:   "claude-3-5-sonnet-20241022",
					Summary: "auto",
				},
			},
			wantErr: false,
		},
		{
			name: "marshal event with task started",
			item: &protocol.Event{
				ID: "event-1",
				Msg: &protocol.EventTaskStarted{
					ModelContextWindow: int64Ptr(200000),
				},
			},
			wantErr: false,
		},
		{
			name:    "marshal invalid type",
			item:    "invalid",
			wantErr: true,
		},
		{
			name:    "marshal nil",
			item:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := MarshalHistoryLine(tt.item)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Verify it's valid JSON
			var decoded interface{}
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err, "marshaled data should be valid JSON")

			// Verify no newline in data
			assert.False(t, bytes.Contains(data, []byte("\n")), "marshaled data should not contain newlines")
		})
	}
}

func TestUnmarshalHistoryLine(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantSub bool
		wantEvt bool
		wantErr bool
	}{
		{
			name:    "unmarshal submission interrupt",
			input:   `{"id":"test-id","op":{"type":"interrupt"}}`,
			wantSub: true,
		},
		{
			name:    "unmarshal submission user turn",
			input:   `{"id":"turn-1","op":{"type":"user_turn","items":[{"type":"text","text":"hello"}],"cwd":"/test","approval_policy":"auto","sandbox_policy":{"mode":"unrestricted"},"model":"claude-3-5-sonnet-20241022","summary":"auto"}}`,
			wantSub: true,
		},
		{
			name:    "unmarshal event error",
			input:   `{"id":"test-id","msg":{"type":"error","message":"test error"}}`,
			wantEvt: true,
		},
		{
			name:    "unmarshal event task started",
			input:   `{"id":"event-1","msg":{"type":"task_started","model_context_window":200000}}`,
			wantEvt: true,
		},
		{
			name:    "unmarshal event agent message",
			input:   `{"id":"msg-1","msg":{"type":"agent_message","message":"Hello world"}}`,
			wantEvt: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   `{invalid}`,
			wantErr: true,
		},
		{
			name:    "neither submission nor event",
			input:   `{"other":"data"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub, evt, err := UnmarshalHistoryLine([]byte(tt.input))

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.wantSub {
				assert.NotNil(t, sub)
				assert.Nil(t, evt)
				assert.NotEmpty(t, sub.ID)
			} else if tt.wantEvt {
				assert.Nil(t, sub)
				assert.NotNil(t, evt)
				assert.NotEmpty(t, evt.ID)
			}
		})
	}
}

func TestRoundTripSubmission(t *testing.T) {
	original := &protocol.Submission{
		ID: "round-trip-1",
		Op: &protocol.OpUserInput{
			Items: []protocol.UserInput{
				{
					Type: "text",
					Text: stringPtr("test input"),
				},
			},
		},
	}

	// Marshal
	data, err := MarshalHistoryLine(original)
	require.NoError(t, err)

	// Unmarshal
	sub, evt, err := UnmarshalHistoryLine(data)
	require.NoError(t, err)
	assert.NotNil(t, sub)
	assert.Nil(t, evt)

	// Compare
	assert.Equal(t, original.ID, sub.ID)
	assert.Equal(t, original.Op.OpType(), sub.Op.OpType())
}

func TestRoundTripEvent(t *testing.T) {
	original := &protocol.Event{
		ID: "round-trip-event",
		Msg: &protocol.EventAgentMessage{
			Message: "Test message",
		},
	}

	// Marshal
	data, err := MarshalHistoryLine(original)
	require.NoError(t, err)

	// Unmarshal
	sub, evt, err := UnmarshalHistoryLine(data)
	require.NoError(t, err)
	assert.Nil(t, sub)
	assert.NotNil(t, evt)

	// Compare
	assert.Equal(t, original.ID, evt.ID)
	assert.Equal(t, original.Msg.EventType(), evt.Msg.EventType())
}

func TestMarshalMultipleLines(t *testing.T) {
	items := []interface{}{
		&protocol.Submission{
			ID: "1",
			Op: &protocol.OpInterrupt{},
		},
		&protocol.Event{
			ID:  "2",
			Msg: &protocol.EventError{Message: "error"},
		},
		&protocol.Submission{
			ID: "3",
			Op: &protocol.OpShutdown{},
		},
	}

	var lines []string
	for _, item := range items {
		data, err := MarshalHistoryLine(item)
		require.NoError(t, err)
		lines = append(lines, string(data))
	}

	// Join with newlines to simulate JSONL format
	jsonl := strings.Join(lines, "\n")
	assert.NotEmpty(t, jsonl)

	// Verify we can read back line by line
	for i, line := range strings.Split(jsonl, "\n") {
		sub, evt, err := UnmarshalHistoryLine([]byte(line))
		require.NoError(t, err, "failed to unmarshal line %d", i)

		if i%2 == 0 {
			assert.NotNil(t, sub, "line %d should be submission", i)
		} else {
			assert.NotNil(t, evt, "line %d should be event", i)
		}
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}
