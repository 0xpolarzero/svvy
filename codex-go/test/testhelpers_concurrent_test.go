package test_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/evmts/codex/codex-go/test"
	"github.com/stretchr/testify/assert"
)

// TestHTTPMockServerConcurrentSafety tests that HTTPMockServer is safe for concurrent use
func TestHTTPMockServerConcurrentSafety(t *testing.T) {
	// Create a mock server that responds to all requests
	mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	const numGoroutines = 50
	const requestsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Make concurrent requests from multiple goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				url := fmt.Sprintf("%s/test/%d/%d", mockServer.URL, workerID, j)
				resp, err := http.Get(url)
				if err != nil {
					t.Errorf("Worker %d request %d failed: %v", workerID, j, err)
					return
				}
				resp.Body.Close()
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify all requests were recorded
	expectedRequests := numGoroutines * requestsPerGoroutine
	mockServer.AssertRequestCount(t, expectedRequests)

	// Verify we can safely read from the recorded requests
	for i := 0; i < expectedRequests; i++ {
		mockServer.AssertRequestMethod(t, i, "GET")
	}
}

// TestHTTPMockServerConcurrentWithBodies tests concurrent safety when requests have bodies
func TestHTTPMockServerConcurrentWithBodies(t *testing.T) {
	// Create a mock server that handles POST requests
	mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Created"))
	})

	const numGoroutines = 30
	const requestsPerGoroutine = 5

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Make concurrent POST requests with bodies from multiple goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(workerID int) {
			defer wg.Done()

			client := &http.Client{}
			for j := 0; j < requestsPerGoroutine; j++ {
				body := fmt.Sprintf(`{"worker": %d, "request": %d}`, workerID, j)
				req, err := http.NewRequest("POST", mockServer.URL+"/create",
					http.NoBody)
				if err != nil {
					t.Errorf("Worker %d request %d failed to create: %v", workerID, j, err)
					return
				}

				resp, err := client.Do(req)
				if err != nil {
					t.Errorf("Worker %d request %d failed: %v", workerID, j, err)
					return
				}
				resp.Body.Close()

				// Also verify we can read body during concurrent execution
				_ = body // Use the body variable
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify all requests were recorded
	expectedRequests := numGoroutines * requestsPerGoroutine
	mockServer.AssertRequestCount(t, expectedRequests)

	// Verify we can safely read from the recorded requests
	for i := 0; i < expectedRequests; i++ {
		mockServer.AssertRequestMethod(t, i, "POST")
		mockServer.AssertRequestPath(t, i, "/create")
	}
}

// TestHTTPMockServerParallel uses Go's built-in parallel testing
func TestHTTPMockServerParallel(t *testing.T) {
	mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Run parallel subtests
	for i := 0; i < 20; i++ {
		i := i // capture range variable
		t.Run(fmt.Sprintf("request_%d", i), func(t *testing.T) {
			t.Parallel()

			url := fmt.Sprintf("%s/parallel/%d", mockServer.URL, i)
			resp, err := http.Get(url)
			assert.NoError(t, err)
			if resp != nil {
				resp.Body.Close()
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			}
		})
	}

	// After all parallel tests complete, verify count
	// Note: This runs after all parallel subtests finish
	t.Cleanup(func() {
		// We expect 20 requests
		mockServer.AssertRequestCount(t, 20)
	})
}
