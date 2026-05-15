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

// commitArgs represents the parsed arguments for git_commit.
type commitArgs struct {
	// Message is the commit message (required)
	Message string `json:"message"`

	// All stages all modified files before committing (-a flag)
	All *bool `json:"all,omitempty"`

	// AllowEmpty allows creating commits with no changes
	AllowEmpty *bool `json:"allow_empty,omitempty"`

	// Amend modifies the previous commit
	Amend *bool `json:"amend,omitempty"`

	// NoVerify bypasses pre-commit and commit-msg hooks
	NoVerify *bool `json:"no_verify,omitempty"`

	// Author overrides the commit author (format: "Name <email>")
	Author string `json:"author,omitempty"`

	// Files specifies specific files to commit (optional)
	Files []string `json:"files,omitempty"`
}

// CommitTool implements git commit functionality.
type CommitTool struct {
	executor *gitExecutor
}

// NewCommitTool creates a new git commit tool.
func NewCommitTool() *CommitTool {
	return &CommitTool{
		executor: newGitExecutor(),
	}
}

// Name returns the tool name.
func (t *CommitTool) Name() string {
	return "git_commit"
}

// Execute runs git commit and returns the output.
func (t *CommitTool) Execute(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
	// Parse arguments
	args, err := t.parseArguments(req.Arguments)
	if err != nil {
		return nil, runtime.NewToolErrorWithCause(
			runtime.ErrorInvalidArguments,
			"failed to parse git_commit arguments",
			err,
		)
	}

	// Validate message
	if strings.TrimSpace(args.Message) == "" {
		return nil, runtime.NewToolError(
			runtime.ErrorInvalidArguments,
			"commit message cannot be empty",
		)
	}

	// Check if we're in a git repository
	if !t.executor.isGitRepo(ctx, req.WorkingDirectory) {
		return &runtime.ToolResponse{
			Content: "Not a git repository",
			Success: boolPtr(false),
		}, nil
	}

	// If files are specified, add them first
	if len(args.Files) > 0 {
		addArgs := append([]string{"add", "--"}, args.Files...)
		_, stderr, addErr := t.executor.executeGit(ctx, req.WorkingDirectory, addArgs...)
		if addErr != nil {
			return nil, runtime.NewToolErrorWithCause(
				runtime.ErrorExecution,
				"git add failed",
				newGitError("add", "", stderr, addErr),
			)
		}
	}

	// Build git commit command arguments
	gitArgs := []string{"commit", "-m", args.Message}

	// Add flags
	if args.All != nil && *args.All {
		gitArgs = append(gitArgs, "-a")
	}

	if args.AllowEmpty != nil && *args.AllowEmpty {
		gitArgs = append(gitArgs, "--allow-empty")
	}

	if args.Amend != nil && *args.Amend {
		gitArgs = append(gitArgs, "--amend")
	}

	if args.NoVerify != nil && *args.NoVerify {
		gitArgs = append(gitArgs, "--no-verify")
	}

	if args.Author != "" {
		gitArgs = append(gitArgs, "--author", args.Author)
	}

	// Execute git commit
	stdout, stderr, execErr := t.executor.executeGit(ctx, req.WorkingDirectory, gitArgs...)

	// Combine output
	output := stdout
	if stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += stderr
	}

	if execErr != nil {
		// Check for common errors
		if strings.Contains(stderr, "nothing to commit") || strings.Contains(stdout, "nothing to commit") {
			return &runtime.ToolResponse{
				Content: "Nothing to commit, working tree clean",
				Success: boolPtr(false),
			}, nil
		}

		return nil, runtime.NewToolErrorWithCause(
			runtime.ErrorExecution,
			"git commit failed",
			newGitError("commit", stdout, stderr, execErr),
		)
	}

	// Format output
	content := formatGitOutput(output)

	// Extract commit hash if available
	var commitHash string
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		// Look for "[branch hash] message" pattern
		if strings.Contains(line, "[") && strings.Contains(line, "]") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if strings.HasSuffix(part, "]") && i > 0 {
					commitHash = strings.TrimSuffix(parts[i], "]")
					break
				}
			}
			break
		}
	}

	success := true
	metadata := map[string]interface{}{
		"destructive": true,
	}

	if commitHash != "" {
		metadata["commit_hash"] = commitHash
	}

	return &runtime.ToolResponse{
		Content:  content,
		Success:  &success,
		Metadata: metadata,
	}, nil
}

