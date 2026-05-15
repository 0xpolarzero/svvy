package test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var updateGolden = flag.Bool("update", false, "update golden files")

// GoldenJSON compares JSON output with a golden file or updates it if -update flag is set
func GoldenJSON(t *testing.T, name string, got interface{}) {
	t.Helper()

	goldenPath := filepath.Join("testdata", "golden", name+".json")

	// Marshal with pretty printing
	gotBytes, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err, "failed to marshal JSON")

	if *updateGolden {
		// Create directory if it doesn't exist
		dir := filepath.Dir(goldenPath)
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err, "failed to create golden directory")

		// Write golden file
		err = os.WriteFile(goldenPath, gotBytes, 0644)
		require.NoError(t, err, "failed to write golden file")
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	// Read golden file
	wantBytes, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "failed to read golden file %s (run with -update to create)", goldenPath)

	// Compare
	require.JSONEq(t, string(wantBytes), string(gotBytes),
		"JSON mismatch for %s (run with -update to update golden file)", name)
}

// GoldenText compares text output with a golden file or updates it if -update flag is set
func GoldenText(t *testing.T, name string, got string) {
	t.Helper()

	goldenPath := filepath.Join("testdata", "golden", name+".txt")

	if *updateGolden {
		// Create directory if it doesn't exist
		dir := filepath.Dir(goldenPath)
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err, "failed to create golden directory")

		// Write golden file
		err = os.WriteFile(goldenPath, []byte(got), 0644)
		require.NoError(t, err, "failed to write golden file")
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	// Read golden file
	wantBytes, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "failed to read golden file %s (run with -update to create)", goldenPath)

	require.Equal(t, string(wantBytes), got,
		"Text mismatch for %s (run with -update to update golden file)", name)
}

// LoadFixture loads a test fixture file from testdata/fixtures/
func LoadFixture(t *testing.T, name string) []byte {
	t.Helper()

	fixturePath := filepath.Join("testdata", "fixtures", name)
	data, err := os.ReadFile(fixturePath)
	require.NoError(t, err, "failed to read fixture file %s", fixturePath)

	return data
}

// LoadFixtureJSON loads and unmarshals a JSON fixture
func LoadFixtureJSON(t *testing.T, name string, dest interface{}) {
	t.Helper()

	data := LoadFixture(t, name)
	err := json.Unmarshal(data, dest)
	require.NoError(t, err, "failed to unmarshal fixture JSON %s", name)
}

// TempDir creates a temporary directory for testing
func TempDir(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "codex-test-*")
	require.NoError(t, err, "failed to create temp dir")

	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	return dir
}

// WriteFile writes data to a file in a test temporary directory
func WriteFile(t *testing.T, dir, filename string, data []byte) string {
	t.Helper()

	path := filepath.Join(dir, filename)
	err := os.WriteFile(path, data, 0644)
	require.NoError(t, err, "failed to write test file %s", path)

	return path
}

// ============================================================================
// HTTP Mock Server Helpers
// ============================================================================

// HTTPMockServer creates a test HTTP server with custom handlers
type HTTPMockServer struct {
	*httptest.Server
	mu       sync.Mutex
	Requests []*http.Request // Record all requests
	Bodies   [][]byte        // Record all request bodies
}

// NewHTTPMockServer creates a new mock HTTP server with the given handler
func NewHTTPMockServer(t *testing.T, handler http.HandlerFunc) *HTTPMockServer {
	t.Helper()

	mock := &HTTPMockServer{
		Requests: make([]*http.Request, 0),
		Bodies:   make([][]byte, 0),
	}

	// Wrap handler to record requests
	recordingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Record request with mutex protection
		mock.mu.Lock()
		mock.Requests = append(mock.Requests, r)
		mock.mu.Unlock()

		// Record body
		if r.Body != nil {
			body, _ := io.ReadAll(r.Body)
			mock.mu.Lock()
			mock.Bodies = append(mock.Bodies, body)
			mock.mu.Unlock()
			r.Body = io.NopCloser(bytes.NewReader(body)) // Restore body for handler
		}

		handler(w, r)
	})

	mock.Server = httptest.NewServer(recordingHandler)

	t.Cleanup(func() {
		mock.Close()
	})

	return mock
}

// NewJSONMockServer creates a mock server that responds with JSON
func NewJSONMockServer(t *testing.T, statusCode int, response interface{}) *HTTPMockServer {
	t.Helper()

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}

	return NewHTTPMockServer(t, handler)
}

// AssertRequestCount asserts the number of requests received
func (m *HTTPMockServer) AssertRequestCount(t *testing.T, expected int) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	assert.Len(t, m.Requests, expected, "unexpected number of requests")
}

