package image

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
)

func TestImageTool_Execute_Success(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a fake PNG file
	imagePath := filepath.Join(tempDir, "test.png")
	imageData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG signature
	if err := os.WriteFile(imagePath, imageData, 0644); err != nil {
		t.Fatalf("failed to create test image: %v", err)
	}

	tool := NewImageTool()
	ctx := context.Background()

	// Test with absolute path
	req := &runtime.ToolRequest{
		CallID:           "test-call-1",
		ToolName:         "view_image",
		Arguments:        `{"path":"` + imagePath + `"}`,
		WorkingDirectory: tempDir,
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
	}

	response, err := tool.Execute(ctx, req, execCtx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify success
	if response.Success == nil || !*response.Success {
		t.Errorf("expected success=true, got %v", response.Success)
	}

	// Verify response content
	if response.Content != "attached local image path" {
		t.Errorf("unexpected content: %s", response.Content)
	}

	// Verify metadata contains image_url
	if response.Metadata == nil {
		t.Fatal("expected metadata to be present")
	}

	imageURL, ok := response.Metadata["image_url"].(string)
	if !ok {
		t.Fatal("expected image_url in metadata")
	}

	if !strings.HasPrefix(imageURL, "data:image/png;base64,") {
		t.Errorf("expected data URL prefix, got: %s", imageURL[:50])
	}

	// Verify metadata contains paths
	if path, ok := response.Metadata["image_path"].(string); !ok || path != imagePath {
		t.Errorf("expected image_path=%s, got %v", imagePath, path)
	}
}

func TestImageTool_Execute_RelativePath(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a fake image in a subdirectory
	subdir := filepath.Join(tempDir, "images")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	imagePath := filepath.Join(subdir, "test.jpg")
	imageData := []byte{0xFF, 0xD8, 0xFF, 0xE0} // JPEG signature
	if err := os.WriteFile(imagePath, imageData, 0644); err != nil {
		t.Fatalf("failed to create test image: %v", err)
	}

	tool := NewImageTool()
	ctx := context.Background()

	// Test with relative path
	req := &runtime.ToolRequest{
		CallID:           "test-call-2",
		ToolName:         "view_image",
		Arguments:        `{"path":"images/test.jpg"}`,
		WorkingDirectory: tempDir,
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
	}

	response, err := tool.Execute(ctx, req, execCtx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify success
	if response.Success == nil || !*response.Success {
		t.Errorf("expected success=true, got %v", response.Success)
	}

	// Verify metadata contains correct absolute path
	absPath, ok := response.Metadata["image_path"].(string)
	if !ok {
		t.Fatal("expected image_path in metadata")
	}

	if absPath != imagePath {
		t.Errorf("expected absolute path %s, got %s", imagePath, absPath)
	}

	// Verify original path is preserved
	origPath, ok := response.Metadata["original_path"].(string)
	if !ok || origPath != "images/test.jpg" {
		t.Errorf("expected original_path=images/test.jpg, got %v", origPath)
	}
}

func TestImageTool_Execute_FileNotFound(t *testing.T) {
	tempDir := t.TempDir()

	tool := NewImageTool()
	ctx := context.Background()

	req := &runtime.ToolRequest{
		CallID:           "test-call-3",
		ToolName:         "view_image",
		Arguments:        `{"path":"nonexistent.png"}`,
		WorkingDirectory: tempDir,
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
	}

	response, err := tool.Execute(ctx, req, execCtx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify failure
	if response.Success == nil || *response.Success {
		t.Errorf("expected success=false, got %v", response.Success)
	}

	// Verify error message
	if !strings.Contains(response.Content, "unable to locate image") {
		t.Errorf("unexpected error message: %s", response.Content)
	}

	if !strings.Contains(response.Content, "file does not exist") {
		t.Errorf("unexpected error message: %s", response.Content)
	}
}

func TestImageTool_Execute_DirectoryPath(t *testing.T) {
	tempDir := t.TempDir()

	// Create a subdirectory
	subdir := filepath.Join(tempDir, "images")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	tool := NewImageTool()
	ctx := context.Background()

	req := &runtime.ToolRequest{
		CallID:           "test-call-4",
		ToolName:         "view_image",
		Arguments:        `{"path":"images"}`,
		WorkingDirectory: tempDir,
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
	}

	response, err := tool.Execute(ctx, req, execCtx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify failure
	if response.Success == nil || *response.Success {
		t.Errorf("expected success=false, got %v", response.Success)
	}

	// Verify error message
	if !strings.Contains(response.Content, "is not a file") {
		t.Errorf("unexpected error message: %s", response.Content)
	}
}

func TestImageTool_Execute_UnsupportedFormat(t *testing.T) {
	tempDir := t.TempDir()

	// Create a file with unsupported extension
	textPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(textPath, []byte("not an image"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := NewImageTool()
	ctx := context.Background()

	req := &runtime.ToolRequest{
		CallID:           "test-call-5",
		ToolName:         "view_image",
		Arguments:        `{"path":"test.txt"}`,
		WorkingDirectory: tempDir,
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
	}

	response, err := tool.Execute(ctx, req, execCtx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify failure
	if response.Success == nil || *response.Success {
		t.Errorf("expected success=false, got %v", response.Success)
	}

	// Verify error message mentions unsupported format
	if !strings.Contains(response.Content, "unsupported image format") {
		t.Errorf("unexpected error message: %s", response.Content)
	}
}

func TestImageTool_Execute_InvalidArguments(t *testing.T) {
	tool := NewImageTool()
	ctx := context.Background()

	tests := []struct {
		name      string
		arguments string
		wantErr   bool
	}{
		{
			name:      "empty arguments",
			arguments: "",
			wantErr:   true,
		},
		{
			name:      "invalid json",
			arguments: "{invalid",
			wantErr:   true,
		},
		{
			name:      "missing path",
			arguments: "{}",
			wantErr:   true,
		},
		{
			name:      "empty path",
			arguments: `{"path":""}`,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &runtime.ToolRequest{
				CallID:           "test-call",
				ToolName:         "view_image",
				Arguments:        tt.arguments,
				WorkingDirectory: t.TempDir(),
			}

			execCtx := &runtime.ExecutionContext{
				SessionID: "test-session",
				TurnID:    "test-turn",
			}

			_, err := tool.Execute(ctx, req, execCtx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil {
				toolErr, ok := err.(*runtime.ToolError)
				if !ok {
					t.Errorf("expected ToolError, got %T", err)
				} else if toolErr.Kind != runtime.ErrorInvalidArguments {
					t.Errorf("expected ErrorInvalidArguments, got %v", toolErr.Kind)
				}
			}
		})
	}
}

func TestImageTool_Execute_AllSupportedFormats(t *testing.T) {
	tempDir := t.TempDir()

	// Test data for each supported format
	formats := []struct {
		extension string
		signature []byte
		mimeType  string
	}{
		{".png", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, "image/png"},
		{".jpg", []byte{0xFF, 0xD8, 0xFF, 0xE0}, "image/jpeg"},
		{".jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0}, "image/jpeg"},
		{".gif", []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61}, "image/gif"},
		{".webp", []byte{0x52, 0x49, 0x46, 0x46}, "image/webp"},
	}

	tool := NewImageTool()
	ctx := context.Background()

	for _, format := range formats {
		t.Run(format.extension, func(t *testing.T) {
			// Create test image
			imagePath := filepath.Join(tempDir, "test"+format.extension)
			if err := os.WriteFile(imagePath, format.signature, 0644); err != nil {
				t.Fatalf("failed to create test image: %v", err)
			}

			req := &runtime.ToolRequest{
				CallID:           "test-call",
				ToolName:         "view_image",
				Arguments:        `{"path":"` + imagePath + `"}`,
				WorkingDirectory: tempDir,
			}

			execCtx := &runtime.ExecutionContext{
				SessionID: "test-session",
				TurnID:    "test-turn",
			}

			response, err := tool.Execute(ctx, req, execCtx)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}

			// Verify success
			if response.Success == nil || !*response.Success {
				t.Errorf("expected success=true, got %v", response.Success)
			}

			// Verify data URL has correct MIME type
			imageURL, ok := response.Metadata["image_url"].(string)
			if !ok {
				t.Fatal("expected image_url in metadata")
			}

			expectedPrefix := "data:" + format.mimeType + ";base64,"
			if !strings.HasPrefix(imageURL, expectedPrefix) {
				t.Errorf("expected prefix %s, got: %s", expectedPrefix, imageURL[:50])
			}
		})
	}
}

func TestImageTool_Name(t *testing.T) {
	tool := NewImageTool()
	if tool.Name() != "view_image" {
		t.Errorf("expected name 'view_image', got '%s'", tool.Name())
	}
}

func TestImageTool_NeedsInitialApproval(t *testing.T) {
	tool := NewImageTool()
	req := &runtime.ToolRequest{}

	// Image viewing should never need approval
	policies := []runtime.ApprovalPolicy{
		runtime.ApprovalNever,
		runtime.ApprovalOnFailure,
		runtime.ApprovalOnRequest,
		runtime.ApprovalUnlessTrusted,
	}

	for _, policy := range policies {
		if tool.NeedsInitialApproval(req, policy, runtime.SandboxReadOnly) {
			t.Errorf("expected no approval needed for policy %v", policy)
		}
	}
}

func TestImageTool_SandboxPreference(t *testing.T) {
	tool := NewImageTool()
	if tool.SandboxPreference() != runtime.SandboxForbid {
		t.Errorf("expected SandboxForbid, got %v", tool.SandboxPreference())
	}
}

func TestImageTool_SupportsParallel(t *testing.T) {
	tool := NewImageTool()
	if !tool.SupportsParallel() {
		t.Error("expected image tool to support parallel execution")
	}
}

func TestImageTool_Execute_ContextCancellation(t *testing.T) {
	tempDir := t.TempDir()

	tool := NewImageTool()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := &runtime.ToolRequest{
		CallID:           "test-call",
		ToolName:         "view_image",
		Arguments:        `{"path":"test.png"}`,
		WorkingDirectory: tempDir,
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
	}

	_, err := tool.Execute(ctx, req, execCtx)
	if err == nil {
		t.Error("expected error for cancelled context")
	}

	toolErr, ok := err.(*runtime.ToolError)
	if !ok {
		t.Errorf("expected ToolError, got %T", err)
	} else if toolErr.Kind != runtime.ErrorExecution {
		t.Errorf("expected ErrorExecution, got %v", toolErr.Kind)
	}
}

func TestImageTool_Execute_ExecutionTime(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test image
	imagePath := filepath.Join(tempDir, "test.png")
	imageData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if err := os.WriteFile(imagePath, imageData, 0644); err != nil {
		t.Fatalf("failed to create test image: %v", err)
	}

	tool := NewImageTool()
	ctx := context.Background()

	req := &runtime.ToolRequest{
		CallID:           "test-call",
		ToolName:         "view_image",
		Arguments:        `{"path":"` + imagePath + `"}`,
		WorkingDirectory: tempDir,
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "test-session",
		TurnID:    "test-turn",
	}

	response, err := tool.Execute(ctx, req, execCtx)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify execution time is recorded and reasonable
	if response.ExecutionTime <= 0 {
		t.Error("expected positive execution time")
	}

	if response.ExecutionTime > 5*time.Second {
		t.Errorf("execution time too long: %v", response.ExecutionTime)
	}
}

func TestEncodeToDataURL(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test PNG file
	pngPath := filepath.Join(tempDir, "test.png")
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if err := os.WriteFile(pngPath, pngData, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	dataURL, err := EncodeToDataURL(pngPath)
	if err != nil {
		t.Fatalf("EncodeToDataURL failed: %v", err)
	}

	// Verify data URL format
	if !strings.HasPrefix(dataURL, "data:image/png;base64,") {
		t.Errorf("unexpected data URL format: %s", dataURL[:50])
	}

	// Decode and verify data matches
	decoded, mimeType, err := DecodeDataURL(dataURL)
	if err != nil {
		t.Fatalf("DecodeDataURL failed: %v", err)
	}

	if mimeType != "image/png" {
		t.Errorf("expected mime type image/png, got %s", mimeType)
	}

	if string(decoded) != string(pngData) {
		t.Error("decoded data does not match original")
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		path       string
		wantFormat string
		wantOk     bool
	}{
		{"test.png", "image/png", true},
		{"test.PNG", "image/png", true}, // case insensitive
		{"test.jpg", "image/jpeg", true},
		{"test.jpeg", "image/jpeg", true},
		{"test.gif", "image/gif", true},
		{"test.webp", "image/webp", true},
		{"test.txt", "", false},
		{"test", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			format, ok := DetectFormat(tt.path)
			if ok != tt.wantOk {
				t.Errorf("DetectFormat(%s) ok = %v, want %v", tt.path, ok, tt.wantOk)
			}
			if ok && format.MimeType != tt.wantFormat {
				t.Errorf("DetectFormat(%s) format = %s, want %s", tt.path, format.MimeType, tt.wantFormat)
			}
		})
	}
}

func TestValidateFormat(t *testing.T) {
	tests := []struct {
		path    string
		wantErr bool
	}{
		{"test.png", false},
		{"test.jpg", false},
		{"test.jpeg", false},
		{"test.gif", false},
		{"test.webp", false},
		{"test.txt", true},
		{"test.bmp", true},
		{"test", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			err := ValidateFormat(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFormat(%s) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestImageTool_ApprovalKey(t *testing.T) {
	tool := NewImageTool()

	req1 := &runtime.ToolRequest{
		ToolName:         "view_image",
		Arguments:        `{"path":"test.png"}`,
		WorkingDirectory: "/tmp",
	}

	req2 := &runtime.ToolRequest{
		ToolName:         "view_image",
		Arguments:        `{"path":"test.png"}`,
		WorkingDirectory: "/tmp",
	}

	req3 := &runtime.ToolRequest{
		ToolName:         "view_image",
		Arguments:        `{"path":"other.png"}`,
		WorkingDirectory: "/tmp",
	}

	key1 := tool.ApprovalKey(req1)
	key2 := tool.ApprovalKey(req2)
	key3 := tool.ApprovalKey(req3)

	// Same request should produce same key
	if key1 != key2 {
		t.Errorf("expected same key for identical requests, got %s and %s", key1, key2)
	}

	// Different request should produce different key
	if key1 == key3 {
		t.Errorf("expected different key for different requests, got %s", key1)
	}
}

// Benchmark tests
func BenchmarkImageTool_Execute(b *testing.B) {
	tempDir := b.TempDir()

	// Create a test image
	imagePath := filepath.Join(tempDir, "test.png")
	imageData := make([]byte, 1024) // 1KB image
	for i := range imageData {
		imageData[i] = byte(i % 256)
	}
	if err := os.WriteFile(imagePath, imageData, 0644); err != nil {
		b.Fatalf("failed to create test image: %v", err)
	}

	tool := NewImageTool()
	ctx := context.Background()

	args, _ := json.Marshal(map[string]string{"path": imagePath})
	req := &runtime.ToolRequest{
		CallID:           "bench-call",
		ToolName:         "view_image",
		Arguments:        string(args),
		WorkingDirectory: tempDir,
	}

	execCtx := &runtime.ExecutionContext{
		SessionID: "bench-session",
		TurnID:    "bench-turn",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tool.Execute(ctx, req, execCtx)
	}
}

func BenchmarkEncodeToDataURL(b *testing.B) {
	tempDir := b.TempDir()

	// Create test images of different sizes
	sizes := []int{1024, 10240, 102400} // 1KB, 10KB, 100KB

	for _, size := range sizes {
		b.Run(fmt.Sprintf("%dKB", size/1024), func(b *testing.B) {
			imagePath := filepath.Join(tempDir, fmt.Sprintf("test_%d.png", size))
			imageData := make([]byte, size)
			if err := os.WriteFile(imagePath, imageData, 0644); err != nil {
				b.Fatalf("failed to create test image: %v", err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = EncodeToDataURL(imagePath)
			}
		})
	}
}
