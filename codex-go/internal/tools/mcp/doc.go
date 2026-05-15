// Package mcp provides Model Context Protocol (MCP) integration for Codex Go.
//
// MCP enables external tool servers to integrate with Codex, allowing dynamic
// discovery and execution of tools from various sources.
//
// # Overview
//
// The MCP package provides:
//   - Stdio and HTTP transport support for MCP servers
//   - Automatic tool discovery and registration
//   - Concurrent-safe client implementations
//   - Tool filtering and configuration
//   - Comprehensive error handling
//
// # Quick Start
//
// Configure MCP servers in ~/.codex/config.toml:
//
//	[mcp_servers.filesystem]
//	command = "npx"
//	args = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
//	enabled = true
//
//	[mcp_servers.weather]
//	url = "https://weather-mcp.example.com/rpc"
//	enabled = true
//	bearer_token_env_var = "WEATHER_API_KEY"
//
// # Usage Example
//
//	package main
//
//	import (
//	    "context"
//	    "log"
//
//	    "github.com/evmts/codex/codex-go/internal/config"
//	    "github.com/evmts/codex/codex-go/internal/tools/mcp"
//	    "github.com/evmts/codex/codex-go/internal/tools/runtime"
//	    "github.com/evmts/codex/codex-go/internal/tools/orchestrator"
//	)
//
//	func main() {
//	    // Load configuration
//	    cfg, err := config.LoadConfig()
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//
//	    // Create MCP manager
//	    mcpManager := mcp.NewMCPManager(cfg)
//	    defer mcpManager.Close()
//
//	    // Initialize all MCP servers
//	    ctx := context.Background()
//	    if err := mcpManager.Initialize(ctx); err != nil {
//	        log.Fatal(err)
//	    }
//
//	    // Create tool registry
//	    builder := runtime.NewToolRegistryBuilder()
//
//	    // Register MCP tools
//	    specs, err := mcpManager.RegisterTools(ctx, nil, builder)
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//
//	    log.Printf("Registered %d MCP tools", len(specs))
//
//	    // Build registry
//	    registry, toolSpecs := builder.Build()
//
//	    // Create orchestrator
//	    orch := orchestrator.NewOrchestrator(
//	        registry,
//	        runtime.NewMemoryApprovalCache(),
//	        nil, // approval handler
//	    )
//
//	    // Execute an MCP tool
//	    req := &runtime.ToolRequest{
//	        CallID:           "call-1",
//	        ToolName:         "mcp__filesystem__read_file",
//	        Arguments:        `{"path": "/tmp/test.txt"}`,
//	        WorkingDirectory: "/tmp",
//	    }
//
//	    execCtx := &runtime.ExecutionContext{
//	        SessionID: "session-1",
//	        TurnID:    "turn-1",
//	        SandboxAttempt: &runtime.SandboxAttempt{
//	            Type:   runtime.SandboxNone,
//	            Policy: runtime.SandboxReadOnly,
//	        },
//	        ApprovalCache: runtime.NewMemoryApprovalCache(),
//	    }
//
//	    result, err := orch.Execute(ctx, req, execCtx)
//	    if err != nil {
//	        log.Printf("Execution failed: %v", err)
//	        return
//	    }
//
//	    log.Printf("Result: %s", result.Response.Content)
//	}
//
// # Tool Naming
//
// MCP tools are automatically prefixed to avoid conflicts:
//
//	Format: mcp__<server-name>__<tool-name>
//
//	Examples:
//	  - mcp__filesystem__read_file
//	  - mcp__weather__get_weather
//	  - mcp__github__create_issue
//
// # Transport Types
//
// Stdio Transport:
//   - Launches MCP server as subprocess
//   - Communicates via stdin/stdout
//   - Supports environment variables
//   - Automatic process lifecycle management
//
// HTTP Transport:
//   - Connects to MCP HTTP endpoint
//   - Supports bearer token authentication
//   - Custom HTTP headers
//   - Connection pooling
//
// # Configuration Options
//
// Per-server configuration:
//
//	[mcp_servers.myserver]
//	command = "mcp-server-binary"           # Executable path
//	args = ["--option", "value"]            # Command arguments
//	cwd = "/working/directory"              # Working directory
//	enabled = true                          # Enable/disable server
//	startup_timeout_sec = 10.0              # Initialization timeout
//	tool_timeout_sec = 30.0                 # Per-tool timeout
//	enabled_tools = ["tool1", "tool2"]      # Whitelist tools
//	disabled_tools = ["dangerous_tool"]     # Blacklist tools
//
//	[mcp_servers.myserver.env]              # Static environment vars
//	KEY = "value"
//
//	env_vars = ["HOME", "USER"]             # Pass-through env vars
//
// For HTTP transport:
//
//	[mcp_servers.myhttp]
//	url = "https://example.com/mcp"
//	bearer_token_env_var = "MCP_TOKEN"
//
//	[mcp_servers.myhttp.http_headers]
//	X-Custom-Header = "value"
//
//	[mcp_servers.myhttp.env_http_headers]   # Headers from env
//	Authorization = "AUTH_TOKEN_VAR"
//
// # Error Handling
//
// The package provides structured error handling:
//
//	result, err := orch.Execute(ctx, req, execCtx)
//	if err != nil {
//	    if toolErr, ok := err.(*runtime.ToolError); ok {
//	        switch toolErr.Kind {
//	        case runtime.ErrorExecution:
//	            // MCP server returned error
//	        case runtime.ErrorTimeout:
//	            // Operation timed out
//	        case runtime.ErrorInvalidArguments:
//	            // Invalid tool arguments
//	        }
//	    }
//	}
//
// # Concurrency
//
// All client implementations are thread-safe and support concurrent execution:
//
//	// Execute multiple MCP tools concurrently
//	results, err := orch.ExecuteParallel(ctx, requests, execCtx)
//
// # Testing
//
// The package includes comprehensive tests with mock MCP servers:
//
//	go test -v github.com/evmts/codex/codex-go/internal/tools/mcp
//
// Run with coverage:
//
//	go test -coverprofile=coverage.out
//	go tool cover -html=coverage.out
//
// # Protocol Compliance
//
// This implementation follows the Model Context Protocol specification:
//   - JSON-RPC 2.0 message format
//   - Standard method names (initialize, tools/list, tools/call)
//   - Structured tool schemas (JSON Schema format)
//   - Content block responses
//
// # Security Considerations
//
//   - MCP tools run externally and are not sandboxed by Codex
//   - Use tool filtering to disable dangerous operations
//   - Validate MCP server sources before enabling
//   - Use bearer tokens for HTTP transport authentication
//   - Environment variables should not contain secrets in logs
//
// For detailed documentation, see README.md in this package.
package mcp
