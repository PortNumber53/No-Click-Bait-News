package handlers

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const freeURLFetchesPerDay = 10

func (h *Handler) hasUnlimitedReading(ctx context.Context, userID uuid.UUID) (bool, error) {
	var allowed bool
	err := h.pool.QueryRow(ctx,
		`SELECT COALESCE((
			SELECT st.unlimited_reading OR st.price_monthly > 0
			FROM user_subscriptions us
			JOIN subscription_tiers st ON st.id = us.tier_id
			WHERE us.user_id = $1 AND us.status IN ('active', 'trialing')
		), false)`, userID,
	).Scan(&allowed)
	return allowed, err
}

func (h *Handler) recordArticleRead(ctx context.Context, userID, articleID uuid.UUID, category string, unlimited bool) (bool, error) {
	if unlimited {
		return true, nil
	}

	category = canonicalReadCategory(category)
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	lockKey := userID.String() + ":" + category
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", lockKey); err != nil {
		return false, err
	}

	var chosenArticleID uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT article_id FROM user_category_daily_reads
		 WHERE user_id = $1 AND read_category = $2 AND read_date = CURRENT_DATE`,
		userID, category,
	).Scan(&chosenArticleID)
	if err == nil {
		return chosenArticleID == articleID, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO user_category_daily_reads (user_id, read_category, article_id)
		 VALUES ($1, $2, $3)`,
		userID, category, articleID,
	); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (h *Handler) reserveURLFetch(ctx context.Context, userID uuid.UUID, sourceURL string) (bool, error) {
	unlimited, err := h.hasUnlimitedReading(ctx, userID)
	if err != nil {
		return false, err
	}
	if unlimited {
		return true, nil
	}
	return h.reserveDailyUsage(ctx, userID, freeURLFetchesPerDay,
		"SELECT EXISTS(SELECT 1 FROM user_url_fetches WHERE user_id = $1 AND source_url = $2 AND fetch_date = CURRENT_DATE)",
		"SELECT COUNT(*) FROM user_url_fetches WHERE user_id = $1 AND fetch_date = CURRENT_DATE",
		"INSERT INTO user_url_fetches (user_id, source_url) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		sourceURL,
	)
}

func (h *Handler) reserveDailyUsage(ctx context.Context, userID uuid.UUID, limit int, existsSQL, countSQL, insertSQL string, resource any) (bool, error) {
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

	var used int
	if err := tx.QueryRow(ctx, countSQL, userID).Scan(&used); err != nil {
		return false, err
	}
	if used >= limit {
		return false, tx.Rollback(ctx)
	}
	if tag, err := tx.Exec(ctx, insertSQL, userID, resource); err != nil {
		return false, err
	} else if tag.RowsAffected() == 0 {
		return false, errors.New("usage reservation was not inserted")
	}
	return true, tx.Commit(ctx)
}

func canonicalReadCategory(category string) string {
	category = strings.ToLower(strings.TrimSpace(category))
	if category == "" {
		return "general"
	}
	return category
}

func articleReadCategory(category *string, categories []string) string {
	if category != nil && strings.TrimSpace(*category) != "" {
		return strings.TrimSpace(*category)
	}
	for _, candidate := range categories {
		if strings.TrimSpace(candidate) != "" {
			return strings.TrimSpace(candidate)
		}
	}
	return "General"
}
