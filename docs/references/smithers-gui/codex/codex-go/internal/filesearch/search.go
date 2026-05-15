package filesearch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// FileMatch represents a file that matches the search query
type FileMatch struct {
	// Path is the relative path from the search directory
	Path string
	// Score is the relevance score (higher is better)
	Score int
	// MatchIndices are the character positions that matched the query
	MatchIndices []int
}

// SearchOptions controls the file search behavior
type SearchOptions struct {
	// MaxResults is the maximum number of results to return
	MaxResults int
	// RespectGitignore controls whether .gitignore files are respected
	RespectGitignore bool
	// IncludeHidden includes hidden files and directories
	IncludeHidden bool
	// Timeout is the maximum duration for the search
	Timeout time.Duration
	// Workers is the number of concurrent workers
	Workers int
}

// DefaultSearchOptions returns sensible defaults
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		MaxResults:       8,
		RespectGitignore: true,
		IncludeHidden:    false,
		Timeout:          2 * time.Second,
		Workers:          2,
	}
}

// Searcher performs fuzzy file searches
type Searcher struct {
	rootDir string
	options SearchOptions
}

// NewSearcher creates a new file searcher
func NewSearcher(rootDir string, options SearchOptions) (*Searcher, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve root directory: %w", err)
	}

	return &Searcher{
		rootDir: absRoot,
		options: options,
	}, nil
}

// Search performs a fuzzy search for files matching the query
func (s *Searcher) Search(ctx context.Context, query string) ([]FileMatch, error) {
	// Create a context with timeout
	searchCtx, cancel := context.WithTimeout(ctx, s.options.Timeout)
	defer cancel()

	// Normalize query for matching
	normalizedQuery := strings.ToLower(query)

	// Channel for collecting matches
	matchChan := make(chan FileMatch, 100)
	var wg sync.WaitGroup

	// Worker goroutines
	fileChan := make(chan string, 100)
	for i := 0; i < s.options.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.matchWorker(searchCtx, normalizedQuery, fileChan, matchChan)
		}()
	}

	// File walker goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(fileChan)
		s.walkFiles(searchCtx, s.rootDir, fileChan)
	}()

	// Collector goroutine
	var matches []FileMatch
	done := make(chan struct{})
	go func() {
		for match := range matchChan {
			matches = append(matches, match)
		}
		close(done)
	}()

	// Wait for workers to complete
	wg.Wait()
	close(matchChan)

	// Wait for collector
	<-done

	// Sort by score (descending) and then by path (ascending)
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].Path < matches[j].Path
	})

	// Limit results
	if len(matches) > s.options.MaxResults {
		matches = matches[:s.options.MaxResults]
	}

	return matches, nil
}

// walkFiles walks the directory tree and sends file paths to the channel
func (s *Searcher) walkFiles(ctx context.Context, root string, fileChan chan<- string) {
	// Load gitignore patterns
	var gitignorePatterns []string
	if s.options.RespectGitignore {
		gitignorePatterns = s.loadGitignorePatterns(root)
	}

	var cancelled bool
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		// Check context cancellation
		select {
		case <-ctx.Done():
			cancelled = true
			return fmt.Errorf("cancelled")
		default:
		}

		if cancelled {
			return fmt.Errorf("cancelled")
		}

		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories we should ignore
		if info.IsDir() {
			// Skip hidden directories unless explicitly included
			if !s.options.IncludeHidden && strings.HasPrefix(info.Name(), ".") && path != root {
				return filepath.SkipDir
			}

			// Skip common directories
			if info.Name() == "node_modules" || info.Name() == ".git" {
				return filepath.SkipDir
			}

			return nil
		}

		// Skip hidden files unless explicitly included
		if !s.options.IncludeHidden && strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Check gitignore
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}

		if s.options.RespectGitignore && s.isIgnored(relPath, gitignorePatterns) {
			return nil
		}

		// Send file path to workers
		select {
		case <-ctx.Done():
			cancelled = true
			return fmt.Errorf("cancelled")
		case fileChan <- relPath:
		}

		return nil
	})
}

// matchWorker processes file paths and checks if they match the query
func (s *Searcher) matchWorker(ctx context.Context, query string, fileChan <-chan string, matchChan chan<- FileMatch) {
	for {
		select {
		case <-ctx.Done():
			return
		case path, ok := <-fileChan:
			if !ok {
				return
			}

			// Perform fuzzy matching
			score, indices := fuzzyMatch(path, query)
			if score > 0 {
				select {
				case <-ctx.Done():
					return
				case matchChan <- FileMatch{
					Path:         path,
					Score:        score,
					MatchIndices: indices,
				}:
				}
			}
		}
	}
}

// fuzzyMatch performs fuzzy matching on a file path
// Returns (score, matchIndices) where score > 0 indicates a match
func fuzzyMatch(path, query string) (int, []int) {
	if query == "" {
		return 0, nil
	}

	pathLower := strings.ToLower(path)
	queryLower := strings.ToLower(query)

	// Simple substring match
	if strings.Contains(pathLower, queryLower) {
		score := 100
		// Bonus for exact basename match
		basename := filepath.Base(pathLower)
		if strings.Contains(basename, queryLower) {
			score += 50
		}
		// Bonus for match at start
		if strings.HasPrefix(pathLower, queryLower) {
			score += 50
		}
		return score, findMatchIndices(pathLower, queryLower)
	}

	// Fuzzy match: all characters of query must appear in order
	indices := make([]int, 0, len(query))
	pathIdx := 0
	score := 0

	for i := 0; i < len(queryLower); i++ {
		found := false
		for pathIdx < len(pathLower) {
			if pathLower[pathIdx] == queryLower[i] {
				indices = append(indices, pathIdx)
				score += 10
				// Bonus for consecutive matches
				if i > 0 && pathIdx > 0 && pathLower[pathIdx-1] == queryLower[i-1] {
					score += 20
				}
				pathIdx++
				found = true
				break
			}
			pathIdx++
		}
		if !found {
			return 0, nil
		}
	}

	// Bonus for matches in basename
	basename := filepath.Base(path)
	if len(indices) > 0 && indices[len(indices)-1] >= len(path)-len(basename) {
		score += 30
	}

	return score, indices
}

// findMatchIndices finds all indices where the query appears in the path
func findMatchIndices(path, query string) []int {
	idx := strings.Index(path, query)
	if idx == -1 {
		return nil
	}

	indices := make([]int, len(query))
	for i := range indices {
		indices[i] = idx + i
	}
	return indices
}

// loadGitignorePatterns loads .gitignore patterns from the root directory
func (s *Searcher) loadGitignorePatterns(root string) []string {
	gitignorePath := filepath.Join(root, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		return nil
	}

	var patterns []string
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	return patterns
}

// isIgnored checks if a path matches any gitignore pattern
func (s *Searcher) isIgnored(path string, patterns []string) bool {
	for _, pattern := range patterns {
		// Simple pattern matching (could be enhanced with proper glob matching)
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		// Remove trailing slash
		pattern = strings.TrimSuffix(pattern, "/")

		// Handle ** patterns (match any directory)
		if strings.Contains(pattern, "**") {
			pattern = strings.ReplaceAll(pattern, "**", "*")
		}

		// Handle directory patterns
		if strings.Contains(path, pattern) {
			return true
		}

		// Handle wildcard patterns
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}

		// Handle directory-level matching
		pathParts := strings.Split(path, string(filepath.Separator))
		for _, part := range pathParts {
			if matched, _ := filepath.Match(pattern, part); matched {
				return true
			}
		}
	}

	return false
}
