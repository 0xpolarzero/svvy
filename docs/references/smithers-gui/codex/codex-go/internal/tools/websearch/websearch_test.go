package websearch

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/evmts/codex/codex-go/internal/tools/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockProvider is a mock search provider for testing.
type MockProvider struct {
	searchFunc func(ctx context.Context, query string, maxResults int) (*SearchResponse, error)
	name       string
}

func (m *MockProvider) Search(ctx context.Context, query string, maxResults int) (*SearchResponse, error) {
	if m.searchFunc != nil {
		return m.searchFunc(ctx, query, maxResults)
	}
	return &SearchResponse{
		Query:     query,
		Results:   []SearchResult{},
		Timestamp: time.Now(),
	}, nil
}

func (m *MockProvider) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

func TestWebSearchTool_Execute_Success(t *testing.T) {
	// Create mock provider that returns test results
	mockProvider := &MockProvider{
		searchFunc: func(ctx context.Context, query string, maxResults int) (*SearchResponse, error) {
			return &SearchResponse{
				Query: query,
				Results: []SearchResult{
					{
						Title:   "Test Result 1",
						URL:     "https://example.com/1",
						Snippet: "This is test result 1",
					},
					{
						Title:   "Test Result 2",
						URL:     "https://example.com/2",
						Snippet: "This is test result 2",
					},
				},
				Timestamp: time.Now(),
			}, nil
		},
	}

	tool := NewWebSearchTool(mockProvider)

	// Prepare request
	args := searchArgs{
		Query:      "golang tutorial",
		MaxResults: 10,
	}
	argsJSON, err := json.Marshal(args)
	require.NoError(t, err)

	req := &runtime.ToolRequest{
		CallID:    "test-call-123",
		ToolName:  "web_search",
		Arguments: string(argsJSON),
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
	}

	// Execute tool
	resp, err := tool.Execute(context.Background(), req, execCtx)

	// Verify response
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, *resp.Success)
	assert.Contains(t, resp.Content, "golang tutorial")
	assert.Contains(t, resp.Content, "Test Result 1")
	assert.Contains(t, resp.Content, "https://example.com/1")
	assert.Contains(t, resp.Content, "Test Result 2")
	assert.Greater(t, resp.ExecutionTime, time.Duration(0))

	// Verify metadata
	assert.Equal(t, "golang tutorial", resp.Metadata["query"])
	assert.Equal(t, 2, resp.Metadata["result_count"])
	assert.Equal(t, "mock", resp.Metadata["provider"])
}

func TestWebSearchTool_Execute_EmptyQuery(t *testing.T) {
	mockProvider := &MockProvider{}
	tool := NewWebSearchTool(mockProvider)

	// Empty query should fail
	args := searchArgs{
		Query:      "",
		MaxResults: 10,
	}
	argsJSON, err := json.Marshal(args)
	require.NoError(t, err)

	req := &runtime.ToolRequest{
		CallID:    "test-call-123",
		ToolName:  "web_search",
		Arguments: string(argsJSON),
	}

	execCtx := &runtime.ExecutionContext{}

	_, err = tool.Execute(context.Background(), req, execCtx)

	// Should return error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query cannot be empty")
}

func TestWebSearchTool_Execute_InvalidJSON(t *testing.T) {
	mockProvider := &MockProvider{}
	tool := NewWebSearchTool(mockProvider)

	req := &runtime.ToolRequest{
		CallID:    "test-call-123",
		ToolName:  "web_search",
		Arguments: "not valid json",
	}

	execCtx := &runtime.ExecutionContext{}

	_, err := tool.Execute(context.Background(), req, execCtx)

	// Should return error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse web search arguments")
}

func TestWebSearchTool_Execute_NoResults(t *testing.T) {
	mockProvider := &MockProvider{
		searchFunc: func(ctx context.Context, query string, maxResults int) (*SearchResponse, error) {
			return &SearchResponse{
				Query:     query,
				Results:   []SearchResult{},
				Timestamp: time.Now(),
			}, nil
		},
	}

	tool := NewWebSearchTool(mockProvider)

	args := searchArgs{
		Query:      "nonexistent query",
		MaxResults: 10,
	}
	argsJSON, err := json.Marshal(args)
	require.NoError(t, err)

	req := &runtime.ToolRequest{
		CallID:    "test-call-123",
		ToolName:  "web_search",
		Arguments: string(argsJSON),
	}

	execCtx := &runtime.ExecutionContext{}

	resp, err := tool.Execute(context.Background(), req, execCtx)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, *resp.Success)
	assert.Contains(t, resp.Content, "No results found")
	assert.Equal(t, 0, resp.Metadata["result_count"])
}

