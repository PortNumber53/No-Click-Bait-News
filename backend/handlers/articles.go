package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/PortNumber53/no-click-bait-news/backend/middleware"
	"github.com/PortNumber53/no-click-bait-news/backend/models"
)

const userSubmittedCategory = "Submitted"

const defaultRewriteTimeout = 5 * time.Minute

const articleCategoriesSelect = "COALESCE(categories, CASE WHEN category IS NULL OR category = '' THEN ARRAY[]::text[] ELSE ARRAY[category] END)"

type articleRewriteJob struct {
	ArticleID       uuid.UUID
	Title           string
	SourceURL       string
	OriginalContent string
}

func (h *Handler) startArticleRewriteWorkers() {
	if h.articleRewriter == nil {
		log.Printf("[articles.rewrite.queue] status=disabled reason=rewriter_not_configured")
		return
	}

	workerCount := intEnv("LLM_REWRITE_WORKERS", 2)
	queueSize := intEnv("LLM_REWRITE_QUEUE_SIZE", 100)
	h.rewriteJobs = make(chan articleRewriteJob, queueSize)

	for workerID := 1; workerID <= workerCount; workerID++ {
		go h.articleRewriteWorker(workerID)
	}

	log.Printf("[articles.rewrite.queue] status=started workers=%d queue_size=%d", workerCount, queueSize)
	go h.enqueueStaleArticleRewrites()
}

func (h *Handler) articleRewriteWorker(workerID int) {
	for job := range h.rewriteJobs {
		h.processArticleRewriteJob(workerID, job)
	}
}

func (h *Handler) enqueueStaleArticleRewrites() {
	if h.articleRewriter == nil || h.rewriteJobs == nil {
		return
	}

	limit := intEnv("LLM_REWRITE_STALE_ON_START_LIMIT", 100)
	targetVersion := h.articleRewriter.AgentVersion()
	rows, err := h.pool.Query(context.Background(),
		`SELECT id, title, source_url, original_content
		 FROM articles
		 WHERE original_content IS NOT NULL
		   AND btrim(original_content) <> ''
		   AND rewrite_status <> 'pending'
		   AND llm_rewrite_version < $1
		 ORDER BY published_at DESC
		 LIMIT $2`,
		targetVersion, limit,
	)
	if err != nil {
		log.Printf("[articles.rewrite.queue] status=stale_scan_failed target_version=%d error=%q", targetVersion, err)
		return
	}
	defer rows.Close()

	queued := 0
	for rows.Next() {
		var articleID uuid.UUID
		var title, sourceURL, originalContent string
		if err := rows.Scan(&articleID, &title, &sourceURL, &originalContent); err != nil {
			log.Printf("[articles.rewrite.queue] status=stale_scan_row_failed target_version=%d error=%q", targetVersion, err)
			continue
		}
		tag, err := h.pool.Exec(context.Background(),
			`UPDATE articles
			 SET rewrite_status = 'pending'
			 WHERE id = $1
			   AND rewrite_status <> 'pending'
			   AND llm_rewrite_version < $2`,
			articleID, targetVersion,
		)
		if err != nil {
			log.Printf("[articles.rewrite.queue] article_id=%s status=stale_mark_failed target_version=%d error=%q", articleID, targetVersion, err)
			continue
		}
		if tag.RowsAffected() == 0 {
			continue
		}
		h.enqueueArticleRewrite(articleID, title, sourceURL, originalContent)
		queued++
	}
	if err := rows.Err(); err != nil {
		log.Printf("[articles.rewrite.queue] status=stale_scan_iter_failed target_version=%d error=%q", targetVersion, err)
	}
	log.Printf("[articles.rewrite.queue] status=stale_scan_complete target_version=%d queued=%d limit=%d", targetVersion, queued, limit)
}

