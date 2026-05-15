package image

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

// ImageTool implements the view_image tool runtime.
// It allows the AI to view local images by encoding them as base64 data URLs
// and adding them to the conversation context.
type ImageTool struct{}

// NewImageTool creates a new ImageTool instance.
func NewImageTool() *ImageTool {
	return &ImageTool{}
}

// imageArgs represents the parsed arguments for the view_image tool.
type imageArgs struct {
	// Path is the file path to the image (relative or absolute)
	Path string `json:"path"`
}

// Name returns the unique identifier for this tool.
func (t *ImageTool) Name() string {
	return "view_image"
}

// Execute loads an image from the filesystem, validates it, encodes it to base64,
// and prepares it for attachment to the conversation.
func (t *ImageTool) Execute(ctx context.Context, req *runtime.ToolRequest, execCtx *runtime.ExecutionContext) (*runtime.ToolResponse, error) {
	startTime := time.Now()

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return nil, runtime.NewToolErrorWithCause(runtime.ErrorExecution, "context cancelled", err)
	}

	// Parse arguments
	var args imageArgs
	if err := json.Unmarshal([]byte(req.Arguments), &args); err != nil {
		return nil, runtime.NewToolErrorWithCause(
			runtime.ErrorInvalidArguments,
			"failed to parse view_image arguments",
			err,
		)
	}

	if args.Path == "" {
		return nil, runtime.NewToolError(
			runtime.ErrorInvalidArguments,
			"path is required",
		)
	}

	// Resolve path (handle both absolute and relative paths)
	absPath := args.Path
	if !filepath.IsAbs(args.Path) {
		// Resolve relative to working directory
		absPath = filepath.Join(req.WorkingDirectory, args.Path)
	}

	// Clean the path to handle any .. or . components
	absPath = filepath.Clean(absPath)

	// Validate that the file exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			success := false
			return &runtime.ToolResponse{
				Content:       fmt.Sprintf("unable to locate image at `%s`: file does not exist", absPath),
				Success:       &success,
				ExecutionTime: time.Since(startTime),
			}, nil
		}
		success := false
		return &runtime.ToolResponse{
			Content:       fmt.Sprintf("unable to locate image at `%s`: %v", absPath, err),
			Success:       &success,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	// Validate that the path is a file (not a directory)
	if info.IsDir() {
		success := false
		return &runtime.ToolResponse{
			Content:       fmt.Sprintf("image path `%s` is not a file", absPath),
			Success:       &success,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	// Validate that the file format is supported
	if err := ValidateFormat(absPath); err != nil {
		success := false
		return &runtime.ToolResponse{
			Content:       fmt.Sprintf("image at `%s` has %v", absPath, err),
			Success:       &success,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	// Encode the image to a data URL
	dataURL, err := EncodeToDataURL(absPath)
	if err != nil {
		success := false
		return &runtime.ToolResponse{
			Content:       fmt.Sprintf("failed to encode image at `%s`: %v", absPath, err),
			Success:       &success,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	// Success! Store the data URL in metadata for the conversation manager to use
	success := true
	response := &runtime.ToolResponse{
		Content:       "attached local image path",
		Success:       &success,
		ExecutionTime: time.Since(startTime),
		Metadata: map[string]interface{}{
			"image_url":     dataURL,
			"image_path":    absPath,
			"original_path": args.Path,
		},
	}

	return response, nil
}

// ApprovalKey generates a unique key for caching approval decisions.
func (t *ImageTool) ApprovalKey(req *runtime.ToolRequest) string {
	return fmt.Sprintf("%s:%s:%s", t.Name(), req.WorkingDirectory, req.Arguments)
}

// NeedsInitialApproval returns false for image viewing (it's a safe read operation).
func (t *ImageTool) NeedsInitialApproval(req *runtime.ToolRequest, approvalPolicy runtime.ApprovalPolicy, sandboxPolicy runtime.SandboxPolicy) bool {
	return false // Viewing images is safe
}

// NeedsRetryApproval returns false (no retry needed for image viewing).
func (t *ImageTool) NeedsRetryApproval(approvalPolicy runtime.ApprovalPolicy) bool {
	return false
}

// SandboxPreference returns Forbid because image viewing doesn't need sandboxing
// and sandboxing may interfere with filesystem access.
func (t *ImageTool) SandboxPreference() runtime.SandboxPreference {
	return runtime.SandboxForbid
}

// EscalateOnFailure returns false (images don't need escalation).
func (t *ImageTool) EscalateOnFailure() bool {
	return false
}

// WantsEscalatedFirstAttempt returns false.
func (t *ImageTool) WantsEscalatedFirstAttempt(req *runtime.ToolRequest) bool {
	return false
}

// SupportsParallel returns true (image viewing can be parallelized).
func (t *ImageTool) SupportsParallel() bool {
	return true
}

// SandboxRetryData returns nil (image viewing doesn't retry).
func (t *ImageTool) SandboxRetryData(req *runtime.ToolRequest) *runtime.SandboxRetryData {
	return nil
}
