package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// WebSearchTool implements the ToolRuntime interface for web search operations.
type WebSearchTool struct {
	provider Provider
	// EventEmitter is called to emit web search events (begin/end).
	// This is optional and can be nil if event emission is not needed.
	EventEmitter func(event protocol.EventMsg)
}

// NewWebSearchTool creates a new web search tool with the given provider.
func NewWebSearchTool(provider Provider) *WebSearchTool {
	return &WebSearchTool{
		provider: provider,
	}
}

// NewWebSearchToolWithEmitter creates a new web search tool with event emission.
func NewWebSearchToolWithEmitter(provider Provider, emitter func(protocol.EventMsg)) *WebSearchTool {
	return &WebSearchTool{
		provider:     provider,
		EventEmitter: emitter,
	}
}

// searchArgs represents the parsed arguments for the web_search tool.
type searchArgs struct {
	// Query is the search query string
	Query string `json:"query"`

	// MaxResults is the maximum number of results to return (optional, default 10)
	MaxResults int `json:"max_results,omitempty"`
}

// Name returns the unique identifier for this tool.
func (t *WebSearchTool) Name() string {
	return "web_search"
}

// Execute performs a web search and returns formatted results.
func (t *WebSearchTool) Execute(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
	startTime := time.Now()

	// Parse arguments
	args, err := t.parseArguments(req.Arguments)
	if err != nil {
		return nil, runtime.NewToolErrorWithCause(
			runtime.ErrorInvalidArguments,
			"failed to parse web search arguments",
			err,
		)
	}

	// Validate query
	if args.Query == "" {
		return nil, runtime.NewToolError(
			runtime.ErrorInvalidArguments,
			"query cannot be empty",
		)
	}

	// Set default max results
	if args.MaxResults <= 0 {
		args.MaxResults = 10
	}

	// Emit web search begin event
	if t.EventEmitter != nil {
		t.EventEmitter(&protocol.EventWebSearchBegin{
			CallID: req.CallID,
		})
	}

	// Perform the search
	response, err := t.provider.Search(ctx, args.Query, args.MaxResults)
	if err != nil {
		// Emit end event even on error
		if t.EventEmitter != nil {
			t.EventEmitter(&protocol.EventWebSearchEnd{
				CallID: req.CallID,
				Query:  args.Query,
			})
		}

		return nil, runtime.NewToolErrorWithCause(
			runtime.ErrorExecution,
			fmt.Sprintf("web search failed: %v", err),
			err,
		)
	}

	// Format results for the model
	content := t.formatResults(response)

	// Emit web search end event
	if t.EventEmitter != nil {
		t.EventEmitter(&protocol.EventWebSearchEnd{
			CallID: req.CallID,
			Query:  args.Query,
		})
	}

	success := true
	return &runtime.ToolResponse{
		Content:       content,
		Success:       &success,
		ExecutionTime: time.Since(startTime),
		Metadata: map[string]interface{}{
			"query":        args.Query,
			"result_count": len(response.Results),
			"provider":     t.provider.Name(),
			"timestamp":    response.Timestamp.Format(time.RFC3339),
		},
	}, nil
}

// parseArguments parses the JSON arguments for the web search tool.
func (t *WebSearchTool) parseArguments(argsJSON string) (*searchArgs, error) {
	var args searchArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return &args, nil
}

// formatResults formats the search results for the model.
// Returns a human-readable string with all search results.
func (t *WebSearchTool) formatResults(response *SearchResponse) string {
	if len(response.Results) == 0 {
		return fmt.Sprintf("No results found for query: %s", response.Query)
	}

	result := fmt.Sprintf("Web search results for \"%s\":\n\n", response.Query)

	for i, res := range response.Results {
		result += fmt.Sprintf("%d. %s\n", i+1, res.Title)
		result += fmt.Sprintf("   URL: %s\n", res.URL)
		if res.Snippet != "" {
			result += fmt.Sprintf("   %s\n", res.Snippet)
		}
		result += "\n"
	}

	return result
}

// ApprovalKey generates a unique key for caching approval decisions.
// Web search operations typically don't require approval, so we return empty.
func (t *WebSearchTool) ApprovalKey(req *runtime.ToolRequest) string {
	return ""
}

// NeedsInitialApproval determines if this tool requires approval before execution.
// Web search is typically safe and doesn't require approval.
func (t *WebSearchTool) NeedsInitialApproval(req *runtime.ToolRequest, approvalPolicy runtime.ApprovalPolicy, sandboxPolicy runtime.SandboxPolicy) bool {
	return false
}

// NeedsRetryApproval determines if this tool needs approval for retry after sandbox denial.
// Not applicable for web search.
func (t *WebSearchTool) NeedsRetryApproval(approvalPolicy runtime.ApprovalPolicy) bool {
	return false
}

// SandboxPreference returns the sandbox preference for this tool.
// Web search doesn't need sandbox isolation.
func (t *WebSearchTool) SandboxPreference() runtime.SandboxPreference {
	return runtime.SandboxForbid
}

// EscalateOnFailure indicates whether to retry without sandbox on permission error.
// Not applicable for web search.
func (t *WebSearchTool) EscalateOnFailure() bool {
	return false
}

// WantsEscalatedFirstAttempt indicates if this tool should skip sandbox on first try.
// Web search doesn't use sandbox.
func (t *WebSearchTool) WantsEscalatedFirstAttempt(req *runtime.ToolRequest) bool {
	return false
}

// SupportsParallel indicates if multiple invocations can run concurrently.
// Web search is safe to run in parallel.
func (t *WebSearchTool) SupportsParallel() bool {
	return true
}

// SandboxRetryData extracts command metadata for sandbox retry.
// Not applicable for web search - returns nil.
func (t *WebSearchTool) SandboxRetryData(req *runtime.ToolRequest) *runtime.SandboxRetryData {
	return nil
}
