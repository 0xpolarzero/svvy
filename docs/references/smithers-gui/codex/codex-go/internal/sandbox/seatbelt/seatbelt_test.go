//go:build darwin

package seatbelt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evmts/codex/codex-go/internal/protocol"
)

func TestGenerateProfile_ReadOnly(t *testing.T) {
	config := &ProfileConfig{
		AllowFileRead:  true,
		AllowFileWrite: false,
		WritableRoots:  []WritableRoot{},
	}

	profile := GenerateProfile(config)

	// Verify base policy is included
	if !strings.Contains(profile, "(version 1)") {
		t.Error("Profile should contain version declaration")
	}

	if !strings.Contains(profile, "(deny default)") {
		t.Error("Profile should contain deny default")
	}

	// Verify file read is allowed
	if !strings.Contains(profile, "(allow file-read*)") {
		t.Error("Profile should allow file-read")
	}

	// Verify file write is not allowed
	if strings.Contains(profile, "(allow file-write*") {
		t.Error("Profile should not allow file-write for read-only")
	}

	// Verify network is not allowed
	if strings.Contains(profile, "(allow network-outbound)") {
		t.Error("Profile should not allow network for read-only")
	}
}

func TestGenerateProfile_WorkspaceWrite(t *testing.T) {
	config := &ProfileConfig{
		AllowFileRead:  true,
		AllowFileWrite: true,
		WritableRoots: []WritableRoot{
			{Root: "/tmp/workspace", ReadOnlySubpaths: []string{}},
		},
		AllowNetworkOutbound: false,
	}

	profile := GenerateProfile(config)
	t.Logf("Generated profile:\n%s", profile)

	// Verify file read is allowed
	if !strings.Contains(profile, "(allow file-read*)") {
		t.Error("Profile should allow file-read")
	}

	// Verify file write is allowed with writable root
	if !strings.Contains(profile, "(allow file-write*") {
		t.Error("Profile should allow file-write")
	}

	if !strings.Contains(profile, `(subpath "/tmp/workspace")`) {
		t.Error("Profile should include writable root")
	}
}

func TestGenerateProfile_WorkspaceWriteWithReadOnlySubpath(t *testing.T) {
	config := &ProfileConfig{
		AllowFileRead:  true,
		AllowFileWrite: true,
		WritableRoots: []WritableRoot{
			{
				Root:             "/tmp/workspace",
				ReadOnlySubpaths: []string{"/tmp/workspace/.git"},
			},
		},
	}

	profile := GenerateProfile(config)

	// Verify file write is allowed
	if !strings.Contains(profile, "(allow file-write*") {
		t.Error("Profile should allow file-write")
	}

	// Verify the .git directory is protected
	if !strings.Contains(profile, "(require-all") {
		t.Error("Profile should use require-all for read-only subpaths")
	}

	if !strings.Contains(profile, `(subpath "/tmp/workspace")`) {
		t.Error("Profile should include writable root")
	}

	if !strings.Contains(profile, `(require-not (subpath "/tmp/workspace/.git"))`) {
		t.Error("Profile should protect .git directory")
	}
}

func TestGenerateProfile_DangerFullAccess(t *testing.T) {
	config := &ProfileConfig{
		AllowFileRead:        true,
		AllowFileWrite:       true,
		WritableRoots:        []WritableRoot{{Root: "/"}},
		AllowNetworkOutbound: true,
		AllowNetworkInbound:  true,
		AllowSystemSocket:    true,
	}

	profile := GenerateProfile(config)

	// Verify all permissions are granted
	if !strings.Contains(profile, "(allow file-read*)") {
		t.Error("Profile should allow file-read")
	}

	if !strings.Contains(profile, "(allow file-write*") {
		t.Error("Profile should allow file-write")
	}

	if !strings.Contains(profile, `(subpath "/")`) {
		t.Error("Profile should allow write to root")
	}

	if !strings.Contains(profile, "(allow network-outbound)") {
		t.Error("Profile should allow network-outbound")
	}

	if !strings.Contains(profile, "(allow network-inbound)") {
		t.Error("Profile should allow network-inbound")
	}

	if !strings.Contains(profile, "(allow system-socket)") {
		t.Error("Profile should allow system-socket")
	}
}

func TestReadOnlyProfile(t *testing.T) {
	profile := ReadOnlyProfile()

	if !strings.Contains(profile, "(allow file-read*)") {
		t.Error("ReadOnlyProfile should allow file-read")
	}

	if strings.Contains(profile, "(allow file-write*") {
		t.Error("ReadOnlyProfile should not allow file-write")
	}

	if strings.Contains(profile, "(allow network-outbound)") {
		t.Error("ReadOnlyProfile should not allow network")
	}
}

