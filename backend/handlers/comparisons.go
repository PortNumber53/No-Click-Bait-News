package handlers

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/PortNumber53/no-click-bait-news/backend/middleware"
	"github.com/PortNumber53/no-click-bait-news/backend/models"
)

func (h *Handler) GetComparison(w http.ResponseWriter, r *http.Request) {
	articleID, err := uuid.Parse(chi.URLParam(r, "articleID"))
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid article ID")
		return
	}

	// Get article metadata
	var resp models.ComparisonResponse
	err = h.pool.QueryRow(r.Context(),
		`SELECT id, COALESCE(original_title, title), source_name, source_url, image_url, category, published_at
		 FROM articles WHERE id = $1`, articleID,
	).Scan(&resp.ArticleID, &resp.OriginalTitle, &resp.SourceName, &resp.SourceURL,
		&resp.ImageURL, &resp.Category, &resp.PublishedAt)
	if err != nil {
		Error(w, http.StatusNotFound, "Article not found")
		return
	}

	// Get completed rewrites (expect exactly 2)
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, rewritten_title, rewritten_summary, rewritten_content
		 FROM article_rewrites
		 WHERE article_id = $1 AND processing_status = 'completed'
		 ORDER BY llm_model_id`, articleID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to fetch rewrites")
		return
	}
	defer rows.Close()

	var rewrites []models.RewriteVersion
	for rows.Next() {
		var rv models.RewriteVersion
		if err := rows.Scan(&rv.ID, &rv.Title, &rv.Summary, &rv.Content); err != nil {
			continue
		}
		rewrites = append(rewrites, rv)
	}

	if len(rewrites) < 2 {
		Error(w, http.StatusNotFound, "Comparison not available yet")
		return
	}

	// Deterministic A/B assignment based on article + user
	swap := false
	user := middleware.GetUser(r.Context())
	if user != nil {
		hash := sha256.Sum256([]byte(articleID.String() + user.ID.String()))
		swap = binary.BigEndian.Uint32(hash[:4])%2 == 1
	}

	if swap {
		resp.VersionA = rewrites[1]
		resp.VersionB = rewrites[0]
	} else {
		resp.VersionA = rewrites[0]
		resp.VersionB = rewrites[1]
	}

	// Check if user already voted
	if user != nil {
		var chosenID uuid.UUID
		err := h.pool.QueryRow(r.Context(),
			`SELECT chosen_rewrite_id FROM rewrite_votes
			 WHERE article_id = $1 AND user_id = $2`, articleID, user.ID,
		).Scan(&chosenID)
		if err == nil {
			if chosenID == resp.VersionA.ID {
				v := "a"
				resp.UserVote = &v
			} else {
				v := "b"
				resp.UserVote = &v
			}
		}
	}

	JSON(w, http.StatusOK, resp)
}

func (h *Handler) SubmitVote(w http.ResponseWriter, r *http.Request) {
	articleID, err := uuid.Parse(chi.URLParam(r, "articleID"))
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid article ID")
		return
	}

	user := middleware.GetUser(r.Context())
	if user == nil {
		Error(w, http.StatusUnauthorized, "Login required to vote")
		return
	}

	var req models.VoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	_, err = h.pool.Exec(r.Context(),
		`INSERT INTO rewrite_votes (article_id, user_id, chosen_rewrite_id, other_rewrite_id)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (article_id, user_id) WHERE user_id IS NOT NULL DO NOTHING`,
		articleID, user.ID, req.ChosenRewriteID, req.OtherRewriteID,
	)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to save vote")
		return
	}

	h.sendVoteStats(w, r, articleID)
}

func (h *Handler) GetVoteStats(w http.ResponseWriter, r *http.Request) {
	articleID, err := uuid.Parse(chi.URLParam(r, "articleID"))
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid article ID")
		return
	}

	// Only reveal stats if user has voted
	user := middleware.GetUser(r.Context())
	if user != nil {
		var exists bool
		h.pool.QueryRow(r.Context(),
			"SELECT EXISTS(SELECT 1 FROM rewrite_votes WHERE article_id = $1 AND user_id = $2)",
			articleID, user.ID,
		).Scan(&exists)
		if !exists {
			Error(w, http.StatusForbidden, "Vote first to see results")
			return
		}
	}

	h.sendVoteStats(w, r, articleID)
}

func (h *Handler) sendVoteStats(w http.ResponseWriter, r *http.Request, articleID uuid.UUID) {
	// Get the two rewrites with their model names
	rows, err := h.pool.Query(r.Context(),
		`SELECT ar.id, lm.display_name
		 FROM article_rewrites ar
		 JOIN llm_models lm ON lm.id = ar.llm_model_id
		 WHERE ar.article_id = $1 AND ar.processing_status = 'completed'
		 ORDER BY ar.llm_model_id`, articleID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to fetch stats")
		return
	}
	defer rows.Close()

	type rewriteInfo struct {
		ID   uuid.UUID
		Name string
	}
	var infos []rewriteInfo
	for rows.Next() {
		var ri rewriteInfo
		if err := rows.Scan(&ri.ID, &ri.Name); err != nil {
			continue
		}
		infos = append(infos, ri)
	}

	if len(infos) < 2 {
		Error(w, http.StatusNotFound, "Stats not available")
		return
	}

	// Count votes for each
	var votesA, votesB int
	h.pool.QueryRow(r.Context(),
		"SELECT COUNT(*) FROM rewrite_votes WHERE chosen_rewrite_id = $1", infos[0].ID,
	).Scan(&votesA)
	h.pool.QueryRow(r.Context(),
		"SELECT COUNT(*) FROM rewrite_votes WHERE chosen_rewrite_id = $1", infos[1].ID,
	).Scan(&votesB)

	JSON(w, http.StatusOK, models.VoteStatsResponse{
		ArticleID:     articleID,
		VersionAID:    infos[0].ID,
		VersionAName:  infos[0].Name,
		VersionAVotes: votesA,
		VersionBID:    infos[1].ID,
		VersionBName:  infos[1].Name,
		VersionBVotes: votesB,
	})
}
