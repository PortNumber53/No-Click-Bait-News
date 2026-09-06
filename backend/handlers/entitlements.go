package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const freeURLFetchesPerDay = 10
const articleAccessRetentionDays = 7

type readingEntitlement struct {
	TierName         string
	MonthlyReadLimit int
	HasPremiumAccess bool
	UnlimitedReading bool
}

type articleReadAccess struct {
	Allowed   bool
	ExpiresAt *time.Time
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

func (h *Handler) recordArticleRead(ctx context.Context, userID, articleID uuid.UUID, category string, access readingEntitlement) (articleReadAccess, error) {
	if access.UnlimitedReading {
		return articleReadAccess{Allowed: true}, nil
	}

	category = canonicalReadCategory(category)
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return articleReadAccess{}, err
	}
	defer tx.Rollback(ctx)

	lockKey := userID.String() + ":article-access"
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", lockKey); err != nil {
		return articleReadAccess{}, err
	}

	var existingExpiry time.Time
	err = tx.QueryRow(ctx,
		`SELECT expires_at FROM user_article_access_grants
		 WHERE user_id = $1 AND article_id = $2 AND expires_at > NOW()`,
		userID, articleID,
	).Scan(&existingExpiry)
	if err == nil {
		return articleReadAccess{Allowed: true, ExpiresAt: &existingExpiry}, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return articleReadAccess{}, err
	}

	if access.MonthlyReadLimit > 0 {
		var used int
		err = tx.QueryRow(ctx,
			`INSERT INTO user_monthly_read_usage (user_id, period_start, reads_used)
			 VALUES ($1, date_trunc('month', CURRENT_DATE)::date, 1)
			 ON CONFLICT (user_id, period_start) DO UPDATE SET
			   reads_used = user_monthly_read_usage.reads_used + 1,
			   updated_at = NOW()
			 WHERE user_monthly_read_usage.reads_used < $2
			 RETURNING reads_used`,
			userID, access.MonthlyReadLimit,
		).Scan(&used)
		if errors.Is(err, pgx.ErrNoRows) {
			return articleReadAccess{Allowed: false}, nil
		}
		if err != nil {
			return articleReadAccess{}, err
		}
	} else {
		var chosenArticleID uuid.UUID
		err = tx.QueryRow(ctx,
			`SELECT article_id FROM user_category_daily_reads
			 WHERE user_id = $1 AND read_category = $2 AND read_date = CURRENT_DATE`,
			userID, category,
		).Scan(&chosenArticleID)
		if err == nil && chosenArticleID != articleID {
			return articleReadAccess{Allowed: false}, nil
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return articleReadAccess{}, err
		}
		if errors.Is(err, pgx.ErrNoRows) {
			if _, err := tx.Exec(ctx,
				`INSERT INTO user_category_daily_reads (user_id, read_category, article_id)
				 VALUES ($1, $2, $3)`,
				userID, category, articleID,
			); err != nil {
				return articleReadAccess{}, err
			}
		}
	}

	var expiresAt time.Time
	err = tx.QueryRow(ctx,
		`INSERT INTO user_article_access_grants (user_id, article_id, granted_at, expires_at)
		 VALUES ($1, $2, NOW(), NOW() + make_interval(days => $3))
		 ON CONFLICT (user_id, article_id) DO UPDATE SET
		   granted_at = EXCLUDED.granted_at,
		   expires_at = EXCLUDED.expires_at
		 RETURNING expires_at`,
		userID, articleID, articleAccessRetentionDays,
	).Scan(&expiresAt)
	if err != nil {
		return articleReadAccess{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return articleReadAccess{}, err
	}
	return articleReadAccess{Allowed: true, ExpiresAt: &expiresAt}, nil
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
