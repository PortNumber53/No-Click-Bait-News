package handlers

import (
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/PortNumber53/no-click-bait-news/backend/middleware"
	"github.com/PortNumber53/no-click-bait-news/backend/models"
)

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if !DecodeJSON(w, r, &req) {
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	if req.Email == "" || req.Password == "" || req.Name == "" {
		Error(w, http.StatusBadRequest, "Email, password, and name are required")
		return
	}
	if len(req.Email) > 254 || len(req.Name) > 100 || len(req.Password) < 8 || len(req.Password) > 72 {
		Error(w, http.StatusBadRequest, "Use a valid email and an 8-72 character password")
		return
	}
	if parsed, err := mail.ParseAddress(req.Email); err != nil || parsed.Address != req.Email {
		Error(w, http.StatusBadRequest, "Enter a valid email address")
		return
	}

	// Check if email already exists
	var exists bool
	if err := h.pool.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", req.Email).Scan(&exists); err != nil {
		Error(w, http.StatusInternalServerError, "Failed to check email availability")
		return
	}
	if exists {
		Error(w, http.StatusBadRequest, "Email already registered")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	userID := uuid.New()
	now := time.Now().UTC()

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer tx.Rollback(r.Context())

	_, err = tx.Exec(r.Context(),
		`INSERT INTO users (id, email, hashed_password, name, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, req.Email, string(hashed), req.Name, now, now,
	)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Assign free tier
	var freeTierID int
	err = tx.QueryRow(r.Context(), "SELECT id FROM subscription_tiers WHERE name = 'free'").Scan(&freeTierID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to assign subscription tier")
		return
	}
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO user_subscriptions (id, user_id, tier_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, 'active', $4, $5)`,
		uuid.New(), userID, freeTierID, now, now,
	); err != nil {
		Error(w, http.StatusInternalServerError, "Failed to assign subscription tier")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		Error(w, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	token, err := h.createToken(userID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to create token")
		return
	}

	tierName := "free"
	JSON(w, http.StatusCreated, models.TokenResponse{
		AccessToken: token,
		TokenType:   "bearer",
		User: models.UserResponse{
			ID:               userID,
			Email:            req.Email,
			Name:             req.Name,
			CreatedAt:        now,
			SubscriptionTier: &tierName,
		},
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if !DecodeJSON(w, r, &req) {
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var user models.User
	var tierName *string
	err := h.pool.QueryRow(r.Context(),
		`SELECT u.id, u.email, u.hashed_password, u.name, u.created_at,
		        CASE WHEN st.unlimited_reading OR st.price_monthly > 0 THEN 'premium'
		             ELSE COALESCE(st.name, 'free') END
		 FROM users u
		 LEFT JOIN user_subscriptions us ON us.user_id = u.id AND us.status IN ('active', 'trialing')
		 LEFT JOIN subscription_tiers st ON st.id = us.tier_id
		 WHERE u.email = $1`, req.Email,
	).Scan(&user.ID, &user.Email, &user.HashedPassword, &user.Name, &user.CreatedAt, &tierName)
	if err != nil {
		Error(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(req.Password)); err != nil {
		Error(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	token, err := h.createToken(user.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to create token")
		return
	}

	JSON(w, http.StatusOK, models.TokenResponse{
		AccessToken: token,
		TokenType:   "bearer",
		User: models.UserResponse{
			ID:               user.ID,
			Email:            user.Email,
			Name:             user.Name,
			CreatedAt:        user.CreatedAt,
			SubscriptionTier: tierName,
		},
	})
}

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		Error(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var tierName *string
	h.pool.QueryRow(r.Context(),
		`SELECT COALESCE(
			(SELECT CASE WHEN st.unlimited_reading OR st.price_monthly > 0 THEN 'premium' ELSE st.name END
			 FROM user_subscriptions us
			 JOIN subscription_tiers st ON st.id = us.tier_id
			 WHERE us.user_id = $1 AND us.status IN ('active', 'trialing')),
			'free'
		)`, user.ID,
	).Scan(&tierName)

	JSON(w, http.StatusOK, models.UserResponse{
		ID:               user.ID,
		Email:            user.Email,
		Name:             user.Name,
		CreatedAt:        user.CreatedAt,
		SubscriptionTier: tierName,
	})
}

func (h *Handler) createToken(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID.String(),
		"iss": "no-click-bait-news",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(h.jwtSecret)
}
