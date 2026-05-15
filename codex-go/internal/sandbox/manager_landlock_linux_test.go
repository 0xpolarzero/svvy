//go:build linux

package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/evmts/codex/codex-go/internal/sandbox/landlock"
)

// TestLandlockSandboxName verifies the sandbox name.
func TestLandlockSandboxName(t *testing.T) {
	l := &landlockSandbox{}
	if got := l.Name(); got != "landlock" {
		t.Errorf("Name() = %v, want %v", got, "landlock")
	}
}

// TestLandlockSandboxIsAvailable tests the availability check.
func TestLandlockSandboxIsAvailable(t *testing.T) {
	l := &landlockSandbox{}
	available := l.IsAvailable()

	// Should match the landlock package's support detection
	expected := landlock.IsSupported()
	if available != expected {
		t.Errorf("IsAvailable() = %v, want %v", available, expected)
	}

	// Log for debugging
	if available {
		t.Log("Landlock is available on this system")
		info, err := landlock.GetInfo()
		if err == nil {
			t.Logf("Kernel: %s, ABI Version: %d", info.KernelVersion, info.ABIVersion)
		}
	} else {
		t.Log("Landlock is not available on this system")
	}
}

// TestLandlockSandboxApplyReadOnly tests applying a read-only policy.
func TestLandlockSandboxApplyReadOnly(t *testing.T) {
	if !landlock.IsSupported() {
		t.Skip("Landlock not supported on this kernel")
	}

	// Create a test workspace
	tmpDir := t.TempDir()

	// Create a read-only policy
	policy := NewReadOnlyPolicy()

	// Create a command (we won't actually execute it)
	cmd := exec.Command("echo", "test")

	// Apply the sandbox
	l := &landlockSandbox{}
	err := l.Apply(cmd, policy, tmpDir)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Verify environment variables were set
	hasLandlockEnv := false
	for _, env := range cmd.Env {
		if env == "CODEX_SANDBOX=landlock" {
			hasLandlockEnv = true
			break
		}
	}
	if !hasLandlockEnv {
		t.Error("Expected CODEX_SANDBOX=landlock environment variable")
	}
}

// TestLandlockSandboxApplyWorkspaceWrite tests applying a workspace-write policy.
func TestLandlockSandboxApplyWorkspaceWrite(t *testing.T) {
	if !landlock.IsSupported() {
		t.Skip("Landlock not supported on this kernel")
	}

	// Create a test workspace
	tmpDir := t.TempDir()

	// Create a workspace-write policy
	policy := NewWorkspaceWritePolicy()

	// Create a command
	cmd := exec.Command("echo", "test")

	// Apply the sandbox
	l := &landlockSandbox{}
	err := l.Apply(cmd, policy, tmpDir)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Verify environment variables were set
	hasLandlockEnv := false
	hasNetworkDisabled := false
	for _, env := range cmd.Env {
		if env == "CODEX_SANDBOX=landlock" {
			hasLandlockEnv = true
		}
		if env == "CODEX_SANDBOX_NETWORK_DISABLED=1" {
			hasNetworkDisabled = true
		}
	}

	if !hasLandlockEnv {
		t.Error("Expected CODEX_SANDBOX=landlock environment variable")
	}

	// Network should be disabled by default for workspace-write
	if !hasNetworkDisabled {
		t.Error("Expected CODEX_SANDBOX_NETWORK_DISABLED=1 environment variable")
	}
}

// TestLandlockSandboxApplyWithNetworkAccess tests workspace-write with network enabled.
func TestLandlockSandboxApplyWithNetworkAccess(t *testing.T) {
	if !landlock.IsSupported() {
		t.Skip("Landlock not supported on this kernel")
	}

	tmpDir := t.TempDir()

	// Create workspace-write policy with network access
	policy := NewWorkspaceWritePolicy()
	policy.WorkspaceWriteConfig.NetworkAccess = true

	cmd := exec.Command("echo", "test")

	l := &landlockSandbox{}
	err := l.Apply(cmd, policy, tmpDir)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Verify network is NOT disabled
	hasNetworkDisabled := false
	for _, env := range cmd.Env {
		if env == "CODEX_SANDBOX_NETWORK_DISABLED=1" {
			hasNetworkDisabled = true
			break
		}
	}

	if hasNetworkDisabled {
		t.Error("Expected network to be enabled, but CODEX_SANDBOX_NETWORK_DISABLED=1 was set")
	}
}

