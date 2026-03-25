package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	subscriptionAPI "github.com/stripe/stripe-go/v82/subscription"
	"github.com/stripe/stripe-go/v82/webhook"

	"github.com/PortNumber53/no-click-bait-news/backend/middleware"
	"github.com/PortNumber53/no-click-bait-news/backend/models"
)

func (h *Handler) GetTiers(w http.ResponseWriter, r *http.Request) {
	// Get user's current tier if authenticated
	var currentTierID int
	user := middleware.GetUser(r.Context())
	if user != nil {
		h.pool.QueryRow(r.Context(),
			"SELECT tier_id FROM user_subscriptions WHERE user_id = $1 AND status = 'active'",
			user.ID,
		).Scan(&currentTierID)
	}

	rows, err := h.pool.Query(r.Context(),
		"SELECT id, name, price_monthly, max_articles_per_day, has_premium_access FROM subscription_tiers")
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to fetch tiers")
		return
	}
	defer rows.Close()

	tiers := make([]models.TierResponse, 0)
	for rows.Next() {
		var t models.TierResponse
		if err := rows.Scan(&t.ID, &t.Name, &t.PriceMonthly, &t.MaxArticlesPerDay, &t.HasPremiumAccess); err != nil {
			continue
		}
		t.IsCurrent = t.ID == currentTierID
		tiers = append(tiers, t)
	}

	JSON(w, http.StatusOK, tiers)
}

func (h *Handler) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		Error(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req models.CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var tier struct {
		ID            int
		StripePriceID *string
	}
	err := h.pool.QueryRow(r.Context(),
		"SELECT id, stripe_price_id FROM subscription_tiers WHERE id = $1", req.TierID,
	).Scan(&tier.ID, &tier.StripePriceID)
	if err != nil || tier.StripePriceID == nil {
		Error(w, http.StatusBadRequest, "Invalid tier or tier not available for purchase")
		return
	}

	stripe.Key = h.stripeKey

	// Build redirect URLs from the request origin
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = r.Header.Get("Referer")
		if origin != "" {
			// Strip path from referer to get origin
			if u, err := url.Parse(origin); err == nil {
				origin = u.Scheme + "://" + u.Host
			}
		}
	}
	if origin == "" {
		origin = "https://ncbnews.truvis.co"
	}
	successURL := origin + "/subscriptions?success=true"
	cancelURL := origin + "/subscriptions?canceled=true"

	params := &stripe.CheckoutSessionParams{
		CustomerEmail: &user.Email,
		Mode:          stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: tier.StripePriceID, Quantity: stripe.Int64(1)},
		},
		SuccessURL: &successURL,
		CancelURL:  &cancelURL,
		Params: stripe.Params{
			Metadata: map[string]string{
				"user_id": user.ID.String(),
				"tier_id": strconv.Itoa(tier.ID),
			},
		},
	}

	sess, err := session.New(params)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to create checkout session")
		return
	}

	JSON(w, http.StatusOK, models.CheckoutResponse{
		CheckoutURL: sess.URL,
		SessionID:   sess.ID,
	})
}

// StripeWebhookThin handles thin (event-only) webhook payloads.
// Thin events contain the event type and object ID but not the full object data.
// The handler fetches the full object from Stripe's API when needed.
func (h *Handler) StripeWebhookThin(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		Error(w, http.StatusBadRequest, "Failed to read body")
		return
	}

	event, err := webhook.ConstructEventWithOptions(body, r.Header.Get("Stripe-Signature"), h.webhookSecretThin, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid webhook signature")
		return
	}

	stripe.Key = h.stripeKey

	switch event.Type {
	case "checkout.session.completed":
		// Thin payload only has the object ID — fetch the full session
		var thinObj struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(event.Data.Raw, &thinObj); err == nil && thinObj.ID != "" {
			sess, err := session.Get(thinObj.ID, &stripe.CheckoutSessionParams{})
			if err == nil {
				h.handleCheckoutCompleted(r, sess)
			}
		}
	case "customer.subscription.updated", "customer.subscription.deleted":
		var thinObj struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(event.Data.Raw, &thinObj); err == nil && thinObj.ID != "" {
			// Fetch the full subscription from Stripe
			params := &stripe.SubscriptionParams{}
			sub, err := subscriptionAPI.Get(thinObj.ID, params)
			if err == nil {
				h.handleSubscriptionUpdated(r, sub)
			}
		}
	}

	JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// StripeWebhookSnapshot handles snapshot (full-object) webhook payloads.
// Snapshot events contain the complete object data inline, same as the legacy default.
func (h *Handler) StripeWebhookSnapshot(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		Error(w, http.StatusBadRequest, "Failed to read body")
		return
	}

	sig := r.Header.Get("Stripe-Signature")
	if sig == "" {
		log.Printf("[webhook/snapshot] Missing Stripe-Signature header")
		Error(w, http.StatusBadRequest, "Missing Stripe-Signature header")
		return
	}

	event, err := webhook.ConstructEventWithOptions(body, sig, h.webhookSecretSnapshot, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		log.Printf("[webhook/snapshot] Signature verification failed: %v", err)
		Error(w, http.StatusBadRequest, "Invalid webhook signature")
		return
	}

	log.Printf("[webhook/snapshot] Received event: %s", event.Type)

	switch event.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err == nil {
			h.handleCheckoutCompleted(r, &sess)
		}
	case "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err == nil {
			h.handleSubscriptionUpdated(r, &sub)
		}
	}

	JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleCheckoutCompleted(r *http.Request, sess *stripe.CheckoutSession) {
	userID := sess.Metadata["user_id"]
	tierID := sess.Metadata["tier_id"]
	if userID == "" || tierID == "" {
		return
	}

	tid, _ := strconv.Atoi(tierID)
	subID := ""
	if sess.Subscription != nil {
		subID = sess.Subscription.ID
	}

	// Upsert subscription
	h.pool.Exec(r.Context(),
		`INSERT INTO user_subscriptions (id, user_id, tier_id, stripe_subscription_id, status, created_at, updated_at)
		 VALUES (gen_random_uuid(), $1, $2, $3, 'active', NOW(), NOW())
		 ON CONFLICT (user_id) DO UPDATE SET tier_id = $2, stripe_subscription_id = $3, status = 'active', updated_at = NOW()`,
		userID, tid, subID,
	)
}

func (h *Handler) handleSubscriptionUpdated(r *http.Request, sub *stripe.Subscription) {
	if sub.ID == "" {
		return
	}
	status := string(sub.Status)
	h.pool.Exec(r.Context(),
		"UPDATE user_subscriptions SET status = $1, updated_at = NOW() WHERE stripe_subscription_id = $2",
		status, sub.ID,
	)
}
