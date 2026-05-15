package orchestrator

import (
	"os"
	"path/filepath"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// SandboxSelector determines the appropriate sandbox configuration for tool execution.
// It implements an escalation strategy: native (bubblewrap) → docker → kubernetes → none.
//
// The selector considers:
//   - Tool sandbox preference (Auto, Require, Forbid)
//   - System sandbox policy (ReadOnly, WorkspaceWrite, DangerFullAccess)
//   - Request-specific flags (escalated permissions)
//   - Retry attempts (escalate after sandbox denial)
type SandboxSelector struct {
	// bubblewrapAvailable indicates if bubblewrap is installed
	bubblewrapAvailable bool
	// dockerAvailable indicates if docker is available
	dockerAvailable bool
	// kubernetesAvailable indicates if kubectl is available
	kubernetesAvailable bool
}

// NewSandboxSelector creates a new sandbox selector.
// It auto-detects available sandbox technologies.
func NewSandboxSelector() *SandboxSelector {
	return &SandboxSelector{
		bubblewrapAvailable: checkBubblewrapAvailable(),
		dockerAvailable:     checkDockerAvailable(),
		kubernetesAvailable: checkKubernetesAvailable(),
	}
}

// SelectSandbox determines the sandbox configuration for a tool execution attempt.
// It returns a SandboxAttempt describing the sandbox to use.
//
// Parameters:
//   - tool: The tool to execute
//   - req: The tool request
//   - policy: The system sandbox policy
//   - skipSandbox: If true, skip sandbox (for retry attempts)
func (s *SandboxSelector) SelectSandbox(
	tool runtime.ToolRuntime,
	req *runtime.ToolRequest,
	policy runtime.SandboxPolicy,
	skipSandbox bool,
) *runtime.SandboxAttempt {
	// Check if tool wants escalated permissions
	wantsEscalated := tool.WantsEscalatedFirstAttempt(req)
	if wantsEscalated || skipSandbox {
		return s.createNoSandboxAttempt(req, policy)
	}

	// Check tool sandbox preference
	preference := tool.SandboxPreference()

	switch preference {
	case runtime.SandboxForbid:
		// Tool explicitly forbids sandbox
		return s.createNoSandboxAttempt(req, policy)

	case runtime.SandboxRequire:
		// Tool requires sandbox - use best available
		return s.selectBestSandbox(req, policy, true)

	case runtime.SandboxAuto:
		// Use sandbox based on policy
		if policy == runtime.SandboxDangerFullAccess {
			return s.createNoSandboxAttempt(req, policy)
		}
		return s.selectBestSandbox(req, policy, false)

	default:
		// Default to auto behavior
		return s.selectBestSandbox(req, policy, false)
	}
}

// selectBestSandbox chooses the best available sandbox technology.
// Priority order: bubblewrap → docker → kubernetes → none (if required=false)
func (s *SandboxSelector) selectBestSandbox(
	req *runtime.ToolRequest,
	policy runtime.SandboxPolicy,
	required bool,
) *runtime.SandboxAttempt {
	// Try bubblewrap first (native Linux sandboxing)
	if s.bubblewrapAvailable {
		return s.createBubblewrapAttempt(req, policy)
	}

	// Try docker
	if s.dockerAvailable {
		return s.createDockerAttempt(req, policy)
	}

	// Try kubernetes
	if s.kubernetesAvailable {
		return s.createKubernetesAttempt(req, policy)
	}

	// If sandbox is required but none available, still create a sandbox attempt
	// The actual execution will fail with appropriate error
	if required {
		return s.createBubblewrapAttempt(req, policy)
	}

	// Fall back to no sandbox
	return s.createNoSandboxAttempt(req, policy)
}

// createBubblewrapAttempt creates a sandbox attempt using bubblewrap.
func (s *SandboxSelector) createBubblewrapAttempt(
	req *runtime.ToolRequest,
	policy runtime.SandboxPolicy,
) *runtime.SandboxAttempt {
	workDir := req.WorkingDirectory
	if workDir == "" {
		workDir = "."
	}

	// Resolve to absolute path
	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		absWorkDir = workDir
	}

	attempt := &runtime.SandboxAttempt{
		Type:             runtime.SandboxBubblewrap,
		Policy:           policy,
		WorkingDirectory: absWorkDir,
		ReadOnlyPaths:    s.getReadOnlyPaths(policy),
		ReadWritePaths:   s.getReadWritePaths(policy, absWorkDir),
		NetworkEnabled:   s.isNetworkEnabled(policy),
	}

	return attempt
}

