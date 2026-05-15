// Package notify provides notification functionality for Codex events.
// It enables running external scripts when specific events occur during
// conversation turns, such as turn completion, errors, or approval requests.
package notify

import (
	"time"
)

// EventType represents the type of notification event.
type EventType string

const (
	// EventTurnComplete is triggered when a turn completes successfully
	EventTurnComplete EventType = "turn_complete"

	// EventTurnError is triggered when a turn encounters an error
	EventTurnError EventType = "turn_error"

	// EventApprovalNeeded is triggered when user approval is required
	EventApprovalNeeded EventType = "approval_needed"

	// EventTurnAborted is triggered when a turn is interrupted/aborted
	EventTurnAborted EventType = "turn_aborted"
)

// NotificationEvent represents a notification event with associated metadata.
// This structure provides all the context needed for notification scripts
// to understand what happened in the session.
type NotificationEvent struct {
	// Type is the event type (turn_complete, turn_error, etc.)
	Type EventType

	// SessionID is the conversation session identifier
	SessionID string

	// TurnID is the specific turn identifier
	TurnID string

	// Timestamp is when the event occurred
	Timestamp time.Time

	// Status is the final status of the turn ("success", "error", "aborted")
	Status string

	// Message is an optional human-readable message
	Message string

	// ErrorMessage is populated when Status is "error"
	ErrorMessage string

	// Metadata contains additional key-value pairs for the event
	Metadata map[string]string
}

// NewNotificationEvent creates a new notification event.
func NewNotificationEvent(eventType EventType, sessionID, turnID string) *NotificationEvent {
	return &NotificationEvent{
		Type:      eventType,
		SessionID: sessionID,
		TurnID:    turnID,
		Timestamp: time.Now(),
		Metadata:  make(map[string]string),
	}
}

// WithStatus sets the status and returns the event for chaining.
func (e *NotificationEvent) WithStatus(status string) *NotificationEvent {
	e.Status = status
	return e
}

// WithMessage sets the message and returns the event for chaining.
func (e *NotificationEvent) WithMessage(msg string) *NotificationEvent {
	e.Message = msg
	return e
}

// WithError sets the error message and returns the event for chaining.
func (e *NotificationEvent) WithError(err string) *NotificationEvent {
	e.ErrorMessage = err
	e.Status = "error"
	return e
}

// WithMetadata adds a metadata key-value pair and returns the event for chaining.
func (e *NotificationEvent) WithMetadata(key, value string) *NotificationEvent {
	if e.Metadata == nil {
		e.Metadata = make(map[string]string)
	}
	e.Metadata[key] = value
	return e
}
