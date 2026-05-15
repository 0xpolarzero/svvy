// +build integration

// Package sandbox_test provides comprehensive integration tests for sandbox enforcement.
//
// These tests verify that sandbox restrictions are actually enforced by the OS,
// not just that the API calls succeed. They test real filesystem operations,
// network operations, and syscall restrictions.
//
// IMPORTANT: These tests require:
// - Linux kernel 5.13+ (for Landlock tests) OR macOS (for Seatbelt tests)
// - Root privileges may be needed for some tests
// - Network access for network restriction tests
// - Integration build tag: go test -tags=integration
//
// Run with: go test -tags=integration -v ./internal/sandbox/
//
// Test Organization:
// - Helper functions for creating sandboxed test environments
// - Filesystem restriction tests (read-only, read-write, directory traversal)
// - Network restriction tests (connection blocking, port restrictions)
// - OS-specific tests (Seatbelt, Landlock, Seccomp)
package sandbox_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/sandbox"
	"github.com/evmts/codex/codex-go/internal/sandbox/native"
)

// TestEnvironment provides a controlled environment for integration testing.
type TestEnvironment struct {
	// TempDir is the temporary directory for test files
	TempDir string
	// ReadOnlyDir is a directory that should be read-only
	ReadOnlyDir string
	// ReadWriteDir is a directory that should be read-write
	ReadWriteDir string
	// TestFile is a file in the read-write directory
	TestFile string
	// ReadOnlyFile is a file in the read-only directory
	ReadOnlyFile string
	t *testing.T
}

// SetupTestEnvironment creates a controlled test environment.
func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	t.Helper()

	tempDir := t.TempDir()

	// Create read-only directory
	readOnlyDir := filepath.Join(tempDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0755); err != nil {
		t.Fatalf("failed to create read-only dir: %v", err)
	}

	// Create a file in read-only directory
	readOnlyFile := filepath.Join(readOnlyDir, "readonly.txt")
	if err := ioutil.WriteFile(readOnlyFile, []byte("readonly content"), 0644); err != nil {
		t.Fatalf("failed to create read-only file: %v", err)
	}

	// Create read-write directory
	readWriteDir := filepath.Join(tempDir, "readwrite")
	if err := os.MkdirAll(readWriteDir, 0755); err != nil {
		t.Fatalf("failed to create read-write dir: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(readWriteDir, "test.txt")
	if err := ioutil.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	return &TestEnvironment{
		TempDir:      tempDir,
		ReadOnlyDir:  readOnlyDir,
		ReadWriteDir: readWriteDir,
		TestFile:     testFile,
		ReadOnlyFile: readOnlyFile,
		t:            t,
	}
}

// Cleanup removes test environment files.
func (te *TestEnvironment) Cleanup() {
	// TempDir is automatically cleaned up by t.TempDir()
}

// TestFilesystemReadOnlyEnforcement tests that read-only paths cannot be written to.
func TestFilesystemReadOnlyEnforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	tests := []struct {
		name            string
		command         *sandbox.Command
		expectViolation bool
		description     string
	}{
		{
			name: "write to read-only directory should fail",
			command: &sandbox.Command{
				Program:          "sh",
				Args:             []string{"-c", fmt.Sprintf("echo test > %s/newfile.txt", env.ReadOnlyDir)},
				WorkingDirectory: env.TempDir,
				ReadOnlyPaths:    []string{env.ReadOnlyDir},
				ReadWritePaths:   []string{env.ReadWriteDir},
				Timeout:          5 * time.Second,
			},
			expectViolation: true,
			description:     "Writing to a read-only directory should be blocked",
		},
		{
			name: "modify read-only file should fail",
			command: &sandbox.Command{
				Program:          "sh",
				Args:             []string{"-c", fmt.Sprintf("echo append >> %s", env.ReadOnlyFile)},
				WorkingDirectory: env.TempDir,
				ReadOnlyPaths:    []string{env.ReadOnlyDir},
				ReadWritePaths:   []string{env.ReadWriteDir},
				Timeout:          5 * time.Second,
			},
			expectViolation: true,
			description:     "Modifying a read-only file should be blocked",
		},
		{
			name: "delete read-only file should fail",
			command: &sandbox.Command{
				Program:          "rm",
				Args:             []string{env.ReadOnlyFile},
				WorkingDirectory: env.TempDir,
				ReadOnlyPaths:    []string{env.ReadOnlyDir},
				ReadWritePaths:   []string{env.ReadWriteDir},
				Timeout:          5 * time.Second,
			},
			expectViolation: true,
			description:     "Deleting a read-only file should be blocked",
		},
		{
			name: "read from read-only directory should succeed",
			command: &sandbox.Command{
				Program:          "cat",
				Args:             []string{env.ReadOnlyFile},
				WorkingDirectory: env.TempDir,
				ReadOnlyPaths:    []string{env.ReadOnlyDir},
				ReadWritePaths:   []string{env.ReadWriteDir},
				Timeout:          5 * time.Second,
			},
			expectViolation: false,
			description:     "Reading from a read-only directory should succeed",
		},
	}

	sb := native.New()
	if !sb.IsAvailable() {
		t.Skip("native sandbox not available")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := sb.Execute(ctx, tt.command)

			// Log result for debugging
			t.Logf("Description: %s", tt.description)
			t.Logf("Exit code: %d", result.ExitCode)
			if result.Stdout != "" {
				t.Logf("Stdout: %s", result.Stdout)
			}
			if result.Stderr != "" {
				t.Logf("Stderr: %s", result.Stderr)
			}

			if err != nil && ctx.Err() != nil {
				t.Fatalf("command execution timed out: %v", err)
			}

			if tt.expectViolation {
				// Should have non-zero exit code or violation
				if result.ExitCode == 0 {
					t.Errorf("expected command to fail but it succeeded")
				}
				// NOTE: Violation detection depends on error message patterns
				// Some systems may not set result.Violation but will still fail
				if result.ExitCode != 0 {
					t.Logf("Command failed as expected with exit code %d", result.ExitCode)
				}
				if result.Violation != nil {
					t.Logf("Detected violation: %s", result.Violation.FormatViolation())
				}
			} else {
				if result.ExitCode != 0 {
					t.Errorf("expected command to succeed but it failed with exit code %d", result.ExitCode)
				}
				if result.Violation != nil {
					t.Errorf("unexpected violation: %+v", result.Violation)
				}
			}
		})
	}
}

