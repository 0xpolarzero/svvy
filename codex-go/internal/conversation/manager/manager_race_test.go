package manager

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/client"
	"github.com/evmts/codex/codex-go/internal/client/mocks"
	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestConcurrentGetSessionAndCloseSession tests the TOCTOU race condition fix
// between GetSession and CloseSession operations.
func TestConcurrentGetSessionAndCloseSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	mgr := createTestManager(t, mockClient)

	// Create a session
	session := createSessionInManager(t, mgr, "race-test-session")
	require.NotNil(t, session)

	const iterations = 100
	var wg sync.WaitGroup
	errors := make(chan error, iterations*2)

	// Start concurrent operations
	for i := 0; i < iterations; i++ {
		// Goroutine 1: Try to acquire session
		wg.Add(1)
		go func() {
			defer wg.Done()
			acquiredSession, err := mgr.AcquireSession("race-test-session")
			if err != nil {
				// Expected if session is closing/closed
				return
			}
			// Hold the session briefly
			time.Sleep(1 * time.Millisecond)
			acquiredSession.Release()
		}()

		// Goroutine 2: Try to close session
		if i == iterations/2 { // Close in the middle
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := mgr.CloseSession("race-test-session")
				if err != nil {
					// May fail if already closed
					return
				}
			}()
		}
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)

	// Verify no panics occurred (test passes if we reach here)
	// The session should either be closed or have all references released
	for err := range errors {
		// All errors should be expected "session closing" or "not found" errors
		assert.Contains(t, err.Error(), "closing", "session", "not found")
	}
}

// TestConcurrentSubmitOpAndCloseSession tests that SubmitOp is safe
// when racing with CloseSession.
func TestConcurrentSubmitOpAndCloseSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	mgr := createTestManager(t, mockClient)
	ctx := context.Background()

	// Setup mock to handle multiple calls
	eventChan := make(chan client.StreamEvent, 1)
	close(eventChan)

	mockClient.EXPECT().
		Stream(gomock.Any(), gomock.Any()).
		Return(eventChan, nil).
		AnyTimes()

	mockClient.EXPECT().
		GetModelContextWindow().
		Return(int64(128000)).
		AnyTimes()

	// Create a session
	session := createSessionInManager(t, mgr, "submit-close-race")
	require.NotNil(t, session)

	var wg sync.WaitGroup
	const concurrency = 20

	// Start concurrent SubmitOp operations
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			op := &protocol.OpUserTurn{
				Items: []protocol.UserInput{
					{Type: "text", Text: strPtr(fmt.Sprintf("message %d", idx))},
				},
				Cwd:            "/test",
				ApprovalPolicy: "auto",
				SandboxPolicy:  protocol.SandboxPolicy{Mode: "off"},
				Model:          "gpt-4",
				Summary:        "off",
			}
			_ = mgr.SubmitOp(ctx, "submit-close-race", op)
		}(i)
	}

	// Close session concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond) // Let some operations start
		_ = mgr.CloseSession("submit-close-race")
	}()

	// Wait for all operations to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out - possible deadlock")
	}
}

// TestReferenceCountingPreventsUseAfterFree verifies that reference counting
// prevents use-after-free when a session is closed while operations are active.
func TestReferenceCountingPreventsUseAfterFree(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	mgr := createTestManager(t, mockClient)

	// Create a session
	session := createSessionInManager(t, mgr, "ref-count-test")
	require.NotNil(t, session)

	// Acquire the session multiple times
	const refCount = 10
	acquiredSessions := make([]*Session, refCount)

	for i := 0; i < refCount; i++ {
		acquired, err := mgr.AcquireSession("ref-count-test")
		require.NoError(t, err)
		acquiredSessions[i] = acquired
	}

	// Start closing the session in a goroutine
	closeDone := make(chan error)
	go func() {
		closeDone <- mgr.CloseSession("ref-count-test")
	}()

	// Close should block until all references are released
	select {
	case <-closeDone:
		t.Fatal("CloseSession completed before all references released")
	case <-time.After(100 * time.Millisecond):
		// Good - close is blocking as expected
	}

	// Release all references
	for i := 0; i < refCount; i++ {
		acquiredSessions[i].Release()
	}

	// Now close should complete
	select {
	case err := <-closeDone:
		require.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("CloseSession did not complete after references released")
	}

	// Verify session cannot be acquired after close
	_, err := mgr.AcquireSession("ref-count-test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestAcquireOnClosingSession verifies that Acquire fails when session is closing.
func TestAcquireOnClosingSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	mgr := createTestManager(t, mockClient)

	// Create a session
	session := createSessionInManager(t, mgr, "closing-test")
	require.NotNil(t, session)

	// Acquire once to keep session busy
	acquired, err := mgr.AcquireSession("closing-test")
	require.NoError(t, err)

	// Start closing (will block)
	closeDone := make(chan struct{})
	go func() {
		_ = mgr.CloseSession("closing-test")
		close(closeDone)
	}()

	// Give close time to start
	time.Sleep(50 * time.Millisecond)

	// Try to acquire while closing - should fail
	_, err = mgr.AcquireSession("closing-test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Release to allow close to complete
	acquired.Release()

	// Wait for close to complete
	select {
	case <-closeDone:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Close did not complete")
	}
}

// TestConcurrentOperationsOnSameSession tests multiple concurrent operations
// on the same session to ensure no data races.
func TestConcurrentOperationsOnSameSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	mgr := createTestManager(t, mockClient)

	// Create a session
	session := createSessionInManager(t, mgr, "concurrent-ops")
	require.NotNil(t, session)

	var wg sync.WaitGroup
	const operations = 50

	// Concurrent GetSession operations
	for i := 0; i < operations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := mgr.GetSession("concurrent-ops")
			if err == nil {
				_ = s.GetTurnContext()
				_ = s.State()
				_ = s.GetTokenUsage()
			}
		}()
	}

	// Concurrent AcquireSession/ReleaseSession operations
	for i := 0; i < operations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := mgr.AcquireSession("concurrent-ops")
			if err == nil {
				time.Sleep(1 * time.Millisecond)
				s.Release()
			}
		}()
	}

	// Concurrent ListSessions operations
	for i := 0; i < operations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mgr.ListSessions()
		}()
	}

	// Wait for all operations
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out")
	}
}

