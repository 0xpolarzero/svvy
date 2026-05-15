//go:build !darwin

package sandbox

import (
	"log"
	"os/exec"
	"runtime"
)

// =============================================================================
// macOS Seatbelt Implementation (stub for non-macOS platforms)
// =============================================================================

type seatbeltSandbox struct {
	logger *log.Logger
}

func (s *seatbeltSandbox) Name() string {
	return "seatbelt"
}

func (s *seatbeltSandbox) IsAvailable() bool {
	// Only available on macOS
	return runtime.GOOS == "darwin"
}

func (s *seatbeltSandbox) Apply(cmd *exec.Cmd, policy *PolicyConfig, workspace string) error {
	// This should never be called since IsAvailable() returns false
	return nil
}
