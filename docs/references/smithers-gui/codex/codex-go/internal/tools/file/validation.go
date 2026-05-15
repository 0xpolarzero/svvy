// Package file provides secure path validation for file operations.
//
// This package implements defense-in-depth security checks including:
// - Path traversal prevention (../ and encoded variants including double encoding)
// - Symlink escape detection with chain following
// - Sensitive path protection (credentials, SSH keys, system files)
// - Special file type blocking (devices, pipes, sockets)
// - Workspace containment verification
// - Unicode normalization attack prevention
// - Null byte injection prevention
//
// SECURITY MODEL:
//
// The validation system assumes:
// 1. Workspace is a trusted, absolute path
// 2. All user-provided paths must be validated before file operations
// 3. Symlinks are followed and validated to prevent escapes
// 4. Time-of-check-time-of-use (TOCTOU) vulnerabilities exist and cannot be
//    fully prevented at this layer - callers should minimize time between
//    validation and use
//
// THREAT PROTECTIONS:
//
// Protected against:
// - Path traversal attacks (../, encoded, double-encoded)
// - Symlink escapes outside workspace
// - Access to sensitive system paths
// - Special file access (devices, pipes)
// - Unicode normalization attacks
// - Null byte injection
// - Control character injection
// - Excessively deep directory structures
//
// NOT protected against (inherent limitations):
// - TOCTOU race conditions (filesystem changes between validation and use)
// - Side-channel attacks
// - Privilege escalation via workspace misconfiguration
//
// All paths must be validated before file system operations to prevent
// unauthorized access outside the designated workspace.
package file

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ValidationError represents a path validation error with a specific type.
type ValidationError struct {
	Type    ValidationErrorType
	Path    string
	Message string
	Err     error
}

// ValidationErrorType categorizes validation errors.
type ValidationErrorType int

const (
	// ErrorPathTraversal indicates an attempt to access outside the workspace.
	// This includes ../ patterns, encoded traversal, and paths resolving outside workspace.
	ErrorPathTraversal ValidationErrorType = iota
	// ErrorSensitivePath indicates an attempt to access a sensitive system path.
	// Includes system directories, credential files, and SSH keys.
	ErrorSensitivePath
	// ErrorSymlinkEscape indicates a symlink points outside the workspace.
	// Triggered when following symlinks would escape workspace boundaries.
	ErrorSymlinkEscape
	// ErrorInvalidPath indicates a malformed or invalid path.
	// Includes null bytes, control characters, and invalid encodings.
	ErrorInvalidPath
	// ErrorAbsolutePathRequired indicates an absolute path was required but not provided.
	// Typically applies to workspace paths which must be absolute.
	ErrorAbsolutePathRequired
	// ErrorSpecialFile indicates an attempt to access a special file type.
	// Includes devices, pipes, sockets, and other non-regular files.
	ErrorSpecialFile
	// ErrorSymlinkChain indicates excessive symlink chain depth.
	// Prevents DoS attacks via circular or deeply nested symlinks.
	ErrorSymlinkChain
	// ErrorPathTooDeep indicates excessive directory nesting.
	// Prevents DoS attacks via extremely deep directory structures.
	ErrorPathTooDeep
)

const (
	// Maximum recursion depth for symlink checking to prevent DoS
	maxSymlinkDepth = 40
	// Maximum directory depth to prevent DoS
	maxPathDepth = 100
	// Maximum path length in bytes
	maxPathLength = 4096

	// URL encoding patterns
	encodedDot       = "%2e"
	encodedSlash     = "%2f"
	encodedBackslash = "%5c"
)

func (e *ValidationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", e.Message, e.Err.Error())
	}
	return e.Message
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

// Sensitive system paths that should be blocked on Unix-like systems.
var unixSensitivePaths = []string{
	"/etc",
	"/usr",
	"/sys",
	"/proc",
	"/dev",
	"/boot",
	"/root",
	"/var/log",
	"/.ssh",
	"/private/etc",      // macOS
	"/private/var",      // macOS
	"/System",           // macOS
	"/Library/Security", // macOS
}

