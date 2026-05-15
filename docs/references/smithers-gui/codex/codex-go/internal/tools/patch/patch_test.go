package patch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
	"github.com/evmts/codex/codex-go/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Parser Tests - Unified Diff Format
// ============================================================================

func TestParseUnifiedDiff_AddFile(t *testing.T) {
	diff := `--- /dev/null
+++ b/newfile.txt
@@ -0,0 +1,3 @@
+line 1
+line 2
+line 3
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)
	require.Len(t, patches, 1)

	patch := patches[0]
	assert.Equal(t, "", patch.OriginalFile)
	assert.Equal(t, "newfile.txt", patch.NewFile)
	assert.Equal(t, OperationAdd, patch.Operation)
	require.Len(t, patch.Hunks, 1)

	hunk := patch.Hunks[0]
	assert.Equal(t, 0, hunk.OriginalStart)
	assert.Equal(t, 0, hunk.OriginalLines)
	assert.Equal(t, 1, hunk.NewStart)
	assert.Equal(t, 3, hunk.NewLines)
	assert.Len(t, hunk.Lines, 3)
	assert.Equal(t, "line 1", hunk.Lines[0].Content)
}

func TestParseUnifiedDiff_DeleteFile(t *testing.T) {
	diff := `--- a/oldfile.txt
+++ /dev/null
@@ -1,3 +0,0 @@
-line 1
-line 2
-line 3
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)
	require.Len(t, patches, 1)

	patch := patches[0]
	assert.Equal(t, "oldfile.txt", patch.OriginalFile)
	assert.Equal(t, "", patch.NewFile)
	assert.Equal(t, OperationDelete, patch.Operation)
}

func TestParseUnifiedDiff_UpdateFile(t *testing.T) {
	diff := `--- a/file.txt
+++ b/file.txt
@@ -1,5 +1,5 @@
 line 1
 line 2
-old line 3
+new line 3
 line 4
 line 5
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)
	require.Len(t, patches, 1)

	patch := patches[0]
	assert.Equal(t, "file.txt", patch.OriginalFile)
	assert.Equal(t, "file.txt", patch.NewFile)
	assert.Equal(t, OperationUpdate, patch.Operation)
	require.Len(t, patch.Hunks, 1)

	hunk := patch.Hunks[0]
	assert.Equal(t, 1, hunk.OriginalStart)
	assert.Equal(t, 5, hunk.OriginalLines)
	assert.Equal(t, 1, hunk.NewStart)
	assert.Equal(t, 5, hunk.NewLines)
}

func TestParseUnifiedDiff_MoveFile(t *testing.T) {
	diff := `--- a/old/path/file.txt
+++ b/new/path/file.txt
@@ -1,3 +1,3 @@
 line 1
 line 2
 line 3
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)
	require.Len(t, patches, 1)

	patch := patches[0]
	assert.Equal(t, "old/path/file.txt", patch.OriginalFile)
	assert.Equal(t, "new/path/file.txt", patch.NewFile)
	assert.Equal(t, OperationMove, patch.Operation)
}

func TestParseUnifiedDiff_MultipleFiles(t *testing.T) {
	diff := `--- a/file1.txt
+++ b/file1.txt
@@ -1,2 +1,2 @@
-old content
+new content
--- a/file2.txt
+++ b/file2.txt
@@ -1,2 +1,2 @@
-old content 2
+new content 2
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)
	require.Len(t, patches, 2)

	assert.Equal(t, "file1.txt", patches[0].OriginalFile)
	assert.Equal(t, "file2.txt", patches[1].OriginalFile)
}

func TestParseUnifiedDiff_MultipleHunks(t *testing.T) {
	diff := `--- a/file.txt
+++ b/file.txt
@@ -1,3 +1,3 @@
 line 1
-old 2
+new 2
 line 3
@@ -10,3 +10,3 @@
 line 10
-old 11
+new 11
 line 12
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)
	require.Len(t, patches, 1)
	require.Len(t, patches[0].Hunks, 2)

	assert.Equal(t, 1, patches[0].Hunks[0].OriginalStart)
	assert.Equal(t, 10, patches[0].Hunks[1].OriginalStart)
}

func TestParseUnifiedDiff_InvalidFormat(t *testing.T) {
	tests := []struct {
		name string
		diff string
	}{
		{
			name: "missing file headers",
			diff: `@@ -1,1 +1,1 @@
-old
+new
`,
		},
		{
			name: "invalid hunk header",
			diff: `--- a/file.txt
+++ b/file.txt
@@ invalid @@
-old
+new
`,
		},
		{
			name: "empty diff",
			diff: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseUnifiedDiff(tt.diff)
			require.Error(t, err)
		})
	}
}

