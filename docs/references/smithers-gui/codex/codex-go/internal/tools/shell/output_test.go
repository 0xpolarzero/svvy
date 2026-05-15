package shell

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSyncBuffer tests the SyncBuffer type
func TestSyncBuffer(t *testing.T) {
	buf := NewSyncBuffer()

	// Write some data
	n, err := buf.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", buf.String())
	assert.Equal(t, []byte("hello"), buf.Bytes())
	assert.Equal(t, 5, buf.Len())
	assert.Greater(t, buf.Cap(), 0)

	// Write more data
	buf.Write([]byte(" world"))
	assert.Equal(t, "hello world", buf.String())

	// Reset
	buf.Reset()
	assert.Equal(t, "", buf.String())
	assert.Equal(t, 0, buf.Len())
}

// TestOutputCapturer tests the OutputCapturer
func TestOutputCapturer(t *testing.T) {
	capturer := NewOutputCapturer("test-call")

	// Write to stdout
	capturer.stdout.Write([]byte("stdout output"))
	assert.Equal(t, "stdout output", capturer.Stdout())

	// Write to stderr
	capturer.stderr.Write([]byte("stderr output"))
	assert.Equal(t, "stderr output", capturer.Stderr())

	// Combined output
	combined := capturer.Combined()
	assert.Contains(t, combined, "stdout output")
	assert.Contains(t, combined, "stderr output")

	// Reset
	capturer.Reset()
	assert.Equal(t, "", capturer.Stdout())
	assert.Equal(t, "", capturer.Stderr())
}

// TestStreamingWriter tests line-buffered streaming
func TestStreamingWriter(t *testing.T) {
	var output bytes.Buffer
	writer := NewStreamingWriter(&output)

	// Write partial line (no newline)
	n, err := writer.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "", output.String()) // Not flushed yet

	// Write newline
	n, err = writer.Write([]byte(" world\n"))
	require.NoError(t, err)
	assert.Equal(t, 7, n)
	assert.Equal(t, "hello world\n", output.String())
	assert.Equal(t, 1, writer.LineCount())

	// Write multiple lines at once
	output.Reset()
	writer = NewStreamingWriter(&output)
	writer.Write([]byte("line1\nline2\nline3\n"))
	assert.Equal(t, "line1\nline2\nline3\n", output.String())
	assert.Equal(t, 3, writer.LineCount())

	// Flush remaining buffer
	output.Reset()
	writer = NewStreamingWriter(&output)
	writer.Write([]byte("partial"))
	assert.Equal(t, "", output.String()) // Not flushed yet
	err = writer.Flush()
	require.NoError(t, err)
	assert.Equal(t, "partial", output.String())
}

// TestTeeWriter tests writing to multiple destinations
func TestTeeWriter(t *testing.T) {
	var buf1, buf2, buf3 bytes.Buffer
	writer := TeeWriter(&buf1, &buf2, &buf3)

	data := []byte("test data")
	n, err := writer.Write(data)
	require.NoError(t, err)
	assert.Equal(t, len(data), n)

	// All buffers should have the same data
	assert.Equal(t, "test data", buf1.String())
	assert.Equal(t, "test data", buf2.String())
	assert.Equal(t, "test data", buf3.String())
}

// TestLimitWriter tests writing with size limits
func TestLimitWriter(t *testing.T) {
	var output bytes.Buffer
	writer := NewLimitWriter(&output, 10)

	// Write within limit
	n, err := writer.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", output.String())
	assert.Equal(t, int64(5), writer.Remaining())

	// Write exactly to limit
	n, err = writer.Write([]byte("world"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "helloworld", output.String())
	assert.Equal(t, int64(0), writer.Remaining())

	// Write beyond limit
	n, err = writer.Write([]byte("extra"))
	assert.Error(t, err)
	assert.Equal(t, 0, n)
	assert.Equal(t, "helloworld", output.String()) // No change
}

// TestLimitWriterPartial tests partial writes when approaching limit
func TestLimitWriterPartial(t *testing.T) {
	var output bytes.Buffer
	writer := NewLimitWriter(&output, 7)

	// Write that exceeds limit
	n, err := writer.Write([]byte("hello world"))
	require.NoError(t, err)
	assert.Equal(t, 7, n) // Only 7 bytes written
	assert.Equal(t, "hello w", output.String())
	assert.Equal(t, int64(0), writer.Remaining())
}

// TestConcurrentSyncBuffer tests thread safety of SyncBuffer
func TestConcurrentSyncBuffer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	buf := NewSyncBuffer()
	done := make(chan bool)

	// Start multiple goroutines writing concurrently
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				buf.Write([]byte("x"))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have exactly 1000 bytes
	assert.Equal(t, 1000, buf.Len())
}

// BenchmarkSyncBuffer benchmarks SyncBuffer writes
func BenchmarkSyncBuffer(b *testing.B) {
	buf := NewSyncBuffer()
	data := []byte("benchmark data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Write(data)
	}
}

// BenchmarkStreamingWriter benchmarks StreamingWriter
func BenchmarkStreamingWriter(b *testing.B) {
	var output bytes.Buffer
	writer := NewStreamingWriter(&output)
	data := []byte("benchmark line\n")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer.Write(data)
	}
}