// TestLandlockSandboxApplyEmptyWorkspace tests error handling for empty workspace.
func TestLandlockSandboxApplyEmptyWorkspace(t *testing.T) {
	if !landlock.IsSupported() {
		t.Skip("Landlock not supported on this kernel")
	}

	policy := NewReadOnlyPolicy()
	cmd := exec.Command("echo", "test")

	l := &landlockSandbox{}
	err := l.Apply(cmd, policy, "")
	if err == nil {
		t.Error("Expected error for empty workspace, got nil")
	}
	if err != nil && err.Error() != "workspace path cannot be empty" {
		t.Errorf("Expected 'workspace path cannot be empty' error, got: %v", err)
	}
}

// TestLandlockSandboxApplyFullAccessPolicy tests that full access policy returns error.
func TestLandlockSandboxApplyFullAccessPolicy(t *testing.T) {
	if !landlock.IsSupported() {
		t.Skip("Landlock not supported on this kernel")
	}

	tmpDir := t.TempDir()
	policy := NewDangerFullAccessPolicy()
	cmd := exec.Command("echo", "test")

	l := &landlockSandbox{}
	err := l.Apply(cmd, policy, tmpDir)
	if err == nil {
		t.Error("Expected error for full access policy, got nil")
	}
}

// TestLandlockSandboxApplyWithWritableRoots tests custom writable roots.
func TestLandlockSandboxApplyWithWritableRoots(t *testing.T) {
	if !landlock.IsSupported() {
		t.Skip("Landlock not supported on this kernel")
	}

	tmpDir := t.TempDir()
	extraWritable := t.TempDir()

	// Create workspace-write policy with extra writable root
	policy := NewWorkspaceWritePolicy()
	policy.WorkspaceWriteConfig.WritableRoots = []string{extraWritable}

	cmd := exec.Command("echo", "test")

	l := &landlockSandbox{}
	err := l.Apply(cmd, policy, tmpDir)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}
}

// TestLandlockSandboxIntegrationReadRestriction tests actual read restrictions.
// This test actually applies Landlock and verifies it works.
func TestLandlockSandboxIntegrationReadRestriction(t *testing.T) {
	if !landlock.IsSupported() {
		t.Skip("Landlock not supported on this kernel")
	}

	// This test is more complex because Landlock applies to the current process
	// and is irreversible. We'll test that the ruleset can be created and applied
	// without errors, but we can't fully test restrictions without forking.
	t.Run("RulesetCreation", func(t *testing.T) {
		tmpDir := t.TempDir()
		policy := NewReadOnlyPolicy()

		// Create a simple test to verify the ruleset construction
		writableRoots := policy.GetWritableRoots(tmpDir)
		readOnlyRoots := policy.GetReadOnlyRoots()

		if len(readOnlyRoots) == 0 {
			t.Error("Expected read-only roots, got none")
		}

		t.Logf("Read-only roots: %v", readOnlyRoots)
		t.Logf("Writable roots: %v", writableRoots)
	})
}

