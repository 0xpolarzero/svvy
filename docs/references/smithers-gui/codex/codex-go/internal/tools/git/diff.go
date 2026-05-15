package git

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// diffArgs represents the parsed arguments for git_diff.
type diffArgs struct {
	// Staged shows only staged changes (default: false, shows unstaged)
	Staged *bool `json:"staged,omitempty"`

	// Cached is an alias for Staged
	Cached *bool `json:"cached,omitempty"`

	// Ref1 is the first reference to compare (optional, e.g., "HEAD", "main")
	Ref1 string `json:"ref1,omitempty"`

	// Ref2 is the second reference to compare (optional)
	Ref2 string `json:"ref2,omitempty"`

	// Path limits the diff to specific files or directories (optional)
	Path string `json:"path,omitempty"`

	// Unified context lines (default: 3)
	Unified *int `json:"unified,omitempty"`

	// MaxLines limits output lines (optional)
	MaxLines *int `json:"max_lines,omitempty"`
}

// DiffTool implements git diff functionality.
type DiffTool struct {
	executor *gitExecutor
}

// NewDiffTool creates a new git diff tool.
func NewDiffTool() *DiffTool {
	return &DiffTool{
		executor: newGitExecutor(),
	}
}

// Name returns the tool name.
func (t *DiffTool) Name() string {
	return "git_diff"
}

// Execute runs git diff and returns the output.
func (t *DiffTool) Execute(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
	// Parse arguments
	args, err := t.parseArguments(req.Arguments)
	if err != nil {
		return nil, runtime.NewToolErrorWithCause(
			runtime.ErrorInvalidArguments,
			"failed to parse git_diff arguments",
			err,
		)
	}

	// Check if we're in a git repository
	if !t.executor.isGitRepo(ctx, req.WorkingDirectory) {
		return &runtime.ToolResponse{
			Content: "Not a git repository",
			Success: boolPtr(false),
		}, nil
	}

	// Build git diff command arguments
	gitArgs := []string{"diff"}

	// Handle staged/cached flag
	staged := false
	if args.Staged != nil {
		staged = *args.Staged
	} else if args.Cached != nil {
		staged = *args.Cached
	}

	if staged {
		gitArgs = append(gitArgs, "--cached")
	}

	// Add unified context lines
	unified := 3
	if args.Unified != nil && *args.Unified >= 0 {
		unified = *args.Unified
	}
	gitArgs = append(gitArgs, fmt.Sprintf("-U%d", unified))

	// Add references if specified
	if args.Ref1 != "" {
		gitArgs = append(gitArgs, args.Ref1)
		if args.Ref2 != "" {
			gitArgs = append(gitArgs, args.Ref2)
		}
	}

	// Add path filter if specified
	if args.Path != "" {
		gitArgs = append(gitArgs, "--", args.Path)
	}

	// Execute git diff
	stdout, stderr, execErr := t.executor.executeGit(ctx, req.WorkingDirectory, gitArgs...)

	// Git diff returns exit code 1 when there are differences, which is not an error
	if execErr != nil {
		// Check if it's just indicating differences exist
		if exitErr, ok := execErr.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// This is normal - differences exist
		} else {
			// Actual error
			return nil, runtime.NewToolErrorWithCause(
				runtime.ErrorExecution,
				"git diff failed",
				newGitError("diff", stdout, stderr, execErr),
			)
		}
	}

	// Format output
	content := formatGitOutput(stdout)

	// Apply line limit if specified
	if args.MaxLines != nil && *args.MaxLines > 0 {
		content = t.limitLines(content, *args.MaxLines)
	}

	// Check if there are no changes
	if content == "" {
		if staged {
			content = "No staged changes"
		} else if args.Ref1 != "" {
			content = fmt.Sprintf("No differences between %s and %s", args.Ref1, args.Ref2)
		} else {
			content = "No unstaged changes"
		}
	}

	success := true
	metadata := map[string]interface{}{
		"staged": staged,
	}

	if args.Ref1 != "" {
		metadata["ref1"] = args.Ref1
	}
	if args.Ref2 != "" {
		metadata["ref2"] = args.Ref2
	}
	if args.Path != "" {
		metadata["path"] = args.Path
	}

	return &runtime.ToolResponse{
		Content:  content,
		Success:  &success,
		Metadata: metadata,
	}, nil
}

// ApprovalKey generates approval cache key.
func (t *DiffTool) ApprovalKey(req *runtime.ToolRequest) string {
	key := fmt.Sprintf("git_diff:%s", req.WorkingDirectory)
	hash := sha256.Sum256([]byte(key))
	return "git_diff:" + hex.EncodeToString(hash[:8])
}

// NeedsInitialApproval determines if approval is required.
// git diff is read-only and safe.
func (t *DiffTool) NeedsInitialApproval(req *runtime.ToolRequest, approvalPolicy runtime.ApprovalPolicy, sandboxPolicy runtime.SandboxPolicy) bool {
	return false
}

// NeedsRetryApproval determines if retry approval is required.
func (t *DiffTool) NeedsRetryApproval(approvalPolicy runtime.ApprovalPolicy) bool {
	return false
}

// SandboxPreference indicates sandboxing preference.
func (t *DiffTool) SandboxPreference() runtime.SandboxPreference {
	return runtime.SandboxAuto
}

// EscalateOnFailure indicates whether to retry without sandbox on failure.
func (t *DiffTool) EscalateOnFailure() bool {
	return false
}

// WantsEscalatedFirstAttempt checks if escalation is requested.
func (t *DiffTool) WantsEscalatedFirstAttempt(req *runtime.ToolRequest) bool {
	return false
}

// SupportsParallel indicates if parallel execution is supported.
func (t *DiffTool) SupportsParallel() bool {
	return true
}

// SandboxRetryData returns retry data for sandbox failures.
func (t *DiffTool) SandboxRetryData(req *runtime.ToolRequest) *runtime.SandboxRetryData {
	return &runtime.SandboxRetryData{
		Command:          []string{"git", "diff"},
		WorkingDirectory: req.WorkingDirectory,
	}
}

// parseArguments parses the JSON arguments.
func (t *DiffTool) parseArguments(arguments string) (*diffArgs, error) {
	if arguments == "" {
		return &diffArgs{}, nil
	}

	var args diffArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &args, nil
}

// limitLines limits the output to the specified number of lines.
func (t *DiffTool) limitLines(content string, maxLines int) string {
	lines := splitLines(content)
	if len(lines) <= maxLines {
		return content
	}

	// Keep first maxLines lines and add truncation notice
	truncated := lines[:maxLines]
	remaining := len(lines) - maxLines
	truncated = append(truncated, fmt.Sprintf("\n... (%d more lines truncated) ...", remaining))

	return joinLines(truncated)
}

// splitLines splits content into lines.
func splitLines(content string) []string {
	if content == "" {
		return []string{}
	}
	return strings.Split(content, "\n")
}

// joinLines joins lines into content.
func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}