// createDockerAttempt creates a sandbox attempt using Docker.
func (s *SandboxSelector) createDockerAttempt(
	req *runtime.ToolRequest,
	policy runtime.SandboxPolicy,
) *runtime.SandboxAttempt {
	workDir := req.WorkingDirectory
	if workDir == "" {
		workDir = "."
	}

	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		absWorkDir = workDir
	}

	attempt := &runtime.SandboxAttempt{
		Type:             runtime.SandboxDocker,
		Policy:           policy,
		WorkingDirectory: absWorkDir,
		ReadOnlyPaths:    s.getReadOnlyPaths(policy),
		ReadWritePaths:   s.getReadWritePaths(policy, absWorkDir),
		NetworkEnabled:   s.isNetworkEnabled(policy),
		SandboxRoot:      "/sandbox",
	}

	return attempt
}

// createKubernetesAttempt creates a sandbox attempt using Kubernetes.
func (s *SandboxSelector) createKubernetesAttempt(
	req *runtime.ToolRequest,
	policy runtime.SandboxPolicy,
) *runtime.SandboxAttempt {
	workDir := req.WorkingDirectory
	if workDir == "" {
		workDir = "."
	}

	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		absWorkDir = workDir
	}

	attempt := &runtime.SandboxAttempt{
		Type:             runtime.SandboxKubernetes,
		Policy:           policy,
		WorkingDirectory: absWorkDir,
		ReadOnlyPaths:    s.getReadOnlyPaths(policy),
		ReadWritePaths:   s.getReadWritePaths(policy, absWorkDir),
		NetworkEnabled:   s.isNetworkEnabled(policy),
		SandboxRoot:      "/sandbox",
	}

	return attempt
}

// createNoSandboxAttempt creates an attempt with no sandboxing.
func (s *SandboxSelector) createNoSandboxAttempt(
	req *runtime.ToolRequest,
	policy runtime.SandboxPolicy,
) *runtime.SandboxAttempt {
	workDir := req.WorkingDirectory
	if workDir == "" {
		workDir = "."
	}

	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		absWorkDir = workDir
	}

	return &runtime.SandboxAttempt{
		Type:             runtime.SandboxNone,
		Policy:           policy,
		WorkingDirectory: absWorkDir,
		NetworkEnabled:   true, // No restrictions when not sandboxed
	}
}

// getReadOnlyPaths returns the list of read-only paths based on policy.
func (s *SandboxSelector) getReadOnlyPaths(policy runtime.SandboxPolicy) []string {
	// Standard system paths that should be read-only
	readOnlyPaths := []string{
		"/usr",
		"/lib",
		"/lib64",
		"/bin",
		"/sbin",
		"/etc",
	}

	return readOnlyPaths
}

// getReadWritePaths returns the list of read-write paths based on policy.
func (s *SandboxSelector) getReadWritePaths(policy runtime.SandboxPolicy, workDir string) []string {
	var readWritePaths []string

	switch policy {
	case runtime.SandboxReadOnly:
		// No write paths in read-only mode
		readWritePaths = []string{}

	case runtime.SandboxWorkspaceWrite:
		// Allow writes to workspace
		readWritePaths = []string{workDir}

		// Also allow /tmp for temporary files
		readWritePaths = append(readWritePaths, "/tmp")

	case runtime.SandboxDangerFullAccess:
		// Full access - no restrictions
		readWritePaths = []string{"/"}

	default:
		// Default to workspace write
		readWritePaths = []string{workDir, "/tmp"}
	}

	return readWritePaths
}

// isNetworkEnabled determines if network access should be enabled.
func (s *SandboxSelector) isNetworkEnabled(policy runtime.SandboxPolicy) bool {
	// Network is only enabled in DangerFullAccess mode
	return policy == runtime.SandboxDangerFullAccess
}

