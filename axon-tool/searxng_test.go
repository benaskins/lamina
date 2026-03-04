package tool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearXNGClient_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := searxngResponse{
			Results: []searxngResult{
				{Title: "Go Programming", URL: "https://go.dev", Content: "The Go language"},
				{Title: "Go Tutorial", URL: "https://go.dev/tour", Content: "A tour of Go"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewSearXNGClient(srv.URL)
	results, err := client.Search(context.Background(), "go programming", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Title != "Go Programming" {
		t.Errorf("got title %q, want %q", results[0].Title, "Go Programming")
	}
	if results[0].URL != "https://go.dev" {
		t.Errorf("got URL %q, want %q", results[0].URL, "https://go.dev")
	}
	if results[0].Snippet != "The Go language" {
		t.Errorf("got snippet %q, want %q", results[0].Snippet, "The Go language")
	}
}

func TestSearXNGClient_HandlesErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewSearXNGClient(srv.URL)
	_, err := client.Search(context.Background(), "test", 5)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestSearXNGClient_LimitParameter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := searxngResponse{
			Results: []searxngResult{
				{Title: "Result 1", URL: "https://example.com/1", Content: "First"},
				{Title: "Result 2", URL: "https://example.com/2", Content: "Second"},
				{Title: "Result 3", URL: "https://example.com/3", Content: "Third"},
				{Title: "Result 4", URL: "https://example.com/4", Content: "Fourth"},
				{Title: "Result 5", URL: "https://example.com/5", Content: "Fifth"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewSearXNGClient(srv.URL)
	results, err := client.Search(context.Background(), "test", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (limit)", len(results))
	}
	if results[1].Title != "Result 2" {
		t.Errorf("got title %q, want %q", results[1].Title, "Result 2")
	}
}

func TestSearXNGClient_QueryParams(t *testing.T) {
	var receivedQuery string
	var receivedFormat string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.Query().Get("q")
		receivedFormat = r.URL.Query().Get("format")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searxngResponse{})
	}))
	defer srv.Close()

	client := NewSearXNGClient(srv.URL)
	_, _ = client.Search(context.Background(), "hello world", 5)

	if receivedQuery != "hello world" {
		t.Errorf("got query %q, want %q", receivedQuery, "hello world")
	}
	if receivedFormat != "json" {
		t.Errorf("got format %q, want %q", receivedFormat, "json")
	}
}

func TestSearXNGClient_TrailingSlashInBaseURL(t *testing.T) {
	var receivedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searxngResponse{})
	}))
	defer srv.Close()

	// Base URL with trailing slash should not produce double-slash.
	client := NewSearXNGClient(srv.URL + "/")
	_, err := client.Search(context.Background(), "test", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedPath != "/search" {
		t.Errorf("got path %q, want %q (no double slash)", receivedPath, "/search")
	}
}

func TestSearXNGClient_ImplementsSearcher(t *testing.T) {
	var _ Searcher = (*SearXNGClient)(nil)
}
