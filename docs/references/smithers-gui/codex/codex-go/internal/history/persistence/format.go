package persistence

import (
	"encoding/json"
	"fmt"

	"github.com/evmts/codex/codex-go/internal/protocol"
)

// MarshalHistoryLine marshals a Submission or Event into a single JSON line
// without any trailing newline. The caller is responsible for adding newlines
// when writing to JSONL format.
func MarshalHistoryLine(item interface{}) ([]byte, error) {
	if item == nil {
		return nil, fmt.Errorf("cannot marshal nil item")
	}

	// Check if it's a valid type
	switch v := item.(type) {
	case *protocol.Submission:
		return json.Marshal(v)
	case *protocol.Event:
		return json.Marshal(v)
	default:
		return nil, fmt.Errorf("unsupported type for history line: %T", item)
	}
}

// UnmarshalHistoryLine unmarshals a single JSON line into either a Submission or Event.
// Returns (submission, nil, nil) for submissions, (nil, event, nil) for events,
// or (nil, nil, error) on failure.
func UnmarshalHistoryLine(data []byte) (*protocol.Submission, *protocol.Event, error) {
	if len(data) == 0 {
		return nil, nil, fmt.Errorf("cannot unmarshal empty data")
	}

	// Try to determine if it's a submission or event by looking for "op" or "msg" field
	var typeCheck struct {
		Op  json.RawMessage `json:"op"`
		Msg json.RawMessage `json:"msg"`
	}

	if err := json.Unmarshal(data, &typeCheck); err != nil {
		return nil, nil, fmt.Errorf("failed to parse history line: %w", err)
	}

	// If it has an "op" field, it's a submission
	if len(typeCheck.Op) > 0 {
		var submission protocol.Submission
		if err := json.Unmarshal(data, &submission); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal submission: %w", err)
		}
		return &submission, nil, nil
	}

	// If it has a "msg" field, it's an event
	if len(typeCheck.Msg) > 0 {
		var event protocol.Event
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal event: %w", err)
		}
		return nil, &event, nil
	}

	return nil, nil, fmt.Errorf("history line is neither a submission nor an event")
}
