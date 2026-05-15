# Config Package

The `config` package provides configuration loading and management for Codex Go, matching the behavior of the Rust implementation.

## Overview

This package handles:
- Loading configuration from `~/.codex/config.toml`
- Environment variable overrides
- Default values matching the Rust implementation
- Configuration validation
- MCP server configuration

## Usage

```go
import "github.com/evmts/codex/codex-go/internal/config"

// Load configuration with defaults, TOML file, and env var overrides
cfg, err := config.LoadConfig()
if err != nil {
    log.Fatalf("Failed to load config: %v", err)
}

// Validate configuration
if err := cfg.Validate(); err != nil {
    log.Fatalf("Invalid config: %v", err)
}

// Access configuration values
fmt.Println("Model:", cfg.Model)
fmt.Println("Codex Home:", cfg.CodexHome)
fmt.Println("API Key:", cfg.APIKey)
```

## Configuration Sources

Configuration is loaded in the following order (later sources override earlier ones):

1. **Default values** - Built-in defaults matching Rust behavior
2. **TOML file** - `~/.codex/config.toml` (or `$CODEX_HOME/config.toml`)
3. **Environment variables** - Env vars take precedence over file config

### Default Values

| Field | Default | Notes |
|-------|---------|-------|
| `Model` | `gpt-5-codex` (Unix) / `gpt-5` (Windows) | Platform-specific default |
| `ReviewModel` | `gpt-5-codex` | Model for review sessions |
| `ModelProvider` | `openai-responses` | Default provider |
| `ChatGPTBaseURL` | `https://chatgpt.com` | Base URL for ChatGPT |
| `CodexHome` | `~/.codex` | Can be overridden with `$CODEX_HOME` |
| `ProjectDocMaxBytes` | `32768` (32 KiB) | Max size of AGENTS.md |

### TOML Configuration

Example `~/.codex/config.toml`:

```toml
model = "gpt-5-codex"
review_model = "gpt-5-codex"
model_provider = "openai-responses"
model_context_window = 200000
model_max_output_tokens = 8192
approval_policy = "auto"
hide_agent_reasoning = false
chatgpt_base_url = "https://chatgpt.com"
project_doc_max_bytes = 65536
disable_paste_burst = false

# MCP Server Configuration
[mcp_servers.filesystem]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
enabled = true
startup_timeout_sec = 30.0
tool_timeout_sec = 60.0

[mcp_servers.git]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-git"]
enabled = false

[mcp_servers.api]
url = "https://api.example.com/mcp"
bearer_token_env_var = "API_TOKEN"
enabled = true
```

### Environment Variables

| Variable | Purpose | Overrides |
|----------|---------|-----------|
| `CODEX_HOME` | Codex home directory | Default `~/.codex` |
| `CODEX_MODEL` | Model selection | `model` in TOML |
| `CODEX_API_KEY` | API key (preferred) | `OPENAI_API_KEY` |
| `OPENAI_API_KEY` | API key (fallback) | - |
| `CODEX_APPROVAL_POLICY` | Approval policy | `approval_policy` in TOML |
| `CODEX_BASE_URL` | ChatGPT base URL | `chatgpt_base_url` in TOML |

API Key Priority: `CODEX_API_KEY` > `OPENAI_API_KEY` > auth.json

## Configuration Fields

### Core Settings

- `Model` - AI model to use (e.g., "gpt-5-codex")
- `ReviewModel` - Model for review sessions
- `ModelProvider` - Provider ID from model_providers map
- `ModelContextWindow` - Context window size in tokens
- `ModelMaxOutputTokens` - Maximum output tokens
- `ModelAutoCompactTokenLimit` - Token threshold for auto-compaction

### Behavior Settings

- `ApprovalPolicy` - Command approval policy: "auto", "always", or "never"
- `HideAgentReasoning` - Suppress agent reasoning from output
- `ShowRawAgentReasoning` - Show raw agent reasoning events
- `DisablePasteBurst` - Disable paste burst detection

### Paths and Limits

- `CodexHome` - Directory for all Codex state
- `ChatGPTBaseURL` - Base URL for ChatGPT requests
- `ProjectDocMaxBytes` - Max bytes from AGENTS.md
- `ProjectDocFallbackFilenames` - Alternative doc filenames

### MCP Servers

- `MCPServers` - Map of MCP server configurations

Each MCP server supports:
- **Stdio transport**: `command`, `args`, `env`, `env_vars`, `cwd`
- **HTTP transport**: `url`, `bearer_token_env_var`, `http_headers`
- **Common settings**: `enabled`, `startup_timeout_sec`, `tool_timeout_sec`
- **Tool filtering**: `enabled_tools`, `disabled_tools`

## Validation

The `Validate()` method checks:
- Required fields are not empty (model, review_model, model_provider, codex_home, chatgpt_base_url)
- `project_doc_max_bytes` is positive
- `approval_policy` is valid ("auto", "always", or "never") if set

```go
if err := cfg.Validate(); err != nil {
    log.Fatalf("Invalid config: %v", err)
}
```

## Helper Methods

```go
// Get paths to various Codex files
configPath := cfg.GetConfigPath()    // ~/.codex/config.toml
historyPath := cfg.GetHistoryPath()  // ~/.codex/history.jsonl
authPath := cfg.GetAuthPath()        // ~/.codex/auth.json
```

## Testing

The package includes comprehensive tests covering:
- Default values
- TOML loading
- Environment variable overrides
- API key precedence
- Missing config files
- Invalid TOML
- CODEX_HOME handling
- Validation rules
- Complex configurations with MCP servers

Run tests:
```bash
go test -v ./internal/config/...
```

## Compatibility with Rust Implementation

This Go implementation matches the Rust version (`codex-rs/core/src/config.rs`):

✅ Same default values (including platform-specific model defaults)
✅ Same TOML structure and field names
✅ Same environment variable names and precedence
✅ Same CODEX_HOME resolution logic
✅ Same API key precedence (CODEX_API_KEY > OPENAI_API_KEY)
✅ Same MCP server configuration format
✅ Graceful handling of missing config files

## Design Decisions

1. **Pointer types for optional integers** - Uses `*int64` for optional numeric fields to distinguish between unset and zero values
2. **Two-stage loading** - Defaults → TOML → Environment variables
3. **No mutation after load** - Config is immutable after loading (except through reload)
4. **Validation is explicit** - Call `Validate()` explicitly after loading
5. **Platform-aware defaults** - Default model differs on Windows vs Unix

## Future Enhancements

Potential additions to match full Rust feature set:
- Profile support (loading from `profiles.*` in TOML)
- Model provider definitions
- Shell environment policy
- History settings
- Project trust levels
- Notices and TUI notifications
- OTEL configuration
