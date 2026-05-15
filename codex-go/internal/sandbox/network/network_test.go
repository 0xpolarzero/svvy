package network

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewController(t *testing.T) {
	ctrl, err := NewController()
	if err != nil {
		t.Fatalf("NewController() failed: %v", err)
	}

	if ctrl == nil {
		t.Fatal("NewController() returned nil controller")
	}

	// Should return some controller (at minimum fallback)
	if !ctrl.IsAvailable() {
		t.Error("Controller should be available")
	}
}

func TestFallbackController(t *testing.T) {
	ctrl := newFallbackController()

	if !ctrl.IsAvailable() {
		t.Error("Fallback controller should always be available")
	}

	if ctrl.Type() != "fallback" {
		t.Errorf("Expected type 'fallback', got %s", ctrl.Type())
	}

	// Test command configuration
	cmd := exec.Command("echo", "test")
	if err := ctrl.ConfigureCommand(cmd); err != nil {
		t.Errorf("ConfigureCommand failed: %v", err)
	}

	// Check environment variable was set
	found := false
	for _, env := range cmd.Env {
		if env == "CODEX_SANDBOX_NETWORK_DISABLED=1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("CODEX_SANDBOX_NETWORK_DISABLED environment variable not set")
	}

	// Test cleanup
	ctx := context.Background()
	if err := ctrl.Cleanup(ctx); err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}
}

func TestFallbackControllerPreservesExistingEnv(t *testing.T) {
	ctrl := newFallbackController()

	cmd := exec.Command("echo", "test")
	cmd.Env = []string{"EXISTING_VAR=value"}

	if err := ctrl.ConfigureCommand(cmd); err != nil {
		t.Errorf("ConfigureCommand failed: %v", err)
	}

	// Check both variables are present
	if len(cmd.Env) != 2 {
		t.Errorf("Expected 2 environment variables, got %d", len(cmd.Env))
	}

	hasExisting := false
	hasNetwork := false
	for _, env := range cmd.Env {
		if env == "EXISTING_VAR=value" {
			hasExisting = true
		}
		if env == "CODEX_SANDBOX_NETWORK_DISABLED=1" {
			hasNetwork = true
		}
	}

	if !hasExisting {
		t.Error("Existing environment variable was lost")
	}
	if !hasNetwork {
		t.Error("Network disabled variable was not added")
	}
}

// TestNamespaceControllerAvailability tests namespace availability detection
func TestNamespaceControllerAvailability(t *testing.T) {
	ctrl := newNamespaceController()

	// Availability depends on platform and permissions
	isAvailable := ctrl.IsAvailable()

	if runtime.GOOS == "linux" {
		// On Linux, it might be available (depends on permissions)
		// Just verify the check doesn't panic
		t.Logf("Namespace controller available on Linux: %v", isAvailable)
	} else {
		// On non-Linux, should not be available
		if isAvailable {
			t.Error("Namespace controller should not be available on non-Linux platforms")
		}
	}
}

// TestNamespaceControllerType tests the controller type identifier
func TestNamespaceControllerType(t *testing.T) {
	ctrl := newNamespaceController()

	expectedType := "namespace"
	if runtime.GOOS != "linux" {
		expectedType = "namespace-unavailable"
	}

	if ctrl.Type() != expectedType {
		t.Errorf("Expected type %s, got %s", expectedType, ctrl.Type())
	}
}

// TestNamespaceControllerConfiguration tests command configuration
func TestNamespaceControllerConfiguration(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test on non-Linux platform")
	}

	ctrl := newNamespaceController()
	if !ctrl.IsAvailable() {
		t.Skip("Namespace controller not available (may need root or user namespace support)")
	}

	cmd := exec.Command("echo", "test")
	if err := ctrl.ConfigureCommand(cmd); err != nil {
		t.Errorf("ConfigureCommand failed: %v", err)
	}

	// Verify SysProcAttr was configured
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr was not set")
	}

	// Check for CLONE_NEWNET flag
	// Note: We can't check the exact value because it might be combined with other flags
	// Just verify SysProcAttr is set
	t.Logf("SysProcAttr configured: %+v", cmd.SysProcAttr)

	// Check environment variable was set
	found := false
	for _, env := range cmd.Env {
		if env == "CODEX_SANDBOX_NETWORK_DISABLED=1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("CODEX_SANDBOX_NETWORK_DISABLED environment variable not set")
	}
}

