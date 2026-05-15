# Code Review: config.go

**File:** `/Users/williamcory/codex/codex-go/internal/config/config.go`
**Review Date:** 2025-10-26
**Coverage:** 77.6% (based on test execution)
**Overall Assessment:** Good foundation with several areas needing attention

---

## Executive Summary

The configuration package provides a solid foundation for loading and managing application settings from TOML files and environment variables. The code is well-structured and includes comprehensive tests. However, there are notable gaps in validation, security considerations, and edge case handling that should be addressed before production use.

**Critical Issues:** 2
**High Priority:** 8
**Medium Priority:** 12
**Low Priority:** 6
**Documentation Issues:** 4

---

## 1. Incomplete Features or Functionality

### 1.1 Missing Profile Support (MEDIUM)
**Location:** Lines 78-79, 204, 312-314
**Issue:** The `Profile` field exists in the Config struct and is loaded from TOML, but there's no implementation to actually use profiles.

```go
// Profile is the active profile name used to derive this Config (if any)
Profile string
```

**Recommendation:**
- Either implement profile loading (from `[profiles.*]` sections) or remove the field
- The README mentions "Profile support (loading from `profiles.*` in TOML)" as a future enhancement
- If keeping, add validation to ensure profile references are valid

### 1.2 MCPServerConfig Enabled Field Logic is Incomplete (HIGH)
**Location:** Lines 301-307
**Issue:** The comment suggests there should be default logic for the `Enabled` field, but the implementation is a no-op:

```go
// Set default enabled=true if not specified
for name, server := range cfg.MCPServers {
    // Default enabled to true if not explicitly set
    // (In TOML, we can't distinguish between false and unset,
    // so we assume if Command or URL is set, it's meant to be enabled by default)
    // Note: Currently no default logic needed - servers are enabled explicitly in config
    cfg.MCPServers[name] = server
}
```

**Recommendation:**
- Implement the intended default logic or remove the misleading comment
- TOML boolean defaults to `false`, so distinguish between:
  - User explicitly set `enabled = false` (disable)
  - User didn't set `enabled` (should default to true if command/URL provided)
- Consider using `*bool` in `configTOML` to distinguish unset from false

### 1.3 Missing Notification Configuration Validation (HIGH)
**Location:** Lines 154-182
**Issue:** `NotifyConfig` and `NotifyTriggerConfig` are loaded but never validated:

```go
type NotifyConfig struct {
    OnTurnComplete *NotifyTriggerConfig `toml:"on_turn_complete"`
    OnError *NotifyTriggerConfig `toml:"on_error"`
    OnApprovalNeeded *NotifyTriggerConfig `toml:"on_approval_needed"`
    OnTurnAborted *NotifyTriggerConfig `toml:"on_turn_aborted"`
    ScriptTimeoutSec *float64 `toml:"script_timeout_sec"`
}
```

**Missing Validations:**
- `Command` field should not be empty when `Enabled` is true
- `ScriptTimeoutSec` should be positive if set
- Command paths should exist and be executable
- Env var keys should not be empty

**Recommendation:**
Add notification validation to `Validate()` method.

### 1.4 WebSearchProvider Not Validated (MEDIUM)
**Location:** Lines 69-73, 315-320
**Issue:** `WebSearchProvider` is loaded but never validated against supported providers:

```go
WebSearchProvider string
WebSearchEnabled bool
```

**Recommendation:**
- Add validation to check if provider is supported (e.g., "duckduckgo")
- Consider making it an enum/constant
- Validate that if `WebSearchEnabled` is true, provider is not empty

### 1.5 MCPServerConfig Transport Validation Missing (CRITICAL)
**Location:** Lines 82-128
**Issue:** No validation ensures servers have valid transport configuration:

```go
type MCPServerConfig struct {
    Command string `toml:"command"`
    URL string `toml:"url"`
    // ... other fields
}
```

**Problems:**
- A server can have both `Command` and `URL` set (ambiguous)
- A server can have neither `Command` nor `URL` set (invalid)
- No validation that stdio transport has a command
- No validation that HTTP transport has a URL