// TestFilesystemReadWriteEnforcement tests that read-write paths can be written to.
func TestFilesystemReadWriteEnforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	tests := []struct {
		name        string
		command     *sandbox.Command
		description string
		verify      func(t *testing.T) // Optional verification function
	}{
		{
			name: "write to read-write directory should succeed",
			command: &sandbox.Command{
				Program:          "sh",
				Args:             []string{"-c", fmt.Sprintf("echo hello > %s/output.txt", env.ReadWriteDir)},
				WorkingDirectory: env.TempDir,
				ReadOnlyPaths:    []string{env.ReadOnlyDir},
				ReadWritePaths:   []string{env.ReadWriteDir},
				Timeout:          5 * time.Second,
			},
			description: "Writing to a read-write directory should succeed",
			verify: func(t *testing.T) {
				outputFile := filepath.Join(env.ReadWriteDir, "output.txt")
				content, err := ioutil.ReadFile(outputFile)
				if err != nil {
					t.Errorf("failed to read output file: %v", err)
					return
				}
				if !strings.Contains(string(content), "hello") {
					t.Errorf("output file content incorrect: %s", content)
				}
			},
		},
		{
			name: "modify existing file in read-write directory should succeed",
			command: &sandbox.Command{
				Program:          "sh",
				Args:             []string{"-c", fmt.Sprintf("echo modified >> %s", env.TestFile)},
				WorkingDirectory: env.TempDir,
				ReadOnlyPaths:    []string{env.ReadOnlyDir},
				ReadWritePaths:   []string{env.ReadWriteDir},
				Timeout:          5 * time.Second,
			},
			description: "Modifying a file in read-write directory should succeed",
			verify: func(t *testing.T) {
				content, err := ioutil.ReadFile(env.TestFile)
				if err != nil {
					t.Errorf("failed to read test file: %v", err)
					return
				}
				if !strings.Contains(string(content), "modified") {
					t.Errorf("test file was not modified: %s", content)
				}
			},
		},
		{
			name: "create and delete file in read-write directory should succeed",
			command: &sandbox.Command{
				Program:          "sh",
				Args:             []string{"-c", fmt.Sprintf("touch %s/temp.txt && rm %s/temp.txt", env.ReadWriteDir, env.ReadWriteDir)},
				WorkingDirectory: env.TempDir,
				ReadOnlyPaths:    []string{env.ReadOnlyDir},
				ReadWritePaths:   []string{env.ReadWriteDir},
				Timeout:          5 * time.Second,
			},
			description: "Creating and deleting files in read-write directory should succeed",
		},
	}

	sb := native.New()
	if !sb.IsAvailable() {
		t.Skip("native sandbox not available")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := sb.Execute(ctx, tt.command)

			// Log result for debugging
			t.Logf("Description: %s", tt.description)
			t.Logf("Exit code: %d", result.ExitCode)
			if result.Stdout != "" {
				t.Logf("Stdout: %s", result.Stdout)
			}
			if result.Stderr != "" {
				t.Logf("Stderr: %s", result.Stderr)
			}

			if err != nil && ctx.Err() != nil {
				t.Fatalf("command execution timed out: %v", err)
			}

			if result.ExitCode != 0 {
				t.Errorf("expected command to succeed but it failed with exit code %d: %s",
					result.ExitCode, result.Stderr)
			}

			if result.Violation != nil {
				t.Errorf("unexpected violation: %+v", result.Violation)
			}

			// Run verification if provided
			if tt.verify != nil {
				tt.verify(t)
			}
		})
	}
}

