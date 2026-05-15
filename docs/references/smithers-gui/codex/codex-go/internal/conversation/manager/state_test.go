package manager

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionStateString(t *testing.T) {
	tests := []struct {
		state    SessionState
		expected string
	}{
		{StateIdle, "idle"},
		{StateProcessingTurn, "processing_turn"},
		{StateAwaitingApproval, "awaiting_approval"},
		{StateInterrupted, "interrupted"},
		{StateCompleted, "completed"},
		{StateError, "error"},
		{StateClosed, "closed"},
		{SessionState(999), "unknown(999)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     SessionState
		to       SessionState
		expected bool
	}{
		// Valid transitions from Idle
		{"Idle to ProcessingTurn", StateIdle, StateProcessingTurn, true},
		{"Idle to Closed", StateIdle, StateClosed, true},

		// Invalid transitions from Idle
		{"Idle to AwaitingApproval", StateIdle, StateAwaitingApproval, false},
		{"Idle to Completed", StateIdle, StateCompleted, false},
		{"Idle to Error", StateIdle, StateError, false},

		// Valid transitions from ProcessingTurn
		{"ProcessingTurn to AwaitingApproval", StateProcessingTurn, StateAwaitingApproval, true},
		{"ProcessingTurn to Completed", StateProcessingTurn, StateCompleted, true},
		{"ProcessingTurn to Error", StateProcessingTurn, StateError, true},
		{"ProcessingTurn to Interrupted", StateProcessingTurn, StateInterrupted, true},
		{"ProcessingTurn to Closed", StateProcessingTurn, StateClosed, true},

		// Invalid transitions from ProcessingTurn
		{"ProcessingTurn to Idle", StateProcessingTurn, StateIdle, false},

		// Valid transitions from AwaitingApproval
		{"AwaitingApproval to ProcessingTurn", StateAwaitingApproval, StateProcessingTurn, true},
		{"AwaitingApproval to Completed", StateAwaitingApproval, StateCompleted, true},
		{"AwaitingApproval to Error", StateAwaitingApproval, StateError, true},
		{"AwaitingApproval to Interrupted", StateAwaitingApproval, StateInterrupted, true},
		{"AwaitingApproval to Closed", StateAwaitingApproval, StateClosed, true},

		// Valid transitions from Interrupted
		{"Interrupted to Idle", StateInterrupted, StateIdle, true},
		{"Interrupted to ProcessingTurn", StateInterrupted, StateProcessingTurn, true},
		{"Interrupted to Closed", StateInterrupted, StateClosed, true},

		// Valid transitions from Completed
		{"Completed to Idle", StateCompleted, StateIdle, true},
		{"Completed to ProcessingTurn", StateCompleted, StateProcessingTurn, true},
		{"Completed to Closed", StateCompleted, StateClosed, true},

		// Valid transitions from Error
		{"Error to Idle", StateError, StateIdle, true},
		{"Error to ProcessingTurn", StateError, StateProcessingTurn, true},
		{"Error to Closed", StateError, StateClosed, true},

		// Closed state cannot transition
		{"Closed to Idle", StateClosed, StateIdle, false},
		{"Closed to ProcessingTurn", StateClosed, StateProcessingTurn, false},
		{"Closed to Error", StateClosed, StateError, false},

		// Self-transitions are invalid
		{"Idle to Idle", StateIdle, StateIdle, false},
		{"ProcessingTurn to ProcessingTurn", StateProcessingTurn, StateProcessingTurn, false},
		{"Completed to Completed", StateCompleted, StateCompleted, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidTransition(tt.from, tt.to)
			assert.Equal(t, tt.expected, result,
				"IsValidTransition(%s, %s) = %v, want %v",
				tt.from, tt.to, result, tt.expected)
		})
	}
}

func TestNewStateMachine(t *testing.T) {
	sm := NewStateMachine()
	require.NotNil(t, sm)
	assert.Equal(t, StateIdle, sm.GetState())
	assert.Equal(t, StateIdle, sm.GetPreviousState())
	assert.Empty(t, sm.GetErrorMessage())
}