**Recommendation:**
Add validation in `Validate()` method to ensure:
```go
// For each enabled MCP server:
// - Must have exactly one of: Command (stdio) or URL (http)
// - If Command: ensure it's not empty
// - If URL: ensure it's valid and not empty
// - If BearerTokenEnvVar: check env var exists
```

### 1.6 OAuth Configuration Not Validated (HIGH)
**Location:** Lines 130-152
**Issue:** `MCPOAuthConfig` is loaded but never validated:

```go
type MCPOAuthConfig struct {
    ClientID string `toml:"client_id"`
    ClientSecret string `toml:"client_secret"`
    AuthURL string `toml:"auth_url"`
    TokenURL string `toml:"token_url"`
    Scopes []string `toml:"scopes"`
    UseDiscovery bool `toml:"use_discovery"`
    UsePKCE bool `toml:"use_pkce"`
}
```

**Missing Validations:**
- `ClientID` should not be empty
- If not using discovery, `AuthURL` and `TokenURL` must be valid URLs
- If using discovery, URLs can be optional but base URL must be derivable
- Scopes should not be empty

**Recommendation:**
Add OAuth validation when MCP server has OAuth configured.

---

## 2. TODO Comments or Technical Debt Markers

### 2.1 No TODO/FIXME Comments Found (GOOD)
**Status:** Clean
The codebase has no TODO, FIXME, HACK, XXX, BUG, or DEPRECATED markers, which is excellent for production code.

---

## 3. Code Quality Issues

### 3.1 Inconsistent Pointer Usage for Optional Fields (MEDIUM)
**Location:** Lines 28-34, 54-55, 115-118, 169
**Issue:** Inconsistent use of pointers for optional fields:

```go
// Pointers used (good for distinguishing unset vs zero):
ModelContextWindow *int64
ModelMaxOutputTokens *int64
StartupTimeoutSec *float64
ScriptTimeoutSec *float64

// Not pointers (cannot distinguish unset from zero):
ProjectDocMaxBytes int  // Line 55
```

**Recommendation:**
- Either use `*int` for `ProjectDocMaxBytes` or document why it's different
- Current implementation always sets default 32*1024, so unset case doesn't exist
- Add comment explaining the design decision

### 3.2 Magic Numbers Without Named Constants (LOW)
**Location:** Line 224
**Issue:** Hard-coded magic number:

```go
ProjectDocMaxBytes: 32 * 1024, // 32 KiB
```

**Recommendation:**
```go
const (
    DefaultProjectDocMaxBytes = 32 * 1024 // 32 KiB
)

// In LoadConfig:
ProjectDocMaxBytes: DefaultProjectDocMaxBytes,
```

### 3.3 Empty Map Initialization Could Be Nil (LOW)
**Location:** Line 226
**Issue:** Unnecessary map allocation:

```go
MCPServers: make(map[string]MCPServerConfig),
```

**Recommendation:**
```go
MCPServers: nil,  // Will be allocated if needed
```

This is more memory-efficient and idiomatic Go. The TOML loader will allocate if needed.

### 3.4 Redundant Else After Return (LOW)
**Location:** Lines 363-367
**Issue:** Unnecessary else clause:

```go
} else if err == nil {
    return "", fmt.Errorf("CODEX_HOME is not a directory: %s", codexHome)
}
```

**Recommendation:**
```go
if err == nil && !info.IsDir() {
    return "", fmt.Errorf("CODEX_HOME is not a directory: %s", codexHome)
}
```

### 3.5 Missing Error Context in Some Cases (LOW)
**Location:** Lines 234-239
**Issue:** Error doesn't include file path context:

```go
if err := loadTOMLConfig(configPath, cfg); err != nil {
    return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
}
```

**Good!** This is actually done correctly. However, at line 252, the TOML decode error lacks context:

```go
if _, err := toml.DecodeFile(path, &tomlCfg); err != nil {
    return err  // Should wrap with context
}
```

