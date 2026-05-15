package config_test

import (
	"fmt"
	"log"
	"os"

	"github.com/evmts/codex/codex-go/internal/config"
)

// ExampleLoadConfig demonstrates basic config loading
func ExampleLoadConfig() {
	// Set up environment for example
	tmpDir := os.TempDir()
	os.Setenv("CODEX_HOME", tmpDir)
	defer os.Unsetenv("CODEX_HOME")

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Model: %s\n", cfg.Model)
	fmt.Printf("Provider: %s\n", cfg.ModelProvider)
	fmt.Printf("Codex Home: %s\n", cfg.CodexHome)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}
}

// ExampleConfig_GetConfigPath demonstrates getting the config file path
func ExampleConfig_GetConfigPath() {
	cfg := &config.Config{
		CodexHome: "/home/user/.codex",
	}

	fmt.Println(cfg.GetConfigPath())
	// Output: /home/user/.codex/config.toml
}

// ExampleConfig_GetHistoryPath demonstrates getting the history file path
func ExampleConfig_GetHistoryPath() {
	cfg := &config.Config{
		CodexHome: "/home/user/.codex",
	}

	fmt.Println(cfg.GetHistoryPath())
	// Output: /home/user/.codex/history.jsonl
}

// ExampleConfig_GetAuthPath demonstrates getting the auth file path
func ExampleConfig_GetAuthPath() {
	cfg := &config.Config{
		CodexHome: "/home/user/.codex",
	}

	fmt.Println(cfg.GetAuthPath())
	// Output: /home/user/.codex/auth.json
}

// ExampleConfig_Validate demonstrates config validation
func ExampleConfig_Validate() {
	// Valid config
	validCfg := &config.Config{
		Model:              "gpt-5-codex",
		ReviewModel:        "gpt-5-codex",
		ModelProvider:      "openai-responses",
		CodexHome:          "/tmp/codex",
		ChatGPTBaseURL:     "https://chatgpt.com",
		ProjectDocMaxBytes: 32768,
	}

	if err := validCfg.Validate(); err != nil {
		fmt.Printf("Valid config error: %v\n", err)
	} else {
		fmt.Println("Valid config: OK")
	}

	// Invalid config (empty model)
	invalidCfg := &config.Config{
		Model:         "",
		ReviewModel:   "gpt-5-codex",
		ModelProvider: "openai-responses",
	}

	if err := invalidCfg.Validate(); err != nil {
		fmt.Println("Invalid config detected")
	}

	// Output:
	// Valid config: OK
	// Invalid config detected
}
