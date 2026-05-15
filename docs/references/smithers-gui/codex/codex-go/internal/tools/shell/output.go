package shell

import (
	"bytes"
	"io"
	"sync"
)

// OutputCapturer captures stdout and stderr from a command execution.
// It provides thread-safe buffering and retrieval of output.
type OutputCapturer struct {
	callID string
	stdout *SyncBuffer
	stderr *SyncBuffer
}

// NewOutputCapturer creates a new output capturer.
func NewOutputCapturer(callID string) *OutputCapturer {
	return &OutputCapturer{
		callID: callID,
		stdout: NewSyncBuffer(),
		stderr: NewSyncBuffer(),
	}
}

// Stdout returns the captured stdout as a string.
func (o *OutputCapturer) Stdout() string {
	return o.stdout.String()
}

// Stderr returns the captured stderr as a string.
func (o *OutputCapturer) Stderr() string {
	return o.stderr.String()
}

// Combined returns stdout and stderr combined.
func (o *OutputCapturer) Combined() string {
	return aggregateOutput(o.Stdout(), o.Stderr())
}

// Reset clears all captured output.
func (o *OutputCapturer) Reset() {
	o.stdout.Reset()
	o.stderr.Reset()
}

// SyncBuffer is a thread-safe buffer that implements io.Writer.
// It wraps bytes.Buffer with a mutex for concurrent access.
type SyncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// NewSyncBuffer creates a new synchronized buffer.
func NewSyncBuffer() *SyncBuffer {
	return &SyncBuffer{}
}

// Write implements io.Writer interface.
func (s *SyncBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

// String returns the buffer contents as a string.
func (s *SyncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// Bytes returns the buffer contents as a byte slice.
func (s *SyncBuffer) Bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Bytes()
}

// Len returns the number of bytes in the buffer.
func (s *SyncBuffer) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Len()
}

// Reset clears the buffer.
func (s *SyncBuffer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.Reset()
}

// Cap returns the capacity of the buffer.
func (s *SyncBuffer) Cap() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Cap()
}

// StreamingWriter wraps an io.Writer to provide line-buffered streaming output.
// This is useful for providing incremental updates during long-running commands.
type StreamingWriter struct {
	writer    io.Writer
	lineBuf   bytes.Buffer
	mu        sync.Mutex
	lineCount int
}

// NewStreamingWriter creates a new streaming writer.
func NewStreamingWriter(writer io.Writer) *StreamingWriter {
	return &StreamingWriter{
		writer: writer,
	}
}

// Write implements io.Writer interface.
// It buffers partial lines and only flushes complete lines.
func (s *StreamingWriter) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	written := 0
	for len(p) > 0 {
		// Find the next newline
		idx := bytes.IndexByte(p, '\n')
		if idx == -1 {
			// No newline found, buffer the rest
			n, err := s.lineBuf.Write(p)
			written += n
			if err != nil {
				return written, err
			}
			break
		}

		// Write the buffered data plus the line (including newline)
		line := p[:idx+1]
		if s.lineBuf.Len() > 0 {
			// Write buffered data first
			_, err := s.writer.Write(s.lineBuf.Bytes())
			if err != nil {
				return written, err
			}
			s.lineBuf.Reset()
		}

		// Write the line
		n, err := s.writer.Write(line)
		written += n
		if err != nil {
			return written, err
		}

		s.lineCount++
		p = p[idx+1:]
	}

	return written, nil
}

// Flush writes any buffered data to the underlying writer.
func (s *StreamingWriter) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lineBuf.Len() > 0 {
		_, err := s.writer.Write(s.lineBuf.Bytes())
		if err != nil {
			return err
		}
		s.lineBuf.Reset()
	}
	return nil
}

// LineCount returns the number of complete lines written.
func (s *StreamingWriter) LineCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lineCount
}

// TeeWriter creates a writer that writes to multiple destinations.
// Similar to io.MultiWriter but with better error handling.
func TeeWriter(writers ...io.Writer) io.Writer {
	return &teeWriter{writers: writers}
}

type teeWriter struct {
	writers []io.Writer
}

func (t *teeWriter) Write(p []byte) (n int, err error) {
	for _, w := range t.writers {
		n, err = w.Write(p)
		if err != nil {
			return n, err
		}
		if n != len(p) {
			return n, io.ErrShortWrite
		}
	}
	return len(p), nil
}

// LimitWriter creates a writer that limits the amount of data written.
// Once the limit is reached, subsequent writes return an error.
type LimitWriter struct {
	writer    io.Writer
	remaining int64
	mu        sync.Mutex
}

// NewLimitWriter creates a new limit writer with the specified maximum size.
func NewLimitWriter(w io.Writer, limit int64) *LimitWriter {
	return &LimitWriter{
		writer:    w,
		remaining: limit,
	}
}

// Write implements io.Writer interface.
func (l *LimitWriter) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.remaining <= 0 {
		return 0, io.ErrShortWrite
	}

	if int64(len(p)) > l.remaining {
		p = p[:l.remaining]
	}

	n, err = l.writer.Write(p)
	l.remaining -= int64(n)
	return n, err
}

// Remaining returns the number of bytes that can still be written.
func (l *LimitWriter) Remaining() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.remaining
}