func TestParseUnifiedDiff_ContextLines(t *testing.T) {
	diff := `--- a/file.txt
+++ b/file.txt
@@ -5,7 +5,7 @@
 context before 1
 context before 2
 context before 3
-old line
+new line
 context after 1
 context after 2
 context after 3
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)
	require.Len(t, patches, 1)

	hunk := patches[0].Hunks[0]
	assert.Len(t, hunk.Lines, 8)

	// Check line types
	assert.Equal(t, LineContext, hunk.Lines[0].Type)
	assert.Equal(t, LineContext, hunk.Lines[1].Type)
	assert.Equal(t, LineContext, hunk.Lines[2].Type)
	assert.Equal(t, LineRemove, hunk.Lines[3].Type)
	assert.Equal(t, LineAdd, hunk.Lines[4].Type)
	assert.Equal(t, LineContext, hunk.Lines[5].Type)
	assert.Equal(t, LineContext, hunk.Lines[6].Type)
	assert.Equal(t, LineContext, hunk.Lines[7].Type)
}

// ============================================================================
// Apply Logic Tests - Atomicity and Rollback
// ============================================================================

func TestApplyPatch_AddNewFile(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/.keep", []byte(""))

	diff := `--- /dev/null
+++ b/newfile.txt
@@ -0,0 +1,2 @@
+Hello
+World
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyPatches(fs, patches, "/workspace", false)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Len(t, result.Added, 1)
	assert.Contains(t, result.Added, "newfile.txt")
	assert.Empty(t, result.Errors)

	// Verify file was created
	content := test.ReadFileFS(t, fs, "/workspace/newfile.txt")
	assert.Equal(t, "Hello\nWorld\n", string(content))
}

func TestApplyPatch_DeleteFile(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/oldfile.txt", []byte("content\n"))

	diff := `--- a/oldfile.txt
+++ /dev/null
@@ -1 +0,0 @@
-content
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyPatches(fs, patches, "/workspace", false)
	require.NoError(t, err)

	assert.Len(t, result.Deleted, 1)
	assert.Contains(t, result.Deleted, "oldfile.txt")
	assert.Empty(t, result.Errors)

	// Verify file was deleted
	test.AssertFileNotExistsFS(t, fs, "/workspace/oldfile.txt")
}

func TestApplyPatch_UpdateFile(t *testing.T) {
	fs := test.NewMemFS(t)
	original := `line 1
line 2
old line 3
line 4
line 5
`
	test.WriteFileFS(t, fs, "/workspace/file.txt", []byte(original))

	diff := `--- a/file.txt
+++ b/file.txt
@@ -1,5 +1,5 @@
 line 1
 line 2
-old line 3
+new line 3
 line 4
 line 5
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyPatches(fs, patches, "/workspace", false)
	require.NoError(t, err)

	assert.Len(t, result.Updated, 1)
	assert.Contains(t, result.Updated, "file.txt")
	assert.Empty(t, result.Errors)

	// Verify file was updated
	content := test.ReadFileFS(t, fs, "/workspace/file.txt")
	expected := `line 1
line 2
new line 3
line 4
line 5
`
	assert.Equal(t, expected, string(content))
}

func TestApplyPatch_MoveFile(t *testing.T) {
	fs := test.NewMemFS(t)
	content := "file content\n"
	test.WriteFileFS(t, fs, "/workspace/old/path/file.txt", []byte(content))

	diff := `--- a/old/path/file.txt
+++ b/new/path/file.txt
@@ -1 +1 @@
 file content
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyPatches(fs, patches, "/workspace", false)
	require.NoError(t, err)

	assert.Len(t, result.Updated, 1)
	assert.Empty(t, result.Errors)

	// Verify old file removed and new file created
	test.AssertFileNotExistsFS(t, fs, "/workspace/old/path/file.txt")
	test.AssertFileExistsFS(t, fs, "/workspace/new/path/file.txt")

	newContent := test.ReadFileFS(t, fs, "/workspace/new/path/file.txt")
	assert.Equal(t, content, string(newContent))
}

func TestApplyPatch_MultipleFiles_Atomic(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/file1.txt", []byte("old 1\n"))
	test.WriteFileFS(t, fs, "/workspace/file2.txt", []byte("old 2\n"))

	diff := `--- a/file1.txt
+++ b/file1.txt
@@ -1 +1 @@
-old 1
+new 1
--- a/file2.txt
+++ b/file2.txt
@@ -1 +1 @@
-old 2
+new 2
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyPatches(fs, patches, "/workspace", false)
	require.NoError(t, err)

	assert.Len(t, result.Updated, 2)
	assert.Contains(t, result.Updated, "file1.txt")
	assert.Contains(t, result.Updated, "file2.txt")
	assert.Empty(t, result.Errors)

	// Verify both files updated
	content1 := test.ReadFileFS(t, fs, "/workspace/file1.txt")
	assert.Equal(t, "new 1\n", string(content1))

	content2 := test.ReadFileFS(t, fs, "/workspace/file2.txt")
	assert.Equal(t, "new 2\n", string(content2))
}

