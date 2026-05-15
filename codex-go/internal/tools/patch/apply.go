package patch

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// ApplyResult contains the results of applying patches.
type ApplyResult struct {
	FilesAffected []string
	Added         []string
	Updated       []string
	Deleted       []string
	Errors        []string
	Summary       string
	DryRun        bool
}

// BackupState stores the original state of files for rollback.
type BackupState struct {
	Path           string
	Content        []byte
	Existed        bool
	Operation      string         // "update", "add", "delete", "move"
	DestPath       string         // Destination path for move operations
	LineEndingInfo LineEndingInfo // Original line ending style for restoration
}

// applyPatches applies a set of patches atomically to the filesystem.
// If any patch fails, all changes are rolled back.
func applyPatches(fs afero.Fs, patches []FilePatch, root string, dryRun bool) (*ApplyResult, error) {
	return applyPatchesWithOptions(fs, patches, root, dryRun, false)
}

// applyPatchesWithOptions applies patches with additional options.
func applyPatchesWithOptions(fs afero.Fs, patches []FilePatch, root string, dryRun bool, allowOutsideRoot bool) (*ApplyResult, error) {
	result := &ApplyResult{
		FilesAffected: []string{},
		Added:         []string{},
		Updated:       []string{},
		Deleted:       []string{},
		Errors:        []string{},
		DryRun:        dryRun,
	}

	// Validate all patches first
	for i := range patches {
		if err := validatePatch(&patches[i]); err != nil {
			result.Errors = append(result.Errors, err.Error())
			return result, err
		}
	}

	// Validate paths and check for path traversal
	for _, patch := range patches {
		files := []string{}
		if patch.OriginalFile != "" {
			files = append(files, patch.OriginalFile)
		}
		if patch.NewFile != "" {
			files = append(files, patch.NewFile)
		}

		for _, file := range files {
			if err := validatePath(root, file, allowOutsideRoot); err != nil {
				result.Errors = append(result.Errors, err.Error())
				return result, err
			}
		}
	}

	// Store backups for rollback
	backups := []BackupState{}

	// Apply each patch
	for _, patch := range patches {
		backup, err := applyPatch(fs, &patch, root, dryRun, &backups)
		if err != nil {
			// Rollback all changes
			rollbackErr := rollbackChanges(fs, backups)
			if rollbackErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("rollback failed: %v", rollbackErr))
			}

			result.Errors = append(result.Errors, err.Error())
			result.Summary = fmt.Sprintf("Failed to apply patches: %v (rolled back)", err)
			return result, err
		}

		if backup != nil {
			backups = append(backups, *backup)
		}

		// Track results
		switch patch.Operation {
		case OperationAdd:
			result.Added = append(result.Added, patch.NewFile)
			result.FilesAffected = append(result.FilesAffected, patch.NewFile)
		case OperationDelete:
			result.Deleted = append(result.Deleted, patch.OriginalFile)
			result.FilesAffected = append(result.FilesAffected, patch.OriginalFile)
		case OperationUpdate:
			result.Updated = append(result.Updated, patch.NewFile)
			result.FilesAffected = append(result.FilesAffected, patch.NewFile)
		case OperationMove:
			result.Updated = append(result.Updated, patch.NewFile)
			result.FilesAffected = append(result.FilesAffected, patch.OriginalFile, patch.NewFile)
		}
	}

	// Generate summary
	result.Summary = generateSummary(result)

	return result, nil
}

// applyPatch applies a single patch to the filesystem.
func applyPatch(fs afero.Fs, patch *FilePatch, root string, dryRun bool, backups *[]BackupState) (*BackupState, error) {
	switch patch.Operation {
	case OperationAdd:
		return applyAddFile(fs, patch, root, dryRun, backups)
	case OperationDelete:
		return applyDeleteFile(fs, patch, root, dryRun, backups)
	case OperationUpdate:
		return applyUpdateFile(fs, patch, root, dryRun, backups)
	case OperationMove:
		return applyMoveFile(fs, patch, root, dryRun, backups)
	default:
		return nil, NewPatchError(ErrorParse, fmt.Sprintf("unknown operation: %v", patch.Operation))
	}
}

