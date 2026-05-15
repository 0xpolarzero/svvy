package filesearch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSimpleWalk(t *testing.T) {
	// Create a simple temp directory with one file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create searcher
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

	// Test walking
	fileChan := make(chan string, 10)
	ctx := context.Background()

	go func() {
		searcher.walkFiles(ctx, tmpDir, fileChan)
		close(fileChan)
	}()

	var files []string
	for file := range fileChan {
		files = append(files, file)
		t.Logf("Found file: %s", file)
	}

	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}

	if len(files) > 0 && files[0] != "test.txt" {
		t.Errorf("expected test.txt, got %s", files[0])
	}
}

func TestSimpleSearch(t *testing.T) {
	// Create a simple temp directory with one file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create searcher
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

	// Search
	ctx := context.Background()
	matches, err := searcher.Search(ctx, "test")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	t.Logf("Found %d matches", len(matches))
	for _, match := range matches {
		t.Logf("  - %s (score: %d)", match.Path, match.Score)
	}

	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(matches))
	}
}