**Recommendation:**
```go
if _, err := toml.DecodeFile(path, &tomlCfg); err != nil {
    return fmt.Errorf("failed to decode TOML: %w", err)
}
```

### 3.6 loadTOMLConfig Function is Too Long (MEDIUM)
**Location:** Lines 248-323
**Issue:** Function has 76 lines with repetitive pointer checking. This violates single responsibility principle.

**Recommendation:**
Extract helper function:
```go
func applyStringField(target *string, source *string) {
    if source != nil {
        *target = *source
    }
}

func applyInt64Field(target **int64, source *int64) {
    if source != nil {
        *target = source
    }
}

func applyBoolField(target *bool, source *bool) {
    if source != nil {
        *target = *source
    }
}
```

### 3.7 No Thread Safety Documentation (MEDIUM)
**Location:** Throughout
**Issue:** No documentation about thread safety of `Config` struct. Is it safe to read from multiple goroutines after loading?

**Recommendation:**
Add package-level documentation:
```go
// Package config provides configuration loading and management for Codex.
//
// Thread Safety: Config instances are safe for concurrent read access
// after loading is complete. Do not modify Config instances after they
// are returned from LoadConfig().
```

### 3.8 Duplicate Field Names Between Config and configTOML (MEDIUM)
**Location:** Lines 17-80 vs 185-205
**Issue:** Maintaining two parallel structs with similar fields is error-prone. Easy to forget to sync changes.

**Recommendation:**
- Add a comment linking the two structs
- Consider code generation or struct tags to reduce duplication
- Add a test that verifies all TOML fields have corresponding Config fields

---

## 4. Missing Test Coverage

### 4.1 CodexHomeDir Method Not Tested (LOW)
**Location:** Line 441
**Coverage:** 0.0%

```go
func (c *Config) CodexHomeDir() string {
    return c.CodexHome
}
```

**Recommendation:**
Add simple test or remove this trivial getter method (accessing field directly is clearer).

### 4.2 getDefaultModel Windows Path Not Tested (MEDIUM)
**Location:** Lines 381-386
**Coverage:** 66.7% (Windows branch likely untested)

```go
func getDefaultModel() string {
    if runtime.GOOS == "windows" {
        return "gpt-5"
    }
    return "gpt-5-codex"
}
```

**Recommendation:**
Add test with OS-specific builds or mock runtime.GOOS.

### 4.3 Validate Method Incomplete Coverage (HIGH)
**Location:** Lines 389-437
**Coverage:** 57.1%

**Missing Test Cases:**
- Empty `ReviewModel`
- Empty `ModelProvider`
- Empty `ChatGPTBaseURL`
- Zero or negative `ProjectDocMaxBytes`
- Invalid approval policies (each specific value)
- Model validation failure paths
- Review model validation failure paths

**Recommendation:**
Add comprehensive validation tests:
```go
func TestValidation(t *testing.T) {
    tests := []struct {
        name    string
        config  Config
        wantErr string
    }{
        {
            name: "empty review model",
            config: Config{
                Model: "gpt-5-codex",
                ReviewModel: "",
                // ...
            },
            wantErr: "review_model",
        },
        // ... more cases
    }
}
```

### 4.4 findCodexHome Edge Cases Not Tested (MEDIUM)
**Location:** Lines 352-377
**Coverage:** 61.5%

**Missing Test Cases:**
- CODEX_HOME points to a file (not directory)
- CODEX_HOME with relative path
- CODEX_HOME that cannot be converted to absolute path
- UserHomeDir() failure

**Recommendation:**
Add edge case tests for error conditions.

### 4.5 MCP Server Configuration Not Thoroughly Tested (HIGH)
**Issue:** While basic MCP server loading is tested, the following are not:
- Servers with both Command and URL (should be invalid)
- Servers with neither Command nor URL (should be invalid)
- HTTP headers and env headers
- OAuth configuration loading
- EnvVars field
- CWD field
- EnabledTools and DisabledTools arrays
- Timeout configurations

**Recommendation:**
Add comprehensive MCP server tests covering all fields.

