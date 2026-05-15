//go:build !darwin

package seatbelt

import "fmt"

// ApplyProfile is not supported on non-darwin platforms.
func ApplyProfile(profile string) error {
	return fmt.Errorf("seatbelt sandboxing is only available on macOS")
}

// ApplyReadOnlyProfile is not supported on non-darwin platforms.
func ApplyReadOnlyProfile() error {
	return fmt.Errorf("seatbelt sandboxing is only available on macOS")
}

// ApplyWorkspaceWriteProfile is not supported on non-darwin platforms.
func ApplyWorkspaceWriteProfile(workspacePath string, networkAccess bool, excludeTmpDir bool, excludeSlashTmp bool) error {
	return fmt.Errorf("seatbelt sandboxing is only available on macOS")
}

// ApplyDangerFullAccessProfile is not supported on non-darwin platforms.
func ApplyDangerFullAccessProfile() error {
	return fmt.Errorf("seatbelt sandboxing is only available on macOS")
}

// IsSupported returns false on non-darwin platforms.
func IsSupported() bool {
	return false
}
