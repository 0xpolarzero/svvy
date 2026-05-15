package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/evmts/codex/codex-go/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPClient_ListResources tests the listResources method
func TestMCPClient_ListResources(t *testing.T) {
	t.Run("successful list via HTTP", func(t *testing.T) {
		// Create mock HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req mcpRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)

			if req.Method == "initialize" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"protocolVersion": "1.0",
						"serverInfo": map[string]interface{}{
							"name":    "test-server",
							"version": "1.0.0",
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
			} else if req.Method == "resources/list" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"resources": []interface{}{
							map[string]interface{}{
								"uri":         "file:///test/document.txt",
								"name":        "Test Document",
								"description": "A test document",
								"mimeType":    "text/plain",
							},
							map[string]interface{}{
								"uri":         "memo://example/note",
								"name":        "Example Note",
								"description": "An example memo",
								"mimeType":    "text/markdown",
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
		err := client.initialize(context.Background())
		require.NoError(t, err)

		resources, err := client.listResources(context.Background())
		require.NoError(t, err)
		assert.Len(t, resources, 2)

		// Verify first resource
		assert.Equal(t, "file:///test/document.txt", resources[0].URI)
		assert.Equal(t, "Test Document", resources[0].Name)
		assert.Equal(t, "A test document", resources[0].Description)
		assert.Equal(t, "text/plain", resources[0].MimeType)

		// Verify second resource
		assert.Equal(t, "memo://example/note", resources[1].URI)
		assert.Equal(t, "Example Note", resources[1].Name)
		assert.Equal(t, "An example memo", resources[1].Description)
		assert.Equal(t, "text/markdown", resources[1].MimeType)
	})

	t.Run("empty resource list", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req mcpRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)

			if req.Method == "initialize" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"protocolVersion": "1.0",
						"serverInfo": map[string]interface{}{
							"name":    "test-server",
							"version": "1.0.0",
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
			} else if req.Method == "resources/list" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"resources": []interface{}{},
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
		err := client.initialize(context.Background())
		require.NoError(t, err)

		resources, err := client.listResources(context.Background())
		require.NoError(t, err)
		assert.Len(t, resources, 0)
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req mcpRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)

			if req.Method == "initialize" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"protocolVersion": "1.0",
						"serverInfo": map[string]interface{}{
							"name":    "test-server",
							"version": "1.0.0",
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
			} else if req.Method == "resources/list" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &mcpError{
						Code:    -32603,
						Message: "Internal error listing resources",
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
		err := client.initialize(context.Background())
		require.NoError(t, err)

		resources, err := client.listResources(context.Background())
		assert.Error(t, err)
		assert.Nil(t, resources)
		assert.Contains(t, err.Error(), "Internal error listing resources")
	})
}

