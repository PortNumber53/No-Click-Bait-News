package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ArticleContentBackfillStats struct {
	Checked int
	Updated int
	Failed  int
}

type articleContentCandidate struct {
	ID        uuid.UUID
	SourceURL string
}

func BackfillArticleContent(ctx context.Context, pool *pgxpool.Pool, tinyFish *TinyFishClient, limit int) (ArticleContentBackfillStats, error) {
	if tinyFish == nil {
		return ArticleContentBackfillStats{}, fmt.Errorf("tinyfish client is not configured")
	}
	if limit < 1 {
		limit = 100
	}

	rows, err := pool.Query(ctx,
		`SELECT id, source_url
		 FROM articles
		 WHERE source_url <> ''
		   AND (content IS NULL OR btrim(content) = '')
		 ORDER BY published_at DESC
		 LIMIT $1`, limit,
	)
	if err != nil {
		return ArticleContentBackfillStats{}, fmt.Errorf("query article content candidates: %w", err)
	}
	defer rows.Close()

	candidates := make([]articleContentCandidate, 0, limit)
	for rows.Next() {
		var candidate articleContentCandidate
		if err := rows.Scan(&candidate.ID, &candidate.SourceURL); err != nil {
			return ArticleContentBackfillStats{}, fmt.Errorf("scan article content candidate: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return ArticleContentBackfillStats{}, fmt.Errorf("iterate article content candidates: %w", err)
	}

	stats := ArticleContentBackfillStats{Checked: len(candidates)}
	for start := 0; start < len(candidates); start += 10 {
		end := start + 10
		if end > len(candidates) {
			end = len(candidates)
		}

		batch := candidates[start:end]
		urls := make([]string, 0, len(batch))
		idsByURL := make(map[string][]uuid.UUID, len(batch))
		for _, candidate := range batch {
			if _, seen := idsByURL[candidate.SourceURL]; !seen {
				urls = append(urls, candidate.SourceURL)
			}
			idsByURL[candidate.SourceURL] = append(idsByURL[candidate.SourceURL], candidate.ID)
		}

		pages, fetchErrors, err := tinyFish.FetchContents(ctx, urls)
		if err != nil {
			stats.Failed += len(batch)
			continue
		}
		for _, fetchError := range fetchErrors {
			if ids, ok := idsByURL[fetchError.URL]; ok {
				stats.Failed += len(ids)
			} else {
				stats.Failed++
			}
		}

		for url, page := range pages {
			content := strings.TrimSpace(page.Text)
			if content == "" {
				stats.Failed++
				continue
			}
			ids, ok := idsByURL[url]
			if !ok {
				continue
			}

			for _, id := range ids {
				tag, err := pool.Exec(ctx,
					`UPDATE articles
					 SET content = $1, original_content = $1, rewrite_status = 'pending', llm_rewrite_version = 0
					 WHERE id = $2`,
					content, id,
				)
				if err != nil {
					stats.Failed++
					continue
				}
				if tag.RowsAffected() > 0 {
					stats.Updated++
				}
			}
		}
	}

	return stats, nil
}
