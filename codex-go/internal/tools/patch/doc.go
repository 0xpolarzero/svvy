// Package patch provides the apply_patch tool runtime.
//
// The apply_patch tool enables the AI to make precise file modifications using
// unified diff format. It supports:
//   - Atomic multi-file patches
//   - Pre-execution diff preview
//   - Approval workflows with change visualization
//   - Rollback on partial failures
//   - Both freeform and function-call formats
//
// This tool requires approval as it modifies files, and typically runs without
// sandbox restrictions since it needs direct filesystem access.
//
// Implementation pending - this is a stub for the Go rewrite.
package patch
