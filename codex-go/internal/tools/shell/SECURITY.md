# Environment Variable Filtering Security

## Overview

The shell tool now implements environment variable filtering to prevent credential leakage to subprocesses. This critical security feature protects sensitive information like API keys, passwords, tokens, and other credentials from being inadvertently exposed to shell commands.

## Security Issue (Before)

Previously, when executing commands with custom environment variables, the code would pass **ALL** system environment variables to the subprocess:

```go
// SECURITY RISK - Passes ALL env vars including secrets!
if len(spec.Environment) > 0 {
    cmd.Env = os.Environ()  // ❌ Dangerous!
    for k, v := range spec.Environment {
        cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
    }
}
```

This meant that sensitive credentials like `AWS_SECRET_ACCESS_KEY`, `GITHUB_TOKEN`, `DATABASE_PASSWORD`, etc., were exposed to every subprocess, creating a significant security risk.

## Solution (After)

The new implementation uses an environment filter that:

1. **Filters system environment variables** before passing them to subprocesses
2. **Removes sensitive patterns** like `*KEY*`, `*SECRET*`, `*TOKEN*`, `*PASSWORD*`, etc.
3. **Preserves essential variables** like `HOME`, `PATH`, `USER`, `TMPDIR`, etc.
4. **Uses case-insensitive matching** to catch variations like `api_key`, `Api_Key`, etc.
5. **Provides customization** through `EnvFilterConfig` for advanced use cases

```go
// SECURE - Filters sensitive environment variables
if len(spec.Environment) > 0 {
    filter := NewDefaultEnvFilter()
    cmd.Env = filter.Filter()  // ✅ Only safe variables passed

    filteredEnv := filter.FilterMap(spec.Environment)
    for k, v := range filteredEnv {
        cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
    }
}
```

## Default Filter Patterns

The default filter excludes environment variables matching these patterns (case-insensitive):

- `*KEY*` - Catches API_KEY, AWS_ACCESS_KEY_ID, etc.
- `*SECRET*` - Catches AWS_SECRET_ACCESS_KEY, CLIENT_SECRET, etc.
- `*TOKEN*` - Catches GITHUB_TOKEN, AUTH_TOKEN, etc.
- `*PASSWORD*` - Catches DATABASE_PASSWORD, MYSQL_PASSWORD, etc.
- `*PASSWD*` - Catches DB_PASSWD, etc.
- `*CREDENTIAL*` - Catches SERVICE_CREDENTIAL, etc.
- `*AUTH*` - Catches OAUTH_TOKEN, etc.
- `*PRIVATE*` - Catches PRIVATE_KEY, etc.

## Essential Variables (Always Preserved)

These variables are preserved even if they match filter patterns:

- `HOME` - User home directory
- `PATH` - System executable paths
- `USER` - Current username
- `TMPDIR`, `TEMP`, `TMP` - Temporary directory paths
- `SHELL` - Default shell
- `LANG`, `LC_*` - Locale settings

## Real-World Credentials Filtered

The filter catches common credential patterns including:

- AWS: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`
- GitHub: `GITHUB_TOKEN`, `GITHUB_PAT`
- GitLab: `GITLAB_TOKEN`
- NPM: `NPM_TOKEN`
- Docker: `DOCKER_PASSWORD`
- Databases: `DATABASE_PASSWORD`, `POSTGRES_PASSWORD`, `MYSQL_PASSWORD`, `REDIS_PASSWORD`
- JWT: `JWT_SECRET`, `SESSION_SECRET`
- Encryption: `ENCRYPTION_KEY`, `PRIVATE_KEY`, `SIGNING_KEY`
- APIs: `API_KEY`, `API_SECRET`, `CLIENT_SECRET`
- OAuth: `OAUTH_TOKEN`, `ACCESS_TOKEN`, `REFRESH_TOKEN`, `BEARER_TOKEN`
- SaaS: `SLACK_TOKEN`, `STRIPE_SECRET_KEY`, `TWILIO_AUTH_TOKEN`, `SENDGRID_API_KEY`

## Usage Examples

### Default Filtering

```go
// Create filter with default configuration
filter := NewDefaultEnvFilter()

// Filter current environment
safeEnv := filter.Filter()

// Filter a map of environment variables
envMap := map[string]string{
    "HOME": "/home/user",
    "API_KEY": "secret123",  // Will be filtered out
    "DEBUG": "true",
}
filtered := filter.FilterMap(envMap)
// Result: {"HOME": "/home/user", "DEBUG": "true"}
```

### Custom Configuration

```go
// Create custom filter configuration
config := &EnvFilterConfig{
    ExcludePatterns: []string{
        "*SECRET*",
        "*INTERNAL*",
        "CUSTOM_SENSITIVE_VAR",
    },
    EssentialVars: []string{
        "HOME",
        "PATH",
        "CUSTOM_REQUIRED_VAR",  // Won't be filtered even if it matches patterns
    },
}

filter := NewEnvFilter(config)
safeEnv := filter.Filter()
```

### Check if Variable Should Be Filtered

```go
filter := NewDefaultEnvFilter()

// Returns true if the variable would be filtered
shouldFilter := filter.ShouldFilterVar("API_KEY")  // true
shouldFilter = filter.ShouldFilterVar("HOME")      // false
shouldFilter = filter.ShouldFilterVar("DEBUG")     // false
```

## Testing

Comprehensive tests are provided in `envfilter_test.go`:

```bash
# Run all environment filter tests
go test ./internal/tools/shell/... -run TestEnvFilter

# Run benchmarks
go test ./internal/tools/shell/... -bench=BenchmarkEnvFilter -benchmem
```

## Performance

The filter is designed to be efficient:

```
BenchmarkEnvFilter-14       	  571857	      2262 ns/op	     560 B/op	      14 allocs/op
BenchmarkEnvFilterMap-14    	  619348	      2070 ns/op	     336 B/op	       2 allocs/op
```

Filtering adds minimal overhead (~2 microseconds) while providing critical security benefits.

## Security Best Practices

1. **Always use the default filter** unless you have specific requirements
2. **Be cautious when customizing** - only add essential variables that you absolutely need
3. **Never disable filtering** for production systems
4. **Audit your environment** - use `ShouldFilterVar()` to test variable names
5. **Keep patterns up to date** - add new patterns as you discover new credential types
6. **Test thoroughly** - ensure your application works with filtered environments

## Migration Guide

If you're upgrading from the old implementation:

1. **No code changes required** - filtering is automatic
2. **Verify subprocess behavior** - ensure your commands don't rely on sensitive env vars
3. **Test your workflows** - run your test suite to catch any issues
4. **Update documentation** - inform users about the security improvement

## Further Reading

- [OWASP Environment Variable Security](https://owasp.org/www-community/vulnerabilities/Missing_Encryption_of_Sensitive_Data)
- [CWE-526: Exposure of Sensitive Information Through Environmental Variables](https://cwe.mitre.org/data/definitions/526.html)
- [Twelve-Factor App Config](https://12factor.net/config)