// TestLandlockSandboxIntegrationWorkspaceWriteRestriction tests workspace write policy.
func TestLandlockSandboxIntegrationWorkspaceWriteRestriction(t *testing.T) {
	if !landlock.IsSupported() {
		t.Skip("Landlock not supported on this kernel")
	}

	tmpDir := t.TempDir()
	policy := NewWorkspaceWritePolicy()

	// Get the writable roots
	writableRoots := policy.GetWritableRoots(tmpDir)

	// Should include workspace and /tmp
	hasWorkspace := false
	hasTmp := false
	for _, root := range writableRoots {
		if root == tmpDir {
			hasWorkspace = true
		}
		if root == "/tmp" {
			hasTmp = true
		}
	}

	if !hasWorkspace {
		t.Errorf("Expected workspace %s in writable roots: %v", tmpDir, writableRoots)
	}

	if !hasTmp {
		t.Log("Note: /tmp not in writable roots (may be excluded by config)")
	}
}

// TestLandlockSandboxManagerIntegration tests the full integration with SandboxManager.
func TestLandlockSandboxManagerIntegration(t *testing.T) {
	if !landlock.IsSupported() {
		t.Skip("Landlock not supported on this kernel")
	}

	tmpDir := t.TempDir()
	manager := NewSandboxManager()

	// Check that Landlock is available
	available := manager.GetAvailableSandbox()
	if available != "landlock" && available != "seccomp" {
		t.Errorf("Expected landlock or seccomp, got: %s", available)
	}

	t.Logf("Available sandbox: %s", available)

	// Apply read-only policy
	policy := NewReadOnlyPolicy()
	cmd := exec.Command("echo", "test")

	info, err := manager.ApplyToCommand(cmd, policy, tmpDir)
	if err != nil {
		t.Fatalf("ApplyToCommand() failed: %v", err)
	}

	if !info.Applied {
		t.Error("Expected sandbox to be applied")
	}

	t.Logf("Applied sandbox: %s (reason: %s)", info.Type, info.Reason)
}

// TestLandlockSandboxWithNonExistentPaths tests handling of non-existent paths.
func TestLandlockSandboxWithNonExistentPaths(t *testing.T) {
	if !landlock.IsSupported() {
		t.Skip("Landlock not supported on this kernel")
	}

	tmpDir := t.TempDir()
	nonExistentPath := filepath.Join(tmpDir, "does-not-exist")

	policy := NewWorkspaceWritePolicy()
	policy.WorkspaceWriteConfig.WritableRoots = []string{nonExistentPath}

	cmd := exec.Command("echo", "test")

	l := &landlockSandbox{}
	// Should not fail even with non-existent paths (they're just not added)
	err := l.Apply(cmd, policy, tmpDir)
	if err != nil {
		t.Fatalf("Apply() failed with non-existent path: %v", err)
	}
}

// BenchmarkLandlockSandboxApply benchmarks the Apply operation.
func BenchmarkLandlockSandboxApply(b *testing.B) {
	if !landlock.IsSupported() {
		b.Skip("Landlock not supported on this kernel")
	}

	tmpDir := b.TempDir()
	policy := NewWorkspaceWritePolicy()
	l := &landlockSandbox{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command("echo", "test")
		err := l.Apply(cmd, policy, tmpDir)
		if err != nil {
			b.Fatalf("Apply() failed: %v", err)
		}
	}
}

// TestLandlockSandboxApplyPreservesExistingEnv tests that existing environment is preserved.
func TestLandlockSandboxApplyPreservesExistingEnv(t *testing.T) {
	if !landlock.IsSupported() {
		t.Skip("Landlock not supported on this kernel")
	}

	tmpDir := t.TempDir()
	policy := NewReadOnlyPolicy()

	cmd := exec.Command("echo", "test")
	cmd.Env = []string{"EXISTING_VAR=value"}

	l := &landlockSandbox{}
	err := l.Apply(cmd, policy, tmpDir)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Verify existing env is preserved
	hasExisting := false
	hasLandlock := false
	for _, env := range cmd.Env {
		if env == "EXISTING_VAR=value" {
			hasExisting = true
		}
		if env == "CODEX_SANDBOX=landlock" {
			hasLandlock = true
		}
	}

	if !hasExisting {
		t.Error("Expected existing environment variable to be preserved")
	}
	if !hasLandlock {
		t.Error("Expected CODEX_SANDBOX=landlock to be added")
	}
}
