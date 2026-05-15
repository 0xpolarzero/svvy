package sandbox

import (
	"os/exec"
)

// SandboxApplier is the interface for platform-specific sandbox implementations.
// Each OS-specific sandbox (Seatbelt, Landlock, Seccomp) implements this interface.
type SandboxApplier interface {
	// Name returns the sandbox implementation name (e.g., "seatbelt", "landlock", "seccomp").
	Name() string

	// IsAvailable checks if this sandbox is available on the current system.
	// For example, Landlock checks kernel version, Seatbelt checks for macOS.
	IsAvailable() bool

	// Apply configures the command to run within the sandbox with the given policy.
	// It may wrap the command with a sandbox wrapper executable or set process attributes.
	// workspace is the current working directory that defines the workspace root.
	Apply(cmd *exec.Cmd, policy *PolicyConfig, workspace string) error
}

// SandboxType represents the type of sandbox being used.
type SandboxType int

const (
	// SandboxTypeNone indicates no sandbox is being used.
	SandboxTypeNone SandboxType = iota

	// SandboxTypeSeatbelt indicates macOS Seatbelt sandbox.
	SandboxTypeSeatbelt

	// SandboxTypeLandlock indicates Linux Landlock LSM (kernel >= 5.13).
	SandboxTypeLandlock

	// SandboxTypeSeccomp indicates Linux Seccomp-BPF (kernel < 5.13).
	SandboxTypeSeccomp
)

// String returns the string representation of the sandbox type.
func (st SandboxType) String() string {
	switch st {
	case SandboxTypeNone:
		return "none"
	case SandboxTypeSeatbelt:
		return "seatbelt"
	case SandboxTypeLandlock:
		return "landlock"
	case SandboxTypeSeccomp:
		return "seccomp"
	default:
		return "unknown"
	}
}

// SandboxInfo contains metadata about the applied sandbox.
type SandboxInfo struct {
	// Type is the sandbox type that was applied.
	Type SandboxType

	// Applied indicates whether sandboxing was successfully applied.
	Applied bool

	// Reason provides additional context (e.g., why sandbox wasn't applied).
	Reason string
}
