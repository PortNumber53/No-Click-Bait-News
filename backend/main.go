package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PortNumber53/no-click-bait-news/backend/handlers"
	"github.com/PortNumber53/no-click-bait-news/backend/middleware"
	"github.com/PortNumber53/no-click-bait-news/backend/services"
)

func main() {
	_ = godotenv.Load()

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		runMigrate()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "fetch-content" {
		runFetchContent()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "crawl-news" {
		runCrawlNews()
		return
	}

	// Default: run the HTTP server
	runServer()
}

func runMigrate() {
	dbURL := mustEnv("DATABASE_URL")

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()

	log.Println("Running migrations...")
	if err := services.Migrate(ctx, pool); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Println("Migrations completed successfully")
}

func runFetchContent() {
	dbURL := mustEnv("DATABASE_URL")
	limit := 100
	if len(os.Args) > 2 {
		parsed, err := strconv.Atoi(os.Args[2])
		if err != nil || parsed < 1 {
			log.Fatalf("Usage: %s fetch-content [positive-limit]", os.Args[0])
		}
		limit = parsed
	}

	tinyFish, err := services.NewTinyFishClientFromEnv()
	if err != nil {
		log.Fatalf("Invalid TinyFish configuration: %v", err)
	}
	if tinyFish == nil {
		log.Fatal("Required environment variable TINYFISH_API_KEY not set")
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	stats, err := services.BackfillArticleContent(context.Background(), pool, tinyFish, limit)
	if err != nil {
		log.Fatalf("Fetch article content failed: %v", err)
	}
	log.Printf("Article content fetch complete: checked=%d updated=%d failed=%d", stats.Checked, stats.Updated, stats.Failed)
}

func runCrawlNews() {
	dbURL := mustEnv("DATABASE_URL")
	limit := 25
	if len(os.Args) > 2 {
		parsed, err := strconv.Atoi(os.Args[2])
		if err != nil || parsed < 1 {
			log.Fatalf("Usage: %s crawl-news [positive-limit]", os.Args[0])
		}
		limit = parsed
	}

	tinyFish, err := services.NewTinyFishClientFromEnv()
	if err != nil {
		log.Fatalf("Invalid TinyFish configuration: %v", err)
	}
	if tinyFish == nil {
		log.Fatal("Required environment variable TINYFISH_API_KEY not set")
	}

	articleRewriter, err := services.NewArticleRewriterFromEnv()
	if err != nil {
		log.Fatalf("Invalid LLM rewrite configuration: %v", err)
	}
	if articleRewriter == nil {
		log.Println("LLM article rewriting disabled for crawler: LLM_API_KEY and LLM_MODEL are not set")
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	start := time.Now()
	stats, err := services.CrawlMajorNews(context.Background(), pool, tinyFish, articleRewriter, limit)
	if err != nil {
		log.Fatalf("News crawl failed: %v", err)
	}
	log.Printf("News crawl complete: feeds=%d urls=%d inserted=%d skipped=%d rewritten=%d failed=%d elapsed=%s", stats.FeedsChecked, stats.URLsFound, stats.Inserted, stats.Skipped, stats.Rewritten, stats.Failed, time.Since(start))
}

func runServer() {
	dbURL := mustEnv("DATABASE_URL")
	jwtSecret := mustEnv("JWT_SECRET_KEY")
	stripeKey := mustEnv("STRIPE_SECRET_KEY")
	webhookSecret := mustEnv("STRIPE_WEBHOOK_SECRET")
	webhookSecretThin := os.Getenv("STRIPE_WEBHOOK_SECRET_THIN")
	webhookSecretSnapshot := os.Getenv("STRIPE_WEBHOOK_SECRET_SNAPSHOT")
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "http://localhost:21010"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "21011"
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to ping database: %v", err)
	}
	log.Println("Connected to database")

	// Sync subscription tiers with Stripe
	if err := services.SyncSubscriptionTiers(context.Background(), pool, stripeKey); err != nil {
		log.Printf("WARNING: Stripe sync failed: %v", err)
	}

	auth := middleware.NewAuth(jwtSecret, pool)
	tinyFish, err := services.NewTinyFishClientFromEnv()
	if err != nil {
		log.Fatalf("Invalid TinyFish configuration: %v", err)
	}
	if tinyFish == nil {
		log.Println("TinyFish content fetching disabled: TINYFISH_API_KEY is not set")
	}

	articleRewriter, err := services.NewArticleRewriterFromEnv()
	if err != nil {
		log.Fatalf("Invalid LLM rewrite configuration: %v", err)
	}
	if articleRewriter == nil {
		log.Println("LLM article rewriting disabled: LLM_API_KEY and LLM_MODEL are not set")
	}

	h := handlers.New(pool, jwtSecret, stripeKey, webhookSecret, webhookSecretThin, webhookSecretSnapshot, tinyFish, articleRewriter)

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   strings.Split(allowedOrigins, ","),
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		handlers.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		// Auth
		r.Post("/auth/register", h.Register)
		r.Post("/auth/login", h.Login)

		// Articles
		r.Group(func(r chi.Router) {
			r.Use(auth.OptionalUser)
			r.Get("/articles/feed", h.GetFeed)
			r.Get("/articles/{articleID}", h.GetArticle)
		})
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireUser)
			r.Get("/articles/my", h.GetMyArticles)
			r.Post("/articles/fetch", h.FetchArticle)
		})

		// Subscriptions
		r.Get("/subscriptions/tiers", h.GetTiers)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireUser)
			r.Post("/subscriptions/checkout", h.CreateCheckout)
		})
		r.Post("/subscriptions/webhook", h.StripeWebhook)
	})

	// Stripe webhooks — thin and snapshot payload formats
	r.Post("/webhook/stripe/thin", h.StripeWebhookThin)
	r.Post("/webhook/stripe/snapshot", h.StripeWebhookSnapshot)

	addr := fmt.Sprintf("0.0.0.0:%s", port)
	log.Printf("Server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("Required environment variable %s not set", key)
	}
	return v
}
