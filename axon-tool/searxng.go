package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// SearXNGClient talks to a SearXNG instance.
type SearXNGClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewSearXNGClient creates a client pointing at a SearXNG instance.
func NewSearXNGClient(baseURL string) *SearXNGClient {
	return &SearXNGClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// searxngResult is a single result in the SearXNG JSON response.
type searxngResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// searxngResponse is the JSON response from SearXNG /search endpoint.
type searxngResponse struct {
	Results []searxngResult `json:"results"`
}

// Search queries SearXNG and returns the top N results.
func (c *SearXNGClient) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	u, err := url.Parse(c.baseURL + "/search")
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	q := u.Query()
	q.Set("q", query)
	q.Set("format", "json")
	q.Set("categories", "general")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("searxng request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("searxng returned %d", resp.StatusCode)
	}

	var sr searxngResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := make([]SearchResult, len(sr.Results))
	for i, r := range sr.Results {
		results[i] = SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
		}
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}
