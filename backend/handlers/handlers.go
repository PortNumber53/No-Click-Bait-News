package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/PortNumber53/no-click-bait-news/backend/services"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	pool                  *pgxpool.Pool
	jwtSecret             []byte
	stripeKey             string
	webhookSecret         string
	webhookSecretThin     string
	webhookSecretSnapshot string
	tinyFish              *services.TinyFishClient
	articleRewriter       *services.ArticleRewriter
	rewriteJobs           chan articleRewriteJob
}

func New(pool *pgxpool.Pool, jwtSecret, stripeKey, webhookSecret, webhookSecretThin, webhookSecretSnapshot string, tinyFish *services.TinyFishClient, articleRewriter *services.ArticleRewriter) *Handler {
	h := &Handler{
		pool:                  pool,
		jwtSecret:             []byte(jwtSecret),
		stripeKey:             stripeKey,
		webhookSecret:         webhookSecret,
		webhookSecretThin:     webhookSecretThin,
		webhookSecretSnapshot: webhookSecretSnapshot,
		tinyFish:              tinyFish,
		articleRewriter:       articleRewriter,
	}
	h.startArticleRewriteWorkers()
	return h
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, status int, detail string) {
	JSON(w, status, map[string]string{"detail": detail})
}
