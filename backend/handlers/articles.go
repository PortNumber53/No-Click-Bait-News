package handlers

import (
	"context"
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
	"github.com/PortNumber53/no-click-bait-news/backend/services"
)

const userSubmittedCategory = "Submitted"

const defaultRewriteTimeout = 5 * time.Minute

const articleCategoriesSelect = "COALESCE(categories, CASE WHEN category IS NULL OR category = '' THEN ARRAY[]::text[] ELSE ARRAY[category] END)"

type articleRewriteJob struct {
	ArticleID       uuid.UUID
	Title           string
	SourceURL       string
	OriginalContent string
	Attempts        int
}

func (h *Handler) startArticleRewriteWorkers() {
	if h.articleRewriter == nil {
		log.Printf("[articles.rewrite.queue] status=disabled reason=rewriter_not_configured")
		return
	}

	workerCount := intEnv("LLM_REWRITE_WORKERS", 2)
	h.rewriteWake = make(chan struct{}, 1)

	for workerID := 1; workerID <= workerCount; workerID++ {
		go h.articleRewriteWorker(workerID)
	}

	log.Printf("[articles.rewrite.queue] status=started workers=%d backend=postgres", workerCount)
	go h.enqueueStaleArticleRewrites()
}

func (h *Handler) articleRewriteWorker(workerID int) {
	for {
		job, found, err := h.claimArticleRewriteJob(context.Background())
		if err != nil {
			log.Printf("[articles.rewrite.queue] worker=%d status=claim_failed error=%q", workerID, err)
		} else if found {
			h.processArticleRewriteJob(workerID, job)
			continue
		}
		select {
		case <-h.rewriteWake:
		case <-time.After(2 * time.Second):
		}
	}
}

func (h *Handler) enqueueStaleArticleRewrites() {
	if h.articleRewriter == nil || h.rewriteWake == nil {
		return
	}

	limit := intEnv("LLM_REWRITE_STALE_ON_START_LIMIT", 100)
	targetVersion := h.articleRewriter.AgentVersion()
	configuredModels := h.configuredRewriteModels()
	rows, err := h.pool.Query(context.Background(),
		`SELECT id, title, source_url, original_content
		 FROM articles
		 WHERE original_content IS NOT NULL
		   AND btrim(original_content) <> ''
		   AND (
		     llm_rewrite_version < $1
		     OR (
		       SELECT COUNT(DISTINCT lm.slug)
		       FROM article_rewrites ar
		       JOIN llm_models lm ON lm.id = ar.llm_model_id
		       WHERE ar.article_id = articles.id
		         AND ar.processing_status = 'completed'
		         AND lm.slug = ANY($3::text[])
		     ) < $4
		   )
		 ORDER BY published_at DESC
		 LIMIT $2`,
		targetVersion, limit, configuredModels, len(configuredModels),
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
		if h.enqueueArticleRewrite(articleID, title, sourceURL, originalContent) {
			queued++
		}
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
		var err error
		hasPremium, err = h.hasPremiumAccess(r.Context(), user.ID)
		if err != nil {
			log.Printf("[feed] premium lookup failed: %v", err)
			Error(w, http.StatusInternalServerError, "Failed to check article access")
			return
		}
	}

	offset := (page - 1) * pageSize
	query := fmt.Sprintf(`SELECT id, title, summary, content, rewrite_status, llm_rewrite_version, source_name, source_url, image_url, category, %s, published_at, COALESCE(is_premium, false), COALESCE(view_count, 0)
		FROM articles WHERE 1=1`, articleCategoriesSelect)
	args := []any{}
	argIdx := 1

	if !hasPremium {
		query += " AND (is_premium = false OR is_premium IS NULL)"
	}
	if category != "" {
		query += fmt.Sprintf(" AND (category = $%d OR $%d = ANY(COALESCE(categories, ARRAY[]::text[])))", argIdx, argIdx)
		args = append(args, category)
		argIdx++
	}
	query += " ORDER BY published_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize+1, offset)

	log.Printf("[feed] user=%v hasPremium=%v query=%s args=%v", user != nil, hasPremium, query, args)

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		log.Printf("[feed] query error: %v", err)
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

	h.attachArticleRewrites(r.Context(), articles)

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

	h.attachArticleRewrites(r.Context(), articles)

	JSON(w, http.StatusOK, models.FeedResponse{
		Articles: articles,
		Page:     page,
		PageSize: pageSize,
		HasMore:  hasMore,
	})
}

