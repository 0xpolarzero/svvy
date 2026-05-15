// Package sandbox provides policy enforcement and OS-specific sandbox selection.
package sandbox

import (
	"os"
	"path/filepath"
)

// Policy represents sandbox access control policies.
// This mirrors the Rust SandboxPolicy enum from codex-rs/core/src/protocol.rs
type Policy int

const (
	// PolicyReadOnly grants read-only access to the entire filesystem.
	PolicyReadOnly Policy = iota

	// PolicyWorkspaceWrite grants read access to the entire filesystem
	// and write access to the workspace directory (and optionally temp dirs).
	PolicyWorkspaceWrite

	// PolicyDangerFullAccess disables all sandbox restrictions.
	// Use with extreme caution - this allows unrestricted system access.
	PolicyDangerFullAccess
)

// String returns the string representation of the policy.
func (p Policy) String() string {
	switch p {
	case PolicyReadOnly:
		return "read-only"
	case PolicyWorkspaceWrite:
		return "workspace-write"
	case PolicyDangerFullAccess:
		return "danger-full-access"
	default:
		return "unknown"
	}
}

// WorkspaceWriteConfig contains additional configuration for PolicyWorkspaceWrite.
type WorkspaceWriteConfig struct {
	// WritableRoots are additional directories (beyond workspace) that should be writable.
	WritableRoots []string

	// NetworkAccess enables network access when true. Default is false.
	NetworkAccess bool

	// ExcludeTmpdirEnvVar when true, excludes the TMPDIR environment variable
	// from writable roots. Default is false (TMPDIR is included).
	ExcludeTmpdirEnvVar bool

	// ExcludeSlashTmp when true, excludes /tmp from writable roots on Unix.
	// Default is false (/tmp is included).
	ExcludeSlashTmp bool
}

// PolicyConfig encapsulates a policy along with its configuration.
type PolicyConfig struct {
	Policy               Policy
	WorkspaceWriteConfig *WorkspaceWriteConfig
}

// NewPolicyConfig creates a new policy configuration.
func NewPolicyConfig(policy Policy, config *WorkspaceWriteConfig) *PolicyConfig {
	return &PolicyConfig{
		Policy:               policy,
		WorkspaceWriteConfig: config,
	}
}

// NewReadOnlyPolicy creates a read-only policy configuration.
func NewReadOnlyPolicy() *PolicyConfig {
	return &PolicyConfig{
		Policy: PolicyReadOnly,
	}
}

// NewWorkspaceWritePolicy creates a workspace-write policy configuration with defaults.
func NewWorkspaceWritePolicy() *PolicyConfig {
	return &PolicyConfig{
		Policy: PolicyWorkspaceWrite,
		WorkspaceWriteConfig: &WorkspaceWriteConfig{
			WritableRoots:       []string{},
			NetworkAccess:       false,
			ExcludeTmpdirEnvVar: false,
			ExcludeSlashTmp:     false,
		},
	}
}

// NewDangerFullAccessPolicy creates a full-access policy configuration.
func NewDangerFullAccessPolicy() *PolicyConfig {
	return &PolicyConfig{
		Policy: PolicyDangerFullAccess,
	}
}

// HasFullNetworkAccess returns true if the policy allows unrestricted network access.
func (pc *PolicyConfig) HasFullNetworkAccess() bool {
	switch pc.Policy {
	case PolicyDangerFullAccess:
		return true
	case PolicyWorkspaceWrite:
		return pc.WorkspaceWriteConfig != nil && pc.WorkspaceWriteConfig.NetworkAccess
	default:
		return false
	}
}

// GetWritableRoots returns the list of writable paths for the given workspace.
// This includes the workspace directory and any additional writable roots configured.
func (pc *PolicyConfig) GetWritableRoots(workspace string) []string {
	if pc.Policy != PolicyWorkspaceWrite {
		return []string{}
	}

	roots := []string{workspace}

	// Add temp directories unless explicitly excluded
	if pc.WorkspaceWriteConfig != nil {
		// Add custom writable roots
		roots = append(roots, pc.WorkspaceWriteConfig.WritableRoots...)

		// Add /tmp unless excluded (Unix only)
		if !pc.WorkspaceWriteConfig.ExcludeSlashTmp {
			roots = append(roots, "/tmp")
		}

		// Add TMPDIR environment variable location unless excluded
		if !pc.WorkspaceWriteConfig.ExcludeTmpdirEnvVar {
			if tmpdir := os.Getenv("TMPDIR"); tmpdir != "" {
				roots = append(roots, tmpdir)
			}
		}
	} else {
		// Default: include both /tmp and TMPDIR
		roots = append(roots, "/tmp")
		if tmpdir := os.Getenv("TMPDIR"); tmpdir != "" {
			roots = append(roots, tmpdir)
		}
	}

	// Clean and deduplicate paths
	return cleanAndDeduplicatePaths(roots)
}

// GetReadOnlyRoots returns the list of read-only paths.
// For read-only and workspace-write policies, this is typically the entire filesystem.
func (pc *PolicyConfig) GetReadOnlyRoots() []string {
	if pc.Policy == PolicyDangerFullAccess {
		return []string{}
	}
	// Grant read access to the entire filesystem
	return []string{"/"}
}

// ShouldSandbox returns true if this policy should use sandboxing.
func (pc *PolicyConfig) ShouldSandbox() bool {
	return pc.Policy != PolicyDangerFullAccess
}

// cleanAndDeduplicatePaths removes duplicates and resolves paths.
func cleanAndDeduplicatePaths(paths []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, path := range paths {
		// Clean the path to resolve . and .. and remove trailing slashes
		cleaned := filepath.Clean(path)

		if !seen[cleaned] {
			seen[cleaned] = true
			result = append(result, cleaned)
		}
	}

	return result
}

// ParsePolicy parses a policy string into a Policy type.
func ParsePolicy(s string) Policy {
	switch s {
	case "read-only":
		return PolicyReadOnly
	case "workspace-write":
		return PolicyWorkspaceWrite
	case "danger-full-access":
		return PolicyDangerFullAccess
	default:
		return PolicyReadOnly // Default to most restrictive
	}
}
