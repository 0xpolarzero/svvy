// Package mcp provides Model Context Protocol (MCP) integration for Codex Go.
//
// This package enables Codex to connect to external MCP servers and use their tools.
// MCP servers can be configured in the config.toml file and are automatically
// discovered and registered with the tool orchestrator.
package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/evmts/codex/codex-go/internal/config"
	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// MCPToolRuntime implements the runtime.ToolRuntime interface for MCP tools.
// Each MCP tool gets its own runtime instance that forwards calls to the MCP server.
type MCPToolRuntime struct {
	serverName string
	tool       MCPTool
	client     MCPClient
}

// NewMCPToolRuntime creates a new MCP tool runtime wrapper
func NewMCPToolRuntime(serverName string, tool MCPTool, client MCPClient) *MCPToolRuntime {
	return &MCPToolRuntime{
		serverName: serverName,
		tool:       tool,
		client:     client,
	}
}

// Name returns the unique identifier for this tool.
// Format: mcp__<server-name>__<tool-name>
func (m *MCPToolRuntime) Name() string {
	return fmt.Sprintf("mcp__%s__%s", m.serverName, m.tool.Name)
}

// Execute runs the MCP tool by forwarding the request to the MCP server.
func (m *MCPToolRuntime) Execute(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
	startTime := time.Now()

	// Parse arguments
	var args map[string]interface{}
	if req.Arguments != "" {
		if err := json.Unmarshal([]byte(req.Arguments), &args); err != nil {
			return nil, runtime.NewToolErrorWithCause(
				runtime.ErrorInvalidArguments,
				fmt.Sprintf("failed to parse arguments for MCP tool %s", m.tool.Name),
				err,
			)
		}
	} else {
		args = make(map[string]interface{})
	}

	// Call the MCP server
	result, err := m.client.callTool(ctx, m.tool.Name, args)
	if err != nil {
		return nil, runtime.NewToolErrorWithCause(
			runtime.ErrorExecution,
			formatToolError(m.serverName, m.tool.Name, err),
			err,
		)
	}

	// Format result
	formattedResult := formatToolResult(m.serverName, m.tool.Name, result)

	success := true
	return &runtime.ToolResponse{
		Content:       formattedResult,
		Success:       &success,
		ExecutionTime: time.Since(startTime),
		Metadata: map[string]interface{}{
			"mcp_server": m.serverName,
			"mcp_tool":   m.tool.Name,
		},
	}, nil
}

// ApprovalKey generates a unique key for caching approval decisions.
func (m *MCPToolRuntime) ApprovalKey(req *runtime.ToolRequest) string {
	// Create a key from server, tool name, and arguments
	key := fmt.Sprintf("mcp:%s:%s:%s", m.serverName, m.tool.Name, req.Arguments)

	// Hash for consistent length
	hash := sha256.Sum256([]byte(key))
	return "mcp:" + hex.EncodeToString(hash[:8])
}

// NeedsInitialApproval determines if approval is required before execution.
// MCP tools generally don't require approval unless the policy demands it.
func (m *MCPToolRuntime) NeedsInitialApproval(req *runtime.ToolRequest, approvalPolicy runtime.ApprovalPolicy, sandboxPolicy runtime.SandboxPolicy) bool {
	// MCP tools are external, so they follow standard approval policies
	// Never policy means no approval
	if approvalPolicy == runtime.ApprovalNever {
		return false
	}

	// Danger full access mode doesn't need approval for on-request policy
	if approvalPolicy == runtime.ApprovalOnRequest && sandboxPolicy == runtime.SandboxDangerFullAccess {
		return false
	}

	// Unless-trusted policy requires approval for external tools
	if approvalPolicy == runtime.ApprovalUnlessTrusted {
		return true
	}

	// On-request policy doesn't require approval for MCP tools by default
	// (they are considered safe as they run externally)
	return false
}

// NeedsRetryApproval determines if approval is required before retrying.
// MCP tools don't use sandbox retry logic.
func (m *MCPToolRuntime) NeedsRetryApproval(approvalPolicy runtime.ApprovalPolicy) bool {
	return false
}

// SandboxPreference indicates that MCP tools run externally and don't need sandboxing.
func (m *MCPToolRuntime) SandboxPreference() runtime.SandboxPreference {
	return runtime.SandboxAuto
}

// EscalateOnFailure returns false as MCP tools don't use escalation.
func (m *MCPToolRuntime) EscalateOnFailure() bool {
	return false
}

// WantsEscalatedFirstAttempt returns false as MCP tools run externally.
func (m *MCPToolRuntime) WantsEscalatedFirstAttempt(req *runtime.ToolRequest) bool {
	return false
}

