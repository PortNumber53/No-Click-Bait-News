package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTinyFishFetchContentDecodesMarkdown(t *testing.T) {
	ttl := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("X-API-Key"); got != "test-key" {
			t.Fatalf("X-API-Key = %q, want test-key", got)
		}

		var req tinyFishFetchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(req.URLs) != 1 || req.URLs[0] != "https://example.com/news" {
			t.Fatalf("URLs = %#v", req.URLs)
		}
		if req.Format != "markdown" {
			t.Fatalf("format = %q, want markdown", req.Format)
		}
		if req.TTL == nil || *req.TTL != 0 {
			t.Fatalf("ttl = %#v, want 0", req.TTL)
		}
		if req.PerURLTimeoutMS != 1234 {
			t.Fatalf("per_url_timeout_ms = %d, want 1234", req.PerURLTimeoutMS)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [{
				"url": "https://example.com/news",
				"final_url": "https://www.example.com/news",
				"title": "Example News",
				"description": "A summary",
				"language": "en",
				"author": "Reporter",
				"published_date": "2026-07-03",
				"text": "# Example News\n\nClean article content.",
				"format": "markdown"
			}],
			"errors": []
		}`))
	}))
	defer server.Close()

	client := NewTinyFishClient("test-key", server.URL, "markdown", &ttl, 1234, server.Client())
	page, err := client.FetchContent(context.Background(), "https://example.com/news")
	if err != nil {
		t.Fatalf("FetchContent returned error: %v", err)
	}

	if page.FinalURL != "https://www.example.com/news" {
		t.Fatalf("FinalURL = %q", page.FinalURL)
	}
	if page.Title == nil || *page.Title != "Example News" {
		t.Fatalf("Title = %#v", page.Title)
	}
	if !strings.Contains(page.Text, "Clean article content.") {
		t.Fatalf("Text = %q", page.Text)
	}
}

func TestTinyFishFetchContentReturnsPerURLError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [],
			"errors": [{
				"url": "https://example.com/news",
				"error": "target_http_error",
				"status": 403
			}]
		}`))
	}))
	defer server.Close()

	client := NewTinyFishClient("test-key", server.URL, "markdown", nil, 1234, server.Client())
	_, err := client.FetchContent(context.Background(), "https://example.com/news")
	if err == nil {
		t.Fatal("FetchContent returned nil error")
	}

	want := "tinyfish fetch failed for https://example.com/news: target_http_error (403)"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestTinyFishFetchContentRejectsTooManyURLs(t *testing.T) {
	client := NewTinyFishClient("test-key", "https://example.invalid", "markdown", nil, 1234, nil)
	urls := []string{
		"https://example.com/1",
		"https://example.com/2",
		"https://example.com/3",
		"https://example.com/4",
		"https://example.com/5",
		"https://example.com/6",
		"https://example.com/7",
		"https://example.com/8",
		"https://example.com/9",
		"https://example.com/10",
		"https://example.com/11",
	}

	_, _, err := client.FetchContents(context.Background(), urls)
	if err == nil || err.Error() != "tinyfish fetch supports at most 10 URLs per request" {
		t.Fatalf("err = %v", err)
	}
}
