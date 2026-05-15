package schema_test

import (
	"encoding/json"
	"testing"

	"github.com/evmts/codex/codex-go/internal/tools"
	"github.com/evmts/codex/codex-go/internal/tools/schema"
)

// TestToolRegistryAndSchemaGeneration validates that:
// 1. The default registry is created with all expected tools
// 2. Schema generation produces valid OpenAI-compatible tool definitions
// 3. All tool schemas have required fields populated
func TestToolRegistryAndSchemaGeneration(t *testing.T) {
	// Create default registry
	registry := tools.NewDefaultRegistry()
	if registry == nil {
		t.Fatal("Failed to create default registry")
	}

	// Check that registry has expected tools
	expectedTools := []string{"shell", "read_file", "write_file", "list_dir", "grep_files"}
	registeredTools := registry.List()

	t.Logf("Registered tools: %v", registeredTools)

	if len(registeredTools) != len(expectedTools) {
		t.Errorf("Expected %d tools in registry, got %d", len(expectedTools), len(registeredTools))
	}

	for _, expectedTool := range expectedTools {
		found := false
		for _, registeredTool := range registeredTools {
			if registeredTool == expectedTool {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool %q not found in registry", expectedTool)
		}
	}

	// Generate schemas
	schemas := schema.GenerateToolSchemas(registry)
	if len(schemas) == 0 {
		t.Fatal("Schema generation produced no schemas")
	}

	t.Logf("Generated %d schemas", len(schemas))

	// Validate each schema
	for i, toolSchema := range schemas {
		t.Run("Schema_"+toolSchema.Function.Name, func(t *testing.T) {
			// Check type
			if toolSchema.Type != "function" {
				t.Errorf("Schema %d: expected type 'function', got %q", i, toolSchema.Type)
			}

			// Check function definition exists
			if toolSchema.Function == nil {
				t.Fatalf("Schema %d: function definition is nil", i)
			}

			// Check name
			if toolSchema.Function.Name == "" {
				t.Errorf("Schema %d: function name is empty", i)
			}

			// Check description
			if toolSchema.Function.Description == "" {
				t.Errorf("Schema %d (%s): description is empty", i, toolSchema.Function.Name)
			}

			// Check parameters
			if len(toolSchema.Function.Parameters) == 0 {
				t.Errorf("Schema %d (%s): parameters are empty", i, toolSchema.Function.Name)
			}

			// Validate parameters are valid JSON Schema
			var params map[string]interface{}
			if err := json.Unmarshal(toolSchema.Function.Parameters, &params); err != nil {
				t.Errorf("Schema %d (%s): parameters are not valid JSON: %v", i, toolSchema.Function.Name, err)
			}

			// Log the schema for debugging
			t.Logf("Schema %d: %s", i, toolSchema.Function.Name)
			t.Logf("  Description: %s", toolSchema.Function.Description)
			t.Logf("  Parameters: %s", string(toolSchema.Function.Parameters))
		})
	}
}

// TestSpecificToolSchemas validates specific tool schemas
func TestSpecificToolSchemas(t *testing.T) {
	registry := tools.NewDefaultRegistry()
	schemas := schema.GenerateToolSchemas(registry)

	// Create a map for easier lookup
	schemaMap := make(map[string]struct {
		description string
		params      map[string]interface{}
	})

	for _, s := range schemas {
		if s.Function != nil {
			var params map[string]interface{}
			_ = json.Unmarshal(s.Function.Parameters, &params)
			schemaMap[s.Function.Name] = struct {
				description string
				params      map[string]interface{}
			}{
				description: s.Function.Description,
				params:      params,
			}
		}
	}

	// Test shell schema
	t.Run("shell", func(t *testing.T) {
		schema, ok := schemaMap["shell"]
		if !ok {
			t.Fatal("shell schema not found")
		}

		if schema.description == "" {
			t.Error("shell schema missing description")
		}

		// Check required parameter
		props, ok := schema.params["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("shell schema missing properties")
		}

		if _, ok := props["command"]; !ok {
			t.Error("shell schema missing 'command' parameter")
		}

		required, ok := schema.params["required"].([]interface{})
		if !ok || len(required) == 0 {
			t.Error("shell schema missing required fields")
		}
	})

	// Test list_dir schema
	t.Run("list_dir", func(t *testing.T) {
		schema, ok := schemaMap["list_dir"]
		if !ok {
			t.Fatal("list_dir schema not found")
		}

		if schema.description == "" {
			t.Error("list_dir schema missing description")
		}

		props, ok := schema.params["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("list_dir schema missing properties")
		}

		// Check optional parameters exist
		if _, ok := props["path"]; !ok {
			t.Error("list_dir schema missing 'path' parameter")
		}
		if _, ok := props["recursive"]; !ok {
			t.Error("list_dir schema missing 'recursive' parameter")
		}
	})

	// Test write_file schema
	t.Run("write_file", func(t *testing.T) {
		schema, ok := schemaMap["write_file"]
		if !ok {
			t.Fatal("write_file schema not found")
		}

		props, ok := schema.params["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("write_file schema missing properties")
		}

		if _, ok := props["path"]; !ok {
			t.Error("write_file schema missing 'path' parameter")
		}
		if _, ok := props["content"]; !ok {
			t.Error("write_file schema missing 'content' parameter")
		}

		// Check required fields
		required, ok := schema.params["required"].([]interface{})
		if !ok || len(required) != 2 {
			t.Error("write_file schema should have 2 required fields (path, content)")
		}
	})
}
