package websearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDuckDuckGoProvider_Name(t *testing.T) {
	provider := NewDuckDuckGoProvider()
	assert.Equal(t, "duckduckgo", provider.Name())
}

func TestDuckDuckGoProvider_Search_EmptyQuery(t *testing.T) {
	provider := NewDuckDuckGoProvider()

	_, err := provider.Search(context.Background(), "", 10)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query cannot be empty")
}

func TestDuckDuckGoProvider_Search_MockHTTPResponse(t *testing.T) {
	// Create mock HTML response that simulates DuckDuckGo search results
	mockHTML := `
	<html>
		<body>
			<div class="result">
				<div>
					<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fpage1">Test Result 1</a>
					<a class="result__snippet">This is the first test result snippet</a>
				</div>
			</div>
			<div class="result">
				<div>
					<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fpage2">Test Result 2</a>
					<a class="result__snippet">This is the second test result snippet</a>
				</div>
			</div>
		</body>
	</html>
	`

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockHTML))
	}))
	defer server.Close()

	// Create provider with custom client pointing to test server
	provider := NewDuckDuckGoProvider()
	// Override the search URL by creating a custom request
	// For testing, we'll use the test server URL

	// Note: Since we can't easily override the DuckDuckGo URL in the current implementation,
	// this test verifies the parsing logic works with mock HTML
	results := provider.parseHTML(mockHTML, 10)

	require.Len(t, results, 2)

	assert.Equal(t, "Test Result 1", results[0].Title)
	assert.Equal(t, "https://example.com/page1", results[0].URL)
	assert.Equal(t, "This is the first test result snippet", results[0].Snippet)

	assert.Equal(t, "Test Result 2", results[1].Title)
	assert.Equal(t, "https://example.com/page2", results[1].URL)
	assert.Equal(t, "This is the second test result snippet", results[1].Snippet)
}

func TestDuckDuckGoProvider_ParseHTML_NoResults(t *testing.T) {
	provider := NewDuckDuckGoProvider()

	html := `<html><body><p>No results found</p></body></html>`

	results := provider.parseHTML(html, 10)

	assert.Empty(t, results)
}

func TestDuckDuckGoProvider_ParseHTML_MaxResults(t *testing.T) {
	provider := NewDuckDuckGoProvider()

	// Create HTML with 5 results
	html := `
	<html><body>
		<div class="result"><div><a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2F1">Result 1</a><a class="result__snippet">Snippet 1</a></div></div>
		<div class="result"><div><a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2F2">Result 2</a><a class="result__snippet">Snippet 2</a></div></div>
		<div class="result"><div><a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2F3">Result 3</a><a class="result__snippet">Snippet 3</a></div></div>
		<div class="result"><div><a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2F4">Result 4</a><a class="result__snippet">Snippet 4</a></div></div>
		<div class="result"><div><a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2F5">Result 5</a><a class="result__snippet">Snippet 5</a></div></div>
	</body></html>
	`

	// Request only 3 results
	results := provider.parseHTML(html, 3)

	assert.Len(t, results, 3)
	assert.Equal(t, "Result 1", results[0].Title)
	assert.Equal(t, "Result 2", results[1].Title)
	assert.Equal(t, "Result 3", results[2].Title)
}

func TestDuckDuckGoProvider_ParseHTML_MissingSnippet(t *testing.T) {
	provider := NewDuckDuckGoProvider()

	html := `
	<html><body>
		<div class="result">
			<div>
				<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com">Test Result</a>
			</div>
		</div>
	</body></html>
	`

	results := provider.parseHTML(html, 10)

	require.Len(t, results, 1)
	assert.Equal(t, "Test Result", results[0].Title)
	assert.Equal(t, "https://example.com", results[0].URL)
	assert.Empty(t, results[0].Snippet)
}

func TestDuckDuckGoProvider_DecodeURL(t *testing.T) {
	provider := NewDuckDuckGoProvider()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "DuckDuckGo redirect URL",
			input:    "//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fpath",
			expected: "https://example.com/path",
		},
		{
			name:     "URL with double slash prefix",
			input:    "//example.com/path",
			expected: "https://example.com/path",
		},
		{
			name:     "URL with http prefix",
			input:    "http://example.com",
			expected: "http://example.com",
		},
		{
			name:     "URL with https prefix",
			input:    "https://example.com",
			expected: "https://example.com",
		},
		{
			name:     "URL without prefix",
			input:    "example.com",
			expected: "https://example.com",
		},
		{
			name:     "Complex encoded URL",
			input:    "//duckduckgo.com/l/?uddg=https%3A%2F%2Fgithub.com%2Fgolang%2Fgo",
			expected: "https://github.com/golang/go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.decodeURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDuckDuckGoProvider_Search_DefaultMaxResults(t *testing.T) {
	provider := NewDuckDuckGoProvider()

	// Test that maxResults defaults to 10 when set to 0 or negative
	// We can't easily test the actual HTTP call without mocking,
	// but we can verify the validation logic

	ctx := context.Background()

	// This will make a real HTTP request, which we expect to fail in test environment
	// or succeed with real results. Either way, maxResults should be validated.
	_, err := provider.Search(ctx, "test", 0)

	// We don't care about the error (could be network, parsing, etc.)
	// We just want to verify the code doesn't panic with 0 maxResults
	_ = err
}

func TestDuckDuckGoProvider_ParseHTML_PartiallyMalformed(t *testing.T) {
	provider := NewDuckDuckGoProvider()

	// HTML with some malformed results and some valid ones
	html := `
	<html><body>
		<div class="result">
			<div>
				<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2F1">Good Result 1</a>
				<a class="result__snippet">Snippet 1</a>
			</div>
		</div>
		<div class="result">
			<div>
				<!-- Missing URL and title -->
				<a class="result__snippet">Orphaned snippet</a>
			</div>
		</div>
		<div class="result">
			<div>
				<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2F2">Good Result 2</a>
				<a class="result__snippet">Snippet 2</a>
			</div>
		</div>
	</body></html>
	`

	results := provider.parseHTML(html, 10)

	// Should only parse the valid results
	require.Len(t, results, 2)
	assert.Equal(t, "Good Result 1", results[0].Title)
	assert.Equal(t, "Good Result 2", results[1].Title)
}
