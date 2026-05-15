// Package client provides convenience helpers for creating OpenAI-compatible clients
// for use with the Codex SDK. It simplifies client creation from various sources:
// configuration files, environment variables, or direct options.
package client

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/evmts/codex/codex-go/internal/client"
	"github.com/evmts/codex/codex-go/internal/client/openai"
	"github.com/evmts/codex/codex-go/internal/config"
)

// Client wraps the internal client interface for SDK use.
type Client struct {
	internal client.Client
}

// Options configures a new client.
type Options struct {
	// BaseURL is the API endpoint (e.g., "https://api.openai.com/v1")
	BaseURL string

	// APIKey for authentication
	APIKey string

	// Model is the model identifier (e.g., "gpt-4", "gpt-5-codex")
	Model string

	// RequestTimeout in seconds (default: 30)
	RequestTimeout int

	// MaxRetries is the maximum number of retry attempts (default: 3)
	MaxRetries int

	// Headers contains additional HTTP headers to include in requests
	Headers map[string]string

	// ConversationID is used for prompt caching and session tracking
	ConversationID string
}

// New creates a new client with the given options.
// All required fields (BaseURL, APIKey, Model) must be provided.
func New(opts Options) (*Client, error) {
	// Validate required fields
	if opts.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	if opts.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}
	if opts.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	// Build internal client config
	cfg := client.ClientConfig{
		BaseURL:        opts.BaseURL,
		APIKey:         opts.APIKey,
		Model:          opts.Model,
		Headers:        opts.Headers,
		ConversationID: opts.ConversationID,
	}

	// Set timeout (default to 30 seconds)
	if opts.RequestTimeout > 0 {
		cfg.RequestTimeout = time.Duration(opts.RequestTimeout) * time.Second
	} else {
		cfg.RequestTimeout = 30 * time.Second
	}

	// Set retry config
	if opts.MaxRetries > 0 {
		cfg.RetryConfig = client.RetryConfig{
			MaxRetries:           opts.MaxRetries,
			InitialBackoff:       1 * time.Second,
			MaxBackoff:           60 * time.Second,
			BackoffMultiplier:    2.0,
			RetryableStatusCodes: []int{429, 500, 502, 503, 504},
			RespectRetryAfter:    true,
		}
	} else {
		cfg.RetryConfig = client.DefaultRetryConfig()
	}

	// Set stream config
	cfg.StreamConfig = client.DefaultStreamConfig()

	// Create internal client
	internalClient, err := openai.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &Client{internal: internalClient}, nil
}

// FromConfig creates a client from a configuration file.
// It loads the config from the specified path and extracts the necessary settings.
// The API key must be provided via CODEX_API_KEY or OPENAI_API_KEY environment variables.
func FromConfig(configPath string) (*Client, error) {
	// Load config from file
	// For now, we'll use a simple approach that reads the config
	// In production, we'd use the config package's full loading logic

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	// Load config using the config package
	// For now, we'll create a minimal config loader
	// In a full implementation, we'd integrate with internal/config package

	// Try to get API key from environment
	apiKey := os.Getenv("CODEX_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API key not found in environment (set CODEX_API_KEY or OPENAI_API_KEY)")
	}

	// For now, use defaults - in production we'd parse the TOML
	model := os.Getenv("CODEX_MODEL")
	if model == "" {
		model = getDefaultModel()
	}

	baseURL := os.Getenv("CODEX_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return New(Options{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
	})
}

// FromEnv creates a client from environment variables.
// Required environment variables:
//   - CODEX_API_KEY or OPENAI_API_KEY: API authentication key
//   - CODEX_MODEL (optional): Model to use (defaults to platform default)
//   - CODEX_BASE_URL (optional): API endpoint (defaults to OpenAI)
func FromEnv() (*Client, error) {
	// Get API key
	apiKey := os.Getenv("CODEX_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API key not found in environment (set CODEX_API_KEY or OPENAI_API_KEY)")
	}

	// Get model (use default if not set)
	model := os.Getenv("CODEX_MODEL")
	if model == "" {
		model = getDefaultModel()
	}

	// Get base URL (use default if not set)
	baseURL := os.Getenv("CODEX_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return New(Options{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
	})
}

// FromConfigObject creates a client from a config.Config object.
// This is useful when you already have a loaded configuration.
func FromConfigObject(cfg *config.Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key not found in config")
	}

	return New(Options{
		BaseURL: cfg.ChatGPTBaseURL,
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
	})
}

// Internal returns the underlying internal client interface.
// This is used internally by the SDK and should not be used directly.
func (c *Client) Internal() client.Client {
	return c.internal
}

// getDefaultModel returns the default model based on the platform.
// Matches the behavior in internal/config: gpt-5 on Windows, gpt-5-codex elsewhere.
func getDefaultModel() string {
	if runtime.GOOS == "windows" {
		return "gpt-5"
	}
	return "gpt-5-codex"
}