// applyAddFile creates a new file.
func applyAddFile(fs afero.Fs, patch *FilePatch, root string, dryRun bool, backups *[]BackupState) (*BackupState, error) {
	fullPath := filepath.Join(root, patch.NewFile)

	// Check if file already exists
	exists, err := afero.Exists(fs, fullPath)
	if err != nil {
		return nil, NewPatchErrorWithFileAndCause(ErrorIO, patch.NewFile, "failed to check if file exists", err)
	}

	backup := &BackupState{
		Path:      fullPath,
		Existed:   exists,
		Operation: "add",
	}

	var existingLineEnding LineEndingInfo
	if exists {
		// Backup existing content and detect line ending style
		content, err := afero.ReadFile(fs, fullPath)
		if err != nil {
			return nil, NewPatchErrorWithFileAndCause(ErrorIO, patch.NewFile, "failed to read existing file", err)
		}
		backup.Content = content
		existingLineEnding = DetectLineEnding(content)
		backup.LineEndingInfo = existingLineEnding
	} else {
		// For new files, default to LF (Unix style)
		existingLineEnding = LineEndingInfo{Style: LineEndingLF}
		backup.LineEndingInfo = existingLineEnding
	}

	// Generate new content from hunks (always in LF format)
	newContent := generateContentFromHunks(patch.Hunks, true)

	// Convert to appropriate line ending style
	newContentBytes := ConvertLineEndings([]byte(newContent), existingLineEnding.Style)

	if !dryRun {
		// Ensure directory exists
		dir := filepath.Dir(fullPath)
		if err := fs.MkdirAll(dir, 0755); err != nil {
			return nil, NewPatchErrorWithFileAndCause(ErrorIO, patch.NewFile, "failed to create directory", err)
		}

		// Write file atomically
		if err := atomicWrite(fs, fullPath, newContentBytes); err != nil {
			return nil, NewPatchErrorWithFileAndCause(ErrorIO, patch.NewFile, "failed to write file", err)
		}
	}

	return backup, nil
}

// applyDeleteFile removes a file.
func applyDeleteFile(fs afero.Fs, patch *FilePatch, root string, dryRun bool, backups *[]BackupState) (*BackupState, error) {
	fullPath := filepath.Join(root, patch.OriginalFile)

	// Check if file exists
	exists, err := afero.Exists(fs, fullPath)
	if err != nil {
		return nil, NewPatchErrorWithFileAndCause(ErrorIO, patch.OriginalFile, "failed to check if file exists", err)
	}

	if !exists {
		return nil, NewPatchErrorWithFile(ErrorFileNotFound, patch.OriginalFile, "file does not exist")
	}

	// Backup content for rollback and detect line ending style
	content, err := afero.ReadFile(fs, fullPath)
	if err != nil {
		return nil, NewPatchErrorWithFileAndCause(ErrorIO, patch.OriginalFile, "failed to read file for backup", err)
	}

	lineEndingInfo := DetectLineEnding(content)

	backup := &BackupState{
		Path:           fullPath,
		Content:        content,
		Existed:        true,
		Operation:      "delete",
		LineEndingInfo: lineEndingInfo,
	}

	if !dryRun {
		if err := fs.Remove(fullPath); err != nil {
			return nil, NewPatchErrorWithFileAndCause(ErrorIO, patch.OriginalFile, "failed to delete file", err)
		}
	}

	return backup, nil
}

