# MCP (Model Context Protocol) Integration

This package provides integration with external MCP servers, allowing Codex to dynamically discover and use tools from external services.

## Overview

The MCP integration enables:

- **Dynamic Tool Discovery**: Automatically discover tools from configured MCP servers
- **Multiple Transports**: Support for both stdio (subprocess) and HTTP transports
- **Concurrent Execution**: Safe concurrent execution of MCP tools
- **Tool Filtering**: Configure which tools to enable/disable per server
- **Timeout Management**: Configurable timeouts for initialization and tool execution
- **Authentication**: Support for bearer tokens and custom HTTP headers

## Architecture

```
┌─────────────┐
│ Orchestrator│
└──────┬──────┘
       │
       ├──> MCPManager
       │    ├──> MCPClient (Server 1) ──> stdio ──> MCP Process
       │    ├──> MCPClient (Server 2) ──> HTTP  ──> MCP HTTP Server
       │    └──> MCPClient (Server 3) ──> HTTP  ──> MCP HTTP Server
       │
       └──> MCPToolRuntime (per tool)
            └──> Forwards to MCPClient
```

### Components

1. **MCPManager**: Manages multiple MCP server connections and tool registration
2. **MCPClient**: Interface for stdio and HTTP transport implementations
3. **MCPToolRuntime**: Wrapper implementing the `runtime.ToolRuntime` interface
4. **Schema Mapping**: Converts MCP tool schemas to runtime tool specs

## Configuration

MCP servers are configured in `~/.codex/config.toml`:

### Stdio Transport Example

```toml
[mcp_servers.filesystem]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
enabled = true
startup_timeout_sec = 10.0
tool_timeout_sec = 30.0

# Optional: Only enable specific tools
enabled_tools = ["read_file", "write_file", "list_directory"]

# Optional: Disable specific tools
# disabled_tools = ["delete_file"]

# Optional: Environment variables
[mcp_servers.filesystem.env]
NODE_ENV = "production"

# Optional: Pass through environment variables
env_vars = ["HOME", "USER"]

# Optional: Working directory
cwd = "/tmp"
```

### HTTP Transport Example

```toml
[mcp_servers.weather]
url = "https://weather-mcp.example.com/rpc"
enabled = true
startup_timeout_sec = 5.0
tool_timeout_sec = 60.0

# Optional: Bearer token from environment variable
bearer_token_env_var = "WEATHER_API_KEY"

# Optional: Custom HTTP headers
[mcp_servers.weather.http_headers]
X-API-Version = "v1"

# Optional: Headers from environment variables
[mcp_servers.weather.env_http_headers]
Authorization = "CUSTOM_AUTH_TOKEN"
```

### Multiple Servers

```toml
[mcp_servers.github]
command = "mcp-server-github"
enabled = true

[mcp_servers.database]
url = "https://db-mcp.example.com/rpc"
enabled = true
bearer_token_env_var = "DB_MCP_TOKEN"

[mcp_servers.experimental]
command = "experimental-mcp"
enabled = false  # Disabled server
```

## Usage

### Initialization

```go
import (
    "context"
    "github.com/evmts/codex/codex-go/internal/config"
    "github.com/evmts/codex/codex-go/internal/tools/mcp"
    "github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// Load configuration
cfg, err := config.LoadConfig()
if err != nil {
    log.Fatal(err)
}

// Create MCP manager
mcpManager := mcp.NewMCPManager(cfg)

// Initialize all MCP server connections
ctx := context.Background()
if err := mcpManager.Initialize(ctx); err != nil {
    log.Fatal(err)
}
defer mcpManager.Close()

// Create tool registry and builder
registry := runtime.NewToolRegistry()
builder := runtime.NewToolRegistryBuilder()

// Register MCP tools
specs, err := mcpManager.RegisterTools(ctx, registry, builder)
if err != nil {
    log.Fatal(err)
}

log.Printf("Registered %d MCP tools", len(specs))
```

### Tool Execution

Once registered, MCP tools can be executed through the orchestrator like any other tool:

```go
// Create orchestrator
orchestrator := orchestrator.NewOrchestrator(
    registry,
    runtime.NewMemoryApprovalCache(),
    approvalHandler,
)

// Execute MCP tool
req := &runtime.ToolRequest{
    CallID:           "call-123",
    ToolName:         "mcp__weather__get_weather",
    Arguments:        `{"location": "San Francisco"}`,
    WorkingDirectory: "/workspace",
}

execCtx := &runtime.ExecutionContext{
    SessionID: "session-1",
    TurnID:    "turn-1",
    SandboxAttempt: &runtime.SandboxAttempt{
        Type:   runtime.SandboxNone,
        Policy: runtime.SandboxReadOnly,
    },
    ApprovalCache: runtime.NewMemoryApprovalCache(),
    StartTime:     time.Now(),
}

result, err := orchestrator.Execute(context.Background(), req, execCtx)
if err != nil {
    log.Printf("Tool execution failed: %v", err)
    return
}

log.Printf("Tool result: %s", result.Response.Content)
```

## Tool Naming Convention

MCP tools are registered with a prefix to avoid name conflicts:

```
Format: mcp__<server-name>__<tool-name>

Examples:
- mcp__filesystem__read_file
- mcp__weather__get_weather
- mcp__github__create_issue
```

## Tool Discovery Flow

1. **Configuration Loading**: Load MCP server configs from `config.toml`
2. **Client Initialization**: Create stdio or HTTP clients for each enabled server
3. **Connection Establishment**: Connect to each MCP server and perform handshake
4. **Tool Discovery**: Query each server for available tools via `tools/list`
5. **Tool Filtering**: Apply `enabled_tools` and `disabled_tools` filters
6. **Schema Conversion**: Convert MCP schemas to runtime tool specs
7. **Registration**: Register each tool with the orchestrator

