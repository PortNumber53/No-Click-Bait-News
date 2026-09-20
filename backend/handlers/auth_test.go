package handlers

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestCreateTokenUsesRequestedSessionLifetime(t *testing.T) {
	h := &Handler{jwtSecret: []byte("test-secret")}

	for _, test := range []struct {
		name       string
		rememberMe bool
		want       time.Duration
	}{
		{name: "session", want: standardSessionLifetime},
		{name: "remembered", rememberMe: true, want: rememberedSessionLifetime},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw, err := h.createToken(uuid.New(), test.rememberMe)
			if err != nil {
				t.Fatalf("createToken returned error: %v", err)
			}
			parsed, err := jwt.Parse(raw, func(token *jwt.Token) (any, error) {
				return h.jwtSecret, nil
			})
			if err != nil || !parsed.Valid {
				t.Fatalf("parse token: %v", err)
			}
			claims := parsed.Claims.(jwt.MapClaims)
			iat := int64(claims["iat"].(float64))
			exp := int64(claims["exp"].(float64))
			if got := time.Duration(exp-iat) * time.Second; got != test.want {
				t.Fatalf("token lifetime = %s, want %s", got, test.want)
			}
		})
	}
}