// applyUpdateFile modifies an existing file.
func applyUpdateFile(fs afero.Fs, patch *FilePatch, root string, dryRun bool, backups *[]BackupState) (*BackupState, error) {
	fullPath := filepath.Join(root, patch.OriginalFile)

	// Check if file exists
	exists, err := afero.Exists(fs, fullPath)
	if err != nil {
		return nil, NewPatchErrorWithFileAndCause(ErrorIO, patch.OriginalFile, "failed to check if file exists", err)
	}

	if !exists {
		return nil, NewPatchErrorWithFile(ErrorFileNotFound, patch.OriginalFile, "file does not exist")
	}

	// Read current content and detect line ending style
	content, err := afero.ReadFile(fs, fullPath)
	if err != nil {
		return nil, NewPatchErrorWithFileAndCause(ErrorIO, patch.OriginalFile, "failed to read file", err)
	}

	lineEndingInfo := DetectLineEnding(content)

	backup := &BackupState{
		Path:           fullPath,
		Content:        content,
		Existed:        true,
		Operation:      "update",
		LineEndingInfo: lineEndingInfo,
	}

	// Normalize content to LF for patch application
	normalizedContent := NormalizeToLF(content)

	// Apply hunks to normalized content
	newContent, err := applyHunks(string(normalizedContent), patch.Hunks)
	if err != nil {
		return nil, NewPatchErrorWithFileAndCause(ErrorConflict, patch.OriginalFile, "failed to apply hunks", err)
	}

	// Convert back to original line ending style
	finalContent := ConvertLineEndings([]byte(newContent), lineEndingInfo.Style)

	if !dryRun {
		// Write file atomically
		if err := atomicWrite(fs, fullPath, finalContent); err != nil {
			return nil, NewPatchErrorWithFileAndCause(ErrorIO, patch.OriginalFile, "failed to write file", err)
		}
	}

	return backup, nil
}

// applyMoveFile renames/moves a file.
func applyMoveFile(fs afero.Fs, patch *FilePatch, root string, dryRun bool, backups *[]BackupState) (*BackupState, error) {
	oldPath := filepath.Join(root, patch.OriginalFile)
	newPath := filepath.Join(root, patch.NewFile)

	// Check if source file exists
	exists, err := afero.Exists(fs, oldPath)
	if err != nil {
		return nil, NewPatchErrorWithFileAndCause(ErrorIO, patch.OriginalFile, "failed to check if file exists", err)
	}

	if !exists {
		return nil, NewPatchErrorWithFile(ErrorFileNotFound, patch.OriginalFile, "source file does not exist")
	}

	// Read content and detect line ending style
	content, err := afero.ReadFile(fs, oldPath)
	if err != nil {
		return nil, NewPatchErrorWithFileAndCause(ErrorIO, patch.OriginalFile, "failed to read file", err)
	}

	lineEndingInfo := DetectLineEnding(content)

	// Normalize content to LF for patch application
	normalizedContent := NormalizeToLF(content)

	// Apply any content changes from hunks
	newContent := string(normalizedContent)
	if len(patch.Hunks) > 0 {
		newContent, err = applyHunks(string(normalizedContent), patch.Hunks)
		if err != nil {
			return nil, NewPatchErrorWithFileAndCause(ErrorConflict, patch.OriginalFile, "failed to apply hunks during move", err)
		}
	}

	// Convert back to original line ending style
	finalContent := ConvertLineEndings([]byte(newContent), lineEndingInfo.Style)

	backup := &BackupState{
		Path:           oldPath,
		Content:        content,
		Existed:        true,
		Operation:      "move",
		DestPath:       newPath,
		LineEndingInfo: lineEndingInfo,
	}

	if !dryRun {
		// Ensure destination directory exists
		destDir := filepath.Dir(newPath)
		if err := fs.MkdirAll(destDir, 0755); err != nil {
			return nil, NewPatchErrorWithFileAndCause(ErrorIO, patch.NewFile, "failed to create destination directory", err)
		}

		// Write to new location atomically
		if err := atomicWrite(fs, newPath, finalContent); err != nil {
			return nil, NewPatchErrorWithFileAndCause(ErrorIO, patch.NewFile, "failed to write file to new location", err)
		}

		// Remove old file
		if err := fs.Remove(oldPath); err != nil {
			// Try to rollback the new file
			_ = fs.Remove(newPath) // nolint:errcheck // Best effort cleanup
			return nil, NewPatchErrorWithFileAndCause(ErrorIO, patch.OriginalFile, "failed to remove old file", err)
		}
	}

	return backup, nil
}