func (h *Handler) GetFeed(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	category := r.URL.Query().Get("category")

	// Check premium access
	hasPremium := false
	user := middleware.GetUser(r.Context())
	if user != nil {
		var premiumAccess *bool
		h.pool.QueryRow(r.Context(),
			`SELECT st.has_premium_access FROM user_subscriptions us
			 JOIN subscription_tiers st ON st.id = us.tier_id
			 WHERE us.user_id = $1`, user.ID,
		).Scan(&premiumAccess)
		if premiumAccess != nil {
			hasPremium = *premiumAccess
		}
	}

	offset := (page - 1) * pageSize
	query := fmt.Sprintf(`SELECT id, title, summary, content, rewrite_status, llm_rewrite_version, source_name, source_url, image_url, category, %s, published_at, is_premium, view_count
		FROM articles WHERE 1=1`, articleCategoriesSelect)
	args := []any{}
	argIdx := 1

	if !hasPremium {
		query += fmt.Sprintf(" AND is_premium = false")
	}
	if category != "" {
		query += fmt.Sprintf(" AND (category = $%d OR $%d = ANY(COALESCE(categories, ARRAY[]::text[])))", argIdx, argIdx)
		args = append(args, category)
		argIdx++
	}
	query += " ORDER BY published_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize+1, offset)

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to fetch articles")
		return
	}
	defer rows.Close()

	articles := make([]models.ArticleResponse, 0)
	for rows.Next() {
		var a models.ArticleResponse
		if err := rows.Scan(&a.ID, &a.Title, &a.Summary, &a.Content, &a.RewriteStatus, &a.LLMRewriteVersion, &a.SourceName, &a.SourceURL,
			&a.ImageURL, &a.Category, &a.Categories, &a.PublishedAt, &a.IsPremium, &a.ViewCount); err != nil {
			continue
		}
		articles = append(articles, a)
	}

	hasMore := len(articles) > pageSize
	if hasMore {
		articles = articles[:pageSize]
	}

	JSON(w, http.StatusOK, models.FeedResponse{
		Articles: articles,
		Page:     page,
		PageSize: pageSize,
		HasMore:  hasMore,
	})
}

