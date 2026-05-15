package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultValues verifies that LoadConfig returns correct default values
// when no config file exists and no environment variables are set
func TestDefaultValues(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Set CODEX_HOME to our temp directory
	oldHome := os.Getenv("CODEX_HOME")
	os.Setenv("CODEX_HOME", tmpDir)
	defer os.Setenv("CODEX_HOME", oldHome)

	// Ensure no env vars interfere
	clearTestEnvVars(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Verify default values matching Rust implementation
	if cfg.Model != getDefaultModel() {
		t.Errorf("Model = %q, want %q", cfg.Model, getDefaultModel())
	}

	if cfg.ReviewModel != "gpt-5-codex" {
		t.Errorf("ReviewModel = %q, want %q", cfg.ReviewModel, "gpt-5-codex")
	}

	if cfg.ModelProvider != "openai-responses" {
		t.Errorf("ModelProvider = %q, want %q", cfg.ModelProvider, "openai-responses")
	}

	if cfg.ChatGPTBaseURL != "https://chatgpt.com" {
		t.Errorf("ChatGPTBaseURL = %q, want %q", cfg.ChatGPTBaseURL, "https://chatgpt.com")
	}

	if cfg.CodexHome != tmpDir {
		t.Errorf("CodexHome = %q, want %q", cfg.CodexHome, tmpDir)
	}

	if cfg.ProjectDocMaxBytes != 32*1024 {
		t.Errorf("ProjectDocMaxBytes = %d, want %d", cfg.ProjectDocMaxBytes, 32*1024)
	}
}

// TestTOMLLoading verifies that configuration is correctly loaded from TOML file
func TestTOMLLoading(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test config file
	configContent := `
model = "gpt-5"
review_model = "gpt-5-codex"
model_provider = "openai-chat-completions"
approval_policy = "never"
hide_agent_reasoning = true
`
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	oldHome := os.Getenv("CODEX_HOME")
	os.Setenv("CODEX_HOME", tmpDir)
	defer os.Setenv("CODEX_HOME", oldHome)

	clearTestEnvVars(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if cfg.Model != "gpt-5" {
		t.Errorf("Model = %q, want %q", cfg.Model, "gpt-5")
	}

	if cfg.ReviewModel != "gpt-5-codex" {
		t.Errorf("ReviewModel = %q, want %q", cfg.ReviewModel, "gpt-5-codex")
	}

	if cfg.ModelProvider != "openai-chat-completions" {
		t.Errorf("ModelProvider = %q, want %q", cfg.ModelProvider, "openai-chat-completions")
	}

	if !cfg.HideAgentReasoning {
		t.Error("HideAgentReasoning should be true")
	}
}

// TestEnvVarOverrides verifies that environment variables override config file values
func TestEnvVarOverrides(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config file with one set of values
	configContent := `
model = "gpt-5"
`
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	oldHome := os.Getenv("CODEX_HOME")
	os.Setenv("CODEX_HOME", tmpDir)
	defer os.Setenv("CODEX_HOME", oldHome)

	// Set environment variables that should override
	os.Setenv("CODEX_MODEL", "gpt-5-codex")
	defer os.Unsetenv("CODEX_MODEL")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Environment variable should override config file
	if cfg.Model != "gpt-5-codex" {
		t.Errorf("Model = %q, want %q (env var should override)", cfg.Model, "gpt-5-codex")
	}
}

// TestAPIKeyFromEnv verifies that API keys are read from environment variables
func TestAPIKeyFromEnv(t *testing.T) {
	tmpDir := t.TempDir()

	oldHome := os.Getenv("CODEX_HOME")
	os.Setenv("CODEX_HOME", tmpDir)
	defer os.Setenv("CODEX_HOME", oldHome)

	// Set API key in environment
	testKey := "sk-test-key-12345"
	os.Setenv("OPENAI_API_KEY", testKey)
	defer os.Unsetenv("OPENAI_API_KEY")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if cfg.APIKey != testKey {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, testKey)
	}
}

// TestCodexAPIKeyOverridesOpenAI verifies CODEX_API_KEY takes precedence
func TestCodexAPIKeyOverridesOpenAI(t *testing.T) {
	tmpDir := t.TempDir()

	oldHome := os.Getenv("CODEX_HOME")
	os.Setenv("CODEX_HOME", tmpDir)
	defer os.Setenv("CODEX_HOME", oldHome)

	// Set both API keys
	os.Setenv("OPENAI_API_KEY", "sk-openai")
	defer os.Unsetenv("OPENAI_API_KEY")

	os.Setenv("CODEX_API_KEY", "sk-codex")
	defer os.Unsetenv("CODEX_API_KEY")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// CODEX_API_KEY should take precedence
	if cfg.APIKey != "sk-codex" {
		t.Errorf("APIKey = %q, want %q (CODEX_API_KEY should override)", cfg.APIKey, "sk-codex")
	}
}

// TestMissingConfigFile verifies graceful handling when config file doesn't exist
func TestMissingConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	oldHome := os.Getenv("CODEX_HOME")
	os.Setenv("CODEX_HOME", tmpDir)
	defer os.Setenv("CODEX_HOME", oldHome)

	clearTestEnvVars(t)

	// Should not error when config file is missing
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() should not error on missing config file: %v", err)
	}

	// Should have default values
	if cfg.Model != getDefaultModel() {
		t.Errorf("Model = %q, want default %q", cfg.Model, getDefaultModel())
	}
}