func TestWorkspaceWriteProfile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	workspace := filepath.Join(tmpDir, "workspace")
	if err := os.Mkdir(workspace, 0755); err != nil {
		t.Fatalf("Failed to create test workspace: %v", err)
	}

	profile := WorkspaceWriteProfile(workspace, false, true, true)

	if !strings.Contains(profile, "(allow file-read*)") {
		t.Error("WorkspaceWriteProfile should allow file-read")
	}

	if !strings.Contains(profile, "(allow file-write*") {
		t.Error("WorkspaceWriteProfile should allow file-write")
	}

	if !strings.Contains(profile, workspace) {
		t.Error("WorkspaceWriteProfile should include workspace path")
	}

	if strings.Contains(profile, "(allow network-outbound)") {
		t.Error("WorkspaceWriteProfile with networkAccess=false should not allow network")
	}
}

func TestWorkspaceWriteProfile_WithGitDir(t *testing.T) {
	// Create a temporary directory with .git
	tmpDir := t.TempDir()
	workspace := filepath.Join(tmpDir, "workspace")
	if err := os.Mkdir(workspace, 0755); err != nil {
		t.Fatalf("Failed to create test workspace: %v", err)
	}

	gitDir := filepath.Join(workspace, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	profile := WorkspaceWriteProfile(workspace, false, true, true)

	// Verify .git is protected
	if !strings.Contains(profile, ".git") {
		t.Error("WorkspaceWriteProfile should protect .git directory")
	}

	if !strings.Contains(profile, "(require-not") {
		t.Error("WorkspaceWriteProfile should use require-not for .git protection")
	}
}

func TestWorkspaceWriteProfile_WithNetwork(t *testing.T) {
	tmpDir := t.TempDir()
	profile := WorkspaceWriteProfile(tmpDir, true, true, true)

	if !strings.Contains(profile, "(allow network-outbound)") {
		t.Error("WorkspaceWriteProfile with networkAccess=true should allow network-outbound")
	}

	if !strings.Contains(profile, "(allow network-inbound)") {
		t.Error("WorkspaceWriteProfile with networkAccess=true should allow network-inbound")
	}

	if !strings.Contains(profile, "(allow system-socket)") {
		t.Error("WorkspaceWriteProfile with networkAccess=true should allow system-socket")
	}
}

func TestDangerFullAccessProfile(t *testing.T) {
	profile := DangerFullAccessProfile()

	if !strings.Contains(profile, "(allow file-read*)") {
		t.Error("DangerFullAccessProfile should allow file-read")
	}

	if !strings.Contains(profile, "(allow file-write*") {
		t.Error("DangerFullAccessProfile should allow file-write")
	}

	if !strings.Contains(profile, `(subpath "/")`) {
		t.Error("DangerFullAccessProfile should allow write to root")
	}

	if !strings.Contains(profile, "(allow network-outbound)") {
		t.Error("DangerFullAccessProfile should allow network")
	}
}

func TestGenerateProfileFromSandboxPolicy_ReadOnly(t *testing.T) {
	policy := &protocol.SandboxPolicy{
		Mode: "read-only",
	}

	profile := GenerateProfileFromSandboxPolicy(policy, "/tmp")

	if !strings.Contains(profile, "(allow file-read*)") {
		t.Error("read-only policy should allow file-read")
	}

	if strings.Contains(profile, "(allow file-write*") {
		t.Error("read-only policy should not allow file-write")
	}
}

func TestGenerateProfileFromSandboxPolicy_WorkspaceWrite(t *testing.T) {
	policy := &protocol.SandboxPolicy{
		Mode:          "workspace-write",
		WritableRoots: []string{"/tmp/workspace"},
		NetworkAccess: true,
	}

	profile := GenerateProfileFromSandboxPolicy(policy, "/tmp/cwd")

	if !strings.Contains(profile, "(allow file-read*)") {
		t.Error("workspace-write policy should allow file-read")
	}

	if !strings.Contains(profile, "(allow file-write*") {
		t.Error("workspace-write policy should allow file-write")
	}

	if !strings.Contains(profile, "(allow network-outbound)") {
		t.Error("workspace-write policy with network access should allow network")
	}
}

func TestGenerateProfileFromSandboxPolicy_DangerFullAccess(t *testing.T) {
	policy := &protocol.SandboxPolicy{
		Mode: "danger-full-access",
	}

	profile := GenerateProfileFromSandboxPolicy(policy, "/tmp")

	if !strings.Contains(profile, "(allow file-read*)") {
		t.Error("danger-full-access policy should allow file-read")
	}

	if !strings.Contains(profile, "(allow file-write*") {
		t.Error("danger-full-access policy should allow file-write")
	}

	if !strings.Contains(profile, `(subpath "/")`) {
		t.Error("danger-full-access policy should allow write to root")
	}

	if !strings.Contains(profile, "(allow network-outbound)") {
		t.Error("danger-full-access policy should allow network")
	}
}

func TestProfileConfigFromSandboxPolicy(t *testing.T) {
	tests := []struct {
		name                 string
		policy               *protocol.SandboxPolicy
		cwd                  string
		expectFileRead       bool
		expectFileWrite      bool
		expectNetwork        bool
		expectWritableRoots  bool
	}{
		{
			name:                "read-only",
			policy:              &protocol.SandboxPolicy{Mode: "read-only"},
			cwd:                 "/tmp",
			expectFileRead:      true,
			expectFileWrite:     false,
			expectNetwork:       false,
			expectWritableRoots: false,
		},
		{
			name: "workspace-write",
			policy: &protocol.SandboxPolicy{
				Mode:          "workspace-write",
				WritableRoots: []string{"/tmp/workspace"},
				NetworkAccess: false,
			},
			cwd:                 "/tmp/cwd",
			expectFileRead:      true,
			expectFileWrite:     true,
			expectNetwork:       false,
			expectWritableRoots: true,
		},
		{
			name: "workspace-write with network",
			policy: &protocol.SandboxPolicy{
				Mode:          "workspace-write",
				WritableRoots: []string{},
				NetworkAccess: true,
			},
			cwd:                 "/tmp/cwd",
			expectFileRead:      true,
			expectFileWrite:     true,
			expectNetwork:       true,
			expectWritableRoots: true,
		},
		{
			name:                "danger-full-access",
			policy:              &protocol.SandboxPolicy{Mode: "danger-full-access"},
			cwd:                 "/tmp",
			expectFileRead:      true,
			expectFileWrite:     true,
			expectNetwork:       true,
			expectWritableRoots: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ProfileConfigFromSandboxPolicy(tt.policy, tt.cwd)

			if config.AllowFileRead != tt.expectFileRead {
				t.Errorf("AllowFileRead: got %v, want %v", config.AllowFileRead, tt.expectFileRead)
			}

			if config.AllowFileWrite != tt.expectFileWrite {
				t.Errorf("AllowFileWrite: got %v, want %v", config.AllowFileWrite, tt.expectFileWrite)
			}

			if config.AllowNetworkOutbound != tt.expectNetwork {
				t.Errorf("AllowNetworkOutbound: got %v, want %v", config.AllowNetworkOutbound, tt.expectNetwork)
			}

			if tt.expectWritableRoots {
				if len(config.WritableRoots) == 0 {
					t.Error("Expected writable roots but got none")
				}
			}
		})
	}
}

