//go:build !linux

// Package seccomp provides Linux Seccomp-BPF syscall filtering.
// This file provides stubs for non-Linux platforms.
package seccomp

import (
	"fmt"
	"runtime"
)

// IsSupported always returns false on non-Linux platforms
func IsSupported() bool {
	return false
}

// SetNoNewPrivs is not supported on non-Linux platforms
func SetNoNewPrivs() error {
	return fmt.Errorf("seccomp is not supported on %s", runtime.GOOS)
}

// SeccompFilter is a stub for non-Linux platforms
type SeccompFilter struct{}

// Apply is not supported on non-Linux platforms
func (f *SeccompFilter) Apply() error {
	return fmt.Errorf("seccomp is not supported on %s", runtime.GOOS)
}

// GetMode is not supported on non-Linux platforms
func GetMode() (int, error) {
	return -1, fmt.Errorf("seccomp is not supported on %s", runtime.GOOS)
}

// GetArchitecture returns the current architecture but seccomp is not available
func GetArchitecture() string {
	return runtime.GOARCH
}

// CreateNetworkFilter is not supported on non-Linux platforms
func CreateNetworkFilter(arch string) (*SeccompFilter, error) {
	return nil, fmt.Errorf("seccomp is not supported on %s", runtime.GOOS)
}

// CreateRestrictiveFilter is not supported on non-Linux platforms
func CreateRestrictiveFilter(arch string, allowedSyscalls []int) (*SeccompFilter, error) {
	return nil, fmt.Errorf("seccomp is not supported on %s", runtime.GOOS)
}

// GetSyscallNumbers is not supported on non-Linux platforms
func GetSyscallNumbers(arch string) *SyscallNumbers {
	return nil
}

// GetSafeSyscallList is not supported on non-Linux platforms
func GetSafeSyscallList(arch string) []int {
	return nil
}

// SyscallNumbers is a stub for non-Linux platforms
type SyscallNumbers struct{}
