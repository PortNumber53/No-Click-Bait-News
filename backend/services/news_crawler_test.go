package services

import (
	"net/url"
	"reflect"
	"testing"
)

func TestInterleaveFeedArticlesUsesRoundRobinOrder(t *testing.T) {
	feeds := [][]feedArticle{
		{{URL: "a1"}, {URL: "a2"}, {URL: "a3"}},
		{{URL: "b1"}},
		{{URL: "c1"}, {URL: "c2"}},
	}

	articles := interleaveFeedArticles(feeds)
	got := make([]string, 0, len(articles))
	for _, article := range articles {
		got = append(got, article.URL)
	}
	want := []string{"a1", "b1", "c1", "a2", "c2", "a3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("interleaved URLs = %#v, want %#v", got, want)
	}
}

func TestDefaultNewsCrawlerFeedsAreUniqueHTTPSURLs(t *testing.T) {
	if len(defaultNewsCrawlerFeeds) != 17 {
		t.Fatalf("default feed count = %d, want 17", len(defaultNewsCrawlerFeeds))
	}

	seen := make(map[string]bool, len(defaultNewsCrawlerFeeds))
	for _, raw := range defaultNewsCrawlerFeeds {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			t.Fatalf("invalid HTTPS feed URL %q", raw)
		}
		if seen[raw] {
			t.Fatalf("duplicate feed URL %q", raw)
		}
		seen[raw] = true
	}
}
