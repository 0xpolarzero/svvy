// Package mcp provides Model Context Protocol (MCP) server integration for Codex.
//
// MCP allows external tool servers to be connected to Codex, enabling:
//   - Dynamic tool discovery from MCP servers
//   - Tool execution forwarding to MCP servers
//   - Support for both stdio and HTTP transports
//   - Concurrent-safe client implementations
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/evmts/codex/codex-go/internal/config"
	"github.com/evmts/codex/codex-go/internal/tools/mcp/oauth"
)

// MCPClient defines the interface for MCP server communication.
// Implementations handle both stdio and HTTP transports.
type MCPClient interface {
	// initialize establishes connection and performs protocol handshake
	initialize(ctx context.Context) error

	// listTools retrieves available tools from the MCP server
	listTools(ctx context.Context) ([]MCPTool, error)

	// callTool executes a tool with the given arguments
	callTool(ctx context.Context, name string, args map[string]interface{}) (string, error)

	// listResources retrieves available resources from the MCP server
	listResources(ctx context.Context) ([]MCPResource, error)

	// readResource reads a resource by URI from the MCP server
	readResource(ctx context.Context, uri string) (*MCPResourceContents, error)

	// listResourceTemplates retrieves available resource templates from the MCP server
	listResourceTemplates(ctx context.Context) ([]MCPResourceTemplate, error)

	// close terminates the connection and cleans up resources
	close() error

	// transportType returns the transport type for debugging
	transportType() string
}

// MCPTool represents a tool exposed by an MCP server
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// jsonrpcRequest represents a JSON-RPC 2.0 request
type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonrpcResponse represents a JSON-RPC 2.0 response
type jsonrpcResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *jsonrpcError `json:"error,omitempty"`
}

// jsonrpcError represents a JSON-RPC error
type jsonrpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// stdioClient implements MCPClient using stdio transport
type stdioClient struct {
	config    config.MCPServerConfig
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	reader    *bufio.Reader
	mu        sync.Mutex
	nextID    int
	stderrBuf strings.Builder // buffer for stderr output
}

// newStdioClient creates a new stdio-based MCP client
func newStdioClient(cfg config.MCPServerConfig) *stdioClient {
	return &stdioClient{
		config: cfg,
		nextID: 1,
	}
}

func (c *stdioClient) initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Set up timeout
	timeout := 30 * time.Second
	if c.config.StartupTimeoutSec != nil {
		timeout = time.Duration(*c.config.StartupTimeoutSec * float64(time.Second))
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build command
	cmd := exec.CommandContext(ctx, c.config.Command, c.config.Args...)

	// Set working directory
	if c.config.CWD != "" {
		cmd.Dir = c.config.CWD
	}

	// Set up environment
	cmd.Env = os.Environ()
	if c.config.Env != nil {
		for k, v := range c.config.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	if c.config.EnvVars != nil {
		for _, envVar := range c.config.EnvVars {
			if val := os.Getenv(envVar); val != "" {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", envVar, val))
			}
		}
	}

	// Set up pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stdout = stdout
	c.stderr = stderr
	c.reader = bufio.NewReader(stdout)

	// Start goroutine to consume stderr to prevent deadlock
	// Stderr must be continuously read or it can fill up the pipe buffer
	// and cause the child process to block
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				c.mu.Lock()
				c.stderrBuf.Write(buf[:n])
				c.mu.Unlock()
			}
			if err != nil {
				// EOF or error means pipe closed, exit goroutine
				break
			}
		}
	}()

	// Send initialize request
	initReq := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "1.0",
			"clientInfo": map[string]interface{}{
				"name":    "codex-go",
				"version": "0.1.0",
			},
		},
	}
	c.nextID++

	resp, err := c.sendRequestLocked(ctx, initReq)
	if err != nil {
		_ = c.close() // nolint:errcheck // Best effort cleanup
		return fmt.Errorf("failed to initialize MCP server: %w", err)
	}

	if resp.Error != nil {
		_ = c.close() // nolint:errcheck // Best effort cleanup
		return fmt.Errorf("MCP server initialization error: %s", resp.Error.Message)
	}

	return nil
}

func (c *stdioClient) listTools(ctx context.Context) ([]MCPTool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "tools/list",
	}
	c.nextID++

	resp, err := c.sendRequestLocked(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("MCP server error: %s", resp.Error.Message)
	}

	// Parse tools from response
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	toolsData, ok := result["tools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tools format")
	}

	tools := make([]MCPTool, 0, len(toolsData))
	for _, toolData := range toolsData {
		toolMap, ok := toolData.(map[string]interface{})
		if !ok {
			continue
		}

		tool := MCPTool{
			Name:        getString(toolMap, "name"),
			Description: getString(toolMap, "description"),
		}

		if schema, ok := toolMap["inputSchema"].(map[string]interface{}); ok {
			tool.InputSchema = schema
		}

		tools = append(tools, tool)
	}

	return tools, nil
}

