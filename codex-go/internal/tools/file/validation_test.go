package file

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePathForRead(t *testing.T) {
	// Create a temporary workspace
	workspace := t.TempDir()

	tests := []struct {
		name      string
		path      string
		expectErr bool
		errType   ValidationErrorType
	}{
		{
			name:      "valid relative path",
			path:      "file.txt",
			expectErr: false,
		},
		{
			name:      "valid nested relative path",
			path:      "dir/subdir/file.txt",
			expectErr: false,
		},
		{
			name:      "valid absolute path within workspace",
			path:      filepath.Join(workspace, "file.txt"),
			expectErr: false,
		},
		{
			name:      "path traversal with ../",
			path:      "../../../etc/passwd",
			expectErr: true,
			errType:   ErrorPathTraversal,
		},
		{
			name:      "path traversal to parent",
			path:      "..",
			expectErr: true,
			errType:   ErrorPathTraversal,
		},
		{
			name:      "path traversal absolute",
			path:      "/etc/passwd",
			expectErr: true,
			errType:   ErrorPathTraversal,
		},
		{
			name:      "complex traversal that stays in workspace",
			path:      "dir/../file.txt",
			expectErr: false, // Resolves to file.txt in workspace
		},
		{
			name:      "escape with complex traversal",
			path:      "dir/../../../etc/passwd",
			expectErr: true,
			errType:   ErrorPathTraversal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePathForRead(tt.path, workspace)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errType != 0 {
					var valErr *ValidationError
					require.ErrorAs(t, err, &valErr)
					assert.Equal(t, tt.errType, valErr.Type)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePathForWrite(t *testing.T) {
	workspace := t.TempDir()

	// Skip sensitive path tests on Windows as the paths are different
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix sensitive path tests on Windows")
	}

	tests := []struct {
		name      string
		path      string
		expectErr bool
		errType   ValidationErrorType
	}{
		{
			name:      "valid write path",
			path:      "output.txt",
			expectErr: false,
		},
		{
			name:      "valid nested write",
			path:      "build/dist/output.js",
			expectErr: false,
		},
		{
			name:      "attempt to write to /etc",
			path:      "/etc/config",
			expectErr: true,
			errType:   ErrorPathTraversal, // First fails traversal check
		},
		{
			name:      "attempt to write to /usr",
			path:      "/usr/local/bin/script",
			expectErr: true,
			errType:   ErrorPathTraversal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePathForWrite(tt.path, workspace)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errType != 0 {
					var valErr *ValidationError
					require.ErrorAs(t, err, &valErr)
					assert.Equal(t, tt.errType, valErr.Type)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsPathInWorkspace(t *testing.T) {
	workspace := "/home/user/project"
	if runtime.GOOS == "windows" {
		workspace = "C:\\Users\\user\\project"
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "path in workspace",
			path:     filepath.Join(workspace, "file.txt"),
			expected: true,
		},
		{
			name:     "nested path in workspace",
			path:     filepath.Join(workspace, "dir", "subdir", "file.txt"),
			expected: true,
		},
		{
			name:     "workspace itself",
			path:     workspace,
			expected: true,
		},
		{
			name:     "path outside workspace",
			path:     "/etc/passwd",
			expected: false,
		},
		{
			name:     "parent of workspace",
			path:     filepath.Dir(workspace),
			expected: false,
		},
		{
			name:     "sibling directory",
			path:     filepath.Join(filepath.Dir(workspace), "other"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPathInWorkspace(tt.path, workspace)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolvePath(t *testing.T) {
	workspace := "/home/user/project"
	if runtime.GOOS == "windows" {
		workspace = "C:\\Users\\user\\project"
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "relative path",
			path:     "file.txt",
			expected: filepath.Join(workspace, "file.txt"),
		},
		{
			name:     "nested relative path",
			path:     "dir/subdir/file.txt",
			expected: filepath.Join(workspace, "dir", "subdir", "file.txt"),
		},
		{
			name:     "absolute path",
			path:     "/tmp/file.txt",
			expected: "/tmp/file.txt",
		},
		{
			name:     "path with dots",
			path:     "dir/../file.txt",
			expected: filepath.Join(workspace, "file.txt"),
		},
		{
			name:     "current directory",
			path:     ".",
			expected: workspace,
		},
		{
			name:     "path with extra separators",
			path:     "dir//subdir///file.txt",
			expected: filepath.Join(workspace, "dir", "subdir", "file.txt"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// On Windows, adjust absolute path test
			if runtime.GOOS == "windows" && tt.path == "/tmp/file.txt" {
				tt.path = "C:\\temp\\file.txt"
				tt.expected = "C:\\temp\\file.txt"
			}

			result, err := ResolvePath(tt.path, workspace)
			require.NoError(t, err)
			assert.Equal(t, filepath.Clean(tt.expected), result)
		})
	}
}

func TestResolvePathErrors(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		workspace string
		errType   ValidationErrorType
	}{
		{
			name:      "relative workspace",
			path:      "file.txt",
			workspace: "relative/path",
			errType:   ErrorAbsolutePathRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolvePath(tt.path, tt.workspace)
			require.Error(t, err)
			var valErr *ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, tt.errType, valErr.Type)
		})
	}
}

func TestSymlinkSafety(t *testing.T) {
	// Create temporary workspace and resolve it (for macOS /var -> /private/var)
	tempWorkspace := t.TempDir()
	workspace, err := filepath.EvalSymlinks(tempWorkspace)
	require.NoError(t, err)

	// Create a file inside workspace
	targetFile := filepath.Join(workspace, "target.txt")
	err = os.WriteFile(targetFile, []byte("test"), 0644)
	require.NoError(t, err)

	// Create a symlink inside workspace pointing to target (safe)
	safeLink := filepath.Join(workspace, "safe_link.txt")
	err = os.Symlink(targetFile, safeLink)
	if err != nil {
		t.Skip("Symlinks not supported on this system")
	}

	// Test safe symlink
	err = ValidatePathForRead(safeLink, workspace)
	assert.NoError(t, err, "safe symlink should be allowed")

	// Create a symlink pointing outside workspace (unsafe)
	unsafeLink := filepath.Join(workspace, "unsafe_link.txt")
	if runtime.GOOS == "windows" {
		err = os.Symlink("C:\\Windows\\System32\\cmd.exe", unsafeLink)
	} else {
		err = os.Symlink("/etc/passwd", unsafeLink)
	}
	require.NoError(t, err)

	// Test unsafe symlink
	err = ValidatePathForRead(unsafeLink, workspace)
	require.Error(t, err, "symlink escaping workspace should be blocked")
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, ErrorSymlinkEscape, valErr.Type)
}

func TestDetectPathTraversal(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "clean path",
			path:     "dir/file.txt",
			expected: false,
		},
		{
			name:     "path with ../",
			path:     "../file.txt",
			expected: true,
		},
		{
			name:     "path with /..",
			path:     "/dir/../file.txt",
			expected: true,
		},
		{
			name:     "path with ..\\ (Windows)",
			path:     "..\\file.txt",
			expected: true,
		},
		{
			name:     "URL encoded traversal",
			path:     "%2e%2e/file.txt",
			expected: true,
		},
		{
			name:     "mixed encoding",
			path:     "..%2ffile.txt",
			expected: true,
		},
		{
			name:     "double encoded",
			path:     "%252e%252e/file.txt",
			expected: false, // Doesn't detect double encoding
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectPathTraversal(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSensitivePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix sensitive path tests on Windows")
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "etc directory",
			path:     "/etc/passwd",
			expected: true,
		},
		{
			name:     "usr directory",
			path:     "/usr/bin/bash",
			expected: true,
		},
		{
			name:     "sys directory",
			path:     "/sys/devices",
			expected: true,
		},
		{
			name:     "proc directory",
			path:     "/proc/cpuinfo",
			expected: true,
		},
		{
			name:     "boot directory",
			path:     "/boot/vmlinuz",
			expected: true,
		},
		{
			name:     "regular home directory file",
			path:     "/home/user/project/file.txt",
			expected: false,
		},
		{
			name:     "tmp directory",
			path:     "/tmp/file.txt",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSensitivePath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSensitiveHomePath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     ".ssh directory",
			path:     filepath.Join(homeDir, ".ssh", "id_rsa"),
			expected: true,
		},
		{
			name:     ".aws directory",
			path:     filepath.Join(homeDir, ".aws", "credentials"),
			expected: true,
		},
		{
			name:     ".gnupg directory",
			path:     filepath.Join(homeDir, ".gnupg", "private-keys"),
			expected: true,
		},
		{
			name:     "regular home file",
			path:     filepath.Join(homeDir, "Documents", "file.txt"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSensitivePath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidatePathComponents(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		expectErr bool
	}{
		{
			name:      "clean path",
			path:      "dir/subdir/file.txt",
			expectErr: false,
		},
		{
			name:      "path with spaces",
			path:      "my folder/my file.txt",
			expectErr: false,
		},
		{
			name:      "path with null byte",
			path:      "file\x00.txt",
			expectErr: true,
		},
		{
			name:      "path with control characters",
			path:      "file\x01.txt",
			expectErr: true,
		},
		{
			name:      "path with tab (allowed)",
			path:      "file\t.txt",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePathComponents(tt.path)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "path with extra separators",
			path:     "dir//subdir///file.txt",
			expected: filepath.Join("dir", "subdir", "file.txt"),
		},
		{
			name:     "path with dots",
			path:     "dir/./subdir/../file.txt",
			expected: filepath.Join("dir", "file.txt"),
		},
		{
			name:     "trailing separator",
			path:     "dir/",
			expected: "dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePath(tt.path)
			expected := tt.expected

			// On case-insensitive systems, expect lowercase
			if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
				expected = strings.ToLower(expected)
			}

			assert.Equal(t, expected, result)
		})
	}
}

func TestIsHiddenPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "hidden file",
			path:     ".hidden",
			expected: true,
		},
		{
			name:     "hidden file in directory",
			path:     "dir/.hidden",
			expected: true,
		},
		{
			name:     "regular file",
			path:     "file.txt",
			expected: false,
		},
		{
			name:     "current directory",
			path:     ".",
			expected: false,
		},
		{
			name:     "parent directory",
			path:     "..",
			expected: false,
		},
		{
			name:     ".git directory",
			path:     ".git",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsHiddenPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPathTraversalAttacks(t *testing.T) {
	workspace := t.TempDir()

	// Common path traversal attack vectors
	attackVectors := []string{
		"../../../etc/passwd",
		"....//....//....//etc/passwd",
		"..%2F..%2F..%2Fetc%2Fpasswd",
		"/%2e%2e/%2e%2e/%2e%2e/etc/passwd",
		"./../.../.././../etc/passwd",
		"dir/../../../../../../etc/passwd",
	}

	// Windows-specific attack (only test on Windows)
	if runtime.GOOS == "windows" {
		attackVectors = append(attackVectors, "..\\..\\..\\windows\\system32\\config\\sam")
	}

	for _, attack := range attackVectors {
		t.Run(attack, func(t *testing.T) {
			err := ValidatePathForRead(attack, workspace)
			assert.Error(t, err, "should block traversal attack: %s", attack)
		})
	}
}

func TestEdgeCases(t *testing.T) {
	workspace := t.TempDir()

	tests := []struct {
		name      string
		path      string
		expectErr bool
	}{
		{
			name:      "empty path",
			path:      "",
			expectErr: false, // Resolves to workspace
		},
		{
			name:      "single dot",
			path:      ".",
			expectErr: false, // Current directory (workspace)
		},
		{
			name:      "double dot alone",
			path:      "..",
			expectErr: true, // Parent directory
		},
		{
			name:      "path with trailing slash",
			path:      "dir/",
			expectErr: false,
		},
		{
			name:      "path with multiple slashes",
			path:      "dir///subdir",
			expectErr: false,
		},
		{
			name:      "Windows-style separators on Unix",
			path:      "dir\\file.txt",
			expectErr: false, // Treated as regular filename on Unix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePathForRead(tt.path, workspace)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestEnhancedPathTraversal tests the new enhanced path traversal detection
func TestEnhancedPathTraversal(t *testing.T) {
	workspace := t.TempDir()

	tests := []struct {
		name      string
		path      string
		expectErr bool
		errType   ValidationErrorType
	}{
		{
			name:      "double encoded dots",
			path:      "%252e%252e/file.txt",
			expectErr: true,
			errType:   ErrorPathTraversal,
		},
		{
			name:      "double encoded slash",
			path:      "..%252ffile.txt",
			expectErr: true,
			errType:   ErrorPathTraversal,
		},
		{
			name:      "triple dots",
			path:      ".../file.txt",
			expectErr: true,
			errType:   ErrorPathTraversal,
		},
		{
			name:      "many dots",
			path:      "......./file.txt",
			expectErr: true,
			errType:   ErrorPathTraversal,
		},
		{
			name:      "URL encoded then path traversal",
			path:      "%2e%2e%2f%2e%2e%2ffile.txt",
			expectErr: true,
			errType:   ErrorPathTraversal,
		},
		{
			name:      "mixed encoding backslash",
			path:      "..%5cfile.txt",
			expectErr: true,
			errType:   ErrorPathTraversal,
		},
		{
			name:      "invalid UTF-8",
			path:      "file\xFF\xFE.txt",
			expectErr: true,
			errType:   ErrorInvalidPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePathForRead(tt.path, workspace)
			if tt.expectErr {
				require.Error(t, err)
				var valErr *ValidationError
				require.ErrorAs(t, err, &valErr)
				assert.Equal(t, tt.errType, valErr.Type)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestSensitiveFilePatterns tests sensitive file pattern matching
func TestSensitiveFilePatterns(t *testing.T) {
	workspace := t.TempDir()

	sensitiveFiles := []string{
		".env",
		".env.local",
		".env.production",
		"credentials.json",
		"password.txt",
		"secret.key",
		"private_key",
		"private-key.pem",
		"id_rsa",
		"id_ed25519",
		"cert.pem",
		"server.key",
		"keystore.p12",
		"app.keystore",
		"store.jks",
		"api_token.txt",
		"auth_config.json",
		".npmrc",
		".pypirc",
		".netrc",
		".git-credentials",
		".dockercfg",
		"master.key",
		".htpasswd",
		"web.config",
		"appsettings.json",
		".pgpass",
		".my.cnf",
		".s3cfg",
		".boto",
		"secrets.tfvars",
		"connection.ovpn",
		"server.rdp",
	}

	for _, filename := range sensitiveFiles {
		t.Run(filename, func(t *testing.T) {
			err := ValidatePathForWrite(filename, workspace)
			require.Error(t, err, "should block sensitive file: %s", filename)
			var valErr *ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, ErrorSensitivePath, valErr.Type)
		})
	}
}

// TestSpecialFileTypes tests rejection of special file types
func TestSpecialFileTypes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Special file type tests are Unix-specific")
	}

	workspace := t.TempDir()

	// Test device file (if accessible)
	t.Run("device file", func(t *testing.T) {
		// /dev/null should be accessible on Unix systems
		err := ValidatePathForRead("/dev/null", workspace)
		// Should fail either due to workspace check or special file check
		assert.Error(t, err)
	})

	// Create a named pipe for testing
	t.Run("named pipe", func(t *testing.T) {
		pipePath := filepath.Join(workspace, "testpipe")
		err := syscall.Mkfifo(pipePath, 0666)
		if err != nil {
			t.Skip("Cannot create named pipe")
		}
		defer os.Remove(pipePath)

		err = ValidatePathForRead(pipePath, workspace)
		require.Error(t, err)
		var valErr *ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, ErrorSpecialFile, valErr.Type)
	})
}

// TestSymlinkChainDepth tests symlink chain depth limits
func TestSymlinkChainDepth(t *testing.T) {
	workspace := t.TempDir()

	// Create a deep chain of symlinks within the workspace
	// Start with a target file
	targetPath := filepath.Join(workspace, "target.txt")
	err := os.WriteFile(targetPath, []byte("test"), 0644)
	require.NoError(t, err)

	// Create a chain where each symlink points to the next one
	// link0 -> target.txt, link1 -> link0, link2 -> link1, etc.
	prevPath := targetPath
	var finalLinkPath string
	for i := 0; i < 45; i++ {
		linkPath := filepath.Join(workspace, fmt.Sprintf("link%d", i))
		// Use absolute path to ensure it stays in workspace
		err := os.Symlink(prevPath, linkPath)
		if err != nil {
			t.Skip("Symlinks not supported")
		}
		prevPath = linkPath
		finalLinkPath = linkPath
	}

	// Validation should fail due to depth exceeding 40
	err = ValidatePathForRead(finalLinkPath, workspace)
	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	// Should be either ErrorSymlinkChain or ErrorSymlinkEscape
	assert.True(t, valErr.Type == ErrorSymlinkChain || valErr.Type == ErrorSymlinkEscape,
		"Expected ErrorSymlinkChain or ErrorSymlinkEscape, got %v", valErr.Type)
}

// TestPathDepthLimit tests directory nesting depth limits
func TestPathDepthLimit(t *testing.T) {
	workspace := t.TempDir()

	// Create a very deep path (exceeds limit)
	deepPath := workspace
	for i := 0; i < 120; i++ {
		deepPath = filepath.Join(deepPath, "dir")
	}

	err := ValidatePathForRead(deepPath, workspace)
	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, ErrorPathTooDeep, valErr.Type)
}

// TestPathLengthLimit tests maximum path length enforcement
func TestPathLengthLimit(t *testing.T) {
	workspace := t.TempDir()

	// Create a path that exceeds 4096 bytes
	longPath := strings.Repeat("a", 5000)

	err := ValidatePathForRead(longPath, workspace)
	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, ErrorInvalidPath, valErr.Type)
	assert.Contains(t, valErr.Message, "exceeds maximum length")
}

// TestUnicodeAttacks tests Unicode-based path attacks
func TestUnicodeAttacks(t *testing.T) {
	workspace := t.TempDir()

	tests := []struct {
		name      string
		path      string
		expectErr bool
	}{
		{
			name:      "right-to-left override",
			path:      "file\u202Etxt.exe", // Looks like file.txt but actually file.exe
			expectErr: true,
		},
		{
			name:      "zero-width space",
			path:      "file\u200Bname.txt",
			expectErr: true,
		},
		{
			name:      "zero-width non-joiner",
			path:      "file\u200Cname.txt",
			expectErr: true,
		},
		{
			name:      "zero-width joiner",
			path:      "file\u200Dname.txt",
			expectErr: true,
		},
		{
			name:      "byte order mark",
			path:      "\uFEFFfilename.txt",
			expectErr: true,
		},
		{
			name:      "left-to-right override",
			path:      "file\u202Dname.txt",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePathForRead(tt.path, workspace)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestWindowsReservedNames tests Windows reserved filename detection
func TestWindowsReservedNames(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	workspace := t.TempDir()

	reservedNames := []string{
		"CON", "con", "Con.txt",
		"PRN", "prn", "PRN.log",
		"AUX", "aux", "AUX.dat",
		"NUL", "nul", "NUL.txt",
		"COM1", "com1", "COM1.txt",
		"LPT1", "lpt1", "LPT1.doc",
	}

	for _, name := range reservedNames {
		t.Run(name, func(t *testing.T) {
			err := ValidatePathForRead(name, workspace)
			require.Error(t, err)
			var valErr *ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, ErrorInvalidPath, valErr.Type)
		})
	}
}

// TestControlCharacters tests control character detection
func TestControlCharacters(t *testing.T) {
	workspace := t.TempDir()

	tests := []struct {
		name      string
		path      string
		expectErr bool
	}{
		{
			name:      "null byte",
			path:      "file\x00name.txt",
			expectErr: true,
		},
		{
			name:      "bell character",
			path:      "file\x07.txt",
			expectErr: true,
		},
		{
			name:      "escape character",
			path:      "file\x1B.txt",
			expectErr: true,
		},
		{
			name:      "C1 control character",
			path:      "file\x81.txt",
			expectErr: true,
		},
		{
			name:      "tab is allowed",
			path:      "file\tname.txt",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePathForRead(tt.path, workspace)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestSensitivePathCaseInsensitive tests case-insensitive path matching
func TestSensitivePathCaseInsensitive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix sensitive path tests on Windows")
	}

	// Test that variations like /ETC are blocked on case-insensitive systems
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "/etc lowercase",
			path:     "/etc/passwd",
			expected: true,
		},
		{
			name:     "/etc mixed case",
			path:     "/Etc/passwd",
			expected: true,
		},
		{
			name:     "/etc uppercase",
			path:     "/ETC/PASSWD",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSensitivePath(tt.path)
			if runtime.GOOS == "darwin" {
				// macOS is case-insensitive, should block all variations
				assert.Equal(t, tt.expected, result)
			} else {
				// Linux is case-sensitive, only lowercase should match
				if tt.path == "/etc/passwd" {
					assert.True(t, result)
				}
			}
		})
	}
}

// TestSeparatorBoundaryMatching tests proper separator-aware prefix matching
func TestSeparatorBoundaryMatching(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix path tests on Windows")
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "/etc itself",
			path:     "/etc",
			expected: true,
		},
		{
			name:     "/etc/file",
			path:     "/etc/passwd",
			expected: true,
		},
		{
			name:     "/etc-backup should NOT match",
			path:     "/etc-backup/file.txt",
			expected: false,
		},
		{
			name:     "/etcd should NOT match",
			path:     "/etcd/config.json",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSensitivePath(tt.path)
			assert.Equal(t, tt.expected, result, "Path: %s", tt.path)
		})
	}
}

// TestSymlinkRelativeTarget tests relative symlink target resolution
func TestSymlinkRelativeTarget(t *testing.T) {
	// Get the real workspace path (resolves /var -> /private/var on macOS)
	tempWorkspace := t.TempDir()
	workspace, err := filepath.EvalSymlinks(tempWorkspace)
	require.NoError(t, err)

	// Create nested directory structure
	subdir := filepath.Join(workspace, "subdir")
	err = os.Mkdir(subdir, 0755)
	require.NoError(t, err)

	targetFile := filepath.Join(subdir, "target.txt")
	err = os.WriteFile(targetFile, []byte("test"), 0644)
	require.NoError(t, err)

	// Create relative symlink in subdir pointing to target in same dir
	linkPath := filepath.Join(subdir, "link.txt")
	err = os.Symlink("target.txt", linkPath)
	if err != nil {
		t.Skip("Symlinks not supported")
	}

	// Should be allowed (relative symlink stays in workspace)
	err = ValidatePathForRead(linkPath, workspace)
	assert.NoError(t, err)

	// Create relative symlink that escapes using ../
	escapeLinkPath := filepath.Join(subdir, "escape.txt")
	err = os.Symlink("../../../etc/passwd", escapeLinkPath)
	require.NoError(t, err)

	// Should be blocked (escapes workspace)
	err = ValidatePathForRead(escapeLinkPath, workspace)
	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, ErrorSymlinkEscape, valErr.Type)
}

// TestComponentLengthLimit tests individual component length limits
func TestComponentLengthLimit(t *testing.T) {
	workspace := t.TempDir()

	// Create a component longer than 255 bytes
	longComponent := strings.Repeat("a", 300)
	path := filepath.Join("dir", longComponent, "file.txt")

	err := ValidatePathForRead(path, workspace)
	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, ErrorInvalidPath, valErr.Type)
	assert.Contains(t, valErr.Message, "exceeds 255 bytes")
}
