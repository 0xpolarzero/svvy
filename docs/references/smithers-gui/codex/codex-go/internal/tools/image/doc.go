// Package image provides the view_image tool for attaching local images to conversations.
//
// The image tool enables AI models to view local images by:
//   - Loading images from the filesystem (with path validation)
//   - Validating image formats (PNG, JPEG, WebP, GIF)
//   - Encoding images to base64 data URLs
//   - Preparing images for attachment to conversation context
//
// # Supported Formats
//
// The tool supports the following image formats:
//   - PNG (Portable Network Graphics)
//   - JPEG/JPG
//   - WebP
//   - GIF (Graphics Interchange Format)
//
// # Security
//
// The tool performs several security checks:
//   - Validates that paths exist and are files (not directories)
//   - Handles both absolute and relative paths safely
//   - Validates image format by file extension
//   - Does not require user approval (read-only operation)
//   - Runs without sandbox restrictions for filesystem access
//
// # Usage Example
//
//	tool := image.NewImageTool()
//	req := &runtime.ToolRequest{
//		CallID:           "call-123",
//		ToolName:         "view_image",
//		Arguments:        `{"path":"screenshot.png"}`,
//		WorkingDirectory: "/workspace",
//	}
//	response, err := tool.Execute(ctx, req, execCtx)
//
// The response includes:
//   - Success status (true/false)
//   - Content message ("attached local image path")
//   - Metadata containing the base64-encoded data URL
//
// # Integration
//
// The tool is registered in the default tool registry and integrated with
// the protocol layer to emit EventViewImageToolCall events when images are
// attached. The conversation manager uses the data URL from the response
// metadata to include the image in subsequent API requests.
//
// # Performance
//
// Benchmarks on Apple M4 Pro (as of 2025):
//   - 1KB image:   ~10µs to encode
//   - 10KB image:  ~16µs to encode
//   - 100KB image: ~79µs to encode
//   - Parallel execution supported for multiple images
package image
