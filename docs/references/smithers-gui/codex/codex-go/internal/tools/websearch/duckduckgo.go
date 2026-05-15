package websearch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// DuckDuckGoProvider implements web search using DuckDuckGo HTML scraping.
// This is a simple implementation that parses HTML search results.
type DuckDuckGoProvider struct {
	client *http.Client
}

// NewDuckDuckGoProvider creates a new DuckDuckGo search provider.
func NewDuckDuckGoProvider() *DuckDuckGoProvider {
	return &DuckDuckGoProvider{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider name.
func (p *DuckDuckGoProvider) Name() string {
	return "duckduckgo"
}

// Search performs a web search using DuckDuckGo.
func (p *DuckDuckGoProvider) Search(ctx context.Context, query string, maxResults int) (*SearchResponse, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	if maxResults <= 0 {
		maxResults = 10
	}

	// Build DuckDuckGo search URL
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent to avoid blocking
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Codex/1.0)")

	// Execute request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed with status: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse HTML results
	results := p.parseHTML(string(body), maxResults)

	return &SearchResponse{
		Query:     query,
		Results:   results,
		Timestamp: time.Now(),
	}, nil
}

// parseHTML extracts search results from DuckDuckGo HTML.
// This is a simple regex-based parser for the HTML format.
func (p *DuckDuckGoProvider) parseHTML(html string, maxResults int) []SearchResult {
	results := []SearchResult{}

	// DuckDuckGo HTML results are in <div class="result"> elements
	// We use regex to extract title, URL, and snippet

	// Pattern to find result blocks - match more broadly
	resultPattern := regexp.MustCompile(`(?s)<div[^>]*class="result[^"]*"[^>]*>(.*?)</div>\s*</div>`)
	resultMatches := resultPattern.FindAllStringSubmatch(html, -1)

	for _, match := range resultMatches {
		if len(results) >= maxResults {
			break
		}

		if len(match) < 2 {
			continue
		}

		resultHTML := match[1]

		// Extract title and URL from <a class="result__a">
		titlePattern := regexp.MustCompile(`<a[^>]*class="result__a"[^>]*href="([^"]+)"[^>]*>([^<]+)</a>`)
		titleMatch := titlePattern.FindStringSubmatch(resultHTML)

		// Extract snippet from <a class="result__snippet">
		snippetPattern := regexp.MustCompile(`<a[^>]*class="result__snippet"[^>]*>([^<]+)</a>`)
		snippetMatch := snippetPattern.FindStringSubmatch(resultHTML)

		var title, resultURL, snippet string

		if len(titleMatch) >= 3 {
			resultURL = p.decodeURL(titleMatch[1])
			title = strings.TrimSpace(titleMatch[2])
		}

		if len(snippetMatch) >= 2 {
			snippet = strings.TrimSpace(snippetMatch[1])
		}

		// Only add result if we have at least a URL
		if resultURL != "" {
			if title == "" {
				title = resultURL
			}
			results = append(results, SearchResult{
				Title:   title,
				URL:     resultURL,
				Snippet: snippet,
			})
		}
	}

	return results
}

// decodeURL extracts the actual URL from DuckDuckGo's redirect URL.
func (p *DuckDuckGoProvider) decodeURL(ddgURL string) string {
	// DuckDuckGo uses redirect URLs like: //duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com
	if strings.Contains(ddgURL, "uddg=") {
		parts := strings.Split(ddgURL, "uddg=")
		if len(parts) >= 2 {
			decoded, err := url.QueryUnescape(parts[1])
			if err == nil {
				return decoded
			}
		}
	}

	// If not a redirect URL, return as-is (might need http/https prefix)
	if strings.HasPrefix(ddgURL, "//") {
		return "https:" + ddgURL
	}
	if !strings.HasPrefix(ddgURL, "http://") && !strings.HasPrefix(ddgURL, "https://") {
		return "https://" + ddgURL
	}

	return ddgURL
}