### 4.6 Notification Configuration Not Tested (HIGH)
**Location:** Lines 154-182
**Issue:** No tests for NotifyConfig loading from TOML.

**Recommendation:**
Add test with notify configuration:
```go
configContent := `
[notify]
script_timeout_sec = 30.0

[notify.on_turn_complete]
command = "/usr/bin/notify"
enabled = true

[notify.on_error]
command = "/usr/bin/error-notify"
enabled = false
`
```

### 4.7 Environment Variable Edge Cases Not Tested (MEDIUM)
**Issue:** Missing tests for:
- Environment variables with empty strings (should they override?)
- Special characters in env vars
- Very long env var values

**Recommendation:**
Add edge case tests for environment variable handling.

---

## 5. Potential Bugs or Edge Cases Not Handled

### 5.1 No Validation for API Key Format (MEDIUM)
**Location:** Lines 342-347
**Issue:** API keys are accepted without validation:

```go
if apiKey := os.Getenv("CODEX_API_KEY"); apiKey != "" {
    cfg.APIKey = apiKey
}
```

**Recommendation:**
- Add basic format validation (e.g., starts with "sk-", minimum length)
- Consider validating against expected provider key formats
- Log warning for suspicious key formats

### 5.2 Race Condition in Config Modification (CRITICAL)
**Location:** Throughout
**Issue:** No protection against concurrent modification. If multiple goroutines call `LoadConfig()` or if config is modified after loading, race conditions could occur.

**Evidence:**
```bash
go test -race ./internal/config/...
```

**Recommendation:**
- Document that Config is immutable after loading
- Consider making fields private with getters
- Add mutex if config needs to be reloadable

### 5.3 No Bounds Checking for Token Limits (MEDIUM)
**Location:** Lines 28-34, 265-272
**Issue:** Token limits can be set to absurd values:

```go
ModelContextWindow *int64
ModelMaxOutputTokens *int64
ModelAutoCompactTokenLimit *int64
```

**Recommendation:**
Add validation:
```go
// In Validate():
if c.ModelContextWindow != nil && *c.ModelContextWindow <= 0 {
    return fmt.Errorf("model_context_window must be positive")
}
if c.ModelContextWindow != nil && *c.ModelContextWindow > 10_000_000 {
    return fmt.Errorf("model_context_window suspiciously large")
}
// Similar checks for other token fields
```

### 5.4 MCPServerConfig Map Value Modification Issue (HIGH)
**Location:** Lines 301-307
**Issue:** Attempting to modify map values through iteration won't work as intended:

```go
for name, server := range cfg.MCPServers {
    cfg.MCPServers[name] = server  // This is a no-op!
}
```

**Explanation:** `server` is a copy, so modifying it doesn't affect the map. The assignment back is necessary but currently does nothing since no modifications were made.

**Recommendation:**
Either implement the intended logic or remove this loop entirely.

### 5.5 No Check for Circular References in Config (LOW)
**Location:** Line 97
**Issue:** `CWD` field in MCP config could theoretically reference the config file location, creating circular dependency.

**Recommendation:**
Add validation to prevent CWD from being the config directory or create safeguards against path traversal attacks.

### 5.6 Time-of-Check to Time-of-Use Race (MEDIUM)
**Location:** Lines 233-239, 356-362
**Issue:** TOCTOU race condition:

```go
if _, err := os.Stat(configPath); err == nil {
    if err := loadTOMLConfig(configPath, cfg); err != nil {
        // File could be deleted between Stat and DecodeFile
    }
}

if info, err := os.Stat(codexHome); err == nil && info.IsDir() {
    // Directory status could change between Stat and use
}
```

**Recommendation:**
- Accept that this is a minor race and not worth complex locking
- Document the assumption that config files don't change during load
- Alternatively, read file first, then parse content

### 5.7 No Cleanup on Partial Load Failure (LOW)
**Location:** Lines 210-244
**Issue:** If `LoadConfig()` fails after partially modifying the config object, no cleanup occurs.

**Recommendation:**
Not critical since the caller receives an error and shouldn't use the config, but consider returning nil config on error for clarity.

