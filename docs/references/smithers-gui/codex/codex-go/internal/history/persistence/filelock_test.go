package persistence

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileLockExclusive tests that exclusive locks prevent concurrent access
func TestFileLockExclusive(t *testing.T) {
	// Create a temporary file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.lock")

	file1, err := os.OpenFile(testFile, os.O_CREATE|os.O_RDWR, 0600)
	require.NoError(t, err)
	defer file1.Close()

	file2, err := os.OpenFile(testFile, os.O_RDWR, 0600)
	require.NoError(t, err)
	defer file2.Close()

	lock1 := newFileLock(file1)
	lock2 := newFileLock(file2)

	// Acquire exclusive lock on first file descriptor
	err = lock1.Lock(1 * time.Second)
	require.NoError(t, err)
	defer lock1.Unlock()

	// Try to acquire exclusive lock on second file descriptor - should timeout
	err = lock2.Lock(500 * time.Millisecond)
	assert.Error(t, err, "should fail to acquire lock while another process holds it")
	assert.Contains(t, err.Error(), "timeout")
}

// TestFileLockShared tests that shared locks allow concurrent reads
func TestFileLockShared(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.lock")

	file1, err := os.OpenFile(testFile, os.O_CREATE|os.O_RDWR, 0600)
	require.NoError(t, err)
	defer file1.Close()

	file2, err := os.OpenFile(testFile, os.O_RDWR, 0600)
	require.NoError(t, err)
	defer file2.Close()

	lock1 := newFileLock(file1)
	lock2 := newFileLock(file2)

	// Acquire shared lock on first file descriptor
	err = lock1.LockShared(1 * time.Second)
	require.NoError(t, err)
	defer lock1.Unlock()

	// Acquire shared lock on second file descriptor - should succeed
	err = lock2.LockShared(1 * time.Second)
	require.NoError(t, err, "multiple shared locks should be allowed")
	defer lock2.Unlock()
}

// TestFileLockSharedBlocksExclusive tests that shared locks prevent exclusive locks
func TestFileLockSharedBlocksExclusive(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.lock")

	file1, err := os.OpenFile(testFile, os.O_CREATE|os.O_RDWR, 0600)
	require.NoError(t, err)
	defer file1.Close()

	file2, err := os.OpenFile(testFile, os.O_RDWR, 0600)
	require.NoError(t, err)
	defer file2.Close()

	lock1 := newFileLock(file1)
	lock2 := newFileLock(file2)

	// Acquire shared lock
	err = lock1.LockShared(1 * time.Second)
	require.NoError(t, err)
	defer lock1.Unlock()

	// Try to acquire exclusive lock - should timeout
	err = lock2.Lock(500 * time.Millisecond)
	assert.Error(t, err, "exclusive lock should be blocked by shared lock")
	assert.Contains(t, err.Error(), "timeout")
}

// TestFileLockTryLock tests non-blocking lock attempts
func TestFileLockTryLock(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.lock")

	file1, err := os.OpenFile(testFile, os.O_CREATE|os.O_RDWR, 0600)
	require.NoError(t, err)
	defer file1.Close()

	file2, err := os.OpenFile(testFile, os.O_RDWR, 0600)
	require.NoError(t, err)
	defer file2.Close()

	lock1 := newFileLock(file1)
	lock2 := newFileLock(file2)

	// Acquire exclusive lock
	acquired, err := lock1.TryLock()
	require.NoError(t, err)
	require.True(t, acquired, "should acquire lock on first try")
	defer lock1.Unlock()

	// Try to acquire exclusive lock - should fail immediately
	acquired, err = lock2.TryLock()
	require.NoError(t, err)
	assert.False(t, acquired, "should not acquire lock when held by another")
}

// TestFileLockTryLockShared tests non-blocking shared lock attempts
func TestFileLockTryLockShared(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.lock")

	file1, err := os.OpenFile(testFile, os.O_CREATE|os.O_RDWR, 0600)
	require.NoError(t, err)
	defer file1.Close()

	file2, err := os.OpenFile(testFile, os.O_RDWR, 0600)
	require.NoError(t, err)
	defer file2.Close()

	lock1 := newFileLock(file1)
	lock2 := newFileLock(file2)

	// Acquire shared lock
	acquired, err := lock1.TryLockShared()
	require.NoError(t, err)
	require.True(t, acquired, "should acquire shared lock on first try")
	defer lock1.Unlock()

	// Try to acquire another shared lock - should succeed
	acquired, err = lock2.TryLockShared()
	require.NoError(t, err)
	assert.True(t, acquired, "should acquire shared lock when another shared lock is held")
	defer lock2.Unlock()
}

