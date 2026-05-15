package file

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
	"github.com/evmts/codex/codex-go/test"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Common utilities tests
// ============================================================================

func TestValidatePath_ValidPaths(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		path    string
		want    string
		wantErr bool
	}{
		{
			name: "absolute path within base",
			base: "/workspace",
			path: "/workspace/file.txt",
			want: "/workspace/file.txt",
		},
		{
			name: "relative path",
			base: "/workspace",
			path: "file.txt",
			want: "/workspace/file.txt",
		},
		{
			name: "nested relative path",
			base: "/workspace",
			path: "src/main.go",
			want: "/workspace/src/main.go",
		},
		{
			name: "path with dots inside",
			base: "/workspace",
			path: "file.test.txt",
			want: "/workspace/file.test.txt",
		},
		{
			name: "path with current dir",
			base: "/workspace",
			path: "./file.txt",
			want: "/workspace/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validatePath(tt.base, tt.path)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestValidatePath_PathTraversalAttempts(t *testing.T) {
	tests := []struct {
		name string
		base string
		path string
	}{
		{
			name: "parent directory with ..",
			base: "/workspace",
			path: "../etc/passwd",
		},
		{
			name: "multiple parent directories",
			base: "/workspace",
			path: "../../etc/passwd",
		},
		{
			name: "parent directory absolute path",
			base: "/workspace",
			path: "/workspace/../etc/passwd",
		},
		{
			name: "hidden parent in path",
			base: "/workspace",
			path: "safe/../../../etc/passwd",
		},
		{
			name: "symlink escape attempt",
			base: "/workspace",
			path: "/etc/passwd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validatePath(tt.base, tt.path)
			require.Error(t, err, "should reject path traversal attempt")
			assert.Contains(t, err.Error(), "outside workspace")
		})
	}
}

func TestIsBinaryFile(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		isBinary bool
	}{
		{
			name:     "text file",
			content:  []byte("Hello, World!\nThis is a text file."),
			isBinary: false,
		},
		{
			name:     "empty file",
			content:  []byte{},
			isBinary: false,
		},
		{
			name:     "file with newlines",
			content:  []byte("line1\nline2\nline3\n"),
			isBinary: false,
		},
		{
			name:     "binary file with null bytes",
			content:  []byte{0x00, 0x01, 0x02, 0x03},
			isBinary: true,
		},
		{
			name:     "binary file PNG header",
			content:  []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			isBinary: true,
		},
		{
			name:     "UTF-8 text with unicode",
			content:  []byte("Hello 世界 🌍"),
			isBinary: false,
		},
		{
			name:     "file with control characters but mostly text",
			content:  []byte("Hello\x00World\nThis has some binary"),
			isBinary: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBinaryFile(tt.content)
			assert.Equal(t, tt.isBinary, got)
		})
	}
}

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		size int64
		want string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatFileSize(tt.size)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ============================================================================
// ReadTool tests
// ============================================================================

func TestReadTool_Name(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewReadTool(fs)
	assert.Equal(t, "read_file", tool.Name())
}

func TestReadTool_ReadTextFile(t *testing.T) {
	fs := test.NewMemFS(t)
	content := "line1\nline2\nline3\nline4\nline5\n"
	test.WriteFileFS(t, fs, "/workspace/test.txt", []byte(content))

	tool := NewReadTool(fs)
	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "read_file",
		Arguments:        `{"path": "test.txt"}`,
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
	assert.Contains(t, resp.Content, "line1")
	assert.Contains(t, resp.Content, "line5")
}

func TestReadTool_ReadBinaryFile(t *testing.T) {
	fs := test.NewMemFS(t)
	binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	test.WriteFileFS(t, fs, "/workspace/image.png", binaryContent)

	tool := NewReadTool(fs)
	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "read_file",
		Arguments:        `{"path": "image.png"}`,
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
	assert.False(t, *resp.Success)
	assert.Contains(t, resp.Content, "binary")
}

