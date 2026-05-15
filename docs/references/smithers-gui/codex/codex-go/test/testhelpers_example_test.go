package test_test

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHTTPMockServer demonstrates using HTTP mock server
func TestHTTPMockServer(t *testing.T) {
	// Create a mock server that returns JSON
	response := map[string]string{"message": "Hello, World!"}
	mockServer := test.NewJSONMockServer(t, http.StatusOK, response)

	// Use the mock server URL in your code
	resp, err := http.Get(mockServer.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify the response
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "Hello, World!", result["message"])

	// Verify requests were made
	mockServer.AssertRequestCount(t, 1)
}

// TestMemFS demonstrates using filesystem mocks
func TestMemFS(t *testing.T) {
	// Create an in-memory filesystem
	fs := test.NewMemFS(t)

	// Write a file
	test.WriteFileFS(t, fs, "/tmp/test.txt", []byte("Hello, World!"))

	// Verify file exists
	test.AssertFileExistsFS(t, fs, "/tmp/test.txt")

	// Read the file back
	data := test.ReadFileFS(t, fs, "/tmp/test.txt")
	assert.Equal(t, "Hello, World!", string(data))
}

// TestContextWithTimeout demonstrates using context helpers
func TestContextWithTimeout(t *testing.T) {
	// Create a context with automatic cleanup
	ctx := test.ContextWithTimeout(t, 5*time.Second)

	// Use context in your operations
	select {
	case <-ctx.Done():
		t.Fatal("Context cancelled unexpectedly")
	default:
		// Context is still valid
	}
}

// TestEventually demonstrates using Eventually for async assertions
func TestEventually(t *testing.T) {
	counter := 0
	var mu sync.Mutex
	go func() {
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		counter = 42
		mu.Unlock()
	}()

	// Wait for counter to be set
	test.Eventually(t, func() bool {
		mu.Lock()
		ok := counter == 42
		mu.Unlock()
		return ok
	}, 1*time.Second, "counter should be 42")

	mu.Lock()
	v := counter
	mu.Unlock()
	assert.Equal(t, 42, v)
}

// TestWaitForChannel demonstrates using channel helpers
func TestWaitForChannel(t *testing.T) {
	ch := make(chan string, 1)

	// Send value in background
	go func() {
		time.Sleep(10 * time.Millisecond)
		ch <- "result"
	}()

	// Wait for the value with timeout
	result := test.WaitForChannel(t, ch, 1*time.Second)
	assert.Equal(t, "result", result)
}

// TestLoadFixture demonstrates loading fixtures
func TestLoadFixture(t *testing.T) {
	// Load a JSON fixture
	var config map[string]interface{}
	test.LoadFixtureJSON(t, "config_minimal.json", &config)

	assert.Equal(t, "claude-sonnet-4-5-20250929", config["model"])
}

// Example: Complete Test Using Multiple Helpers
func TestCompleteExample(t *testing.T) {
	// Skip in short mode
	test.SkipInShort(t, "requires external resources")

	// Create mock HTTP server
	mockAPI := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	// Create in-memory filesystem
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/config.json", []byte(`{"api_url": "test"}`))

	// Simulate async operation
	resultCh := make(chan string, 1)
	go func() {
		time.Sleep(10 * time.Millisecond)
		resultCh <- "completed"
	}()

	// Wait for result
	result := test.WaitForChannel(t, resultCh, 1*time.Second)
	assert.Equal(t, "completed", result)

	// Verify mock server was not called
	mockAPI.AssertRequestCount(t, 0)

	// Verify filesystem state
	test.AssertFileExistsFS(t, fs, "/config.json")
}

// Example: Testing with Command Mocker
func TestCommandMocker(t *testing.T) {
	mocker := test.NewCommandMocker()

	// Setup mock command responses
	mocker.MockSuccess("git", "commit abc123")
	mocker.MockError("docker", "error: daemon not running", 1)

	// Verify mocks were registered
	gitCmd, ok := mocker.Get("git")
	require.True(t, ok)
	assert.Equal(t, "commit abc123", gitCmd.Stdout)
	assert.Equal(t, 0, gitCmd.ExitCode)

	dockerCmd, ok := mocker.Get("docker")
	require.True(t, ok)
	assert.Equal(t, "error: daemon not running", dockerCmd.Stderr)
	assert.Equal(t, 1, dockerCmd.ExitCode)
}

// Example: Testing with Eventually and Custom Assertions
func TestEventuallyWithComplexCondition(t *testing.T) {
	// Simulate a service that takes time to start
	serviceReady := false
	serviceError := error(nil)
	var mu sync.Mutex

	go func() {
		time.Sleep(100 * time.Millisecond)
		mu.Lock()
		serviceReady = true
		mu.Unlock()
	}()

	// Wait for service to be ready
	test.Eventually(t, func() bool {
		mu.Lock()
		ready := serviceReady
		mu.Unlock()
		return ready && serviceError == nil
	}, 1*time.Second, "service should be ready")

	mu.Lock()
	ready := serviceReady
	mu.Unlock()
	assert.True(t, ready)
}

// Example: Using Parallel Tests
func TestParallelExample(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"hello", "hello", "HELLO"},
		{"world", "world", "WORLD"},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			test.RunParallel(t)

			// Your test logic here
			// result := strings.ToUpper(tt.input)
			// assert.Equal(t, tt.want, result)
		})
	}
}

// Example: Using Filesystem with Real Temp Directory
func TestWithRealFilesystem(t *testing.T) {
	fs, tmpDir := test.NewOsFS(t)

	// Write file to real filesystem (in temp dir)
	test.WriteFileFS(t, fs, "test.txt", []byte("content"))

	// File exists in temp directory
	test.AssertFileExistsFS(t, fs, "test.txt")

	// Temp directory is automatically cleaned up
	t.Logf("Using temp directory: %s", tmpDir)
}
