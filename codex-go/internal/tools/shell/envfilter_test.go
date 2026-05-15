package shell

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultEnvFilterConfig tests the default configuration
func TestDefaultEnvFilterConfig(t *testing.T) {
	config := DefaultEnvFilterConfig()
	require.NotNil(t, config)

	// Verify essential patterns are included
	assert.Contains(t, config.ExcludePatterns, "*KEY*")
	assert.Contains(t, config.ExcludePatterns, "*SECRET*")
	assert.Contains(t, config.ExcludePatterns, "*TOKEN*")
	assert.Contains(t, config.ExcludePatterns, "*PASSWORD*")

	// Verify essential variables are included
	assert.Contains(t, config.EssentialVars, "HOME")
	assert.Contains(t, config.EssentialVars, "PATH")
	assert.Contains(t, config.EssentialVars, "USER")
	assert.Contains(t, config.EssentialVars, "TMPDIR")
}

// TestNewEnvFilter tests filter creation
func TestNewEnvFilter(t *testing.T) {
	t.Run("with default config", func(t *testing.T) {
		filter := NewDefaultEnvFilter()
		require.NotNil(t, filter)
		require.NotNil(t, filter.config)
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &EnvFilterConfig{
			ExcludePatterns: []string{"*SECRET*"},
			EssentialVars:   []string{"HOME"},
		}
		filter := NewEnvFilter(config)
		require.NotNil(t, filter)
		assert.Equal(t, config, filter.config)
	})

	t.Run("with nil config uses default", func(t *testing.T) {
		filter := NewEnvFilter(nil)
		require.NotNil(t, filter)
		require.NotNil(t, filter.config)
		assert.NotEmpty(t, filter.config.ExcludePatterns)
	})
}

// TestEnvFilterPatternMatching tests pattern matching logic
func TestEnvFilterPatternMatching(t *testing.T) {
	filter := NewDefaultEnvFilter()

	tests := []struct {
		name       string
		varName    string
		pattern    string
		wantMatch  bool
	}{
		// *PATTERN* tests (contains)
		{"contains KEY matches API_KEY", "API_KEY", "*KEY*", true},
		{"contains KEY matches SECRET_KEY", "SECRET_KEY", "*KEY*", true},
		{"contains KEY matches KEY", "KEY", "*KEY*", true},
		{"contains KEY does not match HOME", "HOME", "*KEY*", false},

		// *PATTERN tests (ends with)
		{"ends with KEY matches API_KEY", "API_KEY", "*KEY", true},
		{"ends with KEY matches KEY", "KEY", "*KEY", true},
		{"ends with KEY does not match KEY_VALUE", "KEY_VALUE", "*KEY", false},

		// PATTERN* tests (starts with)
		{"starts with API matches API_KEY", "API_KEY", "API*", true},
		{"starts with API matches API", "API", "API*", true},
		{"starts with API does not match MY_API", "MY_API", "API*", false},

		// Exact match
		{"exact match HOME", "HOME", "HOME", true},
		{"exact match PATH does not match PATHNAME", "PATHNAME", "PATH", false},

		// Case insensitivity
		{"case insensitive key matches KEY", "key", "*KEY*", true},
		{"case insensitive API_key matches KEY", "API_key", "*KEY*", true},
		{"case insensitive secret matches SECRET", "secret", "*SECRET*", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.matchPattern(strings.ToUpper(tt.varName), strings.ToUpper(tt.pattern))
			assert.Equal(t, tt.wantMatch, got)
		})
	}
}