func (c *stdioClient) listResources(ctx context.Context) ([]MCPResource, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "resources/list",
	}
	c.nextID++

	resp, err := c.sendRequestLocked(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("MCP server error: %s", resp.Error.Message)
	}

	// Parse resources from response
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	resourcesData, ok := result["resources"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid resources format")
	}

	resources := make([]MCPResource, 0, len(resourcesData))
	for _, resourceData := range resourcesData {
		resourceMap, ok := resourceData.(map[string]interface{})
		if !ok {
			continue
		}

		resource := MCPResource{
			URI:         getString(resourceMap, "uri"),
			Name:        getString(resourceMap, "name"),
			Description: getString(resourceMap, "description"),
			MimeType:    getString(resourceMap, "mimeType"),
		}

		if annotations, ok := resourceMap["annotations"].(map[string]interface{}); ok {
			resource.Annotations = annotations
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

func (c *stdioClient) readResource(ctx context.Context, uri string) (*MCPResourceContents, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "resources/read",
		Params: map[string]interface{}{
			"uri": uri,
		},
	}
	c.nextID++

	resp, err := c.sendRequestLocked(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to read resource: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("MCP server error: %s", resp.Error.Message)
	}

	// Parse resource contents from response
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	contentsData, ok := result["contents"].([]interface{})
	if !ok || len(contentsData) == 0 {
		return nil, fmt.Errorf("invalid or empty contents format")
	}

	// Take the first content item
	contentMap, ok := contentsData[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid content format")
	}

	contents := &MCPResourceContents{
		URI:      getString(contentMap, "uri"),
		MimeType: getString(contentMap, "mimeType"),
		Text:     getString(contentMap, "text"),
		Blob:     getString(contentMap, "blob"),
	}

	return contents, nil
}

func (c *stdioClient) listResourceTemplates(ctx context.Context) ([]MCPResourceTemplate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "resources/templates/list",
	}
	c.nextID++

	resp, err := c.sendRequestLocked(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list resource templates: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("MCP server error: %s", resp.Error.Message)
	}

	// Parse resource templates from response
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	templatesData, ok := result["resourceTemplates"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid resource templates format")
	}

	templates := make([]MCPResourceTemplate, 0, len(templatesData))
	for _, templateData := range templatesData {
		templateMap, ok := templateData.(map[string]interface{})
		if !ok {
			continue
		}

		template := MCPResourceTemplate{
			URITemplate: getString(templateMap, "uriTemplate"),
			Name:        getString(templateMap, "name"),
			Description: getString(templateMap, "description"),
			MimeType:    getString(templateMap, "mimeType"),
		}

		if annotations, ok := templateMap["annotations"].(map[string]interface{}); ok {
			template.Annotations = annotations
		}

		templates = append(templates, template)
	}

	return templates, nil
}

func (c *stdioClient) callTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Apply tool timeout if configured
	if c.config.ToolTimeoutSec != nil {
		timeout := time.Duration(*c.config.ToolTimeoutSec * float64(time.Second))
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      name,
			"arguments": args,
		},
	}
	c.nextID++

	resp, err := c.sendRequestLocked(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to call tool: %w", err)
	}

	if resp.Error != nil {
		return "", fmt.Errorf("MCP tool error: %s", resp.Error.Message)
	}

	// Parse result
	return parseToolResult(resp.Result)
}

func (c *stdioClient) sendRequestLocked(ctx context.Context, req jsonrpcRequest) (*jsonrpcResponse, error) {
	// Encode and send request
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	reqData = append(reqData, '\n')

	// Check context before sending
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if _, err := c.stdin.Write(reqData); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response with context
	respChan := make(chan *jsonrpcResponse, 1)
	errChan := make(chan error, 1)

	go func() {
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			errChan <- fmt.Errorf("failed to read response: %w", err)
			return
		}

		var resp jsonrpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			errChan <- fmt.Errorf("failed to decode response: %w", err)
			return
		}

		respChan <- &resp
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errChan:
		return nil, err
	case resp := <-respChan:
		return resp, nil
	}
}

