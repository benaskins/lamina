package tool

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

const testHTML = `<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<article>
<h1>Test Article</h1>
<p>This is a test article with some meaningful content about technology and science.</p>
<p>It contains multiple paragraphs to ensure readability extraction works properly.</p>
</article>
</body>
</html>`

func TestPageFetcher_FetchPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "axon-tool/1.0" {
			t.Errorf("expected User-Agent axon-tool/1.0, got %s", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, testHTML)
	}))
	defer srv.Close()

	pf := NewPageFetcher(nil)
	result, err := pf.FetchAndExtract(context.Background(), srv.URL, "test question")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result, "technology and science") {
		t.Errorf("expected extracted text to contain article content, got: %s", result)
	}
}

func TestPageFetcher_WithTextGenerator(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, testHTML)
	}))
	defer srv.Close()

	generator := func(ctx context.Context, prompt string) (string, error) {
		if !strings.Contains(prompt, "technology and science") {
			t.Errorf("expected prompt to contain page content")
		}
		return "Extracted: relevant facts about technology", nil
	}

	pf := NewPageFetcher(generator)
	result, err := pf.FetchAndExtract(context.Background(), srv.URL, "technology")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Extracted: relevant facts about technology" {
		t.Errorf("expected generator output, got: %s", result)
	}
}

func TestPageFetcher_WithoutTextGenerator(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, testHTML)
	}))
	defer srv.Close()

	pf := NewPageFetcher(nil)
	result, err := pf.FetchAndExtract(context.Background(), srv.URL, "anything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "technology and science") {
		t.Errorf("expected raw readable text, got: %s", result)
	}
}

func TestPageFetcher_RateLimiting(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, testHTML)
	}))
	defer srv.Close()

	pf := NewPageFetcher(nil)

	// First fetch — should be immediate
	start := time.Now()
	_, err := pf.FetchAndExtract(context.Background(), srv.URL, "q1")
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}
	firstDuration := time.Since(start)

	// Second fetch — should be delayed by at least fetchDelayBetween
	start = time.Now()
	_, err = pf.FetchAndExtract(context.Background(), srv.URL, "q2")
	if err != nil {
		t.Fatalf("second fetch failed: %v", err)
	}
	secondDuration := time.Since(start)

	if requestCount != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount)
	}

	// The first request should be fast (no delay)
	if firstDuration > 500*time.Millisecond {
		t.Errorf("first fetch took too long: %v", firstDuration)
	}

	// The second request should include the rate limit delay
	if secondDuration < 800*time.Millisecond {
		t.Errorf("second fetch was too fast (rate limiting not working): %v", secondDuration)
	}
}

func TestPageFetcher_NonHTMLContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"key": "value"}`)
	}))
	defer srv.Close()

	pf := NewPageFetcher(nil)
	_, err := pf.FetchAndExtract(context.Background(), srv.URL, "question")
	if err == nil {
		t.Fatal("expected error for non-HTML content type")
	}
	if !strings.Contains(err.Error(), "content-type") {
		t.Errorf("expected content-type error, got: %v", err)
	}
}

func TestPageFetcher_InvalidURL(t *testing.T) {
	pf := NewPageFetcher(nil)
	_, err := pf.FetchAndExtract(context.Background(), "ftp://example.com", "question")
	if err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
}

func TestPageFetcher_ConcurrentAccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, testHTML)
	}))
	defer srv.Close()

	pf := NewPageFetcher(nil)

	// Run concurrent fetches to exercise the mutex on lastFetch.
	errs := make(chan error, 3)
	for i := 0; i < 3; i++ {
		go func() {
			_, err := pf.FetchAndExtract(context.Background(), srv.URL, "test")
			errs <- err
		}()
	}
	for i := 0; i < 3; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent fetch %d failed: %v", i, err)
		}
	}
}

func TestTruncateRuneSafe(t *testing.T) {
	// Build a string with multi-byte UTF-8 characters.
	// Each CJK character "世" is 3 bytes. Repeat enough to exceed a small limit.
	input := strings.Repeat("\u4e16\u754c", 100) // "世界" repeated = 600 bytes

	// Simulate the truncation logic from FetchAndExtract.
	maxLen := 50
	text := input
	if len(text) > maxLen {
		text = text[:maxLen]
		for len(text) > 0 && !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
	}

	if !utf8.ValidString(text) {
		t.Errorf("truncated text is not valid UTF-8")
	}
	if len(text) > maxLen {
		t.Errorf("truncated text exceeds maxLen: %d > %d", len(text), maxLen)
	}
	// 50 / 3 = 16 full 3-byte chars = 48 bytes
	if len(text) != 48 {
		t.Errorf("expected 48 bytes after rune-safe truncation, got %d", len(text))
	}
}