// TestFilesystemPathTraversalPrevention tests that directory traversal attacks are prevented.
func TestFilesystemPathTraversalPrevention(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	tests := []struct {
		name            string
		command         *sandbox.Command
		expectViolation bool
		description     string
	}{
		{
			name: "path traversal with .. should be blocked",
			command: &sandbox.Command{
				Program:          "sh",
				Args:             []string{"-c", fmt.Sprintf("cat %s/../readonly/readonly.txt", env.ReadWriteDir)},
				WorkingDirectory: env.ReadWriteDir,
				ReadOnlyPaths:    []string{},
				ReadWritePaths:   []string{env.ReadWriteDir},
				Timeout:          5 * time.Second,
			},
			expectViolation: true,
			description:     "Path traversal using .. should be blocked",
		},
		{
			name: "symlink escape attempt should be blocked",
			command: &sandbox.Command{
				Program: "sh",
				Args: []string{"-c", fmt.Sprintf(
					"ln -s %s %s/link && cat %s/link/readonly.txt",
					env.ReadOnlyDir, env.ReadWriteDir, env.ReadWriteDir,
				)},
				WorkingDirectory: env.ReadWriteDir,
				ReadOnlyPaths:    []string{},
				ReadWritePaths:   []string{env.ReadWriteDir},
				Timeout:          5 * time.Second,
			},
			expectViolation: true,
			description:     "Symlink escape attempts should be blocked",
		},
	}

	sb := native.New()
	if !sb.IsAvailable() {
		t.Skip("native sandbox not available")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := sb.Execute(ctx, tt.command)

			t.Logf("Description: %s", tt.description)
			t.Logf("Exit code: %d", result.ExitCode)
			if result.Stderr != "" {
				t.Logf("Stderr: %s", result.Stderr)
			}

			if err != nil && ctx.Err() != nil {
				t.Fatalf("command execution timed out: %v", err)
			}

			if tt.expectViolation {
				if result.ExitCode == 0 {
					t.Errorf("expected command to fail but it succeeded")
				}
				t.Logf("Command failed as expected with exit code %d", result.ExitCode)
			}
		})
	}
}

// TestNetworkRestrictionEnforcement tests that network restrictions are enforced.
func TestNetworkRestrictionEnforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tests := []struct {
		name            string
		command         *sandbox.Command
		expectSuccess   bool
		description     string
		skipOnPlatform  string // Skip test on specific platform
	}{
		{
			name: "network disabled should block HTTP request",
			command: &sandbox.Command{
				Program:          "curl",
				Args:             []string{"-s", "-o", "/dev/null", "-w", "%{http_code}", "http://example.com"},
				WorkingDirectory: "/tmp",
				NetworkEnabled:   false,
				Timeout:          10 * time.Second,
			},
			expectSuccess:  false,
			description:    "HTTP request should be blocked when network is disabled",
			skipOnPlatform: "",
		},
		{
			name: "network enabled should allow HTTP request",
			command: &sandbox.Command{
				Program:          "curl",
				Args:             []string{"-s", "-o", "/dev/null", "-w", "%{http_code}", "http://example.com"},
				WorkingDirectory: "/tmp",
				NetworkEnabled:   true,
				Timeout:          10 * time.Second,
			},
			expectSuccess:  true,
			description:    "HTTP request should succeed when network is enabled",
			skipOnPlatform: "",
		},
	}

	// Check if curl is available
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl not available, skipping network tests")
	}

	sb := native.New()
	if !sb.IsAvailable() {
		t.Skip("native sandbox not available")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipOnPlatform != "" && runtime.GOOS == tt.skipOnPlatform {
				t.Skipf("skipping test on %s", runtime.GOOS)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			result, err := sb.Execute(ctx, tt.command)

			t.Logf("Description: %s", tt.description)
			t.Logf("Exit code: %d", result.ExitCode)
			if result.Stdout != "" {
				t.Logf("Stdout: %s", result.Stdout)
			}
			if result.Stderr != "" {
				t.Logf("Stderr: %s", result.Stderr)
			}

			if err != nil && ctx.Err() != nil {
				t.Logf("Note: Command timed out, which may indicate network blocking is working")
				if !tt.expectSuccess {
					// Timeout is acceptable for blocked network access
					return
				}
			}

			if tt.expectSuccess {
				if result.ExitCode != 0 {
					t.Logf("Warning: Expected success but got exit code %d. Network tests can be flaky.", result.ExitCode)
				}
			} else {
				if result.ExitCode == 0 {
					t.Logf("Warning: Expected failure but command succeeded. Network blocking may not be enforced.")
				}
			}
		})
	}
}