func (h *Handler) GetMyArticles(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		Error(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	rows, err := h.pool.Query(r.Context(),
		fmt.Sprintf(`SELECT id, title, summary, content, rewrite_status, llm_rewrite_version, source_name, source_url, image_url, category, %s, published_at, is_premium, view_count
		 FROM articles
		 WHERE submitted_by_user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`, articleCategoriesSelect),
		user.ID, pageSize+1, offset,
	)
	if err != nil {
		log.Printf("[articles.my] user_id=%s status=query_failed error=%q", user.ID, err)
		Error(w, http.StatusInternalServerError, "Failed to fetch submitted articles")
		return
	}
	defer rows.Close()

	articles := make([]models.ArticleResponse, 0)
	for rows.Next() {
		var a models.ArticleResponse
		if err := rows.Scan(&a.ID, &a.Title, &a.Summary, &a.Content, &a.RewriteStatus, &a.LLMRewriteVersion, &a.SourceName, &a.SourceURL,
			&a.ImageURL, &a.Category, &a.Categories, &a.PublishedAt, &a.IsPremium, &a.ViewCount); err != nil {
			log.Printf("[articles.my] user_id=%s status=scan_failed error=%q", user.ID, err)
			continue
		}
		articles = append(articles, a)
	}

	hasMore := len(articles) > pageSize
	if hasMore {
		articles = articles[:pageSize]
	}

	JSON(w, http.StatusOK, models.FeedResponse{
		Articles: articles,
		Page:     page,
		PageSize: pageSize,
		HasMore:  hasMore,
	})
}

func (h *Handler) GetArticle(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "articleID")
	articleID, err := uuid.Parse(idStr)
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid article ID")
		return
	}

	var a models.ArticleResponse
	err = h.pool.QueryRow(r.Context(),
		fmt.Sprintf(`SELECT id, title, summary, content, original_content, rewrite_status, llm_rewrite_version, source_name, source_url, image_url, category, %s, published_at, is_premium, view_count
		 FROM articles WHERE id = $1`, articleCategoriesSelect), articleID,
	).Scan(&a.ID, &a.Title, &a.Summary, &a.Content, &a.OriginalContent, &a.RewriteStatus, &a.LLMRewriteVersion, &a.SourceName, &a.SourceURL,
		&a.ImageURL, &a.Category, &a.Categories, &a.PublishedAt, &a.IsPremium, &a.ViewCount)
	if err != nil {
		Error(w, http.StatusNotFound, "Article not found")
		return
	}

	if a.IsPremium {
		user := middleware.GetUser(r.Context())
		if user == nil {
			Error(w, http.StatusForbidden, "Premium subscription required")
			return
		}
		var hasPremium bool
		h.pool.QueryRow(r.Context(),
			`SELECT COALESCE(st.has_premium_access, false) FROM user_subscriptions us
			 JOIN subscription_tiers st ON st.id = us.tier_id
			 WHERE us.user_id = $1`, user.ID,
		).Scan(&hasPremium)
		if !hasPremium {
			Error(w, http.StatusForbidden, "Premium subscription required")
			return
		}
	}

	if h.tinyFish != nil && strings.TrimSpace(a.SourceURL) != "" && (a.Content == nil || strings.TrimSpace(*a.Content) == "") {
		page, err := h.tinyFish.FetchContent(r.Context(), a.SourceURL)
		if err != nil {
			log.Printf("WARNING: TinyFish content fetch failed for article %s: %v", articleID, err)
		} else {
			originalContent := strings.TrimSpace(page.Text)
			if originalContent != "" {
				content := originalContent
				a.Content = &content
				a.OriginalContent = &originalContent
				a.RewriteStatus = "pending"
				if _, err := h.pool.Exec(r.Context(), "UPDATE articles SET content = $1, original_content = $2, rewrite_status = 'pending' WHERE id = $3", content, originalContent, articleID); err != nil {
					log.Printf("WARNING: failed to persist fetched content for article %s: %v", articleID, err)
				}
				h.enqueueArticleRewrite(articleID, chooseArticleTitle(page.Title, a.SourceURL), a.SourceURL, originalContent)
			}
		}
	}
	h.enqueueArticleRewriteIfOutdated(r.Context(), &a)

	// Increment view count
	h.pool.Exec(r.Context(), "UPDATE articles SET view_count = view_count + 1 WHERE id = $1", articleID)

	JSON(w, http.StatusOK, a)
}

