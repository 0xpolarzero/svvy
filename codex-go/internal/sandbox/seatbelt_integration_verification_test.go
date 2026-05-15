//go:build darwin

package sandbox

import (
	"os/exec"
	"strings"
	"testing"
)

// TestSeatbeltIntegrationVerification verifies that Seatbelt sandbox
// integration actually wraps commands with sandbox-exec and generates proper profiles.
func TestSeatbeltIntegrationVerification(t *testing.T) {
	// Create a sandbox manager
	sm := NewSandboxManager()

	// Create a test command
	cmd := exec.Command("echo", "test")

	// Apply read-only policy
	policy := NewReadOnlyPolicy()

	// Apply sandbox
	info, err := sm.ApplyToCommand(cmd, policy, "/tmp")
	if err != nil {
		t.Fatalf("Failed to apply sandbox: %v", err)
	}

	if !info.Applied {
		t.Fatal("Sandbox was not applied")
	}

	t.Logf("Sandbox info: Type=%s, Applied=%v, Reason=%s", info.Type, info.Applied, info.Reason)

	// Verify command was wrapped with sandbox-exec
	if cmd.Path != "/usr/bin/sandbox-exec" {
		t.Errorf("Command path is %s, expected /usr/bin/sandbox-exec", cmd.Path)
	}

	// Verify sandbox-exec flags are present
	if len(cmd.Args) < 3 {
		t.Fatalf("Not enough arguments. Got: %v", cmd.Args)
	}

	if cmd.Args[0] != "sandbox-exec" {
		t.Errorf("First arg is %s, expected sandbox-exec", cmd.Args[0])
	}

	if cmd.Args[1] != "-p" {
		t.Errorf("Second arg is %s, expected -p", cmd.Args[1])
	}

	// Verify profile was generated correctly
	profile := cmd.Args[2]

	// Check for essential Seatbelt profile elements
	requiredElements := map[string]string{
		"(version 1)":        "version declaration",
		"(deny default)":     "deny-by-default policy",
		"(allow file-read*)": "read-only file access",
	}

	for element, description := range requiredElements {
		if !strings.Contains(profile, element) {
			t.Errorf("Profile missing %s: %s", description, element)
		}
	}

	// For read-only policy, should NOT have file-write
	if strings.Contains(profile, "(allow file-write*") {
		t.Error("Profile should not allow file-write for read-only policy")
	}

	// Should NOT have network access for read-only policy
	if strings.Contains(profile, "(allow network-outbound)") {
		t.Error("Profile should not allow network for read-only policy")
	}

	// Verify environment variables
	foundSandboxEnv := false
	for _, env := range cmd.Env {
		if env == "CODEX_SANDBOX=seatbelt" {
			foundSandboxEnv = true
			break
		}
	}

	if !foundSandboxEnv {
		t.Error("CODEX_SANDBOX environment variable not set")
	}

	t.Log("SUCCESS: Seatbelt sandbox integration verified!")
	t.Logf("- Command wrapped with sandbox-exec: %s", cmd.Path)
	t.Log("- Profile generated correctly (version 1, deny default, allow file-read*)")
	t.Log("- Environment variables set correctly")
}

// TestSeatbeltWorkspaceWritePolicy verifies workspace-write policy generates correct profile.
func TestSeatbeltWorkspaceWritePolicy(t *testing.T) {
	sm := NewSandboxManager()
	cmd := exec.Command("echo", "test")

	// Create workspace-write policy with network access
	policy := NewWorkspaceWritePolicy()
	policy.WorkspaceWriteConfig.NetworkAccess = true
	policy.WorkspaceWriteConfig.WritableRoots = []string{"/tmp/custom"}

	info, err := sm.ApplyToCommand(cmd, policy, "/tmp/workspace")
	if err != nil {
		t.Fatalf("Failed to apply sandbox: %v", err)
	}

	if !info.Applied {
		t.Fatal("Sandbox was not applied")
	}

	// Get the profile from cmd.Args[2]
	if len(cmd.Args) < 3 {
		t.Fatalf("Not enough arguments")
	}

	profile := cmd.Args[2]

	// Should allow file read and write
	if !strings.Contains(profile, "(allow file-read*)") {
		t.Error("Profile should allow file-read")
	}

	if !strings.Contains(profile, "(allow file-write*") {
		t.Error("Profile should allow file-write for workspace-write policy")
	}

	// Should include workspace path
	if !strings.Contains(profile, "/tmp/workspace") {
		t.Error("Profile should include workspace path")
	}

	// Should allow network access
	if !strings.Contains(profile, "(allow network-outbound)") {
		t.Error("Profile should allow network-outbound when NetworkAccess is enabled")
	}

	if !strings.Contains(profile, "(allow system-socket)") {
		t.Error("Profile should allow system-socket when NetworkAccess is enabled")
	}

	t.Log("SUCCESS: Workspace-write policy verified!")
}

// TestSeatbeltDangerFullAccessPolicy verifies full access policy.
func TestSeatbeltDangerFullAccessPolicy(t *testing.T) {
	sm := NewSandboxManager()
	cmd := exec.Command("echo", "test")

	policy := NewDangerFullAccessPolicy()

	info, err := sm.ApplyToCommand(cmd, policy, "/tmp")
	if err != nil {
		t.Fatalf("Failed to apply sandbox: %v", err)
	}

	// Full access policy should NOT apply sandbox
	if info.Applied {
		t.Error("Full access policy should not apply sandbox")
	}

	if info.Type != SandboxTypeNone {
		t.Errorf("Expected SandboxTypeNone, got %s", info.Type)
	}

	// Command should NOT be wrapped
	if cmd.Path == "/usr/bin/sandbox-exec" {
		t.Error("Command should not be wrapped with sandbox-exec for full access policy")
	}

	t.Log("SUCCESS: Full access policy correctly skips sandboxing")
}

// TestSeatbeltReadOnlyCommandExecution verifies sandboxed command actually runs.
func TestSeatbeltReadOnlyCommandExecution(t *testing.T) {
	sm := NewSandboxManager()
	cmd := exec.Command("echo", "hello sandbox")

	policy := NewReadOnlyPolicy()

	info, err := sm.ApplyToCommand(cmd, policy, "/tmp")
	if err != nil {
		t.Fatalf("Failed to apply sandbox: %v", err)
	}

	if !info.Applied {
		t.Fatal("Sandbox was not applied")
	}

	// Actually run the command
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to run sandboxed command: %v", err)
	}

	expectedOutput := "hello sandbox\n"
	if string(output) != expectedOutput {
		t.Errorf("Unexpected output: got %q, want %q", string(output), expectedOutput)
	}

	t.Log("SUCCESS: Sandboxed command executed correctly")
	t.Logf("Output: %s", string(output))
}
