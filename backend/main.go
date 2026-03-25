package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

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

	// Ensure stripe_product_id column exists (idempotent)
	pool.Exec(context.Background(),
		"ALTER TABLE subscription_tiers ADD COLUMN IF NOT EXISTS stripe_product_id VARCHAR UNIQUE")

	// Sync subscription tiers with Stripe
	if err := services.SyncSubscriptionTiers(context.Background(), pool, stripeKey); err != nil {
		log.Printf("WARNING: Stripe sync failed: %v", err)
	}

	auth := middleware.NewAuth(jwtSecret, pool)
	h := handlers.New(pool, jwtSecret, stripeKey, webhookSecret, webhookSecretThin, webhookSecretSnapshot)

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{allowedOrigins},
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
