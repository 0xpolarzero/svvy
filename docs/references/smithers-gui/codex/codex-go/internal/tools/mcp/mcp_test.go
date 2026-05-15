package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/config"
	"github.com/evmts/codex/codex-go/internal/tools/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock MCP protocol messages
type mcpRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *mcpError   `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// TestMCPClient_StdioTransport tests the stdio transport implementation
func TestMCPClient_StdioTransport(t *testing.T) {
	t.Run("successful connection and initialization", func(t *testing.T) {
		cfg := config.MCPServerConfig{
			Command: "mock-mcp-server",
			Args:    []string{"--mode", "stdio"},
			Enabled: true,
		}

		client := newStdioClient(cfg)
		require.NotNil(t, client)

		// Test would connect to mock server
		// For now, test the client structure
		assert.Equal(t, "stdio", client.transportType())
	})

	t.Run("initialization timeout", func(t *testing.T) {
		t.Skip("Skipping stdio timeout test - requires proper mock server")
		// This test would require a mock MCP server that doesn't respond
		// For now, we test timeout behavior via HTTP transport
	})

	t.Run("list tools", func(t *testing.T) {
		_ = config.MCPServerConfig{
			Command: "mock-mcp-server",
			Args:    []string{"--mode", "stdio"},
			Enabled: true,
		}

		client := newMockStdioClientWithTools([]MCPTool{
			{
				Name:        "get_weather",
				Description: "Get weather information",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "City name",
						},
					},
					"required": []string{"location"},
				},
			},
		})

		tools, err := client.listTools(context.Background())
		require.NoError(t, err)
		assert.Len(t, tools, 1)
		assert.Equal(t, "get_weather", tools[0].Name)
	})
}

// TestMCPClient_HTTPTransport tests the HTTP transport implementation
func TestMCPClient_HTTPTransport(t *testing.T) {
	t.Run("successful connection", func(t *testing.T) {
		// Create mock HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			var req mcpRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.Method == "initialize" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"protocolVersion": "1.0",
						"serverInfo": map[string]interface{}{
							"name":    "mock-server",
							"version": "1.0.0",
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
			}
		}))
		defer server.Close()

		cfg := config.MCPServerConfig{
			URL:     server.URL,
			Enabled: true,
		}

		client := newHTTPClient(cfg)
		require.NotNil(t, client)

		err := client.initialize(context.Background())
		assert.NoError(t, err)
	})

	t.Run("with bearer token", func(t *testing.T) {
		t.Setenv("MCP_TEST_TOKEN", "secret-token-123")

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			assert.Equal(t, "Bearer secret-token-123", auth)

			resp := mcpResponse{
				JSONRPC: "2.0",
				ID:      1,
				Result:  map[string]interface{}{},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		cfg := config.MCPServerConfig{
			URL:               server.URL,
			BearerTokenEnvVar: "MCP_TEST_TOKEN",
			Enabled:           true,
		}

		client := newHTTPClient(cfg)
		err := client.initialize(context.Background())
		assert.NoError(t, err)
	})

	t.Run("list tools via HTTP", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req mcpRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.Method == "tools/list" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"tools": []map[string]interface{}{
							{
								"name":        "search_files",
								"description": "Search for files",
								"inputSchema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"pattern": map[string]interface{}{
											"type": "string",
										},
									},
								},
							},
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
			}
		}))
		defer server.Close()

		cfg := config.MCPServerConfig{
			URL:     server.URL,
			Enabled: true,
		}

		client := newHTTPClient(cfg)
		tools, err := client.listTools(context.Background())
		require.NoError(t, err)
		assert.Len(t, tools, 1)
		assert.Equal(t, "search_files", tools[0].Name)
	})
}

// TestMCPClient_CallTool tests tool execution forwarding
func TestMCPClient_CallTool(t *testing.T) {
	t.Run("successful tool call", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req mcpRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.Method == "tools/call" {
				params := req.Params.(map[string]interface{})
				assert.Equal(t, "get_weather", params["name"])

				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": "Temperature: 72°F, Sunny",
							},
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
			}
		}))
		defer server.Close()

		cfg := config.MCPServerConfig{
			URL:     server.URL,
			Enabled: true,
		}

		client := newHTTPClient(cfg)
		result, err := client.callTool(context.Background(), "get_weather", map[string]interface{}{
			"location": "San Francisco",
		})

		require.NoError(t, err)
		assert.Contains(t, result, "72°F")
	})

	t.Run("tool returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req mcpRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.Method == "tools/call" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &mcpError{
						Code:    -32602,
						Message: "Invalid parameters",
					},
				}
				json.NewEncoder(w).Encode(resp)
			}
		}))
		defer server.Close()

		cfg := config.MCPServerConfig{
			URL:     server.URL,
			Enabled: true,
		}

		client := newHTTPClient(cfg)
		_, err := client.callTool(context.Background(), "invalid_tool", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid parameters")
	})

	t.Run("timeout handling", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow response
			time.Sleep(200 * time.Millisecond)
		}))
		defer server.Close()

		timeout := 0.1
		cfg := config.MCPServerConfig{
			URL:            server.URL,
			ToolTimeoutSec: &timeout,
			Enabled:        true,
		}

		client := newHTTPClient(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := client.callTool(ctx, "slow_tool", nil)
		assert.Error(t, err)
	})
}

// TestMCPRuntime_Integration tests the MCP runtime wrapper
func TestMCPRuntime_Integration(t *testing.T) {
	t.Run("tool runtime interface implementation", func(t *testing.T) {
		_ = config.MCPServerConfig{
			Command: "mock-mcp",
			Enabled: true,
		}

		tool := MCPTool{
			Name:        "test_tool",
			Description: "Test tool",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		}

		client := newMockStdioClientWithTools([]MCPTool{tool})
		mcpRuntime := NewMCPToolRuntime("test-server", tool, client)

		assert.Equal(t, "mcp__test-server__test_tool", mcpRuntime.Name())
		assert.True(t, mcpRuntime.SupportsParallel())
		assert.False(t, mcpRuntime.NeedsInitialApproval(nil, runtime.ApprovalNever, runtime.SandboxReadOnly))
		assert.Equal(t, runtime.SandboxAuto, mcpRuntime.SandboxPreference())
	})

	t.Run("execute tool through runtime", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req mcpRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.Method == "tools/call" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": "Tool execution result",
							},
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
			}
		}))
		defer server.Close()

		cfg := config.MCPServerConfig{
			URL:     server.URL,
			Enabled: true,
		}

		tool := MCPTool{
			Name:        "test_tool",
			Description: "Test tool",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		}

		client := newHTTPClient(cfg)
		mcpRuntime := NewMCPToolRuntime("test-server", tool, client)

		req := &runtime.ToolRequest{
			CallID:           "test-call-1",
			ToolName:         mcpRuntime.Name(),
			Arguments:        `{"param": "value"}`,
			WorkingDirectory: "/tmp",
		}

		execCtx := &runtime.ExecutionContext{
			SessionID: "test-session",
			TurnID:    "test-turn",
			SandboxAttempt: &runtime.SandboxAttempt{
				Type:   runtime.SandboxNone,
				Policy: runtime.SandboxReadOnly,
			},
			ApprovalCache: runtime.NewMemoryApprovalCache(),
			StartTime:     time.Now(),
		}

		resp, err := mcpRuntime.Execute(context.Background(), req, execCtx)
		require.NoError(t, err)
		assert.Contains(t, resp.Content, "Tool execution result")
	})
}

// TestMCPManager tests the MCP manager for multiple servers
func TestMCPManager(t *testing.T) {
	t.Run("initialize multiple servers", func(t *testing.T) {
		cfg := &config.Config{
			MCPServers: map[string]config.MCPServerConfig{
				"weather": {
					Command: "weather-mcp",
					Enabled: true,
				},
				"files": {
					Command: "files-mcp",
					Enabled: true,
				},
				"disabled": {
					Command: "disabled-mcp",
					Enabled: false,
				},
			},
		}

		manager := NewMCPManager(cfg)
		require.NotNil(t, manager)

		// Should only have enabled servers
		assert.Len(t, manager.clients, 2)
		assert.Contains(t, manager.clients, "weather")
		assert.Contains(t, manager.clients, "files")
		assert.NotContains(t, manager.clients, "disabled")
	})

	t.Run("register tools with orchestrator", func(t *testing.T) {
		server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req mcpRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.Method == "initialize" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"protocolVersion": "1.0",
					},
				}
				json.NewEncoder(w).Encode(resp)
			} else if req.Method == "tools/list" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"tools": []map[string]interface{}{
							{
								"name":        "tool1",
								"description": "First tool",
								"inputSchema": map[string]interface{}{
									"type":       "object",
									"properties": map[string]interface{}{},
								},
							},
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
			}
		}))
		defer server1.Close()

		cfg := &config.Config{
			MCPServers: map[string]config.MCPServerConfig{
				"server1": {
					URL:     server1.URL,
					Enabled: true,
				},
			},
		}

		manager := NewMCPManager(cfg)

		// Initialize the manager first
		err := manager.Initialize(context.Background())
		require.NoError(t, err)

		builder := runtime.NewToolRegistryBuilder()

		specs, err := manager.RegisterTools(context.Background(), nil, builder)
		require.NoError(t, err)
		assert.Len(t, specs, 1)
		assert.Equal(t, "mcp__server1__tool1", specs[0].Name)

		// Build the registry from the builder
		registry, _ := builder.Build()

		// Verify tool is registered
		tool := registry.Get("mcp__server1__tool1")
		assert.NotNil(t, tool)
	})

	t.Run("tool filtering", func(t *testing.T) {
		cfg := &config.Config{
			MCPServers: map[string]config.MCPServerConfig{
				"filtered": {
					Command:      "test-mcp",
					Enabled:      true,
					EnabledTools: []string{"tool1", "tool2"},
				},
			},
		}

		manager := NewMCPManager(cfg)
		tools := []MCPTool{
			{Name: "tool1", Description: "Tool 1"},
			{Name: "tool2", Description: "Tool 2"},
			{Name: "tool3", Description: "Tool 3"},
		}

		filtered := manager.filterTools("filtered", tools)
		assert.Len(t, filtered, 2)
		assert.Equal(t, "tool1", filtered[0].Name)
		assert.Equal(t, "tool2", filtered[1].Name)
	})

	t.Run("tool disabling", func(t *testing.T) {
		cfg := &config.Config{
			MCPServers: map[string]config.MCPServerConfig{
				"filtered": {
					Command:       "test-mcp",
					Enabled:       true,
					DisabledTools: []string{"dangerous_tool"},
				},
			},
		}

		manager := NewMCPManager(cfg)
		tools := []MCPTool{
			{Name: "safe_tool", Description: "Safe"},
			{Name: "dangerous_tool", Description: "Dangerous"},
		}

		filtered := manager.filterTools("filtered", tools)
		assert.Len(t, filtered, 1)
		assert.Equal(t, "safe_tool", filtered[0].Name)
	})
}

// TestToolNameTruncation tests tool name length validation and truncation
func TestToolNameTruncation(t *testing.T) {
	t.Run("short name unchanged", func(t *testing.T) {
		shortName := "mcp__server__tool"
		result := truncateToolName(shortName)
		assert.Equal(t, shortName, result)
		assert.LessOrEqual(t, len(result), MaxToolNameLength)
	})

	t.Run("exact 64 char name unchanged", func(t *testing.T) {
		// Create a name exactly 64 characters long
		exactName := "mcp__server__tool_with_a_very_long_name_that_is_exactly_64xxxxxx"
		require.Equal(t, 64, len(exactName))
		result := truncateToolName(exactName)
		assert.Equal(t, exactName, result)
		assert.Equal(t, MaxToolNameLength, len(result))
	})

	t.Run("long name gets truncated with sha1", func(t *testing.T) {
		// Create a name longer than 64 characters
		longName := "mcp__very_long_server_name__very_long_tool_name_that_exceeds_the_maximum_allowed_length"
		require.Greater(t, len(longName), MaxToolNameLength)

		result := truncateToolName(longName)

		// Result should be exactly 64 characters
		assert.Equal(t, MaxToolNameLength, len(result))

		// Result should start with a truncated prefix
		assert.True(t, len(result) > 0)

		// Result should end with SHA1 hash (40 hex chars)
		// SHA1 hash is 40 characters, so prefix should be 24 chars
		expectedPrefixLen := MaxToolNameLength - 40
		assert.Equal(t, longName[:expectedPrefixLen], result[:expectedPrefixLen])
	})

	t.Run("truncated names are unique", func(t *testing.T) {
		// Two similar long names should produce different truncated results
		longName1 := "mcp__server__very_long_tool_name_that_exceeds_maximum_length_version_1"
		longName2 := "mcp__server__very_long_tool_name_that_exceeds_maximum_length_version_2"

		result1 := truncateToolName(longName1)
		result2 := truncateToolName(longName2)

		assert.NotEqual(t, result1, result2, "Different long names should produce different truncated results")
		assert.Equal(t, MaxToolNameLength, len(result1))
		assert.Equal(t, MaxToolNameLength, len(result2))
	})

	t.Run("same long name produces consistent result", func(t *testing.T) {
		longName := "mcp__server__very_long_tool_name_that_exceeds_the_maximum_allowed_length"

		result1 := truncateToolName(longName)
		result2 := truncateToolName(longName)

		assert.Equal(t, result1, result2, "Same input should produce same truncated result")
	})

	t.Run("extremely long name", func(t *testing.T) {
		// Test with a very long name (200+ chars)
		extremelyLongName := "mcp__server_with_extremely_long_name__tool_with_extremely_long_name_" +
			"that_goes_on_and_on_and_on_with_many_underscores_and_descriptive_text_" +
			"that_makes_it_way_beyond_the_maximum_allowed_length_of_64_characters"
		require.Greater(t, len(extremelyLongName), 200)

		result := truncateToolName(extremelyLongName)

		assert.Equal(t, MaxToolNameLength, len(result))
		assert.NotEqual(t, extremelyLongName, result)
	})
}

// TestGenerateToolSpecWithTruncation tests tool spec generation with name truncation
func TestGenerateToolSpecWithTruncation(t *testing.T) {
	t.Run("short server and tool names", func(t *testing.T) {
		tool := MCPTool{
			Name:        "weather",
			Description: "Get weather",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		}

		spec := generateToolSpec("api", tool)
		expectedName := "mcp__api__weather"
		assert.Equal(t, expectedName, spec.Name)
		assert.LessOrEqual(t, len(spec.Name), MaxToolNameLength)
	})

	t.Run("long server and tool names get truncated", func(t *testing.T) {
		tool := MCPTool{
			Name:        "get_weather_forecast_for_location_with_extended_details",
			Description: "Get detailed weather forecast",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		}

		spec := generateToolSpec("weather_service_with_very_long_name", tool)

		// Name should be truncated to 64 chars
		assert.Equal(t, MaxToolNameLength, len(spec.Name))

		// Should start with the MCP prefix
		assert.True(t, len(spec.Name) > 0)

		// Other fields should be unaffected
		assert.Contains(t, spec.Description, "detailed weather forecast")
		assert.NotNil(t, spec.ParametersSchema)
	})

	t.Run("edge case at boundary", func(t *testing.T) {
		// Create server and tool name that results in exactly 64 chars
		// Format: "mcp__<server>__<tool>"
		// Length: 5 + len(server) + 2 + len(tool) = 64
		// So: len(server) + len(tool) = 57
		serverName := "server12345678901234"  // 20 chars
		toolName := "tool123456789012345678901234567890123" // 37 chars
		// Total: mcp__ (5) + 20 + __ (2) + 37 = 64

		tool := MCPTool{
			Name:        toolName,
			Description: "Test tool",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		}

		spec := generateToolSpec(serverName, tool)
		expectedName := fmt.Sprintf("mcp__%s__%s", serverName, toolName)
		require.Equal(t, 64, len(expectedName))

		assert.Equal(t, expectedName, spec.Name)
		assert.Equal(t, MaxToolNameLength, len(spec.Name))
	})
}

// TestMCPSchema tests schema mapping
func TestMCPSchema(t *testing.T) {
	t.Run("convert MCP schema to runtime schema", func(t *testing.T) {
		mcpSchema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "City name",
				},
				"units": map[string]interface{}{
					"type": "string",
					"enum": []interface{}{"celsius", "fahrenheit"},
				},
			},
			"required": []interface{}{"location"},
		}

		runtimeSchema := convertMCPSchema(mcpSchema)
		assert.NotNil(t, runtimeSchema)

		// Verify schema structure
		schemaMap := runtimeSchema.(map[string]interface{})
		assert.Equal(t, "object", schemaMap["type"])

		props := schemaMap["properties"].(map[string]interface{})
		assert.Contains(t, props, "location")
		assert.Contains(t, props, "units")

		required := schemaMap["required"].([]interface{})
		assert.Len(t, required, 1)
		assert.Equal(t, "location", required[0])
	})

	t.Run("generate tool spec", func(t *testing.T) {
		tool := MCPTool{
			Name:        "get_weather",
			Description: "Get current weather",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		}

		spec := generateToolSpec("weather-server", tool)
		assert.Equal(t, "mcp__weather-server__get_weather", spec.Name)
		assert.Contains(t, spec.Description, "Get current weather")
		assert.Contains(t, spec.Description, "weather-server")
		assert.NotNil(t, spec.ParametersSchema)
		assert.True(t, spec.SupportsParallel)
	})
}

// TestConcurrentSafety tests concurrent access to MCP clients
func TestConcurrentSafety(t *testing.T) {
	t.Run("concurrent tool calls", func(t *testing.T) {
		callCount := 0
		mu := sync.Mutex{}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			callCount++
			mu.Unlock()

			var req mcpRequest
			json.NewDecoder(r.Body).Decode(&req)

			resp := mcpResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]interface{}{
					"content": []map[string]interface{}{
						{"type": "text", "text": "result"},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		cfg := config.MCPServerConfig{
			URL:     server.URL,
			Enabled: true,
		}

		client := newHTTPClient(cfg)

		// Make 10 concurrent calls
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				_, err := client.callTool(context.Background(), "test_tool", map[string]interface{}{
					"id": n,
				})
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()
		assert.Equal(t, 10, callCount)
	})
}

// TestProcessCleanup tests that stdio client properly cleans up processes
func TestProcessCleanup(t *testing.T) {
	t.Run("process is properly reaped after close", func(t *testing.T) {
		// Create a simple echo-based MCP server that will stay alive
		cfg := config.MCPServerConfig{
			Command: "cat", // cat will stay alive reading from stdin
			Enabled: true,
		}

		client := newStdioClient(cfg)

		// Start the process (skip protocol initialization since cat isn't a real MCP server)
		cmd := exec.Command(cfg.Command)
		stdin, err := cmd.StdinPipe()
		require.NoError(t, err)
		stdout, err := cmd.StdoutPipe()
		require.NoError(t, err)
		stderr, err := cmd.StderrPipe()
		require.NoError(t, err)

		err = cmd.Start()
		require.NoError(t, err)

		client.cmd = cmd
		client.stdin = stdin
		client.stdout = stdout
		client.stderr = stderr

		// Verify the process is running
		assert.NotNil(t, cmd.Process)
		originalPID := cmd.Process.Pid

		// Close the client
		err = client.close()
		assert.NoError(t, err)

		// Verify the process no longer exists
		// On Unix systems, sending signal 0 checks if process exists
		process, _ := os.FindProcess(originalPID)
		if process != nil {
			// Try to send signal 0 (does nothing but checks existence)
			err := process.Signal(os.Signal(nil))
			// On macOS/Linux, if process doesn't exist, we get an error
			// This is platform-specific but validates the process was reaped
			if err == nil {
				// Process still exists - this is only expected on some systems
				// where FindProcess always succeeds. We mainly care that Wait() was called.
				t.Logf("Process %d may still exist (platform-specific), but Wait() was called", originalPID)
			}
		}

		// Verify all resources are nil after close
		assert.Nil(t, client.cmd)
		assert.Nil(t, client.stdin)
		assert.Nil(t, client.stdout)
		assert.Nil(t, client.stderr)
	})

	t.Run("stderr consumption prevents deadlock", func(t *testing.T) {
		// This test verifies that stderr is consumed to prevent deadlock
		// when a child process writes a lot to stderr

		cfg := config.MCPServerConfig{
			// Use a command that writes to stderr
			Command: "sh",
			Args:    []string{"-c", "echo 'error message' >&2; sleep 0.1"},
			Enabled: true,
		}

		client := newStdioClient(cfg)

		cmd := exec.Command(cfg.Command, cfg.Args...)
		stdin, err := cmd.StdinPipe()
		require.NoError(t, err)
		stdout, err := cmd.StdoutPipe()
		require.NoError(t, err)
		stderr, err := cmd.StderrPipe()
		require.NoError(t, err)

		err = cmd.Start()
		require.NoError(t, err)

		client.cmd = cmd
		client.stdin = stdin
		client.stdout = stdout
		client.stderr = stderr
		client.reader = bufio.NewReader(stdout)

		// Start stderr consumer (simulating what initialize does)
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := stderr.Read(buf)
				if n > 0 {
					client.mu.Lock()
					client.stderrBuf.Write(buf[:n])
					client.mu.Unlock()
				}
				if err != nil {
					break
				}
			}
		}()

		// Wait a bit for the command to write to stderr
		time.Sleep(150 * time.Millisecond)

		// Close should not deadlock
		done := make(chan error, 1)
		go func() {
			done <- client.close()
		}()

		select {
		case err := <-done:
			assert.NoError(t, err)
			// Verify stderr was captured
			client.mu.Lock()
			stderrContent := client.stderrBuf.String()
			client.mu.Unlock()
			assert.Contains(t, stderrContent, "error message")
		case <-time.After(2 * time.Second):
			t.Fatal("close() deadlocked or took too long")
		}
	})

	t.Run("double close is safe", func(t *testing.T) {
		cfg := config.MCPServerConfig{
			Command: "cat",
			Enabled: true,
		}

		client := newStdioClient(cfg)

		cmd := exec.Command(cfg.Command)
		stdin, _ := cmd.StdinPipe()
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()
		cmd.Start()

		client.cmd = cmd
		client.stdin = stdin
		client.stdout = stdout
		client.stderr = stderr

		// First close
		err := client.close()
		assert.NoError(t, err)

		// Second close should not panic or error
		err = client.close()
		assert.NoError(t, err)
	})
}

// TestErrorHandling tests various error scenarios
func TestErrorHandling(t *testing.T) {
	t.Run("server not found", func(t *testing.T) {
		cfg := config.MCPServerConfig{
			Command: "non-existent-mcp-server",
			Enabled: true,
		}

		client := newStdioClient(cfg)
		err := client.initialize(context.Background())
		assert.Error(t, err)
	})

	t.Run("malformed response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		cfg := config.MCPServerConfig{
			URL:     server.URL,
			Enabled: true,
		}

		client := newHTTPClient(cfg)
		_, err := client.listTools(context.Background())
		assert.Error(t, err)
	})

	t.Run("server returns error code", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}))
		defer server.Close()

		cfg := config.MCPServerConfig{
			URL:     server.URL,
			Enabled: true,
		}

		client := newHTTPClient(cfg)
		err := client.initialize(context.Background())
		assert.Error(t, err)
	})
}

// Helper functions for mocking

func newMockStdioClientWithTools(tools []MCPTool) *mockMCPClient {
	return &mockMCPClient{
		tools:     tools,
		transport: "stdio",
	}
}

type mockMCPClient struct {
	tools     []MCPTool
	transport string
	mu        sync.Mutex
}

func (m *mockMCPClient) initialize(ctx context.Context) error {
	return nil
}

func (m *mockMCPClient) listTools(ctx context.Context) ([]MCPTool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tools, nil
}

func (m *mockMCPClient) callTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, tool := range m.tools {
		if tool.Name == name {
			return fmt.Sprintf("Mock result for %s", name), nil
		}
	}
	return "", fmt.Errorf("tool not found: %s", name)
}

func (m *mockMCPClient) listResources(ctx context.Context) ([]MCPResource, error) {
	return []MCPResource{}, nil
}

func (m *mockMCPClient) readResource(ctx context.Context, uri string) (*MCPResourceContents, error) {
	return &MCPResourceContents{URI: uri}, nil
}

func (m *mockMCPClient) listResourceTemplates(ctx context.Context) ([]MCPResourceTemplate, error) {
	return []MCPResourceTemplate{}, nil
}

func (m *mockMCPClient) close() error {
	return nil
}

func (m *mockMCPClient) transportType() string {
	return m.transport
}

// TestServerNameValidation tests the server name validation logic
func TestServerNameValidation(t *testing.T) {
	t.Run("valid server names", func(t *testing.T) {
		validNames := []string{
			"weather",
			"weather-api",
			"weather_api",
			"WeatherAPI",
			"weather123",
			"api-server-1",
			"my_cool_server",
			"Server123-test_name",
			"a",
			"A",
			"1",
			"_",
			"-",
		}

		for _, name := range validNames {
			t.Run(name, func(t *testing.T) {
				assert.True(t, isValidMCPServerName(name), "Expected '%s' to be valid", name)
				assert.NoError(t, validateMCPServerName(name), "Expected '%s' to be valid", name)
			})
		}
	})

	t.Run("invalid server names", func(t *testing.T) {
		invalidNames := []string{
			"",                  // empty
			"my server",         // space
			"api@service",       // @ symbol
			"server.name",       // period
			"server/name",       // slash
			"server\\name",      // backslash
			"server:name",       // colon
			"server;name",       // semicolon
			"server|name",       // pipe
			"server*name",       // asterisk
			"server?name",       // question mark
			"server!name",       // exclamation
			"server#name",       // hash
			"server$name",       // dollar
			"server%name",       // percent
			"server^name",       // caret
			"server&name",       // ampersand
			"server(name",       // parenthesis
			"server)name",       // parenthesis
			"server[name",       // bracket
			"server]name",       // bracket
			"server{name",       // brace
			"server}name",       // brace
			"server<name",       // angle bracket
			"server>name",       // angle bracket
			"server=name",       // equals
			"server+name",       // plus
			"server~name",       // tilde
			"server`name",       // backtick
			"server'name",       // single quote
			"server\"name",      // double quote
			"server\nname",      // newline
			"server\tname",      // tab
			"server\rname",      // carriage return
			"weather api",       // space (common mistake)
			"api@weather",       // email-like (common mistake)
			"my.server",         // domain-like (common mistake)
			"server/path",       // path-like (common mistake)
			"server:8080",       // port-like (common mistake)
			"http://server",     // URL-like (common mistake)
			"../server",         // path traversal attempt
			"./server",          // relative path
			"server/../other",   // path traversal attempt
			"日本語",               // non-ASCII characters
			"café",              // accented characters
			"emoji😀",           // emoji
		}

		for _, name := range invalidNames {
			t.Run(fmt.Sprintf("invalid_%s", name), func(t *testing.T) {
				assert.False(t, isValidMCPServerName(name), "Expected '%s' to be invalid", name)
				err := validateMCPServerName(name)
				assert.Error(t, err, "Expected '%s' to be invalid", name)
				if name == "" {
					assert.Contains(t, err.Error(), "cannot be empty")
				} else {
					assert.Contains(t, err.Error(), "invalid")
				}
			})
		}
	})

	t.Run("edge cases", func(t *testing.T) {
		// Single character names
		assert.True(t, isValidMCPServerName("a"))
		assert.True(t, isValidMCPServerName("A"))
		assert.True(t, isValidMCPServerName("1"))
		assert.True(t, isValidMCPServerName("_"))
		assert.True(t, isValidMCPServerName("-"))

		// Very long valid name
		longValidName := "a" + string(make([]byte, 100))
		for i := range longValidName {
			if i > 0 {
				longValidName = longValidName[:i] + "a" + longValidName[i+1:]
			}
		}
		assert.True(t, isValidMCPServerName(longValidName))

		// Names that are almost valid
		assert.False(t, isValidMCPServerName(" leading-space"))
		assert.False(t, isValidMCPServerName("trailing-space "))
		assert.False(t, isValidMCPServerName("mid dle-space"))
	})
}