// TestInvalidTOML verifies proper error handling for invalid TOML
func TestInvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invalid config file
	configContent := `
model = "gpt-5"
invalid toml syntax here
`
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	oldHome := os.Getenv("CODEX_HOME")
	os.Setenv("CODEX_HOME", tmpDir)
	defer os.Setenv("CODEX_HOME", oldHome)

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig() should error on invalid TOML")
	}
}

// TestCodexHomeEnvVar verifies CODEX_HOME environment variable is respected
func TestCodexHomeEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	customDir := filepath.Join(tmpDir, "custom-codex-home")
	if err := os.MkdirAll(customDir, 0755); err != nil {
		t.Fatalf("Failed to create custom dir: %v", err)
	}

	oldHome := os.Getenv("CODEX_HOME")
	os.Setenv("CODEX_HOME", customDir)
	defer os.Setenv("CODEX_HOME", oldHome)

	clearTestEnvVars(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if cfg.CodexHome != customDir {
		t.Errorf("CodexHome = %q, want %q", cfg.CodexHome, customDir)
	}
}

// TestDefaultCodexHome verifies default ~/.codex is used when CODEX_HOME is not set
func TestDefaultCodexHome(t *testing.T) {
	oldHome := os.Getenv("CODEX_HOME")
	os.Unsetenv("CODEX_HOME")
	defer os.Setenv("CODEX_HOME", oldHome)

	clearTestEnvVars(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}

	expectedCodexHome := filepath.Join(homeDir, ".codex")
	if cfg.CodexHome != expectedCodexHome {
		t.Errorf("CodexHome = %q, want %q", cfg.CodexHome, expectedCodexHome)
	}
}

