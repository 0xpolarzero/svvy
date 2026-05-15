// Package schema provides utilities for converting tool runtime definitions
// to API client tool schemas (JSON Schema format).
//
// This package bridges the gap between the internal tool runtime system
// and the external API that LLMs consume. It generates JSON Schema definitions
// for each tool that describe their parameters and usage.
package schema

import (
	"encoding/json"

	"github.com/evmts/codex/codex-go/internal/client"
	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// GenerateToolSchemas converts all tools in a registry to API tool definitions.
// This generates the Tool[] array that should be sent to the LLM in ChatCompletionRequest.
func GenerateToolSchemas(registry *runtime.ToolRegistry) []client.Tool {
	if registry == nil {
		return nil
	}

	toolNames := registry.List()
	schemas := make([]client.Tool, 0, len(toolNames))

	for _, name := range toolNames {
		tool := registry.Get(name)
		if tool == nil {
			continue
		}

		// Generate schema based on tool name
		// Each tool has a specific parameter schema
		var schema *client.Tool
		switch name {
		case "shell":
			schema = generateShellSchema()
		case "read_file":
			schema = generateReadFileSchema()
		case "write_file":
			schema = generateWriteFileSchema()
		case "list_dir":
			schema = generateListDirectorySchema()
		case "grep_files":
			schema = generateGrepSchema()
		default:
			// Skip unknown tools
			continue
		}

		if schema != nil {
			schemas = append(schemas, *schema)
		}
	}

	return schemas
}

// generateShellSchema creates the schema for the shell command execution tool.
func generateShellSchema() *client.Tool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The shell command to execute. Supports pipes, redirects, and all shell features via 'sh -c'.",
			},
			"working_directory": map[string]interface{}{
				"type":        "string",
				"description": "Optional working directory for command execution. Defaults to the current working directory.",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Optional timeout in milliseconds for command execution.",
			},
			"with_escalated_permissions": map[string]interface{}{
				"type":        "boolean",
				"description": "Request execution without sandbox restrictions (use sparingly).",
			},
			"justification": map[string]interface{}{
				"type":        "string",
				"description": "Explanation for why escalated permissions are needed (required if with_escalated_permissions is true).",
			},
			"environment": map[string]interface{}{
				"type":        "object",
				"description": "Optional environment variables to set for the command.",
				"additionalProperties": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []string{"command"},
	}

	paramsJSON, _ := json.Marshal(params)

	return &client.Tool{
		Type: "function",
		Function: &client.FunctionDefinition{
			Name:        "shell",
			Description: "Execute shell commands in the system. Supports all shell features including pipes, redirects, and variable expansion. Use this to run terminal commands, scripts, build tools, git operations, file management, etc.",
			Parameters:  json.RawMessage(paramsJSON),
		},
	}
}

// generateReadFileSchema creates the schema for the read_file tool.
func generateReadFileSchema() *client.Tool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to read (relative to working directory or absolute).",
			},
			"start_line": map[string]interface{}{
				"type":        "integer",
				"description": "Optional starting line number (1-indexed) to read from.",
			},
			"end_line": map[string]interface{}{
				"type":        "integer",
				"description": "Optional ending line number (inclusive) to read to.",
			},
		},
		"required": []string{"path"},
	}

	paramsJSON, _ := json.Marshal(params)

	return &client.Tool{
		Type: "function",
		Function: &client.FunctionDefinition{
			Name:        "read_file",
			Description: "Read the contents of a file. Supports reading entire files or specific line ranges. Cannot read binary files. Use this to examine code, configuration files, documentation, logs, etc.",
			Parameters:  json.RawMessage(paramsJSON),
		},
	}
}

// generateWriteFileSchema creates the schema for the write_file tool.
func generateWriteFileSchema() *client.Tool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to write (relative to working directory or absolute).",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The content to write to the file. Will overwrite existing content.",
			},
			"create_directories": map[string]interface{}{
				"type":        "boolean",
				"description": "Create parent directories if they don't exist.",
			},
		},
		"required": []string{"path", "content"},
	}

	paramsJSON, _ := json.Marshal(params)

	return &client.Tool{
		Type: "function",
		Function: &client.FunctionDefinition{
			Name:        "write_file",
			Description: "Write content to a file. Creates the file if it doesn't exist, overwrites if it does. Use this to create new files, update existing files, generate code, save outputs, etc.",
			Parameters:  json.RawMessage(paramsJSON),
		},
	}
}

// generateListDirectorySchema creates the schema for the list_directory tool.
func generateListDirectorySchema() *client.Tool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the directory to list (relative to working directory or absolute). Defaults to current directory if not specified.",
			},
			"recursive": map[string]interface{}{
				"type":        "boolean",
				"description": "List files recursively in subdirectories.",
			},
			"include_hidden": map[string]interface{}{
				"type":        "boolean",
				"description": "Include hidden files (those starting with '.').",
			},
		},
		"required": []string{},
	}

	paramsJSON, _ := json.Marshal(params)

	return &client.Tool{
		Type: "function",
		Function: &client.FunctionDefinition{
			Name:        "list_dir",
			Description: "List files and directories. Supports recursive listing and hidden files. Use this to explore directory structures, find files, understand project organization, etc.",
			Parameters:  json.RawMessage(paramsJSON),
		},
	}
}

// generateGrepSchema creates the schema for the grep tool.
func generateGrepSchema() *client.Tool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "Regular expression pattern to search for.",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File or directory to search in (defaults to current directory).",
			},
			"recursive": map[string]interface{}{
				"type":        "boolean",
				"description": "Search recursively in subdirectories.",
			},
			"case_insensitive": map[string]interface{}{
				"type":        "boolean",
				"description": "Perform case-insensitive search.",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of results to return.",
			},
		},
		"required": []string{"pattern"},
	}

	paramsJSON, _ := json.Marshal(params)

	return &client.Tool{
		Type: "function",
		Function: &client.FunctionDefinition{
			Name:        "grep_files",
			Description: "Search for text patterns in files using regular expressions. Supports recursive search and case-insensitive matching. Use this to find code patterns, search logs, locate string occurrences, etc.",
			Parameters:  json.RawMessage(paramsJSON),
		},
	}
}