func TestWebSearchTool_EventEmission(t *testing.T) {
	mockProvider := &MockProvider{
		searchFunc: func(ctx context.Context, query string, maxResults int) (*SearchResponse, error) {
			return &SearchResponse{
				Query: query,
				Results: []SearchResult{
					{Title: "Test", URL: "https://example.com", Snippet: "Test snippet"},
				},
				Timestamp: time.Now(),
			}, nil
		},
	}

	// Track emitted events
	var events []protocol.EventMsg
	emitter := func(event protocol.EventMsg) {
		events = append(events, event)
	}

	tool := NewWebSearchToolWithEmitter(mockProvider, emitter)

	args := searchArgs{
		Query:      "test query",
		MaxResults: 5,
	}
	argsJSON, err := json.Marshal(args)
	require.NoError(t, err)

	req := &runtime.ToolRequest{
		CallID:    "test-call-456",
		ToolName:  "web_search",
		Arguments: string(argsJSON),
	}

	execCtx := &runtime.ExecutionContext{}

	_, err = tool.Execute(context.Background(), req, execCtx)
	require.NoError(t, err)

	// Should have emitted begin and end events
	assert.Len(t, events, 2)

	// Check begin event
	beginEvent, ok := events[0].(*protocol.EventWebSearchBegin)
	assert.True(t, ok)
	assert.Equal(t, "test-call-456", beginEvent.CallID)

	// Check end event
	endEvent, ok := events[1].(*protocol.EventWebSearchEnd)
	assert.True(t, ok)
	assert.Equal(t, "test-call-456", endEvent.CallID)
	assert.Equal(t, "test query", endEvent.Query)
}

func TestWebSearchTool_DefaultMaxResults(t *testing.T) {
	var capturedMaxResults int
	mockProvider := &MockProvider{
		searchFunc: func(ctx context.Context, query string, maxResults int) (*SearchResponse, error) {
			capturedMaxResults = maxResults
			return &SearchResponse{
				Query:     query,
				Results:   []SearchResult{},
				Timestamp: time.Now(),
			}, nil
		},
	}

	tool := NewWebSearchTool(mockProvider)

	// Don't specify max_results
	args := searchArgs{
		Query: "test",
	}
	argsJSON, err := json.Marshal(args)
	require.NoError(t, err)

	req := &runtime.ToolRequest{
		CallID:    "test-call",
		ToolName:  "web_search",
		Arguments: string(argsJSON),
	}

	execCtx := &runtime.ExecutionContext{}

	_, err = tool.Execute(context.Background(), req, execCtx)
	require.NoError(t, err)

	// Should default to 10
	assert.Equal(t, 10, capturedMaxResults)
}

func TestWebSearchTool_ApprovalNotRequired(t *testing.T) {
	mockProvider := &MockProvider{}
	tool := NewWebSearchTool(mockProvider)

	req := &runtime.ToolRequest{
		ToolName:  "web_search",
		Arguments: `{"query": "test"}`,
	}

	// Web search should not require approval
	assert.False(t, tool.NeedsInitialApproval(req, runtime.ApprovalUnlessTrusted, runtime.SandboxReadOnly))
	assert.False(t, tool.NeedsRetryApproval(runtime.ApprovalUnlessTrusted))
}

func TestWebSearchTool_SandboxPreference(t *testing.T) {
	mockProvider := &MockProvider{}
	tool := NewWebSearchTool(mockProvider)

	// Web search doesn't need sandbox
	assert.Equal(t, runtime.SandboxForbid, tool.SandboxPreference())
}

func TestWebSearchTool_Name(t *testing.T) {
	mockProvider := &MockProvider{}
	tool := NewWebSearchTool(mockProvider)

	assert.Equal(t, "web_search", tool.Name())
}

func TestWebSearchTool_FormatResults(t *testing.T) {
	mockProvider := &MockProvider{}
	tool := NewWebSearchTool(mockProvider)

	response := &SearchResponse{
		Query: "test query",
		Results: []SearchResult{
			{
				Title:   "First Result",
				URL:     "https://example.com/first",
				Snippet: "This is the first result",
			},
			{
				Title:   "Second Result",
				URL:     "https://example.com/second",
				Snippet: "This is the second result",
			},
		},
		Timestamp: time.Now(),
	}

	formatted := tool.formatResults(response)

	assert.Contains(t, formatted, "test query")
	assert.Contains(t, formatted, "1. First Result")
	assert.Contains(t, formatted, "https://example.com/first")
	assert.Contains(t, formatted, "This is the first result")
	assert.Contains(t, formatted, "2. Second Result")
	assert.Contains(t, formatted, "https://example.com/second")
	assert.Contains(t, formatted, "This is the second result")
}