// TestValidation verifies config validation rules
func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Model:              "gpt-5-codex",
				ReviewModel:        "gpt-5-codex",
				ModelProvider:      "openai-responses",
				CodexHome:          "/tmp/codex",
				ProjectDocMaxBytes: 32 * 1024,
				ChatGPTBaseURL:     "https://chatgpt.com",
			},
			wantErr: false,
		},
		{
			name: "empty model",
			config: Config{
				Model:              "",
				ReviewModel:        "gpt-5-codex",
				ModelProvider:      "openai-responses",
				CodexHome:          "/tmp/codex",
				ProjectDocMaxBytes: 32 * 1024,
				ChatGPTBaseURL:     "https://chatgpt.com",
			},
			wantErr: true,
		},
		{
			name: "empty codex home",
			config: Config{
				Model:              "gpt-5-codex",
				ReviewModel:        "gpt-5-codex",
				ModelProvider:      "openai-responses",
				CodexHome:          "",
				ProjectDocMaxBytes: 32 * 1024,
				ChatGPTBaseURL:     "https://chatgpt.com",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestComplexTOMLConfig tests loading a more complex configuration
func TestComplexTOMLConfig(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `
model = "gpt-5-codex"
review_model = "gpt-5-codex"
model_provider = "openai-responses"
model_context_window = 200000
model_max_output_tokens = 8192
approval_policy = "auto"
hide_agent_reasoning = false
show_raw_agent_reasoning = true
chatgpt_base_url = "https://custom.chatgpt.com"
project_doc_max_bytes = 65536
disable_paste_burst = true

[mcp_servers.filesystem]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
enabled = true

[mcp_servers.git]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-git"]
enabled = false
`

	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	oldHome := os.Getenv("CODEX_HOME")
	os.Setenv("CODEX_HOME", tmpDir)
	defer os.Setenv("CODEX_HOME", oldHome)

	clearTestEnvVars(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if cfg.Model != "gpt-5-codex" {
		t.Errorf("Model = %q, want %q", cfg.Model, "gpt-5-codex")
	}

	if cfg.ModelContextWindow == nil || *cfg.ModelContextWindow != 200000 {
		t.Errorf("ModelContextWindow = %v, want 200000", cfg.ModelContextWindow)
	}

	if cfg.ModelMaxOutputTokens == nil || *cfg.ModelMaxOutputTokens != 8192 {
		t.Errorf("ModelMaxOutputTokens = %v, want 8192", cfg.ModelMaxOutputTokens)
	}

	if cfg.ChatGPTBaseURL != "https://custom.chatgpt.com" {
		t.Errorf("ChatGPTBaseURL = %q, want %q", cfg.ChatGPTBaseURL, "https://custom.chatgpt.com")
	}

	if cfg.ProjectDocMaxBytes != 65536 {
		t.Errorf("ProjectDocMaxBytes = %d, want %d", cfg.ProjectDocMaxBytes, 65536)
	}

	if !cfg.DisablePasteBurst {
		t.Error("DisablePasteBurst should be true")
	}

	if len(cfg.MCPServers) != 2 {
		t.Errorf("MCPServers length = %d, want 2", len(cfg.MCPServers))
	}

	fs, ok := cfg.MCPServers["filesystem"]
	if !ok {
		t.Fatal("filesystem MCP server not found")
	}
	if fs.Command != "npx" {
		t.Errorf("filesystem command = %q, want %q", fs.Command, "npx")
	}
	if !fs.Enabled {
		t.Error("filesystem should be enabled")
	}

	git, ok := cfg.MCPServers["git"]
	if !ok {
		t.Fatal("git MCP server not found")
	}
	if git.Enabled {
		t.Error("git should be disabled")
	}
}

// TestGoldenFullConfig tests loading a comprehensive configuration file
func TestGoldenFullConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Copy the golden config file to temp directory
	goldenPath := filepath.Join("testdata", "full_config.toml")
	goldenData, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("Failed to read golden config: %v", err)
	}

	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, goldenData, 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	oldHome := os.Getenv("CODEX_HOME")
	os.Setenv("CODEX_HOME", tmpDir)
	defer os.Setenv("CODEX_HOME", oldHome)

	clearTestEnvVars(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Verify key values from golden file
	if cfg.Model != "gpt-5-codex" {
		t.Errorf("Model = %q, want %q", cfg.Model, "gpt-5-codex")
	}

	if cfg.ModelContextWindow == nil || *cfg.ModelContextWindow != 200000 {
		t.Errorf("ModelContextWindow = %v, want 200000", cfg.ModelContextWindow)
	}

	if cfg.ApprovalPolicy != "auto" {
		t.Errorf("ApprovalPolicy = %q, want %q", cfg.ApprovalPolicy, "auto")
	}

	// Verify MCP servers
	expectedServers := []string{"filesystem", "git", "external_api", "disabled_server"}
	for _, name := range expectedServers {
		if _, ok := cfg.MCPServers[name]; !ok {
			t.Errorf("MCP server %q not found", name)
		}
	}

	// Check filesystem server details
	fs := cfg.MCPServers["filesystem"]
	if fs.Command != "npx" {
		t.Errorf("filesystem command = %q, want %q", fs.Command, "npx")
	}
	if !fs.Enabled {
		t.Error("filesystem should be enabled")
	}
	if fs.Env["NODE_ENV"] != "production" {
		t.Errorf("filesystem NODE_ENV = %q, want %q", fs.Env["NODE_ENV"], "production")
	}

	// Check disabled server
	disabled := cfg.MCPServers["disabled_server"]
	if disabled.Enabled {
		t.Error("disabled_server should be disabled")
	}

	// Check HTTP server
	api := cfg.MCPServers["external_api"]
	if api.URL != "https://api.example.com/mcp" {
		t.Errorf("external_api URL = %q, want %q", api.URL, "https://api.example.com/mcp")
	}
	if api.BearerTokenEnvVar != "EXTERNAL_API_TOKEN" {
		t.Errorf("external_api bearer_token_env_var = %q, want %q", api.BearerTokenEnvVar, "EXTERNAL_API_TOKEN")
	}
}

// clearTestEnvVars clears environment variables that might interfere with tests
func clearTestEnvVars(t *testing.T) {
	t.Helper()

	envVars := []string{
		"CODEX_MODEL",
		"CODEX_API_KEY",
		"OPENAI_API_KEY",
		"CODEX_APPROVAL_POLICY",
		"CODEX_BASE_URL",
	}

	for _, v := range envVars {
		os.Unsetenv(v)
	}
}
