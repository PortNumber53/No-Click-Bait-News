package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
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
	webhookSecretThin := os.Getenv("STRIPE_WEBHOOK_SECRET_THIN")
	webhookSecretSnapshot := mustEnv("STRIPE_WEBHOOK_SECRET_SNAPSHOT")
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

	articleRewriters, err := services.NewArticleRewritersFromEnv()
	if err != nil {
		log.Fatalf("Invalid LLM rewrite configuration: %v", err)
	}
	if len(articleRewriters) == 0 {
		log.Println("LLM article rewriting disabled: LLM_API_KEY and LLM_MODEL are not set")
	} else {
		log.Printf("LLM article rewriting enabled: models=%d", len(articleRewriters))
	}

	h := handlers.New(pool, jwtSecret, stripeKey, webhookSecretThin, webhookSecretSnapshot, tinyFish, articleRewriters)

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
		authLimiter := middleware.NewRateLimiter(20, 10*time.Minute)
		r.With(authLimiter.Handler).Post("/auth/register", h.Register)
		r.With(authLimiter.Handler).Post("/auth/login", h.Login)

		// Articles & Subscriptions (optional auth)
		r.Group(func(r chi.Router) {
			r.Use(auth.OptionalUser)
			r.Get("/articles/feed", h.GetFeed)
			r.Get("/articles/{articleID}", h.GetArticle)
			r.Get("/articles/{articleID}/comparison", h.GetComparison)
			r.Get("/articles/{articleID}/vote-stats", h.GetVoteStats)
			r.Post("/articles/{articleID}/vote", h.SubmitVote)
			r.Get("/subscriptions/tiers", h.GetTiers)
		})
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireUser)
			r.Get("/articles/my", h.GetMyArticles)
			r.Post("/articles/fetch", h.FetchArticle)
		})

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireUser)
			r.Get("/auth/me", h.GetMe)
			r.Post("/subscriptions/checkout", h.CreateCheckout)
		})
	})

	// Stripe webhooks
	r.Post("/webhook/stripe/snapshot", h.StripeWebhookSnapshot)
	if webhookSecretThin != "" {
		r.Post("/webhook/stripe/thin", h.StripeWebhookThin)
	}

	addr := fmt.Sprintf("0.0.0.0:%s", port)
	log.Printf("Server listening on %s", addr)
	server := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-shutdownSignal
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown failed: %v", err)
		}
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("Required environment variable %s not set", key)
	}
	return v
}