// TestSessionStateTransitionsUnderLoad tests that session state transitions
// work correctly under concurrent load.
func TestSessionStateTransitionsUnderLoad(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	mgr := createTestManager(t, mockClient)

	// Create a session
	session := createSessionInManager(t, mgr, "state-test")
	require.NotNil(t, session)

	var wg sync.WaitGroup
	const readers = 20

	// Concurrent state readers
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				s, err := mgr.GetSession("state-test")
				if err == nil {
					_ = s.State()
					_ = s.CanAcceptTurn()
					_ = s.IsClosed()
				}
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	// Perform state transitions
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)

		// Manually transition to processing (simulating a turn)
		session.mu.Lock()
		_ = session.stateMachine.Transition(StateProcessingTurn)
		session.mu.Unlock()

		time.Sleep(50 * time.Millisecond)

		// Transition to completed
		session.mu.Lock()
		_ = session.stateMachine.Transition(StateCompleted)
		session.mu.Unlock()

		time.Sleep(50 * time.Millisecond)

		// Reset to idle
		_ = session.ResetToIdle()
	}()

	// Wait for all operations
	wg.Wait()

	// Verify final state is valid
	state := session.State()
	assert.True(t, state == StateIdle || state == StateCompleted)
}

// TestMultipleSessionsConcurrent tests that the manager can handle
// multiple sessions with concurrent operations.
func TestMultipleSessionsConcurrent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	mgr := createTestManager(t, mockClient)
	ctx := context.Background()

	const numSessions = 10
	const opsPerSession = 20

	var wg sync.WaitGroup

	// Create and operate on multiple sessions concurrently
	for i := 0; i < numSessions; i++ {
		sessionID := fmt.Sprintf("multi-session-%d", i)

		// Create session
		cfg := SessionConfig{
			ID: sessionID,
			TurnContext: &TurnContext{
				Cwd:   "/test",
				Model: "gpt-4",
			},
		}
		_, err := mgr.CreateSession(ctx, cfg)
		require.NoError(t, err)

		// Perform concurrent operations on this session
		for j := 0; j < opsPerSession; j++ {
			wg.Add(1)
			go func(sid string, opIdx int) {
				defer wg.Done()

				// Mix of operations
				switch opIdx % 4 {
				case 0:
					_, _ = mgr.GetSession(sid)
				case 1:
					s, err := mgr.AcquireSession(sid)
					if err == nil {
						time.Sleep(1 * time.Millisecond)
						s.Release()
					}
				case 2:
					_ = mgr.ListSessions()
				case 3:
					s, err := mgr.GetSession(sid)
					if err == nil {
						_ = s.GetTurnContext()
					}
				}
			}(sessionID, j)
		}

		// Close some sessions concurrently
		if i%3 == 0 {
			wg.Add(1)
			go func(sid string) {
				defer wg.Done()
				time.Sleep(10 * time.Millisecond)
				_ = mgr.CloseSession(sid)
			}(sessionID)
		}
	}

	// Wait for all operations
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out")
	}

	// Verify manager state is consistent
	sessions := mgr.ListSessions()
	assert.LessOrEqual(t, len(sessions), numSessions)
}

// TestNoDeadlockOnRepeatedAcquireRelease ensures that repeated
// acquire/release cycles don't cause deadlocks.
func TestNoDeadlockOnRepeatedAcquireRelease(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	mgr := createTestManager(t, mockClient)

	// Create a session
	session := createSessionInManager(t, mgr, "deadlock-test")
	require.NotNil(t, session)

	// Perform many acquire/release cycles rapidly
	for i := 0; i < 1000; i++ {
		acquired, err := mgr.AcquireSession("deadlock-test")
		require.NoError(t, err)
		acquired.Release()
	}

	// Verify session is still functional
	acquired, err := mgr.AcquireSession("deadlock-test")
	require.NoError(t, err)
	assert.Equal(t, "deadlock-test", acquired.ID())
	acquired.Release()
}
