package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/PortNumber53/no-click-bait-news/backend/services"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	pool                  *pgxpool.Pool
	jwtSecret             []byte
	stripeKey             string
	webhookSecretThin     string
	webhookSecretSnapshot string
	tinyFish              *services.TinyFishClient
	articleRewriter       *services.ArticleRewriter
	articleRewriters      []*services.ArticleRewriter
	rewriteWake           chan struct{}
}

func New(pool *pgxpool.Pool, jwtSecret, stripeKey, webhookSecretThin, webhookSecretSnapshot string, tinyFish *services.TinyFishClient, articleRewriters []*services.ArticleRewriter) *Handler {
	h := &Handler{
		pool:                  pool,
		jwtSecret:             []byte(jwtSecret),
		stripeKey:             stripeKey,
		webhookSecretThin:     webhookSecretThin,
		webhookSecretSnapshot: webhookSecretSnapshot,
		tinyFish:              tinyFish,
		articleRewriters:      articleRewriters,
	}
	if len(articleRewriters) > 0 {
		h.articleRewriter = articleRewriters[0]
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

func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		Error(w, http.StatusBadRequest, "Request body must contain one JSON object")
		return false
	}
	return true
}
