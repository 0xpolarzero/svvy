# SDK Session Validation Guide

## Overview

The SDK provides comprehensive validation for all session options to ensure security, correctness, and prevent runtime errors.

## Validated Options

### 1. Approval Policy

Controls when tools require user approval before execution.

**Valid Values:**
- `auto` (default) - Automatically approve safe operations, ask for dangerous ones
- `always` - Always ask for approval
- `never` - Never ask for approval (use with caution)

**Example:**
```go
opts := sdk.SessionOptions{
    ApprovalPolicy: "auto",  // ✅ Valid
}

opts := sdk.SessionOptions{
    ApprovalPolicy: "invalid",  // ❌ Error: must be one of: auto, always, never
}
```

### 2. Sandbox Policy

Controls execution restrictions for shell commands and file operations.

**Valid Values:**
- `native` (default) - No sandboxing (full system access)
- `read_only` - Read-only file system access
- `workspace_write` - Write access limited to workspace
- `full_access` - Full system access with explicit permission

**Example:**
```go
opts := sdk.SessionOptions{
    SandboxPolicy: "workspace_write",  // ✅ Valid
}

opts := sdk.SessionOptions{
    SandboxPolicy: "unknown",  // ❌ Error: must be one of: native, read_only, workspace_write, full_access
}
```

### 3. Working Directory

The base directory for file operations and tool execution.

**Requirements:**
- Must be an absolute path
- Must exist on the file system
- Must be a directory (not a file)
- Must be accessible (readable/writable)

**Default:** Current working directory (`os.Getwd()`)

**Example:**
```go
opts := sdk.SessionOptions{
    WorkingDirectory: "/absolute/path/to/project",  // ✅ Valid
}

opts := sdk.SessionOptions{
    WorkingDirectory: "relative/path",  // ❌ Error: must be an absolute path
}

opts := sdk.SessionOptions{
    WorkingDirectory: "/nonexistent/path",  // ❌ Error: directory does not exist
}
```

### 4. Model

The AI model to use for the session.

**Requirements:**
- Must not be empty if provided
- Should be a valid model identifier
- Validated at runtime against available models

**Example:**
```go
opts := sdk.SessionOptions{
    Model: "claude-3-5-sonnet-20241022",  // ✅ Valid
}

opts := sdk.SessionOptions{
    Model: "",  // ✅ Valid (uses client default)
}
```

## Default Values

If you don't provide values, sensible defaults are applied:

```go
opts := sdk.SessionOptions{}  // Empty options

// After defaults:
// ApprovalPolicy:   "auto"
// SandboxPolicy:    "native"
// WorkingDirectory: os.Getwd() (or "." if getcwd fails)
// Model:            (empty, uses client default)
```

## Validation Flow

```
User calls NewSession(opts)
        |
        v
    Validate Options
        |
        v
   Apply Defaults
        |
        v
  Create Manager Session
        |
        v
  Return SDK Session
```

If validation fails at any step, an error is returned and no session is created.

## Complete Examples

### Example 1: Minimal Options
```go
package main

import (
    "context"
    "fmt"

    "github.com/evmts/codex/codex-go/pkg/sdk"
    "github.com/evmts/codex/codex-go/pkg/sdk/client"
)

func main() {
    // Create client
    cli, err := client.FromEnv()
    if err != nil {
        panic(err)
    }

    // Create SDK
    s, err := sdk.New(sdk.Options{
        Client: cli,
    })
    if err != nil {
        panic(err)
    }
    defer s.Close()

    // Create session with minimal options (uses defaults)
    session, err := s.NewSession(context.Background(), sdk.SessionOptions{})
    if err != nil {
        panic(err)
    }

    fmt.Printf("Session created with ID: %s\n", session.ID())
}
```

### Example 2: Custom Options
```go
package main

import (
    "context"
    "fmt"
    "path/filepath"

    "github.com/evmts/codex/codex-go/pkg/sdk"
    "github.com/evmts/codex/codex-go/pkg/sdk/client"
)

func main() {
    cli, err := client.FromEnv()
    if err != nil {
        panic(err)
    }

    s, err := sdk.New(sdk.Options{
        Client: cli,
    })
    if err != nil {
        panic(err)
    }
    defer s.Close()

    // Get absolute workspace path
    workspace, err := filepath.Abs("./my-project")
    if err != nil {
        panic(err)
    }

    // Create session with custom options
    session, err := s.NewSession(context.Background(), sdk.SessionOptions{
        ApprovalPolicy:   "always",           // Always ask for approval
        SandboxPolicy:    "workspace_write",  // Restrict writes to workspace
        WorkingDirectory: workspace,          // Set workspace directory
        Model:            "claude-3-5-sonnet-20241022",
        SystemPrompt:     "You are a helpful coding assistant.",
        Streaming:        true,               // Enable streaming responses
    })
    if err != nil {
        panic(err)
    }

    fmt.Printf("Session created with ID: %s\n", session.ID())
}
```