func TestReadTool_FileNotFound(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewReadTool(fs)
	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "read_file",
		Arguments:        `{"path": "nonexistent.txt"}`,
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
	assert.False(t, *resp.Success)
	assert.Contains(t, resp.Content, "not found")
}

func TestReadTool_PathTraversal(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewReadTool(fs)
	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "read_file",
		Arguments:        `{"path": "../etc/passwd"}`,
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
	// Error should indicate path traversal or being outside workspace
	errMsg := err.Error()
	assert.True(t, strings.Contains(errMsg, "outside workspace") ||
		strings.Contains(errMsg, "traversal") ||
		strings.Contains(errMsg, "suspicious path pattern"),
		"Expected path traversal error, got: %s", errMsg)
}

func TestReadTool_LineRange(t *testing.T) {
	fs := test.NewMemFS(t)
	content := "line1\nline2\nline3\nline4\nline5\n"
	test.WriteFileFS(t, fs, "/workspace/test.txt", []byte(content))

	tool := NewReadTool(fs)
	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "read_file",
		Arguments:        `{"path": "test.txt", "start_line": 2, "end_line": 4}`,
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
	assert.Contains(t, resp.Content, "line2")
	assert.Contains(t, resp.Content, "line4")
	assert.NotContains(t, resp.Content, "line1")
	assert.NotContains(t, resp.Content, "line5")
}

// ============================================================================
// WriteTool tests
// ============================================================================

func TestWriteTool_Name(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewWriteTool(fs)
	assert.Equal(t, "write_file", tool.Name())
}

