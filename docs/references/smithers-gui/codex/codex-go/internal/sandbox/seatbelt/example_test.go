//go:build darwin

package seatbelt_test

import (
	"fmt"
	"log"

	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/evmts/codex/codex-go/internal/sandbox/seatbelt"
)

// Example_readOnlyProfile demonstrates using a read-only sandbox profile.
func Example_readOnlyProfile() {
	if !seatbelt.IsSupported() {
		fmt.Println("Seatbelt not supported on this platform")
		return
	}

	// Generate a read-only profile
	profile := seatbelt.ReadOnlyProfile()
	fmt.Println("Profile allows file-read:", len(profile) > 0)

	// In a real application, you would apply this to the process:
	// err := seatbelt.ApplyProfile(profile)
	// if err != nil {
	//     log.Fatalf("Failed to apply sandbox: %v", err)
	// }

	// Output: Profile allows file-read: true
}

// Example_workspaceWriteProfile demonstrates using a workspace-write sandbox profile.
func Example_workspaceWriteProfile() {
	if !seatbelt.IsSupported() {
		fmt.Println("Seatbelt not supported on this platform")
		return
	}

	// Generate a workspace-write profile with network access disabled
	workspace := "/tmp/my-workspace"
	networkAccess := false
	excludeTmpDir := false
	excludeSlashTmp := false

	profile := seatbelt.WorkspaceWriteProfile(workspace, networkAccess, excludeTmpDir, excludeSlashTmp)
	fmt.Println("Profile generated:", len(profile) > 0)

	// Output: Profile generated: true
}

// Example_protocolIntegration demonstrates converting a protocol.SandboxPolicy.
func Example_protocolIntegration() {
	policy := &protocol.SandboxPolicy{
		Mode:          "workspace-write",
		WritableRoots: []string{"/tmp/workspace1", "/tmp/workspace2"},
		NetworkAccess: true,
	}

	cwd := "/tmp/workspace1"
	profile := seatbelt.GenerateProfileFromSandboxPolicy(policy, cwd)

	fmt.Println("Profile generated from protocol policy:", len(profile) > 0)
	// Output: Profile generated from protocol policy: true
}

// Example_customProfile demonstrates creating a custom profile configuration.
func Example_customProfile() {
	config := &seatbelt.ProfileConfig{
		AllowFileRead:  true,
		AllowFileWrite: true,
		WritableRoots: []seatbelt.WritableRoot{
			{
				Root:             "/workspace",
				ReadOnlySubpaths: []string{"/workspace/.git", "/workspace/node_modules"},
			},
		},
		AllowNetworkOutbound: true,
		AllowNetworkInbound:  false,
		AllowSystemSocket:    true,
	}

	profile := seatbelt.GenerateProfile(config)
	fmt.Println("Custom profile generated:", len(profile) > 0)
	// Output: Custom profile generated: true
}

// Example_applyProfile demonstrates applying a sandbox profile.
// NOTE: This is a documentation example only. Actually applying a sandbox
// would make the test process sandboxed and could break subsequent tests.
func Example_applyProfile() {
	if !seatbelt.IsSupported() {
		log.Println("Seatbelt not supported on this platform")
		return
	}

	// Generate a profile
	_ = seatbelt.ReadOnlyProfile()

	// Apply the profile to the current process
	// WARNING: This is irreversible!
	// err := seatbelt.ApplyProfile(profile)
	// if err != nil {
	//     log.Fatalf("Failed to apply sandbox: %v", err)
	// }

	// All operations after this point would be sandboxed
	fmt.Println("Profile ready to apply")
	// Output: Profile ready to apply
}

// Example_multipleWritableRoots demonstrates a profile with multiple writable roots.
func Example_multipleWritableRoots() {
	roots := []string{
		"/tmp/workspace1",
		"/tmp/workspace2",
		"/tmp/output",
	}

	profile := seatbelt.WorkspaceWriteProfileMultiRoot(roots, false, false, false)
	fmt.Println("Multi-root profile generated:", len(profile) > 0)
	// Output: Multi-root profile generated: true
}
