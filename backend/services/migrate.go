package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate creates all tables and seeds initial data. Idempotent.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}
	log.Println("Connected to database")

	for _, ddl := range schemaDDL {
		if _, err := pool.Exec(ctx, ddl); err != nil {
			return fmt.Errorf("schema migration: %w", err)
		}
	}
	log.Println("Schema up to date")

	if err := seedSubscriptionTiers(ctx, pool); err != nil {
		return fmt.Errorf("seed tiers: %w", err)
	}

	if err := seedLLMModels(ctx, pool); err != nil {
		return fmt.Errorf("seed llm models: %w", err)
	}

	if err := seedSampleArticles(ctx, pool); err != nil {
		return fmt.Errorf("seed articles: %w", err)
	}

	return nil
}

var schemaDDL = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY,
		email VARCHAR NOT NULL UNIQUE,
		hashed_password VARCHAR NOT NULL,
		name VARCHAR NOT NULL,
		stripe_customer_id VARCHAR UNIQUE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS ix_users_email ON users (email)`,

	`CREATE TABLE IF NOT EXISTS articles (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		title VARCHAR NOT NULL,
		summary TEXT NOT NULL,
		content TEXT,
		original_content TEXT,
		rewrite_status VARCHAR NOT NULL DEFAULT 'complete',
		llm_rewrite_version INTEGER NOT NULL DEFAULT 0,
		submitted_by_user_id UUID REFERENCES users(id),
		source_name VARCHAR NOT NULL,
		source_url VARCHAR NOT NULL,
		image_url VARCHAR,
		category VARCHAR,
		categories TEXT[],
		published_at TIMESTAMPTZ NOT NULL,
		is_premium BOOLEAN NOT NULL DEFAULT false,
		view_count INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`ALTER TABLE articles ADD COLUMN IF NOT EXISTS original_content TEXT`,
	`ALTER TABLE articles ADD COLUMN IF NOT EXISTS rewrite_status VARCHAR NOT NULL DEFAULT 'complete'`,
	`ALTER TABLE articles ADD COLUMN IF NOT EXISTS llm_rewrite_version INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE articles ADD COLUMN IF NOT EXISTS submitted_by_user_id UUID REFERENCES users(id)`,
	`ALTER TABLE articles ADD COLUMN IF NOT EXISTS categories TEXT[]`,
	`UPDATE articles
	 SET categories = ARRAY[category]
	 WHERE categories IS NULL AND category IS NOT NULL AND category <> ''`,
	`CREATE INDEX IF NOT EXISTS ix_articles_category ON articles (category)`,
	`CREATE INDEX IF NOT EXISTS ix_articles_categories ON articles USING GIN (categories)`,
	`CREATE INDEX IF NOT EXISTS ix_articles_published_at ON articles (published_at)`,
	`CREATE INDEX IF NOT EXISTS ix_articles_submitted_by_user_id ON articles (submitted_by_user_id)`,

	`CREATE TABLE IF NOT EXISTS subscription_tiers (
		id SERIAL PRIMARY KEY,
		name VARCHAR NOT NULL UNIQUE,
		stripe_product_id VARCHAR UNIQUE,
		stripe_price_id VARCHAR UNIQUE,
		price_monthly NUMERIC(10,2) NOT NULL DEFAULT 0,
		max_articles_per_day INTEGER NOT NULL DEFAULT 10,
		has_premium_access BOOLEAN NOT NULL DEFAULT false,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,

	`CREATE TABLE IF NOT EXISTS user_subscriptions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL UNIQUE REFERENCES users(id),
		tier_id INTEGER NOT NULL REFERENCES subscription_tiers(id),
		stripe_subscription_id VARCHAR UNIQUE,
		status VARCHAR NOT NULL DEFAULT 'active',
		current_period_start TIMESTAMPTZ,
		current_period_end TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,

	// LLM comparison system
	`ALTER TABLE articles ADD COLUMN IF NOT EXISTS original_title VARCHAR`,
	`ALTER TABLE articles ADD COLUMN IF NOT EXISTS original_summary TEXT`,
	`ALTER TABLE articles ADD COLUMN IF NOT EXISTS fetch_status VARCHAR NOT NULL DEFAULT 'raw'`,

	`CREATE TABLE IF NOT EXISTS llm_models (
		id SERIAL PRIMARY KEY,
		slug VARCHAR NOT NULL UNIQUE,
		display_name VARCHAR NOT NULL,
		openrouter_model_id VARCHAR NOT NULL,
		is_active BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,

	`CREATE TABLE IF NOT EXISTS article_rewrites (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		article_id UUID NOT NULL REFERENCES articles(id),
		llm_model_id INTEGER NOT NULL REFERENCES llm_models(id),
		rewritten_title VARCHAR NOT NULL,
		rewritten_summary TEXT NOT NULL,
		rewritten_content TEXT,
		processing_status VARCHAR NOT NULL DEFAULT 'pending',
		error_message TEXT,
		prompt_tokens INTEGER,
		completion_tokens INTEGER,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE(article_id, llm_model_id)
	)`,

	`CREATE TABLE IF NOT EXISTS rewrite_votes (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		article_id UUID NOT NULL REFERENCES articles(id),
		user_id UUID REFERENCES users(id),
		chosen_rewrite_id UUID NOT NULL REFERENCES article_rewrites(id),
		other_rewrite_id UUID NOT NULL REFERENCES article_rewrites(id),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ix_rewrite_votes_user_article
		ON rewrite_votes (article_id, user_id) WHERE user_id IS NOT NULL`,
}

func seedSubscriptionTiers(ctx context.Context, pool *pgxpool.Pool) error {
	tiers := []struct {
		Name              string
		PriceMonthly      float64
		MaxArticlesPerDay int
		HasPremiumAccess  bool
	}{
		{"free", 0, 10, false},
		{"basic", 4.99, 50, false},
		{"premium", 9.99, 999, true},
	}

	for _, t := range tiers {
		_, err := pool.Exec(ctx,
			`INSERT INTO subscription_tiers (name, price_monthly, max_articles_per_day, has_premium_access)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (name) DO NOTHING`,
			t.Name, t.PriceMonthly, t.MaxArticlesPerDay, t.HasPremiumAccess,
		)
		if err != nil {
			return err
		}
	}
	log.Println("Subscription tiers seeded")
	return nil
}

func seedLLMModels(ctx context.Context, pool *pgxpool.Pool) error {
	models := []struct {
		Slug, DisplayName, OpenRouterID string
	}{
		{"anthropic/claude-sonnet-4", "Claude Sonnet 4", "anthropic/claude-sonnet-4"},
		{"openai/gpt-4o", "GPT-4o", "openai/gpt-4o"},
	}
	for _, m := range models {
		_, err := pool.Exec(ctx,
			`INSERT INTO llm_models (slug, display_name, openrouter_model_id)
			 VALUES ($1, $2, $3) ON CONFLICT (slug) DO NOTHING`,
			m.Slug, m.DisplayName, m.OpenRouterID,
		)
		if err != nil {
			return err
		}
	}
	log.Println("LLM models seeded")
	return nil
}

func seedSampleArticles(ctx context.Context, pool *pgxpool.Pool) error {
	if strings.ToLower(os.Getenv("SEED_SAMPLE_ARTICLES")) != "true" {
		log.Println("Sample article seeding disabled")
		return nil
	}

	// Only seed if no articles exist
	var count int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM articles").Scan(&count)
	if count > 0 {
		log.Printf("Articles table already has %d rows, skipping seed", count)
		return nil
	}

	categories := []string{"Technology", "Science", "Business", "Health", "Sports", "World"}
	now := time.Now().UTC()

	for i := 0; i < 60; i++ {
		cat := categories[i%len(categories)]
		isPremium := i%5 == 0
		publishedAt := now.Add(-time.Duration(i) * time.Hour)

		_, err := pool.Exec(ctx,
			`INSERT INTO articles (title, summary, content, source_name, source_url, image_url, category, published_at, is_premium)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			fmt.Sprintf("Sample %s Article #%d: Important Developments Today", cat, i+1),
			fmt.Sprintf("A straightforward summary of key %s developments without sensationalism.", cat),
			fmt.Sprintf("Full article content for %s article #%d. This is a detailed, factual report without clickbait headlines.", cat, i+1),
			"No-Click Bait News",
			fmt.Sprintf("https://example.com/articles/%d", i+1),
			fmt.Sprintf("https://picsum.photos/seed/%d/800/400", i+1),
			cat,
			publishedAt,
			isPremium,
		)
		if err != nil {
			return err
		}
	}
	log.Println("Sample articles seeded (60 articles)")
	return nil
}
