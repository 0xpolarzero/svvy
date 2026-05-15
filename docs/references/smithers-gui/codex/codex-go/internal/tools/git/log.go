package git

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// logArgs represents the parsed arguments for git_log.
type logArgs struct {
	// MaxCount limits the number of commits (default: 10)
	MaxCount *int `json:"max_count,omitempty"`

	// N is an alias for MaxCount
	N *int `json:"n,omitempty"`

	// Since shows commits more recent than a specific date (e.g., "2 weeks ago")
	Since string `json:"since,omitempty"`

	// Until shows commits older than a specific date
	Until string `json:"until,omitempty"`

	// Author filters commits by author name/email
	Author string `json:"author,omitempty"`

	// Grep searches commit messages
	Grep string `json:"grep,omitempty"`

	// Path limits log to specific files or directories
	Path string `json:"path,omitempty"`

	// OneLine shows each commit on a single line
	OneLine *bool `json:"oneline,omitempty"`

	// ShowStat shows file statistics
	ShowStat *bool `json:"stat,omitempty"`

	// Follow follows file renames (only for single file path)
	Follow *bool `json:"follow,omitempty"`
}

// LogTool implements git log functionality.
type LogTool struct {
	executor *gitExecutor
}

// NewLogTool creates a new git log tool.
func NewLogTool() *LogTool {
	return &LogTool{
		executor: newGitExecutor(),
	}
}

// Name returns the tool name.
func (t *LogTool) Name() string {
	return "git_log"
}

// Execute runs git log and returns the output.
func (t *LogTool) Execute(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
	// Parse arguments
	args, err := t.parseArguments(req.Arguments)
	if err != nil {
		return nil, runtime.NewToolErrorWithCause(
			runtime.ErrorInvalidArguments,
			"failed to parse git_log arguments",
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

	// Build git log command arguments
	gitArgs := []string{"log"}

	// Determine max count
	maxCount := 10
	if args.MaxCount != nil {
		maxCount = *args.MaxCount
	} else if args.N != nil {
		maxCount = *args.N
	}
	if maxCount > 0 {
		gitArgs = append(gitArgs, fmt.Sprintf("-n%d", maxCount))
	}

	// Add date filters
	if args.Since != "" {
		gitArgs = append(gitArgs, fmt.Sprintf("--since=%s", args.Since))
	}
	if args.Until != "" {
		gitArgs = append(gitArgs, fmt.Sprintf("--until=%s", args.Until))
	}

	// Add author filter
	if args.Author != "" {
		gitArgs = append(gitArgs, fmt.Sprintf("--author=%s", args.Author))
	}

	// Add message grep
	if args.Grep != "" {
		gitArgs = append(gitArgs, fmt.Sprintf("--grep=%s", args.Grep))
	}

	// Format options
	if args.OneLine != nil && *args.OneLine {
		gitArgs = append(gitArgs, "--oneline")
	} else {
		// Use custom format for better structured output
		gitArgs = append(gitArgs, "--pretty=format:commit %H%nAuthor: %an <%ae>%nDate:   %ad%n%n    %s%n%b")
		gitArgs = append(gitArgs, "--date=relative")
	}

	// Add stat option
	if args.ShowStat != nil && *args.ShowStat {
		gitArgs = append(gitArgs, "--stat")
	}

	// Add follow option (only valid for single file)
	if args.Follow != nil && *args.Follow && args.Path != "" {
		gitArgs = append(gitArgs, "--follow")
	}

	// Add path filter if specified
	if args.Path != "" {
		gitArgs = append(gitArgs, "--", args.Path)
	}

	// Execute git log
	stdout, stderr, execErr := t.executor.executeGit(ctx, req.WorkingDirectory, gitArgs...)

	if execErr != nil {
		return nil, runtime.NewToolErrorWithCause(
			runtime.ErrorExecution,
			"git log failed",
			newGitError("log", stdout, stderr, execErr),
		)
	}

	// Format output
	content := formatGitOutput(stdout)

	if content == "" {
		content = "No commits found"
	}

	success := true
	metadata := map[string]interface{}{
		"max_count": maxCount,
	}

	if args.Path != "" {
		metadata["path"] = args.Path
	}
	if args.Author != "" {
		metadata["author"] = args.Author
	}
	if args.Grep != "" {
		metadata["grep"] = args.Grep
	}

	return &runtime.ToolResponse{
		Content:  content,
		Success:  &success,
		Metadata: metadata,
	}, nil
}

// ApprovalKey generates approval cache key.
func (t *LogTool) ApprovalKey(req *runtime.ToolRequest) string {
	key := fmt.Sprintf("git_log:%s", req.WorkingDirectory)
	hash := sha256.Sum256([]byte(key))
	return "git_log:" + hex.EncodeToString(hash[:8])
}

// NeedsInitialApproval determines if approval is required.
// git log is read-only and safe.
func (t *LogTool) NeedsInitialApproval(req *runtime.ToolRequest, approvalPolicy runtime.ApprovalPolicy, sandboxPolicy runtime.SandboxPolicy) bool {
	return false
}

// NeedsRetryApproval determines if retry approval is required.
func (t *LogTool) NeedsRetryApproval(approvalPolicy runtime.ApprovalPolicy) bool {
	return false
}

// SandboxPreference indicates sandboxing preference.
func (t *LogTool) SandboxPreference() runtime.SandboxPreference {
	return runtime.SandboxAuto
}

// EscalateOnFailure indicates whether to retry without sandbox on failure.
func (t *LogTool) EscalateOnFailure() bool {
	return false
}

// WantsEscalatedFirstAttempt checks if escalation is requested.
func (t *LogTool) WantsEscalatedFirstAttempt(req *runtime.ToolRequest) bool {
	return false
}

// SupportsParallel indicates if parallel execution is supported.
func (t *LogTool) SupportsParallel() bool {
	return true
}

// SandboxRetryData returns retry data for sandbox failures.
func (t *LogTool) SandboxRetryData(req *runtime.ToolRequest) *runtime.SandboxRetryData {
	return &runtime.SandboxRetryData{
		Command:          []string{"git", "log"},
		WorkingDirectory: req.WorkingDirectory,
	}
}

// parseArguments parses the JSON arguments.
func (t *LogTool) parseArguments(arguments string) (*logArgs, error) {
	if arguments == "" {
		return &logArgs{}, nil
	}

	var args logArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &args, nil
}