func TestApplyPatch_PartialFailure_Rollback(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/file1.txt", []byte("content 1\n"))
	// file2.txt does not exist - this will cause failure

	diff := `--- a/file1.txt
+++ b/file1.txt
@@ -1 +1 @@
-content 1
+modified 1
--- a/file2.txt
+++ b/file2.txt
@@ -1 +1 @@
-content 2
+modified 2
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyPatches(fs, patches, "/workspace", false)
	require.Error(t, err)
	require.NotNil(t, result)

	// Should have rolled back - file1 should remain unchanged
	content1 := test.ReadFileFS(t, fs, "/workspace/file1.txt")
	assert.Equal(t, "content 1\n", string(content1))

	// Error should be recorded
	assert.NotEmpty(t, result.Errors)
}

func TestApplyPatch_HunkMismatch_Rollback(t *testing.T) {
	fs := test.NewMemFS(t)
	// File has different content than patch expects
	test.WriteFileFS(t, fs, "/workspace/file.txt", []byte("different\ncontent\n"))

	diff := `--- a/file.txt
+++ b/file.txt
@@ -1,2 +1,2 @@
-expected line 1
-expected line 2
+new line 1
+new line 2
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyPatches(fs, patches, "/workspace", false)
	require.Error(t, err)
	require.NotNil(t, result)

	// File should remain unchanged
	content := test.ReadFileFS(t, fs, "/workspace/file.txt")
	assert.Equal(t, "different\ncontent\n", string(content))

	// Should record conflict error
	assert.NotEmpty(t, result.Errors)
	assert.Contains(t, strings.Join(result.Errors, " "), "conflict")
}

func TestApplyPatch_MoveFile_Rollback(t *testing.T) {
	fs := test.NewMemFS(t)
	originalContent := "original content\n"
	test.WriteFileFS(t, fs, "/workspace/source.txt", []byte(originalContent))
	test.WriteFileFS(t, fs, "/workspace/other.txt", []byte("other file\n"))

	// Patch that moves source.txt to dest.txt, then tries to update other.txt with invalid content
	// This will cause rollback to restore source.txt and remove dest.txt
	diff := `--- a/source.txt
+++ b/dest.txt
@@ -1 +1 @@
 original content
--- a/other.txt
+++ b/other.txt
@@ -1 +1 @@
-other file
+wrong content
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	// Modify other.txt content to cause a conflict
	test.WriteFileFS(t, fs, "/workspace/other.txt", []byte("different content\n"))

	result, err := applyPatches(fs, patches, "/workspace", false)
	require.Error(t, err)
	require.NotNil(t, result)

	// Should have rolled back - source file should be restored
	test.AssertFileExistsFS(t, fs, "/workspace/source.txt")
	content := test.ReadFileFS(t, fs, "/workspace/source.txt")
	assert.Equal(t, originalContent, string(content))

	// Destination file should be removed (rollback)
	test.AssertFileNotExistsFS(t, fs, "/workspace/dest.txt")
}

func TestApplyPatch_MoveFile_RollbackDestinationAlreadyRemoved(t *testing.T) {
	fs := test.NewMemFS(t)
	originalContent := "original content\n"
	test.WriteFileFS(t, fs, "/workspace/source.txt", []byte(originalContent))

	// Create a backup manually to simulate a scenario where destination doesn't exist
	backups := []BackupState{
		{
			Path:      "/workspace/source.txt",
			Content:   []byte(originalContent),
			Existed:   true,
			Operation: "move",
			DestPath:  "/workspace/dest.txt",
		},
	}

	// Rollback should handle case where destination doesn't exist gracefully
	err := rollbackChanges(fs, backups)
	require.NoError(t, err)

	// Source file should be restored
	test.AssertFileExistsFS(t, fs, "/workspace/source.txt")
	content := test.ReadFileFS(t, fs, "/workspace/source.txt")
	assert.Equal(t, originalContent, string(content))
}

func TestApplyPatch_DryRun(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/file.txt", []byte("old content\n"))

	diff := `--- a/file.txt
+++ b/file.txt
@@ -1 +1 @@
-old content
+new content
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyPatches(fs, patches, "/workspace", true)
	require.NoError(t, err)

	assert.Len(t, result.Updated, 1)
	assert.Contains(t, result.Updated, "file.txt")
	assert.Empty(t, result.Errors)

	// In dry-run, file should NOT be modified
	content := test.ReadFileFS(t, fs, "/workspace/file.txt")
	assert.Equal(t, "old content\n", string(content))
}

// ============================================================================
// Security Tests - Path Traversal and Sandboxing
// ============================================================================

