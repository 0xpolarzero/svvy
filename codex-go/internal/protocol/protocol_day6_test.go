package protocol

import (
    "encoding/base64"
    "encoding/json"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestEventToolCallApprovalNeeded_MarshalUnmarshal(t *testing.T) {
    evt := &Event{ID: "sub-1", Msg: &EventToolCallApprovalNeeded{
        CallID:          "call-1",
        ToolName:        "shell",
        Command:         []string{"sh", "-c", "echo"},
        WorkingDirectory: ".",
        Justification:   "test",
        IsRetry:         true,
        RetryReason:     "perm",
        RiskLevel:       "high",
        RiskReasons:     []string{"writes"},
        RiskMitigation:  "sandbox",
    }}

    data, err := json.Marshal(evt)
    require.NoError(t, err)

    var round Event
    require.NoError(t, json.Unmarshal(data, &round))
    got, ok := round.Msg.(*EventToolCallApprovalNeeded)
    require.True(t, ok)
    assert.Equal(t, "call-1", got.CallID)
    assert.Equal(t, "shell", got.ToolName)
    assert.Equal(t, ".", got.WorkingDirectory)
    assert.Equal(t, "high", got.RiskLevel)
    assert.True(t, got.IsRetry)
}

func TestSandboxPolicy_MarshalJSON(t *testing.T) {
    // Workspace-write should include all fields
    s := SandboxPolicy{Mode: "workspace-write", WritableRoots: []string{"/tmp"}, NetworkAccess: true, ExcludeTmpdirEnvVar: true, ExcludeSlashTmp: true}
    b, err := json.Marshal(s)
    require.NoError(t, err)
    var m map[string]interface{}
    require.NoError(t, json.Unmarshal(b, &m))
    assert.Equal(t, "workspace-write", m["mode"])
    assert.Contains(t, m, "writable_roots")
    assert.Contains(t, m, "network_access")

    // Other modes keep minimum required fields
    s = SandboxPolicy{Mode: "native"}
    b, err = json.Marshal(s)
    require.NoError(t, err)
    require.NoError(t, json.Unmarshal(b, &m))
    assert.Equal(t, "native", m["mode"])
}

func TestEventTypes_MarshalUnmarshal(t *testing.T) {
    tests := []struct {
        name  string
        event *Event
    }{
        {
            name: "EventError",
            event: &Event{
                ID: "test-1",
                Msg: &EventError{
                    Message: "test error message",
                },
            },
        },
        {
            name: "EventTaskStarted",
            event: &Event{
                ID: "test-2",
                Msg: &EventTaskStarted{
                    ModelContextWindow: int64Ptr(200000),
                },
            },
        },
        {
            name: "EventTaskComplete",
            event: &Event{
                ID: "test-3",
                Msg: &EventTaskComplete{
                    LastAgentMessage: stringPtr("done"),
                },
            },
        },
        {
            name: "EventAgentMessage",
            event: &Event{
                ID: "test-4",
                Msg: &EventAgentMessage{
                    Message: "Hello",
                },
            },
        },
        {
            name: "EventAgentMessageDelta",
            event: &Event{
                ID: "test-5",
                Msg: &EventAgentMessageDelta{
                    Delta: "H",
                },
            },
        },
        {
            name: "EventUserMessage",
            event: &Event{
                ID: "test-6",
                Msg: &EventUserMessage{
                    Message: "test user message",
                },
            },
        },
        {
            name: "EventAgentReasoning",
            event: &Event{
                ID: "test-7",
                Msg: &EventAgentReasoning{
                    Text: "thinking...",
                },
            },
        },
        {
            name: "EventAgentReasoningDelta",
            event: &Event{
                ID: "test-8",
                Msg: &EventAgentReasoningDelta{
                    Delta: "...",
                },
            },
        },
        {
            name: "EventExecCommandBegin",
            event: &Event{
                ID: "test-9",
                Msg: &EventExecCommandBegin{
                    CallID:    "call-1",
                    Command:   []string{"echo", "test"},
                    ParsedCmd: []interface{}{"echo", "test"},
                    Cwd:       "/tmp",
                },
            },
        },
        {
            name: "EventExecCommandOutputDelta",
            event: &Event{
                ID: "test-10",
                Msg: &EventExecCommandOutputDelta{
                    CallID: "call-1",
                    Stream: "stdout",
                    Chunk:  "b3V0cHV0", // base64 encoded "output"
                },
            },
        },
        {
            name: "EventExecCommandEnd",
            event: &Event{
                ID: "test-11",
                Msg: &EventExecCommandEnd{
                    CallID:           "call-1",
                    ExitCode:         0,
                    Duration:         "100ms",
                    Stdout:           "output",
                    Stderr:           "",
                    AggregatedOutput: "output",
                    FormattedOutput:  "output",
                },
            },
        },
        {
            name: "EventTokenCount",
            event: &Event{
                ID: "test-12",
                Msg: &EventTokenCount{
                    Info: &TokenUsageInfo{
                        TotalTokenUsage: TokenUsage{
                            InputTokens:  100,
                            OutputTokens: 50,
                            TotalTokens:  150,
                        },
                        LastTokenUsage: TokenUsage{
                            InputTokens:  10,
                            OutputTokens: 5,
                            TotalTokens:  15,
                        },
                        ModelContextWindow: int64Ptr(200000),
                    },
                },
            },
        },
        {
            name: "EventShutdownComplete",
            event: &Event{
                ID:  "test-13",
                Msg: &EventShutdownComplete{},
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            data, err := json.Marshal(tt.event)
            require.NoError(t, err)

            var roundTrip Event
            err = json.Unmarshal(data, &roundTrip)
            require.NoError(t, err)

            assert.Equal(t, tt.event.ID, roundTrip.ID)
            assert.NotNil(t, roundTrip.Msg)
        })
    }
}

func TestOperationTypes_MarshalUnmarshal(t *testing.T) {
    tests := []struct {
        name       string
        submission *Submission
    }{
        {
            name: "OpInterrupt",
            submission: &Submission{
                ID: "sub-1",
                Op: &OpInterrupt{},
            },
        },
        {
            name: "OpUserInput",
            submission: &Submission{
                ID: "sub-2",
                Op: &OpUserInput{
                    Items: []UserInput{
                        {Type: "text", Text: stringPtr("hello")},
                    },
                },
            },
        },
        {
            name: "OpOverrideTurnContext",
            submission: &Submission{
                ID: "sub-3",
                Op: &OpOverrideTurnContext{
                    Cwd:            stringPtr("/new/path"),
                    ApprovalPolicy: stringPtr("manual"),
                    Model:          stringPtr("gpt-4"),
                },
            },
        },
        {
            name: "OpExecApproval",
            submission: &Submission{
                ID: "sub-4",
                Op: &OpExecApproval{
                    ID:       "sub-4",
                    Decision: "approve",
                },
            },
        },
        {
            name: "OpPatchApproval",
            submission: &Submission{
                ID: "sub-5",
                Op: &OpPatchApproval{
                    ID:       "sub-5",
                    Decision: "deny",
                },
            },
        },
        {
            name: "OpShutdown",
            submission: &Submission{
                ID: "sub-6",
                Op: &OpShutdown{},
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            data, err := json.Marshal(tt.submission)
            require.NoError(t, err)

            var roundTrip Submission
            err = json.Unmarshal(data, &roundTrip)
            require.NoError(t, err)

            assert.Equal(t, tt.submission.ID, roundTrip.ID)
            assert.NotNil(t, roundTrip.Op)
        })
    }
}

// Helper functions
// Note: intPtr and int64Ptr are defined in protocol_test.go

func stringPtr(s string) *string {
    return &s
}

// TestEventExecCommandOutputDelta_BinaryData tests that binary data is properly base64 encoded
func TestEventExecCommandOutputDelta_BinaryData(t *testing.T) {
    tests := []struct {
        name       string
        binaryData []byte
        wantChunk  string // expected base64 encoded value
    }{
        {
            name:       "null bytes",
            binaryData: []byte{0x00, 0x01, 0x02, 0x03},
            wantChunk:  "AAECAw==",
        },
        {
            name:       "non-UTF8 data",
            binaryData: []byte{0xFF, 0xFE, 0xFD, 0xFC},
            wantChunk:  "//79/A==",
        },
        {
            name:       "mixed ASCII and binary",
            binaryData: []byte("hello\x00world\xFF"),
            wantChunk:  "aGVsbG8Ad29ybGT/",
        },
        {
            name:       "empty data",
            binaryData: []byte{},
            wantChunk:  "",
        },
        {
            name:       "regular text",
            binaryData: []byte("output"),
            wantChunk:  "b3V0cHV0",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create event with base64 encoded chunk
            encoded := base64.StdEncoding.EncodeToString(tt.binaryData)
            assert.Equal(t, tt.wantChunk, encoded, "base64 encoding mismatch")

            event := &Event{
                ID: "test-binary",
                Msg: &EventExecCommandOutputDelta{
                    CallID: "call-1",
                    Stream: "stdout",
                    Chunk:  encoded,
                },
            }

            // Marshal to JSON
            data, err := json.Marshal(event)
            require.NoError(t, err)

            // Unmarshal back
            var roundTrip Event
            err = json.Unmarshal(data, &roundTrip)
            require.NoError(t, err)

            // Verify the event
            outputDelta, ok := roundTrip.Msg.(*EventExecCommandOutputDelta)
            require.True(t, ok)
            assert.Equal(t, "call-1", outputDelta.CallID)
            assert.Equal(t, "stdout", outputDelta.Stream)
            assert.Equal(t, encoded, outputDelta.Chunk)

            // Decode and verify original data
            decoded, err := base64.StdEncoding.DecodeString(outputDelta.Chunk)
            require.NoError(t, err)
            assert.Equal(t, tt.binaryData, decoded)
        })
    }
}

