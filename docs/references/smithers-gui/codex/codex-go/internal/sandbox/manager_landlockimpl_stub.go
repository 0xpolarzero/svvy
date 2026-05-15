//go:build !linux

package sandbox

import (
	"log"
	"os/exec"
)

// =============================================================================
// Linux Landlock Implementation (stub for non-Linux platforms)
// =============================================================================

type landlockSandbox struct {
	logger *log.Logger
}

func (l *landlockSandbox) Name() string {
	return "landlock"
}

func (l *landlockSandbox) IsAvailable() bool {
	// Only available on Linux with kernel >= 5.13
	return false
}

func (l *landlockSandbox) Apply(cmd *exec.Cmd, policy *PolicyConfig, workspace string) error {
	// This should never be called since IsAvailable() returns false
	return nil
}