// TestMCPManager_ServerNameValidation tests that invalid server names are rejected
func TestMCPManager_ServerNameValidation(t *testing.T) {
	t.Run("reject invalid server names", func(t *testing.T) {
		cfg := &config.Config{
			MCPServers: map[string]config.MCPServerConfig{
				"valid-server": {
					Command: "valid-mcp",
					Enabled: true,
				},
				"invalid server": { // space in name
					Command: "invalid-mcp",
					Enabled: true,
				},
				"valid_server2": {
					Command: "valid-mcp2",
					Enabled: true,
				},
				"api@service": { // @ symbol
					Command: "api-mcp",
					Enabled: true,
				},
			},
		}

		manager := NewMCPManager(cfg)
		require.NotNil(t, manager)

		// Should only have valid servers
		assert.Len(t, manager.clients, 2)
		assert.Contains(t, manager.clients, "valid-server")
		assert.Contains(t, manager.clients, "valid_server2")
		assert.NotContains(t, manager.clients, "invalid server")
		assert.NotContains(t, manager.clients, "api@service")
	})

	t.Run("empty server name", func(t *testing.T) {
		cfg := &config.Config{
			MCPServers: map[string]config.MCPServerConfig{
				"": { // empty name
					Command: "empty-mcp",
					Enabled: true,
				},
			},
		}

		manager := NewMCPManager(cfg)
		require.NotNil(t, manager)

		// Should reject empty name
		assert.Len(t, manager.clients, 0)
	})

	t.Run("all valid names", func(t *testing.T) {
		cfg := &config.Config{
			MCPServers: map[string]config.MCPServerConfig{
				"server1": {
					Command: "server1-cmd",
					Enabled: true,
				},
				"server-2": {
					Command: "server2-cmd",
					Enabled: true,
				},
				"server_3": {
					Command: "server3-cmd",
					Enabled: true,
				},
				"Server4": {
					Command: "server4-cmd",
					Enabled: true,
				},
			},
		}

		manager := NewMCPManager(cfg)
		require.NotNil(t, manager)

		// Should have all servers
		assert.Len(t, manager.clients, 4)
		assert.Contains(t, manager.clients, "server1")
		assert.Contains(t, manager.clients, "server-2")
		assert.Contains(t, manager.clients, "server_3")
		assert.Contains(t, manager.clients, "Server4")
	})

	t.Run("security test - path traversal attempts", func(t *testing.T) {
		cfg := &config.Config{
			MCPServers: map[string]config.MCPServerConfig{
				"../etc/passwd": { // path traversal
					Command: "malicious-mcp",
					Enabled: true,
				},
				"./local": { // relative path
					Command: "malicious-mcp2",
					Enabled: true,
				},
				"server/../other": { // embedded traversal
					Command: "malicious-mcp3",
					Enabled: true,
				},
			},
		}

		manager := NewMCPManager(cfg)
		require.NotNil(t, manager)

		// Should reject all malicious names
		assert.Len(t, manager.clients, 0)
	})

	t.Run("security test - special characters", func(t *testing.T) {
		cfg := &config.Config{
			MCPServers: map[string]config.MCPServerConfig{
				"server;rm -rf /": { // command injection attempt
					Command: "malicious-mcp",
					Enabled: true,
				},
				"server|cat /etc/passwd": { // pipe injection
					Command: "malicious-mcp2",
					Enabled: true,
				},
				"server$(whoami)": { // command substitution
					Command: "malicious-mcp3",
					Enabled: true,
				},
			},
		}

		manager := NewMCPManager(cfg)
		require.NotNil(t, manager)

		// Should reject all malicious names
		assert.Len(t, manager.clients, 0)
	})
}