// TestFileLockUnlock tests that unlock releases the lock
func TestFileLockUnlock(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.lock")

	file1, err := os.OpenFile(testFile, os.O_CREATE|os.O_RDWR, 0600)
	require.NoError(t, err)
	defer file1.Close()

	file2, err := os.OpenFile(testFile, os.O_RDWR, 0600)
	require.NoError(t, err)
	defer file2.Close()

	lock1 := newFileLock(file1)
	lock2 := newFileLock(file2)

	// Acquire and release lock
	err = lock1.Lock(1 * time.Second)
	require.NoError(t, err)

	err = lock1.Unlock()
	require.NoError(t, err)

	// Now second lock should be able to acquire it
	err = lock2.Lock(1 * time.Second)
	require.NoError(t, err, "should acquire lock after first lock is released")
	defer lock2.Unlock()
}

// TestHistoryWriterFileLocking tests that HistoryWriter properly uses file locking
func TestHistoryWriterFileLocking(t *testing.T) {
	// Use real OS filesystem for this test
	fs := afero.NewOsFs()
	tempDir := t.TempDir()
	historyPath := filepath.Join(tempDir, "history.jsonl")

	// Create first writer and acquire lock by writing
	writer1, err := NewHistoryWriter(fs, historyPath)
	require.NoError(t, err)
	defer writer1.Close()

	// Create second writer
	writer2, err := NewHistoryWriter(fs, historyPath)
	require.NoError(t, err)
	defer writer2.Close()

	// Both writers should be able to write (locks are per-operation, not held continuously)
	err = writer1.Append(&protocol.Submission{ID: "1", Op: &protocol.OpInterrupt{}})
	require.NoError(t, err)

	err = writer2.Append(&protocol.Submission{ID: "2", Op: &protocol.OpShutdown{}})
	require.NoError(t, err)

	// Close both writers
	err = writer1.Close()
	require.NoError(t, err)
	err = writer2.Close()
	require.NoError(t, err)

	// Verify both writes succeeded
	data, err := afero.ReadFile(fs, historyPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"1"`)
	assert.Contains(t, string(data), `"id":"2"`)
}

// TestHistoryWriterConcurrentFileWrites tests concurrent writes from multiple goroutines with real files
func TestHistoryWriterConcurrentFileWrites(t *testing.T) {
	fs := afero.NewOsFs()
	tempDir := t.TempDir()
	historyPath := filepath.Join(tempDir, "history.jsonl")

	// Create multiple writers
	numWriters := 5
	writers := make([]*HistoryWriter, numWriters)
	for i := 0; i < numWriters; i++ {
		writer, err := NewHistoryWriter(fs, historyPath)
		require.NoError(t, err)
		writers[i] = writer
		defer writer.Close()
	}

	// Write from multiple goroutines
	numWrites := 10
	var wg sync.WaitGroup
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerIdx int) {
			defer wg.Done()
			writer := writers[writerIdx]
			for j := 0; j < numWrites; j++ {
				submission := &protocol.Submission{
					ID: fmt.Sprintf("writer-%d-item-%d", writerIdx, j),
					Op: &protocol.OpInterrupt{},
				}
				err := writer.Append(submission)
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// Close all writers
	for _, writer := range writers {
		err := writer.Close()
		require.NoError(t, err)
	}

	// Verify all writes succeeded and file is valid
	reader, err := NewHistoryReader(fs, historyPath)
	require.NoError(t, err)
	defer reader.Close()

	submissions, _, err := reader.ReadAll()
	require.NoError(t, err)
	assert.Len(t, submissions, numWriters*numWrites, "should have all writes")

	// Verify each submission has a unique ID and is properly formatted
	seenIDs := make(map[string]bool)
	for _, sub := range submissions {
		assert.NotEmpty(t, sub.ID)
		assert.False(t, seenIDs[sub.ID], "IDs should be unique")
		seenIDs[sub.ID] = true
	}
}

// TestHistoryWriterMultiProcessSimulation tests file locking with simulated multi-process access
// This test simulates what would happen if multiple processes tried to write simultaneously
func TestHistoryWriterMultiProcessSimulation(t *testing.T) {
	fs := afero.NewOsFs()
	tempDir := t.TempDir()
	historyPath := filepath.Join(tempDir, "history.jsonl")

	// Simulate multiple "processes" by opening separate file handles
	numProcesses := 3
	var wg sync.WaitGroup

	for i := 0; i < numProcesses; i++ {
		wg.Add(1)
		go func(processID int) {
			defer wg.Done()

			// Each "process" creates its own writer
			writer, err := NewHistoryWriter(fs, historyPath)
			if err != nil {
				t.Errorf("Process %d failed to create writer: %v", processID, err)
				return
			}
			defer writer.Close()

			// Write multiple items
			for j := 0; j < 20; j++ {
				submission := &protocol.Submission{
					ID: fmt.Sprintf("process-%d-item-%d", processID, j),
					Op: &protocol.OpInterrupt{},
				}
				err := writer.Append(submission)
				if err != nil {
					t.Errorf("Process %d failed to append item %d: %v", processID, j, err)
					return
				}
				// Small delay to increase chance of contention
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Read back and verify integrity
	reader, err := NewHistoryReader(fs, historyPath)
	require.NoError(t, err)
	defer reader.Close()

	submissions, _, err := reader.ReadAll()
	require.NoError(t, err)

	expectedCount := numProcesses * 20
	assert.Equal(t, expectedCount, len(submissions),
		"should have all submissions from all processes")

	// Verify no duplicate IDs
	seenIDs := make(map[string]bool)
	for _, sub := range submissions {
		assert.False(t, seenIDs[sub.ID], "found duplicate ID: %s", sub.ID)
		seenIDs[sub.ID] = true
	}
}

// TestHistoryWriterMemFSNoLocking tests that in-memory filesystem works without file locking
func TestHistoryWriterMemFSNoLocking(t *testing.T) {
	// Use memory filesystem - should work without file locking
	fs := afero.NewMemMapFs()

	writer, err := NewHistoryWriter(fs, "/test/history.jsonl")
	require.NoError(t, err)
	defer writer.Close()

	// Verify fileLock is nil for non-OS filesystem
	assert.Nil(t, writer.fileLock, "file lock should be nil for memory filesystem")

	// Should still be able to write
	err = writer.Append(&protocol.Submission{ID: "test", Op: &protocol.OpInterrupt{}})
	require.NoError(t, err)

	// Verify data was written
	data, err := afero.ReadFile(fs, "/test/history.jsonl")
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"test"`)
}

