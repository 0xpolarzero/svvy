package shell

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsBinaryData_TextData tests detection of plain text
func TestIsBinaryData_TextData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "simple text",
			data: []byte("hello world"),
			want: false,
		},
		{
			name: "text with newlines",
			data: []byte("line1\nline2\nline3\n"),
			want: false,
		},
		{
			name: "text with tabs",
			data: []byte("col1\tcol2\tcol3"),
			want: false,
		},
		{
			name: "unicode text",
			data: []byte("Hello 世界 🌍"),
			want: false,
		},
		{
			name: "json data",
			data: []byte(`{"key": "value", "number": 123}`),
			want: false,
		},
		{
			name: "empty data",
			data: []byte{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBinaryData(tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestIsBinaryData_BinaryData tests detection of binary data
func TestIsBinaryData_BinaryData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "null byte",
			data: []byte{0x00, 0x01, 0x02},
			want: true,
		},
		{
			name: "null byte in middle",
			data: []byte("hello\x00world"),
			want: true,
		},
		{
			name: "invalid utf8",
			data: []byte{0xff, 0xfe, 0xfd},
			want: true,
		},
		{
			name: "high control character ratio",
			data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBinaryData(tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestIsBinaryData_RealWorldBinary tests with actual binary formats
func TestIsBinaryData_RealWorldBinary(t *testing.T) {
	// Create gzip data
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	_, err := gzWriter.Write([]byte("test data"))
	require.NoError(t, err)
	require.NoError(t, gzWriter.Close())
	gzipData := buf.Bytes()

	// PNG header
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

	// ELF header with full magic (Linux executable) - includes null bytes
	elfHeader := []byte{0x7f, 0x45, 0x4c, 0x46, 0x02, 0x01, 0x01, 0x00}

	tests := []struct {
		name string
		data []byte
	}{
		{"gzip data", gzipData},
		{"png header", pngHeader},
		{"elf header", elfHeader},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isBinary := IsBinaryData(tt.data)
			assert.True(t, isBinary, "Expected %s to be detected as binary", tt.name)
		})
	}
}

// TestEncodeChunk tests encoding of text and binary chunks
func TestEncodeChunk(t *testing.T) {
	tests := []struct {
		name           string
		data           []byte
		wantIsBinary   bool
		validateResult func(t *testing.T, encoded string, isBinary bool)
	}{
		{
			name:         "text chunk",
			data:         []byte("hello world"),
			wantIsBinary: false,
			validateResult: func(t *testing.T, encoded string, isBinary bool) {
				assert.Equal(t, "hello world", encoded)
				assert.False(t, isBinary)
			},
		},
		{
			name:         "binary chunk with null byte",
			data:         []byte{0x00, 0x01, 0x02, 0x03},
			wantIsBinary: true,
			validateResult: func(t *testing.T, encoded string, isBinary bool) {
				// Should be base64 encoded
				decoded, err := base64.StdEncoding.DecodeString(encoded)
				require.NoError(t, err)
				assert.Equal(t, []byte{0x00, 0x01, 0x02, 0x03}, decoded)
				assert.True(t, isBinary)
			},
		},
		{
			name:         "mixed binary chunk",
			data:         []byte("test\x00data"),
			wantIsBinary: true,
			validateResult: func(t *testing.T, encoded string, isBinary bool) {
				decoded, err := base64.StdEncoding.DecodeString(encoded)
				require.NoError(t, err)
				assert.Equal(t, []byte("test\x00data"), decoded)
				assert.True(t, isBinary)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, isBinary := EncodeChunk(tt.data)
			assert.Equal(t, tt.wantIsBinary, isBinary)
			tt.validateResult(t, encoded, isBinary)
		})
	}
}

// TestDecodeChunk tests decoding of encoded chunks
func TestDecodeChunk(t *testing.T) {
	tests := []struct {
		name     string
		chunk    string
		isBinary bool
		want     []byte
	}{
		{
			name:     "text chunk",
			chunk:    "hello world",
			isBinary: false,
			want:     []byte("hello world"),
		},
		{
			name:     "binary chunk",
			chunk:    base64.StdEncoding.EncodeToString([]byte{0x00, 0x01, 0x02}),
			isBinary: true,
			want:     []byte{0x00, 0x01, 0x02},
		},
		{
			name:     "empty text",
			chunk:    "",
			isBinary: false,
			want:     []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeChunk(tt.chunk, tt.isBinary)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestEncodeDecodeRoundTrip tests round-trip encoding and decoding
func TestEncodeDecodeRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"text", []byte("hello world")},
		{"binary", []byte{0x00, 0x01, 0x02, 0x03, 0x04}},
		{"mixed", []byte("test\x00data\nmore")},
		{"unicode", []byte("Hello 世界 🌍")},
		{"empty", []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, isBinary := EncodeChunk(tt.data)
			decoded, err := DecodeChunk(encoded, isBinary)
			require.NoError(t, err)
			assert.Equal(t, tt.data, decoded)
		})
	}
}

// TestIsBinaryData_EdgeCases tests edge cases in binary detection
func TestIsBinaryData_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "mostly control chars but under threshold",
			data: []byte{0x01, 0x02, 'a', 'b', 'c', 'd', 'e'},
			want: false, // 2/7 = 28.5% < 30%
		},
		{
			name: "exactly at threshold",
			data: []byte{0x01, 0x02, 0x03, 'a', 'b', 'c', 'd', 'e', 'f', 'g'},
			want: false, // 3/10 = 30%, but we use > not >=
		},
		{
			name: "just over threshold",
			data: []byte{0x01, 0x02, 0x03, 0x04, 'a', 'b', 'c', 'd', 'e', 'f'},
			want: true, // 4/10 = 40% > 30%
		},
		{
			name: "carriage return and newline ok",
			data: []byte("line1\r\nline2\r\n"),
			want: false,
		},
		{
			name: "tabs are ok",
			data: []byte("col1\tcol2\tcol3\t"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBinaryData(tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}

// BenchmarkIsBinaryData benchmarks binary detection
func BenchmarkIsBinaryData(b *testing.B) {
	benchmarks := []struct {
		name string
		data []byte
	}{
		{"small text", []byte("hello world")},
		{"small binary", []byte{0x00, 0x01, 0x02, 0x03}},
		{"medium text", bytes.Repeat([]byte("test data\n"), 100)},
		{"medium binary", bytes.Repeat([]byte{0x00, 0x01, 0x02}, 100)},
		{"large text", bytes.Repeat([]byte("test data\n"), 1000)},
		{"large binary", bytes.Repeat([]byte{0x00, 0x01, 0x02}, 1000)},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.SetBytes(int64(len(bm.data)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				IsBinaryData(bm.data)
			}
		})
	}
}

// BenchmarkEncodeChunk benchmarks chunk encoding
func BenchmarkEncodeChunk(b *testing.B) {
	benchmarks := []struct {
		name string
		data []byte
	}{
		{"small text", []byte("hello world")},
		{"small binary", []byte{0x00, 0x01, 0x02, 0x03}},
		{"medium text", bytes.Repeat([]byte("test data\n"), 100)},
		{"medium binary", bytes.Repeat([]byte{0x00, 0x01, 0x02}, 100)},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.SetBytes(int64(len(bm.data)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				EncodeChunk(bm.data)
			}
		})
	}
}