// applyHunks applies a series of hunks to file content.
func applyHunks(content string, hunks []Hunk) (string, error) {
	lines := strings.Split(content, "\n")
	// Remove trailing empty line if content ended with newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Apply hunks in order
	for _, hunk := range hunks {
		var err error
		lines, err = applyHunk(lines, &hunk)
		if err != nil {
			return "", err
		}
	}

	return strings.Join(lines, "\n") + "\n", nil
}

// applyHunk applies a single hunk to the lines using context-based seeking with fuzzy fallback.
func applyHunk(lines []string, hunk *Hunk) ([]string, error) {
	return applyHunkWithContextAndFuzzy(lines, hunk, DefaultContextMatchConfig(), DefaultFuzzyConfig())
}

// applyHunkWithConfig applies a single hunk to the lines using the specified fuzzy matching config.
// Deprecated: Use applyHunkWithContextAndFuzzy for better context-based seeking.
func applyHunkWithConfig(lines []string, hunk *Hunk, config FuzzyMatchConfig) ([]string, error) {
	return applyHunkWithContextAndFuzzy(lines, hunk, DefaultContextMatchConfig(), config)
}

// applyHunkWithContextAndFuzzy applies a hunk using context-based seeking with fuzzy matching fallback.
// This implements the strategy from Rust's seek_sequence:
//  1. Try exact match at expected position
//  2. Search within a window using multiple strategies (exact, trim whitespace, normalize Unicode)
//  3. Fall back to fuzzy matching if needed
func applyHunkWithContextAndFuzzy(lines []string, hunk *Hunk, contextConfig ContextMatchConfig, fuzzyConfig FuzzyMatchConfig) ([]string, error) {
	// Find where to apply the hunk (hunk line numbers are 1-based)
	expectedStart := hunk.OriginalStart - 1
	if expectedStart < 0 {
		expectedStart = 0
	}

	// Extract the pattern to match (context and remove lines)
	var pattern []string
	for _, hunkLine := range hunk.Lines {
		if hunkLine.Type == LineContext || hunkLine.Type == LineRemove {
			pattern = append(pattern, hunkLine.Content)
		}
	}

	// If no pattern to match (pure addition), append at end
	if len(pattern) == 0 {
		result := make([]string, len(lines))
		copy(result, lines)
		for _, hunkLine := range hunk.Lines {
			if hunkLine.Type == LineAdd {
				result = append(result, hunkLine.Content)
			}
		}
		return result, nil
	}

	// Strategy 1: Try exact match at expected position first
	startLine := expectedStart
	if startLine < len(lines) && startLine+len(pattern) <= len(lines) {
		exactMatch := true
		for i, patternLine := range pattern {
			if lines[startLine+i] != patternLine {
				exactMatch = false
				break
			}
		}
		if exactMatch {
			return applyHunkAtPosition(lines, hunk, startLine, fuzzyConfig)
		}
	}

	// Strategy 2: Context-based seeking with window
	result := seekSequenceWithWindow(lines, pattern, expectedStart, contextConfig, false)
	if result.Found {
		return applyHunkAtPosition(lines, hunk, result.StartLine, fuzzyConfig)
	}

	// Strategy 3: Try from the beginning without window constraints
	result = seekSequence(lines, pattern, 0, false)
	if result.Found {
		return applyHunkAtPosition(lines, hunk, result.StartLine, fuzzyConfig)
	}

	// Strategy 4: Last resort - try fuzzy matching at expected position
	if expectedStart < len(lines) && expectedStart+len(pattern) <= len(lines) {
		fuzzyMatch := true
		for i, patternLine := range pattern {
			if !fuzzyMatchLine(patternLine, lines[expectedStart+i], fuzzyConfig) {
				fuzzyMatch = false
				break
			}
		}
		if fuzzyMatch {
			return applyHunkAtPosition(lines, hunk, expectedStart, fuzzyConfig)
		}
	}

	// All strategies failed - build helpful error message
	contextLines := pattern
	if len(contextLines) > 3 {
		contextLines = pattern[:3]
	}
	return nil, NewPatchError(ErrorConflict,
		fmt.Sprintf("could not find context lines near line %d (expected pattern: %q)",
			expectedStart+1, strings.Join(contextLines, "\\n")))
}