### Example 3: Error Handling
```go
package main

import (
    "context"
    "fmt"
    "strings"

    "github.com/evmts/codex/codex-go/pkg/sdk"
    "github.com/evmts/codex/codex-go/pkg/sdk/client"
)

func main() {
    cli, err := client.FromEnv()
    if err != nil {
        panic(err)
    }

    s, err := sdk.New(sdk.Options{
        Client: cli,
    })
    if err != nil {
        panic(err)
    }
    defer s.Close()

    // Try to create session with invalid options
    session, err := s.NewSession(context.Background(), sdk.SessionOptions{
        ApprovalPolicy:   "invalid-policy",
        SandboxPolicy:    "unknown-policy",
        WorkingDirectory: "relative/path",
    })

    if err != nil {
        // Handle specific validation errors
        errMsg := err.Error()

        if strings.Contains(errMsg, "approval policy") {
            fmt.Println("Fix approval policy: use auto, always, or never")
        }

        if strings.Contains(errMsg, "sandbox policy") {
            fmt.Println("Fix sandbox policy: use native, read_only, workspace_write, or full_access")
        }

        if strings.Contains(errMsg, "working directory") {
            fmt.Println("Fix working directory: provide an absolute path that exists")
        }

        fmt.Printf("Validation error: %v\n", err)
        return
    }

    // Session created successfully
    fmt.Printf("Session created: %s\n", session.ID())
}
```

## Testing Validation

Use the provided test suite:

```bash
# Run all validation tests
go test -v ./pkg/sdk -run TestValidate

# Run specific validation test
go test -v ./pkg/sdk -run TestValidateApprovalPolicy
go test -v ./pkg/sdk -run TestValidateSandboxPolicy
go test -v ./pkg/sdk -run TestValidateWorkingDirectory
go test -v ./pkg/sdk -run TestValidateSessionOptions
go test -v ./pkg/sdk -run TestApplySessionDefaults
```

## Common Validation Errors

### Error: Invalid Approval Policy
```
invalid session options: invalid approval policy: must be one of: auto, always, never (got "invalid")
```
**Solution:** Use `auto`, `always`, or `never`

### Error: Invalid Sandbox Policy
```
invalid session options: invalid sandbox policy: must be one of: native, read_only, workspace_write, full_access (got "unknown")
```
**Solution:** Use one of the valid sandbox policy values

### Error: Relative Working Directory
```
invalid session options: invalid working directory: working directory must be an absolute path (got "relative/path")
```
**Solution:** Convert to absolute path using `filepath.Abs()`

### Error: Directory Does Not Exist
```
invalid session options: invalid working directory: working directory does not exist: /path/to/nonexistent
```
**Solution:** Create the directory first or use an existing directory

### Error: Path Is Not A Directory
```
invalid session options: invalid working directory: working directory path is not a directory: /path/to/file.txt
```
**Solution:** Provide a directory path, not a file path

## Best Practices

1. **Always handle validation errors:**
   ```go
   session, err := sdk.NewSession(ctx, opts)
   if err != nil {
       // Handle error - session was not created
       return fmt.Errorf("failed to create session: %w", err)
   }
   ```

2. **Use absolute paths for working directory:**
   ```go
   workspace, err := filepath.Abs("./workspace")
   if err != nil {
       return err
   }
   opts.WorkingDirectory = workspace
   ```

3. **Validate directory existence before creating session:**
   ```go
   if _, err := os.Stat(workspace); os.IsNotExist(err) {
       if err := os.MkdirAll(workspace, 0755); err != nil {
           return err
       }
   }
   ```

4. **Use appropriate sandbox policy for your use case:**
   - Development/testing: `native` or `workspace_write`
   - Production with untrusted input: `read_only`
   - Controlled environments: `workspace_write`

5. **Set approval policy based on security requirements:**
   - Interactive applications: `auto` or `always`
   - Automated workflows: `never` (with proper sandboxing)
   - User-facing tools: `always` (for transparency)

## Integration with Conversation Manager

All validation happens **before** the manager session is created, ensuring:
- No orphaned manager sessions from invalid options
- Clear error messages before any resources are allocated
- Fast-fail behavior for configuration errors
- Consistent state between SDK and manager sessions

## Thread Safety

All validation functions are stateless and thread-safe:
- Multiple goroutines can call `NewSession()` concurrently
- Validation does not modify global state
- Session creation is properly synchronized with mutex

## Performance

Validation is minimal and fast:
- File system checks only for working directory
- String comparisons for policies
- No network calls or expensive operations
- Typical validation time: < 1ms

## Conclusion

The validation system ensures:
- ✅ Secure default configurations
- ✅ Clear error messages for invalid options
- ✅ Prevention of runtime errors
- ✅ Consistent session behavior
- ✅ Integration with conversation manager
- ✅ Thread-safe operations
