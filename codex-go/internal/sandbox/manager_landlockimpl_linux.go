//go:build linux

package sandbox

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	landlockPkg "github.com/evmts/codex/codex-go/internal/sandbox/landlock"
)

// =============================================================================
// Linux Landlock Implementation
// =============================================================================

type landlockSandbox struct {
	logger *log.Logger
}

func (l *landlockSandbox) Name() string {
	return "landlock"
}

func (l *landlockSandbox) IsAvailable() bool {
	// Use the actual Landlock syscall test instead of kernel version check
	// This is more reliable as it checks actual syscall availability
	return landlockPkg.IsSupported()
}

func (l *landlockSandbox) Apply(cmd *exec.Cmd, policy *PolicyConfig, workspace string) error {
	// Landlock restrictions must be applied to the current process before forking.
	// Once applied, they affect the current process and all future child processes.

	// Get writable roots based on policy
	var writableRoots []string
	switch policy.Policy {
	case PolicyReadOnly:
		// Read-only: no writable roots (except /dev/null for output)
		writableRoots = []string{}
	case PolicyWorkspaceWrite:
		// Workspace write: get configured writable roots
		writableRoots = policy.GetWritableRoots(workspace)
	case PolicyDangerFullAccess:
		// Full access: no Landlock restrictions
		if l.logger != nil {
			l.logger.Printf("INFO: Full access policy - skipping Landlock enforcement\n")
		}
		return nil
	}

	// Create a Landlock ruleset
	ruleset := landlockPkg.NewRuleset()

	// Allow read-only access to entire filesystem
	ruleset.AddReadOnlyPath("/")

	// Always allow write access to /dev/null (commonly needed for output redirection)
	ruleset.AddReadWritePath("/dev/null")

	// Add writable roots
	for _, root := range writableRoots {
		// Ensure path exists before adding rule
		if _, err := os.Stat(root); err == nil {
			ruleset.AddReadWritePath(root)
			if l.logger != nil {
				l.logger.Printf("DEBUG: Adding writable root to Landlock: %s\n", root)
			}
		} else if l.logger != nil {
			l.logger.Printf("WARNING: Skipping non-existent writable root: %s (%v)\n", root, err)
		}
	}

	// Apply the ruleset to the current process
	// This will affect this process and all children spawned after this call
	err := ruleset.TryApply()
	if err != nil {
		return fmt.Errorf("failed to apply landlock ruleset: %w", err)
	}

	// Set environment variables for debugging/informational purposes
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "CODEX_SANDBOX=landlock")
	if !policy.HasFullNetworkAccess() {
		cmd.Env = append(cmd.Env, "CODEX_SANDBOX_NETWORK_DISABLED=1")
	}

	if l.logger != nil {
		l.logger.Printf("INFO: Landlock sandbox applied successfully with %d writable roots\n", len(writableRoots))
	}

	return nil
}
