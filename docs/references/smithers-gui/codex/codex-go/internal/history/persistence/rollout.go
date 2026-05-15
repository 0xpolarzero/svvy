package persistence

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/afero"
)

// CreateRollout creates a timestamped snapshot of the history file.
// The rollout file is named "{basename}.{timestamp}" (e.g., history.jsonl.1234567890).
// Returns the path to the created rollout file.
// Rollout files are created with SensitiveFileMode (0600) to maintain the same
// security posture as the original history file.
func CreateRollout(fs afero.Fs, historyPath string) (string, error) {
	// Check if source file exists
	exists, err := afero.Exists(fs, historyPath)
	if err != nil {
		return "", fmt.Errorf("failed to check if file exists: %w", err)
	}
	if !exists {
		return "", fmt.Errorf("history file does not exist: %s", historyPath)
	}

	// Generate rollout filename
	rolloutPath := rolloutFilename(historyPath)

	// Copy the file
	data, err := afero.ReadFile(fs, historyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read history file: %w", err)
	}

	// Write rollout with restricted permissions to protect sensitive data
	if err := afero.WriteFile(fs, rolloutPath, data, SensitiveFileMode); err != nil {
		return "", fmt.Errorf("failed to write rollout file: %w", err)
	}

	return rolloutPath, nil
}

// ListRollouts returns a list of all rollout files for the given history file,
// sorted by timestamp (oldest first).
func ListRollouts(fs afero.Fs, historyPath string) ([]string, error) {
	dir := filepath.Dir(historyPath)
	baseName := filepath.Base(historyPath)

	// Check if directory exists
	exists, err := afero.DirExists(fs, dir)
	if err != nil {
		return nil, fmt.Errorf("failed to check directory: %w", err)
	}
	if !exists {
		return []string{}, nil
	}

	// List files in directory
	files, err := afero.ReadDir(fs, dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var rollouts []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()
		if isRolloutFile(filename, baseName) {
			rollouts = append(rollouts, filepath.Join(dir, filename))
		}
	}

	// Sort by timestamp (oldest first)
	sort.Slice(rollouts, func(i, j int) bool {
		bi := filepath.Base(rollouts[i])
		bj := filepath.Base(rollouts[j])
		ti, errI := parseRolloutTimestamp(bi)
		tj, errJ := parseRolloutTimestamp(bj)
		if errI != nil || errJ != nil {
			// Fallback to lexicographic order if parsing fails
			return bi < bj
		}
		return ti < tj
	})

	return rollouts, nil
}

// DeleteRollout deletes a specific rollout file.
func DeleteRollout(fs afero.Fs, rolloutPath string) error {
	exists, err := afero.Exists(fs, rolloutPath)
	if err != nil {
		return fmt.Errorf("failed to check if rollout exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("rollout file does not exist: %s", rolloutPath)
	}

	if err := fs.Remove(rolloutPath); err != nil {
		return fmt.Errorf("failed to delete rollout: %w", err)
	}

	return nil
}

// CleanupOldRollouts keeps only the most recent N rollouts and deletes the rest.
func CleanupOldRollouts(fs afero.Fs, historyPath string, keepCount int) error {
	rollouts, err := ListRollouts(fs, historyPath)
	if err != nil {
		return err
	}

	// Calculate how many to delete
	deleteCount := len(rollouts) - keepCount
	if deleteCount <= 0 {
		return nil
	}

	// Delete oldest rollouts
	for i := 0; i < deleteCount; i++ {
		if err := DeleteRollout(fs, rollouts[i]); err != nil {
			return fmt.Errorf("failed to delete rollout %s: %w", rollouts[i], err)
		}
	}

	return nil
}

// GetLatestRollout returns the path to the most recent rollout file.
func GetLatestRollout(fs afero.Fs, historyPath string) (string, error) {
	rollouts, err := ListRollouts(fs, historyPath)
	if err != nil {
		return "", err
	}

	if len(rollouts) == 0 {
		return "", fmt.Errorf("no rollouts found for %s", historyPath)
	}

	// Return the last one (newest)
	return rollouts[len(rollouts)-1], nil
}

// rolloutFilename generates a rollout filename with the current timestamp.
func rolloutFilename(basePath string) string {
	timestamp := time.Now().UnixNano()
	return rolloutPathWithTimestamp(basePath, timestamp)
}

// rolloutPathWithTimestamp generates a rollout path with a specific timestamp.
func rolloutPathWithTimestamp(basePath string, timestamp int64) string {
	return fmt.Sprintf("%s.%d", basePath, timestamp)
}

// parseRolloutTimestamp extracts the timestamp from a rollout filename.
func parseRolloutTimestamp(filename string) (int64, error) {
	parts := strings.Split(filename, ".")
	if len(parts) < 3 {
		return 0, fmt.Errorf("invalid rollout filename format: %s", filename)
	}

	// Last part should be the timestamp
	timestampStr := parts[len(parts)-1]
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid timestamp in rollout filename: %s", filename)
	}

	return timestamp, nil
}

// isRolloutFile checks if a filename is a rollout of the given base name.
func isRolloutFile(filename, baseName string) bool {
	// Check if it starts with the base name
	if !strings.HasPrefix(filename, baseName+".") {
		return false
	}

	// Try to parse the timestamp
	_, err := parseRolloutTimestamp(filename)
	return err == nil
}
