package websearch

import (
	"context"
	"time"
)

// SearchResult represents a single result from a web search.
type SearchResult struct {
	// Title is the page title or heading
	Title string `json:"title"`

	// URL is the web address of the result
	URL string `json:"url"`

	// Snippet is a brief excerpt or description
	Snippet string `json:"snippet"`
}

// SearchResponse contains the results of a web search query.
type SearchResponse struct {
	// Query is the search query that was executed
	Query string `json:"query"`

	// Results is the list of search results
	Results []SearchResult `json:"results"`

	// Timestamp records when the search was performed
	Timestamp time.Time `json:"timestamp"`
}

// Provider defines the interface for web search providers.
// Different providers (DuckDuckGo, Google, etc.) implement this interface.
type Provider interface {
	// Search performs a web search and returns results.
	// The context can be used for cancellation and timeout.
	Search(ctx context.Context, query string, maxResults int) (*SearchResponse, error)

	// Name returns the provider name (e.g., "duckduckgo", "google")
	Name() string
}
