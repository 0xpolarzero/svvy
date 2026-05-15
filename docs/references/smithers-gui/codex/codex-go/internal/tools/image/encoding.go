package image

import (
	"encoding/base64"
	"fmt"
	"os"
)

// EncodeToDataURL reads an image file and encodes it as a base64 data URL.
// The data URL format is: data:<mimetype>;base64,<base64-encoded-data>
//
// Example: data:image/png;base64,iVBORw0KGgoAAAANSUhEUg...
func EncodeToDataURL(path string) (string, error) {
	// Validate format first
	format, supported := DetectFormat(path)
	if !supported {
		return "", fmt.Errorf("unsupported image format: %s", path)
	}

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read image file: %w", err)
	}

	// Encode to base64
	encoded := base64.StdEncoding.EncodeToString(data)

	// Construct data URL
	dataURL := fmt.Sprintf("data:%s;base64,%s", format.MimeType, encoded)

	return dataURL, nil
}

// EncodeBytes encodes raw image bytes to a base64 data URL.
// The mimeType parameter should be a valid MIME type (e.g., "image/png").
func EncodeBytes(data []byte, mimeType string) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)
}

// DecodeDataURL extracts the raw image data from a base64 data URL.
// Returns the decoded bytes and the MIME type.
func DecodeDataURL(dataURL string) ([]byte, string, error) {
	// Parse data URL format: data:<mimetype>;base64,<data>
	const prefix = "data:"
	const base64Marker = ";base64,"

	if len(dataURL) < len(prefix) {
		return nil, "", fmt.Errorf("invalid data URL: too short")
	}

	if dataURL[:len(prefix)] != prefix {
		return nil, "", fmt.Errorf("invalid data URL: missing 'data:' prefix")
	}

	// Find the base64 marker
	markerIdx := -1
	for i := len(prefix); i < len(dataURL)-len(base64Marker); i++ {
		if dataURL[i:i+len(base64Marker)] == base64Marker {
			markerIdx = i
			break
		}
	}

	if markerIdx == -1 {
		return nil, "", fmt.Errorf("invalid data URL: missing ';base64,' marker")
	}

	// Extract MIME type
	mimeType := dataURL[len(prefix):markerIdx]

	// Extract base64 data
	base64Data := dataURL[markerIdx+len(base64Marker):]

	// Decode base64
	decoded, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode base64 data: %w", err)
	}

	return decoded, mimeType, nil
}
