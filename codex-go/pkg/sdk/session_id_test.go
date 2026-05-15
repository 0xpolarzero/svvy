package sdk

import (
	"sync"
	"testing"

	"github.com/evmts/codex/codex-go/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateSessionID_Uniqueness tests that generated session IDs are unique.
func TestGenerateSessionID_Uniqueness(t *testing.T) {
	const numIDs = 10000
	ids := make(map[string]bool, numIDs)

	for i := 0; i < numIDs; i++ {
		id := generateSessionID()
		assert.NotEmpty(t, id, "generated ID should not be empty")
		assert.False(t, ids[id], "duplicate ID generated: %s", id)
		ids[id] = true
	}

	assert.Len(t, ids, numIDs, "should generate unique IDs")
}

// TestGenerateSessionID_Format tests that generated session IDs follow UUID format.
func TestGenerateSessionID_Format(t *testing.T) {
	id := generateSessionID()

	// UUID v4 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	// Where x is any hexadecimal digit and y is one of 8, 9, a, or b
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, id,
		"generated ID should match UUID v4 format")
}

// TestGenerateSessionID_Concurrent tests session ID generation under high concurrency.
// This test uses the race detector to verify thread safety.
func TestGenerateSessionID_Concurrent(t *testing.T) {
	const numGoroutines = 100
	const idsPerGoroutine = 100

	// Use a channel to collect all generated IDs
	idsChan := make(chan string, numGoroutines*idsPerGoroutine)
	var wg sync.WaitGroup

	// Launch multiple goroutines generating IDs concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				id := generateSessionID()
				idsChan <- id
			}
		}()
	}

	// Wait for all goroutines to finish
	wg.Wait()
	close(idsChan)

	// Collect all IDs and verify uniqueness
	ids := make(map[string]bool)
	for id := range idsChan {
		assert.NotEmpty(t, id, "generated ID should not be empty")
		assert.False(t, ids[id], "duplicate ID generated under concurrency: %s", id)
		ids[id] = true
	}

	expectedCount := numGoroutines * idsPerGoroutine
	assert.Len(t, ids, expectedCount, "should generate unique IDs under concurrency")
}

// TestNewSession_ConcurrentCreation tests concurrent session creation to verify
// no race conditions exist in session ID generation and session map management.
func TestNewSession_ConcurrentCreation(t *testing.T) {
	sdk := mustCreateTestSDK(t)
	defer sdk.Close()

	const numGoroutines = 50
	const sessionsPerGoroutine = 10

	ctx := test.LongContext(t)
	var wg sync.WaitGroup
	sessionIDs := make(chan string, numGoroutines*sessionsPerGoroutine)
	errors := make(chan error, numGoroutines*sessionsPerGoroutine)

	// Create sessions concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < sessionsPerGoroutine; j++ {
				session, err := sdk.NewSession(ctx, SessionOptions{
					SystemPrompt: "Test session",
				})
				if err != nil {
					errors <- err
					continue
				}
				sessionIDs <- session.ID()
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(sessionIDs)
	close(errors)

	// Check for errors
	for err := range errors {
		require.NoError(t, err, "session creation should not fail")
	}

	// Verify all session IDs are unique
	uniqueIDs := make(map[string]bool)
	for id := range sessionIDs {
		assert.NotEmpty(t, id, "session ID should not be empty")
		assert.False(t, uniqueIDs[id], "duplicate session ID: %s", id)
		uniqueIDs[id] = true
	}

	expectedCount := numGoroutines * sessionsPerGoroutine
	assert.Len(t, uniqueIDs, expectedCount, "all session IDs should be unique")

	// Verify all sessions are retrievable
	for id := range uniqueIDs {
		session, err := sdk.GetSession(id)
		assert.NoError(t, err, "should be able to retrieve session %s", id)
		assert.NotNil(t, session, "session should not be nil")
		assert.Equal(t, id, session.ID(), "session ID should match")
	}
}

// TestNewSession_ConcurrentOperations tests concurrent session operations
// (creation, retrieval, listing, closing) to verify thread safety.
func TestNewSession_ConcurrentOperations(t *testing.T) {
	sdk := mustCreateTestSDK(t)
	defer sdk.Close()

	ctx := test.LongContext(t)
	const numOperations = 100
	var wg sync.WaitGroup

	// Track created session IDs
	sessionIDsMux := sync.Mutex{}
	sessionIDs := make([]string, 0)

	// Concurrent session creation
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			session, err := sdk.NewSession(ctx, SessionOptions{
				SystemPrompt: "Test",
			})
			if err == nil {
				sessionIDsMux.Lock()
				sessionIDs = append(sessionIDs, session.ID())
				sessionIDsMux.Unlock()
			}
		}()
	}

	// Concurrent session listing (while sessions are being created)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sessions := sdk.ListSessions()
			// Just verify it doesn't panic or race
			_ = sessions
		}()
	}

	wg.Wait()

	// Now perform concurrent get/close operations
	var wg2 sync.WaitGroup
	sessionIDsMux.Lock()
	idsToTest := make([]string, len(sessionIDs))
	copy(idsToTest, sessionIDs)
	sessionIDsMux.Unlock()

	// Concurrent get operations
	for _, id := range idsToTest {
		wg2.Add(1)
		id := id // capture for goroutine
		go func() {
			defer wg2.Done()
			_, _ = sdk.GetSession(id)
		}()
	}

	// Close half the sessions concurrently
	for i, id := range idsToTest {
		if i%2 == 0 {
			wg2.Add(1)
			id := id // capture for goroutine
			go func() {
				defer wg2.Done()
				_ = sdk.CloseSession(id)
			}()
		}
	}

	wg2.Wait()
	// Test passes if no race conditions or panics occur
}

// TestGenerateSessionID_NoCollisionsAcrossInstances tests that session IDs
// remain unique even when generating from multiple test instances.
func TestGenerateSessionID_NoCollisionsAcrossInstances(t *testing.T) {
	const numTests = 10
	const idsPerTest = 1000

	allIDs := make(map[string]bool)
	var mu sync.Mutex

	var wg sync.WaitGroup
	for i := 0; i < numTests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			localIDs := make([]string, idsPerTest)
			for j := 0; j < idsPerTest; j++ {
				localIDs[j] = generateSessionID()
			}

			mu.Lock()
			defer mu.Unlock()
			for _, id := range localIDs {
				assert.False(t, allIDs[id], "duplicate ID across test instances: %s", id)
				allIDs[id] = true
			}
		}()
	}

	wg.Wait()
	assert.Len(t, allIDs, numTests*idsPerTest, "all IDs should be unique across instances")
}
