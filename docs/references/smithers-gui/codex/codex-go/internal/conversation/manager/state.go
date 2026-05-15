// Package manager provides the conversation manager for coordinating AI-assisted coding sessions.
package manager

import (
	"fmt"
	"sync"
)

// SessionState represents the current state of a conversation session.
type SessionState int

const (
	// StateIdle indicates the session is created but no turn is active
	StateIdle SessionState = iota

	// StateProcessingTurn indicates a user turn is being processed
	StateProcessingTurn

	// StateAwaitingApproval indicates the session is waiting for user approval (exec or patch)
	StateAwaitingApproval

	// StateInterrupted indicates the current turn was interrupted
	StateInterrupted

	// StateCompleted indicates the turn completed successfully
	StateCompleted

	// StateError indicates an error occurred during processing
	StateError

	// StateClosed indicates the session has been closed
	StateClosed
)

// String returns a human-readable representation of the state.
func (s SessionState) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateProcessingTurn:
		return "processing_turn"
	case StateAwaitingApproval:
		return "awaiting_approval"
	case StateInterrupted:
		return "interrupted"
	case StateCompleted:
		return "completed"
	case StateError:
		return "error"
	case StateClosed:
		return "closed"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// StateTransition represents a valid state transition in the conversation flow.
type StateTransition struct {
	From SessionState
	To   SessionState
}

// validTransitions defines all allowed state transitions.
var validTransitions = map[StateTransition]bool{
	// From Idle
	{StateIdle, StateProcessingTurn}: true,
	{StateIdle, StateClosed}:         true,

	// From ProcessingTurn
	{StateProcessingTurn, StateAwaitingApproval}: true,
	{StateProcessingTurn, StateCompleted}:        true,
	{StateProcessingTurn, StateError}:            true,
	{StateProcessingTurn, StateInterrupted}:      true,
	{StateProcessingTurn, StateClosed}:           true,

	// From AwaitingApproval
	{StateAwaitingApproval, StateProcessingTurn}: true,
	{StateAwaitingApproval, StateCompleted}:      true,
	{StateAwaitingApproval, StateError}:          true,
	{StateAwaitingApproval, StateInterrupted}:    true,
	{StateAwaitingApproval, StateClosed}:         true,

	// From Interrupted
	{StateInterrupted, StateIdle}:           true,
	{StateInterrupted, StateProcessingTurn}: true,
	{StateInterrupted, StateClosed}:         true,

	// From Completed
	{StateCompleted, StateIdle}:           true,
	{StateCompleted, StateProcessingTurn}: true,
	{StateCompleted, StateClosed}:         true,

	// From Error
	{StateError, StateIdle}:           true,
	{StateError, StateProcessingTurn}: true,
	{StateError, StateClosed}:         true,
}

// IsValidTransition checks if a state transition is allowed.
func IsValidTransition(from, to SessionState) bool {
	// Can't transition from closed state
	if from == StateClosed {
		return false
	}

	// Self-transitions are always invalid (except for closed)
	if from == to {
		return false
	}

	return validTransitions[StateTransition{from, to}]
}

// StateMachine manages the state of a conversation session with thread-safety.
type StateMachine struct {
	mu            sync.RWMutex
	currentState  SessionState
	previousState SessionState
	errorMessage  string
}

// NewStateMachine creates a new state machine starting in Idle state.
func NewStateMachine() *StateMachine {
	return &StateMachine{
		currentState:  StateIdle,
		previousState: StateIdle,
	}
}

// GetState returns the current state (thread-safe read).
func (sm *StateMachine) GetState() SessionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState
}

// GetPreviousState returns the previous state (thread-safe read).
func (sm *StateMachine) GetPreviousState() SessionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.previousState
}

// GetErrorMessage returns the error message if in error state.
func (sm *StateMachine) GetErrorMessage() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.errorMessage
}

// Transition attempts to transition to a new state.
// Returns an error if the transition is invalid.
func (sm *StateMachine) Transition(to SessionState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !IsValidTransition(sm.currentState, to) {
		return fmt.Errorf("invalid state transition from %s to %s", sm.currentState, to)
	}

	sm.previousState = sm.currentState
	sm.currentState = to

	// Clear error message on successful transition away from error state
	if sm.previousState == StateError && to != StateError {
		sm.errorMessage = ""
	}

	return nil
}

// TransitionToError transitions to error state with a message.
func (sm *StateMachine) TransitionToError(errMsg string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !IsValidTransition(sm.currentState, StateError) {
		return fmt.Errorf("cannot transition from %s to error state", sm.currentState)
	}

	sm.previousState = sm.currentState
	sm.currentState = StateError
	sm.errorMessage = errMsg

	return nil
}

// CanTransitionTo checks if a transition to the given state is valid without performing it.
func (sm *StateMachine) CanTransitionTo(to SessionState) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return IsValidTransition(sm.currentState, to)
}

// IsInState checks if the state machine is in the given state.
func (sm *StateMachine) IsInState(state SessionState) bool {
	return sm.GetState() == state
}

// IsTerminal checks if the state machine is in a terminal state (Closed).
func (sm *StateMachine) IsTerminal() bool {
	return sm.GetState() == StateClosed
}

// CanAcceptTurn checks if the session can accept a new turn.
func (sm *StateMachine) CanAcceptTurn() bool {
	state := sm.GetState()
	return state == StateIdle || state == StateCompleted || state == StateError || state == StateInterrupted
}