### 5.8 Absolute Path Not Validated (LOW)
**Location:** Lines 358-362
**Issue:** `filepath.Abs()` can fail, but error is returned generically:

```go
absPath, err := filepath.Abs(codexHome)
if err != nil {
    return "", fmt.Errorf("failed to get absolute path: %w", err)
}
```

**Recommendation:**
Add more context: `"failed to get absolute path for CODEX_HOME=%s: %w", codexHome, err`

---

## 6. Documentation Issues

### 6.1 Package Documentation Incomplete (MEDIUM)
**Location:** Lines 1-4
**Issue:** Package doc doesn't mention validation requirements or thread safety.

**Current:**
```go
// Package config provides configuration loading and management for Codex.
// It supports loading from TOML files (~/.codex/config.toml) and environment
// variable overrides, matching the behavior of the Rust implementation.
```

**Recommended:**
```go
// Package config provides configuration loading and management for Codex.
//
// It supports loading from TOML files (~/.codex/config.toml) and environment
// variable overrides, matching the behavior of the Rust implementation.
//
// Usage:
//     cfg, err := config.LoadConfig()
//     if err != nil {
//         return err
//     }
//     if err := cfg.Validate(); err != nil {
//         return err
//     }
//
// Configuration is loaded in the following order of precedence:
//   1. Default values
//   2. TOML file (~/.codex/config.toml)
//   3. Environment variables (highest precedence)
//
// Thread Safety:
// Config instances are safe for concurrent reads after loading.
// Do not modify Config instances after they are returned.
```

### 6.2 Validation Not Documented as Required Step (HIGH)
**Location:** Line 207
**Issue:** `LoadConfig()` documentation doesn't mention that `Validate()` should be called:

```go
// LoadConfig loads configuration from ~/.codex/config.toml and environment variables.
// Environment variables take precedence over file-based configuration.
// If the config file doesn't exist, default values are used.
func LoadConfig() (*Config, error) {
```

**Recommendation:**
```go
// LoadConfig loads configuration from ~/.codex/config.toml and environment variables.
// Environment variables take precedence over file-based configuration.
// If the config file doesn't exist, default values are used.
//
// Note: After loading, call Validate() to ensure the configuration is valid:
//     cfg, err := LoadConfig()
//     if err != nil {
//         return err
//     }
//     if err := cfg.Validate(); err != nil {
//         return err
//     }
func LoadConfig() (*Config, error) {
```

### 6.3 MCPServerConfig Fields Lack Usage Examples (MEDIUM)
**Location:** Lines 82-128
**Issue:** Complex struct with many optional fields but no examples of valid configurations.

**Recommendation:**
Add example configurations in documentation:
```go
// MCPServerConfig defines configuration for an MCP server.
//
// Example stdio transport server:
//     [mcp_servers.filesystem]
//     command = "npx"
//     args = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
//     enabled = true
//     startup_timeout_sec = 30.0
//
// Example HTTP transport server:
//     [mcp_servers.api]
//     url = "https://api.example.com/mcp"
//     bearer_token_env_var = "API_TOKEN"
//     enabled = true
//
// Example server with OAuth:
//     [mcp_servers.oauth_api]
//     url = "https://api.example.com/mcp"
//     enabled = true
//     [mcp_servers.oauth_api.oauth]
//     client_id = "your-client-id"
//     use_pkce = true
//     scopes = ["read", "write"]
type MCPServerConfig struct {
```

### 6.4 Field Comments Could Be More Descriptive (LOW)
**Location:** Various
**Examples:**
```go
// Model is the AI model to use (e.g., "gpt-5-codex")
Model string  // Good!

// DisablePasteBurst disables burst-paste detection for typed input
DisablePasteBurst bool  // What is "burst-paste detection"? Needs more explanation

// EnvVars lists environment variable names to pass through
EnvVars []string  // Pass through to where? The MCP server process?
```

**Recommendation:**
Expand comments with more context about what the field does and how it's used.

---

## 7. Security Concerns

