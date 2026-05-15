package persistence

import (
	"bytes"
	"testing"

	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/evmts/codex/codex-go/test"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHistoryWriter(t *testing.T) {
	fs := test.NewMemFS(t)

	writer, err := NewHistoryWriter(fs, "/test/history.jsonl")
	require.NoError(t, err)
	assert.NotNil(t, writer)
	defer writer.Close()

	// Verify directory was created
	exists, err := afero.DirExists(fs, "/test")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestHistoryWriterAppendSubmission(t *testing.T) {
	fs := test.NewMemFS(t)
	writer, err := NewHistoryWriter(fs, "/test/history.jsonl")
	require.NoError(t, err)
	defer writer.Close()

	submission := &protocol.Submission{
		ID: "test-1",
		Op: &protocol.OpInterrupt{},
	}

	err = writer.Append(submission)
	require.NoError(t, err)

	// Verify file exists and contains data
	data, err := afero.ReadFile(fs, "/test/history.jsonl")
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"test-1"`)
	assert.True(t, bytes.HasSuffix(data, []byte("\n")), "should end with newline")
}

func TestHistoryWriterAppendEvent(t *testing.T) {
	fs := test.NewMemFS(t)
	writer, err := NewHistoryWriter(fs, "/test/history.jsonl")
	require.NoError(t, err)
	defer writer.Close()

	event := &protocol.Event{
		ID: "event-1",
		Msg: &protocol.EventError{
			Message: "test error",
		},
	}

	err = writer.Append(event)
	require.NoError(t, err)

	// Verify file exists and contains data
	data, err := afero.ReadFile(fs, "/test/history.jsonl")
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"event-1"`)
	assert.Contains(t, string(data), `"test error"`)
}

func TestHistoryWriterAppendMultiple(t *testing.T) {
	fs := test.NewMemFS(t)
	writer, err := NewHistoryWriter(fs, "/test/history.jsonl")
	require.NoError(t, err)
	defer writer.Close()

	items := []interface{}{
		&protocol.Submission{ID: "1", Op: &protocol.OpInterrupt{}},
		&protocol.Event{ID: "2", Msg: &protocol.EventError{Message: "e1"}},
		&protocol.Submission{ID: "3", Op: &protocol.OpShutdown{}},
		&protocol.Event{ID: "4", Msg: &protocol.EventTaskStarted{}},
	}

	for _, item := range items {
		err := writer.Append(item)
		require.NoError(t, err)
	}

	// Verify all items are in file
	data, err := afero.ReadFile(fs, "/test/history.jsonl")
	require.NoError(t, err)

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	assert.Len(t, lines, 4, "should have 4 lines")

	// Verify each line is valid JSON
	for i, line := range lines {
		sub, evt, err := UnmarshalHistoryLine(line)
		require.NoError(t, err, "line %d should be valid", i)
		assert.True(t, sub != nil || evt != nil, "line %d should be submission or event", i)
	}
}

func TestHistoryWriterAppendInvalidType(t *testing.T) {
	fs := test.NewMemFS(t)
	writer, err := NewHistoryWriter(fs, "/test/history.jsonl")
	require.NoError(t, err)
	defer writer.Close()

	err = writer.Append("invalid")
	assert.Error(t, err)
}

func TestHistoryWriterFlush(t *testing.T) {
	fs := test.NewMemFS(t)
	writer, err := NewHistoryWriter(fs, "/test/history.jsonl")
	require.NoError(t, err)
	defer writer.Close()

	submission := &protocol.Submission{
		ID: "flush-test",
		Op: &protocol.OpInterrupt{},
	}

	err = writer.Append(submission)
	require.NoError(t, err)

	// Flush should not error
	err = writer.Flush()
	require.NoError(t, err)

	// Data should be written
	data, err := afero.ReadFile(fs, "/test/history.jsonl")
	require.NoError(t, err)
	assert.Contains(t, string(data), "flush-test")
}

func TestHistoryWriterClose(t *testing.T) {
	fs := test.NewMemFS(t)
	writer, err := NewHistoryWriter(fs, "/test/history.jsonl")
	require.NoError(t, err)

	submission := &protocol.Submission{
		ID: "close-test",
		Op: &protocol.OpInterrupt{},
	}

	err = writer.Append(submission)
	require.NoError(t, err)

	// Close should flush and close
	err = writer.Close()
	require.NoError(t, err)

	// Data should be written
	data, err := afero.ReadFile(fs, "/test/history.jsonl")
	require.NoError(t, err)
	assert.Contains(t, string(data), "close-test")

	// Writing after close should error
	err = writer.Append(submission)
	assert.Error(t, err)
}