// checkBubblewrapAvailable checks if bubblewrap is installed on the system.
func checkBubblewrapAvailable() bool {
	// Check if bwrap executable exists in PATH
	_, err := os.Stat("/usr/bin/bwrap")
	if err == nil {
		return true
	}

	// Try common alternative locations
	_, err = os.Stat("/usr/local/bin/bwrap")
	if err == nil {
		return true
	}

	// Could also check PATH, but for simplicity we check common locations
	return false
}

// checkDockerAvailable checks if Docker is available on the system.
func checkDockerAvailable() bool {
	// Check if docker executable exists in PATH
	_, err := os.Stat("/usr/bin/docker")
	if err == nil {
		return true
	}

	// Try common alternative locations
	_, err = os.Stat("/usr/local/bin/docker")
	if err == nil {
		return true
	}

	// Check macOS Docker Desktop location
	_, err = os.Stat("/Applications/Docker.app/Contents/Resources/bin/docker")
	return err == nil
}

// checkKubernetesAvailable checks if kubectl is available on the system.
func checkKubernetesAvailable() bool {
	// Check if kubectl executable exists in PATH
	_, err := os.Stat("/usr/bin/kubectl")
	if err == nil {
		return true
	}

	// Try common alternative locations
	_, err = os.Stat("/usr/local/bin/kubectl")
	return err == nil
}

// EscalateSandbox attempts to escalate to the next level of sandboxing.
// Used when the current sandbox level fails.
//
// Escalation path: bubblewrap → docker → kubernetes → none
func (s *SandboxSelector) EscalateSandbox(
	current *runtime.SandboxAttempt,
) *runtime.SandboxAttempt {
	switch current.Type {
	case runtime.SandboxBubblewrap:
		// Try docker next
		if s.dockerAvailable {
			return &runtime.SandboxAttempt{
				Type:             runtime.SandboxDocker,
				Policy:           current.Policy,
				WorkingDirectory: current.WorkingDirectory,
				ReadOnlyPaths:    current.ReadOnlyPaths,
				ReadWritePaths:   current.ReadWritePaths,
				NetworkEnabled:   current.NetworkEnabled,
				SandboxRoot:      "/sandbox",
			}
		}
		// Fall through to kubernetes

	case runtime.SandboxDocker:
		// Try kubernetes next
		if s.kubernetesAvailable {
			return &runtime.SandboxAttempt{
				Type:             runtime.SandboxKubernetes,
				Policy:           current.Policy,
				WorkingDirectory: current.WorkingDirectory,
				ReadOnlyPaths:    current.ReadOnlyPaths,
				ReadWritePaths:   current.ReadWritePaths,
				NetworkEnabled:   current.NetworkEnabled,
				SandboxRoot:      "/sandbox",
			}
		}
		// Fall through to none

	case runtime.SandboxKubernetes:
		// Kubernetes failed, disable sandbox
		// Fall through to none
	}

	// Final escalation: no sandbox
	return &runtime.SandboxAttempt{
		Type:             runtime.SandboxNone,
		Policy:           current.Policy,
		WorkingDirectory: current.WorkingDirectory,
		NetworkEnabled:   true,
	}
}

// ShouldRetryWithoutSandbox determines if a failed execution should be retried
// without sandbox restrictions.
func (s *SandboxSelector) ShouldRetryWithoutSandbox(
	tool runtime.ToolRuntime,
	attempt *runtime.SandboxAttempt,
	err error,
) bool {
	// Check if error is a sandbox denial
	if toolErr, ok := err.(*runtime.ToolError); ok {
		if toolErr.Kind != runtime.ErrorSandboxDenied {
			return false
		}
	} else {
		return false
	}

	// Check if tool supports escalation
	if !tool.EscalateOnFailure() {
		return false
	}

	// Check if tool has retry data (commands can retry, other tools may not)
	if tool.SandboxRetryData(nil) == nil {
		return false
	}

	// Don't retry if already running without sandbox
	if attempt.Type == runtime.SandboxNone {
		return false
	}

	return true
}
