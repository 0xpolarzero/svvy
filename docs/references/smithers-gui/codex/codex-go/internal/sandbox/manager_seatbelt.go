//go:build darwin

package sandbox

import (
	"log"
	"os"
	"os/exec"

	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/evmts/codex/codex-go/internal/sandbox/seatbelt"
)

// =============================================================================
// macOS Seatbelt Implementation
// =============================================================================

type seatbeltSandbox struct {
	logger *log.Logger
}

func (s *seatbeltSandbox) Name() string {
	return "seatbelt"
}

func (s *seatbeltSandbox) IsAvailable() bool {
	// Check if sandbox-exec exists
	if _, err := os.Stat(seatbelt.SeatbeltExecutablePath); err != nil {
		if s.logger != nil {
			s.logger.Printf("WARNING: sandbox-exec not found at %s: %v\n", seatbelt.SeatbeltExecutablePath, err)
		}
		return false
	}
	return true
}

func (s *seatbeltSandbox) Apply(cmd *exec.Cmd, policy *PolicyConfig, workspace string) error {
	// Generate sandbox profile based on policy
	protocolPolicy := policyToProtocolSandboxPolicy(policy, workspace)
	profile := seatbelt.GenerateProfileFromSandboxPolicy(protocolPolicy, workspace)

	// Log the policy for debugging
	if s.logger != nil {
		s.logger.Printf("DEBUG: Generated Seatbelt profile for policy %s:\n%s\n", policy.Policy, profile)
	}

	// Wrap the command with sandbox-exec
	// Original command: cmd.Path + cmd.Args
	// New command: /usr/bin/sandbox-exec -p <profile> <original command>

	// Save original command details
	originalPath := cmd.Path
	originalArgs := cmd.Args

	// If Path is empty but Args[0] exists, use it
	if originalPath == "" && len(originalArgs) > 0 {
		originalPath = originalArgs[0]
	}

	// Reconstruct original command as string
	// We need to be careful here - we're passing the profile via -p flag
	// and then the original command follows

	// Set up sandbox-exec as the new command
	cmd.Path = seatbelt.SeatbeltExecutablePath
	cmd.Args = []string{
		"sandbox-exec", // argv[0]
		"-p",           // profile flag
		profile,        // profile content
		originalPath,   // original command path
	}

	// Append original arguments (skip argv[0] which is the command name)
	if len(originalArgs) > 1 {
		cmd.Args = append(cmd.Args, originalArgs[1:]...)
	}

	// Set environment variables for debugging/informational purposes
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "CODEX_SANDBOX=seatbelt")
	if !policy.HasFullNetworkAccess() {
		cmd.Env = append(cmd.Env, "CODEX_SANDBOX_NETWORK_DISABLED=1")
	}

	if s.logger != nil {
		s.logger.Printf("DEBUG: Wrapped command with sandbox-exec: %v\n", cmd.Args)
	}

	return nil
}

// policyToProtocolSandboxPolicy converts a PolicyConfig to protocol.SandboxPolicy.
// This is needed because the seatbelt package uses protocol.SandboxPolicy for profile generation.
func policyToProtocolSandboxPolicy(policy *PolicyConfig, workspace string) *protocol.SandboxPolicy {
	protocolPolicy := &protocol.SandboxPolicy{
		Mode:                "",
		WritableRoots:       []string{},
		NetworkAccess:       false,
		ExcludeTmpdirEnvVar: false,
		ExcludeSlashTmp:     false,
	}

	switch policy.Policy {
	case PolicyReadOnly:
		protocolPolicy.Mode = "read-only"

	case PolicyWorkspaceWrite:
		protocolPolicy.Mode = "workspace-write"
		protocolPolicy.NetworkAccess = policy.HasFullNetworkAccess()

		// Add writable roots from policy configuration
		if policy.WorkspaceWriteConfig != nil {
			protocolPolicy.WritableRoots = append(protocolPolicy.WritableRoots, policy.WorkspaceWriteConfig.WritableRoots...)
			protocolPolicy.ExcludeTmpdirEnvVar = policy.WorkspaceWriteConfig.ExcludeTmpdirEnvVar
			protocolPolicy.ExcludeSlashTmp = policy.WorkspaceWriteConfig.ExcludeSlashTmp
		}

	case PolicyDangerFullAccess:
		protocolPolicy.Mode = "danger-full-access"
		protocolPolicy.NetworkAccess = true

	default:
		// Default to read-only for safety
		protocolPolicy.Mode = "read-only"
	}

	return protocolPolicy
}
