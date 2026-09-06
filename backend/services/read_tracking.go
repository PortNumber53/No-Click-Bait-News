package services

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ReadTrackingCleanupStats reports how many bounded entitlement records were
// removed. Monthly usage is aggregated, so only one current-month row is kept
// per user instead of a row for every article.
type ReadTrackingCleanupStats struct {
	ExpiredGrants       int64
	ExpiredFreeReads    int64
	ExpiredMonthlyUsage int64
}

func CleanupReadTracking(ctx context.Context, pool *pgxpool.Pool) (ReadTrackingCleanupStats, error) {
	var stats ReadTrackingCleanupStats

	tag, err := pool.Exec(ctx, "DELETE FROM user_article_access_grants WHERE expires_at <= NOW()")
	if err != nil {
		return stats, err
	}
	stats.ExpiredGrants = tag.RowsAffected()

	// CURRENT_DATE plus the previous six dates is a seven-day rolling window.
	tag, err = pool.Exec(ctx, "DELETE FROM user_category_daily_reads WHERE read_date < CURRENT_DATE - 6")
	if err != nil {
		return stats, err
	}
	stats.ExpiredFreeReads = tag.RowsAffected()

	tag, err = pool.Exec(ctx, `DELETE FROM user_monthly_read_usage
		WHERE period_start < date_trunc('month', CURRENT_DATE)::date`)
	if err != nil {
		return stats, err
	}
	stats.ExpiredMonthlyUsage = tag.RowsAffected()

	return stats, nil
}

func RunReadTrackingCleanup(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) {
	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		stats, err := CleanupReadTracking(cleanupCtx, pool)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("WARNING: read tracking cleanup failed: %v", err)
			}
			return
		}
		if stats.ExpiredGrants+stats.ExpiredFreeReads+stats.ExpiredMonthlyUsage > 0 {
			log.Printf("Read tracking cleanup: grants=%d free_reads=%d monthly_usage=%d",
				stats.ExpiredGrants, stats.ExpiredFreeReads, stats.ExpiredMonthlyUsage)
		}
	}

	cleanup()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
}