// applyHunkAtPosition applies a hunk at a specific line position with fuzzy matching for verification.
func applyHunkAtPosition(lines []string, hunk *Hunk, startLine int, fuzzyConfig FuzzyMatchConfig) ([]string, error) {
	// Build the new lines
	result := make([]string, 0, len(lines))

	// Copy lines before the hunk
	result = append(result, lines[:startLine]...)

	// Process hunk lines
	lineIndex := startLine
	for _, hunkLine := range hunk.Lines {
		switch hunkLine.Type {
		case LineContext:
			// Verify context matches (should match since we found it with seekSequence)
			if lineIndex >= len(lines) {
				return nil, NewPatchError(ErrorConflict,
					fmt.Sprintf("context line extends beyond file (line %d)", lineIndex+1))
			}
			// Use the original file's version to preserve formatting
			result = append(result, lines[lineIndex])
			lineIndex++

		case LineRemove:
			// Skip the line (don't add to result)
			if lineIndex >= len(lines) {
				return nil, NewPatchError(ErrorConflict,
					fmt.Sprintf("remove line extends beyond file (line %d)", lineIndex+1))
			}
			lineIndex++

		case LineAdd:
			// Add new line
			result = append(result, hunkLine.Content)
			// Don't increment lineIndex as we're not consuming an original line
		}
	}

	// Copy remaining lines after the hunk
	result = append(result, lines[lineIndex:]...)

	return result, nil
}

// generateContentFromHunks generates file content from hunks (for new files).
func generateContentFromHunks(hunks []Hunk, onlyAdded bool) string {
	var lines []string

	for _, hunk := range hunks {
		for _, line := range hunk.Lines {
			if onlyAdded && line.Type == LineAdd {
				lines = append(lines, line.Content)
			} else if !onlyAdded && line.Type != LineRemove {
				lines = append(lines, line.Content)
			}
		}
	}

	return strings.Join(lines, "\n") + "\n"
}

// atomicWrite writes content to a file atomically using temp file + rename.
func atomicWrite(fs afero.Fs, path string, content []byte) error {
	// Write to temp file
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tempFile := filepath.Join(dir, "."+base+".tmp")

	if err := afero.WriteFile(fs, tempFile, content, 0644); err != nil {
		return err
	}

	// Rename temp file to target (atomic on Unix-like systems)
	if err := fs.Rename(tempFile, path); err != nil {
		// Clean up temp file on failure
		_ = fs.Remove(tempFile) // nolint:errcheck // Best effort cleanup
		return err
	}

	return nil
}

// rollbackChanges restores the original state of files.
func rollbackChanges(fs afero.Fs, backups []BackupState) error {
	for i := len(backups) - 1; i >= 0; i-- {
		backup := backups[i]

		switch backup.Operation {
		case "add":
			// Remove the added file, or restore original if it existed
			if backup.Existed {
				// Restore original content
				if err := afero.WriteFile(fs, backup.Path, backup.Content, 0644); err != nil {
					return fmt.Errorf("failed to restore file %s: %w", backup.Path, err)
				}
			} else {
				// Remove the file we added
				if err := fs.Remove(backup.Path); err != nil {
					return fmt.Errorf("failed to remove file %s: %w", backup.Path, err)
				}
			}

		case "delete":
			// Restore the deleted file
			if err := afero.WriteFile(fs, backup.Path, backup.Content, 0644); err != nil {
				return fmt.Errorf("failed to restore deleted file %s: %w", backup.Path, err)
			}

		case "update":
			// Restore original content
			if err := afero.WriteFile(fs, backup.Path, backup.Content, 0644); err != nil {
				return fmt.Errorf("failed to restore file %s: %w", backup.Path, err)
			}

		case "move":
			// Restore original file and remove destination file
			if err := afero.WriteFile(fs, backup.Path, backup.Content, 0644); err != nil {
				return fmt.Errorf("failed to restore moved file %s: %w", backup.Path, err)
			}
			// Remove destination file if it exists
			if backup.DestPath != "" {
				exists, err := afero.Exists(fs, backup.DestPath)
				if err != nil {
					return fmt.Errorf("failed to check if destination file %s exists: %w", backup.DestPath, err)
				}
				if exists {
					if err := fs.Remove(backup.DestPath); err != nil {
						return fmt.Errorf("failed to remove destination file %s: %w", backup.DestPath, err)
					}
				}
			}
		}
	}

	return nil
}

