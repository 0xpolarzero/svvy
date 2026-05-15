package protocol

import (
	"encoding/json"
	"testing"
)

func TestProgressEventSerialization(t *testing.T) {
	tests := []struct {
		name     string
		event    EventMsg
		expected string
	}{
		{
			name: "OperationStarted with all fields",
			event: &EventOperationStarted{
				Operation:         "file_read",
				EstimatedDuration: ptr(int64(1000)),
				Total:             ptr(int64(5000)),
				Message:           "Reading large file",
			},
			expected: `{"type":"operation_started","operation":"file_read","estimated_duration_ms":1000,"total":5000,"message":"Reading large file"}`,
		},
		{
			name: "OperationStarted minimal",
			event: &EventOperationStarted{
				Operation: "file_write",
			},
			expected: `{"type":"operation_started","operation":"file_write"}`,
		},
		{
			name: "OperationProgress with percentage",
			event: &EventOperationProgress{
				Operation:  "patch_apply",
				Current:    250,
				Total:      ptr(int64(1000)),
				Percentage: ptrFloat(25.0),
				Message:    "Applying patches",
			},
			expected: `{"type":"operation_progress","operation":"patch_apply","current":250,"total":1000,"percentage":25,"message":"Applying patches"}`,
		},
		{
			name: "OperationProgress without total",
			event: &EventOperationProgress{
				Operation: "mcp_discovery",
				Current:   5,
				Message:   "Discovering tools",
			},
			expected: `{"type":"operation_progress","operation":"mcp_discovery","current":5,"message":"Discovering tools"}`,
		},
		{
			name: "OperationCompleted success",
			event: &EventOperationCompleted{
				Operation:      "file_read",
				DurationMs:     523,
				Status:         "success",
				Message:        "File read complete",
				ProcessedCount: ptr(int64(1024000)),
			},
			expected: `{"type":"operation_completed","operation":"file_read","duration_ms":523,"status":"success","message":"File read complete","processed_count":1024000}`,
		},
		{
			name: "OperationCompleted failed",
			event: &EventOperationCompleted{
				Operation:  "patch_apply",
				DurationMs: 150,
				Status:     "failed",
				Message:    "Patch conflict detected",
			},
			expected: `{"type":"operation_completed","operation":"patch_apply","duration_ms":150,"status":"failed","message":"Patch conflict detected"}`,
		},
		{
			name: "OperationCompleted cancelled",
			event: &EventOperationCompleted{
				Operation:  "history_reconstruct",
				DurationMs: 2500,
				Status:     "cancelled",
				Message:    "User cancelled operation",
			},
			expected: `{"type":"operation_completed","operation":"history_reconstruct","duration_ms":2500,"status":"cancelled","message":"User cancelled operation"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			data, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("Failed to marshal event: %v", err)
			}

			// Parse both JSONs to compare semantically (order-independent)
			var got, expected map[string]interface{}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Failed to unmarshal actual JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &expected); err != nil {
				t.Fatalf("Failed to unmarshal expected JSON: %v", err)
			}

			// Compare as JSON objects
			gotJSON, _ := json.Marshal(got)
			expectedJSON, _ := json.Marshal(expected)
			if string(gotJSON) != string(expectedJSON) {
				t.Errorf("Marshal mismatch:\nGot:      %s\nExpected: %s", string(gotJSON), string(expectedJSON))
			}

			// Test unmarshaling
			var event Event
			fullJSON := `{"id":"test-id","msg":` + string(data) + `}`
			if err := json.Unmarshal([]byte(fullJSON), &event); err != nil {
				t.Fatalf("Failed to unmarshal event: %v", err)
			}

			// Verify event type
			if event.Msg.EventType() != tt.event.EventType() {
				t.Errorf("Event type mismatch: got %s, expected %s",
					event.Msg.EventType(), tt.event.EventType())
			}
		})
	}
}

func TestProgressEventRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		event EventMsg
	}{
		{
			name: "OperationStarted",
			event: &EventOperationStarted{
				Operation:         "file_read",
				EstimatedDuration: ptr(int64(2000)),
				Total:             ptr(int64(10000)),
				Message:           "Reading configuration",
			},
		},
		{
			name: "OperationProgress",
			event: &EventOperationProgress{
				Operation:  "patch_apply",
				Current:    750,
				Total:      ptr(int64(1000)),
				Percentage: ptrFloat(75.0),
				Message:    "Applying changes",
			},
		},
		{
			name: "OperationCompleted",
			event: &EventOperationCompleted{
				Operation:      "file_write",
				DurationMs:     456,
				Status:         "success",
				Message:        "Write successful",
				ProcessedCount: ptr(int64(2048)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Wrap in Event struct
			original := Event{
				ID:  "test-123",
				Msg: tt.event,
			}

			// Marshal
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			// Unmarshal
			var decoded Event
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// Verify ID
			if decoded.ID != original.ID {
				t.Errorf("ID mismatch: got %s, expected %s", decoded.ID, original.ID)
			}

			// Verify event type
			if decoded.Msg.EventType() != original.Msg.EventType() {
				t.Errorf("Event type mismatch: got %s, expected %s",
					decoded.Msg.EventType(), original.Msg.EventType())
			}

			// Marshal again and compare JSON
			data2, err := json.Marshal(decoded)
			if err != nil {
				t.Fatalf("Second marshal failed: %v", err)
			}

			if string(data) != string(data2) {
				t.Errorf("Round-trip JSON mismatch:\nFirst:  %s\nSecond: %s",
					string(data), string(data2))
			}
		})
	}
}

// Helper function to create pointer to int64
func ptr(v int64) *int64 {
	return &v
}

// Helper function to create pointer to float64
func ptrFloat(v float64) *float64 {
	return &v
}
