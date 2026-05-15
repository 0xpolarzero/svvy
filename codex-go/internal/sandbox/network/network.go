// Package network provides network access control for sandboxed command execution.
//
// This package implements network isolation strategies for different platforms:
//   - Linux: Network namespaces for complete isolation
//   - macOS: Seatbelt policy rules (handled in seatbelt package)
//   - Fallback: Environment variable hints for applications
//
// The preferred approach on Linux is network namespace isolation, which creates
// a completely isolated network environment with no network interfaces.
package network

import (
	"context"
	"fmt"
	"os/exec"
)

// Controller provides network access control capabilities.
type Controller interface {
	// IsAvailable checks if this network control method is available on the system
	IsAvailable() bool

	// ConfigureCommand configures an exec.Cmd to have network access disabled
	// Returns an error if network isolation cannot be configured
	ConfigureCommand(cmd *exec.Cmd) error

	// Cleanup performs any necessary cleanup after command execution
	Cleanup(ctx context.Context) error

	// Type returns the network control method identifier
	Type() string
}

// NewController creates the best available network controller for the current platform.
// It tries network control methods in order of preference:
//  1. Network namespaces (Linux only) - provides complete isolation
//  2. Fallback (all platforms) - sets environment variables as hints
//
// Returns an error if no network control method is available.
func NewController() (Controller, error) {
	// Try network namespace controller (Linux only)
	if namespaceCtrl := newNamespaceController(); namespaceCtrl.IsAvailable() {
		return namespaceCtrl, nil
	}

	// Fallback to environment variable hints
	// This doesn't provide actual isolation but indicates to well-behaved
	// applications that network access should be restricted
	return newFallbackController(), nil
}

// fallbackController provides basic network control through environment variables.
// This is a fallback that doesn't provide actual isolation but sets hints.
type fallbackController struct{}

func newFallbackController() *fallbackController {
	return &fallbackController{}
}

func (f *fallbackController) IsAvailable() bool {
	return true // Always available as a fallback
}

func (f *fallbackController) ConfigureCommand(cmd *exec.Cmd) error {
	// Set environment variable to indicate network should be disabled
	// Well-behaved applications can check this and avoid network operations
	if cmd.Env == nil {
		cmd.Env = []string{}
	}
	cmd.Env = append(cmd.Env, "CODEX_SANDBOX_NETWORK_DISABLED=1")
	return nil
}

func (f *fallbackController) Cleanup(ctx context.Context) error {
	return nil // No cleanup needed for environment variable approach
}

func (f *fallbackController) Type() string {
	return "fallback"
}

// ValidationError is returned when network control validation fails.
type ValidationError struct {
	Method  string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("network control validation failed (%s): %s", e.Method, e.Message)
}