func TestStateMachine_Transition(t *testing.T) {
	t.Run("valid transition", func(t *testing.T) {
		sm := NewStateMachine()
		require.Equal(t, StateIdle, sm.GetState())

		err := sm.Transition(StateProcessingTurn)
		require.NoError(t, err)
		assert.Equal(t, StateProcessingTurn, sm.GetState())
		assert.Equal(t, StateIdle, sm.GetPreviousState())
	})

	t.Run("invalid transition", func(t *testing.T) {
		sm := NewStateMachine()
		require.Equal(t, StateIdle, sm.GetState())

		err := sm.Transition(StateAwaitingApproval)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid state transition")
		assert.Equal(t, StateIdle, sm.GetState()) // State unchanged
	})

	t.Run("chain of valid transitions", func(t *testing.T) {
		sm := NewStateMachine()

		// Idle -> ProcessingTurn
		err := sm.Transition(StateProcessingTurn)
		require.NoError(t, err)

		// ProcessingTurn -> AwaitingApproval
		err = sm.Transition(StateAwaitingApproval)
		require.NoError(t, err)
		assert.Equal(t, StateAwaitingApproval, sm.GetState())
		assert.Equal(t, StateProcessingTurn, sm.GetPreviousState())

		// AwaitingApproval -> ProcessingTurn
		err = sm.Transition(StateProcessingTurn)
		require.NoError(t, err)

		// ProcessingTurn -> Completed
		err = sm.Transition(StateCompleted)
		require.NoError(t, err)
		assert.Equal(t, StateCompleted, sm.GetState())
	})

	t.Run("cannot transition from closed state", func(t *testing.T) {
		sm := NewStateMachine()

		// Move to closed state
		err := sm.Transition(StateClosed)
		require.NoError(t, err)

		// Try to transition from closed (should fail)
		err = sm.Transition(StateIdle)
		require.Error(t, err)
		assert.Equal(t, StateClosed, sm.GetState())
	})
}

func TestStateMachine_TransitionToError(t *testing.T) {
	t.Run("transition to error with message", func(t *testing.T) {
		sm := NewStateMachine()
		sm.Transition(StateProcessingTurn)

		errMsg := "test error message"
		err := sm.TransitionToError(errMsg)
		require.NoError(t, err)
		assert.Equal(t, StateError, sm.GetState())
		assert.Equal(t, errMsg, sm.GetErrorMessage())
	})

	t.Run("error message cleared on transition away", func(t *testing.T) {
		sm := NewStateMachine()
		sm.Transition(StateProcessingTurn)

		// Transition to error
		sm.TransitionToError("test error")
		assert.Equal(t, "test error", sm.GetErrorMessage())

		// Transition away from error
		err := sm.Transition(StateIdle)
		require.NoError(t, err)
		assert.Empty(t, sm.GetErrorMessage())
	})
}

func TestStateMachine_CanTransitionTo(t *testing.T) {
	sm := NewStateMachine()

	// From Idle
	assert.True(t, sm.CanTransitionTo(StateProcessingTurn))
	assert.True(t, sm.CanTransitionTo(StateClosed))
	assert.False(t, sm.CanTransitionTo(StateAwaitingApproval))
	assert.False(t, sm.CanTransitionTo(StateCompleted))

	// Move to ProcessingTurn
	sm.Transition(StateProcessingTurn)
	assert.True(t, sm.CanTransitionTo(StateAwaitingApproval))
	assert.True(t, sm.CanTransitionTo(StateCompleted))
	assert.True(t, sm.CanTransitionTo(StateError))
	assert.False(t, sm.CanTransitionTo(StateIdle))
}

func TestStateMachine_IsInState(t *testing.T) {
	sm := NewStateMachine()

	assert.True(t, sm.IsInState(StateIdle))
	assert.False(t, sm.IsInState(StateProcessingTurn))

	sm.Transition(StateProcessingTurn)
	assert.False(t, sm.IsInState(StateIdle))
	assert.True(t, sm.IsInState(StateProcessingTurn))
}

func TestStateMachine_IsTerminal(t *testing.T) {
	sm := NewStateMachine()

	assert.False(t, sm.IsTerminal())

	sm.Transition(StateProcessingTurn)
	assert.False(t, sm.IsTerminal())

	sm.Transition(StateClosed)
	assert.True(t, sm.IsTerminal())
}