// SupportsParallel returns true as MCP tools can run concurrently.
func (m *MCPToolRuntime) SupportsParallel() bool {
	return true
}

// SandboxRetryData returns nil as MCP tools don't use sandbox retry.
func (m *MCPToolRuntime) SandboxRetryData(req *runtime.ToolRequest) *runtime.SandboxRetryData {
	return nil
}

// MCPManager manages multiple MCP servers and their tools
type MCPManager struct {
	config  *config.Config
	clients map[string]MCPClient
}

// NewMCPManager creates a new MCP manager from configuration
func NewMCPManager(cfg *config.Config) *MCPManager {
	manager := &MCPManager{
		config:  cfg,
		clients: make(map[string]MCPClient),
	}

	// Initialize clients for enabled servers
	for name, serverCfg := range cfg.MCPServers {
		if !serverCfg.Enabled {
			continue
		}

		// Validate server name before creating client
		if err := validateMCPServerName(name); err != nil {
			// Skip invalid server names to prevent unsafe tool name construction
			// This is a configuration error that should be fixed in config.toml
			continue
		}

		var client MCPClient
		if serverCfg.URL != "" {
			// HTTP transport
			client = newHTTPClient(serverCfg)
		} else if serverCfg.Command != "" {
			// Stdio transport
			client = newStdioClient(serverCfg)
		} else {
			// Invalid config, skip
			continue
		}

		manager.clients[name] = client
	}

	return manager
}

// Initialize initializes all MCP server connections
func (m *MCPManager) Initialize(ctx context.Context) error {
	for name, client := range m.clients {
		if err := client.initialize(ctx); err != nil {
			return fmt.Errorf("failed to initialize MCP server %s: %w", name, err)
		}
	}
	return nil
}

// RegisterTools discovers tools from all MCP servers and registers them with the orchestrator
func (m *MCPManager) RegisterTools(ctx context.Context, registry *runtime.ToolRegistry, builder *runtime.ToolRegistryBuilder) ([]runtime.ToolSpec, error) {
	var allSpecs []runtime.ToolSpec

	for serverName, client := range m.clients {
		// List tools from this server
		tools, err := client.listTools(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list tools from MCP server %s: %w", serverName, err)
		}

		// Filter tools based on configuration
		tools = m.filterTools(serverName, tools)

		// Register each tool
		for _, tool := range tools {
			// Validate tool
			if err := validateMCPTool(tool); err != nil {
				// Skip invalid tools
				continue
			}

			// Create runtime wrapper
			mcpRuntime := NewMCPToolRuntime(serverName, tool, client)

			// Generate tool spec
			spec := generateToolSpec(serverName, tool)

			// Register with builder
			builder.RegisterTool(mcpRuntime, spec)

			allSpecs = append(allSpecs, spec)
		}
	}

	return allSpecs, nil
}

// filterTools applies enabled_tools and disabled_tools filters from configuration
func (m *MCPManager) filterTools(serverName string, tools []MCPTool) []MCPTool {
	serverCfg, ok := m.config.MCPServers[serverName]
	if !ok {
		return tools
	}

	// If enabled_tools is specified, only include those
	if len(serverCfg.EnabledTools) > 0 {
		enabledSet := make(map[string]bool)
		for _, name := range serverCfg.EnabledTools {
			enabledSet[name] = true
		}

		filtered := make([]MCPTool, 0)
		for _, tool := range tools {
			if enabledSet[tool.Name] {
				filtered = append(filtered, tool)
			}
		}
		return filtered
	}

	// If disabled_tools is specified, exclude those
	if len(serverCfg.DisabledTools) > 0 {
		disabledSet := make(map[string]bool)
		for _, name := range serverCfg.DisabledTools {
			disabledSet[name] = true
		}

		filtered := make([]MCPTool, 0)
		for _, tool := range tools {
			if !disabledSet[tool.Name] {
				filtered = append(filtered, tool)
			}
		}
		return filtered
	}

	// No filters, return all tools
	return tools
}

// Close closes all MCP server connections
func (m *MCPManager) Close() error {
	var errs []error
	for name, client := range m.clients {
		if err := client.close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close MCP server %s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing MCP servers: %v", errs)
	}

	return nil
}

// GetClient returns the MCP client for a given server name
func (m *MCPManager) GetClient(serverName string) MCPClient {
	return m.clients[serverName]
}

// ListServers returns the names of all active MCP servers
func (m *MCPManager) ListServers() []string {
	names := make([]string, 0, len(m.clients))
	for name := range m.clients {
		names = append(names, name)
	}
	return names
}
