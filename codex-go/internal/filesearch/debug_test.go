package filesearch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDebugFileCreation(t *testing.T) {
	tmpDir := t.TempDir()
	t.Logf("Temp dir: %s", tmpDir)

	files := []string{
		"main.go",
		"src/app.go",
	}

	for _, file := range files {
		fullPath := filepath.Join(tmpDir, file)
		t.Logf("Creating file: %s", fullPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}

		// Verify file was created
		if _, err := os.Stat(fullPath); err != nil {
			t.Fatalf("File not created: %v", err)
		}
		t.Logf("File created successfully: %s", fullPath)
	}

	// List all files in tmpDir
	t.Log("Files in tmpDir:")
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(tmpDir, path)
		t.Logf("  %s (isDir: %v)", relPath, info.IsDir())
		return nil
	})

	// Now try to search
	searcher, err := NewSearcher(tmpDir, SearchOptions{
		MaxResults:       10,
		RespectGitignore: false,
		IncludeHidden:    false,
		Workers:          1,
		Timeout:          5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	matches, err := searcher.Search(ctx, "main")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	t.Logf("Found %d matches for 'main'", len(matches))
	for _, match := range matches {
		t.Logf("  - %s (score: %d)", match.Path, match.Score)
	}

	if len(matches) == 0 {
		t.Error("expected at least 1 match")
	}
}
