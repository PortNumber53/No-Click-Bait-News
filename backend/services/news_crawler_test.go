package services

import (
	"encoding/xml"
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

func TestRSSItemImageURLsUsesOnlyDistinctHTTPImages(t *testing.T) {
	item := rssItem{
		Image: "https://cdn.example/hero.jpg",
		MediaContent: []rssMediaImage{
			{URL: "https://cdn.example/second.webp", Type: "image/webp"},
			{URL: "https://cdn.example/video.mp4", Type: "video/mp4"},
		},
		Enclosures: []rssEnclosure{
			{URL: "https://cdn.example/third.png", Type: "image/png"},
		},
		Description: `<img src="https://cdn.example/pixel.gif"><img src="https://cdn.example/fourth.jpg"><img src="javascript:alert(1)">`,
	}

	got := rssItemImageURLs(item)
	want := []string{
		"https://cdn.example/hero.jpg",
		"https://cdn.example/second.webp",
		"https://cdn.example/third.png",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("image URLs = %#v, want %#v", got, want)
	}
}

func TestArticleImageURLsFromTextReadsHTMLAndMarkdown(t *testing.T) {
	got := ArticleImageURLsFromText(`
		<img src="https://cdn.example/hero.jpg">
		![A related chart](https://cdn.example/chart.webp "Chart")
		![Tracking](https://cdn.example/tracking/pixel.gif)
	`)
	want := []string{
		"https://cdn.example/hero.jpg",
		"https://cdn.example/chart.webp",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("image URLs = %#v, want %#v", got, want)
	}
}

func TestRSSMediaNamespaceImageIsDecoded(t *testing.T) {
	var feed rssFeed
	err := xml.Unmarshal([]byte(`
		<rss xmlns:media="http://search.yahoo.com/mrss/">
			<channel><item>
				<title>Story</title>
				<link>https://example.com/story</link>
				<media:content url="https://cdn.example/story.jpg" medium="image" />
			</item></channel>
		</rss>`), &feed)
	if err != nil {
		t.Fatalf("decode RSS: %v", err)
	}
	if len(feed.Channel.Items) != 1 {
		t.Fatalf("item count = %d, want 1", len(feed.Channel.Items))
	}
	got := rssItemImageURLs(feed.Channel.Items[0])
	want := []string{"https://cdn.example/story.jpg"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("image URLs = %#v, want %#v", got, want)
	}
}
