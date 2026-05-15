// Package image provides tools for viewing and attaching images to conversations.
package image

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SupportedFormat represents an image format that can be viewed.
type SupportedFormat struct {
	// Extension is the file extension (e.g., ".png", ".jpg")
	Extension string
	// MimeType is the MIME type for the format (e.g., "image/png")
	MimeType string
	// Description is a human-readable format name
	Description string
}

// Common image formats supported by the view_image tool.
var (
	FormatPNG = SupportedFormat{
		Extension:   ".png",
		MimeType:    "image/png",
		Description: "PNG (Portable Network Graphics)",
	}
	FormatJPEG = SupportedFormat{
		Extension:   ".jpg",
		MimeType:    "image/jpeg",
		Description: "JPEG",
	}
	FormatJPG = SupportedFormat{
		Extension:   ".jpeg",
		MimeType:    "image/jpeg",
		Description: "JPEG",
	}
	FormatWebP = SupportedFormat{
		Extension:   ".webp",
		MimeType:    "image/webp",
		Description: "WebP",
	}
	FormatGIF = SupportedFormat{
		Extension:   ".gif",
		MimeType:    "image/gif",
		Description: "GIF (Graphics Interchange Format)",
	}
)

// SupportedFormats lists all image formats that can be viewed.
var SupportedFormats = []SupportedFormat{
	FormatPNG,
	FormatJPEG,
	FormatJPG,
	FormatWebP,
	FormatGIF,
}

// DetectFormat determines the image format from a file path.
// Returns the format and true if supported, or an empty format and false if unsupported.
func DetectFormat(path string) (SupportedFormat, bool) {
	ext := strings.ToLower(filepath.Ext(path))
	for _, format := range SupportedFormats {
		if format.Extension == ext {
			return format, true
		}
	}
	return SupportedFormat{}, false
}

// ValidateFormat checks if the file extension indicates a supported image format.
// Returns an error if the format is not supported.
func ValidateFormat(path string) error {
	_, supported := DetectFormat(path)
	if !supported {
		ext := filepath.Ext(path)
		return fmt.Errorf("unsupported image format: %s (supported formats: PNG, JPEG, WebP, GIF)", ext)
	}
	return nil
}

// GetMimeType returns the MIME type for a given file path.
// Returns an empty string if the format is not supported.
func GetMimeType(path string) string {
	format, supported := DetectFormat(path)
	if !supported {
		return ""
	}
	return format.MimeType
}