func TestStateMachine_CanAcceptTurn(t *testing.T) {
	tests := []struct {
		state     SessionState
		canAccept bool
	}{
		{StateIdle, true},
		{StateCompleted, true},
		{StateError, true},
		{StateInterrupted, true},
		{StateProcessingTurn, false},
		{StateAwaitingApproval, false},
		{StateClosed, false},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			sm := &StateMachine{currentState: tt.state}
			assert.Equal(t, tt.canAccept, sm.CanAcceptTurn())
		})
	}
}

func TestStateMachine_ThreadSafety(t *testing.T) {
	sm := NewStateMachine()
	const goroutines = 100
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Concurrent reads and writes
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				// Read operations
				_ = sm.GetState()
				_ = sm.GetPreviousState()
				_ = sm.CanTransitionTo(StateProcessingTurn)
				_ = sm.IsInState(StateIdle)

				// Write operations (some will fail, that's ok)
				if id%2 == 0 {
					sm.Transition(StateProcessingTurn)
					sm.Transition(StateCompleted)
					sm.Transition(StateIdle)
				} else {
					sm.Transition(StateProcessingTurn)
					sm.TransitionToError("test")
					sm.Transition(StateIdle)
				}
			}
		}(i)
	}

	wg.Wait()

	// Just ensure we didn't panic and state is valid
	state := sm.GetState()
	assert.True(t, state >= StateIdle && state <= StateClosed)
}

func TestStateMachine_CompleteWorkflow(t *testing.T) {
	t.Run("successful turn workflow", func(t *testing.T) {
		sm := NewStateMachine()

		// Start: Idle
		assert.True(t, sm.CanAcceptTurn())

		// User submits turn
		require.NoError(t, sm.Transition(StateProcessingTurn))
		assert.False(t, sm.CanAcceptTurn())

		// Agent completes turn
		require.NoError(t, sm.Transition(StateCompleted))
		assert.True(t, sm.CanAcceptTurn())

		// Return to idle for next turn
		require.NoError(t, sm.Transition(StateIdle))
		assert.True(t, sm.CanAcceptTurn())
	})

	t.Run("turn with approval workflow", func(t *testing.T) {
		sm := NewStateMachine()

		// Start: Idle
		require.NoError(t, sm.Transition(StateProcessingTurn))

		// Agent needs approval
		require.NoError(t, sm.Transition(StateAwaitingApproval))
		assert.False(t, sm.CanAcceptTurn())

		// User approves, continue processing
		require.NoError(t, sm.Transition(StateProcessingTurn))

		// Complete turn
		require.NoError(t, sm.Transition(StateCompleted))
		assert.True(t, sm.CanAcceptTurn())
	})

	t.Run("interrupted turn workflow", func(t *testing.T) {
		sm := NewStateMachine()

		// Start turn
		require.NoError(t, sm.Transition(StateProcessingTurn))

		// User interrupts
		require.NoError(t, sm.Transition(StateInterrupted))
		assert.True(t, sm.CanAcceptTurn())

		// Return to idle
		require.NoError(t, sm.Transition(StateIdle))
	})

	t.Run("error recovery workflow", func(t *testing.T) {
		sm := NewStateMachine()

		// Start turn
		require.NoError(t, sm.Transition(StateProcessingTurn))

		// Error occurs
		require.NoError(t, sm.TransitionToError("test error"))
		assert.Equal(t, "test error", sm.GetErrorMessage())
		assert.True(t, sm.CanAcceptTurn())

		// Recover and try again
		require.NoError(t, sm.Transition(StateIdle))
		assert.Empty(t, sm.GetErrorMessage())
	})

	t.Run("session closure", func(t *testing.T) {
		sm := NewStateMachine()

		// Can close from idle
		require.NoError(t, sm.Transition(StateClosed))
		assert.True(t, sm.IsTerminal())
		assert.False(t, sm.CanAcceptTurn())

		// Cannot transition from closed
		err := sm.Transition(StateIdle)
		require.Error(t, err)
	})
}
