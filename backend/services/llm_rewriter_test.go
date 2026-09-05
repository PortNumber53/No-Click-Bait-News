package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestArticleRewriterUsesChatCompletionsCompatibleRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q, want /chat/completions", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}

		var req chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "test-model" {
			t.Fatalf("model = %q, want test-model", req.Model)
		}
		if req.Temperature != 0.1 {
			t.Fatalf("temperature = %v, want 0.1", req.Temperature)
		}
		if req.MaxTokens != 500 {
			t.Fatalf("max_tokens = %d, want 500", req.MaxTokens)
		}
		if len(req.Messages) != 2 || !strings.Contains(req.Messages[1].Content, "Original markdown") {
			t.Fatalf("messages = %#v", req.Messages)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "{\"content\":\"# Direct headline\\n\\nConcise rewritten content.\",\"categories\":[\"Business\",\"Technology\",\"Opinion\"]}"
				}
			}]
		}`))
	}))
	defer server.Close()

	rewriter := NewArticleRewriter("test-key", server.URL, "test-model", 0.1, 500, server.Client())
	rewrite, err := rewriter.RewriteArticle(context.Background(), "Title", "https://example.com/story", "# Original")
	if err != nil {
		t.Fatalf("RewriteArticle returned error: %v", err)
	}
	if !strings.Contains(rewrite.Content, "Concise rewritten content.") {
		t.Fatalf("content = %q", rewrite.Content)
	}
	if got, want := strings.Join(rewrite.Categories, ","), "Business,Technology"; got != want {
		t.Fatalf("categories = %q, want %q", got, want)
	}
}

func TestParseArticleRewriteResultStripsMarkdownFenceAndNormalizesCategories(t *testing.T) {
	rewrite, err := parseArticleRewriteResult("```json\n{\"content\":\"Body\",\"categories\":[\" technology \",\"TECHNOLOGY\",\"World\"]}\n```")
	if err != nil {
		t.Fatalf("parseArticleRewriteResult returned error: %v", err)
	}
	if rewrite.Content != "Body" {
		t.Fatalf("content = %q, want Body", rewrite.Content)
	}
	if got, want := strings.Join(rewrite.Categories, ","), "Technology,World"; got != want {
		t.Fatalf("categories = %q, want %q", got, want)
	}
}

func TestParseArticleRewriteResultRemovesHTMLMarkup(t *testing.T) {
	rewrite, err := parseArticleRewriteResult(`{
		"title":"<strong>Direct title</strong>",
		"summary":"<p>Short &amp; factual.</p>",
		"content":"<h2>Update</h2><p>Readable <em>article</em> text.</p><script>ignore()</script>",
		"categories":["World"]
	}`)
	if err != nil {
		t.Fatalf("parseArticleRewriteResult returned error: %v", err)
	}
	if rewrite.Title != "Direct title" {
		t.Fatalf("title = %q, want Direct title", rewrite.Title)
	}
	if rewrite.Summary != "Short & factual." {
		t.Fatalf("summary = %q, want decoded plain text", rewrite.Summary)
	}
	if strings.Contains(rewrite.Content, "<") || strings.Contains(rewrite.Content, "ignore()") {
		t.Fatalf("content retained HTML or script text: %q", rewrite.Content)
	}
	if !strings.Contains(rewrite.Content, "Readable article text.") {
		t.Fatalf("content lost readable text: %q", rewrite.Content)
	}
}

func TestNewArticleRewritersFromEnvSupportsDistinctModelList(t *testing.T) {
	t.Setenv("LLM_API_KEY", "test-key")
	t.Setenv("LLM_MODEL", "")
	t.Setenv("LLM_MODELS", "model-a, model-b, model-a")
	rewriters, err := NewArticleRewritersFromEnv()
	if err != nil {
		t.Fatalf("NewArticleRewritersFromEnv returned error: %v", err)
	}
	if len(rewriters) != 2 {
		t.Fatalf("rewriter count = %d, want 2", len(rewriters))
	}
	if rewriters[0].Model() != "model-a" || rewriters[1].Model() != "model-b" {
		t.Fatalf("models = %q, %q", rewriters[0].Model(), rewriters[1].Model())
	}
	if rewriters[0].httpClient.Timeout != 5*time.Minute {
		t.Fatalf("HTTP timeout = %s, want 5m", rewriters[0].httpClient.Timeout)
	}
}
