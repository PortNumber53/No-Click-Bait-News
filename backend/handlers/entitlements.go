package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const freeURLFetchesPerDay = 10

type readingEntitlement struct {
	TierName         string
	MonthlyReadLimit int
	HasPremiumAccess bool
	UnlimitedReading bool
}

func (h *Handler) getReadingEntitlement(ctx context.Context, userID uuid.UUID) (readingEntitlement, error) {
	var access readingEntitlement
	err := h.pool.QueryRow(ctx,
		`SELECT COALESCE(st.name, 'free'),
		        COALESCE(st.max_articles_per_month, 0),
		        COALESCE(st.has_premium_access, false),
		        COALESCE(st.unlimited_reading, false)
		 FROM users u
		 LEFT JOIN user_subscriptions us
		   ON us.user_id = u.id AND us.status IN ('active', 'trialing')
		 LEFT JOIN subscription_tiers st ON st.id = us.tier_id
		 WHERE u.id = $1`, userID,
	).Scan(&access.TierName, &access.MonthlyReadLimit, &access.HasPremiumAccess, &access.UnlimitedReading)
	return access, err
}

func (h *Handler) hasUnlimitedReading(ctx context.Context, userID uuid.UUID) (bool, error) {
	access, err := h.getReadingEntitlement(ctx, userID)
	return access.UnlimitedReading, err
}

func (h *Handler) recordArticleRead(ctx context.Context, userID, articleID uuid.UUID, category string, access readingEntitlement) (bool, error) {
	if access.UnlimitedReading {
		return true, nil
	}
	if access.MonthlyReadLimit > 0 {
		return h.recordMonthlyArticleRead(ctx, userID, articleID, access.MonthlyReadLimit)
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

func (h *Handler) recordMonthlyArticleRead(ctx context.Context, userID, articleID uuid.UUID, limit int) (bool, error) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	lockKey := userID.String() + ":monthly-article-reads"
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", lockKey); err != nil {
		return false, err
	}

	var alreadyRead bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM user_monthly_article_reads
			WHERE user_id = $1 AND article_id = $2
			  AND period_start = date_trunc('month', CURRENT_DATE)::date
		)`, userID, articleID,
	).Scan(&alreadyRead); err != nil {
		return false, err
	}
	if alreadyRead {
		return true, tx.Commit(ctx)
	}

	var used int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_monthly_article_reads
		 WHERE user_id = $1
		   AND period_start = date_trunc('month', CURRENT_DATE)::date`, userID,
	).Scan(&used); err != nil {
		return false, err
	}
	if used >= limit {
		return false, tx.Rollback(ctx)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO user_monthly_article_reads (user_id, article_id, period_start)
		 VALUES ($1, $2, date_trunc('month', CURRENT_DATE)::date)`, userID, articleID,
	); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func articleReadLimitMessage(access readingEntitlement, category string) string {
	if access.MonthlyReadLimit > 0 {
		return fmt.Sprintf("Your plan includes %d articles per month. Upgrade for unlimited reading.", access.MonthlyReadLimit)
	}
	return fmt.Sprintf("Your free plan includes one %s article per day. Upgrade for more reading.", category)
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