// TestMCPClient_ReadResource tests the readResource method
func TestMCPClient_ReadResource(t *testing.T) {
	t.Run("successful read text resource", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req mcpRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)

			if req.Method == "initialize" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"protocolVersion": "1.0",
						"serverInfo": map[string]interface{}{
							"name":    "test-server",
							"version": "1.0.0",
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
			} else if req.Method == "resources/read" {
				params := req.Params.(map[string]interface{})
				uri := params["uri"].(string)

				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"contents": []interface{}{
							map[string]interface{}{
								"uri":      uri,
								"mimeType": "text/plain",
								"text":     "This is the content of the test resource.",
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
		err := client.initialize(context.Background())
		require.NoError(t, err)

		contents, err := client.readResource(context.Background(), "file:///test/document.txt")
		require.NoError(t, err)
		require.NotNil(t, contents)

		assert.Equal(t, "file:///test/document.txt", contents.URI)
		assert.Equal(t, "text/plain", contents.MimeType)
		assert.Equal(t, "This is the content of the test resource.", contents.Text)
		assert.Empty(t, contents.Blob)
	})

	t.Run("successful read blob resource", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req mcpRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)

			if req.Method == "initialize" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"protocolVersion": "1.0",
						"serverInfo": map[string]interface{}{
							"name":    "test-server",
							"version": "1.0.0",
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
			} else if req.Method == "resources/read" {
				params := req.Params.(map[string]interface{})
				uri := params["uri"].(string)

				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"contents": []interface{}{
							map[string]interface{}{
								"uri":      uri,
								"mimeType": "image/png",
								"blob":     "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
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
		err := client.initialize(context.Background())
		require.NoError(t, err)

		contents, err := client.readResource(context.Background(), "file:///test/image.png")
		require.NoError(t, err)
		require.NotNil(t, contents)

		assert.Equal(t, "file:///test/image.png", contents.URI)
		assert.Equal(t, "image/png", contents.MimeType)
		assert.Empty(t, contents.Text)
		assert.NotEmpty(t, contents.Blob)
	})

	t.Run("resource not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req mcpRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)

			if req.Method == "initialize" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"protocolVersion": "1.0",
						"serverInfo": map[string]interface{}{
							"name":    "test-server",
							"version": "1.0.0",
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
			} else if req.Method == "resources/read" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &mcpError{
						Code:    -32002,
						Message: "Resource not found",
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
		err := client.initialize(context.Background())
		require.NoError(t, err)

		contents, err := client.readResource(context.Background(), "file:///nonexistent.txt")
		assert.Error(t, err)
		assert.Nil(t, contents)
		assert.Contains(t, err.Error(), "Resource not found")
	})
}

// TestMCPClient_ListResourceTemplates tests the listResourceTemplates method
func TestMCPClient_ListResourceTemplates(t *testing.T) {
	t.Run("successful list templates", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req mcpRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)

			if req.Method == "initialize" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"protocolVersion": "1.0",
						"serverInfo": map[string]interface{}{
							"name":    "test-server",
							"version": "1.0.0",
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
			} else if req.Method == "resources/templates/list" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"resourceTemplates": []interface{}{
							map[string]interface{}{
								"uriTemplate": "file:///{path}",
								"name":        "File Template",
								"description": "Access any file by path",
								"mimeType":    "text/plain",
							},
							map[string]interface{}{
								"uriTemplate": "memo://{user}/{id}",
								"name":        "Memo Template",
								"description": "Access user memos by ID",
								"mimeType":    "text/markdown",
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
		err := client.initialize(context.Background())
		require.NoError(t, err)

		templates, err := client.listResourceTemplates(context.Background())
		require.NoError(t, err)
		assert.Len(t, templates, 2)

		// Verify first template
		assert.Equal(t, "file:///{path}", templates[0].URITemplate)
		assert.Equal(t, "File Template", templates[0].Name)
		assert.Equal(t, "Access any file by path", templates[0].Description)
		assert.Equal(t, "text/plain", templates[0].MimeType)

		// Verify second template
		assert.Equal(t, "memo://{user}/{id}", templates[1].URITemplate)
		assert.Equal(t, "Memo Template", templates[1].Name)
		assert.Equal(t, "Access user memos by ID", templates[1].Description)
		assert.Equal(t, "text/markdown", templates[1].MimeType)
	})

	t.Run("empty template list", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req mcpRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)

			if req.Method == "initialize" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"protocolVersion": "1.0",
						"serverInfo": map[string]interface{}{
							"name":    "test-server",
							"version": "1.0.0",
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
			} else if req.Method == "resources/templates/list" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"resourceTemplates": []interface{}{},
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
		err := client.initialize(context.Background())
		require.NoError(t, err)

		templates, err := client.listResourceTemplates(context.Background())
		require.NoError(t, err)
		assert.Len(t, templates, 0)
	})

	t.Run("templates with annotations", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req mcpRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)

			if req.Method == "initialize" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"protocolVersion": "1.0",
						"serverInfo": map[string]interface{}{
							"name":    "test-server",
							"version": "1.0.0",
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
			} else if req.Method == "resources/templates/list" {
				resp := mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"resourceTemplates": []interface{}{
							map[string]interface{}{
								"uriTemplate": "api:///{endpoint}",
								"name":        "API Template",
								"description": "Access API endpoints",
								"mimeType":    "application/json",
								"annotations": map[string]interface{}{
									"priority": "high",
									"cached":   true,
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
		err := client.initialize(context.Background())
		require.NoError(t, err)

		templates, err := client.listResourceTemplates(context.Background())
		require.NoError(t, err)
		assert.Len(t, templates, 1)

		assert.Equal(t, "api:///{endpoint}", templates[0].URITemplate)
		assert.Equal(t, "API Template", templates[0].Name)
		assert.NotNil(t, templates[0].Annotations)
		assert.Equal(t, "high", templates[0].Annotations["priority"])
		assert.Equal(t, true, templates[0].Annotations["cached"])
	})
}

// TestMCPResourceTypes tests the resource type definitions
func TestMCPResourceTypes(t *testing.T) {
	t.Run("MCPResource JSON marshaling", func(t *testing.T) {
		resource := MCPResource{
			URI:         "file:///test.txt",
			Name:        "Test File",
			Description: "A test file",
			MimeType:    "text/plain",
			Annotations: map[string]interface{}{
				"readonly": true,
			},
		}

		data, err := json.Marshal(resource)
		require.NoError(t, err)

		var unmarshaled MCPResource
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resource.URI, unmarshaled.URI)
		assert.Equal(t, resource.Name, unmarshaled.Name)
		assert.Equal(t, resource.Description, unmarshaled.Description)
		assert.Equal(t, resource.MimeType, unmarshaled.MimeType)
		assert.Equal(t, true, unmarshaled.Annotations["readonly"])
	})

	t.Run("MCPResourceTemplate JSON marshaling", func(t *testing.T) {
		template := MCPResourceTemplate{
			URITemplate: "file:///{path}",
			Name:        "File Template",
			Description: "Template for files",
			MimeType:    "text/plain",
		}

		data, err := json.Marshal(template)
		require.NoError(t, err)

		var unmarshaled MCPResourceTemplate
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, template.URITemplate, unmarshaled.URITemplate)
		assert.Equal(t, template.Name, unmarshaled.Name)
		assert.Equal(t, template.Description, unmarshaled.Description)
		assert.Equal(t, template.MimeType, unmarshaled.MimeType)
	})

	t.Run("MCPResourceContents JSON marshaling", func(t *testing.T) {
		contents := MCPResourceContents{
			URI:      "file:///test.txt",
			MimeType: "text/plain",
			Text:     "Hello, world!",
		}

		data, err := json.Marshal(contents)
		require.NoError(t, err)

		var unmarshaled MCPResourceContents
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, contents.URI, unmarshaled.URI)
		assert.Equal(t, contents.MimeType, unmarshaled.MimeType)
		assert.Equal(t, contents.Text, unmarshaled.Text)
		assert.Empty(t, unmarshaled.Blob)
	})
}
