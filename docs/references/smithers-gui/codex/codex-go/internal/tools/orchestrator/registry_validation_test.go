package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// ============================================================================
// SanitizeCallID Tests
// ============================================================================

func TestSanitizeCallID(t *testing.T) {
	tests := []struct {
		name    string
		callID  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid call ID",
			callID:  "call-123_abc",
			wantErr: false,
		},
		{
			name:    "valid alphanumeric",
			callID:  "ABC123xyz",
			wantErr: false,
		},
		{
			name:    "valid with underscores",
			callID:  "call_123_test",
			wantErr: false,
		},
		{
			name:    "valid with hyphens",
			callID:  "call-123-test",
			wantErr: false,
		},
		{
			name:    "empty call ID",
			callID:  "",
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "call ID with special chars",
			callID:  "call-123;rm -rf",
			wantErr: true,
			errMsg:  "invalid character",
		},
		{
			name:    "call ID with spaces",
			callID:  "call 123",
			wantErr: true,
			errMsg:  "invalid character",
		},
		{
			name:    "call ID with null byte",
			callID:  "call\x00id",
			wantErr: true,
			errMsg:  "null byte",
		},
		{
			name:    "oversized call ID",
			callID:  strings.Repeat("a", MaxCallIDLength+1),
			wantErr: true,
			errMsg:  "exceeds maximum length",
		},
		{
			name:    "max length call ID",
			callID:  strings.Repeat("a", MaxCallIDLength),
			wantErr: false,
		},
		{
			name:    "call ID with dots",
			callID:  "call.123",
			wantErr: true,
			errMsg:  "invalid character",
		},
		{
			name:    "call ID with slashes",
			callID:  "call/123",
			wantErr: true,
			errMsg:  "invalid character",
		},
		{
			name:    "call ID with brackets",
			callID:  "call[123]",
			wantErr: true,
			errMsg:  "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SanitizeCallID(tt.callID)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ============================================================================
// SanitizeToolName Tests
// ============================================================================

func TestSanitizeToolName(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid tool name",
			toolName: "shell",
			wantErr:  false,
		},
		{
			name:     "valid namespaced tool",
			toolName: "mcp/fetch",
			wantErr:  false,
		},
		{
			name:     "valid with dots",
			toolName: "tool.v1",
			wantErr:  false,
		},
		{
			name:     "valid with underscores",
			toolName: "my_tool",
			wantErr:  false,
		},
		{
			name:     "valid with hyphens",
			toolName: "my-tool",
			wantErr:  false,
		},
		{
			name:     "empty tool name",
			toolName: "",
			wantErr:  true,
			errMsg:   "cannot be empty",
		},
		{
			name:     "path traversal attack",
			toolName: "../../../etc/passwd",
			wantErr:  true,
			errMsg:   "path traversal",
		},
		{
			name:     "path traversal with dots",
			toolName: "tool/../malicious",
			wantErr:  true,
			errMsg:   "path traversal",
		},
		{
			name:     "tool name with null byte",
			toolName: "tool\x00name",
			wantErr:  true,
			errMsg:   "null byte",
		},
		{
			name:     "oversized tool name",
			toolName: strings.Repeat("a", MaxToolNameLength+1),
			wantErr:  true,
			errMsg:   "exceeds maximum length",
		},
		{
			name:     "max length tool name",
			toolName: strings.Repeat("a", MaxToolNameLength),
			wantErr:  false,
		},
		{
			name:     "tool name with special chars",
			toolName: "tool;rm -rf",
			wantErr:  true,
			errMsg:   "invalid character",
		},
		{
			name:     "tool name with spaces",
			toolName: "my tool",
			wantErr:  true,
			errMsg:   "invalid character",
		},
		{
			name:     "tool name with brackets",
			toolName: "tool[1]",
			wantErr:  true,
			errMsg:   "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SanitizeToolName(tt.toolName)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ============================================================================
// ValidateArgumentStructure Tests
// ============================================================================

func TestValidateArgumentStructure(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid simple args",
			args: map[string]interface{}{
				"name":  "value",
				"count": 42,
			},
			wantErr: false,
		},
		{
			name: "valid nested structure",
			args: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "john",
					"age":  30,
				},
			},
			wantErr: false,
		},
		{
			name: "valid array",
			args: map[string]interface{}{
				"items": []interface{}{"a", "b", "c"},
			},
			wantErr: false,
		},
		{
			name: "valid mixed types",
			args: map[string]interface{}{
				"string":  "value",
				"int":     42,
				"float":   3.14,
				"bool":    true,
				"null":    nil,
				"array":   []interface{}{1, 2, 3},
				"object":  map[string]interface{}{"key": "value"},
			},
			wantErr: false,
		},
		{
			name: "oversized string",
			args: map[string]interface{}{
				"data": strings.Repeat("x", MaxStringSize+1),
			},
			wantErr: true,
			errMsg:  "exceeds maximum size",
		},
		{
			name:    "deeply nested structure",
			args:    createDeeplyNested(MaxArgumentDepth + 1),
			wantErr: true,
			errMsg:  "exceeds maximum depth",
		},
		{
			name: "oversized array",
			args: map[string]interface{}{
				"items": make([]interface{}, MaxArraySize+1),
			},
			wantErr: true,
			errMsg:  "exceeds maximum size",
		},
		{
			name: "null byte in string",
			args: map[string]interface{}{
				"data": "value\x00injection",
			},
			wantErr: true,
			errMsg:  "null byte",
		},
		{
			name: "empty object key",
			args: map[string]interface{}{
				"nested": map[string]interface{}{
					"": "value",
				},
			},
			wantErr: true,
			errMsg:  "key cannot be empty",
		},
		{
			name: "oversized object key",
			args: map[string]interface{}{
				"nested": map[string]interface{}{
					strings.Repeat("k", 257): "value",
				},
			},
			wantErr: true,
			errMsg:  "key exceeds maximum length",
		},
		{
			name: "null byte in object key",
			args: map[string]interface{}{
				"nested": map[string]interface{}{
					"key\x00name": "value",
				},
			},
			wantErr: true,
			errMsg:  "null byte",
		},
		{
			name: "oversized object",
			args: map[string]interface{}{
				"nested": createLargeObject(MaxArraySize + 1),
			},
			wantErr: true,
			errMsg:  "exceeds maximum size",
		},
		{
			name: "max depth nested structure",
			args: createDeeplyNested(MaxArgumentDepth - 1),
			wantErr: false,
		},
		{
			name: "max size array",
			args: map[string]interface{}{
				"items": make([]interface{}, MaxArraySize),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArgumentStructure(tt.args)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ============================================================================
// Enhanced ValidateToolRequest Tests
// ============================================================================

func TestValidateToolRequest_Enhanced(t *testing.T) {
	registry := runtime.NewToolRegistry()
	tool := NewMockTool("valid_tool")
	registry.Register(tool)
	helper := NewRegistryHelper(registry)

	tests := []struct {
		name    string
		req     *runtime.ToolRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid request",
			req: &runtime.ToolRequest{
				CallID:   "call-123",
				ToolName: "valid_tool",
			},
			wantErr: false,
		},
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
			errMsg:  "request is nil",
		},
		{
			name: "invalid call ID - empty",
			req: &runtime.ToolRequest{
				CallID:   "",
				ToolName: "valid_tool",
			},
			wantErr: true,
			errMsg:  "invalid CallID",
		},
		{
			name: "invalid call ID - special chars",
			req: &runtime.ToolRequest{
				CallID:   "call;rm -rf",
				ToolName: "valid_tool",
			},
			wantErr: true,
			errMsg:  "invalid CallID",
		},
		{
			name: "invalid tool name - empty",
			req: &runtime.ToolRequest{
				CallID:   "call-123",
				ToolName: "",
			},
			wantErr: true,
			errMsg:  "invalid ToolName",
		},
		{
			name: "invalid tool name - path traversal",
			req: &runtime.ToolRequest{
				CallID:   "call-123",
				ToolName: "../../../etc/passwd",
			},
			wantErr: true,
			errMsg:  "invalid ToolName",
		},
		{
			name: "tool not found",
			req: &runtime.ToolRequest{
				CallID:   "call-123",
				ToolName: "nonexistent_tool",
			},
			wantErr: true,
			errMsg:  "tool not found",
		},
		{
			name: "valid with arguments",
			req: &runtime.ToolRequest{
				CallID:    "call-123",
				ToolName:  "valid_tool",
				Arguments: `{"key": "value"}`,
			},
			wantErr: false,
		},
		{
			name: "valid with empty arguments",
			req: &runtime.ToolRequest{
				CallID:    "call-123",
				ToolName:  "valid_tool",
				Arguments: "{}",
			},
			wantErr: false,
		},
		{
			name: "invalid arguments - not JSON",
			req: &runtime.ToolRequest{
				CallID:    "call-123",
				ToolName:  "valid_tool",
				Arguments: "not json",
			},
			wantErr: true,
			errMsg:  "not valid JSON",
		},
		{
			name: "invalid arguments - oversized",
			req: &runtime.ToolRequest{
				CallID:    "call-123",
				ToolName:  "valid_tool",
				Arguments: `{"data": "` + strings.Repeat("x", MaxArgumentsSize) + `"}`,
			},
			wantErr: true,
			errMsg:  "exceed maximum size",
		},
		{
			name: "invalid arguments - deeply nested",
			req: &runtime.ToolRequest{
				CallID:    "call-123",
				ToolName:  "valid_tool",
				Arguments: toJSON(createDeeplyNested(MaxArgumentDepth + 1)),
			},
			wantErr: true,
			errMsg:  "exceeds maximum depth",
		},
		{
			name: "invalid arguments - null byte",
			req: &runtime.ToolRequest{
				CallID:    "call-123",
				ToolName:  "valid_tool",
				Arguments: `{"data": "value\u0000injection"}`,
			},
			wantErr: true,
			errMsg:  "null byte",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := helper.ValidateToolRequest(tt.req)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ============================================================================
// Enhanced ValidateToolRequests Tests
// ============================================================================

func TestValidateToolRequests_Enhanced(t *testing.T) {
	registry := runtime.NewToolRegistry()
	tool := NewMockTool("valid_tool")
	registry.Register(tool)
	helper := NewRegistryHelper(registry)

	tests := []struct {
		name     string
		requests []*runtime.ToolRequest
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid single request",
			requests: []*runtime.ToolRequest{
				{
					CallID:   "call-1",
					ToolName: "valid_tool",
				},
			},
			wantErr: false,
		},
		{
			name: "valid multiple requests",
			requests: []*runtime.ToolRequest{
				{
					CallID:   "call-1",
					ToolName: "valid_tool",
				},
				{
					CallID:   "call-2",
					ToolName: "valid_tool",
				},
			},
			wantErr: false,
		},
		{
			name:     "empty requests",
			requests: []*runtime.ToolRequest{},
			wantErr:  true,
			errMsg:   "cannot be empty",
		},
		{
			name:     "nil requests",
			requests: nil,
			wantErr:  true,
			errMsg:   "cannot be empty",
		},
		{
			name:     "too many requests",
			requests: createManyRequests(MaxRequestsPerBatch + 1),
			wantErr:  true,
			errMsg:   "exceeds maximum",
		},
		{
			name:     "max requests",
			requests: createManyRequests(MaxRequestsPerBatch),
			wantErr:  false,
		},
		{
			name: "invalid request in batch",
			requests: []*runtime.ToolRequest{
				{
					CallID:   "call-1",
					ToolName: "valid_tool",
				},
				{
					CallID:   "invalid;call",
					ToolName: "valid_tool",
				},
			},
			wantErr: true,
			errMsg:  "request[1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := helper.ValidateToolRequests(tt.requests)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ============================================================================
// Edge Case Tests
// ============================================================================

func TestValidateValue_UnsupportedType(t *testing.T) {
	type CustomType struct {
		Field string
	}

	err := validateValue(CustomType{Field: "value"}, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported argument type")
}

func TestValidateValue_PrimitiveTypes(t *testing.T) {
	primitives := []interface{}{
		true,
		false,
		int(42),
		int8(42),
		int16(42),
		int32(42),
		int64(42),
		uint(42),
		uint8(42),
		uint16(42),
		uint32(42),
		uint64(42),
		float32(3.14),
		float64(3.14),
		nil,
	}

	for i, val := range primitives {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			err := validateValue(val, 0)
			require.NoError(t, err)
		})
	}
}

func TestValidateValue_ArrayOfPrimitives(t *testing.T) {
	args := []interface{}{1, 2, 3, "a", "b", true, false, nil}
	err := validateValue(args, 0)
	require.NoError(t, err)
}

func TestValidateValue_ComplexNested(t *testing.T) {
	args := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"name": "john",
				"tags": []interface{}{"admin", "user"},
				"metadata": map[string]interface{}{
					"created": "2024-01-01",
					"updated": "2024-01-02",
				},
			},
		},
	}
	err := validateValue(args, 0)
	require.NoError(t, err)
}

// ============================================================================
// Helper Functions
// ============================================================================

func createDeeplyNested(depth int) map[string]interface{} {
	if depth == 0 {
		return map[string]interface{}{"value": "leaf"}
	}
	return map[string]interface{}{
		"nested": createDeeplyNested(depth - 1),
	}
}

func createLargeObject(size int) map[string]interface{} {
	obj := make(map[string]interface{}, size)
	for i := 0; i < size; i++ {
		obj[string(rune('a'+i%26))+string(rune('0'+i/26))] = i
	}
	return obj
}

func createManyRequests(count int) []*runtime.ToolRequest {
	requests := make([]*runtime.ToolRequest, count)
	for i := 0; i < count; i++ {
		requests[i] = &runtime.ToolRequest{
			CallID:   string(rune('a'+i%26)) + "-" + string(rune('0'+i/10)),
			ToolName: "valid_tool",
		}
	}
	return requests
}

func toJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

// ============================================================================
// Security Tests
// ============================================================================

func TestSecurity_CallIDInjection(t *testing.T) {
	injectionAttempts := []string{
		"call; rm -rf /",
		"call && malicious",
		"call | cat /etc/passwd",
		"call\nrm -rf",
		"call\x00injection",
		"call`whoami`",
		"call$(whoami)",
		"call${USER}",
		"call<script>",
	}

	for _, attempt := range injectionAttempts {
		t.Run(attempt, func(t *testing.T) {
			err := SanitizeCallID(attempt)
			require.Error(t, err, "Should reject injection attempt: %s", attempt)
		})
	}
}

func TestSecurity_ToolNameInjection(t *testing.T) {
	injectionAttempts := []string{
		"../../../etc/passwd",
		"tool/../../../sensitive",
		"tool\x00malicious",
		"tool; rm -rf",
		"tool && malicious",
		"tool | cat /etc/passwd",
		"tool`whoami`",
		"tool$(whoami)",
	}

	for _, attempt := range injectionAttempts {
		t.Run(attempt, func(t *testing.T) {
			err := SanitizeToolName(attempt)
			require.Error(t, err, "Should reject injection attempt: %s", attempt)
		})
	}
}

func TestSecurity_ArgumentInjection(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
	}{
		{
			name: "null byte in string",
			args: map[string]interface{}{
				"cmd": "value\x00injection",
			},
		},
		{
			name: "null byte in key",
			args: map[string]interface{}{
				"key\x00name": "value",
			},
		},
		{
			name: "empty key",
			args: map[string]interface{}{
				"": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArgumentStructure(tt.args)
			require.Error(t, err, "Should reject malicious arguments")
		})
	}
}

// ============================================================================
// Performance Tests
// ============================================================================

func BenchmarkSanitizeCallID(b *testing.B) {
	callID := "valid-call-id-123"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizeCallID(callID)
	}
}

func BenchmarkSanitizeToolName(b *testing.B) {
	toolName := "mcp/valid-tool"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizeToolName(toolName)
	}
}

func BenchmarkValidateArgumentStructure_Simple(b *testing.B) {
	args := map[string]interface{}{
		"name":  "value",
		"count": 42,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateArgumentStructure(args)
	}
}

func BenchmarkValidateArgumentStructure_Complex(b *testing.B) {
	args := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"name": "john",
				"tags": []interface{}{"admin", "user"},
			},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateArgumentStructure(args)
	}
}

func BenchmarkValidateToolRequest(b *testing.B) {
	registry := runtime.NewToolRegistry()
	tool := NewMockTool("test_tool")
	registry.Register(tool)
	helper := NewRegistryHelper(registry)

	req := &runtime.ToolRequest{
		CallID:    "call-123",
		ToolName:  "test_tool",
		Arguments: `{"key": "value"}`,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = helper.ValidateToolRequest(req)
	}
}
