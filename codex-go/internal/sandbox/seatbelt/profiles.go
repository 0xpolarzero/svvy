//go:build darwin

package seatbelt

import "os"

// Predefined sandbox profiles for common use cases.

// ReadOnlyProfile generates a profile that allows read access everywhere but denies all writes.
// This is suitable for operations that only need to inspect the filesystem.
func ReadOnlyProfile() string {
	config := &ProfileConfig{
		AllowFileRead:        true,
		AllowFileWrite:       false,
		WritableRoots:        []WritableRoot{},
		AllowNetworkOutbound: false,
		AllowNetworkInbound:  false,
		AllowSystemSocket:    false,
	}
	return GenerateProfile(config)
}

// WorkspaceWriteProfile generates a profile that allows read everywhere and writes
// only in the specified workspace directory and temporary directories.
// The .git directory within the workspace is automatically marked as read-only.
//
// Parameters:
//   - workspacePath: The workspace directory where writes are allowed
//   - networkAccess: Whether to allow network access
//   - excludeTmpDir: Whether to exclude TMPDIR from writable roots
//   - excludeSlashTmp: Whether to exclude /tmp from writable roots
func WorkspaceWriteProfile(workspacePath string, networkAccess bool, excludeTmpDir bool, excludeSlashTmp bool) string {
	config := &ProfileConfig{
		AllowFileRead:  true,
		AllowFileWrite: true,
		WritableRoots:  []WritableRoot{},
	}

	// Add workspace as writable root with .git protection
	if workspacePath != "" {
		config.WritableRoots = append(config.WritableRoots, WritableRoot{
			Root:             workspacePath,
			ReadOnlySubpaths: getGitDirIfExists(workspacePath),
		})
	}

	// Add /tmp unless excluded
	if !excludeSlashTmp {
		tmpPath := "/tmp"
		// Resolve symlinks (on macOS, /tmp -> /private/tmp)
		if resolved, err := os.Readlink(tmpPath); err == nil {
			tmpPath = resolved
		}
		config.WritableRoots = append(config.WritableRoots, WritableRoot{
			Root:             tmpPath,
			ReadOnlySubpaths: []string{},
		})
	}

	// Add TMPDIR unless excluded
	if !excludeTmpDir {
		if tmpDir := os.Getenv("TMPDIR"); tmpDir != "" {
			config.WritableRoots = append(config.WritableRoots, WritableRoot{
				Root:             tmpDir,
				ReadOnlySubpaths: []string{},
			})
		}
	}

	// Configure network access
	if networkAccess {
		config.AllowNetworkOutbound = true
		config.AllowNetworkInbound = true
		config.AllowSystemSocket = true
	}

	return GenerateProfile(config)
}

// WorkspaceWriteProfileMultiRoot generates a profile for multiple writable roots.
// Each root is checked for .git directories which are automatically protected.
//
// Parameters:
//   - roots: The directory paths where writes are allowed
//   - networkAccess: Whether to allow network access
//   - excludeTmpDir: Whether to exclude TMPDIR from writable roots
//   - excludeSlashTmp: Whether to exclude /tmp from writable roots
func WorkspaceWriteProfileMultiRoot(roots []string, networkAccess bool, excludeTmpDir bool, excludeSlashTmp bool) string {
	config := &ProfileConfig{
		AllowFileRead:  true,
		AllowFileWrite: true,
		WritableRoots:  []WritableRoot{},
	}

	// Add each root with .git protection
	for _, root := range roots {
		if root != "" {
			config.WritableRoots = append(config.WritableRoots, WritableRoot{
				Root:             root,
				ReadOnlySubpaths: getGitDirIfExists(root),
			})
		}
	}

	// Add /tmp unless excluded
	if !excludeSlashTmp {
		tmpPath := "/tmp"
		// Resolve symlinks (on macOS, /tmp -> /private/tmp)
		if resolved, err := os.Readlink(tmpPath); err == nil {
			tmpPath = resolved
		}
		config.WritableRoots = append(config.WritableRoots, WritableRoot{
			Root:             tmpPath,
			ReadOnlySubpaths: []string{},
		})
	}

	// Add TMPDIR unless excluded
	if !excludeTmpDir {
		if tmpDir := os.Getenv("TMPDIR"); tmpDir != "" {
			config.WritableRoots = append(config.WritableRoots, WritableRoot{
				Root:             tmpDir,
				ReadOnlySubpaths: []string{},
			})
		}
	}

	// Configure network access
	if networkAccess {
		config.AllowNetworkOutbound = true
		config.AllowNetworkInbound = true
		config.AllowSystemSocket = true
	}

	return GenerateProfile(config)
}

// DangerFullAccessProfile generates a profile with no restrictions.
// This allows all file operations, network access, and system calls.
// Use this only in trusted environments like CI or Docker containers.
func DangerFullAccessProfile() string {
	config := &ProfileConfig{
		AllowFileRead:        true,
		AllowFileWrite:       true,
		WritableRoots:        []WritableRoot{{Root: "/"}},
		AllowNetworkOutbound: true,
		AllowNetworkInbound:  true,
		AllowSystemSocket:    true,
	}
	return GenerateProfile(config)
}

// getGitDirIfExists returns a slice containing the .git path if it exists in root.
// This is used to automatically protect .git directories from writes.
func getGitDirIfExists(root string) []string {
	gitPath := root + "/.git"
	if stat, err := os.Stat(gitPath); err == nil && stat.IsDir() {
		return []string{gitPath}
	}
	return []string{}
}
