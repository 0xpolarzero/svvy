// Package config provides configuration loading and management for Codex.
// It supports loading from TOML files (~/.codex/config.toml) and environment
// variable overrides, matching the behavior of the Rust implementation.
package config

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/evmts/codex/codex-go/internal/models"
)

// Config represents the application configuration loaded from disk and merged with overrides.
type Config struct {
	// Model is the AI model to use (e.g., "gpt-5-codex")
	Model string

	// ReviewModel is the model used specifically for review sessions
	ReviewModel string

	// ModelProvider is the key into the model_providers map specifying which provider to use
	ModelProvider string

	// ModelContextWindow is the size of the context window for the model, in tokens
	ModelContextWindow *int64

	// ModelMaxOutputTokens is the maximum number of output tokens
	ModelMaxOutputTokens *int64

	// ModelAutoCompactTokenLimit is the token usage threshold triggering auto-compaction
	ModelAutoCompactTokenLimit *int64

	// ApprovalPolicy controls the approval policy for executing commands ("auto", "always", "never")
	ApprovalPolicy string

	// HideAgentReasoning when true suppresses AgentReasoning events from frontend output
	HideAgentReasoning bool

	// ShowRawAgentReasoning when true shows AgentReasoningRawContentEvent events in UI
	ShowRawAgentReasoning bool

	// ChatGPTBaseURL is the base URL for requests to ChatGPT
	ChatGPTBaseURL string

	// ForcedChatGPTWorkspaceID restricts ChatGPT login to a specific workspace
	ForcedChatGPTWorkspaceID string

	// CodexHome is the directory containing all Codex state (defaults to ~/.codex)
	CodexHome string

	// ProjectDocMaxBytes is the maximum number of bytes to include from AGENTS.md
	ProjectDocMaxBytes int

	// ProjectDocFallbackFilenames are additional filenames to try when AGENTS.md is missing
	ProjectDocFallbackFilenames []string

	// DisablePasteBurst disables burst-paste detection for typed input
	DisablePasteBurst bool

	// MCPServers defines MCP servers that Codex can reach out to for tool calls
	MCPServers map[string]MCPServerConfig

	// Notify contains notification configurations for various events
	Notify NotifyConfig

	// WebSearchProvider specifies the web search provider to use (e.g., "duckduckgo")
	WebSearchProvider string

	// WebSearchEnabled controls whether web search functionality is available
	WebSearchEnabled bool

	// APIKey is the API key for the model provider (from env vars or auth.json)
	APIKey string

	// Profile is the active profile name used to derive this Config (if any)
	Profile string
}

// MCPServerConfig defines configuration for an MCP server
type MCPServerConfig struct {
	// Command is the executable to run for stdio transport
	Command string `toml:"command"`

	// Args are the arguments to pass to the command
	Args []string `toml:"args"`

	// Env contains environment variables to set for the command
	Env map[string]string `toml:"env"`

	// EnvVars lists environment variable names to pass through
	EnvVars []string `toml:"env_vars"`

	// CWD is the working directory for the command
	CWD string `toml:"cwd"`

	// URL is the endpoint for HTTP transport
	URL string `toml:"url"`

	// BearerTokenEnvVar is the name of the env var containing the bearer token
	BearerTokenEnvVar string `toml:"bearer_token_env_var"`

	// HTTPHeaders are additional HTTP headers for HTTP transport
	HTTPHeaders map[string]string `toml:"http_headers"`

	// EnvHTTPHeaders are HTTP headers where values come from env vars
	EnvHTTPHeaders map[string]string `toml:"env_http_headers"`

	// Enabled controls whether this MCP server is active
	Enabled bool `toml:"enabled"`

	// StartupTimeoutSec is the timeout for initializing the MCP server
	StartupTimeoutSec *float64 `toml:"startup_timeout_sec"`

	// ToolTimeoutSec is the default timeout for tool calls via this server
	ToolTimeoutSec *float64 `toml:"tool_timeout_sec"`

	// EnabledTools is an explicit allow-list of tools from this server
	EnabledTools []string `toml:"enabled_tools"`

	// DisabledTools is an explicit deny-list of tools from this server
	DisabledTools []string `toml:"disabled_tools"`

	// OAuth contains OAuth 2.0 configuration for authenticated servers
	OAuth *MCPOAuthConfig `toml:"oauth"`
}