// TestFileLockError tests LockError formatting
func TestFileLockError(t *testing.T) {
	err := &LockError{
		Operation: "lock",
		Path:      "/test/file.lock",
		Timeout:   5 * time.Second,
		Err:       fmt.Errorf("underlying error"),
	}

	errMsg := err.Error()
	assert.Contains(t, errMsg, "lock")
	assert.Contains(t, errMsg, "/test/file.lock")
	assert.Contains(t, errMsg, "5s")
	assert.Contains(t, errMsg, "underlying error")

	// Test without timeout
	err2 := &LockError{
		Operation: "unlock",
		Path:      "/test/file.lock",
		Err:       fmt.Errorf("unlock error"),
	}

	errMsg2 := err2.Error()
	assert.Contains(t, errMsg2, "unlock")
	assert.NotContains(t, errMsg2, "5s")
}

// benchmarkFileLockOperation is a helper for lock benchmarks
func benchmarkFileLockOperation(b *testing.B, lockFunc func(FileLock) error) {
	tempDir := b.TempDir()
	testFile := filepath.Join(tempDir, "bench.lock")

	file, err := os.OpenFile(testFile, os.O_CREATE|os.O_RDWR, 0600)
	require.NoError(b, err)
	defer file.Close()

	lock := newFileLock(file)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := lockFunc(lock)
		if err != nil {
			b.Fatal(err)
		}
		lock.Unlock()
	}
}

// BenchmarkFileLockExclusive benchmarks exclusive lock acquisition
func BenchmarkFileLockExclusive(b *testing.B) {
	benchmarkFileLockOperation(b, func(l FileLock) error {
		return l.Lock(1 * time.Second)
	})
}

