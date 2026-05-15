//go:build linux

package sandbox

import (
	"os"
	"os/exec"
	"syscall"
	"testing"

	seccompPkg "github.com/evmts/codex/codex-go/internal/sandbox/seccomp"
)

// =============================================================================
// Seccomp Sandbox Tests
// =============================================================================

func TestSeccompSandboxName(t *testing.T) {
	s := &seccompSandbox{}
	if name := s.Name(); name != "seccomp" {
		t.Errorf("Expected name 'seccomp', got %q", name)
	}
}

func TestSeccompSandboxIsAvailable(t *testing.T) {
	s := &seccompSandbox{}
	available := s.IsAvailable()

	// On Linux, seccomp should generally be available unless running on very old kernel
	t.Logf("Seccomp available: %v", available)

	// Also test the underlying IsSupported function
	supported := seccompPkg.IsSupported()
	if available != supported {
		t.Errorf("IsAvailable() = %v, but seccompPkg.IsSupported() = %v", available, supported)
	}
}

func TestSeccompSandboxApplyReadOnlyPolicy(t *testing.T) {
	// Skip if seccomp is not supported
	s := &seccompSandbox{}
	if !s.IsAvailable() {
		t.Skip("Seccomp not available on this system")
	}

	// Create a test command
	cmd := exec.Command("/bin/echo", "test")
	policy := NewReadOnlyPolicy()
	workspace := "/tmp/test-workspace"

	// Apply seccomp sandbox
	err := s.Apply(cmd, policy, workspace)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Verify environment variables are set
	envMap := make(map[string]string)
	for _, env := range cmd.Env {
		// Parse KEY=VALUE format
		for i := 0; i < len(env); i++ {
			if env[i] == '=' {
				envMap[env[:i]] = env[i+1:]
				break
			}
		}
	}

	if envMap["CODEX_SANDBOX"] != "seccomp" {
		t.Errorf("Expected CODEX_SANDBOX=seccomp, got %q", envMap["CODEX_SANDBOX"])
	}

	if envMap["CODEX_SANDBOX_NETWORK_DISABLED"] != "1" {
		t.Errorf("Expected CODEX_SANDBOX_NETWORK_DISABLED=1 for read-only policy")
	}

	if envMap["CODEX_SECCOMP_ENABLED"] != "1" {
		t.Errorf("Expected CODEX_SECCOMP_ENABLED=1")
	}

	// Verify SysProcAttr is configured
	if cmd.SysProcAttr == nil {
		t.Error("Expected SysProcAttr to be set")
	} else {
		if cmd.SysProcAttr.Pdeathsig != syscall.SIGKILL {
			t.Errorf("Expected Pdeathsig=SIGKILL, got %v", cmd.SysProcAttr.Pdeathsig)
		}
		if !cmd.SysProcAttr.Setpgid {
			t.Error("Expected Setpgid=true")
		}
	}
}

func TestSeccompSandboxApplyWorkspaceWriteNoNetwork(t *testing.T) {
	s := &seccompSandbox{}
	if !s.IsAvailable() {
		t.Skip("Seccomp not available on this system")
	}

	cmd := exec.Command("/bin/echo", "test")
	policy := NewWorkspaceWritePolicy()
	workspace := "/tmp/test-workspace"

	err := s.Apply(cmd, policy, workspace)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Verify environment variables
	envMap := make(map[string]string)
	for _, env := range cmd.Env {
		for i := 0; i < len(env); i++ {
			if env[i] == '=' {
				envMap[env[:i]] = env[i+1:]
				break
			}
		}
	}

	if envMap["CODEX_SANDBOX"] != "seccomp" {
		t.Errorf("Expected CODEX_SANDBOX=seccomp, got %q", envMap["CODEX_SANDBOX"])
	}

	// Network should be disabled by default for workspace-write
	if envMap["CODEX_SANDBOX_NETWORK_DISABLED"] != "1" {
		t.Errorf("Expected CODEX_SANDBOX_NETWORK_DISABLED=1 for workspace-write without network")
	}
}