// Validate checks if the MCPServerConfig is valid
func (m *MCPServerConfig) Validate() error {
	// Ensure exactly one transport type is configured
	hasCommand := m.Command != ""
	hasURL := m.URL != ""

	if !hasCommand && !hasURL {
		return fmt.Errorf("must specify either command or url")
	}
	if hasCommand && hasURL {
		return fmt.Errorf("cannot specify both command and url")
	}

	// Validate stdio transport
	if hasCommand {
		// Prevent path traversal in command
		if strings.Contains(m.Command, "..") {
			return fmt.Errorf("command cannot contain path traversal (..) : %s", m.Command)
		}
		// Validate command exists and is executable
		if !filepath.IsAbs(m.Command) {
			if _, err := exec.LookPath(m.Command); err != nil {
				return fmt.Errorf("command not found in PATH: %s", m.Command)
			}
		}
		// Validate CWD if specified
		if m.CWD != "" {
			if strings.Contains(m.CWD, "..") {
				return fmt.Errorf("cwd cannot contain path traversal (..): %s", m.CWD)
			}
			if info, err := os.Stat(m.CWD); err != nil {
				return fmt.Errorf("cwd does not exist: %s", m.CWD)
			} else if !info.IsDir() {
				return fmt.Errorf("cwd is not a directory: %s", m.CWD)
			}
		}
	}

	// Validate HTTP transport
	if hasURL {
		if _, err := url.Parse(m.URL); err != nil {
			return fmt.Errorf("invalid url: %w", err)
		}
		// Ensure URL uses https
		if u, _ := url.Parse(m.URL); u.Scheme != "https" && u.Scheme != "http" {
			return fmt.Errorf("url must use http or https scheme: %s", m.URL)
		}
	}

	// Validate timeouts
	if m.StartupTimeoutSec != nil && *m.StartupTimeoutSec <= 0 {
		return fmt.Errorf("startup_timeout_sec must be positive")
	}
	if m.ToolTimeoutSec != nil && *m.ToolTimeoutSec <= 0 {
		return fmt.Errorf("tool_timeout_sec must be positive")
	}

	// Validate OAuth config if present
	if m.OAuth != nil {
		if err := m.OAuth.Validate(); err != nil {
			return fmt.Errorf("oauth: %w", err)
		}
	}

	return nil
}

// MCPOAuthConfig defines OAuth 2.0 configuration for an MCP server
type MCPOAuthConfig struct {
	// ClientID is the OAuth 2.0 client identifier
	ClientID string `toml:"client_id"`

	// ClientSecret is the OAuth 2.0 client secret (optional for public clients)
	ClientSecret string `toml:"client_secret"`

	// AuthURL is the authorization endpoint (optional if using discovery)
	AuthURL string `toml:"auth_url"`

	// TokenURL is the token endpoint (optional if using discovery)
	TokenURL string `toml:"token_url"`

	// Scopes are the requested OAuth scopes
	Scopes []string `toml:"scopes"`

	// UseDiscovery enables automatic OAuth endpoint discovery
	UseDiscovery bool `toml:"use_discovery"`

	// UsePKCE enables PKCE (Proof Key for Code Exchange)
	UsePKCE bool `toml:"use_pkce"`
}

