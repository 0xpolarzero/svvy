// Package file provides file operation tool runtimes.
//
// This package includes tools for:
//   - read_file: Read file contents with line ranges and binary detection
//   - write_file: Write file contents atomically with sandboxing
//   - list_dir: List directory entries with recursive depth control and glob patterns
//   - grep_files: Search file contents using regular expressions with case-insensitive support
//
// Features:
//   - Path validation and sandboxing (prevents path traversal attacks)
//   - Binary file detection (prevents reading non-text files)
//   - Atomic writes (write to temp file, then rename)
//   - Glob pattern matching for file filtering
//   - Large file handling (streaming support)
//   - Parallel execution support (except writes)
//
// Security:
//   - All paths are validated against the workspace directory
//   - Path traversal attempts (e.g., ../) are blocked
//   - Read operations don't require approval (safe by default)
//   - Write operations may require approval based on policy
//
// Implementation uses afero.Fs for filesystem abstraction, enabling:
//   - Testing with in-memory filesystems
//   - Sandboxed filesystem access
//   - Cross-platform compatibility
package file
