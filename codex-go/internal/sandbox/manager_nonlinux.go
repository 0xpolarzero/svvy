//go:build !linux

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
)

// =============================================================================
// Linux Seccomp Implementation (Stub for non-Linux platforms)
// =============================================================================

type seccompSandbox struct{}

func (s *seccompSandbox) Name() string {
	return "seccomp"
}

func (s *seccompSandbox) IsAvailable() bool {
	return false
}

func (s *seccompSandbox) Apply(cmd *exec.Cmd, policy *PolicyConfig, workspace string) error {
	// Seccomp is only available on Linux
	return fmt.Errorf("seccomp is not available on this platform")
}

// Stub implementations for Linux-specific functions that may be called
// These are no-ops on non-Linux platforms

func init() {
	// Ensure environment is set up if needed
	_ = os.Getenv("PATH")
}