func TestIsSupported(t *testing.T) {
	// On darwin, IsSupported should return true
	if !IsSupported() {
		t.Error("IsSupported should return true on darwin")
	}
}

func TestBasePolicyTemplate(t *testing.T) {
	// Verify base policy contains essential elements
	if !strings.Contains(BasePolicyTemplate, "(version 1)") {
		t.Error("BasePolicyTemplate should contain version declaration")
	}

	if !strings.Contains(BasePolicyTemplate, "(deny default)") {
		t.Error("BasePolicyTemplate should start with deny default")
	}

	if !strings.Contains(BasePolicyTemplate, "(allow process-exec)") {
		t.Error("BasePolicyTemplate should allow process-exec")
	}

	if !strings.Contains(BasePolicyTemplate, "(allow process-fork)") {
		t.Error("BasePolicyTemplate should allow process-fork")
	}

	// Verify sysctls are allowed
	if !strings.Contains(BasePolicyTemplate, "(allow sysctl-read") {
		t.Error("BasePolicyTemplate should allow sysctl-read")
	}

	// Verify IPC is allowed for Python multiprocessing
	if !strings.Contains(BasePolicyTemplate, "(allow ipc-posix-sem)") {
		t.Error("BasePolicyTemplate should allow ipc-posix-sem for Python multiprocessing")
	}
}

// TestWorkspaceWriteProfileMultiRoot tests multiple writable roots
func TestWorkspaceWriteProfileMultiRoot(t *testing.T) {
	// Create test directories
	tmpDir := t.TempDir()
	root1 := filepath.Join(tmpDir, "root1")
	root2 := filepath.Join(tmpDir, "root2")

	if err := os.Mkdir(root1, 0755); err != nil {
		t.Fatalf("Failed to create root1: %v", err)
	}
	if err := os.Mkdir(root2, 0755); err != nil {
		t.Fatalf("Failed to create root2: %v", err)
	}

	profile := WorkspaceWriteProfileMultiRoot([]string{root1, root2}, false, true, true)

	if !strings.Contains(profile, root1) {
		t.Error("Profile should include root1")
	}

	if !strings.Contains(profile, root2) {
		t.Error("Profile should include root2")
	}

	if !strings.Contains(profile, "(allow file-write*") {
		t.Error("Profile should allow file-write")
	}
}

// Mock tests for ApplyProfile (cannot actually apply sandbox in tests)
func TestApplyProfile_Validation(t *testing.T) {
	// We can't actually test ApplyProfile without applying a sandbox to the test process,
	// which would break subsequent tests. Instead, we test profile generation.

	// Test that profiles are syntactically valid (start with version, contain deny default)
	profiles := []struct {
		name    string
		profile string
	}{
		{"ReadOnly", ReadOnlyProfile()},
		{"DangerFullAccess", DangerFullAccessProfile()},
		{"WorkspaceWrite", WorkspaceWriteProfile("/tmp", false, true, true)},
	}

	for _, tt := range profiles {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.HasPrefix(strings.TrimSpace(tt.profile), "(version 1)") {
				t.Errorf("%s profile should start with version declaration", tt.name)
			}

			if !strings.Contains(tt.profile, "(deny default)") {
				t.Errorf("%s profile should contain deny default", tt.name)
			}
		})
	}
}
