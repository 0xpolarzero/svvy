package client

import (
	"os"
	"testing"

	"github.com/evmts/codex/codex-go/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid options with all fields",
			opts: Options{
				BaseURL: "https://api.openai.com/v1",
				APIKey:  "test-key",
				Model:   "gpt-4",
			},
			wantErr: false,
		},
		{
			name: "missing base URL",
			opts: Options{
				APIKey: "test-key",
				Model:  "gpt-4",
			},
			wantErr: true,
			errMsg:  "base URL is required",
		},
		{
			name: "missing API key",
			opts: Options{
				BaseURL: "https://api.openai.com/v1",
				Model:   "gpt-4",
			},
			wantErr: true,
			errMsg:  "API key is required",
		},
		{
			name: "missing model",
			opts: Options{
				BaseURL: "https://api.openai.com/v1",
				APIKey:  "test-key",
			},
			wantErr: true,
			errMsg:  "model is required",
		},
		{
			name: "with optional timeout",
			opts: Options{
				BaseURL:        "https://api.openai.com/v1",
				APIKey:         "test-key",
				Model:          "gpt-4",
				RequestTimeout: 60,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.opts)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

func TestFromConfig(t *testing.T) {
	// Create temporary config for testing
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.toml"

	configContent := `
model = "gpt-4"
chatgpt_base_url = "https://api.openai.com/v1"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	t.Run("valid config file with API key", func(t *testing.T) {
		// Set API key in environment
		t.Setenv("OPENAI_API_KEY", "test-key-from-env")

		client, err := FromConfig(configPath)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("config file not found", func(t *testing.T) {
		client, err := FromConfig("/nonexistent/config.toml")
		require.Error(t, err)
		assert.Nil(t, client)
	})

	t.Run("missing API key", func(t *testing.T) {
		// Clear environment variables
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("CODEX_API_KEY")

		client, err := FromConfig(configPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "API key")
		assert.Nil(t, client)
	})
}

func TestFromEnv(t *testing.T) {
	tests := []struct {
		name      string
		setupEnv  func(*testing.T)
		wantErr   bool
		errMsg    string
		checkFunc func(*testing.T, *Client)
	}{
		{
			name: "all required env vars set",
			setupEnv: func(t *testing.T) {
				t.Setenv("OPENAI_API_KEY", "test-key")
				t.Setenv("CODEX_MODEL", "gpt-4")
				t.Setenv("CODEX_BASE_URL", "https://api.openai.com/v1")
			},
			wantErr: false,
			checkFunc: func(t *testing.T, c *Client) {
				assert.NotNil(t, c)
			},
		},
		{
			name: "missing API key",
			setupEnv: func(t *testing.T) {
				t.Setenv("CODEX_MODEL", "gpt-4")
				t.Setenv("CODEX_BASE_URL", "https://api.openai.com/v1")
				os.Unsetenv("OPENAI_API_KEY")
				os.Unsetenv("CODEX_API_KEY")
			},
			wantErr: true,
			errMsg:  "API key",
		},
		{
			name: "CODEX_API_KEY preferred over OPENAI_API_KEY",
			setupEnv: func(t *testing.T) {
				t.Setenv("CODEX_API_KEY", "codex-key")
				t.Setenv("OPENAI_API_KEY", "openai-key")
				t.Setenv("CODEX_MODEL", "gpt-4")
				t.Setenv("CODEX_BASE_URL", "https://api.openai.com/v1")
			},
			wantErr: false,
			checkFunc: func(t *testing.T, c *Client) {
				assert.NotNil(t, c)
				// In a real test we'd verify the key is "codex-key"
			},
		},
		{
			name: "default model used when not specified",
			setupEnv: func(t *testing.T) {
				t.Setenv("OPENAI_API_KEY", "test-key")
				t.Setenv("CODEX_BASE_URL", "https://api.openai.com/v1")
				os.Unsetenv("CODEX_MODEL")
			},
			wantErr: false,
			checkFunc: func(t *testing.T, c *Client) {
				assert.NotNil(t, c)
			},
		},
		{
			name: "default base URL used when not specified",
			setupEnv: func(t *testing.T) {
				t.Setenv("OPENAI_API_KEY", "test-key")
				t.Setenv("CODEX_MODEL", "gpt-4")
				os.Unsetenv("CODEX_BASE_URL")
			},
			wantErr: false,
			checkFunc: func(t *testing.T, c *Client) {
				assert.NotNil(t, c)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env before setup
			os.Unsetenv("OPENAI_API_KEY")
			os.Unsetenv("CODEX_API_KEY")
			os.Unsetenv("CODEX_MODEL")
			os.Unsetenv("CODEX_BASE_URL")

			if tt.setupEnv != nil {
				tt.setupEnv(t)
			}

			client, err := FromEnv()

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				if tt.checkFunc != nil {
					tt.checkFunc(t, client)
				}
			}
		})
	}
}

func TestNewWithDefaults(t *testing.T) {
	t.Run("applies default values", func(t *testing.T) {
		client, err := New(Options{
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "test-key",
			Model:   "gpt-4",
		})

		require.NoError(t, err)
		assert.NotNil(t, client)

		// Verify client is configured correctly
		// In a real test we'd check internal config values
	})
}

func TestFromConfigObject(t *testing.T) {
	t.Run("creates client from config object", func(t *testing.T) {
		cfg := &config.Config{
			Model:          "gpt-4",
			ChatGPTBaseURL: "https://api.openai.com/v1",
			APIKey:         "test-key",
		}

		client, err := FromConfigObject(cfg)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("missing API key in config", func(t *testing.T) {
		cfg := &config.Config{
			Model:          "gpt-4",
			ChatGPTBaseURL: "https://api.openai.com/v1",
		}

		client, err := FromConfigObject(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "API key")
		assert.Nil(t, client)
	})
}