func TestSeccompSandboxApplyWorkspaceWriteWithNetwork(t *testing.T) {
	s := &seccompSandbox{}
	if !s.IsAvailable() {
		t.Skip("Seccomp not available on this system")
	}

	cmd := exec.Command("/bin/echo", "test")
	policy := &PolicyConfig{
		Policy: PolicyWorkspaceWrite,
		WorkspaceWriteConfig: &WorkspaceWriteConfig{
			NetworkAccess: true,
		},
	}
	workspace := "/tmp/test-workspace"

	err := s.Apply(cmd, policy, workspace)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Verify environment variables
	envMap := make(map[string]string)
	for _, env := range cmd.Env {
		for i := 0; i < len(env); i++ {
			if env[i] == '=' {
				envMap[env[:i]] = env[i+1:]
				break
			}
		}
	}

	if envMap["CODEX_SANDBOX"] != "seccomp" {
		t.Errorf("Expected CODEX_SANDBOX=seccomp, got %q", envMap["CODEX_SANDBOX"])
	}

	// Network should NOT be disabled when NetworkAccess is true
	if envMap["CODEX_SANDBOX_NETWORK_DISABLED"] != "" {
		t.Errorf("Expected CODEX_SANDBOX_NETWORK_DISABLED to be unset, got %q", envMap["CODEX_SANDBOX_NETWORK_DISABLED"])
	}

	// CODEX_SECCOMP_ENABLED should NOT be set when no filter is applied
	if envMap["CODEX_SECCOMP_ENABLED"] != "" {
		t.Errorf("Expected CODEX_SECCOMP_ENABLED to be unset when network is allowed")
	}
}

func TestSeccompSandboxApplyDangerFullAccess(t *testing.T) {
	s := &seccompSandbox{}
	if !s.IsAvailable() {
		t.Skip("Seccomp not available on this system")
	}

	cmd := exec.Command("/bin/echo", "test")
	policy := NewDangerFullAccessPolicy()
	workspace := "/tmp/test-workspace"

	err := s.Apply(cmd, policy, workspace)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Verify environment has sandbox type set
	envMap := make(map[string]string)
	for _, env := range cmd.Env {
		for i := 0; i < len(env); i++ {
			if env[i] == '=' {
				envMap[env[:i]] = env[i+1:]
				break
			}
		}
	}

	if envMap["CODEX_SANDBOX"] != "seccomp" {
		t.Errorf("Expected CODEX_SANDBOX=seccomp, got %q", envMap["CODEX_SANDBOX"])
	}

	// No seccomp filter should be applied for full access
	if envMap["CODEX_SECCOMP_ENABLED"] != "" {
		t.Errorf("Expected CODEX_SECCOMP_ENABLED to be unset for full access")
	}

	// SysProcAttr may or may not be set, that's okay for full access
}

// =============================================================================
// Seccomp Filter Creation Tests
// =============================================================================

func TestCreateReadOnlyFilter(t *testing.T) {
	s := &seccompSandbox{}
	if !s.IsAvailable() {
		t.Skip("Seccomp not available on this system")
	}

	arch := seccompPkg.GetArchitecture()
	policy := NewReadOnlyPolicy()

	filter, err := s.createReadOnlyFilter(arch, policy)
	if err != nil {
		t.Fatalf("createReadOnlyFilter() failed: %v", err)
	}

	if filter == nil {
		t.Fatal("Expected non-nil filter")
	}

	// Filter should have BPF instructions
	// We can't inspect the internal program directly, but we verified it exists
	t.Logf("Read-only filter created successfully for arch %s", arch)
}

func TestCreateModerateFilter(t *testing.T) {
	s := &seccompSandbox{}
	if !s.IsAvailable() {
		t.Skip("Seccomp not available on this system")
	}

	arch := seccompPkg.GetArchitecture()
	policy := NewReadOnlyPolicy()

	filter, err := s.createModerateFilter(arch, policy)
	if err != nil {
		t.Fatalf("createModerateFilter() failed: %v", err)
	}

	if filter == nil {
		t.Fatal("Expected non-nil filter")
	}

	t.Logf("Moderate filter created successfully for arch %s", arch)
}

func TestCreateModerateFilterWithNetwork(t *testing.T) {
	s := &seccompSandbox{}
	if !s.IsAvailable() {
		t.Skip("Seccomp not available on this system")
	}

	arch := seccompPkg.GetArchitecture()
	policy := &PolicyConfig{
		Policy: PolicyWorkspaceWrite,
		WorkspaceWriteConfig: &WorkspaceWriteConfig{
			NetworkAccess: true,
		},
	}

	filter, err := s.createModerateFilter(arch, policy)
	if err != nil {
		t.Fatalf("createModerateFilter() with network failed: %v", err)
	}

	if filter == nil {
		t.Fatal("Expected non-nil filter")
	}

	t.Logf("Moderate filter with network created successfully for arch %s", arch)
}

