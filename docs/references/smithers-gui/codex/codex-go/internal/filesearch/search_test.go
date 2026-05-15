package filesearch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		query       string
		shouldMatch bool
		minScore    int
	}{
		{
			name:        "exact substring match",
			path:        "src/main.go",
			query:       "main",
			shouldMatch: true,
			minScore:    100,
		},
		{
			name:        "fuzzy match",
			path:        "src/components/Button.tsx",
			query:       "btn",
			shouldMatch: true,
			minScore:    10,
		},
		{
			name:        "no match",
			path:        "src/main.go",
			query:       "xyz",
			shouldMatch: false,
		},
		{
			name:        "case insensitive",
			path:        "src/Main.GO",
			query:       "main.go",
			shouldMatch: true,
			minScore:    100,
		},
		{
			name:        "empty query",
			path:        "src/main.go",
			query:       "",
			shouldMatch: false,
		},
		{
			name:        "prefix match gets bonus",
			path:        "main.go",
			query:       "main",
			shouldMatch: true,
			minScore:    150, // Base + prefix bonus
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, _ := fuzzyMatch(tt.path, tt.query)
			if tt.shouldMatch && score == 0 {
				t.Errorf("expected match but got score 0")
			}
			if !tt.shouldMatch && score > 0 {
				t.Errorf("expected no match but got score %d", score)
			}
			if tt.shouldMatch && score < tt.minScore {
				t.Errorf("score %d is less than minimum %d", score, tt.minScore)
			}
		})
	}
}

func TestSearcher(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	files := []string{
		"main.go",
		"src/app.go",
		"src/components/Button.tsx",
		"src/components/Input.tsx",
		"test/main_test.go",
		"README.md",
		".gitignore",
	}

	for _, file := range files {
		fullPath := filepath.Join(tmpDir, file)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// First test: list all files to verify walking works
	t.Run("list_all_files", func(t *testing.T) {
		searcher, err := NewSearcher(tmpDir, SearchOptions{
			MaxResults:       100,
			RespectGitignore: false,
			IncludeHidden:    true,
			Workers:          2,
		})
		if err != nil {
			t.Fatal(err)
		}

		ctx := context.Background()
		// Use fuzzy match that matches everything by testing with single common letter
		matches, err := searcher.Search(ctx, "a")
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}

		t.Logf("Found %d files with query 'a'", len(matches))
		for _, match := range matches {
			t.Logf("  - %s (score: %d)", match.Path, match.Score)
		}

		if len(matches) == 0 {
			t.Error("expected to find at least some files, got 0")
		}
	})

	searcher, err := NewSearcher(tmpDir, SearchOptions{
		MaxResults:       10,
		RespectGitignore: false,
		IncludeHidden:    true,
		Workers:          2,
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		query        string
		expectAtLeast int
		expectPaths  []string
	}{
		{
			name:          "search for main",
			query:         "main",
			expectAtLeast: 1,
			expectPaths:   []string{"main.go", "test/main_test.go"},
		},
		{
			name:          "search for tsx",
			query:         "tsx",
			expectAtLeast: 2,
			expectPaths:   []string{"src/components/Button.tsx", "src/components/Input.tsx"},
		},
		{
			name:          "search for button",
			query:         "button",
			expectAtLeast: 1,
			expectPaths:   []string{"src/components/Button.tsx"},
		},
		{
			name:          "empty query",
			query:         "",
			expectAtLeast: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			matches, err := searcher.Search(ctx, tt.query)
			if err != nil {
				t.Fatalf("search failed: %v", err)
			}

			if len(matches) < tt.expectAtLeast {
				t.Errorf("expected at least %d matches, got %d", tt.expectAtLeast, len(matches))
			}

			// Check if expected paths are in results
			if tt.expectPaths != nil {
				matchedPaths := make(map[string]bool)
				for _, match := range matches {
					matchedPaths[match.Path] = true
				}

				for _, expectedPath := range tt.expectPaths {
					if !matchedPaths[expectedPath] {
						t.Errorf("expected path %q not found in results", expectedPath)
					}
				}
			}
		})
	}
}

func TestGitignore(t *testing.T) {
	// Create temporary directory with .gitignore
	tmpDir := t.TempDir()

	gitignoreContent := `node_modules/
*.log
build/
.env
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte(gitignoreContent), 0644); err != nil {
		t.Fatal(err)
	}

	files := []string{
		"main.go",
		"debug.log",
		"build/output.js",
		"node_modules/package/index.js",
	}

	for _, file := range files {
		fullPath := filepath.Join(tmpDir, file)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	searcher, err := NewSearcher(tmpDir, SearchOptions{
		MaxResults:       10,
		RespectGitignore: true,
		IncludeHidden:    false,
		Workers:          2,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	matches, err := searcher.Search(ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	// Check that ignored files are not in results
	for _, match := range matches {
		if match.Path == "debug.log" {
			t.Error("expected debug.log to be ignored")
		}
		if match.Path == "build/output.js" {
			t.Error("expected build/output.js to be ignored")
		}
		if filepath.HasPrefix(match.Path, "node_modules/") {
			t.Error("expected node_modules to be ignored")
		}
	}
}

func TestIsIgnored(t *testing.T) {
	patterns := []string{
		"*.log",
		"node_modules",
		"build/",
		".env",
	}

	searcher := &Searcher{options: SearchOptions{RespectGitignore: true}}

	tests := []struct {
		path          string
		shouldIgnore bool
	}{
		{"debug.log", true},
		{"error.log", true},
		{"main.go", false},
		{"node_modules/package/index.js", true},
		{"build/output.js", true},
		{".env", true},
		{"src/main.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			ignored := searcher.isIgnored(tt.path, patterns)
			if ignored != tt.shouldIgnore {
				t.Errorf("isIgnored(%q) = %v, want %v", tt.path, ignored, tt.shouldIgnore)
			}
		})
	}
}
