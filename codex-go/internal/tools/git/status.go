package git

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// statusArgs represents the parsed arguments for git_status.
type statusArgs struct {
	// ShowUntracked controls whether untracked files are shown (default: true)
	ShowUntracked *bool `json:"show_untracked,omitempty"`

	// ShowIgnored controls whether ignored files are shown (default: false)
	ShowIgnored *bool `json:"show_ignored,omitempty"`
}

// StatusTool implements git status functionality.
type StatusTool struct {
	executor *gitExecutor
}

// NewStatusTool creates a new git status tool.
func NewStatusTool() *StatusTool {
	return &StatusTool{
		executor: newGitExecutor(),
	}
}

// Name returns the tool name.
func (t *StatusTool) Name() string {
	return "git_status"
}

// Execute runs git status and returns structured output.
func (t *StatusTool) Execute(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
	// Parse arguments
	args, err := t.parseArguments(req.Arguments)
	if err != nil {
		return nil, runtime.NewToolErrorWithCause(
			runtime.ErrorInvalidArguments,
			"failed to parse git_status arguments",
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

	// Build git status command arguments
	gitArgs := []string{"status", "--porcelain=v1", "-b"}

	showUntracked := true
	if args.ShowUntracked != nil {
		showUntracked = *args.ShowUntracked
	}

	if !showUntracked {
		gitArgs = append(gitArgs, "-uno")
	}

	if args.ShowIgnored != nil && *args.ShowIgnored {
		gitArgs = append(gitArgs, "--ignored")
	}

	// Execute git status
	stdout, stderr, execErr := t.executor.executeGit(ctx, req.WorkingDirectory, gitArgs...)

	if execErr != nil {
		return nil, runtime.NewToolErrorWithCause(
			runtime.ErrorExecution,
			"git status failed",
			newGitError("status", stdout, stderr, execErr),
		)
	}

	// Parse the output
	status := t.parseStatus(stdout)

	// Format output
	content := t.formatStatus(status)

	success := true
	return &runtime.ToolResponse{
		Content: content,
		Success: &success,
		Metadata: map[string]interface{}{
			"branch":         status.Branch,
			"ahead":          status.Ahead,
			"behind":         status.Behind,
			"staged_count":   len(status.StagedFiles),
			"unstaged_count": len(status.UnstagedFiles),
			"untracked_count": len(status.UntrackedFiles),
		},
	}, nil
}

// ApprovalKey generates approval cache key.
func (t *StatusTool) ApprovalKey(req *runtime.ToolRequest) string {
	key := fmt.Sprintf("git_status:%s", req.WorkingDirectory)
	hash := sha256.Sum256([]byte(key))
	return "git_status:" + hex.EncodeToString(hash[:8])
}

// NeedsInitialApproval determines if approval is required.
// git status is read-only and generally safe, so no approval needed.
func (t *StatusTool) NeedsInitialApproval(req *runtime.ToolRequest, approvalPolicy runtime.ApprovalPolicy, sandboxPolicy runtime.SandboxPolicy) bool {
	return false
}

// NeedsRetryApproval determines if retry approval is required.
func (t *StatusTool) NeedsRetryApproval(approvalPolicy runtime.ApprovalPolicy) bool {
	return false
}

// SandboxPreference indicates sandboxing preference.
func (t *StatusTool) SandboxPreference() runtime.SandboxPreference {
	return runtime.SandboxAuto
}

// EscalateOnFailure indicates whether to retry without sandbox on failure.
func (t *StatusTool) EscalateOnFailure() bool {
	return false
}

// WantsEscalatedFirstAttempt checks if escalation is requested.
func (t *StatusTool) WantsEscalatedFirstAttempt(req *runtime.ToolRequest) bool {
	return false
}

// SupportsParallel indicates if parallel execution is supported.
func (t *StatusTool) SupportsParallel() bool {
	return true
}

// SandboxRetryData returns retry data for sandbox failures.
func (t *StatusTool) SandboxRetryData(req *runtime.ToolRequest) *runtime.SandboxRetryData {
	return &runtime.SandboxRetryData{
		Command:          []string{"git", "status", "--porcelain=v1", "-b"},
		WorkingDirectory: req.WorkingDirectory,
	}
}

// parseArguments parses the JSON arguments.
func (t *StatusTool) parseArguments(arguments string) (*statusArgs, error) {
	if arguments == "" {
		return &statusArgs{}, nil
	}

	var args statusArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &args, nil
}

// gitStatus represents the parsed git status information.
type gitStatus struct {
	Branch         string
	Ahead          int
	Behind         int
	StagedFiles    []fileStatus
	UnstagedFiles  []fileStatus
	UntrackedFiles []string
	IgnoredFiles   []string
}

// fileStatus represents a file with its status.
type fileStatus struct {
	Path    string
	OldPath string // For renames
	Status  string
	Code    string
}

// parseStatus parses git status --porcelain=v1 output.
func (t *StatusTool) parseStatus(output string) *gitStatus {
	status := &gitStatus{
		StagedFiles:    []fileStatus{},
		UnstagedFiles:  []fileStatus{},
		UntrackedFiles: []string{},
		IgnoredFiles:   []string{},
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		// Parse branch line (## branch...upstream [ahead N, behind M])
		if strings.HasPrefix(line, "##") {
			t.parseBranchLine(line, status)
			continue
		}

		// Parse file status lines
		xy, path, oldPath := parseFileStatus(line)

		if xy == "!!" {
			// Ignored file
			status.IgnoredFiles = append(status.IgnoredFiles, path)
			continue
		}

		if xy == "??" {
			// Untracked file
			status.UntrackedFiles = append(status.UntrackedFiles, path)
			continue
		}

		// Staged changes (index)
		if xy[0] != ' ' && xy[0] != '?' {
			status.StagedFiles = append(status.StagedFiles, fileStatus{
				Path:    path,
				OldPath: oldPath,
				Status:  statusCodeDescription(string(xy[0]) + " "),
				Code:    xy,
			})
		}

		// Unstaged changes (working tree)
		if xy[1] != ' ' && xy[1] != '?' {
			status.UnstagedFiles = append(status.UnstagedFiles, fileStatus{
				Path:   path,
				Status: statusCodeDescription(" " + string(xy[1])),
				Code:   xy,
			})
		}
	}

	return status
}

// parseBranchLine parses the branch information line.
// Format: ## branch...upstream [ahead N, behind M]
func (t *StatusTool) parseBranchLine(line string, status *gitStatus) {
	// Remove ## prefix
	line = strings.TrimSpace(line[2:])

	// Extract branch name (before ...)
	parts := strings.Split(line, "...")
	if len(parts) > 0 {
		status.Branch = strings.TrimSpace(parts[0])
	}

	// Parse ahead/behind counts
	if strings.Contains(line, "[") {
		bracketStart := strings.Index(line, "[")
		bracketEnd := strings.Index(line, "]")
		if bracketStart >= 0 && bracketEnd > bracketStart {
			info := line[bracketStart+1 : bracketEnd]

			// Parse "ahead N, behind M"
			for _, part := range strings.Split(info, ",") {
				part = strings.TrimSpace(part)
				if strings.HasPrefix(part, "ahead ") {
					fmt.Sscanf(part, "ahead %d", &status.Ahead)
				} else if strings.HasPrefix(part, "behind ") {
					fmt.Sscanf(part, "behind %d", &status.Behind)
				}
			}
		}
	}
}

// formatStatus formats the git status for display.
func (t *StatusTool) formatStatus(status *gitStatus) string {
	var builder strings.Builder

	// Branch information
	if status.Branch != "" {
		builder.WriteString(fmt.Sprintf("On branch %s\n", status.Branch))

		if status.Ahead > 0 && status.Behind > 0 {
			builder.WriteString(fmt.Sprintf("Your branch and upstream have diverged (ahead %d, behind %d)\n", status.Ahead, status.Behind))
		} else if status.Ahead > 0 {
			builder.WriteString(fmt.Sprintf("Your branch is ahead of upstream by %d commit(s)\n", status.Ahead))
		} else if status.Behind > 0 {
			builder.WriteString(fmt.Sprintf("Your branch is behind upstream by %d commit(s)\n", status.Behind))
		}
	}

	// Staged changes
	if len(status.StagedFiles) > 0 {
		builder.WriteString("\nChanges to be committed:\n")
		for _, file := range status.StagedFiles {
			if file.OldPath != "" {
				builder.WriteString(fmt.Sprintf("  %s: %s -> %s\n", file.Status, file.OldPath, file.Path))
			} else {
				builder.WriteString(fmt.Sprintf("  %s: %s\n", file.Status, file.Path))
			}
		}
	}

	// Unstaged changes
	if len(status.UnstagedFiles) > 0 {
		builder.WriteString("\nChanges not staged for commit:\n")
		for _, file := range status.UnstagedFiles {
			builder.WriteString(fmt.Sprintf("  %s: %s\n", file.Status, file.Path))
		}
	}

	// Untracked files
	if len(status.UntrackedFiles) > 0 {
		builder.WriteString("\nUntracked files:\n")
		for _, file := range status.UntrackedFiles {
			builder.WriteString(fmt.Sprintf("  %s\n", file))
		}
	}

	// Ignored files
	if len(status.IgnoredFiles) > 0 {
		builder.WriteString("\nIgnored files:\n")
		for _, file := range status.IgnoredFiles {
			builder.WriteString(fmt.Sprintf("  %s\n", file))
		}
	}

	// Clean status
	if len(status.StagedFiles) == 0 && len(status.UnstagedFiles) == 0 && len(status.UntrackedFiles) == 0 {
		builder.WriteString("\nNothing to commit, working tree clean\n")
	}

	return formatGitOutput(builder.String())
}

// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool {
	return &b
}
