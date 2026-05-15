package sandbox

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// SandboxManager orchestrates sandbox selection and application based on OS and policy.
// This mirrors the functionality of Rust's SandboxManager in codex-rs/core/src/sandboxing/mod.rs
type SandboxManager struct {
	// appliers contains registered sandbox implementations, ordered by preference
	appliers []SandboxApplier

	// logger is used for warning messages (e.g., when sandbox is unavailable)
	logger *log.Logger
}

// NewSandboxManager creates a new sandbox manager with OS-appropriate defaults.
func NewSandboxManager() *SandboxManager {
	sm := &SandboxManager{
		logger: log.New(os.Stderr, "[sandbox] ", log.LstdFlags),
	}

	// Register sandbox implementations in order of preference
	sm.registerDefaultAppliers()

	return sm
}

// registerDefaultAppliers registers the appropriate sandbox implementations for the current OS.
func (sm *SandboxManager) registerDefaultAppliers() {
	switch runtime.GOOS {
	case "darwin":
		// macOS: Use Seatbelt
		sm.appliers = []SandboxApplier{
			&seatbeltSandbox{},
		}

	case "linux":
		// Linux: Try Landlock first (kernel >= 5.13), fall back to Seccomp
		landlock := &landlockSandbox{}
		seccomp := &seccompSandbox{}

		if landlock.IsAvailable() {
			sm.appliers = []SandboxApplier{landlock, seccomp}
		} else {
			sm.appliers = []SandboxApplier{seccomp}
		}

	case "windows":
		// Windows: No sandbox implementation yet
		sm.logger.Println("WARNING: Sandbox not supported on Windows - commands run with full system access")
		sm.appliers = []SandboxApplier{}

	default:
		sm.logger.Printf("WARNING: Sandbox not supported on %s - commands run with full system access\n", runtime.GOOS)
		sm.appliers = []SandboxApplier{}
	}
}

// ApplyToCommand applies the appropriate sandbox to the given command based on the policy.
// Returns SandboxInfo describing what sandbox was applied (if any).
func (sm *SandboxManager) ApplyToCommand(cmd *exec.Cmd, policy *PolicyConfig, workspace string) (*SandboxInfo, error) {
	// If policy doesn't require sandboxing, skip it
	if !policy.ShouldSandbox() {
		return &SandboxInfo{
			Type:    SandboxTypeNone,
			Applied: false,
			Reason:  "policy allows full access",
		}, nil
	}

	// Try to apply the first available sandbox
	for _, applier := range sm.appliers {
		if !applier.IsAvailable() {
			continue
		}

		err := applier.Apply(cmd, policy, workspace)
		if err != nil {
			return nil, fmt.Errorf("failed to apply %s sandbox: %w", applier.Name(), err)
		}

		return &SandboxInfo{
			Type:    sandboxTypeFromName(applier.Name()),
			Applied: true,
			Reason:  fmt.Sprintf("using %s sandbox", applier.Name()),
		}, nil
	}

	// No sandbox available - log warning and proceed without sandboxing
	sm.logger.Printf("WARNING: No sandbox available for policy %s - command will run with full system access\n", policy.Policy)

	return &SandboxInfo{
		Type:    SandboxTypeNone,
		Applied: false,
		Reason:  "no sandbox available on this system",
	}, nil
}

// GetAvailableSandbox returns the name of the best available sandbox, or empty string if none.
func (sm *SandboxManager) GetAvailableSandbox() string {
	for _, applier := range sm.appliers {
		if applier.IsAvailable() {
			return applier.Name()
		}
	}
	return ""
}

// sandboxTypeFromName converts a sandbox name to SandboxType.
func sandboxTypeFromName(name string) SandboxType {
	switch name {
	case "seatbelt":
		return SandboxTypeSeatbelt
	case "landlock":
		return SandboxTypeLandlock
	case "seccomp":
		return SandboxTypeSeccomp
	default:
		return SandboxTypeNone
	}
}

// =============================================================================
// macOS Seatbelt Implementation
// =============================================================================
// NOTE: Seatbelt implementation is in manager_seatbelt.go (macOS/darwin)
// and manager_seatbelt_stub.go (non-macOS platforms) due to build tag requirements.

// =============================================================================
// Linux Landlock Implementation
// =============================================================================
// NOTE: Landlock implementation is in manager_landlock.go (Linux)
// and manager_landlock_stub.go (non-Linux platforms) due to build tag requirements.

// =============================================================================
// Linux Seccomp Implementation
// =============================================================================
// The seccompSandbox type and methods are implemented in:
// - manager_linux.go (for Linux with full Seccomp-BPF support)
// - manager_nonlinux.go (stub implementation for other platforms)
//
// This allows the same SandboxManager code to work across all platforms
// with platform-specific sandbox implementations.

// =============================================================================
// Linux Kernel Version Detection
// =============================================================================
// NOTE: These functions are kept for testing purposes but are no longer used
// for Landlock detection. Use landlock.IsSupported() instead, which tests
// actual syscall availability rather than parsing kernel versions.

type kernelVersion struct {
	Major int
	Minor int
	Patch int
}

func getLinuxKernelVersion() (*kernelVersion, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("not running on Linux")
	}

	// Read /proc/version or use uname
	cmd := exec.Command("uname", "-r")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get kernel version: %w", err)
	}

	return parseKernelVersion(strings.TrimSpace(string(output)))
}

func parseKernelVersion(version string) (*kernelVersion, error) {
	// Version format is typically: 5.13.0-generic or 5.13.0-1234-generic
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid kernel version format: %s", version)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %w", err)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %w", err)
	}

	patch := 0
	if len(parts) >= 3 {
		// Extract numeric part before any suffix (e.g., "0" from "0-generic")
		patchStr := strings.Split(parts[2], "-")[0]
		patch, _ = strconv.Atoi(patchStr)
	}

	return &kernelVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
	}, nil
}

// SetNetworkDisabled is a helper to set the network disabled environment variable
// This can be checked by commands to voluntarily disable network access
const EnvNetworkDisabled = "CODEX_SANDBOX_NETWORK_DISABLED"
const EnvSandboxType = "CODEX_SANDBOX"
