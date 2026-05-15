package shell

import (
	"os"
	"strings"
)

// EnvFilterConfig defines configuration for environment variable filtering.
type EnvFilterConfig struct {
	// ExcludePatterns are case-insensitive patterns that will be filtered out.
	// Patterns support wildcards (*) at the beginning and end.
	// Default patterns: *KEY*, *SECRET*, *TOKEN*
	ExcludePatterns []string

	// EssentialVars are variables that should always be included, even if they match exclude patterns.
	// Default: HOME, PATH, USER, TMPDIR, TEMP, TMP, SHELL, LANG, LC_*
	EssentialVars []string
}

// DefaultEnvFilterConfig returns the default environment filter configuration
// that protects against common credential patterns.
func DefaultEnvFilterConfig() *EnvFilterConfig {
	return &EnvFilterConfig{
		ExcludePatterns: []string{
			"*KEY*",
			"*SECRET*",
			"*TOKEN*",
			"*PASSWORD*",
			"*PASSWD*",
			"*CREDENTIAL*",
			"*AUTH*",
			"*PRIVATE*",
		},
		EssentialVars: []string{
			"HOME",
			"PATH",
			"USER",
			"TMPDIR",
			"TEMP",
			"TMP",
			"SHELL",
			"LANG",
			"LC_ALL",
			"LC_CTYPE",
			"LC_NUMERIC",
			"LC_TIME",
			"LC_COLLATE",
			"LC_MONETARY",
			"LC_MESSAGES",
			"LC_PAPER",
			"LC_NAME",
			"LC_ADDRESS",
			"LC_TELEPHONE",
			"LC_MEASUREMENT",
			"LC_IDENTIFICATION",
		},
	}
}

// EnvFilter filters environment variables to prevent credential leakage.
type EnvFilter struct {
	config *EnvFilterConfig
}

// NewEnvFilter creates a new environment filter with the given configuration.
// If config is nil, uses the default configuration.
func NewEnvFilter(config *EnvFilterConfig) *EnvFilter {
	if config == nil {
		config = DefaultEnvFilterConfig()
	}
	return &EnvFilter{
		config: config,
	}
}

// NewDefaultEnvFilter creates a new environment filter with default configuration.
func NewDefaultEnvFilter() *EnvFilter {
	return NewEnvFilter(nil)
}

// Filter filters the current environment variables, removing any that match
// exclude patterns (unless they are essential variables).
func (f *EnvFilter) Filter() []string {
	return f.FilterEnv(os.Environ())
}

// FilterEnv filters the provided environment variables, removing any that match
// exclude patterns (unless they are essential variables).
func (f *EnvFilter) FilterEnv(env []string) []string {
	var filtered []string

	for _, envVar := range env {
		// Split into key=value
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			// Malformed env var, skip it
			continue
		}

		key := parts[0]

		// Skip empty keys
		if key == "" {
			continue
		}

		// Check if it's an essential variable
		if f.isEssential(key) {
			filtered = append(filtered, envVar)
			continue
		}

		// Check if it matches any exclude pattern
		if f.shouldExclude(key) {
			// Skip this variable - it's potentially sensitive
			continue
		}

		// Include the variable
		filtered = append(filtered, envVar)
	}

	return filtered
}

// FilterMap filters a map of environment variables, removing any that match
// exclude patterns (unless they are essential variables).
// Returns a new map with filtered variables.
func (f *EnvFilter) FilterMap(env map[string]string) map[string]string {
	filtered := make(map[string]string)

	for key, value := range env {
		// Check if it's an essential variable
		if f.isEssential(key) {
			filtered[key] = value
			continue
		}

		// Check if it matches any exclude pattern
		if f.shouldExclude(key) {
			// Skip this variable - it's potentially sensitive
			continue
		}

		// Include the variable
		filtered[key] = value
	}

	return filtered
}

// isEssential checks if a variable name is in the essential list.
func (f *EnvFilter) isEssential(key string) bool {
	upperKey := strings.ToUpper(key)
	for _, essential := range f.config.EssentialVars {
		if strings.ToUpper(essential) == upperKey {
			return true
		}
	}
	return false
}

// shouldExclude checks if a variable name matches any exclude pattern.
func (f *EnvFilter) shouldExclude(key string) bool {
	upperKey := strings.ToUpper(key)

	for _, pattern := range f.config.ExcludePatterns {
		if f.matchPattern(upperKey, strings.ToUpper(pattern)) {
			return true
		}
	}

	return false
}

// matchPattern performs case-insensitive wildcard pattern matching.
// Supports * at the beginning, end, or both.
func (f *EnvFilter) matchPattern(str, pattern string) bool {
	// Handle exact match (no wildcards)
	if !strings.Contains(pattern, "*") {
		return str == pattern
	}

	// Handle *PATTERN* (contains)
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		substr := strings.Trim(pattern, "*")
		if substr == "" {
			return true // "*" matches everything
		}
		return strings.Contains(str, substr)
	}

	// Handle *PATTERN (ends with)
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(str, suffix)
	}

	// Handle PATTERN* (starts with)
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(str, prefix)
	}

	// No wildcard case (already handled above, but just in case)
	return str == pattern
}

// ShouldFilterVar checks if a specific variable should be filtered.
// This is useful for testing or debugging.
func (f *EnvFilter) ShouldFilterVar(key string) bool {
	if f.isEssential(key) {
		return false
	}
	return f.shouldExclude(key)
}
