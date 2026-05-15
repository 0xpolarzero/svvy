// Package tools provides helper functions for initializing the tool registry
// with all available tools.
package tools

import (
	"github.com/evmts/codex/codex-go/internal/tools/file"
	"github.com/evmts/codex/codex-go/internal/tools/git"
	"github.com/evmts/codex/codex-go/internal/tools/image"
	"github.com/evmts/codex/codex-go/internal/tools/plan"
	"github.com/evmts/codex/codex-go/internal/tools/runtime"
	"github.com/evmts/codex/codex-go/internal/tools/shell"
	"github.com/evmts/codex/codex-go/internal/tools/websearch"
	"github.com/spf13/afero"
)

// NewDefaultRegistry creates and populates a tool registry with all standard tools.
// This includes:
//   - shell: Execute shell commands
//   - read_file: Read file contents
//   - write_file: Write file contents
//   - list_dir: List directory contents
//   - grep_files: Search files with regex patterns
//   - view_image: View and attach images to conversation
//   - update_plan: Update task/todo list
//   - web_search: Perform web searches (if enabled)
//   - git_status: Show git working tree status
//   - git_diff: Show changes between commits and working tree
//   - git_log: Show commit history
//   - git_commit: Record changes to the repository
func NewDefaultRegistry() *runtime.ToolRegistry {
	registry := runtime.NewToolRegistry()

	// Use OS filesystem for all file operations
	fs := afero.NewOsFs()

	// Register shell tool
	registry.Register(shell.NewShellTool())

	// Register file tools
	registry.Register(file.NewReadTool(fs))
	registry.Register(file.NewWriteTool(fs))
	registry.Register(file.NewListTool(fs))
	registry.Register(file.NewGrepTool(fs))

	// Register image tool
	registry.Register(image.NewImageTool())

	// Register plan tool (without event emitter for basic registry)
	// Sessions can replace this with an event-emitting version if needed
	registry.Register(plan.NewBasicUpdatePlanTool())

	// Register git tools
	registry.Register(git.NewStatusTool())
	registry.Register(git.NewDiffTool())
	registry.Register(git.NewLogTool())
	registry.Register(git.NewCommitTool())

	return registry
}

// NewDefaultRegistryWithWebSearch creates a registry with web search enabled.
// provider: The search provider to use (e.g., "duckduckgo")
func NewDefaultRegistryWithWebSearch(provider string) *runtime.ToolRegistry {
	registry := NewDefaultRegistry()

	// Add web search tool based on provider
	var searchProvider websearch.Provider
	switch provider {
	case "duckduckgo", "":
		searchProvider = websearch.NewDuckDuckGoProvider()
	default:
		// Default to DuckDuckGo if unknown provider
		searchProvider = websearch.NewDuckDuckGoProvider()
	}

	registry.Register(websearch.NewWebSearchTool(searchProvider))

	return registry
}

// NewAutoApprovalCache creates an approval cache that auto-approves everything.
// This is suitable for trusted environments where user approval is not required.
func NewAutoApprovalCache() runtime.ApprovalCache {
	return &autoApprovalCache{
		cache: make(map[string]runtime.ApprovalDecision),
	}
}

// autoApprovalCache implements ApprovalCache with auto-approval behavior.
type autoApprovalCache struct {
	cache map[string]runtime.ApprovalDecision
}

// Get retrieves a cached approval decision.
func (c *autoApprovalCache) Get(key string) *runtime.ApprovalDecision {
	if decision, ok := c.cache[key]; ok {
		return &decision
	}
	return nil
}

// Put stores an approval decision.
func (c *autoApprovalCache) Put(key string, decision runtime.ApprovalDecision) {
	c.cache[key] = decision
}
