package persistence

import (
	"testing"

	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/evmts/codex/codex-go/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHistoryReader(t *testing.T) {
	fs := test.NewMemFS(t)

	// Create a file with some data
	test.WriteFileFS(t, fs, "/test/history.jsonl", []byte(`{"id":"1","op":{"type":"interrupt"}}`+"\n"))

	reader, err := NewHistoryReader(fs, "/test/history.jsonl")
	require.NoError(t, err)
	assert.NotNil(t, reader)
	defer reader.Close()
}

func TestHistoryReaderNonExistentFile(t *testing.T) {
	fs := test.NewMemFS(t)

	_, err := NewHistoryReader(fs, "/nonexistent/history.jsonl")
	assert.Error(t, err)
}

func TestHistoryReaderReadAll(t *testing.T) {
	fs := test.NewMemFS(t)

	// Write test data
	writer, err := NewHistoryWriter(fs, "/test/history.jsonl")
	require.NoError(t, err)

	items := []interface{}{
		&protocol.Submission{ID: "1", Op: &protocol.OpInterrupt{}},
		&protocol.Event{ID: "2", Msg: &protocol.EventError{Message: "e1"}},
		&protocol.Submission{ID: "3", Op: &protocol.OpShutdown{}},
	}

	for _, item := range items {
		err := writer.Append(item)
		require.NoError(t, err)
	}
	writer.Close()

	// Read all data
	reader, err := NewHistoryReader(fs, "/test/history.jsonl")
	require.NoError(t, err)
	defer reader.Close()

	submissions, events, err := reader.ReadAll()
	require.NoError(t, err)

	assert.Len(t, submissions, 2)
	assert.Len(t, events, 1)

	assert.Equal(t, "1", submissions[0].ID)
	assert.Equal(t, "3", submissions[1].ID)
	assert.Equal(t, "2", events[0].ID)
}

func TestHistoryReaderReadNext(t *testing.T) {
	fs := test.NewMemFS(t)

	// Write test data
	writer, err := NewHistoryWriter(fs, "/test/history.jsonl")
	require.NoError(t, err)

	err = writer.Append(&protocol.Submission{ID: "1", Op: &protocol.OpInterrupt{}})
	require.NoError(t, err)
	err = writer.Append(&protocol.Event{ID: "2", Msg: &protocol.EventError{Message: "e1"}})
	require.NoError(t, err)
	writer.Close()

	// Read line by line
	reader, err := NewHistoryReader(fs, "/test/history.jsonl")
	require.NoError(t, err)
	defer reader.Close()

	// Read first item
	sub1, evt1, err := reader.ReadNext()
	require.NoError(t, err)
	assert.NotNil(t, sub1)
	assert.Nil(t, evt1)
	assert.Equal(t, "1", sub1.ID)

	// Read second item
	sub2, evt2, err := reader.ReadNext()
	require.NoError(t, err)
	assert.Nil(t, sub2)
	assert.NotNil(t, evt2)
	assert.Equal(t, "2", evt2.ID)

	// EOF
	sub3, evt3, err := reader.ReadNext()
	assert.Nil(t, sub3)
	assert.Nil(t, evt3)
	assert.Error(t, err) // Should be EOF or similar
}

func TestHistoryReaderEmptyFile(t *testing.T) {
	fs := test.NewMemFS(t)

	// Create empty file
	test.WriteFileFS(t, fs, "/test/history.jsonl", []byte(""))

	reader, err := NewHistoryReader(fs, "/test/history.jsonl")
	require.NoError(t, err)
	defer reader.Close()

	submissions, events, err := reader.ReadAll()
	require.NoError(t, err)
	assert.Empty(t, submissions)
	assert.Empty(t, events)
}

func TestHistoryReaderMalformedLine(t *testing.T) {
	fs := test.NewMemFS(t)

	// Write invalid JSON
	data := `{"id":"1","op":{"type":"interrupt"}}
invalid json line
{"id":"3","op":{"type":"shutdown"}}
`
	test.WriteFileFS(t, fs, "/test/history.jsonl", []byte(data))

	reader, err := NewHistoryReader(fs, "/test/history.jsonl")
	require.NoError(t, err)
	defer reader.Close()

	// ReadAll should fail on malformed line
	_, _, err = reader.ReadAll()
	assert.Error(t, err)
}

func TestHistoryReaderSkipEmptyLines(t *testing.T) {
	fs := test.NewMemFS(t)

	// Write data with empty lines
	data := `{"id":"1","op":{"type":"interrupt"}}

{"id":"2","msg":{"type":"error","message":"e1"}}

{"id":"3","op":{"type":"shutdown"}}
`
	test.WriteFileFS(t, fs, "/test/history.jsonl", []byte(data))

	reader, err := NewHistoryReader(fs, "/test/history.jsonl")
	require.NoError(t, err)
	defer reader.Close()

	submissions, events, err := reader.ReadAll()
	require.NoError(t, err)

	// Should skip empty lines
	assert.Len(t, submissions, 2)
	assert.Len(t, events, 1)
}

func TestHistoryReaderPosition(t *testing.T) {
	fs := test.NewMemFS(t)

	// Write test data
	writer, err := NewHistoryWriter(fs, "/test/history.jsonl")
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		err := writer.Append(&protocol.Submission{ID: string(rune('a' + i)), Op: &protocol.OpInterrupt{}})
		require.NoError(t, err)
	}
	writer.Close()

	// Read and track position
	reader, err := NewHistoryReader(fs, "/test/history.jsonl")
	require.NoError(t, err)
	defer reader.Close()

	assert.Equal(t, int64(0), reader.Position())

	// Read first item
	_, _, err = reader.ReadNext()
	require.NoError(t, err)
	assert.Greater(t, reader.Position(), int64(0))

	pos1 := reader.Position()

	// Read second item
	_, _, err = reader.ReadNext()
	require.NoError(t, err)
	assert.Greater(t, reader.Position(), pos1)
}

