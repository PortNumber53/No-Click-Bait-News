package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/PortNumber53/no-click-bait-news/backend/middleware"
	"github.com/PortNumber53/no-click-bait-news/backend/models"
)

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
	query := `SELECT id, title, summary, content, source_name, source_url, image_url, category, published_at, is_premium, view_count
		FROM articles WHERE 1=1`
	args := []any{}
	argIdx := 1

	if !hasPremium {
		query += fmt.Sprintf(" AND is_premium = false")
	}
	if category != "" {
		query += fmt.Sprintf(" AND category = $%d", argIdx)
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
		if err := rows.Scan(&a.ID, &a.Title, &a.Summary, &a.Content, &a.SourceName, &a.SourceURL,
			&a.ImageURL, &a.Category, &a.PublishedAt, &a.IsPremium, &a.ViewCount); err != nil {
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
		`SELECT id, title, summary, content, source_name, source_url, image_url, category, published_at, is_premium, view_count
		 FROM articles WHERE id = $1`, articleID,
	).Scan(&a.ID, &a.Title, &a.Summary, &a.Content, &a.SourceName, &a.SourceURL,
		&a.ImageURL, &a.Category, &a.PublishedAt, &a.IsPremium, &a.ViewCount)
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

	// Increment view count
	h.pool.Exec(r.Context(), "UPDATE articles SET view_count = view_count + 1 WHERE id = $1", articleID)

	JSON(w, http.StatusOK, a)
}