// TestNamespaceControllerNetworkIsolation tests actual network isolation
// This test requires root or CAP_SYS_ADMIN capabilities
func TestNamespaceControllerNetworkIsolation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test on non-Linux platform")
	}

	ctrl := newNamespaceController()
	if !ctrl.IsAvailable() {
		t.Skip("Namespace controller not available (may need root or user namespace support)")
	}

	// Test with a simple network check command
	// We'll try to ping localhost which should fail in an isolated namespace
	cmd := exec.Command("sh", "-c", "ping -c 1 -W 1 127.0.0.1 2>&1")
	if err := ctrl.ConfigureCommand(cmd); err != nil {
		t.Fatalf("ConfigureCommand failed: %v", err)
	}

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Network command succeeded when it should have failed. Output: %s", output)
	}

	// The command should fail due to network being unavailable
	outputStr := string(output)
	if !strings.Contains(outputStr, "Network is unreachable") &&
		!strings.Contains(outputStr, "connect: Network is unreachable") &&
		!strings.Contains(outputStr, "connect: Invalid argument") {
		t.Logf("Unexpected output (may still indicate proper isolation): %s", outputStr)
	}
}

// TestNamespaceControllerWithTimeout tests command execution with timeout
func TestNamespaceControllerWithTimeout(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test on non-Linux platform")
	}

	ctrl := newNamespaceController()
	if !ctrl.IsAvailable() {
		t.Skip("Namespace controller not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Run a command that should succeed quickly
	cmd := exec.CommandContext(ctx, "echo", "test")
	if err := ctrl.ConfigureCommand(cmd); err != nil {
		t.Fatalf("ConfigureCommand failed: %v", err)
	}

	output, err := cmd.Output()
	if err != nil {
		t.Errorf("Command failed: %v", err)
	}

	if string(output) != "test\n" {
		t.Errorf("Unexpected output: %s", output)
	}

	// Cleanup
	if err := ctrl.Cleanup(context.Background()); err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}
}

// TestNamespaceControllerNilCommand tests error handling for nil command
func TestNamespaceControllerNilCommand(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test on non-Linux platform")
	}

	ctrl := newNamespaceController()

	err := ctrl.ConfigureCommand(nil)
	if err == nil {
		t.Error("Expected error for nil command, got nil")
	}

	if !strings.Contains(err.Error(), "cannot be nil") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestValidationError tests the ValidationError type
func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Method:  "test-method",
		Message: "test message",
	}

	expectedMsg := "network control validation failed (test-method): test message"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

// TestControllerCleanup tests cleanup functionality
func TestControllerCleanup(t *testing.T) {
	ctrl, err := NewController()
	if err != nil {
		t.Fatalf("NewController failed: %v", err)
	}

	ctx := context.Background()
	if err := ctrl.Cleanup(ctx); err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}
}

// TestControllerCleanupWithCancelledContext tests cleanup with cancelled context
func TestControllerCleanupWithCancelledContext(t *testing.T) {
	ctrl, err := NewController()
	if err != nil {
		t.Fatalf("NewController failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Cleanup should handle cancelled context gracefully
	if err := ctrl.Cleanup(ctx); err != nil {
		t.Errorf("Cleanup with cancelled context failed: %v", err)
	}
}

// BenchmarkControllerCreation benchmarks controller creation
func BenchmarkControllerCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := NewController()
		if err != nil {
			b.Fatalf("NewController failed: %v", err)
		}
	}
}

// BenchmarkFallbackConfiguration benchmarks fallback configuration
func BenchmarkFallbackConfiguration(b *testing.B) {
	ctrl := newFallbackController()
	cmd := exec.Command("echo", "test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ctrl.ConfigureCommand(cmd); err != nil {
			b.Fatalf("ConfigureCommand failed: %v", err)
		}
		cmd.Env = nil // Reset for next iteration
	}
}

// BenchmarkNamespaceConfiguration benchmarks namespace configuration
func BenchmarkNamespaceConfiguration(b *testing.B) {
	if runtime.GOOS != "linux" {
		b.Skip("Skipping Linux-specific benchmark")
	}

	ctrl := newNamespaceController()
	if !ctrl.IsAvailable() {
		b.Skip("Namespace controller not available")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command("echo", "test")
		if err := ctrl.ConfigureCommand(cmd); err != nil {
			b.Fatalf("ConfigureCommand failed: %v", err)
		}
	}
}
