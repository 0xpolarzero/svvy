package patch

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	MaxPatchSize      = 5 * 1024 * 1024 // 5MB
	MaxHunks          = 1000             // Max hunks per patch
	MaxLinesPerHunk   = 50000            // Max lines per hunk
	MaxTotalLines     = 100000           // Max total lines across all hunks
	MaxFilePathLength = 4096             // Max path length
)

// validatePatchPath validates a file path from a patch
// Note: Path traversal validation (../) is done at apply time where AllowOutsideRoot flag is available
func validatePatchPath(path string) error {
	if path == "" {
		return nil // Empty path is okay (used for /dev/null)
	}

	if len(path) > MaxFilePathLength {
		return fmt.Errorf("path exceeds maximum length of %d: %s", MaxFilePathLength, path)
	}

	// Check for null bytes - these are never allowed
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("path contains null byte")
	}

	// Prevent absolute paths - these are security risks
	// (Apply level will resolve relative paths within appropriate root)
	if filepath.IsAbs(path) {
		return fmt.Errorf("absolute paths not allowed in patches: %s", path)
	}

	// Note: We don't check for path traversal (..) here because:
	// 1. The AllowOutsideRoot flag controls this behavior
	// 2. Path traversal validation happens in apply.go with proper context
	// 3. The parser shouldn't enforce policy that depends on execution context

	return nil
}

// validateLineNumber checks for integer overflow and negative numbers
func validateLineNumber(lineNum int, fieldName string) error {
	if lineNum < 0 {
		return fmt.Errorf("%s cannot be negative: %d", fieldName, lineNum)
	}

	// Check for unreasonably large line numbers (likely overflow)
	const MaxReasonableLineNumber = 10_000_000 // 10 million lines
	if lineNum > MaxReasonableLineNumber {
		return fmt.Errorf("%s exceeds reasonable maximum: %d", fieldName, lineNum)
	}

	return nil
}

// validateLineCount validates line counts don't overflow
func validateLineCount(count int, fieldName string) error {
	if count < 0 {
		return fmt.Errorf("%s cannot be negative: %d", fieldName, count)
	}

	if count > MaxLinesPerHunk {
		return fmt.Errorf("%s exceeds maximum of %d: %d", fieldName, MaxLinesPerHunk, count)
	}

	return nil
}

// PatchOperation indicates the type of file operation.
type PatchOperation int

const (
	// OperationAdd indicates a new file is being created.
	OperationAdd PatchOperation = iota

	// OperationDelete indicates a file is being removed.
	OperationDelete

	// OperationUpdate indicates a file is being modified in place.
	OperationUpdate

	// OperationMove indicates a file is being renamed/moved.
	OperationMove
)

// FilePatch represents changes to a single file.
type FilePatch struct {
	OriginalFile string
	NewFile      string
	Operation    PatchOperation
	Hunks        []Hunk
}

// Hunk represents a continuous block of changes within a file.
type Hunk struct {
	OriginalStart int
	OriginalLines int
	NewStart      int
	NewLines      int
	Lines         []Line
}

// Line represents a single line in a hunk.
type Line struct {
	Type    LineType
	Content string
}

// LineType indicates what kind of line this is.
type LineType int

const (
	// LineContext is an unchanged context line.
	LineContext LineType = iota

	// LineAdd is a line being added.
	LineAdd

	// LineRemove is a line being removed.
	LineRemove
)

var (
	// Regular expressions for parsing unified diff format
	fileHeaderRegex = regexp.MustCompile(`^--- (.+)$`)
	newFileRegex    = regexp.MustCompile(`^\+\+\+ (.+)$`)
	hunkHeaderRegex = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)
	binaryFileRegex = regexp.MustCompile(`^Binary files .+ differ$`)
)