func (c *stdioClient) close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error

	// Close stdin first to signal the process we're done
	if c.stdin != nil {
		if err := c.stdin.Close(); err != nil {
			errs = append(errs, err)
		}
		c.stdin = nil
	}

	// Close stdout to stop any pending reads
	if c.stdout != nil {
		if err := c.stdout.Close(); err != nil {
			errs = append(errs, err)
		}
		c.stdout = nil
	}

	// Close stderr to stop the consumption goroutine
	if c.stderr != nil {
		if err := c.stderr.Close(); err != nil {
			errs = append(errs, err)
		}
		c.stderr = nil
	}

	// Kill the process if it's still running
	if c.cmd != nil && c.cmd.Process != nil {
		if err := c.cmd.Process.Kill(); err != nil {
			// Ignore "already finished" errors
			if err.Error() != "os: process already finished" {
				errs = append(errs, fmt.Errorf("kill process: %w", err))
			}
		}

		// Wait for the process to exit to reap the zombie process
		// This is critical to prevent process leaks
		if err := c.cmd.Wait(); err != nil {
			// Ignore exit errors since we just killed the process
			// We only care that Wait() was called to reap the process
			// Don't report errors like "signal: killed" or non-zero exit codes
			// since we intentionally killed the process
			_ = err // Expected error after kill, ignore it
		}
		c.cmd = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}

	return nil
}

func (c *stdioClient) transportType() string {
	return "stdio"
}

// httpClient implements MCPClient using HTTP transport
type httpClient struct {
	config       config.MCPServerConfig
	httpClient   *http.Client
	tokenManager *oauth.TokenManager
	serverName   string
	mu           sync.Mutex
	nextID       int
}

// newHTTPClient creates a new HTTP-based MCP client
func newHTTPClient(cfg config.MCPServerConfig) *httpClient {
	return &httpClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		nextID: 1,
	}
}

// newHTTPClientWithOAuth creates a new HTTP-based MCP client with OAuth support
func newHTTPClientWithOAuth(cfg config.MCPServerConfig, serverName string, tokenManager *oauth.TokenManager) *httpClient {
	return &httpClient{
		config:       cfg,
		serverName:   serverName,
		tokenManager: tokenManager,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		nextID: 1,
	}
}

func (c *httpClient) initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Set up timeout
	timeout := 30 * time.Second
	if c.config.StartupTimeoutSec != nil {
		timeout = time.Duration(*c.config.StartupTimeoutSec * float64(time.Second))
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Send initialize request
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "1.0",
			"clientInfo": map[string]interface{}{
				"name":    "codex-go",
				"version": "0.1.0",
			},
		},
	}
	c.nextID++

	resp, err := c.sendRequestLocked(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to initialize MCP server: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("MCP server initialization error: %s", resp.Error.Message)
	}

	return nil
}

func (c *httpClient) listTools(ctx context.Context) ([]MCPTool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "tools/list",
	}
	c.nextID++

	resp, err := c.sendRequestLocked(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("MCP server error: %s", resp.Error.Message)
	}

	// Parse tools from response
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	toolsData, ok := result["tools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tools format")
	}

	tools := make([]MCPTool, 0, len(toolsData))
	for _, toolData := range toolsData {
		toolMap, ok := toolData.(map[string]interface{})
		if !ok {
			continue
		}

		tool := MCPTool{
			Name:        getString(toolMap, "name"),
			Description: getString(toolMap, "description"),
		}

		if schema, ok := toolMap["inputSchema"].(map[string]interface{}); ok {
			tool.InputSchema = schema
		}

		tools = append(tools, tool)
	}

	return tools, nil
}

