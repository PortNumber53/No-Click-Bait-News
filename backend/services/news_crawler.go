package services

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var defaultNewsCrawlerFeeds = []string{
	"https://feeds.bbci.co.uk/news/rss.xml",
	"https://www.aljazeera.com/xml/rss/all.xml",
	"https://www.npr.org/rss/rss.php?id=1001",
	"https://www.theguardian.com/world/rss",
	"https://feeds.nbcnews.com/nbcnews/public/news",
}

type NewsCrawlerStats struct {
	FeedsChecked  int
	URLsFound     int
	Inserted      int
	Skipped       int
	Failed        int
	RewriteQueued int
}

type feedArticle struct {
	Title       string
	URL         string
	Description string
	PublishedAt *time.Time
}

type contentFetchJob struct {
	Article feedArticle
}

type rssFeed struct {
	Channel struct {
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
			PubDate     string `xml:"pubDate"`
		} `xml:"item"`
	} `xml:"channel"`
}

func CrawlMajorNews(ctx context.Context, pool *pgxpool.Pool, tinyFish *TinyFishClient, rewriter *ArticleRewriter, limit int) (NewsCrawlerStats, error) {
	if tinyFish == nil {
		return NewsCrawlerStats{}, fmt.Errorf("tinyfish client is not configured")
	}
	if limit < 1 {
		limit = 25
	}

	feeds := newsCrawlerFeedsFromEnv()
	stats := &crawlerStats{}
	stats.feedsChecked.Store(int64(len(feeds)))

	feedJobs := make(chan string, crawlerIntEnv("NEWS_CRAWLER_RSS_QUEUE_SIZE", 100))
	contentJobs := make(chan contentFetchJob, crawlerIntEnv("NEWS_CRAWLER_URL_QUEUE_SIZE", 250))
	seen := &sync.Map{}
	var inserted atomic.Int64
	var claimedSlots atomic.Int64
	var feedWG sync.WaitGroup
	var contentWG sync.WaitGroup

	for i := 1; i <= crawlerIntEnv("NEWS_CRAWLER_FETCH_WORKERS", 4); i++ {
		contentWG.Add(1)
		go func(workerID int) {
			defer contentWG.Done()
			for job := range contentJobs {
				if !claimCrawlerSlot(&claimedSlots, int64(limit)) {
					continue
				}
				articleID, _, _, insertedRow := crawlFetchContent(ctx, pool, tinyFish, job.Article, stats)
				if !insertedRow {
					claimedSlots.Add(-1)
					continue
				}
				inserted.Add(1)
				if rewriter == nil {
					if _, err := pool.Exec(ctx,
						"UPDATE articles SET rewrite_status = 'complete' WHERE id = $1",
						articleID,
					); err != nil {
						stats.failed.Add(1)
						log.Printf("[news.crawler.fetch] article_id=%s status=finalize_failed error=%q", articleID, err)
					}
					continue
				}
				if err := queueCrawlerRewrite(ctx, pool, articleID); err != nil {
					stats.failed.Add(1)
					_, _ = pool.Exec(context.Background(), "UPDATE articles SET rewrite_status = 'failed' WHERE id = $1", articleID)
					log.Printf("[news.crawler.rewrite.queue] worker=%d article_id=%s status=persist_failed url=%q error=%q", workerID, articleID, job.Article.URL, err)
					continue
				}
				stats.rewriteQueued.Add(1)
				log.Printf("[news.crawler.rewrite.queue] worker=%d article_id=%s status=queued url=%q", workerID, articleID, job.Article.URL)
			}
		}(i)
	}

	for i := 1; i <= crawlerIntEnv("NEWS_CRAWLER_RSS_WORKERS", 2); i++ {
		feedWG.Add(1)
		go func(workerID int) {
			defer feedWG.Done()
			for feedURL := range feedJobs {
				articles, err := fetchFeedArticles(ctx, feedURL)
				if err != nil {
					stats.failed.Add(1)
					log.Printf("[news.crawler.feed] worker=%d feed=%q status=failed error=%q", workerID, feedURL, err)
					continue
				}
				log.Printf("[news.crawler.feed] worker=%d feed=%q status=parsed urls=%d", workerID, feedURL, len(articles))
				for _, article := range articles {
					if inserted.Load() >= int64(limit) {
						return
					}
					if article.URL == "" {
						continue
					}
					if _, loaded := seen.LoadOrStore(article.URL, true); loaded {
						continue
					}
					stats.urlsFound.Add(1)
					select {
					case contentJobs <- contentFetchJob{Article: article}:
					case <-ctx.Done():
						return
					}
				}
			}
		}(i)
	}

	for _, feedURL := range feeds {
		select {
		case feedJobs <- feedURL:
		case <-ctx.Done():
			break
		}
	}
	close(feedJobs)

	feedWG.Wait()
	close(contentJobs)
	contentWG.Wait()

	return stats.snapshot(), nil
}

