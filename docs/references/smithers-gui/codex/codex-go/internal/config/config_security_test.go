package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMCPServerConfig_PathTraversal_Command tests that path traversal is blocked in Command fields
func TestMCPServerConfig_PathTraversal_Command(t *testing.T) {
	tests := []struct {
		name      string
		config    MCPServerConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid command - absolute path",
			config: MCPServerConfig{
				Command: "/usr/bin/npx",
			},
			wantError: false,
		},
		{
			name: "valid command - relative name",
			config: MCPServerConfig{
				Command: "npx",
			},
			wantError: false,
		},
		{
			name: "path traversal in command",
			config: MCPServerConfig{
				Command: "../../usr/bin/evil",
			},
			wantError: true,
			errorMsg:  "path traversal",
		},
		{
			name: "path traversal with spaces",
			config: MCPServerConfig{
				Command: "/usr/bin/../evil",
			},
			wantError: true,
			errorMsg:  "path traversal",
		},
		{
			name: "multiple path traversals",
			config: MCPServerConfig{
				Command: "../../../etc/passwd",
			},
			wantError: true,
			errorMsg:  "path traversal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestMCPServerConfig_PathTraversal_CWD tests that path traversal is blocked in CWD fields
func TestMCPServerConfig_PathTraversal_CWD(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		config    MCPServerConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid cwd",
			config: MCPServerConfig{
				Command: "npx",
				CWD:     tmpDir,
			},
			wantError: false,
		},
		{
			name: "path traversal in cwd",
			config: MCPServerConfig{
				Command: "npx",
				CWD:     tmpDir + "/../etc",
			},
			wantError: true,
			errorMsg:  "path traversal",
		},
		{
			name: "relative path traversal in cwd",
			config: MCPServerConfig{
				Command: "npx",
				CWD:     "../../../etc",
			},
			wantError: true,
			errorMsg:  "path traversal",
		},
		{
			name: "cwd does not exist",
			config: MCPServerConfig{
				Command: "npx",
				CWD:     "/nonexistent/directory",
			},
			wantError: true,
			errorMsg:  "does not exist",
		},
		{
			name: "cwd is a file not directory",
			config: MCPServerConfig{
				Command: "npx",
				CWD:     func() string {
					f := filepath.Join(tmpDir, "file.txt")
					os.WriteFile(f, []byte("test"), 0644)
					return f
				}(),
			},
			wantError: true,
			errorMsg:  "not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestMCPServerConfig_TransportValidation tests that exactly one transport is configured
func TestMCPServerConfig_TransportValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    MCPServerConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid stdio transport",
			config: MCPServerConfig{
				Command: "npx",
			},
			wantError: false,
		},
		{
			name: "valid http transport",
			config: MCPServerConfig{
				URL: "https://api.example.com/mcp",
			},
			wantError: false,
		},
		{
			name: "both command and url set",
			config: MCPServerConfig{
				Command: "npx",
				URL:     "https://api.example.com/mcp",
			},
			wantError: true,
			errorMsg:  "cannot specify both command and url",
		},
		{
			name:      "neither command nor url set",
			config:    MCPServerConfig{},
			wantError: true,
			errorMsg:  "must specify either command or url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestMCPServerConfig_URLValidation tests URL validation
func TestMCPServerConfig_URLValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    MCPServerConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid https url",
			config: MCPServerConfig{
				URL: "https://api.example.com/mcp",
			},
			wantError: false,
		},
		{
			name: "valid http url",
			config: MCPServerConfig{
				URL: "http://localhost:8080/mcp",
			},
			wantError: false,
		},
		{
			name: "invalid url scheme - ftp",
			config: MCPServerConfig{
				URL: "ftp://example.com/mcp",
			},
			wantError: true,
			errorMsg:  "must use http or https scheme",
		},
		{
			name: "invalid url scheme - file",
			config: MCPServerConfig{
				URL: "file:///etc/passwd",
			},
			wantError: true,
			errorMsg:  "must use http or https scheme",
		},
		{
			name: "invalid url format",
			config: MCPServerConfig{
				URL: "ht!tp://invalid url",
			},
			wantError: true,
			errorMsg:  "invalid url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestMCPServerConfig_TimeoutValidation tests timeout validation
func TestMCPServerConfig_TimeoutValidation(t *testing.T) {
	negativeFloat := -1.0
	zeroFloat := 0.0
	positiveFloat := 30.0

	tests := []struct {
		name      string
		config    MCPServerConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid positive timeout",
			config: MCPServerConfig{
				Command:           "npx",
				StartupTimeoutSec: &positiveFloat,
				ToolTimeoutSec:    &positiveFloat,
			},
			wantError: false,
		},
		{
			name: "negative startup timeout",
			config: MCPServerConfig{
				Command:           "npx",
				StartupTimeoutSec: &negativeFloat,
			},
			wantError: true,
			errorMsg:  "startup_timeout_sec must be positive",
		},
		{
			name: "zero startup timeout",
			config: MCPServerConfig{
				Command:           "npx",
				StartupTimeoutSec: &zeroFloat,
			},
			wantError: true,
			errorMsg:  "startup_timeout_sec must be positive",
		},
		{
			name: "negative tool timeout",
			config: MCPServerConfig{
				Command:        "npx",
				ToolTimeoutSec: &negativeFloat,
			},
			wantError: true,
			errorMsg:  "tool_timeout_sec must be positive",
		},
		{
			name: "zero tool timeout",
			config: MCPServerConfig{
				Command:        "npx",
				ToolTimeoutSec: &zeroFloat,
			},
			wantError: true,
			errorMsg:  "tool_timeout_sec must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestMCPOAuthConfig_Validation tests OAuth configuration validation
func TestMCPOAuthConfig_Validation(t *testing.T) {
	tests := []struct {
		name      string
		config    MCPOAuthConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid oauth with urls",
			config: MCPOAuthConfig{
				ClientID: "client-id",
				AuthURL:  "https://auth.example.com/oauth",
				TokenURL: "https://auth.example.com/token",
				Scopes:   []string{"read", "write"},
			},
			wantError: false,
		},
		{
			name: "valid oauth with discovery",
			config: MCPOAuthConfig{
				ClientID:     "client-id",
				UseDiscovery: true,
				Scopes:       []string{"read"},
			},
			wantError: false,
		},
		{
			name: "empty client_id",
			config: MCPOAuthConfig{
				ClientID: "",
				AuthURL:  "https://auth.example.com/oauth",
				TokenURL: "https://auth.example.com/token",
				Scopes:   []string{"read"},
			},
			wantError: true,
			errorMsg:  "client_id cannot be empty",
		},
		{
			name: "discovery with urls set",
			config: MCPOAuthConfig{
				ClientID:     "client-id",
				UseDiscovery: true,
				AuthURL:      "https://auth.example.com/oauth",
				TokenURL:     "https://auth.example.com/token",
				Scopes:       []string{"read"},
			},
			wantError: true,
			errorMsg:  "cannot specify auth_url or token_url when use_discovery is true",
		},
		{
			name: "no discovery missing auth_url",
			config: MCPOAuthConfig{
				ClientID: "client-id",
				TokenURL: "https://auth.example.com/token",
				Scopes:   []string{"read"},
			},
			wantError: true,
			errorMsg:  "auth_url and token_url are required",
		},
		{
			name: "no discovery missing token_url",
			config: MCPOAuthConfig{
				ClientID: "client-id",
				AuthURL:  "https://auth.example.com/oauth",
				Scopes:   []string{"read"},
			},
			wantError: true,
			errorMsg:  "auth_url and token_url are required",
		},
		{
			name: "empty scopes",
			config: MCPOAuthConfig{
				ClientID: "client-id",
				AuthURL:  "https://auth.example.com/oauth",
				TokenURL: "https://auth.example.com/token",
				Scopes:   []string{},
			},
			wantError: true,
			errorMsg:  "scopes cannot be empty",
		},
		{
			name: "invalid auth_url",
			config: MCPOAuthConfig{
				ClientID: "client-id",
				AuthURL:  "ht!tp://invalid",
				TokenURL: "https://auth.example.com/token",
				Scopes:   []string{"read"},
			},
			wantError: true,
			errorMsg:  "invalid auth_url",
		},
		{
			name: "invalid token_url",
			config: MCPOAuthConfig{
				ClientID: "client-id",
				AuthURL:  "https://auth.example.com/oauth",
				TokenURL: "ht!tp://invalid",
				Scopes:   []string{"read"},
			},
			wantError: true,
			errorMsg:  "invalid token_url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestNotifyConfig_Validation tests notification configuration validation
func TestNotifyConfig_Validation(t *testing.T) {
	negativeTimeout := -1.0
	zeroTimeout := 0.0
	positiveTimeout := 30.0

	tests := []struct {
		name      string
		config    NotifyConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid notify config",
			config: NotifyConfig{
				ScriptTimeoutSec: &positiveTimeout,
				OnTurnComplete: &NotifyTriggerConfig{
					Command: "/usr/bin/notify",
					Enabled: true,
				},
			},
			wantError: false,
		},
		{
			name: "negative script timeout",
			config: NotifyConfig{
				ScriptTimeoutSec: &negativeTimeout,
			},
			wantError: true,
			errorMsg:  "script_timeout_sec must be positive",
		},
		{
			name: "zero script timeout",
			config: NotifyConfig{
				ScriptTimeoutSec: &zeroTimeout,
			},
			wantError: true,
			errorMsg:  "script_timeout_sec must be positive",
		},
		{
			name: "invalid on_turn_complete",
			config: NotifyConfig{
				OnTurnComplete: &NotifyTriggerConfig{
					Enabled: true,
					Command: "", // Empty command when enabled
				},
			},
			wantError: true,
			errorMsg:  "on_turn_complete",
		},
		{
			name: "invalid on_error",
			config: NotifyConfig{
				OnError: &NotifyTriggerConfig{
					Enabled: true,
					Command: "", // Empty command when enabled
				},
			},
			wantError: true,
			errorMsg:  "on_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestNotifyTriggerConfig_Validation tests notification trigger validation
func TestNotifyTriggerConfig_Validation(t *testing.T) {
	tests := []struct {
		name      string
		config    NotifyTriggerConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid trigger config",
			config: NotifyTriggerConfig{
				Command: "/usr/bin/notify",
				Enabled: true,
			},
			wantError: false,
		},
		{
			name: "disabled with no command",
			config: NotifyTriggerConfig{
				Enabled: false,
			},
			wantError: false,
		},
		{
			name: "enabled with empty command",
			config: NotifyTriggerConfig{
				Command: "",
				Enabled: true,
			},
			wantError: true,
			errorMsg:  "command cannot be empty when enabled is true",
		},
		{
			name: "path traversal in command",
			config: NotifyTriggerConfig{
				Command: "../../evil/script",
				Enabled: true,
			},
			wantError: true,
			errorMsg:  "path traversal",
		},
		{
			name: "empty env var key",
			config: NotifyTriggerConfig{
				Command: "/usr/bin/notify",
				Enabled: true,
				Env: map[string]string{
					"": "value",
				},
			},
			wantError: true,
			errorMsg:  "environment variable key cannot be empty",
		},
		{
			name: "env var key with equals",
			config: NotifyTriggerConfig{
				Command: "/usr/bin/notify",
				Enabled: true,
				Env: map[string]string{
					"KEY=VALUE": "value",
				},
			},
			wantError: true,
			errorMsg:  "invalid characters",
		},
		{
			name: "env var key with null byte",
			config: NotifyTriggerConfig{
				Command: "/usr/bin/notify",
				Enabled: true,
				Env: map[string]string{
					"KEY\x00": "value",
				},
			},
			wantError: true,
			errorMsg:  "invalid characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestConfig_WebSearchValidation tests web search provider validation
func TestConfig_WebSearchValidation(t *testing.T) {
	baseConfig := Config{
		Model:              "gpt-5-codex",
		ReviewModel:        "gpt-5-codex",
		ModelProvider:      "openai-responses",
		CodexHome:          "/tmp/codex",
		ProjectDocMaxBytes: 32 * 1024,
		ChatGPTBaseURL:     "https://chatgpt.com",
	}

	tests := []struct {
		name      string
		config    Config
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid web search - duckduckgo",
			config: func() Config {
				c := baseConfig
				c.WebSearchEnabled = true
				c.WebSearchProvider = "duckduckgo"
				return c
			}(),
			wantError: false,
		},
		{
			name: "valid web search - google",
			config: func() Config {
				c := baseConfig
				c.WebSearchEnabled = true
				c.WebSearchProvider = "google"
				return c
			}(),
			wantError: false,
		},
		{
			name: "web search disabled - any provider ok",
			config: func() Config {
				c := baseConfig
				c.WebSearchEnabled = false
				c.WebSearchProvider = "anything"
				return c
			}(),
			wantError: false,
		},
		{
			name: "web search enabled - empty provider",
			config: func() Config {
				c := baseConfig
				c.WebSearchEnabled = true
				c.WebSearchProvider = ""
				return c
			}(),
			wantError: true,
			errorMsg:  "web_search_provider cannot be empty",
		},
		{
			name: "web search enabled - invalid provider",
			config: func() Config {
				c := baseConfig
				c.WebSearchEnabled = true
				c.WebSearchProvider = "bing"
				return c
			}(),
			wantError: true,
			errorMsg:  "invalid web_search_provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestConfig_ProjectDocMaxBytes tests project doc size limits
func TestConfig_ProjectDocMaxBytes(t *testing.T) {
	baseConfig := Config{
		Model:          "gpt-5-codex",
		ReviewModel:    "gpt-5-codex",
		ModelProvider:  "openai-responses",
		CodexHome:      "/tmp/codex",
		ChatGPTBaseURL: "https://chatgpt.com",
	}

	tests := []struct {
		name      string
		maxBytes  int
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid size - 32KB",
			maxBytes:  32 * 1024,
			wantError: false,
		},
		{
			name:      "valid size - 1MB",
			maxBytes:  1024 * 1024,
			wantError: false,
		},
		{
			name:      "valid size - 10MB (max)",
			maxBytes:  10 * 1024 * 1024,
			wantError: false,
		},
		{
			name:      "exceeds maximum - 11MB",
			maxBytes:  11 * 1024 * 1024,
			wantError: true,
			errorMsg:  "exceeds maximum",
		},
		{
			name:      "exceeds maximum - 100MB",
			maxBytes:  100 * 1024 * 1024,
			wantError: true,
			errorMsg:  "exceeds maximum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := baseConfig
			config.ProjectDocMaxBytes = tt.maxBytes
			err := config.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestConfig_MCPServerIntegration tests MCP server validation in full config
func TestConfig_MCPServerIntegration(t *testing.T) {
	baseConfig := Config{
		Model:              "gpt-5-codex",
		ReviewModel:        "gpt-5-codex",
		ModelProvider:      "openai-responses",
		CodexHome:          "/tmp/codex",
		ProjectDocMaxBytes: 32 * 1024,
		ChatGPTBaseURL:     "https://chatgpt.com",
	}

	tests := []struct {
		name      string
		servers   map[string]MCPServerConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid mcp servers",
			servers: map[string]MCPServerConfig{
				"filesystem": {
					Command: "npx",
				},
				"api": {
					URL: "https://api.example.com/mcp",
				},
			},
			wantError: false,
		},
		{
			name: "invalid server - both command and url",
			servers: map[string]MCPServerConfig{
				"bad": {
					Command: "npx",
					URL:     "https://api.example.com/mcp",
				},
			},
			wantError: true,
			errorMsg:  "mcp_servers[bad]",
		},
		{
			name: "invalid server - path traversal",
			servers: map[string]MCPServerConfig{
				"evil": {
					Command: "../../evil/bin",
				},
			},
			wantError: true,
			errorMsg:  "mcp_servers[evil]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := baseConfig
			config.MCPServers = tt.servers
			err := config.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestMCPServerConfig_OAuth_Integration tests OAuth validation in MCP servers
func TestMCPServerConfig_OAuth_Integration(t *testing.T) {
	tests := []struct {
		name      string
		config    MCPServerConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid server with oauth",
			config: MCPServerConfig{
				URL: "https://api.example.com/mcp",
				OAuth: &MCPOAuthConfig{
					ClientID: "client-id",
					AuthURL:  "https://auth.example.com/oauth",
					TokenURL: "https://auth.example.com/token",
					Scopes:   []string{"read"},
				},
			},
			wantError: false,
		},
		{
			name: "server with invalid oauth",
			config: MCPServerConfig{
				URL: "https://api.example.com/mcp",
				OAuth: &MCPOAuthConfig{
					ClientID: "", // Invalid
					AuthURL:  "https://auth.example.com/oauth",
					TokenURL: "https://auth.example.com/token",
					Scopes:   []string{"read"},
				},
			},
			wantError: true,
			errorMsg:  "oauth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}