// AssertRequestMethod asserts the HTTP method of a specific request
func (m *HTTPMockServer) AssertRequestMethod(t *testing.T, index int, method string) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	require.Less(t, index, len(m.Requests), "request index out of bounds")
	assert.Equal(t, method, m.Requests[index].Method)
}

// AssertRequestPath asserts the path of a specific request
func (m *HTTPMockServer) AssertRequestPath(t *testing.T, index int, path string) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	require.Less(t, index, len(m.Requests), "request index out of bounds")
	assert.Equal(t, path, m.Requests[index].URL.Path)
}

// GetRequestBody returns the body of a specific request
func (m *HTTPMockServer) GetRequestBody(t *testing.T, index int) []byte {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	require.Less(t, index, len(m.Bodies), "request body index out of bounds")
	return m.Bodies[index]
}

// ============================================================================
// Process Execution Mocks
// ============================================================================

// MockCommand represents a mock command execution
type MockCommand struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// CommandMocker provides a way to mock command executions
type CommandMocker struct {
	Commands map[string]*MockCommand
}

// NewCommandMocker creates a new command mocker
func NewCommandMocker() *CommandMocker {
	return &CommandMocker{
		Commands: make(map[string]*MockCommand),
	}
}

// Mock registers a mock response for a command
func (m *CommandMocker) Mock(cmdName string, stdout, stderr string, exitCode int) {
	m.Commands[cmdName] = &MockCommand{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}
}

// MockSuccess registers a successful command execution
func (m *CommandMocker) MockSuccess(cmdName, stdout string) {
	m.Mock(cmdName, stdout, "", 0)
}

// MockError registers a failed command execution
func (m *CommandMocker) MockError(cmdName, stderr string, exitCode int) {
	m.Mock(cmdName, "", stderr, exitCode)
}

// Get retrieves a mock command by name
func (m *CommandMocker) Get(cmdName string) (*MockCommand, bool) {
	cmd, ok := m.Commands[cmdName]
	return cmd, ok
}

// CaptureExecCommand captures exec.Command calls for testing
// Usage in tests:
//
//	defer CaptureExecCommand(t, mocker)()
//	result := yourFunctionThatCallsExecCommand()
func CaptureExecCommand(t *testing.T, mocker *CommandMocker) func() {
	t.Helper()
	// Note: This is a placeholder. In real implementation, you'd use
	// dependency injection to pass a CommandRunner interface instead
	// of mocking exec.Command directly
	return func() {}
}

// ============================================================================
// Filesystem Mocks
// ============================================================================

// NewMemFS creates an in-memory filesystem for testing
func NewMemFS(t *testing.T) afero.Fs {
	t.Helper()
	return afero.NewMemMapFs()
}

// NewOsFS creates a real OS filesystem with a temp directory
func NewOsFS(t *testing.T) (afero.Fs, string) {
	t.Helper()

	tmpDir := TempDir(t)
	fs := afero.NewBasePathFs(afero.NewOsFs(), tmpDir)

	return fs, tmpDir
}

// WriteFileFS writes a file to an afero filesystem
func WriteFileFS(t *testing.T, fs afero.Fs, path string, data []byte) {
	t.Helper()

	// Create directory if needed
	dir := filepath.Dir(path)
	if dir != "." && dir != "/" {
		err := fs.MkdirAll(dir, 0755)
		require.NoError(t, err, "failed to create directory %s", dir)
	}

	err := afero.WriteFile(fs, path, data, 0644)
	require.NoError(t, err, "failed to write file %s", path)
}

// ReadFileFS reads a file from an afero filesystem
func ReadFileFS(t *testing.T, fs afero.Fs, path string) []byte {
	t.Helper()

	data, err := afero.ReadFile(fs, path)
	require.NoError(t, err, "failed to read file %s", path)

	return data
}

// FileExistsFS checks if a file exists in an afero filesystem
func FileExistsFS(t *testing.T, fs afero.Fs, path string) bool {
	t.Helper()

	exists, err := afero.Exists(fs, path)
	require.NoError(t, err, "failed to check if file exists %s", path)

	return exists
}

// AssertFileExistsFS asserts that a file exists in the filesystem
func AssertFileExistsFS(t *testing.T, fs afero.Fs, path string) {
	t.Helper()
	assert.True(t, FileExistsFS(t, fs, path), "file should exist: %s", path)
}

// AssertFileNotExistsFS asserts that a file does not exist in the filesystem
func AssertFileNotExistsFS(t *testing.T, fs afero.Fs, path string) {
	t.Helper()
	assert.False(t, FileExistsFS(t, fs, path), "file should not exist: %s", path)
}

