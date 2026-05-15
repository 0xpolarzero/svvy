package shell

import (
	"io"
	"sync"

	"github.com/evmts/codex/codex-go/internal/protocol"
)

// BinaryAwareWriter wraps an io.Writer to handle both text and binary output.
// It detects binary data and base64-encodes it, while passing through text as-is.
// This writer is specifically designed for streaming command output via protocol events.
type BinaryAwareWriter struct {
	callID    string
	stream    string // "stdout" or "stderr"
	eventFunc func(*protocol.EventExecCommandOutputDelta) error
	mu        sync.Mutex
}

// NewBinaryAwareWriter creates a writer that detects and encodes binary output.
// callID identifies the command execution
// stream specifies whether this is "stdout" or "stderr"
// eventFunc is called with each output delta event
func NewBinaryAwareWriter(callID, stream string, eventFunc func(*protocol.EventExecCommandOutputDelta) error) *BinaryAwareWriter {
	return &BinaryAwareWriter{
		callID:    callID,
		stream:    stream,
		eventFunc: eventFunc,
	}
}

// Write implements io.Writer interface with binary detection and encoding.
func (w *BinaryAwareWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Detect if data is binary and encode accordingly
	encoded, isBinary := EncodeChunk(p)

	// Create and send the event
	event := &protocol.EventExecCommandOutputDelta{
		CallID:   w.callID,
		Stream:   w.stream,
		Chunk:    encoded,
		IsBinary: isBinary,
	}

	if err := w.eventFunc(event); err != nil {
		return 0, err
	}

	// Always return the original length to satisfy io.Writer contract
	return len(p), nil
}

// ChunkingBinaryWriter buffers and chunks output for efficient streaming.
// It accumulates data up to a chunk size before writing, but also supports flushing.
type ChunkingBinaryWriter struct {
	writer    *BinaryAwareWriter
	buffer    []byte
	chunkSize int
	mu        sync.Mutex
}

// NewChunkingBinaryWriter creates a writer that buffers output into chunks.
func NewChunkingBinaryWriter(callID, stream string, chunkSize int, eventFunc func(*protocol.EventExecCommandOutputDelta) error) *ChunkingBinaryWriter {
	return &ChunkingBinaryWriter{
		writer:    NewBinaryAwareWriter(callID, stream, eventFunc),
		buffer:    make([]byte, 0, chunkSize),
		chunkSize: chunkSize,
	}
}

// Write implements io.Writer interface with buffering.
func (w *ChunkingBinaryWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	originalLen := len(p)

	for len(p) > 0 {
		// Calculate how much we can add to buffer
		spaceLeft := w.chunkSize - len(w.buffer)
		if spaceLeft <= 0 {
			// Buffer is full, flush it
			if err := w.flushLocked(); err != nil {
				return n, err
			}
			spaceLeft = w.chunkSize
		}

		// Take as much as we can fit
		toAdd := len(p)
		if toAdd > spaceLeft {
			toAdd = spaceLeft
		}

		w.buffer = append(w.buffer, p[:toAdd]...)
		p = p[toAdd:]
		n += toAdd

		// If buffer is full, flush it
		if len(w.buffer) >= w.chunkSize {
			if err := w.flushLocked(); err != nil {
				return n, err
			}
		}
	}

	return originalLen, nil
}

// Flush writes any buffered data.
func (w *ChunkingBinaryWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked()
}

// flushLocked flushes the buffer without locking (must be called with lock held).
func (w *ChunkingBinaryWriter) flushLocked() error {
	if len(w.buffer) == 0 {
		return nil
	}

	_, err := w.writer.Write(w.buffer)
	w.buffer = w.buffer[:0] // Reset buffer but keep capacity
	return err
}

// MultiWriterWithFlusher creates a writer that writes to multiple destinations.
// It's similar to io.MultiWriter but also implements Flusher interface.
type MultiWriterWithFlusher struct {
	writers []io.Writer
}

// NewMultiWriterWithFlusher creates a multi-writer that can flush buffered writers.
func NewMultiWriterWithFlusher(writers ...io.Writer) *MultiWriterWithFlusher {
	return &MultiWriterWithFlusher{
		writers: writers,
	}
}

// Write implements io.Writer interface.
func (m *MultiWriterWithFlusher) Write(p []byte) (n int, err error) {
	for _, w := range m.writers {
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

// Flush flushes all writers that implement a Flush() method.
func (m *MultiWriterWithFlusher) Flush() error {
	for _, w := range m.writers {
		if flusher, ok := w.(interface{ Flush() error }); ok {
			if err := flusher.Flush(); err != nil {
				return err
			}
		}
	}
	return nil
}