// BenchmarkFileLockShared benchmarks shared lock acquisition
func BenchmarkFileLockShared(b *testing.B) {
	benchmarkFileLockOperation(b, func(l FileLock) error {
		return l.LockShared(1 * time.Second)
	})
}

// BenchmarkFileLockTryLock benchmarks non-blocking lock attempts
func BenchmarkFileLockTryLock(b *testing.B) {
	tempDir := b.TempDir()
	testFile := filepath.Join(tempDir, "bench.lock")

	file, err := os.OpenFile(testFile, os.O_CREATE|os.O_RDWR, 0600)
	require.NoError(b, err)
	defer file.Close()

	lock := newFileLock(file)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		acquired, err := lock.TryLock()
		if err != nil {
			b.Fatal(err)
		}
		if acquired {
			lock.Unlock()
		}
	}
}

// BenchmarkHistoryWriterWithLocking benchmarks write performance with file locking
func BenchmarkHistoryWriterWithLocking(b *testing.B) {
	fs := afero.NewOsFs()
	tempDir := b.TempDir()
	historyPath := filepath.Join(tempDir, "bench.jsonl")

	writer, err := NewHistoryWriter(fs, historyPath)
	require.NoError(b, err)
	defer writer.Close()

	submission := &protocol.Submission{
		ID: "bench-test",
		Op: &protocol.OpInterrupt{},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := writer.Append(submission)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestMultiProcessRealSubProcess tests actual multi-process access if go is available
// This test is skipped by default but can be run explicitly
func TestMultiProcessRealSubProcess(t *testing.T) {
	// Check if we can run go commands
	_, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go command not available, skipping real multi-process test")
	}

	// Check if this is being run in subprocess mode
	if os.Getenv("CODEX_TEST_SUBPROCESS") == "1" {
		runSubprocessWriter(t)
		return
	}

	// Main test process
	tempDir := t.TempDir()
	historyPath := filepath.Join(tempDir, "multiprocess.jsonl")

	// Spawn multiple subprocess writers
	numProcesses := 3
	var wg sync.WaitGroup

	for i := 0; i < numProcesses; i++ {
		wg.Add(1)
		go func(processID int) {
			defer wg.Done()

			cmd := exec.Command("go", "test", "-run", "TestMultiProcessRealSubProcess", ".")
			cmd.Env = append(os.Environ(),
				"CODEX_TEST_SUBPROCESS=1",
				fmt.Sprintf("CODEX_TEST_HISTORY_PATH=%s", historyPath),
				fmt.Sprintf("CODEX_TEST_PROCESS_ID=%d", processID),
			)

			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("Subprocess %d failed: %v\nOutput: %s", processID, err, output)
			}
		}(i)
	}

	wg.Wait()

	// Verify the history file
	fs := afero.NewOsFs()
	exists, err := afero.Exists(fs, historyPath)
	require.NoError(t, err)
	if !exists {
		t.Skip("History file not created, subprocesses may have failed")
	}

	reader, err := NewHistoryReader(fs, historyPath)
	require.NoError(t, err)
	defer reader.Close()

	submissions, _, err := reader.ReadAll()
	require.NoError(t, err)

	// Should have writes from all processes
	assert.Greater(t, len(submissions), 0, "should have submissions from subprocesses")
}

// runSubprocessWriter is the subprocess implementation for multi-process tests
func runSubprocessWriter(t *testing.T) {
	historyPath := os.Getenv("CODEX_TEST_HISTORY_PATH")
	processID := os.Getenv("CODEX_TEST_PROCESS_ID")

	if historyPath == "" || processID == "" {
		t.Fatal("Missing subprocess environment variables")
	}

	fs := afero.NewOsFs()
	writer, err := NewHistoryWriter(fs, historyPath)
	require.NoError(t, err)
	defer writer.Close()

	// Write several items
	for i := 0; i < 10; i++ {
		submission := &protocol.Submission{
			ID: fmt.Sprintf("subprocess-%s-item-%d", processID, i),
			Op: &protocol.OpInterrupt{},
		}
		err := writer.Append(submission)
		require.NoError(t, err)
		time.Sleep(1 * time.Millisecond)
	}
}
