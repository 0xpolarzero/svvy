package file

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
	"github.com/spf13/afero"
)

// ListTool implements the list_dir tool runtime.
type ListTool struct {
	fs afero.Fs
}

// NewListTool creates a new ListTool with the given filesystem.
func NewListTool(fs afero.Fs) *ListTool {
	return &ListTool{fs: fs}
}

// listArgs represents the arguments for list_dir tool.
type listArgs struct {
	Path       string `json:"path"`
	Recursive  bool   `json:"recursive,omitempty"`
	Pattern    string `json:"pattern,omitempty"`
	ShowHidden bool   `json:"show_hidden,omitempty"`
	MaxDepth   *int   `json:"max_depth,omitempty"`
}

// fileEntry represents a file or directory entry.
type fileEntry struct {
	Path  string
	IsDir bool
	Size  int64
}

// Name returns the tool name.
func (t *ListTool) Name() string {
	return "list_dir"
}

// Execute lists directory contents.
func (t *ListTool) Execute(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
	startTime := time.Now()

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return nil, runtime.NewToolErrorWithCause(runtime.ErrorExecution, "context cancelled", err)
	}

	// Parse arguments
	var args listArgs
	if err := json.Unmarshal([]byte(req.Arguments), &args); err != nil {
		return nil, runtime.NewToolErrorWithCause(runtime.ErrorInvalidArguments, "invalid arguments", err)
	}

	if args.Path == "" {
		args.Path = "."
	}

	// Validate path
	fullPath, err := validatePath(req.WorkingDirectory, args.Path)
	if err != nil {
		return nil, runtime.NewToolErrorWithCause(runtime.ErrorInvalidArguments, err.Error(), err)
	}

	// Check if path exists
	exists, err := afero.Exists(t.fs, fullPath)
	if err != nil {
		success := false
		return &runtime.ToolResponse{
			Content:       fmt.Sprintf("Error checking path: %v", err),
			Success:       &success,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	if !exists {
		success := false
		return &runtime.ToolResponse{
			Content:       fmt.Sprintf("Directory not found: %s", args.Path),
			Success:       &success,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	// Check if it's a directory
	info, err := t.fs.Stat(fullPath)
	if err != nil {
		success := false
		return &runtime.ToolResponse{
			Content:       fmt.Sprintf("Error reading path info: %v", err),
			Success:       &success,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	if !info.IsDir() {
		success := false
		return &runtime.ToolResponse{
			Content:       fmt.Sprintf("Path is not a directory: %s", args.Path),
			Success:       &success,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	// List directory contents
	var entries []fileEntry
	if args.Recursive {
		maxDepth := -1 // No limit by default
		if args.MaxDepth != nil {
			maxDepth = *args.MaxDepth
		}
		entries, err = t.listRecursive(fullPath, req.WorkingDirectory, 0, maxDepth, &args)
	} else {
		entries, err = t.listDir(fullPath, req.WorkingDirectory, &args)
	}

	if err != nil {
		success := false
		return &runtime.ToolResponse{
			Content:       fmt.Sprintf("Error listing directory: %v", err),
			Success:       &success,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	// Sort entries
	sort.Slice(entries, func(i, j int) bool {
		// Directories first, then alphabetically
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Path < entries[j].Path
	})

	// Format output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Directory listing for: %s\n\n", args.Path))

	if len(entries) == 0 {
		output.WriteString("(empty directory)\n")
	} else {
		for _, entry := range entries {
			if entry.IsDir {
				output.WriteString(fmt.Sprintf("  [DIR]  %s\n", entry.Path))
			} else {
				output.WriteString(fmt.Sprintf("  [FILE] %s (%s)\n", entry.Path, formatFileSize(entry.Size)))
			}
		}
		output.WriteString(fmt.Sprintf("\nTotal: %d items\n", len(entries)))
	}

	success := true
	response := &runtime.ToolResponse{
		Content:       output.String(),
		Success:       &success,
		ExecutionTime: time.Since(startTime),
	}

	// Stream output if writer is available
	if execCtx.OutputWriter != nil {
		// Best effort streaming write
		_, _ = io.WriteString(execCtx.OutputWriter, response.Content) // nolint:errcheck
		response.StreamedOutput = true
	}

	return response, nil
}

// listDir lists a single directory (non-recursive).
func (t *ListTool) listDir(fullPath, basePath string, args *listArgs) ([]fileEntry, error) {
	entries, err := afero.ReadDir(t.fs, fullPath)
	if err != nil {
		return nil, err
	}

	var result []fileEntry
	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files unless requested
		if !args.ShowHidden && isHiddenFile(name) {
			continue
		}

		// Check pattern match
		if args.Pattern != "" {
			matched, err := matchPattern(args.Pattern, name)
			if err != nil {
				return nil, fmt.Errorf("invalid pattern: %w", err)
			}
			if !matched {
				continue
			}
		}

		// Get relative path from base
		entryPath := filepath.Join(fullPath, name)
		relPath, err := filepath.Rel(basePath, entryPath)
		if err != nil {
			relPath = name
		}

		result = append(result, fileEntry{
			Path:  relPath,
			IsDir: entry.IsDir(),
			Size:  entry.Size(),
		})
	}

	return result, nil
}

// listRecursive lists directory recursively.
func (t *ListTool) listRecursive(fullPath, basePath string, depth, maxDepth int, args *listArgs) ([]fileEntry, error) {
	// Check depth limit
	if maxDepth >= 0 && depth > maxDepth {
		return nil, nil
	}

	entries, err := afero.ReadDir(t.fs, fullPath)
	if err != nil {
		return nil, err
	}

	var result []fileEntry
	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files unless requested
		if !args.ShowHidden && isHiddenFile(name) {
			continue
		}

		entryPath := filepath.Join(fullPath, name)

		// Get relative path from base
		relPath, err := filepath.Rel(basePath, entryPath)
		if err != nil {
			relPath = name
		}

		// Check pattern match
		matchesPattern := true
		if args.Pattern != "" {
			matched, err := matchPattern(args.Pattern, name)
			if err != nil {
				return nil, fmt.Errorf("invalid pattern: %w", err)
			}
			matchesPattern = matched
		}

		if matchesPattern {
			result = append(result, fileEntry{
				Path:  relPath,
				IsDir: entry.IsDir(),
				Size:  entry.Size(),
			})
		}

		// Recurse into directories
		if entry.IsDir() {
			subEntries, err := t.listRecursive(entryPath, basePath, depth+1, maxDepth, args)
			if err != nil {
				return nil, err
			}
			result = append(result, subEntries...)
		}
	}

	return result, nil
}

// ApprovalKey generates a unique key for approval caching.
func (t *ListTool) ApprovalKey(req *runtime.ToolRequest) string {
	return fmt.Sprintf("%s:%s:%s", t.Name(), req.WorkingDirectory, req.Arguments)
}

// NeedsInitialApproval returns false for list operations (they're safe).
func (t *ListTool) NeedsInitialApproval(req *runtime.ToolRequest, approvalPolicy runtime.ApprovalPolicy, sandboxPolicy runtime.SandboxPolicy) bool {
	return false // List operations don't need approval
}

// NeedsRetryApproval returns false (no retry needed).
func (t *ListTool) NeedsRetryApproval(approvalPolicy runtime.ApprovalPolicy) bool {
	return false
}

// SandboxPreference returns Auto (can run with or without sandbox).
func (t *ListTool) SandboxPreference() runtime.SandboxPreference {
	return runtime.SandboxAuto
}

// EscalateOnFailure returns false (lists don't need escalation).
func (t *ListTool) EscalateOnFailure() bool {
	return false
}

// WantsEscalatedFirstAttempt returns false.
func (t *ListTool) WantsEscalatedFirstAttempt(req *runtime.ToolRequest) bool {
	return false
}

// SupportsParallel returns true (lists can be parallelized).
func (t *ListTool) SupportsParallel() bool {
	return true
}

// SandboxRetryData returns nil (lists don't retry).
func (t *ListTool) SandboxRetryData(req *runtime.ToolRequest) *runtime.SandboxRetryData {
	return nil
}
