package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
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

	rewrites, err := h.loadCompletedRewrites(r.Context(), articleID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to fetch rewrites")
		return
	}

	if len(rewrites) < 2 {
		Error(w, http.StatusNotFound, "Comparison not available yet")
		return
	}

	// Pick a deterministic random pair for this user+article
	user := middleware.GetUser(r.Context())
	seed := articleID.String()
	if user != nil {
		seed += user.ID.String()
	}
	idxA, idxB := pickPair(seed, len(rewrites))
	resp.VersionA = rewrites[idxA]
	resp.VersionB = rewrites[idxB]
	// Comparisons are loaded eagerly by feed clients, including older app
	// versions that did not send the preview query parameter. Never expose full
	// article text or consume a daily category selection here. GetArticle is the
	// single entitlement gate and returns the selected rewrites with content.
	resp.VersionA.Content = nil
	resp.VersionB.Content = nil

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
			} else if chosenID == resp.VersionB.ID {
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
	if !DecodeJSON(w, r, &req) {
		return
	}
	if req.ChosenRewriteID == req.OtherRewriteID {
		Error(w, http.StatusBadRequest, "Rewrite choices must be distinct")
		return
	}
	rewrites, err := h.loadCompletedRewrites(r.Context(), articleID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to validate rewrite choices")
		return
	}
	if len(rewrites) < 2 {
		Error(w, http.StatusBadRequest, "Comparison is not available")
		return
	}
	idxA, idxB := pickPair(articleID.String()+user.ID.String(), len(rewrites))
	expectedA, expectedB := rewrites[idxA].ID, rewrites[idxB].ID
	validPair := (req.ChosenRewriteID == expectedA && req.OtherRewriteID == expectedB) ||
		(req.ChosenRewriteID == expectedB && req.OtherRewriteID == expectedA)
	if !validPair {
		Error(w, http.StatusBadRequest, "Rewrite choices do not match the presented comparison")
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
	user := middleware.GetUser(r.Context())
	if user == nil {
		Error(w, http.StatusUnauthorized, "Login required")
		return
	}

	// Get the pair the user voted on
	var chosenID, otherID uuid.UUID
	err := h.pool.QueryRow(r.Context(),
		`SELECT chosen_rewrite_id, other_rewrite_id FROM rewrite_votes
		 WHERE article_id = $1 AND user_id = $2`, articleID, user.ID,
	).Scan(&chosenID, &otherID)
	if err != nil {
		Error(w, http.StatusNotFound, "No vote found")
		return
	}

	type rewriteInfo struct {
		ID   uuid.UUID
		Name string
	}

	getInfo := func(rewriteID uuid.UUID) rewriteInfo {
		var ri rewriteInfo
		h.pool.QueryRow(r.Context(),
			`SELECT ar.id, lm.display_name FROM article_rewrites ar
			 JOIN llm_models lm ON lm.id = ar.llm_model_id
			 WHERE ar.id = $1 AND ar.article_id = $2 AND ar.processing_status = 'completed'`, rewriteID, articleID,
		).Scan(&ri.ID, &ri.Name)
		return ri
	}

	infoA := getInfo(chosenID)
	infoB := getInfo(otherID)

	var votesA, votesB int
	h.pool.QueryRow(r.Context(),
		"SELECT COUNT(*) FROM rewrite_votes WHERE article_id = $1 AND chosen_rewrite_id = $2", articleID, chosenID,
	).Scan(&votesA)
	h.pool.QueryRow(r.Context(),
		"SELECT COUNT(*) FROM rewrite_votes WHERE article_id = $1 AND chosen_rewrite_id = $2", articleID, otherID,
	).Scan(&votesB)

	JSON(w, http.StatusOK, models.VoteStatsResponse{
		ArticleID:     articleID,
		VersionAID:    infoA.ID,
		VersionAName:  infoA.Name,
		VersionAVotes: votesA,
		VersionBID:    infoB.ID,
		VersionBName:  infoB.Name,
		VersionBVotes: votesB,
	})
}

func (h *Handler) loadCompletedRewrites(ctx context.Context, articleID uuid.UUID) ([]models.RewriteVersion, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT ar.id, lm.display_name, ar.rewritten_title, ar.rewritten_summary, ar.rewritten_content
		 FROM article_rewrites ar
		 JOIN llm_models lm ON lm.id = ar.llm_model_id
		 WHERE ar.article_id = $1 AND ar.processing_status = 'completed'
		 ORDER BY ar.llm_model_id`, articleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rewrites := make([]models.RewriteVersion, 0)
	for rows.Next() {
		var rv models.RewriteVersion
		if err := rows.Scan(&rv.ID, &rv.ModelName, &rv.Title, &rv.Summary, &rv.Content); err != nil {
			return nil, err
		}
		rewrites = append(rewrites, rv)
	}
	return rewrites, rows.Err()
}

// pickPair returns two distinct indices from [0, n) deterministically based on seed.
func pickPair(seed string, n int) (int, int) {
	hash := sha256.Sum256([]byte(seed))
	h := new(big.Int).SetBytes(hash[:])

	idxA := int(new(big.Int).Mod(h, big.NewInt(int64(n))).Int64())

	// Use second half of hash for idxB, ensuring it differs from idxA
	hash2 := sha256.Sum256(hash[:])
	h2 := new(big.Int).SetBytes(hash2[:])
	idxB := int(new(big.Int).Mod(h2, big.NewInt(int64(n-1))).Int64())
	if idxB >= idxA {
		idxB++
	}

	// Use a third hash to decide A/B label order
	hash3 := sha256.Sum256(hash2[:])
	if binary.BigEndian.Uint32(hash3[:4])%2 == 1 {
		idxA, idxB = idxB, idxA
	}

	return idxA, idxB
}
