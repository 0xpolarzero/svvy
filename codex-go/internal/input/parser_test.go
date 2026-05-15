package input

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFileReferences(t *testing.T) {
	// Create temporary directory and files for testing
	tmpDir := t.TempDir()
	testFile1 := filepath.Join(tmpDir, "test1.txt")
	testFile2 := filepath.Join(tmpDir, "file with spaces.txt")
	testDir := filepath.Join(tmpDir, "subdir")
	testFile3 := filepath.Join(testDir, "test3.txt")

	if err := os.WriteFile(testFile1, []byte("content1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile2, []byte("content2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile3, []byte("content3"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		input         string
		workingDir    string
		wantText      string
		wantRefCount  int
		wantError     bool
		wantRefPaths  []string
	}{
		{
			name:         "single file reference",
			input:        "Check @test1.txt for errors",
			workingDir:   tmpDir,
			wantText:     "Check [file: test1.txt] for errors",
			wantRefCount: 1,
			wantRefPaths: []string{testFile1},
		},
		{
			name:         "multiple file references",
			input:        "Compare @test1.txt and @subdir/test3.txt",
			workingDir:   tmpDir,
			wantText:     "Compare [file: test1.txt] and [file: subdir/test3.txt]",
			wantRefCount: 2,
			wantRefPaths: []string{testFile1, testFile3},
		},
		{
			name:         "quoted file with spaces",
			input:        "Review @\"file with spaces.txt\" please",
			workingDir:   tmpDir,
			wantText:     "Review [file: file with spaces.txt] please",
			wantRefCount: 1,
			wantRefPaths: []string{testFile2},
		},
		{
			name:         "no file references",
			input:        "Just regular text with @ symbol but no file",
			workingDir:   tmpDir,
			wantText:     "Just regular text with @ symbol but no file",
			wantRefCount: 0,
		},
		{
			name:         "nonexistent file",
			input:        "Check @nonexistent.txt",
			workingDir:   tmpDir,
			wantText:     "Check @nonexistent.txt",
			wantRefCount: 0,
			wantError:    false, // Should not error, just skip invalid references
		},
		{
			name:         "absolute path",
			input:        "Check @" + testFile1,
			workingDir:   tmpDir,
			wantText:     "Check [file: test1.txt]",
			wantRefCount: 1,
			wantRefPaths: []string{testFile1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseFileReferences(tt.input, tt.workingDir)
			if tt.wantError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if err != nil {
				return
			}

			if result.ProcessedText != tt.wantText {
				t.Errorf("ProcessedText = %q, want %q", result.ProcessedText, tt.wantText)
			}

			if len(result.FileReferences) != tt.wantRefCount {
				t.Errorf("FileReferences count = %d, want %d", len(result.FileReferences), tt.wantRefCount)
			}

			if tt.wantRefPaths != nil {
				for i, ref := range result.FileReferences {
					if i >= len(tt.wantRefPaths) {
						break
					}
					// Resolve expected path symlinks for comparison (e.g., /var -> /private/var on macOS)
					expectedPath := tt.wantRefPaths[i]
					if resolved, err := filepath.EvalSymlinks(expectedPath); err == nil {
						expectedPath = resolved
					}
					if ref.Path != expectedPath {
						t.Errorf("FileReference[%d].Path = %q, want %q", i, ref.Path, expectedPath)
					}
				}
			}
		})
	}
}

func TestGetCurrentToken(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		cursorPos int
		wantToken string
		wantHasAt bool
	}{
		{
			name:      "cursor at end of token",
			input:     "Check @test.txt for errors",
			cursorPos: 15, // After "test.txt"
			wantToken: "test.txt",
			wantHasAt: true,
		},
		{
			name:      "cursor in middle of token",
			input:     "Check @test.txt for errors",
			cursorPos: 10, // In "test"
			wantToken: "test.txt",
			wantHasAt: true,
		},
		{
			name:      "cursor right after @",
			input:     "Check @test.txt for errors",
			cursorPos: 7, // Right after @
			wantToken: "test.txt",
			wantHasAt: true,
		},
		{
			name:      "no @ symbol",
			input:     "Just regular text",
			cursorPos: 10,
			wantToken: "",
			wantHasAt: false,
		},
		{
			name:      "empty token after @",
			input:     "Check @ for errors",
			cursorPos: 7, // Right after @
			wantToken: "",
			wantHasAt: true,
		},
		{
			name:      "quoted path",
			input:     "Check @\"file with spaces.txt\" done",
			cursorPos: 20, // In middle of quoted path
			wantToken: "file with spaces.txt",
			wantHasAt: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, _, hasAt := GetCurrentToken(tt.input, tt.cursorPos)
			if token != tt.wantToken {
				t.Errorf("token = %q, want %q", token, tt.wantToken)
			}
			if hasAt != tt.wantHasAt {
				t.Errorf("hasAt = %v, want %v", hasAt, tt.wantHasAt)
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		path          string
		workingDir    string
		wantAbsPath   string
		wantDisplay   string
		wantError     bool
	}{
		{
			name:        "relative path",
			path:        "test.txt",
			workingDir:  tmpDir,
			wantAbsPath: testFile,
			wantDisplay: "test.txt",
			wantError:   false,
		},
		{
			name:        "absolute path",
			path:        testFile,
			workingDir:  tmpDir,
			wantAbsPath: testFile,
			wantDisplay: "test.txt",
			wantError:   false,
		},
		{
			name:       "nonexistent file",
			path:       "nonexistent.txt",
			workingDir: tmpDir,
			wantError:  true,
		},
		{
			name:       "path traversal attempt",
			path:       "../../../etc/passwd",
			workingDir: tmpDir,
			wantError:  true,
		},
		{
			name:       "empty path",
			path:       "",
			workingDir: tmpDir,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			absPath, display, err := resolvePath(tt.path, tt.workingDir)
			if tt.wantError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if err != nil {
				return
			}

			// Resolve expected path symlinks for comparison (e.g., /var -> /private/var on macOS)
			expectedAbsPath := tt.wantAbsPath
			if resolved, err := filepath.EvalSymlinks(tt.wantAbsPath); err == nil {
				expectedAbsPath = resolved
			}
			if absPath != expectedAbsPath {
				t.Errorf("absPath = %q, want %q", absPath, expectedAbsPath)
			}
			if display != tt.wantDisplay {
				t.Errorf("display = %q, want %q", display, tt.wantDisplay)
			}
		})
	}
}