// parseUnifiedDiff parses a unified diff format string into FilePatch structures.
func parseUnifiedDiff(diff string) ([]FilePatch, error) {
	if strings.TrimSpace(diff) == "" {
		return nil, NewPatchError(ErrorParse, "empty diff")
	}

	// Check patch size to prevent DoS
	if len(diff) > MaxPatchSize {
		return nil, NewPatchError(ErrorParse, fmt.Sprintf("patch exceeds maximum size of %d bytes", MaxPatchSize))
	}

	lines := strings.Split(diff, "\n")
	var patches []FilePatch
	var currentPatch *FilePatch
	var currentHunk *Hunk

	i := 0
	for i < len(lines) {
		line := lines[i]

		// Check for binary file indicator
		if binaryFileRegex.MatchString(line) {
			return nil, NewPatchError(ErrorParse, "binary files are not supported")
		}

		// Check for file header (---)
		if matches := fileHeaderRegex.FindStringSubmatch(line); matches != nil {
			// Save previous patch if exists
			if currentPatch != nil {
				if currentHunk != nil {
					currentPatch.Hunks = append(currentPatch.Hunks, *currentHunk)
					currentHunk = nil
				}
				patches = append(patches, *currentPatch)
			}

			// Start new patch
			originalPath := cleanPath(matches[1])
			// Validate original file path
			if err := validatePatchPath(originalPath); err != nil {
				return nil, NewPatchError(ErrorParse, fmt.Sprintf("invalid original file path: %v", err))
			}

			currentPatch = &FilePatch{
				OriginalFile: originalPath,
				Hunks:        []Hunk{},
			}

			// Next line should be +++ header
			i++
			if i >= len(lines) {
				return nil, NewPatchError(ErrorParse, "missing +++ header after --- header")
			}

			if matches := newFileRegex.FindStringSubmatch(lines[i]); matches != nil {
				newPath := cleanPath(matches[1])
				// Validate new file path
				if err := validatePatchPath(newPath); err != nil {
					return nil, NewPatchError(ErrorParse, fmt.Sprintf("invalid new file path: %v", err))
				}
				currentPatch.NewFile = newPath
			} else {
				return nil, NewPatchError(ErrorParse, fmt.Sprintf("expected +++ header, got: %s", lines[i]))
			}

			// Determine operation type
			currentPatch.Operation = determineOperation(currentPatch.OriginalFile, currentPatch.NewFile)

			i++
			continue
		}

		// Check for hunk header (@@)
		if matches := hunkHeaderRegex.FindStringSubmatch(line); matches != nil {
			if currentPatch == nil {
				return nil, NewPatchError(ErrorParse, "hunk header without file header")
			}

			// Save previous hunk if exists
			if currentHunk != nil {
				currentPatch.Hunks = append(currentPatch.Hunks, *currentHunk)
			}

			// Check total hunk count
			totalHunks := 0
			for _, patch := range patches {
				totalHunks += len(patch.Hunks)
			}
			if currentPatch != nil {
				totalHunks += len(currentPatch.Hunks)
			}
			totalHunks++ // Add 1 for the hunk we're about to create
			if totalHunks > MaxHunks {
				return nil, NewPatchError(ErrorParse, fmt.Sprintf("patch exceeds maximum of %d hunks", MaxHunks))
			}

			// Parse hunk header with validation
			originalStart, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, NewPatchError(ErrorParse, fmt.Sprintf("invalid original start line number: %v", err))
			}
			if err := validateLineNumber(originalStart, "original start"); err != nil {
				return nil, NewPatchError(ErrorParse, err.Error())
			}

			originalLines := 1
			if matches[2] != "" {
				originalLines, err = strconv.Atoi(matches[2])
				if err != nil {
					return nil, NewPatchError(ErrorParse, fmt.Sprintf("invalid original line count: %v", err))
				}
				if err := validateLineCount(originalLines, "original line count"); err != nil {
					return nil, NewPatchError(ErrorParse, err.Error())
				}
			}

			newStart, err := strconv.Atoi(matches[3])
			if err != nil {
				return nil, NewPatchError(ErrorParse, fmt.Sprintf("invalid new start line number: %v", err))
			}
			if err := validateLineNumber(newStart, "new start"); err != nil {
				return nil, NewPatchError(ErrorParse, err.Error())
			}

			newLines := 1
			if matches[4] != "" {
				newLines, err = strconv.Atoi(matches[4])
				if err != nil {
					return nil, NewPatchError(ErrorParse, fmt.Sprintf("invalid new line count: %v", err))
				}
				if err := validateLineCount(newLines, "new line count"); err != nil {
					return nil, NewPatchError(ErrorParse, err.Error())
				}
			}

			currentHunk = &Hunk{
				OriginalStart: originalStart,
				OriginalLines: originalLines,
				NewStart:      newStart,
				NewLines:      newLines,
				Lines:         []Line{},
			}

			i++
			continue
		}

		// Parse hunk content lines
		if currentHunk != nil && len(line) > 0 {
			// Check total line count to prevent DoS
			totalLines := 0
			for _, patch := range patches {
				for _, hunk := range patch.Hunks {
					totalLines += len(hunk.Lines)
				}
			}
			if currentPatch != nil {
				for _, hunk := range currentPatch.Hunks {
					totalLines += len(hunk.Lines)
				}
			}
			if currentHunk != nil {
				totalLines += len(currentHunk.Lines)
			}
			if totalLines > MaxTotalLines {
				return nil, NewPatchError(ErrorParse, fmt.Sprintf("patch exceeds maximum of %d total lines", MaxTotalLines))
			}

			var lineType LineType
			content := line

			switch line[0] {
			case ' ':
				lineType = LineContext
				content = line[1:]
			case '+':
				lineType = LineAdd
				content = line[1:]
			case '-':
				lineType = LineRemove
				content = line[1:]
			case '\\':
				// Handle "\ No newline at end of file" marker
				i++
				continue
			default:
				// Empty line or end of hunk
				if strings.TrimSpace(line) == "" {
					i++
					continue
				}
			}

			currentHunk.Lines = append(currentHunk.Lines, Line{
				Type:    lineType,
				Content: content,
			})
		}

		i++
	}

	// Save last patch and hunk
	if currentPatch != nil {
		if currentHunk != nil {
			currentPatch.Hunks = append(currentPatch.Hunks, *currentHunk)
		}
		patches = append(patches, *currentPatch)
	}

	if len(patches) == 0 {
		return nil, NewPatchError(ErrorParse, "no valid patches found in diff")
	}

	// Validate that each patch has at least one hunk (unless it's a delete with no hunks)
	for i, patch := range patches {
		if len(patch.Hunks) == 0 && patch.Operation != OperationDelete {
			return nil, NewPatchError(ErrorParse, fmt.Sprintf("patch %d has no hunks", i))
		}
	}

	return patches, nil
}