// Validate checks if the MCPOAuthConfig is valid
func (o *MCPOAuthConfig) Validate() error {
	if o.ClientID == "" {
		return fmt.Errorf("client_id cannot be empty")
	}

	// If using discovery, AuthURL and TokenURL should not be set
	if o.UseDiscovery {
		if o.AuthURL != "" || o.TokenURL != "" {
			return fmt.Errorf("cannot specify auth_url or token_url when use_discovery is true")
		}
	} else {
		// If not using discovery, both URLs must be set
		if o.AuthURL == "" || o.TokenURL == "" {
			return fmt.Errorf("auth_url and token_url are required when use_discovery is false")
		}
		// Validate URLs
		if _, err := url.Parse(o.AuthURL); err != nil {
			return fmt.Errorf("invalid auth_url: %w", err)
		}
		if _, err := url.Parse(o.TokenURL); err != nil {
			return fmt.Errorf("invalid token_url: %w", err)
		}
	}

	// Scopes should not be empty
	if len(o.Scopes) == 0 {
		return fmt.Errorf("scopes cannot be empty")
	}

	return nil
}

// NotifyConfig contains notification configurations.
type NotifyConfig struct {
	// OnTurnComplete is triggered when a turn completes successfully
	OnTurnComplete *NotifyTriggerConfig `toml:"on_turn_complete"`

	// OnError is triggered when a turn encounters an error
	OnError *NotifyTriggerConfig `toml:"on_error"`

	// OnApprovalNeeded is triggered when user approval is required
	OnApprovalNeeded *NotifyTriggerConfig `toml:"on_approval_needed"`

	// OnTurnAborted is triggered when a turn is aborted
	OnTurnAborted *NotifyTriggerConfig `toml:"on_turn_aborted"`

	// ScriptTimeoutSec is the maximum time scripts can run in seconds
	ScriptTimeoutSec *float64 `toml:"script_timeout_sec"`
}

// NotifyTriggerConfig defines configuration for a notification trigger.
type NotifyTriggerConfig struct {
	// Command is the script command to execute
	Command string `toml:"command"`

	// Enabled controls whether this notification is active
	Enabled bool `toml:"enabled"`

	// Env contains additional environment variables for the script
	Env map[string]string `toml:"env"`
}

// Validate checks if the NotifyConfig is valid
func (n *NotifyConfig) Validate() error {
	if n.ScriptTimeoutSec != nil && *n.ScriptTimeoutSec <= 0 {
		return fmt.Errorf("script_timeout_sec must be positive")
	}

	if n.OnTurnComplete != nil {
		if err := n.OnTurnComplete.Validate(); err != nil {
			return fmt.Errorf("on_turn_complete: %w", err)
		}
	}
	if n.OnError != nil {
		if err := n.OnError.Validate(); err != nil {
			return fmt.Errorf("on_error: %w", err)
		}
	}
	if n.OnApprovalNeeded != nil {
		if err := n.OnApprovalNeeded.Validate(); err != nil {
			return fmt.Errorf("on_approval_needed: %w", err)
		}
	}
	if n.OnTurnAborted != nil {
		if err := n.OnTurnAborted.Validate(); err != nil {
			return fmt.Errorf("on_turn_aborted: %w", err)
		}
	}

	return nil
}

// Validate checks if the NotifyTriggerConfig is valid
func (t *NotifyTriggerConfig) Validate() error {
	if t.Enabled && t.Command == "" {
		return fmt.Errorf("command cannot be empty when enabled is true")
	}

	if t.Command != "" {
		// Prevent path traversal
		if strings.Contains(t.Command, "..") {
			return fmt.Errorf("command cannot contain path traversal (..): %s", t.Command)
		}
	}

	// Validate environment variable keys
	for key := range t.Env {
		if key == "" {
			return fmt.Errorf("environment variable key cannot be empty")
		}
		if strings.ContainsAny(key, "=\x00") {
			return fmt.Errorf("environment variable key contains invalid characters: %s", key)
		}
	}

	return nil
}

