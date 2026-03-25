package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/customer"
	"golang.org/x/crypto/bcrypt"

	"github.com/PortNumber53/no-click-bait-news/backend/models"
)

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" || req.Name == "" {
		Error(w, http.StatusBadRequest, "Email, password, and name are required")
		return
	}

	// Check if email already exists
	var exists bool
	h.pool.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", req.Email).Scan(&exists)
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

	// Create Stripe customer
	stripe.Key = h.stripeKey
	cust, err := customer.New(&stripe.CustomerParams{
		Email: &req.Email,
		Name:  &req.Name,
		Params: stripe.Params{
			Metadata: map[string]string{"user_id": userID.String()},
		},
	})
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to create Stripe customer")
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer tx.Rollback(r.Context())

	_, err = tx.Exec(r.Context(),
		`INSERT INTO users (id, email, hashed_password, name, stripe_customer_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		userID, req.Email, string(hashed), req.Name, cust.ID, now, now,
	)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Assign free tier
	var freeTierID int
	err = tx.QueryRow(r.Context(), "SELECT id FROM subscription_tiers WHERE name = 'free'").Scan(&freeTierID)
	if err == nil {
		_, _ = tx.Exec(r.Context(),
			`INSERT INTO user_subscriptions (id, user_id, tier_id, status, created_at, updated_at)
			 VALUES ($1, $2, $3, 'active', $4, $5)`,
			uuid.New(), userID, freeTierID, now, now,
		)
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var user models.User
	var tierName *string
	err := h.pool.QueryRow(r.Context(),
		`SELECT u.id, u.email, u.hashed_password, u.name, u.created_at, st.name
		 FROM users u
		 LEFT JOIN user_subscriptions us ON us.user_id = u.id
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

func (h *Handler) createToken(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID.String(),
		"exp": time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(h.jwtSecret)
}