func TestWriteTool_CreateNewFile(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewWriteTool(fs)

	content := "Hello, World!"
	args := map[string]interface{}{
		"path":    "test.txt",
		"content": content,
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "write_file",
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

	// Verify file was created
	data := test.ReadFileFS(t, fs, "/workspace/test.txt")
	assert.Equal(t, content, string(data))
}

func TestWriteTool_OverwriteExistingFile(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/test.txt", []byte("old content"))

	tool := NewWriteTool(fs)
	newContent := "new content"
	args := map[string]interface{}{
		"path":    "test.txt",
		"content": newContent,
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "write_file",
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

	// Verify file was overwritten
	data := test.ReadFileFS(t, fs, "/workspace/test.txt")
	assert.Equal(t, newContent, string(data))
}

func TestWriteTool_CreateNestedDirectories(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewWriteTool(fs)

	content := "nested file"
	args := map[string]interface{}{
		"path":    "dir1/dir2/test.txt",
		"content": content,
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "write_file",
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

	// Verify file was created in nested directory
	data := test.ReadFileFS(t, fs, "/workspace/dir1/dir2/test.txt")
	assert.Equal(t, content, string(data))
}

func TestWriteTool_PathTraversal(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewWriteTool(fs)

	args := map[string]interface{}{
		"path":    "../etc/passwd",
		"content": "malicious",
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "write_file",
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
	// Error should indicate path traversal or being outside workspace
	errMsg := err.Error()
	assert.True(t, strings.Contains(errMsg, "outside workspace") ||
		strings.Contains(errMsg, "traversal") ||
		strings.Contains(errMsg, "suspicious path pattern"),
		"Expected path traversal error, got: %s", errMsg)
}

func TestWriteTool_AtomicWrite(t *testing.T) {
	// Test that writes are atomic (write to temp, then rename)
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/test.txt", []byte("original"))

	tool := NewWriteTool(fs)
	args := map[string]interface{}{
		"path":    "test.txt",
		"content": "updated",
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "write_file",
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

	// Verify final content
	data := test.ReadFileFS(t, fs, "/workspace/test.txt")
	assert.Equal(t, "updated", string(data))

	// Verify no temp files left behind
	entries, err := afero.ReadDir(fs, "/workspace")
	require.NoError(t, err)
	for _, entry := range entries {
		assert.NotContains(t, entry.Name(), ".tmp")
	}
}

// ============================================================================
// ListTool tests
// ============================================================================

func TestListTool_Name(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewListTool(fs)
	assert.Equal(t, "list_dir", tool.Name())
}

func TestListTool_ListFiles(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/file1.txt", []byte("content1"))
	test.WriteFileFS(t, fs, "/workspace/file2.txt", []byte("content2"))
	test.WriteFileFS(t, fs, "/workspace/dir1/file3.txt", []byte("content3"))

	tool := NewListTool(fs)
	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "list_dir",
		Arguments:        `{"path": "."}`,
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
	assert.Contains(t, resp.Content, "file1.txt")
	assert.Contains(t, resp.Content, "file2.txt")
	assert.Contains(t, resp.Content, "dir1")
}

func TestListTool_RecursiveList(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/file1.txt", []byte("content1"))
	test.WriteFileFS(t, fs, "/workspace/dir1/file2.txt", []byte("content2"))
	test.WriteFileFS(t, fs, "/workspace/dir1/dir2/file3.txt", []byte("content3"))

	tool := NewListTool(fs)
	args := map[string]interface{}{
		"path":      ".",
		"recursive": true,
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "list_dir",
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
	assert.Contains(t, resp.Content, "file1.txt")
	assert.Contains(t, resp.Content, "dir1/file2.txt")
	assert.Contains(t, resp.Content, "dir1/dir2/file3.txt")
}

func TestListTool_GlobPattern(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/file1.txt", []byte("content1"))
	test.WriteFileFS(t, fs, "/workspace/file2.go", []byte("content2"))
	test.WriteFileFS(t, fs, "/workspace/file3.txt", []byte("content3"))

	tool := NewListTool(fs)
	args := map[string]interface{}{
		"path":    ".",
		"pattern": "*.txt",
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "list_dir",
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
	assert.Contains(t, resp.Content, "file1.txt")
	assert.Contains(t, resp.Content, "file3.txt")
	assert.NotContains(t, resp.Content, "file2.go")
}

func TestListTool_DirectoryNotFound(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewListTool(fs)
	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "list_dir",
		Arguments:        `{"path": "nonexistent"}`,
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
	assert.False(t, *resp.Success)
	assert.Contains(t, resp.Content, "not found")
}

// ============================================================================
// GrepTool tests
// ============================================================================

func TestGrepTool_Name(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewGrepTool(fs)
	assert.Equal(t, "grep_files", tool.Name())
}

func TestGrepTool_SearchSingleFile(t *testing.T) {
	fs := test.NewMemFS(t)
	content := "line1 hello\nline2 world\nline3 hello world\n"
	test.WriteFileFS(t, fs, "/workspace/test.txt", []byte(content))

	tool := NewGrepTool(fs)
	args := map[string]interface{}{
		"pattern": "hello",
		"path":    "test.txt",
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "grep_files",
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
	assert.Contains(t, resp.Content, "line1 hello")
	assert.Contains(t, resp.Content, "line3 hello world")
	assert.NotContains(t, resp.Content, "line2 world")
}

func TestGrepTool_SearchMultipleFiles(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/file1.txt", []byte("match here\nno match\n"))
	test.WriteFileFS(t, fs, "/workspace/file2.txt", []byte("no match\nmatch here too\n"))

	tool := NewGrepTool(fs)
	args := map[string]interface{}{
		"pattern": "match here",
		"path":    ".",
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "grep_files",
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
	assert.Contains(t, resp.Content, "file1.txt")
	assert.Contains(t, resp.Content, "file2.txt")
}

func TestGrepTool_RegexPattern(t *testing.T) {
	fs := test.NewMemFS(t)
	content := "func test123()\nfunc test456()\nvar test = 1\n"
	test.WriteFileFS(t, fs, "/workspace/test.go", []byte(content))

	tool := NewGrepTool(fs)
	args := map[string]interface{}{
		"pattern": "func test[0-9]+",
		"path":    "test.go",
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "grep_files",
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
	assert.Contains(t, resp.Content, "func test123")
	assert.Contains(t, resp.Content, "func test456")
	assert.NotContains(t, resp.Content, "var test")
}

func TestGrepTool_CaseInsensitive(t *testing.T) {
	fs := test.NewMemFS(t)
	content := "Hello World\nhello world\nHELLO WORLD\n"
	test.WriteFileFS(t, fs, "/workspace/test.txt", []byte(content))

	tool := NewGrepTool(fs)
	args := map[string]interface{}{
		"pattern":          "hello",
		"path":             "test.txt",
		"case_insensitive": true,
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "grep_files",
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
	// All three lines should match
	assert.Contains(t, resp.Content, "Hello World")
	assert.Contains(t, resp.Content, "hello world")
	assert.Contains(t, resp.Content, "HELLO WORLD")
}

func TestGrepTool_FileGlobPattern(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/file1.go", []byte("package main\n"))
	test.WriteFileFS(t, fs, "/workspace/file2.go", []byte("package test\n"))
	test.WriteFileFS(t, fs, "/workspace/file3.txt", []byte("package docs\n"))

	tool := NewGrepTool(fs)
	args := map[string]interface{}{
		"pattern":      "package",
		"path":         ".",
		"file_pattern": "*.go",
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "grep_files",
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
	assert.Contains(t, resp.Content, "file1.go")
	assert.Contains(t, resp.Content, "file2.go")
	assert.NotContains(t, resp.Content, "file3.txt")
}

func TestGrepTool_NoMatches(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/test.txt", []byte("line1\nline2\nline3\n"))

	tool := NewGrepTool(fs)
	args := map[string]interface{}{
		"pattern": "notfound",
		"path":    "test.txt",
	}
	argsJSON, _ := json.Marshal(args)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "grep_files",
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
	assert.Contains(t, resp.Content, "No matches found")
}

// ============================================================================
// ToolRuntime interface compliance tests
// ============================================================================

func TestReadTool_ImplementsToolRuntime(t *testing.T) {
	fs := test.NewMemFS(t)
	var _ runtime.ToolRuntime = NewReadTool(fs)
}

func TestWriteTool_ImplementsToolRuntime(t *testing.T) {
	fs := test.NewMemFS(t)
	var _ runtime.ToolRuntime = NewWriteTool(fs)
}

func TestListTool_ImplementsToolRuntime(t *testing.T) {
	fs := test.NewMemFS(t)
	var _ runtime.ToolRuntime = NewListTool(fs)
}

func TestGrepTool_ImplementsToolRuntime(t *testing.T) {
	fs := test.NewMemFS(t)
	var _ runtime.ToolRuntime = NewGrepTool(fs)
}

// Test sandbox preferences for all tools
func TestFileTools_SandboxPreferences(t *testing.T) {
	fs := test.NewMemFS(t)

	tests := []struct {
		name string
		tool runtime.ToolRuntime
		want runtime.SandboxPreference
	}{
		{"ReadTool", NewReadTool(fs), runtime.SandboxAuto},
		{"WriteTool", NewWriteTool(fs), runtime.SandboxAuto},
		{"ListTool", NewListTool(fs), runtime.SandboxAuto},
		{"GrepTool", NewGrepTool(fs), runtime.SandboxAuto},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tool.SandboxPreference()
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test parallel execution support for all tools
func TestFileTools_SupportsParallel(t *testing.T) {
	fs := test.NewMemFS(t)

	tests := []struct {
		name string
		tool runtime.ToolRuntime
		want bool
	}{
		{"ReadTool", NewReadTool(fs), true},
		{"WriteTool", NewWriteTool(fs), false}, // Writes should be sequential
		{"ListTool", NewListTool(fs), true},
		{"GrepTool", NewGrepTool(fs), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tool.SupportsParallel()
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test that file tools don't require approval for read operations
func TestReadTools_NoApprovalRequired(t *testing.T) {
	fs := test.NewMemFS(t)

	tools := []runtime.ToolRuntime{
		NewReadTool(fs),
		NewListTool(fs),
		NewGrepTool(fs),
	}

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "test",
		Arguments:        `{}`,
		WorkingDirectory: "/workspace",
	}

	for _, tool := range tools {
		t.Run(tool.Name(), func(t *testing.T) {
			needsApproval := tool.NeedsInitialApproval(req, runtime.ApprovalOnRequest, runtime.SandboxWorkspaceWrite)
			assert.False(t, needsApproval, "Read operations should not require approval")
		})
	}
}

// Test that write tool may require approval
func TestWriteTool_ApprovalPolicy(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewWriteTool(fs)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "write_file",
		Arguments:        `{"path": "test.txt", "content": "data"}`,
		WorkingDirectory: "/workspace",
	}

	// Should require approval with strict policy
	needsApproval := tool.NeedsInitialApproval(req, runtime.ApprovalUnlessTrusted, runtime.SandboxWorkspaceWrite)
	assert.True(t, needsApproval, "Write operations should require approval with strict policy")

	// Should not require approval with never policy
	needsApproval = tool.NeedsInitialApproval(req, runtime.ApprovalNever, runtime.SandboxWorkspaceWrite)
	assert.False(t, needsApproval, "Write operations should not require approval with never policy")
}

// Test context cancellation
func TestFileTools_ContextCancellation(t *testing.T) {
	fs := test.NewMemFS(t)
	test.WriteFileFS(t, fs, "/workspace/test.txt", []byte("content"))

	tool := NewReadTool(fs)
	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "read_file",
		Arguments:        `{"path": "test.txt"}`,
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

// Test large file handling
func TestReadTool_LargeFile(t *testing.T) {
	fs := test.NewMemFS(t)

	// Create a large file (> 1MB)
	largeContent := make([]byte, 2*1024*1024) // 2MB
	for i := range largeContent {
		largeContent[i] = byte('a' + (i % 26))
	}
	test.WriteFileFS(t, fs, "/workspace/large.txt", largeContent)

	tool := NewReadTool(fs)
	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "read_file",
		Arguments:        `{"path": "large.txt"}`,
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
	// Large files should still succeed but may be truncated or summarized
	assert.True(t, *resp.Success)
}

// Test invalid JSON arguments
func TestFileTools_InvalidJSON(t *testing.T) {
	fs := test.NewMemFS(t)
	tool := NewReadTool(fs)

	req := &runtime.ToolRequest{
		CallID:           "call_123",
		ToolName:         "read_file",
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

// Test approval key generation
func TestFileTools_ApprovalKeys(t *testing.T) {
	fs := test.NewMemFS(t)

	tests := []struct {
		name string
		tool runtime.ToolRuntime
		args string
	}{
		{
			name: "ReadTool",
			tool: NewReadTool(fs),
			args: `{"path": "test.txt"}`,
		},
		{
			name: "WriteTool",
			tool: NewWriteTool(fs),
			args: `{"path": "test.txt", "content": "data"}`,
		},
		{
			name: "ListTool",
			tool: NewListTool(fs),
			args: `{"path": "."}`,
		},
		{
			name: "GrepTool",
			tool: NewGrepTool(fs),
			args: `{"pattern": "test", "path": "."}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &runtime.ToolRequest{
				CallID:           "call_123",
				ToolName:         tt.tool.Name(),
				Arguments:        tt.args,
				WorkingDirectory: "/workspace",
			}

			key := tt.tool.ApprovalKey(req)
			assert.NotEmpty(t, key, "Approval key should not be empty")

			// Same request should generate same key
			key2 := tt.tool.ApprovalKey(req)
			assert.Equal(t, key, key2, "Same request should generate same approval key")

			// Different working directory should generate different key
			req.WorkingDirectory = "/other"
			key3 := tt.tool.ApprovalKey(req)
			assert.NotEqual(t, key, key3, "Different workspace should generate different approval key")
		})
	}
}