func TestCreateStrictFilter(t *testing.T) {
	s := &seccompSandbox{}
	if !s.IsAvailable() {
		t.Skip("Seccomp not available on this system")
	}

	arch := seccompPkg.GetArchitecture()
	policy := NewReadOnlyPolicy()

	filter, err := s.createStrictFilter(arch, policy)
	if err != nil {
		t.Fatalf("createStrictFilter() failed: %v", err)
	}

	if filter == nil {
		t.Fatal("Expected non-nil filter")
	}

	t.Logf("Strict filter created successfully for arch %s", arch)
}

// =============================================================================
// Integration Tests (run in child process)
// =============================================================================

func TestSeccompSandboxIntegrationNetworkBlocking(t *testing.T) {
	s := &seccompSandbox{}
	if !s.IsAvailable() {
		t.Skip("Seccomp not available on this system")
	}

	// This test would need to run in a child process to actually apply the filter
	// and verify that network syscalls are blocked. For now, we just verify
	// that the filter can be created and configured.

	// Run in child process
	if os.Getenv("TEST_SECCOMP_NETWORK_BLOCKING") == "1" {
		// This code runs in the child process
		arch := seccompPkg.GetArchitecture()
		filter, err := seccompPkg.CreateNetworkFilter(arch)
		if err != nil {
			t.Fatalf("CreateNetworkFilter() failed: %v", err)
		}

		// Apply the filter
		if err := filter.Apply(); err != nil {
			t.Fatalf("filter.Apply() failed: %v", err)
		}

		// Try to create a network socket - should fail with EPERM
		fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
		if err == nil {
			syscall.Close(fd)
			t.Fatal("Expected socket creation to be blocked by seccomp filter")
		}

		if err != syscall.EPERM {
			t.Logf("Warning: Expected EPERM, got %v", err)
		}

		// Try Unix socket - should succeed
		fd, err = syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
		if err != nil {
			t.Fatalf("AF_UNIX socket should be allowed: %v", err)
		}
		syscall.Close(fd)

		t.Log("Network blocking verified successfully")
		os.Exit(0)
	}

	// Run the test in a child process
	cmd := exec.Command(os.Args[0], "-test.run=TestSeccompSandboxIntegrationNetworkBlocking")
	cmd.Env = append(os.Environ(), "TEST_SECCOMP_NETWORK_BLOCKING=1")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Logf("Child output: %s", output)
		t.Fatalf("Child process failed: %v", err)
	}

	t.Log("Network blocking integration test passed")
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkSeccompApplyReadOnly(b *testing.B) {
	s := &seccompSandbox{}
	if !s.IsAvailable() {
		b.Skip("Seccomp not available on this system")
	}

	policy := NewReadOnlyPolicy()
	workspace := "/tmp/test-workspace"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command("/bin/echo", "test")
		err := s.Apply(cmd, policy, workspace)
		if err != nil {
			b.Fatalf("Apply() failed: %v", err)
		}
	}
}

func BenchmarkSeccompFilterCreation(b *testing.B) {
	s := &seccompSandbox{}
	if !s.IsAvailable() {
		b.Skip("Seccomp not available on this system")
	}

	arch := seccompPkg.GetArchitecture()
	policy := NewReadOnlyPolicy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.createReadOnlyFilter(arch, policy)
		if err != nil {
			b.Fatalf("createReadOnlyFilter() failed: %v", err)
		}
	}
}

func BenchmarkModerateFilterCreation(b *testing.B) {
	s := &seccompSandbox{}
	if !s.IsAvailable() {
		b.Skip("Seccomp not available on this system")
	}

	arch := seccompPkg.GetArchitecture()
	policy := NewWorkspaceWritePolicy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.createModerateFilter(arch, policy)
		if err != nil {
			b.Fatalf("createModerateFilter() failed: %v", err)
		}
	}
}

func BenchmarkStrictFilterCreation(b *testing.B) {
	s := &seccompSandbox{}
	if !s.IsAvailable() {
		b.Skip("Seccomp not available on this system")
	}

	arch := seccompPkg.GetArchitecture()
	policy := NewReadOnlyPolicy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.createStrictFilter(arch, policy)
		if err != nil {
			b.Fatalf("createStrictFilter() failed: %v", err)
		}
	}
}
