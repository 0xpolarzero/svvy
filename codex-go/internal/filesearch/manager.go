package filesearch

import (
	"context"
	"sync"
	"time"
)

const (
	// DebounceDelay is the delay before executing a search after user input
	DebounceDelay = 100 * time.Millisecond
)

// SearchResultMsg represents a search result message
type SearchResultMsg struct {
	Query   string
	Matches []FileMatch
	Error   error
}

// SearchManager manages debounced file searches
type SearchManager struct {
	searcher     *Searcher
	mu           sync.Mutex
	latestQuery  string
	searchTimer  *time.Timer
	cancelSearch context.CancelFunc
	resultChan   chan SearchResultMsg
}

// NewSearchManager creates a new search manager
func NewSearchManager(searcher *Searcher, resultChan chan SearchResultMsg) *SearchManager {
	return &SearchManager{
		searcher:   searcher,
		resultChan: resultChan,
	}
}

// OnUserQuery handles a new query from the user
// This implements debouncing: rapid queries will be batched together
func (m *SearchManager) OnUserQuery(query string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.latestQuery = query

	// Cancel any pending timer
	if m.searchTimer != nil {
		m.searchTimer.Stop()
	}

	// Cancel previous search if the new query is not a prefix of the old one
	// This optimization allows continuing searches when the user is typing more
	if m.cancelSearch != nil {
		m.cancelSearch()
		m.cancelSearch = nil
	}

	// Schedule a new search after the debounce delay
	m.searchTimer = time.AfterFunc(DebounceDelay, func() {
		m.executeSearch()
	})
}

// executeSearch executes the search for the latest query
func (m *SearchManager) executeSearch() {
	m.mu.Lock()
	query := m.latestQuery
	m.mu.Unlock()

	if query == "" {
		// Empty query, send empty results
		m.resultChan <- SearchResultMsg{
			Query:   query,
			Matches: []FileMatch{},
		}
		return
	}

	// Create a cancellable context
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

	m.mu.Lock()
	m.cancelSearch = cancel
	m.mu.Unlock()

	// Perform the search
	matches, err := m.searcher.Search(ctx, query)

	// Send results
	select {
	case m.resultChan <- SearchResultMsg{
		Query:   query,
		Matches: matches,
		Error:   err,
	}:
	default:
		// Channel full, drop this result
	}

	// Clean up
	m.mu.Lock()
	m.cancelSearch = nil
	m.mu.Unlock()
}

// Cancel cancels any pending or in-progress searches
func (m *SearchManager) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.searchTimer != nil {
		m.searchTimer.Stop()
		m.searchTimer = nil
	}

	if m.cancelSearch != nil {
		m.cancelSearch()
		m.cancelSearch = nil
	}
}
