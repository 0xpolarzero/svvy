package persistence

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/protocol"
	"github.com/evmts/codex/codex-go/test"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateRollout(t *testing.T) {
	fs := test.NewMemFS(t)

	// Write some data to main history file
	writer, err := NewHistoryWriter(fs, "/test/history.jsonl")
	require.NoError(t, err)

	err = writer.Append(&protocol.Submission{ID: "1", Op: &protocol.OpInterrupt{}})
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)

	// Create rollout
	rolloutPath, err := CreateRollout(fs, "/test/history.jsonl")
	require.NoError(t, err)
	assert.NotEmpty(t, rolloutPath)

	// Verify rollout file exists
	exists, err := afero.Exists(fs, rolloutPath)
	require.NoError(t, err)
	assert.True(t, exists)

	// Verify rollout has timestamp format
	assert.Contains(t, rolloutPath, "history.jsonl.")
	assert.NotEqual(t, "/test/history.jsonl", rolloutPath)
}

func TestCreateRolloutNonExistentFile(t *testing.T) {
	fs := test.NewMemFS(t)

	_, err := CreateRollout(fs, "/test/nonexistent.jsonl")
	assert.Error(t, err)
}

func TestCreateRolloutEmptyFile(t *testing.T) {
	fs := test.NewMemFS(t)

	// Create empty file
	test.WriteFileFS(t, fs, "/test/history.jsonl", []byte(""))

	rolloutPath, err := CreateRollout(fs, "/test/history.jsonl")
	require.NoError(t, err)

	// Rollout should exist and be empty
	data, err := afero.ReadFile(fs, rolloutPath)
	require.NoError(t, err)
	assert.Empty(t, data)
}

func TestListRollouts(t *testing.T) {
	fs := test.NewMemFS(t)

	// Create main history file
	test.WriteFileFS(t, fs, "/test/history.jsonl", []byte("data"))

	// Create multiple rollouts
	rollout1, err := CreateRollout(fs, "/test/history.jsonl")
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond) // Ensure different timestamps

	rollout2, err := CreateRollout(fs, "/test/history.jsonl")
	require.NoError(t, err)

	// List rollouts
	rollouts, err := ListRollouts(fs, "/test/history.jsonl")
	require.NoError(t, err)

	assert.Len(t, rollouts, 2)
	assert.Contains(t, rollouts, rollout1)
	assert.Contains(t, rollouts, rollout2)
}

func TestListRolloutsNoRollouts(t *testing.T) {
	fs := test.NewMemFS(t)

	// Create main history file
	test.WriteFileFS(t, fs, "/test/history.jsonl", []byte("data"))

	rollouts, err := ListRollouts(fs, "/test/history.jsonl")
	require.NoError(t, err)
	assert.Empty(t, rollouts)
}

func TestListRolloutsNonExistentDirectory(t *testing.T) {
	fs := test.NewMemFS(t)

	rollouts, err := ListRollouts(fs, "/nonexistent/history.jsonl")
	require.NoError(t, err)
	assert.Empty(t, rollouts)
}

func TestDeleteRollout(t *testing.T) {
	fs := test.NewMemFS(t)

	// Create history and rollout
	test.WriteFileFS(t, fs, "/test/history.jsonl", []byte("data"))
	rolloutPath, err := CreateRollout(fs, "/test/history.jsonl")
	require.NoError(t, err)

	// Verify rollout exists
	exists, err := afero.Exists(fs, rolloutPath)
	require.NoError(t, err)
	assert.True(t, exists)

	// Delete rollout
	err = DeleteRollout(fs, rolloutPath)
	require.NoError(t, err)

	// Verify rollout is deleted
	exists, err = afero.Exists(fs, rolloutPath)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestDeleteRolloutNonExistent(t *testing.T) {
	fs := test.NewMemFS(t)

	err := DeleteRollout(fs, "/test/nonexistent.jsonl.123456")
	assert.Error(t, err)
}

func TestCleanupOldRollouts(t *testing.T) {
	fs := test.NewMemFS(t)

	// Create history file
	test.WriteFileFS(t, fs, "/test/history.jsonl", []byte("data"))

	// Create multiple rollouts
	var rollouts []string
	for i := 0; i < 5; i++ {
		rollout, err := CreateRollout(fs, "/test/history.jsonl")
		require.NoError(t, err)
		rollouts = append(rollouts, rollout)
		time.Sleep(10 * time.Millisecond)
	}

	// Keep only 2 newest
	err := CleanupOldRollouts(fs, "/test/history.jsonl", 2)
	require.NoError(t, err)

	// Verify only 2 rollouts remain
	remaining, err := ListRollouts(fs, "/test/history.jsonl")
	require.NoError(t, err)
	assert.Len(t, remaining, 2)

	// Verify the newest 2 are kept
	assert.Contains(t, remaining, rollouts[3])
	assert.Contains(t, remaining, rollouts[4])

	// Verify the oldest are deleted
	for i := 0; i < 3; i++ {
		exists, err := afero.Exists(fs, rollouts[i])
		require.NoError(t, err)
		assert.False(t, exists)
	}
}

func TestCleanupOldRolloutsKeepAll(t *testing.T) {
	fs := test.NewMemFS(t)

	// Create history file
	test.WriteFileFS(t, fs, "/test/history.jsonl", []byte("data"))

	// Create 3 rollouts
	for i := 0; i < 3; i++ {
		_, err := CreateRollout(fs, "/test/history.jsonl")
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
	}

	// Keep 5 (more than exist)
	err := CleanupOldRollouts(fs, "/test/history.jsonl", 5)
	require.NoError(t, err)

	// All should remain
	remaining, err := ListRollouts(fs, "/test/history.jsonl")
	require.NoError(t, err)
	assert.Len(t, remaining, 3)
}

func TestCleanupOldRolloutsKeepNone(t *testing.T) {
	fs := test.NewMemFS(t)

	// Create history file
	test.WriteFileFS(t, fs, "/test/history.jsonl", []byte("data"))

	// Create 3 rollouts
	for i := 0; i < 3; i++ {
		_, err := CreateRollout(fs, "/test/history.jsonl")
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
	}

	// Keep 0
	err := CleanupOldRollouts(fs, "/test/history.jsonl", 0)
	require.NoError(t, err)

	// All should be deleted
	remaining, err := ListRollouts(fs, "/test/history.jsonl")
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

func TestGetLatestRollout(t *testing.T) {
	fs := test.NewMemFS(t)

	// Create history file
	test.WriteFileFS(t, fs, "/test/history.jsonl", []byte("data"))

	// Create multiple rollouts
	var rollouts []string
	for i := 0; i < 3; i++ {
		rollout, err := CreateRollout(fs, "/test/history.jsonl")
		require.NoError(t, err)
		rollouts = append(rollouts, rollout)
		time.Sleep(10 * time.Millisecond)
	}

	// Get latest
	latest, err := GetLatestRollout(fs, "/test/history.jsonl")
	require.NoError(t, err)

	// Should be the last created
	assert.Equal(t, rollouts[2], latest)
}

func TestGetLatestRolloutNoRollouts(t *testing.T) {
	fs := test.NewMemFS(t)

	test.WriteFileFS(t, fs, "/test/history.jsonl", []byte("data"))

	latest, err := GetLatestRollout(fs, "/test/history.jsonl")
	assert.Error(t, err)
	assert.Empty(t, latest)
}

func TestRolloutFilename(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
	}{
		{
			name:     "simple path",
			basePath: "/test/history.jsonl",
		},
		{
			name:     "nested path",
			basePath: "/a/b/c/history.jsonl",
		},
		{
			name:     "different name",
			basePath: "/test/session.jsonl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := rolloutFilename(tt.basePath)
			assert.NotEmpty(t, filename)

			// Should contain base name
			baseName := filepath.Base(tt.basePath)
			assert.Contains(t, filename, baseName)

			// Should have timestamp
			assert.Contains(t, filename, ".")

			// Should be in same directory
			dir := filepath.Dir(tt.basePath)
			fullPath := filepath.Join(dir, filename)
			assert.Contains(t, fullPath, dir)
		})
	}
}

func TestParseRolloutTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{
			name:     "valid rollout",
			filename: "history.jsonl.1234567890",
			wantErr:  false,
		},
		{
			name:     "valid with nanoseconds",
			filename: "history.jsonl.1234567890123456789",
			wantErr:  false,
		},
		{
			name:     "invalid no timestamp",
			filename: "history.jsonl",
			wantErr:  true,
		},
		{
			name:     "invalid format",
			filename: "history.jsonl.abc",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timestamp, err := parseRolloutTimestamp(tt.filename)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Greater(t, timestamp, int64(0))
			}
		})
	}
}

func TestRolloutPathWithTimestamp(t *testing.T) {
	basePath := "/test/history.jsonl"
	timestamp := int64(1234567890)

	rolloutPath := rolloutPathWithTimestamp(basePath, timestamp)
	assert.Equal(t, fmt.Sprintf("/test/history.jsonl.%d", timestamp), rolloutPath)
}

func TestIsRolloutFile(t *testing.T) {
	tests := []struct {
		name       string
		filename   string
		baseName   string
		wantResult bool
	}{
		{
			name:       "valid rollout",
			filename:   "history.jsonl.1234567890",
			baseName:   "history.jsonl",
			wantResult: true,
		},
		{
			name:       "not a rollout",
			filename:   "history.jsonl",
			baseName:   "history.jsonl",
			wantResult: false,
		},
		{
			name:       "different file",
			filename:   "other.jsonl.1234567890",
			baseName:   "history.jsonl",
			wantResult: false,
		},
		{
			name:       "invalid timestamp",
			filename:   "history.jsonl.abc",
			baseName:   "history.jsonl",
			wantResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRolloutFile(tt.filename, tt.baseName)
			assert.Equal(t, tt.wantResult, result)
		})
	}
}

func TestRolloutFilePermissions(t *testing.T) {
	// Use OS filesystem to verify real permissions
	fs := afero.NewOsFs()

	// Create temp directory for test
	tempDir := t.TempDir()
	historyPath := filepath.Join(tempDir, "history.jsonl")

	// Create initial history file
	writer, err := NewHistoryWriter(fs, historyPath)
	require.NoError(t, err)

	err = writer.Append(&protocol.Submission{ID: "rollout-perm-test", Op: &protocol.OpInterrupt{}})
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)

	// Create rollout
	rolloutPath, err := CreateRollout(fs, historyPath)
	require.NoError(t, err)
	require.NotEmpty(t, rolloutPath)

	// Verify rollout file permissions
	info, err := fs.Stat(rolloutPath)
	require.NoError(t, err)

	// Check file mode is 0600 (owner read/write only)
	mode := info.Mode()
	assert.Equal(t, SensitiveFileMode, mode.Perm(),
		"rollout file should have 0600 permissions to protect sensitive conversation history")

	// Verify the rollout file content matches the original
	originalData, err := afero.ReadFile(fs, historyPath)
	require.NoError(t, err)

	rolloutData, err := afero.ReadFile(fs, rolloutPath)
	require.NoError(t, err)

	assert.Equal(t, originalData, rolloutData,
		"rollout should be an exact copy of the history file")
}
