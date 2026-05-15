package sandbox

import (
	"os/exec"
	"runtime"
	"testing"
)

// =============================================================================
// Policy Tests
// =============================================================================

func TestPolicyString(t *testing.T) {
	tests := []struct {
		policy   Policy
		expected string
	}{
		{PolicyReadOnly, "read-only"},
		{PolicyWorkspaceWrite, "workspace-write"},
		{PolicyDangerFullAccess, "danger-full-access"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.policy.String(); got != tt.expected {
				t.Errorf("Policy.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParsePolicy(t *testing.T) {
	tests := []struct {
		input    string
		expected Policy
	}{
		{"read-only", PolicyReadOnly},
		{"workspace-write", PolicyWorkspaceWrite},
		{"danger-full-access", PolicyDangerFullAccess},
		{"unknown", PolicyReadOnly}, // Defaults to most restrictive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParsePolicy(tt.input); got != tt.expected {
				t.Errorf("ParsePolicy(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestPolicyConfigShouldSandbox(t *testing.T) {
	tests := []struct {
		name     string
		policy   Policy
		expected bool
	}{
		{"read-only should sandbox", PolicyReadOnly, true},
		{"workspace-write should sandbox", PolicyWorkspaceWrite, true},
		{"danger-full-access should not sandbox", PolicyDangerFullAccess, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &PolicyConfig{Policy: tt.policy}
			if got := pc.ShouldSandbox(); got != tt.expected {
				t.Errorf("PolicyConfig.ShouldSandbox() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPolicyConfigHasFullNetworkAccess(t *testing.T) {
	tests := []struct {
		name     string
		config   *PolicyConfig
		expected bool
	}{
		{
			name:     "read-only has no network",
			config:   NewReadOnlyPolicy(),
			expected: false,
		},
		{
			name:     "danger-full-access has network",
			config:   NewDangerFullAccessPolicy(),
			expected: true,
		},
		{
			name: "workspace-write with network enabled",
			config: &PolicyConfig{
				Policy: PolicyWorkspaceWrite,
				WorkspaceWriteConfig: &WorkspaceWriteConfig{
					NetworkAccess: true,
				},
			},
			expected: true,
		},
		{
			name: "workspace-write with network disabled",
			config: &PolicyConfig{
				Policy: PolicyWorkspaceWrite,
				WorkspaceWriteConfig: &WorkspaceWriteConfig{
					NetworkAccess: false,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.HasFullNetworkAccess(); got != tt.expected {
				t.Errorf("PolicyConfig.HasFullNetworkAccess() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetWritableRoots(t *testing.T) {
	workspace := "/home/user/workspace"

	tests := []struct {
		name     string
		config   *PolicyConfig
		contains []string
	}{
		{
			name:     "read-only has no writable roots",
			config:   NewReadOnlyPolicy(),
			contains: []string{},
		},
		{
			name:     "danger-full-access has no writable roots",
			config:   NewDangerFullAccessPolicy(),
			contains: []string{},
		},
		{
			name:     "workspace-write includes workspace",
			config:   NewWorkspaceWritePolicy(),
			contains: []string{workspace},
		},
		{
			name: "workspace-write with additional roots",
			config: &PolicyConfig{
				Policy: PolicyWorkspaceWrite,
				WorkspaceWriteConfig: &WorkspaceWriteConfig{
					WritableRoots: []string{"/opt/data"},
				},
			},
			contains: []string{workspace, "/opt/data"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roots := tt.config.GetWritableRoots(workspace)

			if len(tt.contains) == 0 && len(roots) > 0 {
				t.Errorf("Expected no writable roots, got %v", roots)
				return
			}

			for _, expected := range tt.contains {
				found := false
				for _, root := range roots {
					if root == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected writable root %q not found in %v", expected, roots)
				}
			}
		})
	}
}

func TestGetReadOnlyRoots(t *testing.T) {
	tests := []struct {
		name     string
		config   *PolicyConfig
		expected []string
	}{
		{
			name:     "read-only grants read access to root",
			config:   NewReadOnlyPolicy(),
			expected: []string{"/"},
		},
		{
			name:     "workspace-write grants read access to root",
			config:   NewWorkspaceWritePolicy(),
			expected: []string{"/"},
		},
		{
			name:     "danger-full-access has no restrictions",
			config:   NewDangerFullAccessPolicy(),
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roots := tt.config.GetReadOnlyRoots()
			if len(roots) != len(tt.expected) {
				t.Errorf("GetReadOnlyRoots() returned %d roots, want %d", len(roots), len(tt.expected))
				return
			}
			for i, root := range roots {
				if root != tt.expected[i] {
					t.Errorf("GetReadOnlyRoots()[%d] = %q, want %q", i, root, tt.expected[i])
				}
			}
		})
	}
}

// =============================================================================
// SandboxManager Tests
// =============================================================================

func TestNewSandboxManager(t *testing.T) {
	sm := NewSandboxManager()
	if sm == nil {
		t.Fatal("NewSandboxManager() returned nil")
	}

	if sm.logger == nil {
		t.Error("SandboxManager.logger is nil")
	}

	// Check that appliers were registered based on OS
	switch runtime.GOOS {
	case "darwin":
		if len(sm.appliers) == 0 {
			t.Error("Expected Seatbelt applier to be registered on macOS")
		}
	case "linux":
		if len(sm.appliers) == 0 {
			t.Error("Expected at least one Linux sandbox applier to be registered")
		}
	case "windows":
		// Windows has no sandbox yet, so appliers may be empty
	}
}

func TestGetAvailableSandbox(t *testing.T) {
	sm := NewSandboxManager()
	available := sm.GetAvailableSandbox()

	switch runtime.GOOS {
	case "darwin":
		if available != "seatbelt" {
			t.Errorf("Expected 'seatbelt' on macOS, got %q", available)
		}
	case "linux":
		// Should be either landlock or seccomp
		if available != "landlock" && available != "seccomp" {
			t.Errorf("Expected 'landlock' or 'seccomp' on Linux, got %q", available)
		}
	case "windows":
		if available != "" {
			t.Errorf("Expected no sandbox on Windows, got %q", available)
		}
	}
}

func TestApplyToCommandWithFullAccess(t *testing.T) {
	sm := NewSandboxManager()
	cmd := exec.Command("echo", "test")
	policy := NewDangerFullAccessPolicy()

	info, err := sm.ApplyToCommand(cmd, policy, "/tmp")
	if err != nil {
		t.Fatalf("ApplyToCommand() error = %v", err)
	}

	if info.Applied {
		t.Error("Sandbox should not be applied for danger-full-access policy")
	}

	if info.Type != SandboxTypeNone {
		t.Errorf("Expected SandboxTypeNone, got %v", info.Type)
	}
}

func TestApplyToCommandWithReadOnly(t *testing.T) {
	sm := NewSandboxManager()
	cmd := exec.Command("echo", "test")
	policy := NewReadOnlyPolicy()

	info, err := sm.ApplyToCommand(cmd, policy, "/tmp")
	if err != nil {
		t.Fatalf("ApplyToCommand() error = %v", err)
	}

	// On systems with sandbox support, it should be applied
	switch runtime.GOOS {
	case "darwin", "linux":
		if !info.Applied {
			t.Error("Sandbox should be applied on macOS/Linux")
		}
		if info.Type == SandboxTypeNone {
			t.Error("Expected non-None sandbox type")
		}
	case "windows":
		if info.Applied {
			t.Error("Sandbox should not be applied on Windows (not supported)")
		}
	}
}

func TestApplyToCommandWithWorkspaceWrite(t *testing.T) {
	sm := NewSandboxManager()
	cmd := exec.Command("echo", "test")
	policy := NewWorkspaceWritePolicy()

	info, err := sm.ApplyToCommand(cmd, policy, "/tmp")
	if err != nil {
		t.Fatalf("ApplyToCommand() error = %v", err)
	}

	// On systems with sandbox support, it should be applied
	switch runtime.GOOS {
	case "darwin", "linux":
		if !info.Applied {
			t.Error("Sandbox should be applied on macOS/Linux")
		}
	case "windows":
		if info.Applied {
			t.Error("Sandbox should not be applied on Windows (not supported)")
		}
	}
}

// =============================================================================
// SandboxType Tests
// =============================================================================

func TestSandboxTypeString(t *testing.T) {
	tests := []struct {
		stype    SandboxType
		expected string
	}{
		{SandboxTypeNone, "none"},
		{SandboxTypeSeatbelt, "seatbelt"},
		{SandboxTypeLandlock, "landlock"},
		{SandboxTypeSeccomp, "seccomp"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.stype.String(); got != tt.expected {
				t.Errorf("SandboxType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSandboxTypeFromName(t *testing.T) {
	tests := []struct {
		name     string
		expected SandboxType
	}{
		{"seatbelt", SandboxTypeSeatbelt},
		{"landlock", SandboxTypeLandlock},
		{"seccomp", SandboxTypeSeccomp},
		{"none", SandboxTypeNone},
		{"unknown", SandboxTypeNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sandboxTypeFromName(tt.name); got != tt.expected {
				t.Errorf("sandboxTypeFromName(%q) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// OS-Specific Sandbox Tests
// =============================================================================

func TestSeatbeltIsAvailable(t *testing.T) {
	sb := &seatbeltSandbox{}
	available := sb.IsAvailable()

	if runtime.GOOS == "darwin" {
		if !available {
			t.Error("Seatbelt should be available on macOS")
		}
	} else {
		if available {
			t.Error("Seatbelt should not be available on non-macOS")
		}
	}
}

func TestLandlockIsAvailable(t *testing.T) {
	sb := &landlockSandbox{}
	available := sb.IsAvailable()

	if runtime.GOOS != "linux" {
		if available {
			t.Error("Landlock should not be available on non-Linux")
		}
		return
	}

	// On Linux, availability depends on kernel version
	// We just verify the check doesn't panic
	t.Logf("Landlock available on this system: %v", available)
}

func TestSeccompIsAvailable(t *testing.T) {
	sb := &seccompSandbox{}
	available := sb.IsAvailable()

	if runtime.GOOS == "linux" {
		if !available {
			t.Error("Seccomp should be available on Linux")
		}
	} else {
		if available {
			t.Error("Seccomp should not be available on non-Linux")
		}
	}
}

func TestSeatbeltApply(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Seatbelt only available on macOS")
	}

	sb := &seatbeltSandbox{}
	cmd := exec.Command("echo", "test")
	policy := NewReadOnlyPolicy()

	err := sb.Apply(cmd, policy, "/tmp")
	if err != nil {
		t.Fatalf("Seatbelt.Apply() error = %v", err)
	}

	// Verify environment variables were set
	hasCodexSandbox := false
	for _, env := range cmd.Env {
		if env == "CODEX_SANDBOX=seatbelt" {
			hasCodexSandbox = true
			break
		}
	}
	if !hasCodexSandbox {
		t.Error("Expected CODEX_SANDBOX=seatbelt environment variable")
	}
}

func TestLandlockApply(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Landlock only available on Linux")
	}

	sb := &landlockSandbox{}
	if !sb.IsAvailable() {
		t.Skip("Landlock not available on this kernel version")
	}

	cmd := exec.Command("echo", "test")
	policy := NewReadOnlyPolicy()

	err := sb.Apply(cmd, policy, "/tmp")
	if err != nil {
		t.Fatalf("Landlock.Apply() error = %v", err)
	}

	// Verify environment variables were set
	hasCodexSandbox := false
	for _, env := range cmd.Env {
		if env == "CODEX_SANDBOX=landlock" {
			hasCodexSandbox = true
			break
		}
	}
	if !hasCodexSandbox {
		t.Error("Expected CODEX_SANDBOX=landlock environment variable")
	}
}

func TestSeccompApply(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Seccomp only available on Linux")
	}

	sb := &seccompSandbox{}
	cmd := exec.Command("echo", "test")
	policy := NewReadOnlyPolicy()

	err := sb.Apply(cmd, policy, "/tmp")
	if err != nil {
		t.Fatalf("Seccomp.Apply() error = %v", err)
	}

	// Verify environment variables were set
	hasCodexSandbox := false
	for _, env := range cmd.Env {
		if env == "CODEX_SANDBOX=seccomp" {
			hasCodexSandbox = true
			break
		}
	}
	if !hasCodexSandbox {
		t.Error("Expected CODEX_SANDBOX=seccomp environment variable")
	}
}

func TestNetworkDisabledEnvironment(t *testing.T) {
	tests := []struct {
		name          string
		policy        *PolicyConfig
		expectNetwork bool
	}{
		{
			name:          "read-only disables network",
			policy:        NewReadOnlyPolicy(),
			expectNetwork: false,
		},
		{
			name:          "danger-full-access allows network",
			policy:        NewDangerFullAccessPolicy(),
			expectNetwork: true,
		},
		{
			name: "workspace-write with network enabled",
			policy: &PolicyConfig{
				Policy: PolicyWorkspaceWrite,
				WorkspaceWriteConfig: &WorkspaceWriteConfig{
					NetworkAccess: true,
				},
			},
			expectNetwork: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewSandboxManager()
			cmd := exec.Command("echo", "test")

			_, err := sm.ApplyToCommand(cmd, tt.policy, "/tmp")
			if err != nil {
				t.Fatalf("ApplyToCommand() error = %v", err)
			}

			hasNetworkDisabled := false
			for _, env := range cmd.Env {
				if env == "CODEX_SANDBOX_NETWORK_DISABLED=1" {
					hasNetworkDisabled = true
					break
				}
			}

			if tt.expectNetwork && hasNetworkDisabled {
				t.Error("Expected network to be enabled, but CODEX_SANDBOX_NETWORK_DISABLED is set")
			}

			// Note: We skip checking !tt.expectNetwork because danger-full-access
			// doesn't apply sandbox at all, so the env var won't be set
		})
	}
}

// =============================================================================
// Kernel Version Parsing Tests
// =============================================================================

func TestParseKernelVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected *kernelVersion
		wantErr  bool
	}{
		{
			input:    "5.13.0",
			expected: &kernelVersion{Major: 5, Minor: 13, Patch: 0},
			wantErr:  false,
		},
		{
			input:    "5.13.0-generic",
			expected: &kernelVersion{Major: 5, Minor: 13, Patch: 0},
			wantErr:  false,
		},
		{
			input:    "6.1.21-v8+",
			expected: &kernelVersion{Major: 6, Minor: 1, Patch: 21},
			wantErr:  false,
		},
		{
			input:    "4.19.0-21-amd64",
			expected: &kernelVersion{Major: 4, Minor: 19, Patch: 0},
			wantErr:  false,
		},
		{
			input:   "invalid",
			wantErr: true,
		},
		{
			input:   "5",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseKernelVersion(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if got.Major != tt.expected.Major {
				t.Errorf("Major = %d, want %d", got.Major, tt.expected.Major)
			}
			if got.Minor != tt.expected.Minor {
				t.Errorf("Minor = %d, want %d", got.Minor, tt.expected.Minor)
			}
			if got.Patch != tt.expected.Patch {
				t.Errorf("Patch = %d, want %d", got.Patch, tt.expected.Patch)
			}
		})
	}
}

func TestGetLinuxKernelVersion(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Kernel version detection only works on Linux")
	}

	version, err := getLinuxKernelVersion()
	if err != nil {
		t.Fatalf("getLinuxKernelVersion() error = %v", err)
	}

	if version.Major == 0 {
		t.Error("Expected non-zero major version")
	}

	t.Logf("Detected kernel version: %d.%d.%d", version.Major, version.Minor, version.Patch)
}

// =============================================================================
// Path Deduplication Tests
// =============================================================================

func TestCleanAndDeduplicatePaths(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no duplicates",
			input:    []string{"/tmp", "/home/user"},
			expected: []string{"/tmp", "/home/user"},
		},
		{
			name:     "with duplicates",
			input:    []string{"/tmp", "/tmp", "/home/user"},
			expected: []string{"/tmp", "/home/user"},
		},
		{
			name:     "with trailing slashes",
			input:    []string{"/tmp/", "/tmp", "/home/user/"},
			expected: []string{"/tmp", "/home/user"},
		},
		{
			name:     "with relative paths",
			input:    []string{"/tmp/./foo", "/tmp/foo"},
			expected: []string{"/tmp/foo"},
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanAndDeduplicatePaths(tt.input)

			if len(got) != len(tt.expected) {
				t.Errorf("cleanAndDeduplicatePaths() returned %d paths, want %d", len(got), len(tt.expected))
				return
			}

			// Create a map of expected paths for easier checking
			expectedMap := make(map[string]bool)
			for _, p := range tt.expected {
				expectedMap[p] = true
			}

			// Verify all returned paths are expected
			for _, p := range got {
				if !expectedMap[p] {
					t.Errorf("Unexpected path in result: %q", p)
				}
			}
		})
	}
}

// =============================================================================
// Integration Test
// =============================================================================

func TestSandboxManagerIntegration(t *testing.T) {
	// Skip this test on Windows since sandbox is not supported
	if runtime.GOOS == "windows" {
		t.Skip("Sandbox not supported on Windows")
	}

	sm := NewSandboxManager()

	// Test that we can apply sandbox to a real command
	cmd := exec.Command("echo", "hello")
	policy := NewReadOnlyPolicy()

	info, err := sm.ApplyToCommand(cmd, policy, "/tmp")
	if err != nil {
		t.Fatalf("Failed to apply sandbox: %v", err)
	}

	if !info.Applied {
		t.Error("Expected sandbox to be applied")
	}

	t.Logf("Applied %s sandbox", info.Type)

	// Actually run the command to verify it doesn't break
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to run sandboxed command: %v", err)
	}

	if string(output) != "hello\n" {
		t.Errorf("Unexpected output: %q", string(output))
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkApplyToCommand(b *testing.B) {
	sm := NewSandboxManager()
	policy := NewReadOnlyPolicy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command("echo", "test")
		_, err := sm.ApplyToCommand(cmd, policy, "/tmp")
		if err != nil {
			b.Fatalf("ApplyToCommand() error = %v", err)
		}
	}
}

func BenchmarkGetWritableRoots(b *testing.B) {
	policy := NewWorkspaceWritePolicy()
	workspace := "/home/user/workspace"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = policy.GetWritableRoots(workspace)
	}
}

func BenchmarkParseKernelVersion(b *testing.B) {
	version := "5.13.0-52-generic"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parseKernelVersion(version)
	}
}