// cleanPath removes the a/ or b/ prefix and handles /dev/null.
func cleanPath(path string) string {
	path = strings.TrimSpace(path)

	if path == "/dev/null" {
		return ""
	}

	// Remove a/ or b/ prefix
	if strings.HasPrefix(path, "a/") {
		return path[2:]
	}
	if strings.HasPrefix(path, "b/") {
		return path[2:]
	}

	return path
}

// determineOperation determines what operation is being performed based on file paths.
func determineOperation(originalFile, newFile string) PatchOperation {
	if originalFile == "" && newFile != "" {
		return OperationAdd
	}
	if originalFile != "" && newFile == "" {
		return OperationDelete
	}
	if originalFile == newFile {
		return OperationUpdate
	}
	// Different paths = move/rename
	return OperationMove
}

// validatePatch performs validation on a parsed patch.
func validatePatch(patch *FilePatch) error {
	if patch.Operation == OperationAdd && patch.NewFile == "" {
		return NewPatchError(ErrorInvalidHunk, "add operation requires new file path")
	}
	if patch.Operation == OperationDelete && patch.OriginalFile == "" {
		return NewPatchError(ErrorInvalidHunk, "delete operation requires original file path")
	}
	if patch.Operation == OperationUpdate && patch.OriginalFile == "" {
		return NewPatchError(ErrorInvalidHunk, "update operation requires file path")
	}

	for _, hunk := range patch.Hunks {
		if err := validateHunk(&hunk); err != nil {
			return err
		}
	}

	return nil
}

// validateHunk validates a single hunk's line counts match actual lines.
func validateHunk(hunk *Hunk) error {
	addCount := 0
	removeCount := 0
	contextCount := 0

	for _, line := range hunk.Lines {
		switch line.Type {
		case LineAdd:
			addCount++
		case LineRemove:
			removeCount++
		case LineContext:
			contextCount++
		}
	}

	// The counts in the hunk header should match the actual line counts
	expectedOriginalLines := removeCount + contextCount
	expectedNewLines := addCount + contextCount

	if hunk.OriginalLines != expectedOriginalLines {
		return NewPatchError(ErrorInvalidHunk,
			fmt.Sprintf("hunk at line %d: original line count mismatch: header says %d, actual is %d",
				hunk.OriginalStart, hunk.OriginalLines, expectedOriginalLines))
	}

	if hunk.NewLines != expectedNewLines {
		return NewPatchError(ErrorInvalidHunk,
			fmt.Sprintf("hunk at line %d: new line count mismatch: header says %d, actual is %d",
				hunk.NewStart, hunk.NewLines, expectedNewLines))
	}

	return nil
}
