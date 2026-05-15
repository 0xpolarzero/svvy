package mcp

import (
	"crypto/sha1"
	"fmt"
	"regexp"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

const (
	// MaxToolNameLength is the maximum length for tool names as required by OpenAI's API.
	// Tool names exceeding this limit will be truncated with a SHA1 hash appended.
	MaxToolNameLength = 64
)

var (
	// mcpServerNameRegex defines the valid pattern for MCP server names.
	// Server names must:
	// - Not be empty
	// - Contain only ASCII alphanumeric characters, underscores, and hyphens
	// This ensures safe tool name construction (mcp__<server-name>__<tool-name>).
	mcpServerNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// MCPResource represents a resource exposed by an MCP server
type MCPResource struct {
	URI         string                 `json:"uri"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	MimeType    string                 `json:"mimeType,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

// MCPResourceTemplate represents a resource template with URI patterns
type MCPResourceTemplate struct {
	URITemplate string                 `json:"uriTemplate"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	MimeType    string                 `json:"mimeType,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

// MCPResourceContents represents the contents of a resource
type MCPResourceContents struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"` // base64-encoded binary data
}

// isValidMCPServerName validates that a server name conforms to the required pattern.
// Server names must be non-empty and contain only ASCII alphanumeric characters,
// underscores, and hyphens. This ensures safe tool name construction.
//
// Valid examples: "my-server", "weather_api", "FileSystem2"
// Invalid examples: "", "my server", "api@service", "server.name"
func isValidMCPServerName(name string) bool {
	return name != "" && mcpServerNameRegex.MatchString(name)
}

// validateMCPServerName validates a server name and returns an error if invalid.
// This function is called during MCP manager initialization to ensure all server
// names are safe for tool name construction.
func validateMCPServerName(name string) error {
	if name == "" {
		return fmt.Errorf("MCP server name cannot be empty")
	}

	if !mcpServerNameRegex.MatchString(name) {
		return fmt.Errorf("MCP server name '%s' is invalid: must contain only ASCII alphanumeric characters, underscores, and hyphens (pattern: ^[a-zA-Z0-9_-]+$)", name)
	}

	return nil
}

// convertMCPSchema converts an MCP tool input schema to the runtime tool schema format.
// MCP schemas follow JSON Schema format, but we sanitize them to ensure compatibility
// with OpenAI's API requirements:
//   - Infer missing types from schema keywords
//   - Convert boolean schemas to proper objects
//   - Normalize Unicode strings to NFC form
//   - Add default values for required fields
func convertMCPSchema(mcpSchema map[string]interface{}) interface{} {
	// Sanitize the schema to handle edge cases from various MCP servers
	// This ensures compatibility with OpenAI's API expectations
	return sanitizeJSONSchema(mcpSchema)
}

// truncateToolName ensures the tool name doesn't exceed MaxToolNameLength.
// If the name is too long, it truncates and appends a SHA1 hash to ensure uniqueness.
func truncateToolName(toolName string) string {
	if len(toolName) <= MaxToolNameLength {
		return toolName
	}

	// Calculate SHA1 hash of the full name
	hasher := sha1.New()
	hasher.Write([]byte(toolName))
	sha1Hash := hasher.Sum(nil)
	sha1Str := fmt.Sprintf("%x", sha1Hash)

	// Calculate how much of the prefix we can keep
	// We need room for the SHA1 hash (40 chars)
	prefixLen := MaxToolNameLength - len(sha1Str)
	if prefixLen < 0 {
		prefixLen = 0
	}

	// Truncate the name and append the hash
	return toolName[:prefixLen] + sha1Str
}

// generateToolSpec creates a runtime.ToolSpec from an MCP tool definition.
// The tool name is prefixed with "mcp__<server-name>__" to avoid conflicts.
// If the resulting name exceeds MaxToolNameLength (64 chars), it will be truncated
// and a SHA1 hash will be appended to ensure uniqueness.
func generateToolSpec(serverName string, tool MCPTool) runtime.ToolSpec {
	// Generate unique tool name with MCP prefix
	toolName := fmt.Sprintf("mcp__%s__%s", serverName, tool.Name)

	// Ensure the tool name doesn't exceed the maximum length
	toolName = truncateToolName(toolName)

	// Build description with server context
	description := fmt.Sprintf("[MCP: %s] %s", serverName, tool.Description)
	if tool.Description == "" {
		description = fmt.Sprintf("[MCP: %s] Tool: %s", serverName, tool.Name)
	}

	// Convert schema
	var schema interface{}
	if tool.InputSchema != nil {
		schema = convertMCPSchema(tool.InputSchema)
	} else {
		// Default empty object schema
		schema = map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}

	return runtime.ToolSpec{
		Name:             toolName,
		Description:      description,
		ParametersSchema: schema,
		Strict:           false, // MCP tools may have flexible schemas
		SupportsParallel: true,  // MCP tools can run in parallel
	}
}

// validateMCPTool checks if an MCP tool definition is valid
func validateMCPTool(tool MCPTool) error {
	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	// Schema validation is now lenient - we'll sanitize it later
	// The sanitization process will infer types and fix structural issues
	// We only check that the schema is not fundamentally broken

	return nil
}

// formatToolError formats an MCP tool error for display to the AI model
func formatToolError(serverName, toolName string, err error) string {
	return fmt.Sprintf("Error calling MCP tool '%s' on server '%s': %v", toolName, serverName, err)
}

// formatToolResult formats the successful result from an MCP tool call
func formatToolResult(serverName, toolName, result string) string {
	// For now, just return the result directly
	// In the future, we could add metadata or formatting
	return result
}