func (c *httpClient) listResources(ctx context.Context) ([]MCPResource, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "resources/list",
	}
	c.nextID++

	resp, err := c.sendRequestLocked(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("MCP server error: %s", resp.Error.Message)
	}

	// Parse resources from response
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	resourcesData, ok := result["resources"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid resources format")
	}

	resources := make([]MCPResource, 0, len(resourcesData))
	for _, resourceData := range resourcesData {
		resourceMap, ok := resourceData.(map[string]interface{})
		if !ok {
			continue
		}

		resource := MCPResource{
			URI:         getString(resourceMap, "uri"),
			Name:        getString(resourceMap, "name"),
			Description: getString(resourceMap, "description"),
			MimeType:    getString(resourceMap, "mimeType"),
		}

		if annotations, ok := resourceMap["annotations"].(map[string]interface{}); ok {
			resource.Annotations = annotations
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

func (c *httpClient) readResource(ctx context.Context, uri string) (*MCPResourceContents, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "resources/read",
		Params: map[string]interface{}{
			"uri": uri,
		},
	}
	c.nextID++

	resp, err := c.sendRequestLocked(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to read resource: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("MCP server error: %s", resp.Error.Message)
	}

	// Parse resource contents from response
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	contentsData, ok := result["contents"].([]interface{})
	if !ok || len(contentsData) == 0 {
		return nil, fmt.Errorf("invalid or empty contents format")
	}

	// Take the first content item
	contentMap, ok := contentsData[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid content format")
	}

	contents := &MCPResourceContents{
		URI:      getString(contentMap, "uri"),
		MimeType: getString(contentMap, "mimeType"),
		Text:     getString(contentMap, "text"),
		Blob:     getString(contentMap, "blob"),
	}

	return contents, nil
}

func (c *httpClient) listResourceTemplates(ctx context.Context) ([]MCPResourceTemplate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "resources/templates/list",
	}
	c.nextID++

	resp, err := c.sendRequestLocked(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list resource templates: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("MCP server error: %s", resp.Error.Message)
	}

	// Parse resource templates from response
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	templatesData, ok := result["resourceTemplates"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid resource templates format")
	}

	templates := make([]MCPResourceTemplate, 0, len(templatesData))
	for _, templateData := range templatesData {
		templateMap, ok := templateData.(map[string]interface{})
		if !ok {
			continue
		}

		template := MCPResourceTemplate{
			URITemplate: getString(templateMap, "uriTemplate"),
			Name:        getString(templateMap, "name"),
			Description: getString(templateMap, "description"),
			MimeType:    getString(templateMap, "mimeType"),
		}

		if annotations, ok := templateMap["annotations"].(map[string]interface{}); ok {
			template.Annotations = annotations
		}

		templates = append(templates, template)
	}

	return templates, nil
}

func (c *httpClient) callTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Apply tool timeout if configured
	if c.config.ToolTimeoutSec != nil {
		timeout := time.Duration(*c.config.ToolTimeoutSec * float64(time.Second))
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      name,
			"arguments": args,
		},
	}
	c.nextID++

	resp, err := c.sendRequestLocked(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to call tool: %w", err)
	}

	if resp.Error != nil {
		return "", fmt.Errorf("MCP tool error: %s", resp.Error.Message)
	}

	// Parse result
	return parseToolResult(resp.Result)
}

func (c *httpClient) sendRequestLocked(ctx context.Context, req jsonrpcRequest) (*jsonrpcResponse, error) {
	// Encode request
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.config.URL, strings.NewReader(string(reqData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Try OAuth token first if token manager is available
	if c.tokenManager != nil && c.serverName != "" && c.config.URL != "" {
		token, err := c.tokenManager.GetToken(c.serverName, c.config.URL)
		if err == nil && token != nil {
			// Inject OAuth token
			if err := oauth.InjectOAuthHeader(httpReq, token); err != nil {
				// Log warning but continue with request
				// Token might be refreshed on next attempt
			}
		}
	}

	// Add bearer token if configured (fallback if OAuth not available)
	if c.config.BearerTokenEnvVar != "" {
		token := os.Getenv(c.config.BearerTokenEnvVar)
		if token != "" {
			httpReq.Header.Set("Authorization", "Bearer "+token)
		}
	}

	// Add custom headers
	if c.config.HTTPHeaders != nil {
		for k, v := range c.config.HTTPHeaders {
			httpReq.Header.Set(k, v)
		}
	}

	// Add environment-based headers
	if c.config.EnvHTTPHeaders != nil {
		for k, envVar := range c.config.EnvHTTPHeaders {
			if val := os.Getenv(envVar); val != "" {
				httpReq.Header.Set(k, val)
			}
		}
	}

	// Send request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(httpResp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("HTTP error %d and failed to read body: %v", httpResp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("HTTP error %d: %s", httpResp.StatusCode, string(body))
	}

	// Decode response
	var resp jsonrpcResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &resp, nil
}

func (c *httpClient) close() error {
	// HTTP client doesn't need explicit cleanup
	return nil
}

func (c *httpClient) transportType() string {
	return "http"
}

// Helper functions

func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func parseToolResult(result interface{}) (string, error) {
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		// If result is a string, return it directly
		if str, ok := result.(string); ok {
			return str, nil
		}
		return "", fmt.Errorf("invalid result format")
	}

	// Handle content array format
	if content, ok := resultMap["content"].([]interface{}); ok {
		var parts []string
		for _, item := range content {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if text, ok := itemMap["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n"), nil
	}

	// Handle direct text format
	if text, ok := resultMap["text"].(string); ok {
		return text, nil
	}

	// Fallback: marshal to JSON
	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(data), nil
}
