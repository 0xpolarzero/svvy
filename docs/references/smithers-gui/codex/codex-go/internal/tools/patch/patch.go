package patch

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
	"github.com/spf13/afero"
)

// PatchTool implements the apply_patch tool runtime.
type PatchTool struct {
	fs afero.Fs
}

// NewPatchTool creates a new patch tool instance.
func NewPatchTool(fs afero.Fs) *PatchTool {
	return &PatchTool{
		fs: fs,
	}
}

// PatchArgs represents the arguments for the apply_patch tool.
type PatchArgs struct {
	Patch            string `json:"patch"`
	Approve          bool   `json:"approve,omitempty"`
	DryRun           bool   `json:"dry_run,omitempty"`
	Root             string `json:"root,omitempty"`
	AllowOutsideRoot bool   `json:"allow_outside_root,omitempty"`
}

// Name returns the tool name.
func (t *PatchTool) Name() string {
	return "apply_patch"
}

// Execute runs the patch tool.
func (t *PatchTool) Execute(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, runtime.NewToolError(runtime.ErrorTimeout, "context cancelled")
	default:
	}

	// Parse arguments
	var args PatchArgs
	if err := json.Unmarshal([]byte(req.Arguments), &args); err != nil {
		return nil, runtime.NewToolErrorWithCause(runtime.ErrorInvalidArguments, "failed to parse arguments", err)
	}

	// Validate patch argument
	if args.Patch == "" {
		return nil, runtime.NewToolError(runtime.ErrorInvalidArguments, "patch argument is required")
	}

	// Determine root directory
	root := args.Root
	if root == "" {
		root = req.WorkingDirectory
	}

	// Parse the unified diff
	patches, err := parseUnifiedDiff(args.Patch)
	if err != nil {
		success := false
		return &runtime.ToolResponse{
			Content: fmt.Sprintf("Failed to parse patch: %v", err),
			Success: &success,
		}, runtime.NewToolErrorWithCause(runtime.ErrorInvalidArguments, "invalid patch format", err)
	}

	// Apply patches
	result, err := applyPatchesWithOptions(t.fs, patches, root, args.DryRun, args.AllowOutsideRoot)

	// Build response
	success := err == nil
	content := formatResult(result)

	resp := &runtime.ToolResponse{
		Content: content,
		Success: &success,
	}

	if err != nil {
		return resp, runtime.NewToolErrorWithCause(runtime.ErrorExecution, "failed to apply patches", err)
	}

	return resp, nil
}

// ApprovalKey generates a unique key for approval caching.
func (t *PatchTool) ApprovalKey(req *runtime.ToolRequest) string {
	// Use tool name and working directory as key
	// We could include patch hash for more granularity
	return fmt.Sprintf("%s:%s", t.Name(), req.WorkingDirectory)
}

// NeedsInitialApproval determines if approval is required before execution.
func (t *PatchTool) NeedsInitialApproval(req *runtime.ToolRequest, approvalPolicy runtime.ApprovalPolicy, sandboxPolicy runtime.SandboxPolicy) bool {
	switch approvalPolicy {
	case runtime.ApprovalNever:
		return false
	case runtime.ApprovalOnFailure:
		return false
	case runtime.ApprovalOnRequest:
		// Patch modifies files, so require approval unless in danger mode
		return sandboxPolicy != runtime.SandboxDangerFullAccess
	case runtime.ApprovalUnlessTrusted:
		// Always require approval for file modifications
		return true
	default:
		return true
	}
}

// NeedsRetryApproval determines if approval is required for retry without sandbox.
func (t *PatchTool) NeedsRetryApproval(approvalPolicy runtime.ApprovalPolicy) bool {
	return approvalPolicy == runtime.ApprovalOnFailure || approvalPolicy == runtime.ApprovalUnlessTrusted
}

// SandboxPreference indicates how this tool interacts with sandboxing.
func (t *PatchTool) SandboxPreference() runtime.SandboxPreference {
	// Patch tool needs direct filesystem access, forbid sandbox
	return runtime.SandboxForbid
}

// EscalateOnFailure returns whether to retry without sandbox on failure.
func (t *PatchTool) EscalateOnFailure() bool {
	return false // Already forbids sandbox
}

// WantsEscalatedFirstAttempt checks if request asks for escalated permissions.
func (t *PatchTool) WantsEscalatedFirstAttempt(req *runtime.ToolRequest) bool {
	return false // Not applicable for this tool
}

// SupportsParallel returns whether multiple instances can run concurrently.
func (t *PatchTool) SupportsParallel() bool {
	// Patch operations should be sequential to avoid conflicts
	return false
}

// SandboxRetryData extracts data needed for sandbox retry.
func (t *PatchTool) SandboxRetryData(req *runtime.ToolRequest) *runtime.SandboxRetryData {
	// Not applicable for file operations
	return nil
}

// formatResult formats the ApplyResult into a human-readable string.
func formatResult(result *ApplyResult) string {
	if result == nil {
		return "No result available"
	}

	output := ""

	if result.DryRun {
		output += "DRY RUN MODE - No files were actually modified\n\n"
	}

	if len(result.Added) > 0 {
		output += fmt.Sprintf("Files Added (%d):\n", len(result.Added))
		for _, file := range result.Added {
			output += fmt.Sprintf("  + %s\n", file)
		}
		output += "\n"
	}

	if len(result.Updated) > 0 {
		output += fmt.Sprintf("Files Updated (%d):\n", len(result.Updated))
		for _, file := range result.Updated {
			output += fmt.Sprintf("  ~ %s\n", file)
		}
		output += "\n"
	}

	if len(result.Deleted) > 0 {
		output += fmt.Sprintf("Files Deleted (%d):\n", len(result.Deleted))
		for _, file := range result.Deleted {
			output += fmt.Sprintf("  - %s\n", file)
		}
		output += "\n"
	}

	if len(result.Errors) > 0 {
		output += fmt.Sprintf("Errors (%d):\n", len(result.Errors))
		for _, err := range result.Errors {
			output += fmt.Sprintf("  ! %s\n", err)
		}
		output += "\n"
		output += "All changes have been rolled back due to errors.\n\n"
	}

	if result.Summary != "" {
		output += "Summary: " + result.Summary + "\n"
	}

	if len(result.FilesAffected) > 0 && len(result.Errors) == 0 {
		if result.DryRun {
			output += fmt.Sprintf("\nWould affect %d file(s). Run without dry_run to apply changes.\n", len(result.FilesAffected))
		} else {
			output += fmt.Sprintf("\nSuccessfully applied changes to %d file(s).\n", len(result.FilesAffected))
		}
	}

	return output
}