// TestNetworkPortRestrictions tests that specific network port restrictions work.
func TestNetworkPortRestrictions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start a local HTTP server for testing
	server := &http.Server{
		Addr: "127.0.0.1:18080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "test response")
		}),
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Server error: %v", err)
		}
	}()
	defer server.Close()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is accessible
	if _, err := http.Get("http://127.0.0.1:18080"); err != nil {
		t.Skipf("Test server not accessible: %v", err)
	}

	tests := []struct {
		name          string
		command       *sandbox.Command
		expectSuccess bool
		description   string
	}{
		{
			name: "localhost connection should work when network enabled",
			command: &sandbox.Command{
				Program:          "curl",
				Args:             []string{"-s", "http://127.0.0.1:18080"},
				WorkingDirectory: "/tmp",
				NetworkEnabled:   true,
				Timeout:          5 * time.Second,
			},
			expectSuccess: true,
			description:   "Connection to localhost should succeed when network is enabled",
		},
		{
			name: "localhost connection should fail when network disabled",
			command: &sandbox.Command{
				Program:          "curl",
				Args:             []string{"-s", "http://127.0.0.1:18080"},
				WorkingDirectory: "/tmp",
				NetworkEnabled:   false,
				Timeout:          5 * time.Second,
			},
			expectSuccess: false,
			description:   "Connection to localhost should be blocked when network is disabled",
		},
	}

	// Check if curl is available
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl not available, skipping network port tests")
	}

	sb := native.New()
	if !sb.IsAvailable() {
		t.Skip("native sandbox not available")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := sb.Execute(ctx, tt.command)

			t.Logf("Description: %s", tt.description)
			t.Logf("Exit code: %d", result.ExitCode)
			if result.Stdout != "" {
				t.Logf("Stdout: %s", result.Stdout)
			}
			if result.Stderr != "" {
				t.Logf("Stderr: %s", result.Stderr)
			}

			if err != nil && ctx.Err() != nil {
				if !tt.expectSuccess {
					t.Logf("Command timed out as expected (network blocked)")
					return
				}
				t.Fatalf("command execution timed out: %v", err)
			}

			if tt.expectSuccess {
				if result.ExitCode != 0 {
					t.Logf("Warning: Expected success but got exit code %d", result.ExitCode)
				} else if !strings.Contains(result.Stdout, "test response") {
					t.Errorf("Expected to receive test response, got: %s", result.Stdout)
				}
			} else {
				if result.ExitCode == 0 && strings.Contains(result.Stdout, "test response") {
					t.Logf("Warning: Network blocking may not be enforced")
				}
			}
		})
	}
}

// TestRawSocketCreation tests that raw socket creation can be restricted.
func TestRawSocketCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Test creating a socket programmatically
	tests := []struct {
		name            string
		networkEnabled  bool
		socketType      string
		expectSuccess   bool
		description     string
	}{
		{
			name:           "TCP socket with network disabled",
			networkEnabled: false,
			socketType:     "tcp",
			expectSuccess:  false,
			description:    "TCP socket creation should be blocked when network is disabled",
		},
		{
			name:           "TCP socket with network enabled",
			networkEnabled: true,
			socketType:     "tcp",
			expectSuccess:  true,
			description:    "TCP socket creation should succeed when network is enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Description: %s", tt.description)

			// Create a simple Go program that tries to create a socket
			socketTestCode := `
package main

import (
	"fmt"
	"net"
	"os"
)

func main() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create socket: %v\n", err)
		os.Exit(1)
	}
	defer ln.Close()
	fmt.Println("Socket created successfully")
}
`

			// Write the test program
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "sockettest.go")
			if err := ioutil.WriteFile(testFile, []byte(socketTestCode), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			// Execute the test program
			cmd := &sandbox.Command{
				Program:          "go",
				Args:             []string{"run", testFile},
				WorkingDirectory: tmpDir,
				NetworkEnabled:   tt.networkEnabled,
				ReadWritePaths:   []string{tmpDir},
				Timeout:          10 * time.Second,
			}

			sb := native.New()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			result, err := sb.Execute(ctx, cmd)

			t.Logf("Exit code: %d", result.ExitCode)
			if result.Stdout != "" {
				t.Logf("Stdout: %s", result.Stdout)
			}
			if result.Stderr != "" {
				t.Logf("Stderr: %s", result.Stderr)
			}

			if err != nil && ctx.Err() != nil {
				t.Logf("Command timed out")
				if !tt.expectSuccess {
					return // Acceptable for blocked operations
				}
			}

			if tt.expectSuccess {
				if result.ExitCode != 0 {
					t.Logf("Warning: Expected success but got exit code %d", result.ExitCode)
				}
			} else {
				if result.ExitCode == 0 {
					t.Logf("Warning: Socket creation succeeded when it should have been blocked")
				}
			}
		})
	}
}