func TestHistoryReaderLargeFile(t *testing.T) {
	fs := test.NewMemFS(t)

	// Write many items
	writer, err := NewHistoryWriter(fs, "/test/history.jsonl")
	require.NoError(t, err)

	count := 1000
	for i := 0; i < count; i++ {
		if i%2 == 0 {
			err := writer.Append(&protocol.Submission{ID: string(rune(i)), Op: &protocol.OpInterrupt{}})
			require.NoError(t, err)
		} else {
			err := writer.Append(&protocol.Event{ID: string(rune(i)), Msg: &protocol.EventError{Message: "test"}})
			require.NoError(t, err)
		}
	}
	writer.Close()

	// Read all
	reader, err := NewHistoryReader(fs, "/test/history.jsonl")
	require.NoError(t, err)
	defer reader.Close()

	submissions, events, err := reader.ReadAll()
	require.NoError(t, err)

	assert.Len(t, submissions, count/2)
	assert.Len(t, events, count/2)
}

func TestHistoryReaderClose(t *testing.T) {
	fs := test.NewMemFS(t)

	test.WriteFileFS(t, fs, "/test/history.jsonl", []byte(`{"id":"1","op":{"type":"interrupt"}}`+"\n"))

	reader, err := NewHistoryReader(fs, "/test/history.jsonl")
	require.NoError(t, err)

	err = reader.Close()
	require.NoError(t, err)

	// Reading after close should error
	_, _, err = reader.ReadNext()
	assert.Error(t, err)
}

func TestHistoryReaderPath(t *testing.T) {
	fs := test.NewMemFS(t)

	test.WriteFileFS(t, fs, "/test/history.jsonl", []byte(`{"id":"1","op":{"type":"interrupt"}}`+"\n"))

	reader, err := NewHistoryReader(fs, "/test/history.jsonl")
	require.NoError(t, err)
	defer reader.Close()

	assert.Equal(t, "/test/history.jsonl", reader.Path())
}
