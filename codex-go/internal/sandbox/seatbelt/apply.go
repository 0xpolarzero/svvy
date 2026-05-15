//go:build darwin

package seatbelt

/*
#cgo LDFLAGS: -lSystem
#include <stdlib.h>

// sandbox_init is the macOS system call to apply a sandbox profile to the current process.
// It takes a profile string, flags, and an error pointer.
// Returns 0 on success, -1 on failure.
//
// From the macOS SDK:
// int sandbox_init(const char *profile, uint64_t flags, char **errorbuf);
//
// The errorbuf will be allocated by the system if there's an error and must be freed
// with sandbox_free_error().
extern int sandbox_init(const char *profile, uint64_t flags, char **errorbuf);

// sandbox_free_error frees the error buffer allocated by sandbox_init.
extern void sandbox_free_error(char *errorbuf);
*/
import "C"

import (
	"fmt"
	"unsafe"
)

const (
	// SANDBOX_NAMED is a flag for using a named profile (not used in this implementation)
	SANDBOX_NAMED = 0x0001
)

// ApplyProfile applies a Seatbelt sandbox profile to the current process.
// Once applied, the process and all its children will be restricted by the profile.
//
// WARNING: This operation is irreversible for the current process. The sandbox
// restrictions will apply to all subsequent operations in this process.
//
// Parameters:
//   - profile: The Seatbelt profile string (Sandbox Profile Language)
//
// Returns:
//   - error: nil on success, error describing the failure otherwise
func ApplyProfile(profile string) error {
	// Convert Go string to C string
	cProfile := C.CString(profile)
	defer C.free(unsafe.Pointer(cProfile))

	// Error buffer pointer
	var errorBuf *C.char

	// Call sandbox_init
	// flags = 0 means we're using a profile string (not a named profile)
	ret := C.sandbox_init(cProfile, 0, &errorBuf)

	// Check for error
	if ret != 0 {
		// Convert error buffer to Go string
		var errMsg string
		if errorBuf != nil {
			errMsg = C.GoString(errorBuf)
			// Free the error buffer
			C.sandbox_free_error(errorBuf)
		} else {
			errMsg = "unknown error"
		}
		return fmt.Errorf("sandbox_init failed: %s", errMsg)
	}

	return nil
}

// ApplyReadOnlyProfile applies a read-only sandbox profile to the current process.
// This is a convenience function that generates and applies a ReadOnlyProfile.
func ApplyReadOnlyProfile() error {
	profile := ReadOnlyProfile()
	return ApplyProfile(profile)
}

// ApplyWorkspaceWriteProfile applies a workspace-write sandbox profile to the current process.
// This is a convenience function that generates and applies a WorkspaceWriteProfile.
func ApplyWorkspaceWriteProfile(workspacePath string, networkAccess bool, excludeTmpDir bool, excludeSlashTmp bool) error {
	profile := WorkspaceWriteProfile(workspacePath, networkAccess, excludeTmpDir, excludeSlashTmp)
	return ApplyProfile(profile)
}

// ApplyDangerFullAccessProfile applies a full-access sandbox profile to the current process.
// This is a convenience function that generates and applies a DangerFullAccessProfile.
// Note: Even with "full access", the sandbox is still applied, just with no restrictions.
func ApplyDangerFullAccessProfile() error {
	profile := DangerFullAccessProfile()
	return ApplyProfile(profile)
}

// IsSupported returns true if Seatbelt sandboxing is supported on this system.
// Seatbelt is only available on macOS (darwin).
func IsSupported() bool {
	// This function will only compile on darwin due to the build tag
	return true
}