func claimCrawlerSlot(claimed *atomic.Int64, limit int64) bool {
	for {
		current := claimed.Load()
		if current >= limit {
			return false
		}
		if claimed.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

type crawlerStats struct {
	feedsChecked  atomic.Int64
	urlsFound     atomic.Int64
	inserted      atomic.Int64
	skipped       atomic.Int64
	failed        atomic.Int64
	rewriteQueued atomic.Int64
}

func (s *crawlerStats) snapshot() NewsCrawlerStats {
	return NewsCrawlerStats{
		FeedsChecked:  int(s.feedsChecked.Load()),
		URLsFound:     int(s.urlsFound.Load()),
		Inserted:      int(s.inserted.Load()),
		Skipped:       int(s.skipped.Load()),
		Failed:        int(s.failed.Load()),
		RewriteQueued: int(s.rewriteQueued.Load()),
	}
}

func crawlFetchContent(ctx context.Context, pool *pgxpool.Pool, tinyFish *TinyFishClient, article feedArticle, stats *crawlerStats) (uuid.UUID, string, string, bool) {
	var existingID uuid.UUID
	err := pool.QueryRow(ctx, "SELECT id FROM articles WHERE source_url = $1 LIMIT 1", article.URL).Scan(&existingID)
	if err == nil {
		stats.skipped.Add(1)
		return uuid.Nil, "", "", false
	}
	if err != pgx.ErrNoRows {
		stats.failed.Add(1)
		log.Printf("[news.crawler.fetch] url=%q status=lookup_failed error=%q", article.URL, err)
		return uuid.Nil, "", "", false
	}

	page, err := tinyFish.FetchContent(ctx, article.URL)
	if err != nil {
		stats.failed.Add(1)
		log.Printf("[news.crawler.fetch] url=%q status=tinyfish_failed error=%q", article.URL, err)
		return uuid.Nil, "", "", false
	}

	originalContent := strings.TrimSpace(page.Text)
	if originalContent == "" {
		stats.failed.Add(1)
		log.Printf("[news.crawler.fetch] url=%q status=tinyfish_empty", article.URL)
		return uuid.Nil, "", "", false
	}

	title := strings.TrimSpace(article.Title)
	if page.Title != nil && strings.TrimSpace(*page.Title) != "" {
		title = strings.TrimSpace(*page.Title)
	}
	if title == "" {
		title = article.URL
	}

	summary := article.Description
	if summary == "" {
		summary = summarizeText(originalContent, 500)
	}
	publishedAt := time.Now().UTC()
	if article.PublishedAt != nil {
		publishedAt = *article.PublishedAt
	}

	var articleID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO articles (title, summary, content, original_content, rewrite_status, llm_rewrite_version, source_name, source_url, category, categories, published_at, is_premium)
		 VALUES ($1, $2, $3, $4, 'pending', 0, $5, $6, 'Crawled', ARRAY['Crawled'], $7, false)
		 RETURNING id`,
		truncateText(title, 240),
		truncateText(summary, 500),
		originalContent,
		originalContent,
		sourceNameFromURL(article.URL),
		article.URL,
		publishedAt,
	).Scan(&articleID)
	if err != nil {
		stats.failed.Add(1)
		log.Printf("[news.crawler.fetch] url=%q status=insert_failed error=%q", article.URL, err)
		return uuid.Nil, "", "", false
	}

	stats.inserted.Add(1)
	log.Printf("[news.crawler.fetch] article_id=%s url=%q status=inserted bytes=%d", articleID, article.URL, len(originalContent))
	return articleID, title, originalContent, true
}

func queueCrawlerRewrite(ctx context.Context, pool *pgxpool.Pool, articleID uuid.UUID) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO article_rewrite_jobs (article_id, status, attempts, available_at, locked_at, last_error, updated_at)
		 VALUES ($1, 'pending', 0, NOW(), NULL, NULL, NOW())
		 ON CONFLICT (article_id) DO UPDATE SET
		   status = 'pending', attempts = 0, available_at = NOW(), locked_at = NULL, last_error = NULL, updated_at = NOW()
		 WHERE article_rewrite_jobs.status IN ('completed', 'failed')`,
		articleID,
	)
	return err
}

func fetchFeedArticles(ctx context.Context, feedURL string) ([]feedArticle, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "No-Click-Bait-News/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("feed returned HTTP %d", resp.StatusCode)
	}

	var rss rssFeed
	decoder := xml.NewDecoder(resp.Body)
	if err := decoder.Decode(&rss); err != nil {
		return nil, err
	}
	if len(rss.Channel.Items) == 0 {
		return nil, fmt.Errorf("feed did not include RSS items")
	}

	articles := make([]feedArticle, 0, len(rss.Channel.Items))
	for _, item := range rss.Channel.Items {
		articles = append(articles, feedArticle{
			Title:       strings.TrimSpace(item.Title),
			URL:         strings.TrimSpace(item.Link),
			Description: summarizeText(item.Description, 500),
			PublishedAt: parseFeedTime(item.PubDate),
		})
	}
	return articles, nil
}

func newsCrawlerFeedsFromEnv() []string {
	raw := strings.TrimSpace(os.Getenv("NEWS_CRAWLER_FEEDS"))
	if raw == "" {
		return defaultNewsCrawlerFeeds
	}
	parts := strings.Split(raw, ",")
	feeds := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			feeds = append(feeds, value)
		}
	}
	if len(feeds) == 0 {
		return defaultNewsCrawlerFeeds
	}
	return feeds
}

func crawlerIntEnv(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 1 {
		log.Printf("[config] key=%s status=invalid value=%q fallback=%d", key, raw, fallback)
		return fallback
	}
	return parsed
}

func parseFeedTime(raw string) *time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC3339, "Mon, 02 Jan 2006 15:04:05 MST"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}

func sourceNameFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return "Unknown"
	}
	return strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
}

func summarizeText(value string, limit int) string {
	return truncateText(strings.Join(strings.Fields(value), " "), limit)
}

func truncateText(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
