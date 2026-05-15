package file

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
	"github.com/spf13/afero"
)

// GrepTool implements the grep_files tool runtime.
type GrepTool struct {
	fs afero.Fs
}

// NewGrepTool creates a new GrepTool with the given filesystem.
func NewGrepTool(fs afero.Fs) *GrepTool {
	return &GrepTool{fs: fs}
}

// grepArgs represents the arguments for grep_files tool.
type grepArgs struct {
	Pattern         string `json:"pattern"`
	Path            string `json:"path"`
	FilePattern     string `json:"file_pattern,omitempty"`
	CaseInsensitive bool   `json:"case_insensitive,omitempty"`
	MaxResults      *int   `json:"max_results,omitempty"`
	ContextLines    *int   `json:"context_lines,omitempty"`
}

// grepResult represents a match result.
type grepResult struct {
	File       string
	LineNumber int
	Line       string
}

// Name returns the tool name.
func (t *GrepTool) Name() string {
	return "grep_files"
}

// Execute searches for patterns in files.
func (t *GrepTool) Execute(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
	startTime := time.Now()

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return nil, runtime.NewToolErrorWithCause(runtime.ErrorExecution, "context cancelled", err)
	}

	// Parse arguments
	var args grepArgs
	if err := json.Unmarshal([]byte(req.Arguments), &args); err != nil {
		return nil, runtime.NewToolErrorWithCause(runtime.ErrorInvalidArguments, "invalid arguments", err)
	}

	if args.Pattern == "" {
		return nil, runtime.NewToolError(runtime.ErrorInvalidArguments, "pattern is required")
	}

	if args.Path == "" {
		args.Path = "."
	}

	// Validate path
	fullPath, err := validatePath(req.WorkingDirectory, args.Path)
	if err != nil {
		return nil, runtime.NewToolErrorWithCause(runtime.ErrorInvalidArguments, err.Error(), err)
	}

	// Compile regex pattern
	var re *regexp.Regexp
	if args.CaseInsensitive {
		re, err = regexp.Compile("(?i)" + args.Pattern)
	} else {
		re, err = regexp.Compile(args.Pattern)
	}
	if err != nil {
		return nil, runtime.NewToolErrorWithCause(runtime.ErrorInvalidArguments, "invalid regex pattern", err)
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
			Content:       fmt.Sprintf("Path not found: %s", args.Path),
			Success:       &success,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	// Determine if path is a file or directory
	info, err := t.fs.Stat(fullPath)
	if err != nil {
		success := false
		return &runtime.ToolResponse{
			Content:       fmt.Sprintf("Error reading path info: %v", err),
			Success:       &success,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	// Search for matches
	var results []grepResult
	maxResults := 100 // Default max results
	if args.MaxResults != nil {
		maxResults = *args.MaxResults
	}

	if info.IsDir() {
		results, err = t.grepDirectory(fullPath, req.WorkingDirectory, re, &args, maxResults)
	} else {
		results, err = t.grepFile(fullPath, req.WorkingDirectory, re, &args, maxResults)
	}

	if err != nil {
		success := false
		return &runtime.ToolResponse{
			Content:       fmt.Sprintf("Error searching files: %v", err),
			Success:       &success,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	// Format output
	var output strings.Builder
	if len(results) == 0 {
		output.WriteString(fmt.Sprintf("No matches found for pattern: %s\n", args.Pattern))
	} else {
		output.WriteString(fmt.Sprintf("Found %d matches for pattern: %s\n\n", len(results), args.Pattern))

		currentFile := ""
		for _, result := range results {
			if result.File != currentFile {
				if currentFile != "" {
					output.WriteString("\n")
				}
				output.WriteString(fmt.Sprintf("%s:\n", result.File))
				currentFile = result.File
			}
			output.WriteString(fmt.Sprintf("  %d: %s\n", result.LineNumber, result.Line))
		}
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

// grepFile searches a single file for matches.
func (t *GrepTool) grepFile(fullPath, basePath string, re *regexp.Regexp, args *grepArgs, maxResults int) ([]grepResult, error) {
	// Read file
	data, err := afero.ReadFile(t.fs, fullPath)
	if err != nil {
		return nil, err
	}

	// Skip binary files
	if isBinaryFile(data) {
		return nil, nil
	}

	// Get relative path
	relPath, err := filepath.Rel(basePath, fullPath)
	if err != nil {
		relPath = filepath.Base(fullPath)
	}

	// Search line by line
	var results []grepResult
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	lineNumber := 1

	for scanner.Scan() && len(results) < maxResults {
		line := scanner.Text()
		if re.MatchString(line) {
			results = append(results, grepResult{
				File:       relPath,
				LineNumber: lineNumber,
				Line:       line,
			})
		}
		lineNumber++
	}

	return results, scanner.Err()
}

// grepDirectory searches all files in a directory recursively.
func (t *GrepTool) grepDirectory(fullPath, basePath string, re *regexp.Regexp, args *grepArgs, maxResults int) ([]grepResult, error) {
	var allResults []grepResult

	err := afero.Walk(t.fs, fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't read
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip hidden files
		if isHiddenFile(filepath.Base(path)) {
			return nil
		}

		// Check file pattern
		if args.FilePattern != "" {
			matched, err := matchPattern(args.FilePattern, filepath.Base(path))
			if err != nil {
				return err
			}
			if !matched {
				return nil
			}
		}

		// Search this file
		results, err := t.grepFile(path, basePath, re, args, maxResults-len(allResults))
		if err != nil {
			return nil // Skip files we can't read
		}

		allResults = append(allResults, results...)

		// Stop if we've hit max results
		if len(allResults) >= maxResults {
			return io.EOF
		}

		return nil
	})

	// EOF is not an error in this case (just means we hit max results)
	if err == io.EOF {
		err = nil
	}

	return allResults, err
}

// ApprovalKey generates a unique key for approval caching.
func (t *GrepTool) ApprovalKey(req *runtime.ToolRequest) string {
	return fmt.Sprintf("%s:%s:%s", t.Name(), req.WorkingDirectory, req.Arguments)
}

// NeedsInitialApproval returns false for grep operations (they're safe).
func (t *GrepTool) NeedsInitialApproval(req *runtime.ToolRequest, approvalPolicy runtime.ApprovalPolicy, sandboxPolicy runtime.SandboxPolicy) bool {
	return false // Grep operations don't need approval
}

// NeedsRetryApproval returns false (no retry needed).
func (t *GrepTool) NeedsRetryApproval(approvalPolicy runtime.ApprovalPolicy) bool {
	return false
}

// SandboxPreference returns Auto (can run with or without sandbox).
func (t *GrepTool) SandboxPreference() runtime.SandboxPreference {
	return runtime.SandboxAuto
}

// EscalateOnFailure returns false (grep doesn't need escalation).
func (t *GrepTool) EscalateOnFailure() bool {
	return false
}

// WantsEscalatedFirstAttempt returns false.
func (t *GrepTool) WantsEscalatedFirstAttempt(req *runtime.ToolRequest) bool {
	return false
}

// SupportsParallel returns true (greps can be parallelized).
func (t *GrepTool) SupportsParallel() bool {
	return true
}

// SandboxRetryData returns nil (grep doesn't retry).
func (t *GrepTool) SandboxRetryData(req *runtime.ToolRequest) *runtime.SandboxRetryData {
	return nil
}