func TestApplyPatch_PathTraversal_Blocked(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/.keep", []byte(""))

	tests := []struct {
		name string
		diff string
	}{
		{
			name: "parent directory traversal",
			diff: `--- /dev/null
+++ b/../etc/passwd
@@ -0,0 +1 @@
+malicious
`,
		},
		{
			name: "absolute path outside root",
			diff: `--- /dev/null
+++ /etc/passwd
@@ -0,0 +1 @@
+malicious
`,
		},
		{
			name: "hidden parent traversal",
			diff: `--- /dev/null
+++ b/safe/../../etc/passwd
@@ -0,0 +1 @@
+malicious
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patches, parseErr := parseUnifiedDiff(tt.diff)

			// Absolute paths should fail at parse time (defense in depth)
			// Relative paths with .. will fail at apply time (depends on AllowOutsideRoot)
			if strings.Contains(tt.diff, "+++ /") || strings.Contains(tt.diff, "--- /") && !strings.Contains(tt.diff, "/dev/null") {
				// Absolute path - should fail at parse time
				require.Error(t, parseErr)
				errMsg := strings.ToLower(parseErr.Error())
				assert.True(t, strings.Contains(errMsg, "absolute paths"),
					"error should indicate absolute path issue: %s", parseErr.Error())
				return
			}

			// Relative path traversal - should fail at apply time
			require.NoError(t, parseErr)

			result, err := applyPatches(fs, patches, "/workspace", false)
			require.Error(t, err)
			require.NotNil(t, result)

			// Error should indicate path traversal or security issue
			errMsg := strings.ToLower(err.Error())
			assert.True(t, strings.Contains(errMsg, "outside root") ||
				strings.Contains(errMsg, "path traversal"),
				"error should indicate path security issue: %s", err.Error())
		})
	}
}

func TestApplyPatch_AllowOutsideRoot_WithFlag(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/.keep", []byte(""))

	diff := `--- /dev/null
+++ b/../sibling/file.txt
@@ -0,0 +1 @@
+content
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	// Should fail by default
	_, err = applyPatches(fs, patches, "/workspace", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside root")

	// Should succeed with allowOutsideRoot flag
	result, err := applyPatchesWithOptions(fs, patches, "/workspace", false, true)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Len(t, result.Added, 1)
	test.AssertFileExistsFS(t, fs, "/sibling/file.txt")
}

func TestApplyPatch_SymlinkAttack_Blocked(t *testing.T) {
	fs := test.NewMemFS(t)

	// Try to create a file that would follow a symlink
	// Note: afero MemMapFs doesn't support symlinks, but we test the path validation
	diff := `--- /dev/null
+++ b/link/../../../etc/passwd
@@ -0,0 +1 @@
+malicious
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	result, err := applyPatches(fs, patches, "/workspace", false)
	require.Error(t, err)
	require.NotNil(t, result)

	assert.Contains(t, err.Error(), "outside root")
}

// ============================================================================
// Tool Runtime Tests
// ============================================================================

func TestPatchTool_Name(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewPatchTool(fs)
	assert.Equal(t, "apply_patch", tool.Name())
}

func TestPatchTool_Execute_Success(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/file.txt", []byte("old content\n"))

	tool := NewPatchTool(fs)

	diff := `--- a/file.txt
+++ b/file.txt
@@ -1 +1 @@
-old content
+new content
`

	args := map[string]interface{}{
		"patch": diff,
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "apply_patch",
		Arguments:        string(argsJSON),
		WorkingDirectory: "/workspace",
	}

	ctx := test.LongContext(t)
	execCtx := &runtime.ExecutionContext{
		SessionID: "session_123",
		TurnID:    "turn_123",
		StartTime: time.Now(),
	}

	resp, err := tool.Execute(ctx, req, execCtx)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, *resp.Success)

	// Verify file was updated
	content := test.ReadFileFS(t, fs, "/workspace/file.txt")
	assert.Equal(t, "new content\n", string(content))

	// Check response content has summary
	assert.Contains(t, resp.Content, "file.txt")
	assert.Contains(t, strings.ToLower(resp.Content), "updated")
}

func TestPatchTool_Execute_DryRun(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/file.txt", []byte("old content\n"))

	tool := NewPatchTool(fs)

	diff := `--- a/file.txt
+++ b/file.txt
@@ -1 +1 @@
-old content
+new content
`

	args := map[string]interface{}{
		"patch":   diff,
		"dry_run": true,
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "apply_patch",
		Arguments:        string(argsJSON),
		WorkingDirectory: "/workspace",
	}

	ctx := test.LongContext(t)
	execCtx := &runtime.ExecutionContext{
		SessionID: "session_123",
		TurnID:    "turn_123",
		StartTime: time.Now(),
	}

	resp, err := tool.Execute(ctx, req, execCtx)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, *resp.Success)

	// In dry-run mode, file should NOT be modified
	content := test.ReadFileFS(t, fs, "/workspace/file.txt")
	assert.Equal(t, "old content\n", string(content))

	// Response should indicate dry-run
	assert.Contains(t, resp.Content, "dry")
}

func TestPatchTool_Execute_InvalidPatch(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewPatchTool(fs)

	args := map[string]interface{}{
		"patch": "invalid patch format",
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "apply_patch",
		Arguments:        string(argsJSON),
		WorkingDirectory: "/workspace",
	}

	ctx := test.LongContext(t)
	execCtx := &runtime.ExecutionContext{
		SessionID: "session_123",
		TurnID:    "turn_123",
		StartTime: time.Now(),
	}

	_, err := tool.Execute(ctx, req, execCtx)
	require.Error(t, err)

	var toolErr *runtime.ToolError
	require.ErrorAs(t, err, &toolErr)
	assert.Equal(t, runtime.ErrorInvalidArguments, toolErr.Kind)
}

func TestPatchTool_Execute_CustomRoot(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/custom/root/file.txt", []byte("old\n"))

	tool := NewPatchTool(fs)

	diff := `--- a/file.txt
+++ b/file.txt
@@ -1 +1 @@
-old
+new
`

	args := map[string]interface{}{
		"patch": diff,
		"root":  "/custom/root",
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "apply_patch",
		Arguments:        string(argsJSON),
		WorkingDirectory: "/workspace", // Different from root
	}

	ctx := test.LongContext(t)
	execCtx := &runtime.ExecutionContext{
		SessionID: "session_123",
		TurnID:    "turn_123",
		StartTime: time.Now(),
	}

	resp, err := tool.Execute(ctx, req, execCtx)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, *resp.Success)

	// Verify file was updated in custom root
	content := test.ReadFileFS(t, fs, "/custom/root/file.txt")
	assert.Equal(t, "new\n", string(content))
}

func TestPatchTool_Execute_FailureRollback(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/file1.txt", []byte("content 1\n"))
	// file2.txt missing - will cause failure

	tool := NewPatchTool(fs)

	diff := `--- a/file1.txt
+++ b/file1.txt
@@ -1 +1 @@
-content 1
+modified 1
--- a/file2.txt
+++ b/file2.txt
@@ -1 +1 @@
-content 2
+modified 2
`

	args := map[string]interface{}{
		"patch": diff,
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "apply_patch",
		Arguments:        string(argsJSON),
		WorkingDirectory: "/workspace",
	}

	ctx := test.LongContext(t)
	execCtx := &runtime.ExecutionContext{
		SessionID: "session_123",
		TurnID:    "turn_123",
		StartTime: time.Now(),
	}

	resp, err := tool.Execute(ctx, req, execCtx)
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.False(t, *resp.Success)

	// file1 should be rolled back to original
	content1 := test.ReadFileFS(t, fs, "/workspace/file1.txt")
	assert.Equal(t, "content 1\n", string(content1))

	// Response should contain error info
	assert.Contains(t, strings.ToLower(resp.Content), "error")
	assert.Contains(t, strings.ToLower(resp.Content), "rolled back")
}

func TestPatchTool_Execute_PathTraversalBlocked(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/.keep", []byte(""))

	tool := NewPatchTool(fs)

	diff := `--- /dev/null
+++ b/../etc/passwd
@@ -0,0 +1 @@
+malicious
`

	args := map[string]interface{}{
		"patch": diff,
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "apply_patch",
		Arguments:        string(argsJSON),
		WorkingDirectory: "/workspace",
	}

	ctx := test.LongContext(t)
	execCtx := &runtime.ExecutionContext{
		SessionID: "session_123",
		TurnID:    "turn_123",
		StartTime: time.Now(),
	}

	_, err := tool.Execute(ctx, req, execCtx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside root")
}

func TestPatchTool_Execute_AllowOutsideRoot(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/.keep", []byte(""))

	tool := NewPatchTool(fs)

	diff := `--- /dev/null
+++ b/../sibling/file.txt
@@ -0,0 +1 @@
+content
`

	args := map[string]interface{}{
		"patch":              diff,
		"allow_outside_root": true,
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "apply_patch",
		Arguments:        string(argsJSON),
		WorkingDirectory: "/workspace",
	}

	ctx := test.LongContext(t)
	execCtx := &runtime.ExecutionContext{
		SessionID: "session_123",
		TurnID:    "turn_123",
		StartTime: time.Now(),
	}

	resp, err := tool.Execute(ctx, req, execCtx)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, *resp.Success)

	test.AssertFileExistsFS(t, fs, "/sibling/file.txt")
}

// ============================================================================
// ToolRuntime Interface Compliance Tests
// ============================================================================

func TestPatchTool_ImplementsToolRuntime(t *testing.T) {
	fs := test.NewMemFS(t)
	var _ runtime.ToolRuntime = NewPatchTool(fs)
}

func TestPatchTool_SandboxPreference(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewPatchTool(fs)

	// Patch tool should forbid sandbox as it needs direct filesystem access
	assert.Equal(t, runtime.SandboxForbid, tool.SandboxPreference())
}

func TestPatchTool_SupportsParallel(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewPatchTool(fs)

	// Patch operations should be sequential to avoid conflicts
	assert.False(t, tool.SupportsParallel())
}

func TestPatchTool_ApprovalRequired(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewPatchTool(fs)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "apply_patch",
		Arguments:        `{"patch": "..."}`,
		WorkingDirectory: "/workspace",
	}

	// Patch modifies files, so should require approval with strict policy
	needsApproval := tool.NeedsInitialApproval(req, runtime.ApprovalUnlessTrusted, runtime.SandboxWorkspaceWrite)
	assert.True(t, needsApproval)

	// Should not require approval with never policy
	needsApproval = tool.NeedsInitialApproval(req, runtime.ApprovalNever, runtime.SandboxWorkspaceWrite)
	assert.False(t, needsApproval)
}

func TestPatchTool_ApprovalKey(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewPatchTool(fs)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "apply_patch",
		Arguments:        `{"patch": "diff content"}`,
		WorkingDirectory: "/workspace",
	}

	key := tool.ApprovalKey(req)
	assert.NotEmpty(t, key)

	// Same request should generate same key
	key2 := tool.ApprovalKey(req)
	assert.Equal(t, key, key2)

	// Different workspace should generate different key
	req.WorkingDirectory = "/other"
	key3 := tool.ApprovalKey(req)
	assert.NotEqual(t, key, key3)
}

func TestPatchTool_ContextCancellation(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/file.txt", []byte("content\n"))

	tool := NewPatchTool(fs)

	diff := `--- a/file.txt
+++ b/file.txt
@@ -1 +1 @@
-content
+new content
`

	args := map[string]interface{}{
		"patch": diff,
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "apply_patch",
		Arguments:        string(argsJSON),
		WorkingDirectory: "/workspace",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	execCtx := &runtime.ExecutionContext{
		SessionID: "session_123",
		TurnID:    "turn_123",
		StartTime: time.Now(),
	}

	_, err := tool.Execute(ctx, req, execCtx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

func TestPatchTool_InvalidJSON(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewPatchTool(fs)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "apply_patch",
		Arguments:        `{invalid json}`,
		WorkingDirectory: "/workspace",
	}

	ctx := test.LongContext(t)
	execCtx := &runtime.ExecutionContext{
		SessionID: "session_123",
		TurnID:    "turn_123",
		StartTime: time.Now(),
	}

	_, err := tool.Execute(ctx, req, execCtx)
	require.Error(t, err)

	var toolErr *runtime.ToolError
	require.ErrorAs(t, err, &toolErr)
	assert.Equal(t, runtime.ErrorInvalidArguments, toolErr.Kind)
}

// ============================================================================
// Edge Cases and Complex Scenarios
// ============================================================================

func TestApplyPatch_EmptyFile(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/empty.txt", []byte(""))

	diff := `--- a/empty.txt
+++ b/empty.txt
@@ -0,0 +1 @@
+new line
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	_, err = applyPatches(fs, patches, "/workspace", false)
	require.NoError(t, err)

	content := test.ReadFileFS(t, fs, "/workspace/empty.txt")
	assert.Equal(t, "new line\n", string(content))
}

func TestApplyPatch_NoNewlineAtEOF(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/file.txt", []byte("line without newline"))

	diff := `--- a/file.txt
+++ b/file.txt
@@ -1 +1 @@
-line without newline
\ No newline at end of file
+line with newline
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	_, err = applyPatches(fs, patches, "/workspace", false)
	require.NoError(t, err)

	content := test.ReadFileFS(t, fs, "/workspace/file.txt")
	assert.Equal(t, "line with newline\n", string(content))
}

func TestApplyPatch_LargeFile(t *testing.T) {
	fs := test.NewMemFS(t)

	// Create a large file with 1000 lines
	var lines []string
	for i := 1; i <= 1000; i++ {
		if i == 500 {
			lines = append(lines, "old line 500")
		} else {
			lines = append(lines, fmt.Sprintf("line %d", i))
		}
	}
	content := strings.Join(lines, "\n") + "\n"
	test.WriteFileFS(t, fs, "/workspace/large.txt", []byte(content))

	// Patch line 500
	diff := `--- a/large.txt
+++ b/large.txt
@@ -498,5 +498,5 @@
 line 498
 line 499
-old line 500
+new line 500
 line 501
 line 502
`
	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	_, err = applyPatches(fs, patches, "/workspace", false)
	require.NoError(t, err)

	content = string(test.ReadFileFS(t, fs, "/workspace/large.txt"))
	assert.Contains(t, content, "new line 500")
	assert.NotContains(t, content, "old line 500")
}

func TestApplyPatch_BinaryFilesNotSupported(t *testing.T) {
	// Git shows binary files differently
	diff := `diff --git a/image.png b/image.png
index 1234567..abcdefg 100644
Binary files a/image.png and b/image.png differ
`
	_, err := parseUnifiedDiff(diff)
	// Should either error or skip binary files
	require.Error(t, err)
}

func TestPatchTool_ComplexMultiFileScenario(t *testing.T) {
	fs := test.NewMemFS(t)

	// Setup: multiple files, some exist, some don't
	test.WriteFileFS(t, fs, "/workspace/update.txt", []byte("old content\n"))
	test.WriteFileFS(t, fs, "/workspace/delete.txt", []byte("to be deleted\n"))
	test.WriteFileFS(t, fs, "/workspace/move_from.txt", []byte("moving file\n"))

	tool := NewPatchTool(fs)

	diff := `--- a/update.txt
+++ b/update.txt
@@ -1 +1 @@
-old content
+new content
--- a/delete.txt
+++ /dev/null
@@ -1 +0,0 @@
-to be deleted
--- /dev/null
+++ b/new.txt
@@ -0,0 +1 @@
+brand new file
--- a/move_from.txt
+++ b/move_to.txt
@@ -1 +1 @@
 moving file
`

	args := map[string]interface{}{
		"patch": diff,
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "apply_patch",
		Arguments:        string(argsJSON),
		WorkingDirectory: "/workspace",
	}

	ctx := test.LongContext(t)
	execCtx := &runtime.ExecutionContext{
		SessionID: "session_123",
		TurnID:    "turn_123",
		StartTime: time.Now(),
	}

	resp, err := tool.Execute(ctx, req, execCtx)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, *resp.Success)

	// Verify all operations
	updateContent := test.ReadFileFS(t, fs, "/workspace/update.txt")
	assert.Equal(t, "new content\n", string(updateContent))

	test.AssertFileNotExistsFS(t, fs, "/workspace/delete.txt")

	newContent := test.ReadFileFS(t, fs, "/workspace/new.txt")
	assert.Equal(t, "brand new file\n", string(newContent))

	test.AssertFileNotExistsFS(t, fs, "/workspace/move_from.txt")
	moveContent := test.ReadFileFS(t, fs, "/workspace/move_to.txt")
	assert.Equal(t, "moving file\n", string(moveContent))
}

// ============================================================================
// CRLF Line Ending Tests
// ============================================================================

func TestApplyPatch_CRLFFileWithLFPatch(t *testing.T) {
	// Test that a file with CRLF endings can be patched with LF-style patch
	fs := test.NewMemFS(t)
	tool := NewPatchTool(fs)

	// Create a file with CRLF line endings
	originalContent := "line1\r\nline2\r\nline3\r\n"
	test.WriteFileFS(t, fs, "/workspace/test.txt", []byte(originalContent))

	// Create a unified diff with LF endings (as Git would generate)
	diff := `--- a/test.txt
+++ b/test.txt
@@ -1,3 +1,3 @@
 line1
-line2
+line2 modified
 line3
`

	args := PatchArgs{
		Patch: diff,
		Root:  "/workspace",
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:             "apply_patch",
		Arguments:        string(argsJSON),
		WorkingDirectory: "/workspace",
	}

	ctx := test.LongContext(t)
	resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{
		SessionID: "session_123",
		TurnID:    "turn_123",
	})
	require.NoError(t, err)
	assert.True(t, *resp.Success)

	// Read the result and verify CRLF is preserved
	result := test.ReadFileFS(t, fs, "/workspace/test.txt")
	expected := "line1\r\nline2 modified\r\nline3\r\n"
	assert.Equal(t, expected, string(result), "CRLF line endings should be preserved")
}

func TestApplyPatch_LFFileWithCRLFPatch(t *testing.T) {
	// Test that a file with LF endings can be patched with CRLF-style patch
	fs := test.NewMemFS(t)
	tool := NewPatchTool(fs)

	// Create a file with LF line endings
	originalContent := "line1\nline2\nline3\n"
	test.WriteFileFS(t, fs, "/workspace/test.txt", []byte(originalContent))

	// Create a unified diff with CRLF endings
	diff := "--- a/test.txt\r\n+++ b/test.txt\r\n@@ -1,3 +1,3 @@\r\n line1\r\n-line2\r\n+line2 modified\r\n line3\r\n"

	args := PatchArgs{
		Patch: diff,
		Root:  "/workspace",
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:             "apply_patch",
		Arguments:        string(argsJSON),
		WorkingDirectory: "/workspace",
	}

	ctx := test.LongContext(t)
	resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{
		SessionID: "session_123",
		TurnID:    "turn_123",
	})
	require.NoError(t, err)
	assert.True(t, *resp.Success)

	// Read the result and verify LF is preserved
	result := test.ReadFileFS(t, fs, "/workspace/test.txt")
	expected := "line1\nline2 modified\nline3\n"
	assert.Equal(t, expected, string(result), "LF line endings should be preserved")
}

func TestApplyPatch_MixedLineEndings(t *testing.T) {
	// Test that a file with mixed line endings gets normalized to LF
	fs := test.NewMemFS(t)
	tool := NewPatchTool(fs)

	// Create a file with mixed line endings
	originalContent := "line1\r\nline2\nline3\r\n"
	test.WriteFileFS(t, fs, "/workspace/test.txt", []byte(originalContent))

	// Create a unified diff
	diff := `--- a/test.txt
+++ b/test.txt
@@ -1,3 +1,3 @@
 line1
-line2
+line2 modified
 line3
`

	args := PatchArgs{
		Patch: diff,
		Root:  "/workspace",
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:             "apply_patch",
		Arguments:        string(argsJSON),
		WorkingDirectory: "/workspace",
	}

	ctx := test.LongContext(t)
	resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{
		SessionID: "session_123",
		TurnID:    "turn_123",
	})
	require.NoError(t, err)
	assert.True(t, *resp.Success)

	// Read the result - mixed files are normalized to LF
	result := test.ReadFileFS(t, fs, "/workspace/test.txt")
	expected := "line1\nline2 modified\nline3\n"
	assert.Equal(t, expected, string(result), "Mixed line endings should be normalized to LF")
}

func TestApplyPatch_CRLFAddFile(t *testing.T) {
	// Test adding a new file preserves default LF endings
	fs := test.NewMemFS(t)
	tool := NewPatchTool(fs)

	// Create a unified diff to add a new file
	diff := `--- /dev/null
+++ b/newfile.txt
@@ -0,0 +1,3 @@
+line1
+line2
+line3
`

	args := PatchArgs{
		Patch: diff,
		Root:  "/workspace",
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:             "apply_patch",
		Arguments:        string(argsJSON),
		WorkingDirectory: "/workspace",
	}

	ctx := test.LongContext(t)
	resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{
		SessionID: "session_123",
		TurnID:    "turn_123",
	})
	require.NoError(t, err)
	assert.True(t, *resp.Success)

	// Read the result - new files should default to LF
	result := test.ReadFileFS(t, fs, "/workspace/newfile.txt")
	expected := "line1\nline2\nline3\n"
	assert.Equal(t, expected, string(result), "New files should use LF line endings by default")
}

func TestApplyPatch_CRLFDeleteFile(t *testing.T) {
	// Test deleting a file with CRLF endings
	fs := test.NewMemFS(t)
	tool := NewPatchTool(fs)

	// Create a file with CRLF line endings
	originalContent := "line1\r\nline2\r\nline3\r\n"
	test.WriteFileFS(t, fs, "/workspace/delete.txt", []byte(originalContent))

	// Create a unified diff to delete the file
	diff := `--- a/delete.txt
+++ /dev/null
@@ -1,3 +0,0 @@
-line1
-line2
-line3
`

	args := PatchArgs{
		Patch: diff,
		Root:  "/workspace",
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:             "apply_patch",
		Arguments:        string(argsJSON),
		WorkingDirectory: "/workspace",
	}

	ctx := test.LongContext(t)
	resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{
		SessionID: "session_123",
		TurnID:    "turn_123",
	})
	require.NoError(t, err)
	assert.True(t, *resp.Success)

	// Verify file was deleted
	test.AssertFileNotExistsFS(t, fs, "/workspace/delete.txt")
}

func TestApplyPatch_CRLFMoveFile(t *testing.T) {
	// Test moving a file with CRLF endings preserves the line endings
	fs := test.NewMemFS(t)
	tool := NewPatchTool(fs)

	// Create a file with CRLF line endings
	originalContent := "line1\r\nline2\r\nline3\r\n"
	test.WriteFileFS(t, fs, "/workspace/source.txt", []byte(originalContent))

	// Create a unified diff to move/rename the file
	diff := `--- a/source.txt
+++ b/dest.txt
@@ -1,3 +1,3 @@
 line1
-line2
+line2 modified
 line3
`

	args := PatchArgs{
		Patch: diff,
		Root:  "/workspace",
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:             "apply_patch",
		Arguments:        string(argsJSON),
		WorkingDirectory: "/workspace",
	}

	ctx := test.LongContext(t)
	resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{
		SessionID: "session_123",
		TurnID:    "turn_123",
	})
	require.NoError(t, err)
	assert.True(t, *resp.Success)

	// Verify source file was removed
	test.AssertFileNotExistsFS(t, fs, "/workspace/source.txt")

	// Verify destination file has correct content with CRLF preserved
	result := test.ReadFileFS(t, fs, "/workspace/dest.txt")
	expected := "line1\r\nline2 modified\r\nline3\r\n"
	assert.Equal(t, expected, string(result), "CRLF line endings should be preserved during move")
}

func TestApplyPatch_CRLFRollback(t *testing.T) {
	// Test that rollback preserves original CRLF line endings
	fs := test.NewMemFS(t)

	// Create two files with CRLF line endings
	file1Content := "line1\r\nline2\r\nline3\r\n"
	file2Content := "foo\r\nbar\r\n"
	test.WriteFileFS(t, fs, "/workspace/file1.txt", []byte(file1Content))
	test.WriteFileFS(t, fs, "/workspace/file2.txt", []byte(file2Content))

	// Create a patch that will succeed for file1 but fail for file2
	diff := `--- a/file1.txt
+++ b/file1.txt
@@ -1,3 +1,3 @@
 line1
-line2
+line2 modified
 line3
--- a/file2.txt
+++ b/file2.txt
@@ -1,2 +1,2 @@
 foo
-nonexistent line
+this will fail
`

	patches, err := parseUnifiedDiff(diff)
	require.NoError(t, err)

	// Apply patches - should fail and rollback
	_, err = applyPatches(fs, patches, "/workspace", false)
	assert.Error(t, err, "Should fail because file2 patch doesn't match")

	// Verify file1 was rolled back with original CRLF
	result1 := test.ReadFileFS(t, fs, "/workspace/file1.txt")
	assert.Equal(t, file1Content, string(result1), "Rollback should preserve original CRLF in file1")

	// Verify file2 was unchanged with original CRLF
	result2 := test.ReadFileFS(t, fs, "/workspace/file2.txt")
	assert.Equal(t, file2Content, string(result2), "file2 should be unchanged with original CRLF")
}

func TestApplyPatch_ComplexCRLFScenario(t *testing.T) {
	// Test a complex scenario with multiple files having different line endings
	fs := test.NewMemFS(t)
	tool := NewPatchTool(fs)

	// Create files with different line endings
	test.WriteFileFS(t, fs, "/workspace/crlf.txt", []byte("a\r\nb\r\nc\r\n"))
	test.WriteFileFS(t, fs, "/workspace/lf.txt", []byte("x\ny\nz\n"))

	// Create a patch that modifies both files
	diff := `--- a/crlf.txt
+++ b/crlf.txt
@@ -1,3 +1,3 @@
 a
-b
+B
 c
--- a/lf.txt
+++ b/lf.txt
@@ -1,3 +1,3 @@
 x
-y
+Y
 z
`

	args := PatchArgs{
		Patch: diff,
		Root:  "/workspace",
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:             "apply_patch",
		Arguments:        string(argsJSON),
		WorkingDirectory: "/workspace",
	}

	ctx := test.LongContext(t)
	resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{
		SessionID: "session_123",
		TurnID:    "turn_123",
	})
	require.NoError(t, err)
	assert.True(t, *resp.Success)

	// Verify CRLF file kept CRLF
	crlfResult := test.ReadFileFS(t, fs, "/workspace/crlf.txt")
	assert.Equal(t, "a\r\nB\r\nc\r\n", string(crlfResult), "CRLF file should maintain CRLF")

	// Verify LF file kept LF
	lfResult := test.ReadFileFS(t, fs, "/workspace/lf.txt")
	assert.Equal(t, "x\nY\nz\n", string(lfResult), "LF file should maintain LF")
}
