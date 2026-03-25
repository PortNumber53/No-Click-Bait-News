package models

import (
	"time"

	"github.com/google/uuid"
)

// Database models

type User struct {
	ID               uuid.UUID  `json:"id"`
	Email            string     `json:"email"`
	HashedPassword   string     `json:"-"`
	Name             string     `json:"name"`
	StripeCustomerID *string    `json:"stripe_customer_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type Article struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Summary     string     `json:"summary"`
	Content     *string    `json:"content"`
	SourceName  string     `json:"source_name"`
	SourceURL   string     `json:"source_url"`
	ImageURL    *string    `json:"image_url"`
	Category    *string    `json:"category"`
	PublishedAt time.Time  `json:"published_at"`
	IsPremium   bool       `json:"is_premium"`
	ViewCount   int        `json:"view_count"`
	CreatedAt   time.Time  `json:"created_at"`
}

type SubscriptionTier struct {
	ID                int     `json:"id"`
	Name              string  `json:"name"`
	StripeProductID   *string `json:"-"`
	StripePriceID     *string `json:"-"`
	PriceMonthly      float64 `json:"price_monthly"`
	MaxArticlesPerDay int     `json:"max_articles_per_day"`
	HasPremiumAccess  bool    `json:"has_premium_access"`
}

type UserSubscription struct {
	ID                   uuid.UUID  `json:"id"`
	UserID               uuid.UUID  `json:"user_id"`
	TierID               int        `json:"tier_id"`
	StripeSubscriptionID *string    `json:"stripe_subscription_id,omitempty"`
	Status               string     `json:"status"`
	CurrentPeriodStart   *time.Time `json:"current_period_start,omitempty"`
	CurrentPeriodEnd     *time.Time `json:"current_period_end,omitempty"`
}

// Request/Response types

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserResponse struct {
	ID               uuid.UUID `json:"id"`
	Email            string    `json:"email"`
	Name             string    `json:"name"`
	CreatedAt        time.Time `json:"created_at"`
	SubscriptionTier *string   `json:"subscription_tier"`
}

type TokenResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	User        UserResponse `json:"user"`
}

type ArticleResponse struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Summary     string     `json:"summary"`
	Content     *string    `json:"content"`
	SourceName  string     `json:"source_name"`
	SourceURL   string     `json:"source_url"`
	ImageURL    *string    `json:"image_url"`
	Category    *string    `json:"category"`
	PublishedAt time.Time  `json:"published_at"`
	IsPremium   bool       `json:"is_premium"`
	ViewCount   int        `json:"view_count"`
}

type FeedResponse struct {
	Articles []ArticleResponse `json:"articles"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	HasMore  bool              `json:"has_more"`
}

type TierResponse struct {
	ID               int     `json:"id"`
	Name             string  `json:"name"`
	PriceMonthly     float64 `json:"price_monthly"`
	MaxArticlesPerDay int    `json:"max_articles_per_day"`
	HasPremiumAccess bool    `json:"has_premium_access"`
}

type CheckoutRequest struct {
	TierID int `json:"tier_id"`
}

type CheckoutResponse struct {
	CheckoutURL string `json:"checkout_url"`
	SessionID   string `json:"session_id"`
}