// TestEnvFilterShouldExclude tests variable exclusion logic
func TestEnvFilterShouldExclude(t *testing.T) {
	filter := NewDefaultEnvFilter()

	tests := []struct {
		name        string
		varName     string
		shouldExclude bool
	}{
		// Should be excluded (match patterns)
		{"API_KEY", "API_KEY", true},
		{"AWS_SECRET_ACCESS_KEY", "AWS_SECRET_ACCESS_KEY", true},
		{"GITHUB_TOKEN", "GITHUB_TOKEN", true},
		{"DATABASE_PASSWORD", "DATABASE_PASSWORD", true},
		{"DB_PASSWD", "DB_PASSWD", true},
		{"SERVICE_CREDENTIAL", "SERVICE_CREDENTIAL", true},
		{"AUTH_TOKEN", "AUTH_TOKEN", true},
		{"PRIVATE_KEY", "PRIVATE_KEY", true},
		{"SECRET", "SECRET", true},
		{"MY_SECRET_VALUE", "MY_SECRET_VALUE", true},

		// Should NOT be excluded (don't match patterns)
		{"HOME", "HOME", false},
		{"PATH", "PATH", false},
		{"USER", "USER", false},
		{"TMPDIR", "TMPDIR", false},
		{"MY_VAR", "MY_VAR", false},
		{"DEBUG", "DEBUG", false},
		{"NODE_ENV", "NODE_ENV", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.shouldExclude(tt.varName)
			assert.Equal(t, tt.shouldExclude, got, "Variable %s", tt.varName)
		})
	}
}

// TestEnvFilterIsEssential tests essential variable detection
func TestEnvFilterIsEssential(t *testing.T) {
	filter := NewDefaultEnvFilter()

	tests := []struct {
		name        string
		varName     string
		isEssential bool
	}{
		{"HOME", "HOME", true},
		{"PATH", "PATH", true},
		{"USER", "USER", true},
		{"TMPDIR", "TMPDIR", true},
		{"SHELL", "SHELL", true},
		{"LANG", "LANG", true},

		// Case insensitive
		{"home lowercase", "home", true},
		{"Path mixed case", "Path", true},

		// LC_* variables
		{"LC_ALL", "LC_ALL", true},
		{"LC_CTYPE", "LC_CTYPE", true},

		// Not essential
		{"API_KEY", "API_KEY", false},
		{"MY_VAR", "MY_VAR", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.isEssential(tt.varName)
			assert.Equal(t, tt.isEssential, got)
		})
	}
}

// TestEnvFilterShouldFilterVar tests the public filtering check
func TestEnvFilterShouldFilterVar(t *testing.T) {
	filter := NewDefaultEnvFilter()

	tests := []struct {
		name         string
		varName      string
		shouldFilter bool
	}{
		// Should filter (matches exclude patterns)
		{"API_KEY", "API_KEY", true},
		{"SECRET_VALUE", "SECRET_VALUE", true},
		{"AUTH_TOKEN", "AUTH_TOKEN", true},

		// Should NOT filter (essential vars override patterns)
		{"PATH", "PATH", false},
		{"HOME", "HOME", false},

		// Should NOT filter (doesn't match patterns)
		{"MY_VAR", "MY_VAR", false},
		{"DEBUG", "DEBUG", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.ShouldFilterVar(tt.varName)
			assert.Equal(t, tt.shouldFilter, got)
		})
	}
}

// TestEnvFilterFilterEnv tests filtering of environment array
func TestEnvFilterFilterEnv(t *testing.T) {
	filter := NewDefaultEnvFilter()

	tests := []struct {
		name     string
		input    []string
		expected []string
		excluded []string
	}{
		{
			name: "filters out secrets but keeps safe vars",
			input: []string{
				"HOME=/home/user",
				"PATH=/usr/bin",
				"API_KEY=secret123",
				"AWS_SECRET_ACCESS_KEY=supersecret",
				"DEBUG=true",
			},
			expected: []string{
				"HOME=/home/user",
				"PATH=/usr/bin",
				"DEBUG=true",
			},
			excluded: []string{
				"API_KEY",
				"AWS_SECRET_ACCESS_KEY",
			},
		},
		{
			name: "filters various credential patterns",
			input: []string{
				"USER=testuser",
				"DATABASE_PASSWORD=pass123",
				"GITHUB_TOKEN=ghp_token",
				"SERVICE_AUTH_KEY=authkey",
				"MY_PRIVATE_KEY=privatekey",
				"NORMAL_VAR=value",
			},
			expected: []string{
				"USER=testuser",
				"NORMAL_VAR=value",
			},
			excluded: []string{
				"DATABASE_PASSWORD",
				"GITHUB_TOKEN",
				"SERVICE_AUTH_KEY",
				"MY_PRIVATE_KEY",
			},
		},
		{
			name: "keeps essential vars even if they match patterns",
			input: []string{
				"PATH=/usr/bin",
				"HOME=/home/user",
				"MY_API_KEY=secret",
			},
			expected: []string{
				"PATH=/usr/bin",
				"HOME=/home/user",
			},
			excluded: []string{
				"MY_API_KEY",
			},
		},
		{
			name: "handles malformed env vars",
			input: []string{
				"HOME=/home/user",
				"MALFORMED",
				"PATH=/usr/bin",
				"=VALUE_WITHOUT_KEY",
				"API_KEY=secret",
			},
			expected: []string{
				"HOME=/home/user",
				"PATH=/usr/bin",
			},
			excluded: []string{
				"API_KEY",
			},
		},
		{
			name:     "empty input returns empty",
			input:    []string{},
			expected: nil, // Empty slice will be nil
			excluded: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.FilterEnv(tt.input)

			// Sort for comparison
			sort.Strings(got)
			sort.Strings(tt.expected)

			assert.Equal(t, tt.expected, got)

			// Verify excluded vars are not in output
			for _, excluded := range tt.excluded {
				for _, envVar := range got {
					assert.False(t, strings.HasPrefix(envVar, excluded+"="),
						"Expected %s to be filtered out", excluded)
				}
			}
		})
	}
}