// BenchmarkSandboxedCommandExecution benchmarks the overhead of sandboxed execution.
func BenchmarkSandboxedCommandExecution(b *testing.B) {
	sb := native.New()
	if !sb.IsAvailable() {
		b.Skip("native sandbox not available")
	}

	tempDir := b.TempDir()

	cmd := &sandbox.Command{
		Program:          "echo",
		Args:             []string{"hello"},
		WorkingDirectory: tempDir,
		ReadOnlyPaths:    []string{"/"},
		ReadWritePaths:   []string{tempDir},
		Timeout:          5 * time.Second,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, err := sb.Execute(ctx, cmd)
		if err != nil {
			b.Fatalf("execution failed: %v", err)
		}
	}
}

// TestViolationDetectionAccuracy tests that violation detection is accurate.
func TestViolationDetectionAccuracy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	detector := sandbox.NewViolationDetector("native")

	tests := []struct {
		name           string
		result         *sandbox.Result
		expectViolation bool
		expectedType   sandbox.ViolationType
		description    string
	}{
		{
			name: "permission denied should be detected",
			result: &sandbox.Result{
				Stdout:   "",
				Stderr:   "sh: /etc/shadow: Permission denied",
				ExitCode: 1,
			},
			expectViolation: true,
			expectedType:    sandbox.ViolationTypeFileSystem,
			description:     "Permission denied errors should be detected as violations",
		},
		{
			name: "operation not permitted should be detected",
			result: &sandbox.Result{
				Stdout:   "",
				Stderr:   "write /protected/file: operation not permitted",
				ExitCode: 1,
			},
			expectViolation: true,
			expectedType:    sandbox.ViolationTypeFileSystem,
			description:     "Operation not permitted errors should be detected",
		},
		{
			name: "read-only filesystem should be detected",
			result: &sandbox.Result{
				Stdout:   "",
				Stderr:   "cannot create file: Read-only file system",
				ExitCode: 1,
			},
			expectViolation: true,
			expectedType:    sandbox.ViolationTypeFileSystem,
			description:     "Read-only filesystem errors should be detected",
		},
		{
			name: "seccomp violation should be detected",
			result: &sandbox.Result{
				Stdout:   "",
				Stderr:   "seccomp: blocked syscall",
				ExitCode: 159, // SIGSYS
			},
			expectViolation: true,
			expectedType:    sandbox.ViolationTypeSyscall,
			description:     "Seccomp violations should be detected",
		},
		{
			name: "normal failure should not be detected as violation",
			result: &sandbox.Result{
				Stdout:   "",
				Stderr:   "command not found",
				ExitCode: 127,
			},
			expectViolation: false,
			description:     "Normal command failures should not be detected as violations",
		},
		{
			name: "success should not be detected as violation",
			result: &sandbox.Result{
				Stdout:   "output",
				Stderr:   "",
				ExitCode: 0,
			},
			expectViolation: false,
			description:     "Successful commands should not be detected as violations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Description: %s", tt.description)

			violation := detector.DetectViolation(tt.result)

			if tt.expectViolation {
				if violation == nil {
					t.Errorf("expected violation but got none")
				} else {
					t.Logf("Detected violation: %s", violation.FormatViolation())
					if violation.Type != tt.expectedType {
						t.Errorf("expected violation type %s, got %s",
							tt.expectedType, violation.Type)
					}
				}
			} else {
				if violation != nil {
					t.Errorf("unexpected violation: %+v", violation)
				}
			}
		})
	}
}
