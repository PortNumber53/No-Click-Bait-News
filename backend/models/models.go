package models

import (
	"time"

	"github.com/google/uuid"
)

// Database models

type User struct {
	ID               uuid.UUID `json:"id"`
	Email            string    `json:"email"`
	HashedPassword   string    `json:"-"`
	Name             string    `json:"name"`
	StripeCustomerID *string   `json:"stripe_customer_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Article struct {
	ID                uuid.UUID `json:"id"`
	Title             string    `json:"title"`
	Summary           string    `json:"summary"`
	Content           *string   `json:"content"`
	OriginalContent   *string   `json:"original_content,omitempty"`
	RewriteStatus     string    `json:"rewrite_status"`
	LLMRewriteVersion int       `json:"llm_rewrite_version"`
	SourceName        string    `json:"source_name"`
	SourceURL         string    `json:"source_url"`
	ImageURL          *string   `json:"image_url"`
	Category          *string   `json:"category"`
	Categories        []string  `json:"categories"`
	PublishedAt       time.Time `json:"published_at"`
	IsPremium         bool      `json:"is_premium"`
	ViewCount         int       `json:"view_count"`
	CreatedAt         time.Time `json:"created_at"`
}

type SubscriptionTier struct {
	ID                  int     `json:"id"`
	Name                string  `json:"name"`
	StripeProductID     *string `json:"-"`
	StripePriceID       *string `json:"-"`
	PriceMonthly        float64 `json:"price_monthly"`
	MaxArticlesPerDay   int     `json:"max_articles_per_day"`
	MaxArticlesPerMonth int     `json:"max_articles_per_month"`
	HasPremiumAccess    bool    `json:"has_premium_access"`
	UnlimitedReading    bool    `json:"unlimited_reading"`
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

type FetchArticleRequest struct {
	URL string `json:"url"`
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
	ID                uuid.UUID        `json:"id"`
	Title             string           `json:"title"`
	Summary           string           `json:"summary"`
	Content           *string          `json:"content"`
	OriginalContent   *string          `json:"original_content,omitempty"`
	RewriteStatus     string           `json:"rewrite_status"`
	LLMRewriteVersion int              `json:"llm_rewrite_version"`
	SourceName        string           `json:"source_name"`
	SourceURL         string           `json:"source_url"`
	ImageURL          *string          `json:"image_url"`
	Category          *string          `json:"category"`
	Categories        []string         `json:"categories"`
	PublishedAt       time.Time        `json:"published_at"`
	IsPremium         bool             `json:"is_premium"`
	ViewCount         int              `json:"view_count"`
	Rewrites          []RewriteVersion `json:"rewrites,omitempty"`
	AccessExpiresAt   *time.Time       `json:"access_expires_at,omitempty"`
}

type FeedResponse struct {
	Articles []ArticleResponse `json:"articles"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	HasMore  bool              `json:"has_more"`
}

type TierResponse struct {
	ID                  int     `json:"id"`
	Name                string  `json:"name"`
	PriceMonthly        float64 `json:"price_monthly"`
	MaxArticlesPerDay   int     `json:"max_articles_per_day"`
	MaxArticlesPerMonth int     `json:"max_articles_per_month"`
	HasPremiumAccess    bool    `json:"has_premium_access"`
	UnlimitedReading    bool    `json:"unlimited_reading"`
	IsCurrent           bool    `json:"is_current"`
}

type CheckoutRequest struct {
	TierID int `json:"tier_id"`
}

type CheckoutResponse struct {
	CheckoutURL string `json:"checkout_url"`
	SessionID   string `json:"session_id"`
}

type BillingPortalResponse struct {
	PortalURL string `json:"portal_url"`
}

// LLM comparison types

type RewriteVersion struct {
	ID        uuid.UUID `json:"id"`
	ModelName string    `json:"model_name"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	Content   *string   `json:"content"`
}

type ComparisonResponse struct {
	ArticleID     uuid.UUID      `json:"article_id"`
	OriginalTitle string         `json:"original_title"`
	SourceName    string         `json:"source_name"`
	SourceURL     string         `json:"source_url"`
	ImageURL      *string        `json:"image_url"`
	Category      *string        `json:"category"`
	PublishedAt   time.Time      `json:"published_at"`
	VersionA      RewriteVersion `json:"version_a"`
	VersionB      RewriteVersion `json:"version_b"`
	UserVote      *string        `json:"user_vote"`
}

type VoteRequest struct {
	ChosenRewriteID uuid.UUID `json:"chosen_rewrite_id"`
	OtherRewriteID  uuid.UUID `json:"other_rewrite_id"`
}

type VoteStatsResponse struct {
	ArticleID     uuid.UUID `json:"article_id"`
	VersionAID    uuid.UUID `json:"version_a_id"`
	VersionAName  string    `json:"version_a_name"`
	VersionAVotes int       `json:"version_a_votes"`
	VersionBID    uuid.UUID `json:"version_b_id"`
	VersionBName  string    `json:"version_b_name"`
	VersionBVotes int       `json:"version_b_votes"`
}