func (h *Handler) FetchArticle(w http.ResponseWriter, r *http.Request) {
	requestID := requestIDFromHeader(r)
	start := time.Now()
	log.Printf("[articles.fetch] request_id=%s status=start remote_addr=%s", requestID, r.RemoteAddr)

	if h.tinyFish == nil {
		log.Printf("[articles.fetch] request_id=%s status=not_configured dependency=tinyfish", requestID)
		Error(w, http.StatusServiceUnavailable, "Article fetching is not configured")
		return
	}

	var req models.FetchArticleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[articles.fetch] request_id=%s status=bad_request error=%q", requestID, err)
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	sourceURL, sourceName, err := normalizeNewsURL(req.URL)
	if err != nil {
		log.Printf("[articles.fetch] request_id=%s status=invalid_url error=%q", requestID, err)
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	log.Printf("[articles.fetch] request_id=%s status=normalized url=%q source_name=%q", requestID, sourceURL, sourceName)

	existing, found, err := h.findArticleBySourceURL(r, sourceURL)
	if err != nil {
		log.Printf("[articles.fetch] request_id=%s status=lookup_failed url=%q error=%q", requestID, sourceURL, err)
		Error(w, http.StatusInternalServerError, "Failed to check article URL")
		return
	}
	if found {
		log.Printf("[articles.fetch] request_id=%s status=existing article_id=%s url=%q elapsed_ms=%d", requestID, existing.ID, sourceURL, time.Since(start).Milliseconds())
		if user := middleware.GetUser(r.Context()); user != nil {
			if _, err := h.pool.Exec(r.Context(), "UPDATE articles SET submitted_by_user_id = COALESCE(submitted_by_user_id, $1) WHERE id = $2", user.ID, existing.ID); err != nil {
				log.Printf("[articles.fetch] request_id=%s status=claim_existing_failed article_id=%s user_id=%s error=%q", requestID, existing.ID, user.ID, err)
			}
		}
		h.enqueueArticleRewriteIfOutdated(r.Context(), &existing)
		JSON(w, http.StatusOK, existing)
		return
	}

	log.Printf("[articles.fetch] request_id=%s status=tinyfish_start url=%q", requestID, sourceURL)
	page, err := h.tinyFish.FetchContent(r.Context(), sourceURL)
	if err != nil {
		log.Printf("[articles.fetch] request_id=%s status=tinyfish_failed url=%q error=%q elapsed_ms=%d", requestID, sourceURL, err, time.Since(start).Milliseconds())
		Error(w, http.StatusBadGateway, "Failed to fetch article URL")
		return
	}

	originalContent := strings.TrimSpace(page.Text)
	if originalContent == "" {
		log.Printf("[articles.fetch] request_id=%s status=tinyfish_empty url=%q elapsed_ms=%d", requestID, sourceURL, time.Since(start).Milliseconds())
		Error(w, http.StatusBadGateway, "Fetched article did not include readable content")
		return
	}
	log.Printf("[articles.fetch] request_id=%s status=tinyfish_ok url=%q bytes=%d elapsed_ms=%d", requestID, sourceURL, len(originalContent), time.Since(start).Milliseconds())

	title := chooseArticleTitle(page.Title, sourceURL)
	summary := chooseArticleSummary(page.Description, originalContent)
	publishedAt := parseTinyFishPublishedDate(page.PublishedDate)
	category := userSubmittedCategory
	var submittedBy *uuid.UUID
	if user := middleware.GetUser(r.Context()); user != nil {
		submittedBy = &user.ID
	}

	var article models.ArticleResponse
	err = h.pool.QueryRow(r.Context(),
		fmt.Sprintf(`INSERT INTO articles (title, summary, content, original_content, rewrite_status, llm_rewrite_version, source_name, source_url, category, categories, published_at, is_premium, submitted_by_user_id)
		 VALUES ($1, $2, $3, $4, 'pending', 0, $5, $6, $7, ARRAY[$7], $8, false, $9)
		 RETURNING id, title, summary, content, original_content, rewrite_status, llm_rewrite_version, source_name, source_url, image_url, category, %s, published_at, is_premium, view_count`, articleCategoriesSelect),
		title, summary, originalContent, originalContent, sourceName, sourceURL, category, publishedAt, submittedBy,
	).Scan(&article.ID, &article.Title, &article.Summary, &article.Content, &article.OriginalContent, &article.RewriteStatus, &article.LLMRewriteVersion, &article.SourceName, &article.SourceURL,
		&article.ImageURL, &article.Category, &article.Categories, &article.PublishedAt, &article.IsPremium, &article.ViewCount)
	if err != nil {
		log.Printf("[articles.fetch] request_id=%s status=insert_failed url=%q error=%q", requestID, sourceURL, err)
		Error(w, http.StatusInternalServerError, "Failed to save fetched article")
		return
	}

	log.Printf("[articles.fetch] request_id=%s status=created article_id=%s url=%q elapsed_ms=%d", requestID, article.ID, sourceURL, time.Since(start).Milliseconds())
	JSON(w, http.StatusCreated, article)

	if h.articleRewriter != nil {
		h.enqueueArticleRewrite(article.ID, title, sourceURL, originalContent)
	} else {
		log.Printf("[articles.rewrite] article_id=%s status=skipped reason=not_configured", article.ID)
	}
}

func (h *Handler) enqueueArticleRewrite(articleID uuid.UUID, title, sourceURL, originalContent string) {
	if h.articleRewriter == nil || h.rewriteJobs == nil {
		log.Printf("[articles.rewrite.queue] article_id=%s status=skipped reason=not_configured", articleID)
		return
	}

	job := articleRewriteJob{
		ArticleID:       articleID,
		Title:           title,
		SourceURL:       sourceURL,
		OriginalContent: originalContent,
	}

	select {
	case h.rewriteJobs <- job:
		log.Printf("[articles.rewrite.queue] article_id=%s status=queued url=%q queue_depth=%d", articleID, sourceURL, len(h.rewriteJobs))
	default:
		log.Printf("[articles.rewrite.queue] article_id=%s status=dropped reason=queue_full url=%q queue_depth=%d", articleID, sourceURL, len(h.rewriteJobs))
	}
}

func (h *Handler) enqueueArticleRewriteIfOutdated(ctx context.Context, article *models.ArticleResponse) {
	if h.articleRewriter == nil || h.rewriteJobs == nil || article == nil || article.OriginalContent == nil {
		return
	}
	originalContent := strings.TrimSpace(*article.OriginalContent)
	if originalContent == "" || article.RewriteStatus == "pending" || article.LLMRewriteVersion >= h.articleRewriter.AgentVersion() {
		return
	}

	tag, err := h.pool.Exec(ctx,
		`UPDATE articles
		 SET rewrite_status = 'pending'
		 WHERE id = $1
		   AND original_content IS NOT NULL
		   AND rewrite_status <> 'pending'
		   AND llm_rewrite_version < $2`,
		article.ID, h.articleRewriter.AgentVersion(),
	)
	if err != nil {
		log.Printf("[articles.rewrite.queue] article_id=%s status=stale_check_failed error=%q", article.ID, err)
		return
	}
	if tag.RowsAffected() == 0 {
		return
	}

	article.RewriteStatus = "pending"
	log.Printf("[articles.rewrite.queue] article_id=%s status=outdated current_version=%d target_version=%d", article.ID, article.LLMRewriteVersion, h.articleRewriter.AgentVersion())
	h.enqueueArticleRewrite(article.ID, article.Title, article.SourceURL, originalContent)
}

func (h *Handler) processArticleRewriteJob(workerID int, job articleRewriteJob) {
	start := time.Now()
	log.Printf("[articles.rewrite] worker=%d article_id=%s status=start url=%q bytes=%d", workerID, job.ArticleID, job.SourceURL, len(job.OriginalContent))

	ctx, cancel := context.WithTimeout(context.Background(), rewriteTimeoutFromEnv())
	defer cancel()

	rewrite, err := h.articleRewriter.RewriteArticle(ctx, job.Title, job.SourceURL, job.OriginalContent)
	if err != nil {
		log.Printf("[articles.rewrite] worker=%d article_id=%s status=failed url=%q error=%q elapsed_ms=%d", workerID, job.ArticleID, job.SourceURL, err, time.Since(start).Milliseconds())
		_, _ = h.pool.Exec(context.Background(), "UPDATE articles SET rewrite_status = 'failed' WHERE id = $1", job.ArticleID)
		return
	}

	summary := chooseArticleSummary(nil, rewrite.Content)
	category := models.PrimaryArticleCategory(rewrite.Categories, nil)
	tag, err := h.pool.Exec(ctx,
		`UPDATE articles
		 SET content = $1, summary = $2, category = $3, categories = $4, rewrite_status = 'complete', llm_rewrite_version = $5
		 WHERE id = $6 AND original_content IS NOT NULL`,
		rewrite.Content, summary, category, rewrite.Categories, h.articleRewriter.AgentVersion(), job.ArticleID,
	)
	if err != nil {
		log.Printf("[articles.rewrite] worker=%d article_id=%s status=update_failed url=%q error=%q elapsed_ms=%d", workerID, job.ArticleID, job.SourceURL, err, time.Since(start).Milliseconds())
		_, _ = h.pool.Exec(context.Background(), "UPDATE articles SET rewrite_status = 'failed' WHERE id = $1", job.ArticleID)
		return
	}

	log.Printf("[articles.rewrite] worker=%d article_id=%s status=completed url=%q rows=%d output_bytes=%d categories=%q elapsed_ms=%d", workerID, job.ArticleID, job.SourceURL, tag.RowsAffected(), len(rewrite.Content), strings.Join(rewrite.Categories, ","), time.Since(start).Milliseconds())
}

func (h *Handler) findArticleBySourceURL(r *http.Request, sourceURL string) (models.ArticleResponse, bool, error) {
	var article models.ArticleResponse
	err := h.pool.QueryRow(r.Context(),
		fmt.Sprintf(`SELECT id, title, summary, content, original_content, rewrite_status, llm_rewrite_version, source_name, source_url, image_url, category, %s, published_at, is_premium, view_count
		 FROM articles WHERE source_url = $1
		 ORDER BY published_at DESC
		 LIMIT 1`, articleCategoriesSelect), sourceURL,
	).Scan(&article.ID, &article.Title, &article.Summary, &article.Content, &article.OriginalContent, &article.RewriteStatus, &article.LLMRewriteVersion, &article.SourceName, &article.SourceURL,
		&article.ImageURL, &article.Category, &article.Categories, &article.PublishedAt, &article.IsPremium, &article.ViewCount)
	if err != nil {
		if err == pgx.ErrNoRows {
			return models.ArticleResponse{}, false, nil
		}
		return models.ArticleResponse{}, false, err
	}
	return article, true, nil
}

func normalizeNewsURL(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("URL is required")
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", "", fmt.Errorf("Enter a valid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", fmt.Errorf("Only HTTP and HTTPS URLs are supported")
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", "", fmt.Errorf("Enter a valid URL")
	}
	if host == "localhost" {
		return "", "", fmt.Errorf("Private network URLs are not supported")
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
		return "", "", fmt.Errorf("Private network URLs are not supported")
	}

	parsed.Fragment = ""
	return parsed.String(), strings.TrimPrefix(host, "www."), nil
}