// ApprovalKey generates approval cache key.
// For commits, we include the message in the key since each commit is unique.
func (t *CommitTool) ApprovalKey(req *runtime.ToolRequest) string {
	args, err := t.parseArguments(req.Arguments)
	if err != nil {
		return ""
	}

	key := fmt.Sprintf("git_commit:%s:%s", req.WorkingDirectory, args.Message)
	hash := sha256.Sum256([]byte(key))
	return "git_commit:" + hex.EncodeToString(hash[:8])
}

// NeedsInitialApproval determines if approval is required.
// git commit is destructive and always requires approval unless policy is Never.
func (t *CommitTool) NeedsInitialApproval(req *runtime.ToolRequest, approvalPolicy runtime.ApprovalPolicy, sandboxPolicy runtime.SandboxPolicy) bool {
	// Never policy means no approval needed
	if approvalPolicy == runtime.ApprovalNever {
		return false
	}

	// Commit is destructive and should always require approval
	return true
}

// NeedsRetryApproval determines if retry approval is required.
func (t *CommitTool) NeedsRetryApproval(approvalPolicy runtime.ApprovalPolicy) bool {
	// Commit should always get approval for retry if the policy requires it
	return approvalPolicy == runtime.ApprovalOnFailure || approvalPolicy == runtime.ApprovalUnlessTrusted
}

// SandboxPreference indicates sandboxing preference.
// Commits modify the repository, so they should prefer no sandbox.
func (t *CommitTool) SandboxPreference() runtime.SandboxPreference {
	return runtime.SandboxForbid
}

// EscalateOnFailure indicates whether to retry without sandbox on failure.
func (t *CommitTool) EscalateOnFailure() bool {
	return true
}

// WantsEscalatedFirstAttempt checks if escalation is requested.
// Commits should always run without sandbox.
func (t *CommitTool) WantsEscalatedFirstAttempt(req *runtime.ToolRequest) bool {
	return true
}

// SupportsParallel indicates if parallel execution is supported.
// Commits should not run in parallel to avoid conflicts.
func (t *CommitTool) SupportsParallel() bool {
	return false
}

// SandboxRetryData returns retry data for sandbox failures.
func (t *CommitTool) SandboxRetryData(req *runtime.ToolRequest) *runtime.SandboxRetryData {
	args, err := t.parseArguments(req.Arguments)
	if err != nil {
		return nil
	}

	cmd := []string{"git", "commit", "-m", args.Message}

	return &runtime.SandboxRetryData{
		Command:          cmd,
		WorkingDirectory: req.WorkingDirectory,
	}
}

// parseArguments parses the JSON arguments.
func (t *CommitTool) parseArguments(arguments string) (*commitArgs, error) {
	if arguments == "" {
		return nil, fmt.Errorf("arguments cannot be empty")
	}

	var args commitArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &args, nil
}

// GetApprovalContext generates context for approval requests.
// This includes showing the diff that will be committed.
func (t *CommitTool) GetApprovalContext(ctx context.Context, req *runtime.ToolRequest) (string, error) {
	args, err := t.parseArguments(req.Arguments)
	if err != nil {
		return "", err
	}

	var contextBuilder strings.Builder

	// Show commit message
	contextBuilder.WriteString("Commit Message:\n")
	contextBuilder.WriteString(args.Message)
	contextBuilder.WriteString("\n\n")

	// Show what will be committed
	contextBuilder.WriteString("Changes to be committed:\n")

	// Get diff of staged changes
	diffArgs := []string{"diff", "--cached", "--stat"}
	stdout, _, diffErr := t.executor.executeGit(ctx, req.WorkingDirectory, diffArgs...)

	if diffErr == nil && stdout != "" {
		contextBuilder.WriteString(stdout)
		contextBuilder.WriteString("\n")

		// Also get the actual diff (limited to avoid too much output)
		diffArgs = []string{"diff", "--cached", "-U3"}
		stdout, _, diffErr = t.executor.executeGit(ctx, req.WorkingDirectory, diffArgs...)
		if diffErr == nil && stdout != "" {
			// Limit diff output to first 100 lines
			lines := strings.Split(stdout, "\n")
			if len(lines) > 100 {
				lines = lines[:100]
				lines = append(lines, fmt.Sprintf("\n... (%d more lines) ...", len(strings.Split(stdout, "\n"))-100))
			}
			contextBuilder.WriteString("\nDiff:\n")
			contextBuilder.WriteString(strings.Join(lines, "\n"))
		}
	} else {
		contextBuilder.WriteString("(No staged changes)")
	}

	return contextBuilder.String(), nil
}
