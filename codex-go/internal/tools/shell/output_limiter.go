package shell

import (
	"fmt"
	"io"
)

// LimitedWriter wraps an io.Writer with size limits
type LimitedWriter struct {
	writer    io.Writer
	written   int64
	limit     int64
	limitName string
}

// NewLimitedWriter creates a writer that enforces a size limit
func NewLimitedWriter(w io.Writer, limit int64, limitName string) *LimitedWriter {
	return &LimitedWriter{
		writer:    w,
		limit:     limit,
		limitName: limitName,
	}
}

// Write implements io.Writer with size limiting
func (lw *LimitedWriter) Write(p []byte) (n int, err error) {
	if lw.written >= lw.limit {
		// Already at limit, discard
		return len(p), nil
	}

	remaining := lw.limit - lw.written
	toWrite := int64(len(p))

	if toWrite > remaining {
		// Would exceed limit, write what we can
		n, err = lw.writer.Write(p[:remaining])
		lw.written += int64(n)
		if err == nil && lw.written >= lw.limit {
			// Add truncation warning
			warning := fmt.Sprintf("\n[Output truncated: %s limit of %d bytes exceeded]\n", lw.limitName, lw.limit)
			lw.writer.Write([]byte(warning))
		}
		return len(p), nil // Pretend we wrote it all
	}

	n, err = lw.writer.Write(p)
	lw.written += int64(n)
	return n, err
}

// Written returns the number of bytes written
func (lw *LimitedWriter) Written() int64 {
	return lw.written
}