func TestHistoryWriterConcurrentWrites(t *testing.T) {
	fs := test.NewMemFS(t)
	writer, err := NewHistoryWriter(fs, "/test/history.jsonl")
	require.NoError(t, err)
	defer writer.Close()

	// Write from multiple goroutines
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			submission := &protocol.Submission{
				ID: string(rune('a' + id)),
				Op: &protocol.OpInterrupt{},
			}
			err := writer.Append(submission)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all writes
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all writes succeeded
	data, err := afero.ReadFile(fs, "/test/history.jsonl")
	require.NoError(t, err)

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	assert.Len(t, lines, 10, "should have 10 lines")
}

func TestHistoryWriterResume(t *testing.T) {
	fs := test.NewMemFS(t)

	// Write some data
	writer1, err := NewHistoryWriter(fs, "/test/history.jsonl")
	require.NoError(t, err)

	err = writer1.Append(&protocol.Submission{ID: "1", Op: &protocol.OpInterrupt{}})
	require.NoError(t, err)
	err = writer1.Close()
	require.NoError(t, err)

	// Open again and append more
	writer2, err := NewHistoryWriter(fs, "/test/history.jsonl")
	require.NoError(t, err)
	defer writer2.Close()

	err = writer2.Append(&protocol.Submission{ID: "2", Op: &protocol.OpShutdown{}})
	require.NoError(t, err)

	// Verify both items are in file
	data, err := afero.ReadFile(fs, "/test/history.jsonl")
	require.NoError(t, err)

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	assert.Len(t, lines, 2, "should have 2 lines")
	assert.Contains(t, string(lines[0]), `"id":"1"`)
	assert.Contains(t, string(lines[1]), `"id":"2"`)
}

func TestHistoryWriterNestedDirectory(t *testing.T) {
	fs := test.NewMemFS(t)

	// Should create nested directories
	writer, err := NewHistoryWriter(fs, "/a/b/c/d/history.jsonl")
	require.NoError(t, err)
	defer writer.Close()

	err = writer.Append(&protocol.Submission{ID: "1", Op: &protocol.OpInterrupt{}})
	require.NoError(t, err)

	// Verify file exists
	exists, err := afero.Exists(fs, "/a/b/c/d/history.jsonl")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestHistoryWriterPath(t *testing.T) {
	fs := test.NewMemFS(t)
	writer, err := NewHistoryWriter(fs, "/test/history.jsonl")
	require.NoError(t, err)
	defer writer.Close()

	assert.Equal(t, "/test/history.jsonl", writer.Path())
}

func TestHistoryWriterFilePermissions(t *testing.T) {
	// Use OS filesystem to verify real permissions
	fs := afero.NewOsFs()

	// Create temp directory for test - use a subdirectory so we can control permissions
	tempDir := t.TempDir()
	sessionDir := tempDir + "/test-session"
	historyPath := sessionDir + "/history.jsonl"

	// Create writer - this should create the session directory with 0700
	writer, err := NewHistoryWriter(fs, historyPath)
	require.NoError(t, err)
	defer writer.Close()

	// Write some data
	submission := &protocol.Submission{
		ID: "perm-test",
		Op: &protocol.OpInterrupt{},
	}
	err = writer.Append(submission)
	require.NoError(t, err)

	// Verify file permissions
	info, err := fs.Stat(historyPath)
	require.NoError(t, err)

	// Check file mode is 0600 (owner read/write only)
	mode := info.Mode()
	assert.Equal(t, SensitiveFileMode, mode.Perm(),
		"history file should have 0600 permissions to prevent unauthorized access")

	// Verify directory permissions - check the directory we created
	dirInfo, err := fs.Stat(sessionDir)
	require.NoError(t, err)

	// Directory should have 0700 permissions (owner only)
	dirMode := dirInfo.Mode()
	assert.Equal(t, SensitiveDirMode, dirMode.Perm(),
		"session directory should have 0700 permissions to protect sensitive data")
}