// configTOML represents the raw configuration structure from TOML file
type configTOML struct {
	Model                       *string                    `toml:"model"`
	ReviewModel                 *string                    `toml:"review_model"`
	ModelProvider               *string                    `toml:"model_provider"`
	ModelContextWindow          *int64                     `toml:"model_context_window"`
	ModelMaxOutputTokens        *int64                     `toml:"model_max_output_tokens"`
	ModelAutoCompactTokenLimit  *int64                     `toml:"model_auto_compact_token_limit"`
	ApprovalPolicy              *string                    `toml:"approval_policy"`
	HideAgentReasoning          *bool                      `toml:"hide_agent_reasoning"`
	ShowRawAgentReasoning       *bool                      `toml:"show_raw_agent_reasoning"`
	ChatGPTBaseURL              *string                    `toml:"chatgpt_base_url"`
	ForcedChatGPTWorkspaceID    *string                    `toml:"forced_chatgpt_workspace_id"`
	ProjectDocMaxBytes          *int                       `toml:"project_doc_max_bytes"`
	ProjectDocFallbackFilenames []string                   `toml:"project_doc_fallback_filenames"`
	DisablePasteBurst           *bool                      `toml:"disable_paste_burst"`
	MCPServers                  map[string]MCPServerConfig `toml:"mcp_servers"`
	Notify                      *NotifyConfig              `toml:"notify"`
	WebSearchProvider           *string                    `toml:"web_search_provider"`
	WebSearchEnabled            *bool                      `toml:"web_search_enabled"`
	Profile                     *string                    `toml:"profile"`
}