### 7.1 No Validation of Command Paths (CRITICAL)
**Location:** Line 85, Line 175
**Issue:** Command paths in MCP servers and notifications are not validated:

```go
Command string `toml:"command"`
```

**Risks:**
- Path traversal attacks
- Execution of arbitrary binaries
- No verification that command exists or is executable
- Could execute malicious scripts from untrusted config files

**Recommendation:**
```go
// In Validate():
for name, server := range c.MCPServers {
    if server.Enabled && server.Command != "" {
        // Check command exists and is executable
        // Validate against whitelist if possible
        // Reject commands with suspicious patterns (../../, etc.)
        if !isValidCommand(server.Command) {
            return fmt.Errorf("invalid command for MCP server '%s': %s", name, server.Command)
        }
    }
}
```

### 7.2 No Sanitization of Environment Variables (HIGH)
**Location:** Lines 91-94, 180-181
**Issue:** Environment variables from config are passed through without sanitization:

```go
Env map[string]string `toml:"env"`
EnvVars []string `toml:"env_vars"`
```

**Risks:**
- Could override critical env vars (PATH, LD_PRELOAD, etc.)
- Could inject malicious environment
- No bounds checking on env var size

**Recommendation:**
- Blacklist dangerous env vars (LD_PRELOAD, DYLD_*, etc.)
- Validate env var names match expected pattern
- Limit env var value length
- Consider running MCP servers in isolated environment

### 7.3 Bearer Token from Environment Not Validated (MEDIUM)
**Location:** Line 103
**Issue:** Bearer token is read from env var but not validated:

```go
BearerTokenEnvVar string `toml:"bearer_token_env_var"`
```

**Risks:**
- No check if env var exists (leads to empty auth header)
- No validation of token format
- Token could be logged in error messages

**Recommendation:**
```go
// In Validate():
if server.BearerTokenEnvVar != "" {
    token := os.Getenv(server.BearerTokenEnvVar)
    if token == "" {
        return fmt.Errorf("bearer token env var %s is not set for server %s",
            server.BearerTokenEnvVar, name)
    }
    // Don't log the token value!
}
```

### 7.4 API Keys Could Be Logged (HIGH)
**Location:** Lines 342-347
**Issue:** API keys are stored in plain string, could appear in logs:

```go
cfg.APIKey = apiKey
```

**Risks:**
- Accidental logging of API keys
- Memory dumps could expose keys
- Error messages might include config dump

**Recommendation:**
- Never log entire Config struct
- Add `String()` method that redacts sensitive fields
- Consider using secure string type
- Document that Config contains secrets

### 7.5 No Protection Against Config File Permission Issues (MEDIUM)
**Location:** Lines 232-239
**Issue:** Config file permissions are not checked:

```go
if _, err := os.Stat(configPath); err == nil {
    if err := loadTOMLConfig(configPath, cfg); err != nil {
```

**Risks:**
- World-readable config files could expose secrets
- Attacker could modify config if permissions are too loose

**Recommendation:**
```go
// In LoadConfig():
if info, err := os.Stat(configPath); err == nil {
    mode := info.Mode()
    if mode.Perm()&0077 != 0 {
        // Warn or error if config is readable/writable by group/others
        log.Warning("Config file has insecure permissions: %s", mode)
    }
    if err := loadTOMLConfig(configPath, cfg); err != nil {
```

### 7.6 URL Fields Not Validated (HIGH)
**Location:** Lines 46, 100, 139, 141
**Issue:** URL fields are not validated for:

```go
ChatGPTBaseURL string
URL string `toml:"url"`
AuthURL string `toml:"auth_url"`
TokenURL string `toml:"token_url"`
```

**Risks:**
- SSRF (Server-Side Request Forgery) attacks
- Redirect to malicious URLs
- Internal network scanning
- Scheme injection (javascript:, file:, etc.)