func chooseArticleTitle(title *string, sourceURL string) string {
	if title != nil && strings.TrimSpace(*title) != "" {
		return truncateRunes(strings.TrimSpace(*title), 240)
	}
	return truncateRunes(sourceURL, 240)
}

func chooseArticleSummary(description *string, content string) string {
	if description != nil && strings.TrimSpace(*description) != "" {
		return truncateRunes(strings.TrimSpace(*description), 500)
	}
	return truncateRunes(strings.Join(strings.Fields(content), " "), 500)
}

func parseTinyFishPublishedDate(raw *string) time.Time {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return time.Now().UTC()
	}
	value := strings.TrimSpace(*raw)
	layouts := []string{
		time.RFC3339,
		time.RFC1123Z,
		time.RFC1123,
		"2006-01-02",
		"January 2, 2006",
		"Jan 2, 2006",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Now().UTC()
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func requestIDFromHeader(r *http.Request) string {
	for _, header := range []string{"X-Request-ID", "Cf-Ray", "X-Correlation-ID"} {
		if value := strings.TrimSpace(r.Header.Get(header)); value != "" {
			return value
		}
	}
	return uuid.NewString()
}

func intEnv(key string, fallback int) int {
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

func rewriteTimeoutFromEnv() time.Duration {
	seconds := intEnv("LLM_REWRITE_TIMEOUT_SECONDS", int(defaultRewriteTimeout/time.Second))
	return time.Duration(seconds) * time.Second
}