func (h *Handler) attachArticleRewrites(ctx context.Context, articles []models.ArticleResponse) {
	if len(articles) == 0 {
		return
	}
	ids := make([]uuid.UUID, len(articles))
	indexByID := make(map[uuid.UUID]int, len(articles))
	for i := range articles {
		ids[i] = articles[i].ID
		indexByID[articles[i].ID] = i
	}
	rows, err := h.pool.Query(ctx,
		`SELECT ar.article_id, ar.id, lm.display_name, ar.rewritten_title, ar.rewritten_summary, ar.rewritten_content
		 FROM article_rewrites ar
		 JOIN llm_models lm ON lm.id = ar.llm_model_id
		 WHERE ar.article_id = ANY($1::uuid[]) AND ar.processing_status = 'completed'
		 ORDER BY ar.article_id, ar.llm_model_id`, ids)
	if err != nil {
		log.Printf("[articles.rewrites] status=query_failed error=%q", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var articleID uuid.UUID
		var rv models.RewriteVersion
		if err := rows.Scan(&articleID, &rv.ID, &rv.ModelName, &rv.Title, &rv.Summary, &rv.Content); err != nil {
			log.Printf("[articles.rewrites] status=scan_failed error=%q", err)
			continue
		}
		if index, ok := indexByID[articleID]; ok {
			articles[index].Rewrites = append(articles[index].Rewrites, rv)
		}
	}
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
		fmt.Sprintf(`SELECT id, title, summary, content, original_content, rewrite_status, llm_rewrite_version, source_name, source_url, image_url, category, %s, published_at, COALESCE(is_premium, false), COALESCE(view_count, 0)
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
		hasPremium, err := h.hasPremiumAccess(r.Context(), user.ID)
		if err != nil {
			Error(w, http.StatusInternalServerError, "Failed to check article access")
			return
		}
		if !hasPremium {
			Error(w, http.StatusForbidden, "Premium subscription required")
			return
		}
	}
	if user := middleware.GetUser(r.Context()); user != nil {
		allowed, err := h.recordArticleRead(r.Context(), user.ID, articleID)
		if err != nil {
			log.Printf("[articles.read] user_id=%s article_id=%s status=usage_failed error=%q", user.ID, articleID, err)
			Error(w, http.StatusInternalServerError, "Failed to record article usage")
			return
		}
		if !allowed {
			Error(w, http.StatusTooManyRequests, "Daily article limit reached")
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

	articles := []models.ArticleResponse{a}
	h.attachArticleRewrites(r.Context(), articles)
	a = articles[0]

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
	if !DecodeJSON(w, r, &req) {
		log.Printf("[articles.fetch] request_id=%s status=bad_request", requestID)
		return
	}

	sourceURL, sourceName, err := normalizeNewsURL(r.Context(), req.URL)
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
	user := middleware.GetUser(r.Context())
	if user == nil {
		Error(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	allowed, err := h.reserveURLFetch(r.Context(), user.ID, sourceURL)
	if err != nil {
		log.Printf("[articles.fetch] request_id=%s status=usage_failed user_id=%s error=%q", requestID, user.ID, err)
		Error(w, http.StatusInternalServerError, "Failed to record URL fetch usage")
		return
	}
	if !allowed {
		Error(w, http.StatusTooManyRequests, "Daily URL fetch limit reached")
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

func (h *Handler) enqueueArticleRewrite(articleID uuid.UUID, title, sourceURL, originalContent string) bool {
	if h.articleRewriter == nil || h.rewriteWake == nil {
		log.Printf("[articles.rewrite.queue] article_id=%s status=skipped reason=not_configured", articleID)
		return false
	}
	if strings.TrimSpace(originalContent) == "" {
		return false
	}
	tag, err := h.pool.Exec(context.Background(),
		`INSERT INTO article_rewrite_jobs (article_id, status, attempts, available_at, locked_at, last_error, updated_at)
		 VALUES ($1, 'pending', 0, NOW(), NULL, NULL, NOW())
		 ON CONFLICT (article_id) DO UPDATE SET
		   status = 'pending', attempts = 0, available_at = NOW(), locked_at = NULL, last_error = NULL, updated_at = NOW()
		 WHERE article_rewrite_jobs.status IN ('completed', 'failed')`, articleID)
	if err != nil {
		log.Printf("[articles.rewrite.queue] article_id=%s status=persist_failed url=%q error=%q", articleID, sourceURL, err)
		return false
	}
	_, _ = h.pool.Exec(context.Background(), "UPDATE articles SET rewrite_status = 'pending' WHERE id = $1", articleID)
	select {
	case h.rewriteWake <- struct{}{}:
	default:
	}
	log.Printf("[articles.rewrite.queue] article_id=%s status=persisted url=%q changed=%v", articleID, sourceURL, tag.RowsAffected() > 0)
	return true
}

func (h *Handler) enqueueArticleRewriteIfOutdated(ctx context.Context, article *models.ArticleResponse) {
	if h.articleRewriter == nil || h.rewriteWake == nil || article == nil || article.OriginalContent == nil {
		return
	}
	originalContent := strings.TrimSpace(*article.OriginalContent)
	if originalContent == "" {
		return
	}
	configuredModels := h.configuredRewriteModels()
	if article.LLMRewriteVersion >= h.articleRewriter.AgentVersion() {
		var completedModels int
		err := h.pool.QueryRow(ctx,
			`SELECT COUNT(DISTINCT lm.slug)
			 FROM article_rewrites ar
			 JOIN llm_models lm ON lm.id = ar.llm_model_id
			 WHERE ar.article_id = $1
			   AND ar.processing_status = 'completed'
			   AND lm.slug = ANY($2::text[])`,
			article.ID, configuredModels,
		).Scan(&completedModels)
		if err == nil && completedModels >= len(configuredModels) {
			return
		}
		if err != nil {
			log.Printf("[articles.rewrite.queue] article_id=%s status=model_coverage_check_failed error=%q", article.ID, err)
		}
	}
	article.RewriteStatus = "pending"
	log.Printf("[articles.rewrite.queue] article_id=%s status=outdated current_version=%d target_version=%d configured_models=%d", article.ID, article.LLMRewriteVersion, h.articleRewriter.AgentVersion(), len(configuredModels))
	h.enqueueArticleRewrite(article.ID, article.Title, article.SourceURL, originalContent)
}

func (h *Handler) configuredRewriteModels() []string {
	models := make([]string, 0, len(h.articleRewriters))
	for _, rewriter := range h.articleRewriters {
		models = append(models, rewriter.Model())
	}
	return models
}

func (h *Handler) claimArticleRewriteJob(ctx context.Context) (articleRewriteJob, bool, error) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return articleRewriteJob{}, false, err
	}
	defer tx.Rollback(ctx)

	var job articleRewriteJob
	err = tx.QueryRow(ctx,
		`SELECT j.article_id, a.title, a.source_url, a.original_content, j.attempts
		 FROM article_rewrite_jobs j
		 JOIN articles a ON a.id = j.article_id
		 WHERE (j.status = 'pending' AND j.available_at <= NOW())
		    OR (j.status = 'running' AND j.locked_at < NOW() - INTERVAL '10 minutes')
		 ORDER BY j.available_at, j.created_at
		 FOR UPDATE OF j SKIP LOCKED
		 LIMIT 1`,
	).Scan(&job.ArticleID, &job.Title, &job.SourceURL, &job.OriginalContent, &job.Attempts)
	if err != nil {
		if err == pgx.ErrNoRows {
			return articleRewriteJob{}, false, nil
		}
		return articleRewriteJob{}, false, err
	}
	job.Attempts++
	if _, err := tx.Exec(ctx,
		`UPDATE article_rewrite_jobs
		 SET status = 'running', attempts = $2, locked_at = NOW(), updated_at = NOW()
		 WHERE article_id = $1`, job.ArticleID, job.Attempts,
	); err != nil {
		return articleRewriteJob{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return articleRewriteJob{}, false, err
	}
	return job, true, nil
}

func (h *Handler) processArticleRewriteJob(workerID int, job articleRewriteJob) {
	start := time.Now()
	log.Printf("[articles.rewrite] worker=%d article_id=%s status=start url=%q bytes=%d", workerID, job.ArticleID, job.SourceURL, len(job.OriginalContent))

	ctx, cancel := context.WithTimeout(context.Background(), rewriteTimeoutFromEnv())
	defer cancel()

	type generatedRewrite struct {
		Rewriter *services.ArticleRewriter
		Result   services.ArticleRewriteResult
	}
	generated := make([]generatedRewrite, 0, len(h.articleRewriters))
	for _, rewriter := range h.articleRewriters {
		rewrite, err := rewriter.RewriteArticle(ctx, job.Title, job.SourceURL, job.OriginalContent)
		if err != nil {
			log.Printf("[articles.rewrite] worker=%d article_id=%s model=%q status=failed url=%q error=%q elapsed_ms=%d", workerID, job.ArticleID, rewriter.Model(), job.SourceURL, err, time.Since(start).Milliseconds())
			h.failArticleRewriteJob(job, err)
			return
		}
		if rewrite.Title == "" {
			rewrite.Title = job.Title
		}
		if rewrite.Summary == "" {
			rewrite.Summary = chooseArticleSummary(nil, rewrite.Content)
		}
		generated = append(generated, generatedRewrite{Rewriter: rewriter, Result: rewrite})
	}
	if len(generated) == 0 {
		h.failArticleRewriteJob(job, fmt.Errorf("no rewrite models are configured"))
		return
	}

	primary := generated[0].Result
	category := models.PrimaryArticleCategory(primary.Categories, nil)
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		h.failArticleRewriteJob(job, err)
		return
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx,
		`UPDATE articles
		 SET original_title = COALESCE(original_title, title),
		     original_summary = COALESCE(original_summary, summary),
		     title = $1, content = $2, summary = $3, category = $4, categories = $5,
		     rewrite_status = 'complete', llm_rewrite_version = $6
		 WHERE id = $7 AND original_content IS NOT NULL`,
		primary.Title, primary.Content, primary.Summary, category, primary.Categories, h.articleRewriter.AgentVersion(), job.ArticleID,
	)
	if err != nil {
		log.Printf("[articles.rewrite] worker=%d article_id=%s status=update_failed url=%q error=%q elapsed_ms=%d", workerID, job.ArticleID, job.SourceURL, err, time.Since(start).Milliseconds())
		_ = tx.Rollback(ctx)
		h.failArticleRewriteJob(job, err)
		return
	}
	if tag.RowsAffected() == 0 {
		_ = tx.Rollback(ctx)
		h.failArticleRewriteJob(job, fmt.Errorf("article is no longer available for rewriting"))
		return
	}
	for _, item := range generated {
		var modelID int
		if err := tx.QueryRow(ctx,
			`INSERT INTO llm_models (slug, display_name, openrouter_model_id, is_active)
			 VALUES ($1, $1, $1, true)
			 ON CONFLICT (slug) DO UPDATE SET openrouter_model_id = EXCLUDED.openrouter_model_id
			 RETURNING id`, item.Rewriter.Model(),
		).Scan(&modelID); err != nil {
			_ = tx.Rollback(ctx)
			h.failArticleRewriteJob(job, err)
			return
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO article_rewrites
			 (article_id, llm_model_id, rewritten_title, rewritten_summary, rewritten_content, processing_status)
			 VALUES ($1, $2, $3, $4, $5, 'completed')
			 ON CONFLICT (article_id, llm_model_id) DO UPDATE SET
			 rewritten_title = EXCLUDED.rewritten_title,
			 rewritten_summary = EXCLUDED.rewritten_summary,
			 rewritten_content = EXCLUDED.rewritten_content,
			 processing_status = 'completed', error_message = NULL`,
			job.ArticleID, modelID, item.Result.Title, item.Result.Summary, item.Result.Content,
		); err != nil {
			_ = tx.Rollback(ctx)
			h.failArticleRewriteJob(job, err)
			return
		}
	}
	if _, err := tx.Exec(ctx,
		`UPDATE article_rewrite_jobs
		 SET status = 'completed', locked_at = NULL, last_error = NULL, updated_at = NOW()
		 WHERE article_id = $1`, job.ArticleID,
	); err != nil {
		_ = tx.Rollback(ctx)
		h.failArticleRewriteJob(job, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		h.failArticleRewriteJob(job, err)
		return
	}

	log.Printf("[articles.rewrite] worker=%d article_id=%s status=completed url=%q rows=%d models=%d output_bytes=%d categories=%q elapsed_ms=%d", workerID, job.ArticleID, job.SourceURL, tag.RowsAffected(), len(generated), len(primary.Content), strings.Join(primary.Categories, ","), time.Since(start).Milliseconds())
}

func (h *Handler) failArticleRewriteJob(job articleRewriteJob, jobErr error) {
	maxAttempts := intEnv("LLM_REWRITE_MAX_ATTEMPTS", 3)
	if job.Attempts >= maxAttempts {
		_, _ = h.pool.Exec(context.Background(),
			`WITH failed_job AS (
				UPDATE article_rewrite_jobs
				SET status = 'failed', locked_at = NULL, last_error = $2, updated_at = NOW()
				WHERE article_id = $1
			)
			UPDATE articles SET rewrite_status = 'failed' WHERE id = $1`,
			job.ArticleID, truncateRunes(jobErr.Error(), 2000))
		return
	}
	delay := time.Duration(job.Attempts*job.Attempts) * 5 * time.Second
	_, _ = h.pool.Exec(context.Background(),
		`UPDATE article_rewrite_jobs
		 SET status = 'pending', available_at = $2, locked_at = NULL, last_error = $3, updated_at = NOW()
		 WHERE article_id = $1`,
		job.ArticleID, time.Now().UTC().Add(delay), truncateRunes(jobErr.Error(), 2000))
	select {
	case h.rewriteWake <- struct{}{}:
	default:
	}
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

func normalizeNewsURL(ctx context.Context, raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("URL is required")
	}
	if len(raw) > 2048 {
		return "", "", fmt.Errorf("URL is too long")
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", "", fmt.Errorf("Enter a valid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", fmt.Errorf("Only HTTP and HTTPS URLs are supported")
	}
	if parsed.User != nil {
		return "", "", fmt.Errorf("URLs with embedded credentials are not supported")
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", "", fmt.Errorf("Enter a valid URL")
	}
	if host == "localhost" {
		return "", "", fmt.Errorf("Private network URLs are not supported")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return "", "", fmt.Errorf("Private network URLs are not supported")
		}
	} else {
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(addresses) == 0 {
			return "", "", fmt.Errorf("Could not resolve URL host")
		}
		for _, address := range addresses {
			if !isPublicIP(address.IP) {
				return "", "", fmt.Errorf("Private network URLs are not supported")
			}
		}
	}

	parsed.Fragment = ""
	return parsed.String(), strings.TrimPrefix(host, "www."), nil
}

func isPublicIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	for _, cidr := range []string{
		"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24",
		"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4",
		"2001:db8::/32",
	} {
		_, blocked, _ := net.ParseCIDR(cidr)
		if blocked.Contains(ip) {
			return false
		}
	}
	return true
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