// LoadConfig loads configuration from ~/.codex/config.toml and environment variables.
// Environment variables take precedence over file-based configuration.
// If the config file doesn't exist, default values are used.
func LoadConfig() (*Config, error) {
	// Determine CODEX_HOME
	codexHome, err := findCodexHome()
	if err != nil {
		return nil, fmt.Errorf("failed to determine CODEX_HOME: %w", err)
	}

	// Initialize config with defaults
	cfg := &Config{
		Model:                       getDefaultModel(),
		ReviewModel:                 "gpt-5-codex",
		ModelProvider:               "openai-responses",
		ChatGPTBaseURL:              "https://chatgpt.com",
		CodexHome:                   codexHome,
		ProjectDocMaxBytes:          32 * 1024, // 32 KiB
		ProjectDocFallbackFilenames: []string{},
		MCPServers:                  make(map[string]MCPServerConfig),
		WebSearchProvider:           "duckduckgo",
		WebSearchEnabled:            false, // Disabled by default
	}

	// Load from TOML file if it exists
	configPath := filepath.Join(codexHome, "config.toml")
	if _, err := os.Stat(configPath); err == nil {
		if err := loadTOMLConfig(configPath, cfg); err != nil {
			return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to check config file: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(cfg)

	return cfg, nil
}

// loadTOMLConfig loads configuration from a TOML file and merges with existing config
func loadTOMLConfig(path string, cfg *Config) error {
	var tomlCfg configTOML

	if _, err := toml.DecodeFile(path, &tomlCfg); err != nil {
		return err
	}

	// Apply TOML values, only overriding if set
	if tomlCfg.Model != nil {
		cfg.Model = *tomlCfg.Model
	}
	if tomlCfg.ReviewModel != nil {
		cfg.ReviewModel = *tomlCfg.ReviewModel
	}
	if tomlCfg.ModelProvider != nil {
		cfg.ModelProvider = *tomlCfg.ModelProvider
	}
	if tomlCfg.ModelContextWindow != nil {
		cfg.ModelContextWindow = tomlCfg.ModelContextWindow
	}
	if tomlCfg.ModelMaxOutputTokens != nil {
		cfg.ModelMaxOutputTokens = tomlCfg.ModelMaxOutputTokens
	}
	if tomlCfg.ModelAutoCompactTokenLimit != nil {
		cfg.ModelAutoCompactTokenLimit = tomlCfg.ModelAutoCompactTokenLimit
	}
	if tomlCfg.ApprovalPolicy != nil {
		cfg.ApprovalPolicy = *tomlCfg.ApprovalPolicy
	}
	if tomlCfg.HideAgentReasoning != nil {
		cfg.HideAgentReasoning = *tomlCfg.HideAgentReasoning
	}
	if tomlCfg.ShowRawAgentReasoning != nil {
		cfg.ShowRawAgentReasoning = *tomlCfg.ShowRawAgentReasoning
	}
	if tomlCfg.ChatGPTBaseURL != nil {
		cfg.ChatGPTBaseURL = *tomlCfg.ChatGPTBaseURL
	}
	if tomlCfg.ForcedChatGPTWorkspaceID != nil {
		cfg.ForcedChatGPTWorkspaceID = *tomlCfg.ForcedChatGPTWorkspaceID
	}
	if tomlCfg.ProjectDocMaxBytes != nil {
		cfg.ProjectDocMaxBytes = *tomlCfg.ProjectDocMaxBytes
	}
	if tomlCfg.ProjectDocFallbackFilenames != nil {
		cfg.ProjectDocFallbackFilenames = tomlCfg.ProjectDocFallbackFilenames
	}
	if tomlCfg.DisablePasteBurst != nil {
		cfg.DisablePasteBurst = *tomlCfg.DisablePasteBurst
	}
	if tomlCfg.MCPServers != nil {
		cfg.MCPServers = tomlCfg.MCPServers
		// Set default enabled=true if not specified
		for name, server := range cfg.MCPServers {
			// Default enabled to true if not explicitly set
			// (In TOML, we can't distinguish between false and unset,
			// so we assume if Command or URL is set, it's meant to be enabled by default)
			// Note: Currently no default logic needed - servers are enabled explicitly in config
			cfg.MCPServers[name] = server
		}
	}
	if tomlCfg.Notify != nil {
		cfg.Notify = *tomlCfg.Notify
	}
	if tomlCfg.Profile != nil {
		cfg.Profile = *tomlCfg.Profile
	}
	if tomlCfg.WebSearchProvider != nil {
		cfg.WebSearchProvider = *tomlCfg.WebSearchProvider
	}
	if tomlCfg.WebSearchEnabled != nil {
		cfg.WebSearchEnabled = *tomlCfg.WebSearchEnabled
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides to the configuration
func applyEnvOverrides(cfg *Config) {
	// CODEX_MODEL overrides model selection
	if model := os.Getenv("CODEX_MODEL"); model != "" {
		cfg.Model = model
	}

	// CODEX_APPROVAL_POLICY overrides approval policy
	if policy := os.Getenv("CODEX_APPROVAL_POLICY"); policy != "" {
		cfg.ApprovalPolicy = policy
	}

	// CODEX_BASE_URL overrides ChatGPT base URL
	if baseURL := os.Getenv("CODEX_BASE_URL"); baseURL != "" {
		cfg.ChatGPTBaseURL = baseURL
	}

	// API key priority: CODEX_API_KEY > OPENAI_API_KEY
	if apiKey := os.Getenv("CODEX_API_KEY"); apiKey != "" {
		cfg.APIKey = apiKey
	} else if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		cfg.APIKey = apiKey
	}
}

// findCodexHome returns the path to the Codex configuration directory.
// It checks the CODEX_HOME environment variable first, then falls back to ~/.codex.
func findCodexHome() (string, error) {
	// Honor CODEX_HOME environment variable when set
	if codexHome := os.Getenv("CODEX_HOME"); codexHome != "" {
		// Canonicalize the path if it exists
		if info, err := os.Stat(codexHome); err == nil && info.IsDir() {
			// Path exists and is a directory
			absPath, err := filepath.Abs(codexHome)
			if err != nil {
				return "", fmt.Errorf("failed to get absolute path: %w", err)
			}
			return absPath, nil
		} else if err == nil {
			return "", fmt.Errorf("CODEX_HOME is not a directory: %s", codexHome)
		}
		// If path doesn't exist, return it as-is (will be created later if needed)
		return codexHome, nil
	}

	// Fall back to ~/.codex
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(homeDir, ".codex"), nil
}

// getDefaultModel returns the default model based on the platform.
// Matches the Rust implementation: gpt-5 on Windows, gpt-5-codex elsewhere.
func getDefaultModel() string {
	if runtime.GOOS == "windows" {
		return "gpt-5"
	}
	return "gpt-5-codex"
}

// Validate checks that the configuration is valid and returns an error if not.
func (c *Config) Validate() error {
	if c.Model == "" {
		return fmt.Errorf("configuration error: 'model' cannot be empty. Set CODEX_MODEL environment variable or add 'model' to config.toml")
	}

	// Validate model exists in registry
	if err := models.ValidateModel(c.Model); err != nil {
		return fmt.Errorf("configuration error: invalid model '%s': %w. Run 'codex models list' to see available models", c.Model, err)
	}

	if c.ReviewModel == "" {
		return fmt.Errorf("configuration error: 'review_model' cannot be empty. Add 'review_model' to config.toml")
	}

	// Validate review model exists in registry
	if err := models.ValidateModel(c.ReviewModel); err != nil {
		return fmt.Errorf("configuration error: invalid review_model '%s': %w. Run 'codex models list' to see available models", c.ReviewModel, err)
	}

	if c.ModelProvider == "" {
		return fmt.Errorf("configuration error: 'model_provider' cannot be empty. Add 'model_provider' to config.toml (e.g., 'openai-responses')")
	}

	if c.CodexHome == "" {
		return fmt.Errorf("configuration error: 'codex_home' cannot be empty. Set CODEX_HOME environment variable or let it default to ~/.codex")
	}

	if c.ChatGPTBaseURL == "" {
		return fmt.Errorf("configuration error: 'chatgpt_base_url' cannot be empty. Set CODEX_BASE_URL or add 'chatgpt_base_url' to config.toml")
	}

	if c.ProjectDocMaxBytes <= 0 {
		return fmt.Errorf("configuration error: 'project_doc_max_bytes' must be positive (got: %d). Set a value like 32768 (32 KiB)", c.ProjectDocMaxBytes)
	}

	// Validate approval policy if set
	if c.ApprovalPolicy != "" {
		validPolicies := map[string]bool{
			"auto":   true,
			"always": true,
			"never":  true,
		}
		if !validPolicies[c.ApprovalPolicy] {
			return fmt.Errorf("configuration error: invalid approval_policy '%s'. Must be one of: 'auto', 'always', or 'never'", c.ApprovalPolicy)
		}
	}

	// Validate MCP servers
	for name, server := range c.MCPServers {
		if err := server.Validate(); err != nil {
			return fmt.Errorf("mcp_servers[%s]: %w", name, err)
		}
	}

	// Validate notification config (always validates, even if empty struct)
	if err := c.Notify.Validate(); err != nil {
		return fmt.Errorf("notify: %w", err)
	}

	// Validate WebSearchProvider if enabled
	if c.WebSearchEnabled {
		if c.WebSearchProvider == "" {
			return fmt.Errorf("web_search_provider cannot be empty when web_search_enabled is true")
		}
		validProviders := map[string]bool{
			"duckduckgo": true,
			"google":     true,
		}
		if !validProviders[c.WebSearchProvider] {
			return fmt.Errorf("invalid web_search_provider '%s'. Must be one of: duckduckgo, google", c.WebSearchProvider)
		}
	}

	// Validate ProjectDocMaxBytes has reasonable limits
	const MaxProjectDocSize = 10 * 1024 * 1024 // 10MB
	if c.ProjectDocMaxBytes > MaxProjectDocSize {
		return fmt.Errorf("project_doc_max_bytes exceeds maximum of %d bytes (10MB)", MaxProjectDocSize)
	}

	return nil
}

// CodexHomeDir returns the codex home directory.
// This is a convenience function for accessing the CodexHome field.
func (c *Config) CodexHomeDir() string {
	return c.CodexHome
}

// GetConfigPath returns the path to the config.toml file.
func (c *Config) GetConfigPath() string {
	return filepath.Join(c.CodexHome, "config.toml")
}

// GetHistoryPath returns the path to the history.jsonl file.
func (c *Config) GetHistoryPath() string {
	return filepath.Join(c.CodexHome, "history.jsonl")
}

// GetAuthPath returns the path to the auth.json file.
func (c *Config) GetAuthPath() string {
	return filepath.Join(c.CodexHome, "auth.json")
}
