package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PortNumber53/no-click-bait-news/backend/models"
)

type contextKey string

const UserKey contextKey = "user"

type Auth struct {
	secret []byte
	pool   *pgxpool.Pool
}

func NewAuth(secret string, pool *pgxpool.Pool) *Auth {
	return &Auth{secret: []byte(secret), pool: pool}
}

func (a *Auth) parseToken(r *http.Request) (*models.User, error) {
	header := r.Header.Get("Authorization")
	if header == "" || !strings.HasPrefix(header, "Bearer ") {
		return nil, nil
	}

	tokenStr := strings.TrimPrefix(header, "Bearer ")
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return a.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, nil
	}

	sub, _ := claims.GetSubject()
	userID, err := uuid.Parse(sub)
	if err != nil {
		return nil, err
	}

	var user models.User
	var tierName *string
	err = a.pool.QueryRow(r.Context(),
		`SELECT u.id, u.email, u.name, u.hashed_password, u.stripe_customer_id, u.created_at, u.updated_at, st.name
		 FROM users u
		 LEFT JOIN user_subscriptions us ON us.user_id = u.id
		 LEFT JOIN subscription_tiers st ON st.id = us.tier_id
		 WHERE u.id = $1`, userID,
	).Scan(&user.ID, &user.Email, &user.Name, &user.HashedPassword, &user.StripeCustomerID, &user.CreatedAt, &user.UpdatedAt, &tierName)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (a *Auth) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := a.parseToken(r)
		if err != nil || user == nil {
			http.Error(w, `{"detail":"Not authenticated"}`, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), UserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Auth) OptionalUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := a.parseToken(r)
		if user != nil {
			ctx := context.WithValue(r.Context(), UserKey, user)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func GetUser(ctx context.Context) *models.User {
	user, _ := ctx.Value(UserKey).(*models.User)
	return user
}
