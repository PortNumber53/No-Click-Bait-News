package handlers

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

func (h *Handler) hasPremiumAccess(ctx context.Context, userID uuid.UUID) (bool, error) {
	var allowed bool
	err := h.pool.QueryRow(ctx,
		`SELECT COALESCE((
			SELECT st.has_premium_access
			FROM user_subscriptions us
			JOIN subscription_tiers st ON st.id = us.tier_id
			WHERE us.user_id = $1 AND us.status IN ('active', 'trialing')
		), false)`, userID,
	).Scan(&allowed)
	return allowed, err
}

func (h *Handler) recordArticleRead(ctx context.Context, userID, articleID uuid.UUID) (bool, error) {
	return h.reserveDailyUsage(ctx, userID,
		"SELECT EXISTS(SELECT 1 FROM user_article_reads WHERE user_id = $1 AND article_id = $2 AND read_date = CURRENT_DATE)",
		"SELECT COUNT(*) FROM user_article_reads WHERE user_id = $1 AND read_date = CURRENT_DATE",
		"INSERT INTO user_article_reads (user_id, article_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		articleID,
	)
}

func (h *Handler) reserveURLFetch(ctx context.Context, userID uuid.UUID, sourceURL string) (bool, error) {
	return h.reserveDailyUsage(ctx, userID,
		"SELECT EXISTS(SELECT 1 FROM user_url_fetches WHERE user_id = $1 AND source_url = $2 AND fetch_date = CURRENT_DATE)",
		"SELECT COUNT(*) FROM user_url_fetches WHERE user_id = $1 AND fetch_date = CURRENT_DATE",
		"INSERT INTO user_url_fetches (user_id, source_url) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		sourceURL,
	)
}

func (h *Handler) reserveDailyUsage(ctx context.Context, userID uuid.UUID, existsSQL, countSQL, insertSQL string, resource any) (bool, error) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", userID.String()); err != nil {
		return false, err
	}
	var exists bool
	if err := tx.QueryRow(ctx, existsSQL, userID, resource).Scan(&exists); err != nil {
		return false, err
	}
	if exists {
		return true, tx.Commit(ctx)
	}

	var limit, used int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE((
			SELECT st.max_articles_per_day
			FROM user_subscriptions us
			JOIN subscription_tiers st ON st.id = us.tier_id
			WHERE us.user_id = $1 AND us.status IN ('active', 'trialing')
		), (SELECT max_articles_per_day FROM subscription_tiers WHERE name = 'free'), 10)`,
		userID,
	).Scan(&limit); err != nil {
		return false, err
	}
	if err := tx.QueryRow(ctx, countSQL, userID).Scan(&used); err != nil {
		return false, err
	}
	if used >= limit {
		return false, tx.Rollback(ctx)
	}
	if tag, err := tx.Exec(ctx, insertSQL, userID, resource); err != nil {
		return false, err
	} else if tag.RowsAffected() == 0 {
		return false, fmt.Errorf("usage reservation was not inserted")
	}
	return true, tx.Commit(ctx)
}
