package file

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
	"github.com/spf13/afero"
)

// WriteTool implements the write_file tool runtime.
type WriteTool struct {
	fs afero.Fs
}

// NewWriteTool creates a new WriteTool with the given filesystem.
func NewWriteTool(fs afero.Fs) *WriteTool {
	return &WriteTool{fs: fs}
}

// writeArgs represents the arguments for write_file tool.
type writeArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Name returns the tool name.
func (t *WriteTool) Name() string {
	return "write_file"
}

// Execute writes content to a file atomically.
func (t *WriteTool) Execute(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
	startTime := time.Now()

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return nil, runtime.NewToolErrorWithCause(runtime.ErrorExecution, "context cancelled", err)
	}

	// Parse arguments
	var args writeArgs
	if err := json.Unmarshal([]byte(req.Arguments), &args); err != nil {
		return nil, runtime.NewToolErrorWithCause(runtime.ErrorInvalidArguments, "invalid arguments", err)
	}

	if args.Path == "" {
		return nil, runtime.NewToolError(runtime.ErrorInvalidArguments, "path is required")
	}

	// Validate path for write access (includes sensitive path checks)
	if err := ValidatePathForWrite(args.Path, req.WorkingDirectory); err != nil {
		return nil, runtime.NewToolErrorWithCause(runtime.ErrorInvalidArguments, err.Error(), err)
	}

	// Resolve path to absolute
	fullPath, err := ResolvePath(args.Path, req.WorkingDirectory)
	if err != nil {
		return nil, runtime.NewToolErrorWithCause(runtime.ErrorInvalidArguments, err.Error(), err)
	}

	// Check if path exists and is a directory
	if info, err := t.fs.Stat(fullPath); err == nil && info.IsDir() {
		success := false
		return &runtime.ToolResponse{
			Content:       fmt.Sprintf("Cannot write to '%s': path is a directory, not a file. Provide a file path instead", args.Path),
			Success:       &success,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	// Create parent directories if needed
	dir := filepath.Dir(fullPath)
	if err := t.fs.MkdirAll(dir, 0755); err != nil {
		success := false
		msg := fmt.Sprintf("Failed to create parent directories for '%s': %v", args.Path, err)
		if os.IsPermission(err) {
			msg += ". Check directory permissions or try running with appropriate privileges"
		}
		return &runtime.ToolResponse{
			Content:       msg,
			Success:       &success,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	// Perform atomic write using temp file + rename
	if err := t.atomicWrite(fullPath, []byte(args.Content)); err != nil {
		success := false
		msg := fmt.Sprintf("Failed to write file '%s': %v", args.Path, err)
		if os.IsPermission(err) {
			msg += ". Check file permissions or try running with appropriate privileges"
		}
		return &runtime.ToolResponse{
			Content:       msg,
			Success:       &success,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	success := true
	response := &runtime.ToolResponse{
		Content:       fmt.Sprintf("Successfully wrote %d bytes to %s", len(args.Content), args.Path),
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

// atomicWrite writes data to a file atomically by writing to a temp file
// and then renaming it to the target path.
func (t *WriteTool) atomicWrite(path string, data []byte) error {
	// Generate random suffix for temp file
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Errorf("failed to generate temp file suffix: %w", err)
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tempPath := filepath.Join(dir, fmt.Sprintf(".%s.tmp.%s", base, hex.EncodeToString(suffix)))

	// Write to temp file
	if err := afero.WriteFile(t.fs, tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Rename temp file to target (atomic on most filesystems)
	if err := t.fs.Rename(tempPath, path); err != nil {
		// Clean up temp file on failure
		_ = t.fs.Remove(tempPath) // nolint:errcheck // Best effort cleanup
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// ApprovalKey generates a unique key for approval caching.
func (t *WriteTool) ApprovalKey(req *runtime.ToolRequest) string {
	return fmt.Sprintf("%s:%s:%s", t.Name(), req.WorkingDirectory, req.Arguments)
}

// NeedsInitialApproval checks if approval is needed based on policy.
func (t *WriteTool) NeedsInitialApproval(req *runtime.ToolRequest, approvalPolicy runtime.ApprovalPolicy, sandboxPolicy runtime.SandboxPolicy) bool {
	switch approvalPolicy {
	case runtime.ApprovalNever:
		return false
	case runtime.ApprovalUnlessTrusted:
		return true // Write operations are not trusted by default
	case runtime.ApprovalOnRequest:
		// Don't require approval in danger mode
		return sandboxPolicy != runtime.SandboxDangerFullAccess
	case runtime.ApprovalOnFailure:
		return false
	default:
		return false
	}
}

// NeedsRetryApproval returns false (writes don't retry with escalation).
func (t *WriteTool) NeedsRetryApproval(approvalPolicy runtime.ApprovalPolicy) bool {
	return false
}

// SandboxPreference returns Auto (can run with or without sandbox).
func (t *WriteTool) SandboxPreference() runtime.SandboxPreference {
	return runtime.SandboxAuto
}

// EscalateOnFailure returns false (writes don't need escalation).
func (t *WriteTool) EscalateOnFailure() bool {
	return false
}

// WantsEscalatedFirstAttempt returns false.
func (t *WriteTool) WantsEscalatedFirstAttempt(req *runtime.ToolRequest) bool {
	return false
}

// SupportsParallel returns false (writes should be sequential to avoid conflicts).
func (t *WriteTool) SupportsParallel() bool {
	return false
}

// SandboxRetryData returns nil (writes don't retry).
func (t *WriteTool) SandboxRetryData(req *runtime.ToolRequest) *runtime.SandboxRetryData {
	return nil
}
