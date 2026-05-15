//go:build !linux

// Package network provides network namespace stubs for non-Linux platforms.
package network

import (
	"context"
	"os/exec"
)

// namespaceController is not available on non-Linux platforms.
type namespaceController struct{}

func newNamespaceController() *namespaceController {
	return &namespaceController{}
}

func (n *namespaceController) IsAvailable() bool {
	// Network namespaces are Linux-only
	return false
}

func (n *namespaceController) ConfigureCommand(cmd *exec.Cmd) error {
	// Should never be called since IsAvailable returns false
	return nil
}

func (n *namespaceController) Cleanup(ctx context.Context) error {
	return nil
}

func (n *namespaceController) Type() string {
	return "namespace-unavailable"
}