// validatePath validates that a path is within the root and doesn't contain path traversal.
// This uses the enhanced validation from the file package.
func validatePath(root, path string, allowOutsideRoot bool) error {
	if path == "" {
		return nil
	}

	// If we allow outside root, only check for URL-encoded attacks
	// Normal traversal patterns like ../ are acceptable in this mode
	if allowOutsideRoot {
		// Only block URL-encoded attacks which are never legitimate
		lowerPath := strings.ToLower(path)
		if strings.Contains(lowerPath, "%2e") ||
		   strings.Contains(lowerPath, "%2f") ||
		   strings.Contains(lowerPath, "%5c") {
			return NewPatchErrorWithFile(ErrorPathTraversal, path, "suspicious encoded path pattern detected")
		}
		return nil
	}

	// Reject absolute paths that aren't relative to root
	if filepath.IsAbs(path) {
		return NewPatchErrorWithFile(ErrorPathTraversal, path, "absolute paths are not allowed")
	}

	// Use enhanced validation - resolve the path
	fullPath := filepath.Join(root, path)
	cleanPath := filepath.Clean(fullPath)
	cleanRoot := filepath.Clean(root)

	// Check if path escapes root using filepath.Rel
	rel, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil {
		return NewPatchErrorWithFile(ErrorPathTraversal, path, "invalid path")
	}

	// If relative path starts with "..", it's outside root
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return NewPatchErrorWithFile(ErrorPathTraversal, path, "path is outside root directory")
	}

	// Check for symlink safety if the file exists
	if evalPath, err := filepath.EvalSymlinks(cleanPath); err == nil {
		// Verify symlink target is still within root
		rel, err := filepath.Rel(cleanRoot, evalPath)
		if err != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
			return NewPatchErrorWithFile(ErrorPathTraversal, path, "symlink points outside root directory")
		}
	}

	return nil
}

// detectPathTraversal checks for common path traversal patterns.
func detectPathTraversal(path string) bool {
	// Check for obvious traversal patterns
	traversalPatterns := []string{
		"../",
		"..\\",
		"/..",
		"\\..",
	}

	for _, pattern := range traversalPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	return false
}

// generateSummary creates a human-readable summary of the patch results.
func generateSummary(result *ApplyResult) string {
	var parts []string

	if result.DryRun {
		parts = append(parts, "[DRY RUN]")
	}

	if len(result.Added) > 0 {
		parts = append(parts, fmt.Sprintf("Added %d file(s): %s",
			len(result.Added), strings.Join(result.Added, ", ")))
	}

	if len(result.Updated) > 0 {
		parts = append(parts, fmt.Sprintf("Updated %d file(s): %s",
			len(result.Updated), strings.Join(result.Updated, ", ")))
	}

	if len(result.Deleted) > 0 {
		parts = append(parts, fmt.Sprintf("Deleted %d file(s): %s",
			len(result.Deleted), strings.Join(result.Deleted, ", ")))
	}

	if len(result.Errors) > 0 {
		parts = append(parts, fmt.Sprintf("Errors: %s", strings.Join(result.Errors, "; ")))
	}

	if len(parts) == 0 {
		return "No changes applied"
	}

	return strings.Join(parts, ". ")
}
