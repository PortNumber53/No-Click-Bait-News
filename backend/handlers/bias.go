package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/PortNumber53/no-click-bait-news/backend/middleware"
	"github.com/PortNumber53/no-click-bait-news/backend/models"
)

func (h *Handler) RevealBiasReasoning(w http.ResponseWriter, r *http.Request) {
	articleID, err := uuid.Parse(chi.URLParam(r, "articleID"))
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid article ID")
		return
	}
	rewriteID, err := uuid.Parse(chi.URLParam(r, "rewriteID"))
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid rewrite ID")
		return
	}
	user := middleware.GetUser(r.Context())
	if user == nil {
		Error(w, http.StatusUnauthorized, "Sign in to view bias reasoning")
		return
	}

	var label, reasoning string
	err = h.pool.QueryRow(r.Context(),
		`SELECT bias_label, bias_reasoning
		 FROM article_rewrites
		 WHERE id = $1 AND article_id = $2 AND processing_status = 'completed'
		   AND bias_label IS NOT NULL AND bias_reasoning IS NOT NULL`,
		rewriteID, articleID,
	).Scan(&label, &reasoning)
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "Bias analysis is not available yet")
		return
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to load bias analysis")
		return
	}

	access, err := h.getReadingEntitlement(r.Context(), user.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to check bias reasoning access")
		return
	}
	response := models.BiasReasoningResponse{
		ArticleID:     articleID,
		RewriteID:     rewriteID,
		BiasLabel:     label,
		BiasReasoning: reasoning,
	}
	if !isPaidTier(access) {
		allowed, err := h.reserveDailyUsage(r.Context(), user.ID, freeBiasReasoningsPerDay,
			"SELECT EXISTS(SELECT 1 FROM user_bias_reasoning_reads WHERE user_id = $1 AND rewrite_id = $2 AND read_date = CURRENT_DATE)",
			"SELECT COUNT(*) FROM user_bias_reasoning_reads WHERE user_id = $1 AND read_date = CURRENT_DATE",
			"INSERT INTO user_bias_reasoning_reads (user_id, rewrite_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
			rewriteID,
		)
		if err != nil {
			Error(w, http.StatusInternalServerError, "Failed to record bias reasoning usage")
			return
		}
		if !allowed {
			Error(w, http.StatusTooManyRequests, "Your free plan includes 5 bias explanations per day. Upgrade for unlimited explanations.")
			return
		}
		var used int
		if err := h.pool.QueryRow(r.Context(),
			"SELECT COUNT(*) FROM user_bias_reasoning_reads WHERE user_id = $1 AND read_date = CURRENT_DATE",
			user.ID,
		).Scan(&used); err != nil {
			Error(w, http.StatusInternalServerError, "Failed to check remaining bias explanations")
			return
		}
		remaining := max(0, freeBiasReasoningsPerDay-used)
		response.RemainingToday = &remaining
	}

	JSON(w, http.StatusOK, response)
}
