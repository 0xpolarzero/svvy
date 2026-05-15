package protocol

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventSandboxViolation_MarshalJSON(t *testing.T) {
	path := "/etc/passwd"
	syscall := "connect"

	event := &EventSandboxViolation{
		CallID:       "call-123",
		SandboxType:  "native",
		Operation:    "write",
		Path:         &path,
		Syscall:      &syscall,
		ErrorMessage: "permission denied",
		ExitCode:     1,
		Timestamp:    time.Now().Format(time.RFC3339),
	}

	// Marshal to JSON
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	// Verify required fields are present
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	expectedFields := []string{"type", "call_id", "sandbox_type", "operation", "error_message", "exit_code", "timestamp"}
	for _, field := range expectedFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("expected field %q not found in JSON", field)
		}
	}

	// Verify type is correct
	if raw["type"] != "sandbox_violation" {
		t.Errorf("expected type 'sandbox_violation', got %v", raw["type"])
	}

	// Verify optional fields are present
	if raw["path"] != "/etc/passwd" {
		t.Errorf("expected path '/etc/passwd', got %v", raw["path"])
	}
	if raw["syscall"] != "connect" {
		t.Errorf("expected syscall 'connect', got %v", raw["syscall"])
	}

	t.Logf("Marshaled JSON: %s", string(data))
}

func TestEventSandboxViolation_MarshalJSON_WithoutOptionalFields(t *testing.T) {
	event := &EventSandboxViolation{
		CallID:       "call-456",
		SandboxType:  "docker",
		Operation:    "network",
		ErrorMessage: "network access denied",
		ExitCode:     1,
		Timestamp:    time.Now().Format(time.RFC3339),
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	// Verify optional fields are not present
	if _, ok := raw["path"]; ok {
		t.Error("path should not be present when nil")
	}
	if _, ok := raw["syscall"]; ok {
		t.Error("syscall should not be present when nil")
	}

	t.Logf("Marshaled JSON: %s", string(data))
}

func TestEventSandboxViolation_UnmarshalJSON(t *testing.T) {
	jsonData := `{
		"type": "sandbox_violation",
		"call_id": "call-789",
		"sandbox_type": "kubernetes",
		"operation": "write",
		"path": "/var/log/app.log",
		"error_message": "operation not permitted",
		"exit_code": 1,
		"timestamp": "2025-01-15T10:30:00Z"
	}`

	var evt EventSandboxViolation
	if err := json.Unmarshal([]byte(jsonData), &evt); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify fields
	if evt.CallID != "call-789" {
		t.Errorf("expected CallID 'call-789', got %s", evt.CallID)
	}
	if evt.SandboxType != "kubernetes" {
		t.Errorf("expected SandboxType 'kubernetes', got %s", evt.SandboxType)
	}
	if evt.Operation != "write" {
		t.Errorf("expected Operation 'write', got %s", evt.Operation)
	}
	if evt.Path == nil || *evt.Path != "/var/log/app.log" {
		t.Errorf("expected Path '/var/log/app.log', got %v", evt.Path)
	}
	if evt.ExitCode != 1 {
		t.Errorf("expected ExitCode 1, got %d", evt.ExitCode)
	}
}

func TestEvent_UnmarshalJSON_SandboxViolation(t *testing.T) {
	jsonData := `{
		"id": "sub-123",
		"msg": {
			"type": "sandbox_violation",
			"call_id": "call-999",
			"sandbox_type": "seatbelt",
			"operation": "read",
			"syscall": "open",
			"error_message": "seatbelt denied",
			"exit_code": 1,
			"timestamp": "2025-01-15T10:30:00Z"
		}
	}`

	var event Event
	if err := json.Unmarshal([]byte(jsonData), &event); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	// Verify event ID
	if event.ID != "sub-123" {
		t.Errorf("expected ID 'sub-123', got %s", event.ID)
	}

	// Verify message type
	violation, ok := event.Msg.(*EventSandboxViolation)
	if !ok {
		t.Fatalf("expected EventSandboxViolation, got %T", event.Msg)
	}

	// Verify violation fields
	if violation.CallID != "call-999" {
		t.Errorf("expected CallID 'call-999', got %s", violation.CallID)
	}
	if violation.SandboxType != "seatbelt" {
		t.Errorf("expected SandboxType 'seatbelt', got %s", violation.SandboxType)
	}
	if violation.Syscall == nil || *violation.Syscall != "open" {
		t.Errorf("expected Syscall 'open', got %v", violation.Syscall)
	}
}

func TestEventSandboxViolation_EventType(t *testing.T) {
	event := &EventSandboxViolation{}
	if event.EventType() != "sandbox_violation" {
		t.Errorf("expected EventType 'sandbox_violation', got %s", event.EventType())
	}
}
