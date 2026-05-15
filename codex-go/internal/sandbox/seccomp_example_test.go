//go:build linux

package sandbox_test

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/evmts/codex/codex-go/internal/sandbox"
)

// ExampleSandboxManager_seccomp demonstrates using the Seccomp sandbox
// on Linux to restrict syscalls for a command.
func ExampleSandboxManager_seccomp() {
	// Create a sandbox manager
	sm := sandbox.NewSandboxManager()

	// Create a read-only policy (blocks network syscalls)
	policy := sandbox.NewReadOnlyPolicy()

	// Create a command to run
	cmd := exec.Command("/bin/echo", "Hello from sandboxed process")

	// Apply sandbox restrictions
	info, err := sm.ApplyToCommand(cmd, policy, "/tmp/workspace")
	if err != nil {
		log.Fatalf("Failed to apply sandbox: %v", err)
	}

	fmt.Printf("Sandbox applied: %v (type: %s)\n", info.Applied, info.Type)

	// Run the command (seccomp filter would be applied if wrapper is configured)
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Command failed: %v", err)
	}

	fmt.Printf("Output: %s\n", output)
	// Output:
	// Sandbox applied: true (type: seccomp)
	// Output: Hello from sandboxed process
}

// ExampleSandboxManager_workspaceWriteNoNetwork demonstrates a workspace-write
// policy with network restrictions on Linux.
func ExampleSandboxManager_workspaceWriteNoNetwork() {
	sm := sandbox.NewSandboxManager()

	// Create workspace-write policy (no network by default)
	policy := sandbox.NewWorkspaceWritePolicy()

	cmd := exec.Command("/bin/pwd")

	info, err := sm.ApplyToCommand(cmd, policy, "/tmp/workspace")
	if err != nil {
		log.Fatalf("Failed to apply sandbox: %v", err)
	}

	fmt.Printf("Network restricted: %v\n", !policy.HasFullNetworkAccess())
	fmt.Printf("Sandbox type: %s\n", info.Type)

	// Output:
	// Network restricted: true
	// Sandbox type: seccomp
}

// ExampleSandboxManager_workspaceWriteWithNetwork demonstrates a workspace-write
// policy with network access enabled.
func ExampleSandboxManager_workspaceWriteWithNetwork() {
	sm := sandbox.NewSandboxManager()

	// Create workspace-write policy with network access
	policy := &sandbox.PolicyConfig{
		Policy: sandbox.PolicyWorkspaceWrite,
		WorkspaceWriteConfig: &sandbox.WorkspaceWriteConfig{
			NetworkAccess: true,
		},
	}

	cmd := exec.Command("/bin/echo", "Network enabled")

	info, err := sm.ApplyToCommand(cmd, policy, "/tmp/workspace")
	if err != nil {
		log.Fatalf("Failed to apply sandbox: %v", err)
	}

	fmt.Printf("Network enabled: %v\n", policy.HasFullNetworkAccess())
	fmt.Printf("Sandbox applied: %v\n", info.Applied)

	// Output:
	// Network enabled: true
	// Sandbox applied: true
}

// ExampleSeccompFilter demonstrates the different policy levels
func ExampleSeccompFilter() {
	// Seccomp provides three levels of filtering:
	//
	// 1. Read-only policy:
	//    - Blocks network syscalls (except AF_UNIX)
	//    - Allows filesystem reads
	//    - Use for untrusted code that only needs to read data
	readOnly := sandbox.NewReadOnlyPolicy()
	fmt.Printf("Read-only has network: %v\n", readOnly.HasFullNetworkAccess())

	// 2. Workspace-write policy (no network):
	//    - Blocks network syscalls
	//    - Allows filesystem reads and writes
	//    - Use for trusted code that needs to modify files
	workspaceWrite := sandbox.NewWorkspaceWritePolicy()
	fmt.Printf("Workspace-write has network: %v\n", workspaceWrite.HasFullNetworkAccess())

	// 3. Workspace-write policy (with network):
	//    - Allows all operations
	//    - Still provides process isolation via Setpgid
	//    - Use for code that needs network access
	workspaceWithNetwork := &sandbox.PolicyConfig{
		Policy: sandbox.PolicyWorkspaceWrite,
		WorkspaceWriteConfig: &sandbox.WorkspaceWriteConfig{
			NetworkAccess: true,
		},
	}
	fmt.Printf("Workspace with network has network: %v\n", workspaceWithNetwork.HasFullNetworkAccess())

	// Output:
	// Read-only has network: false
	// Workspace-write has network: false
	// Workspace with network has network: true
}