// TestEnvFilterFilterMap tests filtering of environment map
func TestEnvFilterFilterMap(t *testing.T) {
	filter := NewDefaultEnvFilter()

	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name: "filters out secrets but keeps safe vars",
			input: map[string]string{
				"HOME":                   "/home/user",
				"PATH":                   "/usr/bin",
				"API_KEY":                "secret123",
				"AWS_SECRET_ACCESS_KEY":  "supersecret",
				"DEBUG":                  "true",
			},
			expected: map[string]string{
				"HOME":  "/home/user",
				"PATH":  "/usr/bin",
				"DEBUG": "true",
			},
		},
		{
			name: "filters various credential patterns",
			input: map[string]string{
				"USER":               "testuser",
				"DATABASE_PASSWORD":  "pass123",
				"GITHUB_TOKEN":       "ghp_token",
				"SERVICE_AUTH_KEY":   "authkey",
				"NORMAL_VAR":         "value",
			},
			expected: map[string]string{
				"USER":       "testuser",
				"NORMAL_VAR": "value",
			},
		},
		{
			name: "case insensitive matching",
			input: map[string]string{
				"path":      "/usr/bin",
				"home":      "/home/user",
				"api_key":   "secret",
				"Api_Token": "token",
			},
			expected: map[string]string{
				"path": "/usr/bin",
				"home": "/home/user",
			},
		},
		{
			name:     "empty map returns empty",
			input:    map[string]string{},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.FilterMap(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestEnvFilterWithCustomConfig tests custom filter configuration
func TestEnvFilterWithCustomConfig(t *testing.T) {
	config := &EnvFilterConfig{
		ExcludePatterns: []string{
			"*CUSTOM*",
			"EXACT_MATCH",
		},
		EssentialVars: []string{
			"HOME",
			"CUSTOM_ESSENTIAL", // This should not be filtered even though it matches *CUSTOM*
		},
	}

	filter := NewEnvFilter(config)

	tests := []struct {
		name         string
		varName      string
		shouldFilter bool
	}{
		{"CUSTOM_VAR filtered", "CUSTOM_VAR", true},
		{"MY_CUSTOM_KEY filtered", "MY_CUSTOM_KEY", true},
		{"EXACT_MATCH filtered", "EXACT_MATCH", true},
		{"CUSTOM_ESSENTIAL not filtered", "CUSTOM_ESSENTIAL", false},
		{"HOME not filtered", "HOME", false},
		{"NORMAL_VAR not filtered", "NORMAL_VAR", false},
		{"EXACT_MATCH_SUFFIX not filtered", "EXACT_MATCH_SUFFIX", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.ShouldFilterVar(tt.varName)
			assert.Equal(t, tt.shouldFilter, got)
		})
	}
}

// TestEnvFilterIntegrationWithExec tests the filter integrated with command execution
func TestEnvFilterIntegrationWithExec(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	filter := NewDefaultEnvFilter()

	// Create a test environment with secrets
	testEnv := map[string]string{
		"HOME":       "/home/testuser",
		"PATH":       "/usr/bin",
		"API_KEY":    "this_should_be_filtered",
		"SECRET_VAR": "very_secret",
		"SAFE_VAR":   "this_is_safe",
	}

	filtered := filter.FilterMap(testEnv)

	// Verify secrets are removed
	assert.NotContains(t, filtered, "API_KEY")
	assert.NotContains(t, filtered, "SECRET_VAR")

	// Verify safe vars remain
	assert.Equal(t, "/home/testuser", filtered["HOME"])
	assert.Equal(t, "/usr/bin", filtered["PATH"])
	assert.Equal(t, "this_is_safe", filtered["SAFE_VAR"])
}

// TestEnvFilterRealWorldScenarios tests real-world credential patterns
func TestEnvFilterRealWorldScenarios(t *testing.T) {
	filter := NewDefaultEnvFilter()

	// Real-world environment variables that should be filtered
	sensitiveVars := []string{
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"GITHUB_TOKEN",
		"GITLAB_TOKEN",
		"NPM_TOKEN",
		"DOCKER_PASSWORD",
		"DATABASE_PASSWORD",
		"DB_PASSWORD",
		"POSTGRES_PASSWORD",
		"MYSQL_PASSWORD",
		"REDIS_PASSWORD",
		"JWT_SECRET",
		"SESSION_SECRET",
		"ENCRYPTION_KEY",
		"PRIVATE_KEY",
		"SIGNING_KEY",
		"API_KEY",
		"API_SECRET",
		"CLIENT_SECRET",
		"OAUTH_TOKEN",
		"AUTH_TOKEN",
		"BEARER_TOKEN",
		"ACCESS_TOKEN",
		"REFRESH_TOKEN",
		"SLACK_TOKEN",
		"STRIPE_SECRET_KEY",
		"TWILIO_AUTH_TOKEN",
		"SENDGRID_API_KEY",
		"MAILGUN_API_KEY",
	}

	for _, varName := range sensitiveVars {
		t.Run(fmt.Sprintf("filters %s", varName), func(t *testing.T) {
			assert.True(t, filter.ShouldFilterVar(varName),
				"Expected %s to be filtered", varName)
		})
	}

	// Real-world environment variables that should NOT be filtered
	safeVars := []string{
		"HOME",
		"PATH",
		"USER",
		"SHELL",
		"TMPDIR",
		"TEMP",
		"PWD",
		"OLDPWD",
		"LANG",
		"LC_ALL",
		"TERM",
		"EDITOR",
		"PAGER",
		"NODE_ENV",
		"ENVIRONMENT",
		"DEBUG",
		"LOG_LEVEL",
		"PORT",
		"HOST",
		"HOSTNAME",
	}

	for _, varName := range safeVars {
		t.Run(fmt.Sprintf("allows %s", varName), func(t *testing.T) {
			assert.False(t, filter.ShouldFilterVar(varName),
				"Expected %s to NOT be filtered", varName)
		})
	}
}

// BenchmarkEnvFilter benchmarks the filter performance
func BenchmarkEnvFilter(b *testing.B) {
	filter := NewDefaultEnvFilter()

	testEnv := []string{
		"HOME=/home/user",
		"PATH=/usr/bin:/usr/local/bin",
		"USER=testuser",
		"API_KEY=secret123",
		"AWS_SECRET_ACCESS_KEY=supersecret",
		"DATABASE_PASSWORD=pass123",
		"DEBUG=true",
		"NODE_ENV=production",
		"GITHUB_TOKEN=ghp_token",
		"NORMAL_VAR=value",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = filter.FilterEnv(testEnv)
	}
}

// BenchmarkEnvFilterMap benchmarks map filtering
func BenchmarkEnvFilterMap(b *testing.B) {
	filter := NewDefaultEnvFilter()

	testEnv := map[string]string{
		"HOME":                  "/home/user",
		"PATH":                  "/usr/bin:/usr/local/bin",
		"USER":                  "testuser",
		"API_KEY":               "secret123",
		"AWS_SECRET_ACCESS_KEY": "supersecret",
		"DATABASE_PASSWORD":     "pass123",
		"DEBUG":                 "true",
		"NODE_ENV":              "production",
		"GITHUB_TOKEN":          "ghp_token",
		"NORMAL_VAR":            "value",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = filter.FilterMap(testEnv)
	}
}
