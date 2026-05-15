//go:build !linux

// Package landlock provides Linux Landlock LSM support.
// This file provides stub implementations for non-Linux systems.
package landlock

import "fmt"

// IsSupported always returns false on non-Linux systems.
func IsSupported() bool {
	return false
}

// GetABIVersion always returns 0 on non-Linux systems.
func GetABIVersion() int {
	return 0
}

// Ruleset represents a Landlock ruleset (stub for non-Linux).
type Ruleset struct{}

// NewRuleset creates a new ruleset stub.
func NewRuleset() *Ruleset {
	return &Ruleset{}
}

// AddRule is a no-op on non-Linux systems.
func (r *Ruleset) AddRule(path string, access uint64) *Ruleset {
	return r
}

// AddReadOnlyPath is a no-op on non-Linux systems.
func (r *Ruleset) AddReadOnlyPath(path string) *Ruleset {
	return r
}

// AddReadWritePath is a no-op on non-Linux systems.
func (r *Ruleset) AddReadWritePath(path string) *Ruleset {
	return r
}

// Apply returns an error on non-Linux systems.
func (r *Ruleset) Apply() error {
	return fmt.Errorf("landlock is only supported on Linux")
}

// TryApply returns nil on non-Linux systems (graceful degradation).
func (r *Ruleset) TryApply() error {
	return nil
}

// Close is a no-op on non-Linux systems.
func (r *Ruleset) Close() error {
	return nil
}

// ApplyDefault returns an error on non-Linux systems.
func ApplyDefault(writableRoots []string) error {
	return fmt.Errorf("landlock is only supported on Linux")
}

// TryApplyDefault returns nil on non-Linux systems.
func TryApplyDefault(writableRoots []string) error {
	return nil
}

// ApplyReadOnly returns an error on non-Linux systems.
func ApplyReadOnly() error {
	return fmt.Errorf("landlock is only supported on Linux")
}

// TryApplyReadOnly returns nil on non-Linux systems.
func TryApplyReadOnly() error {
	return nil
}

// GetKernelVersion returns an error on non-Linux systems.
func GetKernelVersion() (string, error) {
	return "", fmt.Errorf("kernel version only available on Linux")
}

// Info represents Landlock support information.
type Info struct {
	KernelVersion string
	Supported     bool
	ABIVersion    int
}

// GetInfo returns stub info on non-Linux systems.
func GetInfo() (*Info, error) {
	return &Info{
		KernelVersion: "n/a",
		Supported:     false,
		ABIVersion:    0,
	}, nil
}