**Recommendation:**
```go
// In Validate():
if err := validateURL(c.ChatGPTBaseURL); err != nil {
    return fmt.Errorf("invalid chatgpt_base_url: %w", err)
}

func validateURL(urlStr string) error {
    if urlStr == "" {
        return fmt.Errorf("URL cannot be empty")
    }

    u, err := url.Parse(urlStr)
    if err != nil {
        return fmt.Errorf("invalid URL format: %w", err)
    }

    // Only allow http/https
    if u.Scheme != "http" && u.Scheme != "https" {
        return fmt.Errorf("invalid URL scheme: %s (must be http or https)", u.Scheme)
    }

    // Reject localhost/internal IPs for external APIs
    if isInternalIP(u.Hostname()) {
        return fmt.Errorf("URL points to internal network: %s", u.Hostname())
    }

    return nil
}
```

### 7.7 No Rate Limiting or Abuse Prevention (LOW)
**Location:** Throughout
**Issue:** Config can specify unlimited MCP servers, unlimited timeouts, huge buffer sizes, etc.

**Recommendation:**
Add reasonable limits:
```go
const (
    MaxMCPServers = 50
    MaxProjectDocBytes = 10 * 1024 * 1024  // 10 MB
    MaxTimeoutSec = 3600  // 1 hour
)

// In Validate():
if len(c.MCPServers) > MaxMCPServers {
    return fmt.Errorf("too many MCP servers: %d (max %d)", len(c.MCPServers), MaxMCPServers)
}
```

### 7.8 Working Directory Not Validated (MEDIUM)
**Location:** Line 97
**Issue:** `CWD` for MCP servers is not validated:

```go
CWD string `toml:"cwd"`
```

**Risks:**
- Path traversal
- Access to sensitive directories
- No check if directory exists

**Recommendation:**
```go
// In Validate():
if server.CWD != "" {
    absPath, err := filepath.Abs(server.CWD)
    if err != nil {
        return fmt.Errorf("invalid cwd for server %s: %w", name, err)
    }

    info, err := os.Stat(absPath)
    if err != nil {
        return fmt.Errorf("cwd does not exist for server %s: %s", name, absPath)
    }

    if !info.IsDir() {
        return fmt.Errorf("cwd is not a directory for server %s: %s", name, absPath)
    }

    // Reject suspicious paths
    if strings.Contains(absPath, "..") {
        return fmt.Errorf("cwd contains suspicious path components for server %s: %s", name, absPath)
    }
}
```

---

## 8. Performance Considerations

### 8.1 Unnecessary Map Copies (LOW)
**Location:** Lines 298-307
**Issue:** Map is copied unnecessarily during merge:

```go
if tomlCfg.MCPServers != nil {
    cfg.MCPServers = tomlCfg.MCPServers  // This copies the map
```

**Recommendation:**
This is fine for config loading (not performance critical), but consider if config needs to be loaded frequently.

### 8.2 Validation on Every Use (MEDIUM)
**Issue:** Design requires manual validation call. Easy to forget.

**Recommendation:**
Consider validating automatically in LoadConfig() or create a LoadAndValidateConfig() convenience function:
```go
func LoadAndValidateConfig() (*Config, error) {
    cfg, err := LoadConfig()
    if err != nil {
        return nil, err
    }
    if err := cfg.Validate(); err != nil {
        return nil, err
    }
    return cfg, nil
}
```

---

## 9. Recommendations Summary

### Immediate Action Required (Critical/High)
1. **Add comprehensive validation for MCP server transport configuration** - Ensure servers have valid Command or URL
2. **Implement security validation for command paths** - Prevent arbitrary command execution
3. **Add URL validation** - Prevent SSRF attacks
4. **Fix race condition documentation** - Document thread safety guarantees
5. **Add notification configuration validation** - Ensure enabled notifications have valid commands
6. **Add OAuth configuration validation** - Ensure OAuth configs are complete
7. **Test coverage for Validate() method** - Increase from 57.1% to >90%

### Short-term Improvements (Medium)
1. Implement or remove Profile support
2. Add validation for WebSearchProvider
3. Add validation for token limits (bounds checking)
4. Improve error messages with more context
5. Refactor loadTOMLConfig to reduce repetition
6. Add thread safety documentation
7. Add comprehensive MCP server tests
8. Add notification configuration tests