// ============================================================================
// Context Timeout Helpers
// ============================================================================

// ContextWithTimeout creates a context with a timeout and automatic cleanup
func ContextWithTimeout(t *testing.T, timeout time.Duration) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	return ctx
}

// ContextWithDeadline creates a context with a deadline and automatic cleanup
func ContextWithDeadline(t *testing.T, deadline time.Time) context.Context {
	t.Helper()

	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	t.Cleanup(cancel)

	return ctx
}

// ContextWithCancel creates a cancellable context with automatic cleanup
func ContextWithCancel(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	return ctx, cancel
}

// ShortContext creates a context with a short 100ms timeout for fast tests
func ShortContext(t *testing.T) context.Context {
	return ContextWithTimeout(t, 100*time.Millisecond)
}

// LongContext creates a context with a 5 second timeout for longer tests
func LongContext(t *testing.T) context.Context {
	return ContextWithTimeout(t, 5*time.Second)
}

// ============================================================================
// Async Operation Assertions
// ============================================================================

// Eventually repeatedly checks a condition until it passes or times out
func Eventually(t *testing.T, condition func() bool, timeout time.Duration, message string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if condition() {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("Eventually timed out after %v: %s", timeout, message)
		}

		<-ticker.C
	}
}

// EventuallyWithContext repeatedly checks a condition with context until it passes or times out
func EventuallyWithContext(t *testing.T, ctx context.Context, condition func() bool, message string) {
	t.Helper()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if condition() {
			return
		}

		select {
		case <-ctx.Done():
			t.Fatalf("EventuallyWithContext cancelled or timed out: %s", message)
		case <-ticker.C:
			// Continue checking
		}
	}
}

// WaitForChannel waits for a channel to receive a value or times out
func WaitForChannel[T any](t *testing.T, ch <-chan T, timeout time.Duration) T {
	t.Helper()

	select {
	case val := <-ch:
		return val
	case <-time.After(timeout):
		var zero T
		t.Fatalf("WaitForChannel timed out after %v", timeout)
		return zero
	}
}

// AssertChannelReceives asserts that a channel receives a value within timeout
func AssertChannelReceives[T any](t *testing.T, ch <-chan T, timeout time.Duration, message string) T {
	t.Helper()
	return WaitForChannel(t, ch, timeout)
}

// AssertChannelEmpty asserts that a channel is empty (non-blocking check)
func AssertChannelEmpty[T any](t *testing.T, ch <-chan T, message string) {
	t.Helper()

	select {
	case val := <-ch:
		t.Fatalf("AssertChannelEmpty failed, received value: %v. %s", val, message)
	default:
		// Channel is empty, as expected
	}
}

// ============================================================================
// Additional Assertion Helpers
// ============================================================================

// AssertNoError is a convenience wrapper for require.NoError with better error messages
func AssertNoError(t *testing.T, err error, context string) {
	t.Helper()
	require.NoError(t, err, "unexpected error in %s: %v", context, err)
}

// AssertError asserts that an error occurred
func AssertError(t *testing.T, err error, context string) {
	t.Helper()
	require.Error(t, err, "expected error in %s but got nil", context)
}

// AssertErrorContains asserts that an error contains a specific message
func AssertErrorContains(t *testing.T, err error, substring, context string) {
	t.Helper()
	require.Error(t, err, "expected error in %s but got nil", context)
	assert.Contains(t, err.Error(), substring, "error message should contain substring")
}

// MustMarshalJSON marshals a value to JSON or fails the test
func MustMarshalJSON(t *testing.T, v interface{}) []byte {
	t.Helper()

	data, err := json.Marshal(v)
	require.NoError(t, err, "failed to marshal JSON")

	return data
}

// MustUnmarshalJSON unmarshals JSON or fails the test
func MustUnmarshalJSON(t *testing.T, data []byte, dest interface{}) {
	t.Helper()

	err := json.Unmarshal(data, dest)
	require.NoError(t, err, "failed to unmarshal JSON")
}

// ============================================================================
// Parallel Test Helpers
// ============================================================================

// RunParallel marks a test to run in parallel and returns a cleanup function
func RunParallel(t *testing.T) {
	t.Helper()
	t.Parallel()
}

// SkipInShort skips a test when running with -short flag
func SkipInShort(t *testing.T, reason string) {
	t.Helper()
	if testing.Short() {
		t.Skipf("Skipping in short mode: %s", reason)
	}
}

// SkipInCI skips a test when running in CI environment
func SkipInCI(t *testing.T, reason string) {
	t.Helper()
	if os.Getenv("CI") != "" {
		t.Skipf("Skipping in CI: %s", reason)
	}
}