## Tool Execution Flow

1. **AI Model Request**: Model calls a tool like `mcp__weather__get_weather`
2. **Runtime Lookup**: Orchestrator looks up the MCPToolRuntime
3. **Argument Parsing**: Parse JSON arguments from the request
4. **MCP Call**: Forward the call to the appropriate MCP server
5. **Result Processing**: Parse and format the MCP server response
6. **Response Return**: Return formatted result to the AI model

## Error Handling

The MCP integration handles various error scenarios:

### Server Connection Errors

```go
// Initialization timeout
if err := mcpManager.Initialize(ctx); err != nil {
    // Server failed to start or respond in time
    log.Printf("MCP initialization error: %v", err)
}
```

### Tool Execution Errors

```go
result, err := orchestrator.Execute(ctx, req, execCtx)
if err != nil {
    if toolErr, ok := err.(*runtime.ToolError); ok {
        switch toolErr.Kind {
        case runtime.ErrorExecution:
            // MCP server returned an error
            log.Printf("MCP tool error: %v", toolErr)
        case runtime.ErrorTimeout:
            // Tool execution timed out
            log.Printf("MCP tool timeout: %v", toolErr)
        case runtime.ErrorInvalidArguments:
            // Invalid arguments provided
            log.Printf("Invalid arguments: %v", toolErr)
        }
    }
}
```

### Malformed Responses

The client automatically handles:
- Invalid JSON responses
- Missing required fields
- HTTP errors (non-200 status codes)
- Protocol violations

## Testing

### Unit Tests

```bash
cd internal/tools/mcp
go test -v
```

### Integration Tests

The test suite includes:
- Mock stdio MCP servers
- Mock HTTP MCP servers
- Concurrent execution safety tests
- Error handling tests
- Schema conversion tests

### Running Tests with Coverage

```bash
go test -v -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## MCP Protocol

This implementation follows the Model Context Protocol specification:

### JSON-RPC 2.0 Format

```json
// Request
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "get_weather",
    "arguments": {
      "location": "San Francisco"
    }
  }
}

// Response
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Temperature: 72°F, Sunny"
      }
    ]
  }
}
```

### Supported Methods

- `initialize`: Protocol handshake
- `tools/list`: Discover available tools
- `tools/call`: Execute a tool

## Performance Considerations

### Concurrent Execution

MCP tools support parallel execution. The client implementations are thread-safe:

```go
// Multiple tools can execute concurrently
results, err := orchestrator.ExecuteParallel(ctx, requests, execCtx)
```

### Connection Pooling

For HTTP transport, the client uses Go's default HTTP client with connection pooling.

### Timeouts

Configure timeouts to prevent hanging:

```toml
[mcp_servers.myserver]
startup_timeout_sec = 10.0   # Server initialization
tool_timeout_sec = 30.0      # Individual tool calls
```

## Security Considerations

### Sandbox Isolation

MCP tools run externally and are not sandboxed by Codex. Ensure MCP servers are trusted.

### Authentication

Use bearer tokens for HTTP transport:

```toml
[mcp_servers.myserver]
url = "https://example.com/rpc"
bearer_token_env_var = "MCP_TOKEN"  # Reads from environment
```

### Tool Filtering

Disable dangerous tools:

```toml
[mcp_servers.filesystem]
command = "filesystem-mcp"
disabled_tools = ["delete_file", "format_disk"]
```

## Troubleshooting

### Server Won't Start

Check:
1. Command is in PATH
2. Arguments are correct
3. Working directory exists
4. Environment variables are set

Enable debug logging:
```bash
CODEX_LOG_LEVEL=debug codex
```

### Tools Not Appearing

Check:
1. Server is enabled: `enabled = true`
2. Tools are not in `disabled_tools`
3. If using `enabled_tools`, verify tool names match exactly

### Timeout Errors

Increase timeout values:
```toml
startup_timeout_sec = 30.0
tool_timeout_sec = 120.0
```

### HTTP Authentication Issues

Verify environment variable is set:
```bash
echo $MY_MCP_TOKEN
```

## Examples

### Example: Filesystem MCP Server

```toml
[mcp_servers.filesystem]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem", "/workspace"]
enabled = true
```

Tools available:
- `mcp__filesystem__read_file`
- `mcp__filesystem__write_file`
- `mcp__filesystem__list_directory`

### Example: Custom HTTP MCP Server

```toml
[mcp_servers.custom]
url = "http://localhost:8080/mcp"
enabled = true
tool_timeout_sec = 60.0
```

### Example: Multiple Filesystem Servers

```toml
[mcp_servers.workspace]
command = "filesystem-mcp"
args = ["/workspace"]
enabled = true

[mcp_servers.home]
command = "filesystem-mcp"
args = ["/home/user"]
enabled = true
```

Tools will be namespaced:
- `mcp__workspace__read_file`
- `mcp__home__read_file`

## Future Enhancements

Potential improvements:
- Streaming output support for long-running tools
- Tool result caching
- Server health monitoring
- Automatic reconnection on failure
- Tool usage analytics
- Resource discovery (in addition to tools)

## Contributing

When adding features to the MCP integration:

1. Write tests first (TDD)
2. Ensure thread safety
3. Handle errors gracefully
4. Update this README
5. Add configuration examples

## References

- [Model Context Protocol Specification](https://modelcontextprotocol.io)
- [MCP Server Examples](https://github.com/modelcontextprotocol/servers)
- [Codex Runtime Documentation](../runtime/README.md)
