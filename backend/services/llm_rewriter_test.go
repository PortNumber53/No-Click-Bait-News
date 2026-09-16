package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

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
					"content": "{\"content\":\"# Direct headline\\n\\nConcise rewritten content.\",\"categories\":[\"Business\",\"Technology\",\"Opinion\"],\"bias_label\":\"No clear bias\",\"bias_reasoning\":\"The article uses attributed factual language.\"}"
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
	rewrite, err := parseArticleRewriteResult("```json\n{\"content\":\"Body\",\"categories\":[\" technology \",\"TECHNOLOGY\",\"World\"],\"bias_label\":\"source imbalance\",\"bias_reasoning\":\"Only one source is quoted.\"}\n```")
	if err != nil {
		t.Fatalf("parseArticleRewriteResult returned error: %v", err)
	}
	if rewrite.Content != "Body" {
		t.Fatalf("content = %q, want Body", rewrite.Content)
	}
	if got, want := strings.Join(rewrite.Categories, ","), "Technology,World"; got != want {
		t.Fatalf("categories = %q, want %q", got, want)
	}
	if rewrite.BiasLabel != "Source imbalance" {
		t.Fatalf("bias label = %q, want Source imbalance", rewrite.BiasLabel)
	}
}

func TestParseArticleRewriteResultRemovesHTMLMarkup(t *testing.T) {
	rewrite, err := parseArticleRewriteResult(`{
		"title":"<strong>Direct title</strong>",
		"summary":"<p>Short &amp; factual.</p>",
		"content":"<h2>Update</h2><p>Readable <em>article</em> text.</p><script>ignore()</script>",
		"categories":["World"],
		"bias_label":"Loaded or sensational framing",
		"bias_reasoning":"<p>The headline uses <strong>emotionally loaded</strong> wording.</p>"
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
	if strings.Contains(rewrite.BiasReasoning, "<") || rewrite.BiasReasoning != "The headline uses emotionally loaded wording." {
		t.Fatalf("bias reasoning retained HTML: %q", rewrite.BiasReasoning)
	}
}

func TestParseArticleRewriteResultRequiresBiasAssessment(t *testing.T) {
	_, err := parseArticleRewriteResult(`{"content":"Body","categories":["World"]}`)
	if err == nil || !strings.Contains(err.Error(), "bias label") {
		t.Fatalf("error = %v, want missing bias label error", err)
	}
}

func TestNewArticleRewritersFromEnvSupportsDistinctModelList(t *testing.T) {
	t.Setenv("LLM_BATCH_API_KEYS", "batch-key-a, batch-key-b, batch-key-a")
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
	if rewriters[0].Control() != rewriters[1].Control() {
		t.Fatal("configured rewriters do not share rewrite control")
	}
	first, firstSlot, err := rewriters[0].Control().nextAPIKey()
	if err != nil {
		t.Fatalf("first batch key: %v", err)
	}
	second, secondSlot, err := rewriters[0].Control().nextAPIKey()
	if err != nil {
		t.Fatalf("second batch key: %v", err)
	}
	if first != "batch-key-a" || firstSlot != 1 || second != "batch-key-b" || secondSlot != 2 {
		t.Fatalf("round robin keys = (%q, %d), (%q, %d)", first, firstSlot, second, secondSlot)
	}
}

func TestNewArticleRewritersFromEnvDoesNotUseLegacyInteractiveKey(t *testing.T) {
	t.Setenv("LLM_API_KEY", "interactive-key")
	t.Setenv("LLM_BATCH_API_KEYS", "")
	t.Setenv("LLM_MODEL", "model-a")
	t.Setenv("LLM_MODELS", "")
	_, err := NewArticleRewritersFromEnv()
	if err == nil || !strings.Contains(err.Error(), "LLM_BATCH_API_KEYS") {
		t.Fatalf("error = %v, want missing batch key error", err)
	}
}

func TestParseRetryAfterSupportsSecondsAndHTTPDate(t *testing.T) {
	now := time.Date(2026, time.September, 15, 12, 0, 0, 0, time.UTC)
	if got := parseRetryAfter("75", now); got != 75*time.Second {
		t.Fatalf("numeric Retry-After = %s, want 75s", got)
	}
	if got := parseRetryAfter(now.Add(2*time.Minute).Format(http.TimeFormat), now); got != 2*time.Minute {
		t.Fatalf("date Retry-After = %s, want 2m", got)
	}
	if got := parseRetryAfter("not-a-date", now); got != 0 {
		t.Fatalf("invalid Retry-After = %s, want 0", got)
	}
}

func TestArticleRewriterReturnsTyped429AndOpensSharedCircuit(t *testing.T) {
	now := time.Date(2026, time.September, 15, 12, 0, 0, 0, time.UTC)
	control := newRewriteControl([]string{"batch-key"}, 10, time.Minute, time.Minute, 15*time.Minute)
	control.now = func() time.Time { return now }
	control.jitter = func(time.Duration) time.Duration { return 0 }
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("Authorization"); got != "Bearer batch-key" {
			t.Fatalf("Authorization = %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Retry-After": []string{"120"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":"rate limited"}`)),
			Request:    r,
		}, nil
	})}
	rewriter := newArticleRewriter(control, "https://provider.example/v1", "model-a", 0.2, 500, client)

	_, err := rewriter.RewriteArticle(context.Background(), "Title", "https://example.com/story", "Original")
	providerErr, ok := IsLLMRateLimitError(err)
	if !ok {
		t.Fatalf("error = %v, want typed rate limit error", err)
	}
	if providerErr.KeySlot != 1 || providerErr.RetryAfter != 2*time.Minute || !providerErr.RetryAt.Equal(now.Add(2*time.Minute)) {
		t.Fatalf("provider error = %#v", providerErr)
	}
	if _, wait := control.tryReserveJobStart(now); wait != 2*time.Minute {
		t.Fatalf("shared circuit wait = %s, want 2m", wait)
	}
	_, err = rewriter.RewriteArticle(context.Background(), "Another title", "https://example.com/other", "Original")
	providerErr, ok = IsLLMRateLimitError(err)
	if !ok || providerErr.KeySlot != 0 || !providerErr.RetryAt.Equal(now.Add(2*time.Minute)) {
		t.Fatalf("open-circuit error = %#v, %v", providerErr, err)
	}
}

func TestArticleRewriterTransportErrorDoesNotOpenCircuit(t *testing.T) {
	now := time.Date(2026, time.September, 15, 12, 0, 0, 0, time.UTC)
	control := newRewriteControl([]string{"batch-key"}, 10, time.Minute, time.Minute, 15*time.Minute)
	control.now = func() time.Time { return now }
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network unavailable")
	})}
	rewriter := newArticleRewriter(control, "https://provider.example/v1", "model-a", 0.2, 500, client)

	_, err := rewriter.RewriteArticle(context.Background(), "Title", "https://example.com/story", "Original")
	if err == nil || !strings.Contains(err.Error(), "network unavailable") {
		t.Fatalf("error = %v, want transport error", err)
	}
	if control.breakerOpen {
		t.Fatal("transport error opened the rate-limit circuit")
	}
}