### Long-term Enhancements (Low)
1. Extract named constants for magic numbers
2. Add examples to complex struct documentation
3. Consider nil map instead of empty map initialization
4. Add CodexHomeDir test or remove method
5. Add OS-specific tests for getDefaultModel
6. Consider struct generation to reduce Config/configTOML duplication

---

## 10. Test Coverage Report

| Function | Coverage | Priority to Test |
|----------|----------|------------------|
| LoadConfig | 83.3% | Medium |
| loadTOMLConfig | 90.9% | Medium |
| applyEnvOverrides | 80.0% | Medium |
| findCodexHome | 61.5% | High |
| getDefaultModel | 66.7% | Medium |
| Validate | 57.1% | **Critical** |
| CodexHomeDir | 0.0% | Low |
| GetConfigPath | Not shown | Low |
| GetHistoryPath | Not shown | Low |
| GetAuthPath | Not shown | Low |

**Overall Coverage:** 77.6%
**Target Coverage:** >90%
**Gap:** ~12.4 percentage points

---

## 11. Conclusion

The config package provides a solid foundation with good test coverage (77.6%) and clean code structure. The major concerns are:

1. **Security**: Insufficient validation of user-controlled paths, commands, and URLs could lead to serious vulnerabilities
2. **Validation**: Many configuration fields lack validation, allowing invalid states
3. **Documentation**: Thread safety and validation requirements need better documentation
4. **Test Coverage**: Critical validation paths need more comprehensive testing

**Recommendation:** Address critical security issues before production use. The code is well-structured and should be straightforward to enhance with the recommended validations and tests.

**Estimated Effort:**
- Critical fixes: 2-3 days
- Medium priority: 3-5 days
- Low priority: 2-3 days
- **Total: ~1-2 weeks** for complete remediation

---

## Appendix A: Suggested Validation Function

```go
func (c *Config) Validate() error {
    // Existing validations...

    // MCP Server validations
    for name, server := range c.MCPServers {
        if !server.Enabled {
            continue
        }

        // Must have exactly one transport
        hasStdio := server.Command != ""
        hasHTTP := server.URL != ""
        if !hasStdio && !hasHTTP {
            return fmt.Errorf("MCP server '%s': must specify either 'command' or 'url'", name)
        }
        if hasStdio && hasHTTP {
            return fmt.Errorf("MCP server '%s': cannot specify both 'command' and 'url'", name)
        }

        // Validate stdio transport
        if hasStdio {
            if err := validateCommand(server.Command); err != nil {
                return fmt.Errorf("MCP server '%s': invalid command: %w", name, err)
            }
        }

        // Validate HTTP transport
        if hasHTTP {
            if err := validateURL(server.URL); err != nil {
                return fmt.Errorf("MCP server '%s': invalid URL: %w", name, err)
            }
        }

        // Validate OAuth if present
        if server.OAuth != nil {
            if err := validateOAuth(server.OAuth); err != nil {
                return fmt.Errorf("MCP server '%s': invalid OAuth config: %w", name, err)
            }
        }

        // Validate timeouts
        if server.StartupTimeoutSec != nil && *server.StartupTimeoutSec <= 0 {
            return fmt.Errorf("MCP server '%s': startup_timeout_sec must be positive", name)
        }
        if server.ToolTimeoutSec != nil && *server.ToolTimeoutSec <= 0 {
            return fmt.Errorf("MCP server '%s': tool_timeout_sec must be positive", name)
        }
    }

    // Web search validation
    if c.WebSearchEnabled && c.WebSearchProvider == "" {
        return fmt.Errorf("web_search_provider must be set when web_search_enabled is true")
    }

    // Notification validation
    if c.Notify.OnTurnComplete != nil && c.Notify.OnTurnComplete.Enabled {
        if c.Notify.OnTurnComplete.Command == "" {
            return fmt.Errorf("notify.on_turn_complete: command cannot be empty when enabled")
        }
    }
    // Similar for other notification triggers...

    return nil
}
```

---

**End of Review**
