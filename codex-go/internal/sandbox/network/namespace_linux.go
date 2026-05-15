//go:build linux

// Package network provides Linux network namespace isolation.
//
// Network namespaces provide complete network isolation by creating a new
// network namespace with no network interfaces. This prevents all network
// access including localhost connections.
package network

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"golang.org/x/sys/unix"
)

// namespaceController implements network isolation using Linux network namespaces.
type namespaceController struct {
	// Store any namespace-specific state if needed
}

func newNamespaceController() *namespaceController {
	return &namespaceController{}
}

// IsAvailable checks if network namespace support is available.
// This requires:
//  1. Linux kernel with namespace support (CONFIG_NET_NS)
//  2. Sufficient permissions (CAP_SYS_ADMIN or user namespaces)
func (n *namespaceController) IsAvailable() bool {
	// Check if we're on Linux
	if runtime.GOOS != "linux" {
		return false
	}

	// Check if /proc/self/ns/net exists (indicates namespace support)
	if _, err := os.Stat("/proc/self/ns/net"); os.IsNotExist(err) {
		return false
	}

	// Try to create a network namespace to verify we have permissions
	// We do this in a child process to avoid affecting the parent
	cmd := exec.Command("/bin/sh", "-c", "true")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNET,
	}

	if err := cmd.Run(); err != nil {
		// If we can't create a namespace, this method isn't available
		return false
	}

	return true
}

// ConfigureCommand configures the command to run in an isolated network namespace.
// The namespace will have no network interfaces, providing complete isolation.
func (n *namespaceController) ConfigureCommand(cmd *exec.Cmd) error {
	if cmd == nil {
		return fmt.Errorf("command cannot be nil")
	}

	// Initialize SysProcAttr if not already set
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	// Set the CLONE_NEWNET flag to create a new network namespace
	// This creates an isolated network stack with no network interfaces
	// (not even loopback unless explicitly configured)
	cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWNET

	// Also set unshare flags for additional isolation
	// CLONE_NEWNS creates a new mount namespace (doesn't affect network but good practice)
	cmd.SysProcAttr.Unshareflags |= syscall.CLONE_NEWNET

	// Set environment variable to indicate network is disabled
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "CODEX_SANDBOX_NETWORK_DISABLED=1")

	return nil
}

// Cleanup performs any necessary cleanup.
// Network namespaces are automatically cleaned up when the process exits.
func (n *namespaceController) Cleanup(ctx context.Context) error {
	// Network namespaces are automatically cleaned up by the kernel
	// when the last process in the namespace exits
	return nil
}

func (n *namespaceController) Type() string {
	return "namespace"
}

// CreateIsolatedNetworkNamespace creates a new network namespace with complete isolation.
// This is a lower-level function that can be called from pre-exec hooks if needed.
//
// Returns an error if namespace creation fails.
func CreateIsolatedNetworkNamespace() error {
	// Unshare the network namespace, creating a new isolated network stack
	if err := unix.Unshare(unix.CLONE_NEWNET); err != nil {
		return fmt.Errorf("failed to create network namespace: %w", err)
	}

	// The new namespace starts with no network interfaces
	// We intentionally don't set up loopback or any other interfaces
	// to ensure complete network isolation

	return nil
}

// ValidateNetworkIsolation checks if network access is actually blocked.
// This can be used in tests to verify isolation is working.
func ValidateNetworkIsolation() error {
	// Try to create a socket - this should work
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	if err != nil {
		return fmt.Errorf("socket creation failed: %w", err)
	}
	defer unix.Close(fd)

	// Try to connect to a common external address (1.1.1.1:80)
	// This should fail because there are no network interfaces
	addr := &unix.SockaddrInet4{
		Port: 80,
		Addr: [4]byte{1, 1, 1, 1},
	}

	err = unix.Connect(fd, addr)
	if err == nil {
		return &ValidationError{
			Method:  "namespace",
			Message: "network connection succeeded when it should have failed",
		}
	}

	// Check that the error is due to network being unreachable
	// Common errors: ENETUNREACH (network unreachable), EHOSTUNREACH (host unreachable)
	if err != unix.ENETUNREACH && err != unix.EHOSTUNREACH && err != unix.EINVAL {
		return &ValidationError{
			Method:  "namespace",
			Message: fmt.Sprintf("unexpected error: %v (expected ENETUNREACH or EHOSTUNREACH)", err),
		}
	}

	return nil
}

// GetNetworkInterfaces returns the list of network interfaces in the current namespace.
// This is useful for debugging and validation.
func GetNetworkInterfaces() ([]string, error) {
	// Read /proc/net/dev to get list of interfaces
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc/net/dev: %w", err)
	}

	// Parse the interface list
	// Format: interface: bytes packets...
	// We just want to check if any interfaces exist
	lines := string(data)
	interfaces := []string{}

	// Simple parsing - look for lines with colons (interface definitions)
	for i, line := range []byte(lines) {
		if line == ':' && i > 0 {
			// Found an interface
			// In an isolated namespace, this should only show 'lo' at most
			// and even that won't be UP
			start := i - 1
			for start > 0 && lines[start] != '\n' {
				start--
			}
			if start > 0 {
				start++ // Skip the newline
			}
			iface := lines[start:i]
			// Trim whitespace
			for len(iface) > 0 && (iface[0] == ' ' || iface[0] == '\t') {
				iface = iface[1:]
			}
			if len(iface) > 0 {
				interfaces = append(interfaces, iface)
			}
		}
	}

	return interfaces, nil
}
