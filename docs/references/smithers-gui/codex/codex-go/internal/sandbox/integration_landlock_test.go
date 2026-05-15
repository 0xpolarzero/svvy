//go:build linux && integration

package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evmts/codex/codex-go/internal/sandbox/landlock"
)

// TestLandlockIntegrationFilesystemRestrictions tests actual filesystem restrictions.
// This test verifies that Landlock actually blocks filesystem access.
//
// Run with: go test -tags=integration ./internal/sandbox -run TestLandlockIntegration
func TestLandlockIntegrationFilesystemRestrictions(t *testing.T) {
	if !landlock.IsSupported() {
		t.Skip("Landlock not supported on this kernel (requires Linux 5.13+)")
	}

	// Create test directories
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")
	restrictedDir := filepath.Join(tmpDir, "restricted")

	if err := os.Mkdir(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace dir: %v", err)
	}
	if err := os.Mkdir(restrictedDir, 0755); err != nil {
		t.Fatalf("Failed to create restricted dir: %v", err)
	}

	// Create test files
	workspaceFile := filepath.Join(workspaceDir, "allowed.txt")
	restrictedFile := filepath.Join(restrictedDir, "blocked.txt")

	if err := os.WriteFile(workspaceFile, []byte("allowed content"), 0644); err != nil {
		t.Fatalf("Failed to create workspace file: %v", err)
	}
	if err := os.WriteFile(restrictedFile, []byte("restricted content"), 0644); err != nil {
		t.Fatalf("Failed to create restricted file: %v", err)
	}

	t.Run("ReadOnlyPolicy", func(t *testing.T) {
		// Test that read-only policy prevents writes
		policy := NewReadOnlyPolicy()
		manager := NewSandboxManager()

		// Try to write to workspace - should be blocked
		writeCmd := exec.Command("sh", "-c", "echo test > "+workspaceFile)
		info, err := manager.ApplyToCommand(writeCmd, policy, tmpDir)
		if err != nil {
			t.Fatalf("Failed to apply sandbox: %v", err)
		}

		if !info.Applied {
			t.Error("Expected sandbox to be applied")
		}

		// Execute the command - it should fail due to Landlock restrictions
		output, err := writeCmd.CombinedOutput()
		if err == nil {
			t.Error("Expected write to fail with read-only policy, but it succeeded")
		} else {
			t.Logf("Write blocked as expected: %v (output: %s)", err, output)
		}
	})
}

// TestLandlockIntegrationKernelInfo logs kernel and Landlock information.
func TestLandlockIntegrationKernelInfo(t *testing.T) {
	if !landlock.IsSupported() {
		t.Skip("Landlock not supported on this kernel")
	}

	info, err := landlock.GetInfo()
	if err != nil {
		t.Fatalf("Failed to get Landlock info: %v", err)
	}

	t.Logf("Kernel Version: %s", info.KernelVersion)
	t.Logf("Landlock Supported: %v", info.Supported)
	t.Logf("Landlock ABI Version: %d", info.ABIVersion)

	if info.ABIVersion < 1 {
		t.Error("Expected ABI version >= 1")
	}
}