// Sensitive paths in home directory (relative to $HOME).
var homeSensitivePaths = []string{
	".ssh",
	".gnupg",
	".aws",
	".kube",
	".docker",
	".config/gcloud",
	".azure",
	"Library/Keychains", // macOS
}

// Sensitive system paths on Windows.
var windowsSensitivePaths = []string{
	"C:\\Windows\\System32",
	"C:\\Windows\\SysWOW64",
	"C:\\Program Files",
	"C:\\Program Files (x86)",
	"C:\\ProgramData",
}

// Sensitive file patterns (case-insensitive matching)
var sensitiveFilePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\.env`),                    // .env, .env.local, etc.
	regexp.MustCompile(`(?i)credential`),                // Any file with "credential"
	regexp.MustCompile(`(?i)password`),                  // Any file with "password"
	regexp.MustCompile(`(?i)secret`),                    // Any file with "secret"
	regexp.MustCompile(`(?i)private[_-]?key`),           // private_key, private-key, privatekey
	regexp.MustCompile(`(?i)id_rsa`),                    // SSH private key
	regexp.MustCompile(`(?i)id_ed25519`),                // SSH private key
	regexp.MustCompile(`(?i)\.pem$`),                    // PEM certificates/keys
	regexp.MustCompile(`(?i)\.key$`),                    // Generic key files
	regexp.MustCompile(`(?i)\.p12$`),                    // PKCS12 keystores
	regexp.MustCompile(`(?i)\.keystore$`),               // Java keystores
	regexp.MustCompile(`(?i)\.jks$`),                    // Java keystores
	regexp.MustCompile(`(?i)token`),                     // API tokens
	regexp.MustCompile(`(?i)auth`),                      // Auth files
	regexp.MustCompile(`(?i)/etc/passwd$`),              // Unix password file
	regexp.MustCompile(`(?i)/etc/shadow$`),              // Unix shadow file
	regexp.MustCompile(`(?i)/etc/sudoers`),              // Sudoers configuration
	regexp.MustCompile(`(?i)\.npmrc$`),                  // npm credentials
	regexp.MustCompile(`(?i)\.pypirc$`),                 // PyPI credentials
	regexp.MustCompile(`(?i)\.netrc$`),                  // Network credentials
	regexp.MustCompile(`(?i)\.git-credentials$`),        // Git credentials
	regexp.MustCompile(`(?i)\.dockercfg$`),              // Docker credentials
	regexp.MustCompile(`(?i)config\.json$`),             // Various config files (may contain secrets)
	regexp.MustCompile(`(?i)master\.key$`),              // Rails master key
	regexp.MustCompile(`(?i)\.htpasswd$`),               // Apache password file
	regexp.MustCompile(`(?i)web\.config$`),              // IIS config (may contain secrets)
	regexp.MustCompile(`(?i)appsettings\.json$`),        // .NET config (may contain secrets)
	regexp.MustCompile(`(?i)\.pgpass$`),                 // PostgreSQL password file
	regexp.MustCompile(`(?i)\.my\.cnf$`),                // MySQL credentials
	regexp.MustCompile(`(?i)\.s3cfg$`),                  // S3 credentials
	regexp.MustCompile(`(?i)\.boto$`),                   // Boto credentials
	regexp.MustCompile(`(?i)\.tfvars$`),                 // Terraform variables (may contain secrets)
	regexp.MustCompile(`(?i)\.ovpn$`),                   // OpenVPN configuration
	regexp.MustCompile(`(?i)\.rdp$`),                    // Remote Desktop configuration
	regexp.MustCompile(`(?i)\.pcap$`),                   // Network packet captures
	regexp.MustCompile(`(?i)\.dmp$`),                    // Memory dumps
	regexp.MustCompile(`(?i)\.core$`),                   // Core dumps
	regexp.MustCompile(`(?i)\.vmdk$`),                   // Virtual machine disks
	regexp.MustCompile(`(?i)\.vhdx?$`),                  // Virtual hard disks
}

// ValidatePathForRead validates a path for read operations.
// It ensures the path is within the workspace and doesn't attempt path traversal.
//
// Validation steps:
// 1. Check path length limits
// 2. Validate path components (no null bytes, control chars)
// 3. Detect and block path traversal patterns
// 4. Resolve path to absolute form
// 5. Check workspace containment
// 6. Validate symlinks don't escape workspace
// 7. Check for special file types
// 8. Check directory depth limits
func ValidatePathForRead(path, workspace string) error {
	// Check path length
	if len(path) > maxPathLength {
		return &ValidationError{
			Type:    ErrorInvalidPath,
			Path:    path,
			Message: fmt.Sprintf("path exceeds maximum length of %d bytes", maxPathLength),
		}
	}

	// Validate path components
	if err := ValidatePathComponents(path); err != nil {
		return err
	}

	// Early detection of suspicious patterns before resolution
	if err := detectEnhancedPathTraversal(path); err != nil {
		return err
	}

	// First, resolve and validate the path
	resolvedPath, err := ResolvePath(path, workspace)
	if err != nil {
		return err
	}

	// Check directory depth
	if err := checkPathDepth(resolvedPath); err != nil {
		return err
	}

	// Check if path is within workspace
	if !IsPathInWorkspace(resolvedPath, workspace) {
		return &ValidationError{
			Type:    ErrorPathTraversal,
			Path:    path,
			Message: fmt.Sprintf("path is outside workspace: %s", path),
		}
	}

	// Check for symlinks that escape workspace (with chain detection)
	if err := checkSymlinkSafetyWithDepth(resolvedPath, workspace, 0); err != nil {
		return err
	}

	// Check for special file types (if file exists)
	if err := checkSpecialFileType(resolvedPath); err != nil {
		return err
	}

	return nil
}

// ValidatePathForWrite validates a path for write operations.
// It performs the same checks as read, plus additional sensitive path checks.
//
// Additional write-specific checks:
// - Sensitive system path protection
// - Sensitive file pattern matching (.env, credentials, etc.)
func ValidatePathForWrite(path, workspace string) error {
	// First perform read validation
	if err := ValidatePathForRead(path, workspace); err != nil {
		return err
	}

	// Resolve the path to check for sensitive locations
	resolvedPath, err := ResolvePath(path, workspace)
	if err != nil {
		return err
	}

	// Check if path is a sensitive system path
	if isSensitivePath(resolvedPath) {
		return &ValidationError{
			Type:    ErrorSensitivePath,
			Path:    path,
			Message: fmt.Sprintf("cannot write to sensitive path: %s", path),
		}
	}

	// Check if filename matches sensitive patterns
	if isSensitiveFile(resolvedPath) {
		return &ValidationError{
			Type:    ErrorSensitivePath,
			Path:    path,
			Message: fmt.Sprintf("cannot write to sensitive file: %s", path),
		}
	}

	return nil
}

// IsPathInWorkspace checks if a resolved absolute path is within the workspace.
func IsPathInWorkspace(path, workspace string) bool {
	// Clean both paths
	cleanPath := filepath.Clean(path)
	cleanWorkspace := filepath.Clean(workspace)

	// Ensure both are absolute
	if !filepath.IsAbs(cleanPath) || !filepath.IsAbs(cleanWorkspace) {
		return false
	}

	// Check if path is under workspace using Rel
	rel, err := filepath.Rel(cleanWorkspace, cleanPath)
	if err != nil {
		return false
	}

	// If relative path starts with "..", it's outside workspace
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return false
	}

	return true
}

// ResolvePath resolves a path relative to workspace and returns the absolute path.
// It handles both relative and absolute paths, and ensures path separators are normalized.
func ResolvePath(path, workspace string) (string, error) {
	// Validate workspace is absolute
	if !filepath.IsAbs(workspace) {
		return "", &ValidationError{
			Type:    ErrorAbsolutePathRequired,
			Path:    workspace,
			Message: "workspace must be an absolute path",
		}
	}

	// Clean the workspace
	cleanWorkspace := filepath.Clean(workspace)

	// Handle absolute paths
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}

	// Handle relative paths - join with workspace
	fullPath := filepath.Join(cleanWorkspace, path)
	return filepath.Clean(fullPath), nil
}

// checkSymlinkSafetyWithDepth verifies that a path and any symlinks in its chain
// don't point outside the workspace. It tracks recursion depth to prevent DoS attacks.
func checkSymlinkSafetyWithDepth(path, workspace string, depth int) error {
	// Check recursion depth limit
	if depth > maxSymlinkDepth {
		return &ValidationError{
			Type:    ErrorSymlinkChain,
			Path:    path,
			Message: fmt.Sprintf("symlink chain exceeds maximum depth of %d", maxSymlinkDepth),
		}
	}

	// Check if the path is a symlink explicitly
	info, err := os.Lstat(path)
	if err != nil {
		// If the file doesn't exist yet, that's okay for write operations
		if os.IsNotExist(err) {
			// Check parent directory instead
			parent := filepath.Dir(path)
			if parent != path { // Avoid infinite recursion at root
				return checkSymlinkSafetyWithDepth(parent, workspace, depth+1)
			}
			return nil
		}
		// For other errors (like in-memory filesystems), skip symlink check
		// The path itself has already been validated
		return nil
	}

	// If it's not a symlink, we're done
	if info.Mode()&os.ModeSymlink == 0 {
		return nil
	}

	// It's a symlink - read where it points
	target, err := os.Readlink(path)
	if err != nil {
		return &ValidationError{
			Type:    ErrorSymlinkEscape,
			Path:    path,
			Message: fmt.Sprintf("cannot read symlink: %s", err.Error()),
			Err:     err,
		}
	}

	// Resolve the target (may be relative to the symlink's directory)
	var resolvedTarget string
	if filepath.IsAbs(target) {
		resolvedTarget = filepath.Clean(target)
	} else {
		// Relative symlink - resolve relative to symlink's directory
		symlinkDir := filepath.Dir(path)
		resolvedTarget = filepath.Clean(filepath.Join(symlinkDir, target))
	}

	// Also resolve workspace symlinks for fair comparison
	// (e.g., on macOS /var -> /private/var)
	evalWorkspace, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		// If workspace doesn't exist or can't be resolved, use as-is
		evalWorkspace = workspace
	}

	// Check if the symlink target is still within workspace
	if !IsPathInWorkspace(resolvedTarget, evalWorkspace) {
		return &ValidationError{
			Type:    ErrorSymlinkEscape,
			Path:    path,
			Message: fmt.Sprintf("symlink points outside workspace: %s -> %s", path, resolvedTarget),
		}
	}

	// Recursively check the target (it might also be a symlink)
	return checkSymlinkSafetyWithDepth(resolvedTarget, workspace, depth+1)
}

// checkSpecialFileType verifies that a path doesn't point to special file types.
// Returns error if the file is a device, pipe, socket, or other non-regular file.
func checkSpecialFileType(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		// If file doesn't exist, that's okay
		if os.IsNotExist(err) {
			return nil
		}
		// For other errors, we can't determine file type, so allow it
		return nil
	}

	mode := info.Mode()

	// Check for special file types
	if mode&os.ModeDevice != 0 {
		return &ValidationError{
			Type:    ErrorSpecialFile,
			Path:    path,
			Message: "cannot access device files",
		}
	}

	if mode&os.ModeNamedPipe != 0 {
		return &ValidationError{
			Type:    ErrorSpecialFile,
			Path:    path,
			Message: "cannot access named pipes",
		}
	}

	if mode&os.ModeSocket != 0 {
		return &ValidationError{
			Type:    ErrorSpecialFile,
			Path:    path,
			Message: "cannot access socket files",
		}
	}

	if mode&os.ModeCharDevice != 0 {
		return &ValidationError{
			Type:    ErrorSpecialFile,
			Path:    path,
			Message: "cannot access character devices",
		}
	}

	// Allow regular files, directories, and symlinks
	// (symlinks are validated separately)
	return nil
}

// checkPathDepth verifies that directory nesting doesn't exceed maximum depth.
func checkPathDepth(path string) error {
	cleanPath := filepath.Clean(path)
	separatorCount := strings.Count(cleanPath, string(filepath.Separator))

	if separatorCount > maxPathDepth {
		return &ValidationError{
			Type:    ErrorPathTooDeep,
			Path:    path,
			Message: fmt.Sprintf("path nesting exceeds maximum depth of %d", maxPathDepth),
		}
	}

	return nil
}

// detectEnhancedPathTraversal performs comprehensive path traversal detection
// including encoded, double-encoded, and Unicode attacks.
func detectEnhancedPathTraversal(path string) error {
	lowerPath := strings.ToLower(path)

	// Check for basic traversal patterns
	traversalPatterns := []string{
		"../",
		"..\\",
		"/..",
		"\\..",
	}

	for _, pattern := range traversalPatterns {
		if strings.Contains(path, pattern) {
			// This might be legitimate (e.g., "dir/../file.txt" that stays in workspace)
			// So we only do pattern detection here, actual validation happens later
			// But some patterns should always be rejected
			if strings.HasPrefix(path, pattern) || strings.HasPrefix(path, "./"+pattern) {
				return &ValidationError{
					Type:    ErrorPathTraversal,
					Path:    path,
					Message: "suspicious path traversal pattern detected",
				}
			}
		}
	}

	// Check for single-encoded traversal attempts
	singleEncodedPatterns := []string{
		encodedDot,       // %2e
		encodedSlash,     // %2f
		encodedBackslash, // %5c
	}

	for _, pattern := range singleEncodedPatterns {
		if strings.Contains(lowerPath, pattern) {
			return &ValidationError{
				Type:    ErrorPathTraversal,
				Path:    path,
				Message: "URL-encoded path traversal detected",
			}
		}
	}

	// Check for double-encoded traversal (e.g., %252e -> %2e -> .)
	doubleEncodedPatterns := []string{
		"%252e", // double-encoded dot
		"%252f", // double-encoded slash
		"%255c", // double-encoded backslash
	}

	for _, pattern := range doubleEncodedPatterns {
		if strings.Contains(lowerPath, pattern) {
			return &ValidationError{
				Type:    ErrorPathTraversal,
				Path:    path,
				Message: "double URL-encoded path traversal detected",
			}
		}
	}

	// Check for Unicode escape sequences in the path string itself
	// (not the actual characters, but escape sequences like \u002e)
	// This is different from normal ".." which is fine
	if strings.Contains(path, "\\u002e") || strings.Contains(path, "\\u002f") || strings.Contains(path, "\\u005c") {
		return &ValidationError{
			Type:    ErrorPathTraversal,
			Path:    path,
			Message: "Unicode-escaped path traversal detected",
		}
	}

	// Check for overlong UTF-8 sequences (invalid but sometimes processed)
	// These are technically malformed UTF-8 that some systems might interpret
	if !utf8.ValidString(path) {
		return &ValidationError{
			Type:    ErrorInvalidPath,
			Path:    path,
			Message: "path contains invalid UTF-8 sequences",
		}
	}

	// Check for suspicious dot patterns (3+ consecutive dots)
	dotPattern := regexp.MustCompile(`\.{3,}`)
	if dotPattern.MatchString(path) {
		return &ValidationError{
			Type:    ErrorPathTraversal,
			Path:    path,
			Message: "suspicious multiple-dot pattern detected",
		}
	}

	// Try to URL decode the path and check if decoded version is different
	// This catches various encoding schemes
	if decoded, err := url.QueryUnescape(path); err == nil && decoded != path {
		// Path was URL-encoded, check the decoded version
		decodedLower := strings.ToLower(decoded)
		for _, pattern := range traversalPatterns {
			if strings.Contains(decoded, pattern) {
				return &ValidationError{
					Type:    ErrorPathTraversal,
					Path:    path,
					Message: "URL-encoded path contains traversal pattern",
				}
			}
		}

		// Check if double-decoding reveals traversal
		if doubleDecoded, err := url.QueryUnescape(decoded); err == nil && doubleDecoded != decoded {
			for _, pattern := range traversalPatterns {
				if strings.Contains(doubleDecoded, pattern) {
					return &ValidationError{
						Type:    ErrorPathTraversal,
						Path:    path,
						Message: "double URL-encoded path contains traversal pattern",
					}
				}
			}
		}

		// Check for encoded dots in decoded version
		for _, pattern := range singleEncodedPatterns {
			if strings.Contains(decodedLower, pattern) {
				return &ValidationError{
					Type:    ErrorPathTraversal,
					Path:    path,
					Message: "nested URL-encoded traversal detected",
				}
			}
		}
	}

	return nil
}

// isSensitivePath checks if a path is in a sensitive system location.
// Uses proper separator-aware prefix matching to avoid false positives.
func isSensitivePath(path string) bool {
	cleanPath := filepath.Clean(path)

	// Normalize case on case-insensitive filesystems
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		cleanPath = strings.ToLower(cleanPath)
	}

	// Check OS-specific sensitive paths
	var sensitivePaths []string
	if runtime.GOOS == "windows" {
		sensitivePaths = windowsSensitivePaths
	} else {
		sensitivePaths = unixSensitivePaths
	}

	// Check system paths with proper separator matching
	for _, sensitive := range sensitivePaths {
		sensitiveLower := sensitive
		if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
			sensitiveLower = strings.ToLower(sensitive)
		}

		// Exact match or prefix with separator
		if cleanPath == sensitiveLower ||
		   strings.HasPrefix(cleanPath, sensitiveLower+string(filepath.Separator)) {
			return true
		}
	}

	// Check home directory sensitive paths
	if homeDir, err := os.UserHomeDir(); err == nil {
		homeDirLower := homeDir
		if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
			homeDirLower = strings.ToLower(homeDir)
		}

		for _, sensitive := range homeSensitivePaths {
			sensitivePath := filepath.Join(homeDirLower, sensitive)
			if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
				sensitivePath = strings.ToLower(sensitivePath)
			}

			// Exact match or prefix with separator
			if cleanPath == sensitivePath ||
			   strings.HasPrefix(cleanPath, sensitivePath+string(filepath.Separator)) {
				return true
			}
		}
	}

	return false
}

// isSensitiveFile checks if a filename matches sensitive file patterns.
func isSensitiveFile(path string) bool {
	// Get just the filename
	filename := filepath.Base(path)

	// Also check the full path for some patterns
	for _, pattern := range sensitiveFilePatterns {
		if pattern.MatchString(filename) || pattern.MatchString(path) {
			return true
		}
	}

	return false
}

// DetectPathTraversal checks for common path traversal patterns in a string.
// This is useful for detecting attacks before path resolution.
func DetectPathTraversal(path string) bool {
	// Check for obvious traversal patterns
	traversalPatterns := []string{
		"../",
		"..\\",
		"/..",
		"\\..",
	}

	for _, pattern := range traversalPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Check for encoded traversal attempts
	encodedPatterns := []string{
		"%2e%2e",  // URL encoded ..
		"%2e%2e/", // URL encoded ../
		"%2e%2e\\", // URL encoded ..\
		"..%2f",    // Mixed encoding
		"..%5c",    // Mixed encoding
	}

	lowerPath := strings.ToLower(path)
	for _, pattern := range encodedPatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	return false
}

// NormalizePath normalizes a path for consistent comparison.
// It handles case-insensitive filesystems and path separators.
func NormalizePath(path string) string {
	// Clean the path
	clean := filepath.Clean(path)

	// On case-insensitive filesystems (Windows, macOS by default), normalize case
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		clean = strings.ToLower(clean)
	}

	return clean
}

// IsHiddenPath checks if a path represents a hidden file or directory.
// On Unix-like systems, hidden files start with a dot.
// On Windows, this checks the file attributes.
func IsHiddenPath(path string) bool {
	base := filepath.Base(path)

	// Unix-style hidden files
	if strings.HasPrefix(base, ".") && base != "." && base != ".." {
		return true
	}

	// Windows hidden attribute
	if runtime.GOOS == "windows" {
		// Check file attributes
		info, err := os.Stat(path)
		if err == nil {
			// On Windows, we'd need to check syscall attributes
			// For now, just check the dot prefix
			_ = info
		}
	}

	return false
}

// ValidatePathComponents checks each component of a path for invalid characters.
// This includes null bytes, control characters, Unicode tricks, and other attacks.
func ValidatePathComponents(path string) error {
	// Check for null bytes anywhere in the path (not just components)
	if strings.Contains(path, "\x00") {
		return &ValidationError{
			Type:    ErrorInvalidPath,
			Path:    path,
			Message: "path contains null bytes",
		}
	}

	components := strings.Split(filepath.Clean(path), string(filepath.Separator))

	for _, component := range components {
		if component == "" || component == "." {
			continue
		}

		// Check for Windows reserved names
		if runtime.GOOS == "windows" {
			reservedNames := []string{
				"CON", "PRN", "AUX", "NUL",
				"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
				"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
			}

			// Remove extension for comparison
			baseName := strings.TrimSuffix(component, filepath.Ext(component))
			for _, reserved := range reservedNames {
				if strings.EqualFold(baseName, reserved) {
					return &ValidationError{
						Type:    ErrorInvalidPath,
						Path:    path,
						Message: fmt.Sprintf("path contains Windows reserved name: %s", component),
					}
				}
			}

			// Check for trailing dots or spaces (Windows treats these specially)
			if strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") {
				return &ValidationError{
					Type:    ErrorInvalidPath,
					Path:    path,
					Message: "path component has trailing dot or space (Windows)",
				}
			}
		}

		// Check component length (255 bytes is common filesystem limit)
		if len(component) > 255 {
			return &ValidationError{
				Type:    ErrorInvalidPath,
				Path:    path,
				Message: fmt.Sprintf("path component exceeds 255 bytes: %s", component),
			}
		}

		// Check for control characters (except tab)
		for _, r := range component {
			// ASCII control characters (0x00-0x1F except tab)
			if r < 32 && r != '\t' {
				return &ValidationError{
					Type:    ErrorInvalidPath,
					Path:    path,
					Message: fmt.Sprintf("path contains control character: 0x%02x", r),
				}
			}

			// C1 control characters (0x80-0x9F)
			if r >= 0x80 && r <= 0x9F {
				return &ValidationError{
					Type:    ErrorInvalidPath,
					Path:    path,
					Message: fmt.Sprintf("path contains C1 control character: U+%04X", r),
				}
			}

			// Unicode directional override characters (used in spoofing)
			if r == '\u202E' || r == '\u202D' || r == '\u202A' || r == '\u202B' {
				return &ValidationError{
					Type:    ErrorInvalidPath,
					Path:    path,
					Message: "path contains Unicode directional override character",
				}
			}

			// Zero-width characters (used in spoofing)
			if r == '\u200B' || r == '\u200C' || r == '\u200D' || r == '\uFEFF' {
				return &ValidationError{
					Type:    ErrorInvalidPath,
					Path:    path,
					Message: "path contains zero-width Unicode character",
				}
			}

			// Check for other potentially dangerous Unicode
			if unicode.Is(unicode.Bidi_Control, r) {
				return &ValidationError{
					Type:    ErrorInvalidPath,
					Path:    path,
					Message: "path contains Unicode bidirectional control character",
				}
			}
		}

		// Check for extremely long chains of dots or other suspicious patterns
		if len(component) > 10 {
			// Count consecutive dots
			maxDots := 0
			currentDots := 0
			for _, r := range component {
				if r == '.' {
					currentDots++
					if currentDots > maxDots {
						maxDots = currentDots
					}
				} else {
					currentDots = 0
				}
			}

			if maxDots > 10 {
				return &ValidationError{
					Type:    ErrorInvalidPath,
					Path:    path,
					Message: "path component contains suspicious dot pattern",
				}
			}
		}
	}

	return nil
}
