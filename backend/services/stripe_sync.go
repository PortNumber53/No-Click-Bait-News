package services

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/price"
	"github.com/stripe/stripe-go/v82/product"
)

// SyncSubscriptionTiers ensures every local subscription tier has a corresponding
// Stripe product and price. Stripe is the source of truth for conflicts.
//
// Flow per tier:
//  1. If tier has a stripe_product_id, fetch the product from Stripe.
//     - If it exists, reconcile the product name.
//     - If not (deleted on Stripe side), clear the local reference and create a new one.
//  2. If tier has no stripe_product_id, search Stripe for a product with matching
//     metadata (tier_id), or create one.
//  3. Same logic for price: look up existing, reconcile amount, or create new.
//  4. Write stripe_product_id and stripe_price_id back to the local DB.
func SyncSubscriptionTiers(ctx context.Context, pool *pgxpool.Pool, stripeKey string) error {
	stripe.Key = stripeKey

	rows, err := pool.Query(ctx,
		`SELECT id, name, stripe_product_id, stripe_price_id, price_monthly, max_articles_per_day, has_premium_access
		 FROM subscription_tiers ORDER BY id`)
	if err != nil {
		return fmt.Errorf("query tiers: %w", err)
	}
	defer rows.Close()

	type localTier struct {
		ID                int
		Name              string
		StripeProductID   *string
		StripePriceID     *string
		PriceMonthly      float64
		MaxArticlesPerDay int
		HasPremiumAccess  bool
	}

	var tiers []localTier
	for rows.Next() {
		var t localTier
		if err := rows.Scan(&t.ID, &t.Name, &t.StripeProductID, &t.StripePriceID,
			&t.PriceMonthly, &t.MaxArticlesPerDay, &t.HasPremiumAccess); err != nil {
			return fmt.Errorf("scan tier: %w", err)
		}
		tiers = append(tiers, t)
	}

	for _, t := range tiers {
		if t.PriceMonthly == 0 {
			// Free tier — no Stripe product/price needed
			log.Printf("[stripe-sync] Skipping free tier %q", t.Name)
			continue
		}

		productID, err := ensureProduct(t.ID, t.Name, t.StripeProductID)
		if err != nil {
			return fmt.Errorf("ensure product for tier %q: %w", t.Name, err)
		}

		priceID, err := ensurePrice(productID, t.PriceMonthly, t.StripePriceID)
		if err != nil {
			return fmt.Errorf("ensure price for tier %q: %w", t.Name, err)
		}

		_, err = pool.Exec(ctx,
			"UPDATE subscription_tiers SET stripe_product_id = $1, stripe_price_id = $2 WHERE id = $3",
			productID, priceID, t.ID)
		if err != nil {
			return fmt.Errorf("update tier %q: %w", t.Name, err)
		}

		log.Printf("[stripe-sync] Tier %q synced: product=%s price=%s", t.Name, productID, priceID)
	}

	return nil
}

// ensureProduct finds or creates a Stripe product for the given tier.
func ensureProduct(tierID int, tierName string, existingProductID *string) (string, error) {
	displayName := "No-Click Bait News — " + capitalize(tierName)

	// If we have an existing product ID, verify it still exists on Stripe
	if existingProductID != nil && *existingProductID != "" {
		p, err := product.Get(*existingProductID, nil)
		if err == nil {
			// Product exists — update name if it drifted (Stripe is source of truth,
			// but we push local name changes to Stripe since these are our products)
			if p.Name != displayName {
				product.Update(p.ID, &stripe.ProductParams{
					Name: &displayName,
				})
			}
			return p.ID, nil
		}
		// Product was deleted on Stripe — fall through to create
		log.Printf("[stripe-sync] Product %s no longer exists on Stripe, creating new one", *existingProductID)
	}

	// Search for existing product by metadata
	params := &stripe.ProductSearchParams{}
	params.Query = fmt.Sprintf("metadata['tier_id']:'%d'", tierID)
	iter := product.Search(params)
	for iter.Next() {
		return iter.Product().ID, nil
	}

	// Create new product
	p, err := product.New(&stripe.ProductParams{
		Name: &displayName,
		Params: stripe.Params{
			Metadata: map[string]string{
				"tier_id":   strconv.Itoa(tierID),
				"tier_name": tierName,
			},
		},
	})
	if err != nil {
		return "", err
	}
	return p.ID, nil
}

// ensurePrice finds or creates a recurring monthly price for the product.
func ensurePrice(productID string, monthlyAmount float64, existingPriceID *string) (string, error) {
	unitAmount := int64(math.Round(monthlyAmount * 100))

	// If we have an existing price, check it on Stripe
	if existingPriceID != nil && *existingPriceID != "" {
		p, err := price.Get(*existingPriceID, nil)
		if err == nil {
			if p.UnitAmount == unitAmount && p.Currency == stripe.CurrencyUSD && !p.Deleted {
				// Price matches — use it
				return p.ID, nil
			}
			// Price exists but amount changed — archive old, create new
			// (Stripe doesn't allow updating the amount on an existing price)
			if p.Active {
				active := false
				price.Update(p.ID, &stripe.PriceParams{Active: &active})
			}
			log.Printf("[stripe-sync] Price %s amount mismatch (stripe=%d, local=%d), creating new price",
				*existingPriceID, p.UnitAmount, unitAmount)
		}
		// Price deleted or not found — fall through to search/create
	}

	// Search for an active price on this product with the right amount
	listParams := &stripe.PriceListParams{
		Product: &productID,
		Active:  stripe.Bool(true),
	}
	listParams.Filters.AddFilter("type", "", "recurring")
	iter := price.List(listParams)
	for iter.Next() {
		p := iter.Price()
		if p.UnitAmount == unitAmount && p.Currency == stripe.CurrencyUSD &&
			p.Recurring != nil && p.Recurring.Interval == stripe.PriceRecurringIntervalMonth {
			return p.ID, nil
		}
	}

	// Create new price
	interval := string(stripe.PriceRecurringIntervalMonth)
	p, err := price.New(&stripe.PriceParams{
		Product:    &productID,
		UnitAmount: &unitAmount,
		Currency:   stripe.String(string(stripe.CurrencyUSD)),
		Recurring: &stripe.PriceRecurringParams{
			Interval: &interval,
		},
	})
	if err != nil {
		return "", err
	}
	return p.ID, nil
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}
