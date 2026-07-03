package repository

import (
	"context"
	"fmt"

	"clientroll/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PlatformBillingConfig stores platform-wide billing settings and encrypted Stripe secrets.
// This table is platform-scoped (not tenant-scoped).
type PlatformBillingConfig struct {
	ID uuid.UUID `json:"id"`

	PerMemberPriceUSD float64 `json:"per_member_price_usd"`
	Currency          string  `json:"currency"`

	StripePublishableKeyEncrypted *string `json:"-"`
	StripeSecretKeyEncrypted      *string `json:"-"`
	StripeWebhookSecretEncrypted  *string `json:"-"`
	GooglePlacesAPIKeyEncrypted   *string `json:"-"`

	StripeProductID *string `json:"stripe_product_id,omitempty"`
	StripePriceID   *string `json:"stripe_price_id,omitempty"`

	IsActive             bool `json:"is_active"`
	GooglePlacesIsActive bool `json:"google_places_is_active"`
}

// PlatformBillingConfigPublic is safe to return via API.
type PlatformBillingConfigPublic struct {
	PerMemberPriceUSD float64 `json:"per_member_price_usd"`
	Currency          string  `json:"currency"`

	HasPublishableKey  bool `json:"has_publishable_key"`
	HasSecretKey       bool `json:"has_secret_key"`
	HasWebhookSecret   bool `json:"has_webhook_secret"`
	HasGooglePlacesKey bool `json:"has_google_places_key"`

	StripeProductID *string `json:"stripe_product_id,omitempty"`
	StripePriceID   *string `json:"stripe_price_id,omitempty"`

	IsActive             bool `json:"is_active"`
	GooglePlacesIsActive bool `json:"google_places_is_active"`
}

// PlatformBillingConfigUpdateInput is used to update platform billing config.
// Fields are pointers so callers can omit values.
type PlatformBillingConfigUpdateInput struct {
	PerMemberPriceUSD *float64 `json:"per_member_price_usd,omitempty"`
	Currency          *string  `json:"currency,omitempty"`

	StripeProductID *string `json:"stripe_product_id,omitempty"`
	StripePriceID   *string `json:"stripe_price_id,omitempty"`

	// Encrypted values (callers should encrypt using database.Encryptor).
	StripePublishableKeyEncrypted *string `json:"-"`
	StripeSecretKeyEncrypted      *string `json:"-"`
	StripeWebhookSecretEncrypted  *string `json:"-"`
	GooglePlacesAPIKeyEncrypted   *string `json:"-"`

	IsActive             *bool `json:"is_active,omitempty"`
	GooglePlacesIsActive *bool `json:"google_places_is_active,omitempty"`
}

type PlatformBillingConfigRepo struct {
	db *database.Postgres
}

func NewPlatformBillingConfigRepo(db *database.Postgres) *PlatformBillingConfigRepo {
	return &PlatformBillingConfigRepo{db: db}
}

// GetOrCreate returns the single platform billing config row, creating it if missing.
func (r *PlatformBillingConfigRepo) GetOrCreate(ctx context.Context) (*PlatformBillingConfig, error) {
	// There should be only one row; create one if none exists.
	query := `
		WITH existing AS (
			SELECT id, per_member_price_usd, currency,
			       stripe_publishable_key_encrypted, stripe_secret_key_encrypted, stripe_webhook_secret_encrypted, google_places_api_key_encrypted,
			       stripe_product_id, stripe_price_id,
			       is_active, google_places_is_active
			FROM platform_billing_config
			ORDER BY updated_at DESC
			LIMIT 1
		),
		ins AS (
			INSERT INTO platform_billing_config (per_member_price_usd, currency, is_active)
			SELECT 1.00, 'USD', false
			WHERE NOT EXISTS (SELECT 1 FROM existing)
			RETURNING id, per_member_price_usd, currency,
			          stripe_publishable_key_encrypted, stripe_secret_key_encrypted, stripe_webhook_secret_encrypted, google_places_api_key_encrypted,
			          stripe_product_id, stripe_price_id,
			          is_active, google_places_is_active
		)
		SELECT * FROM existing
		UNION ALL
		SELECT * FROM ins
		LIMIT 1`

	var c PlatformBillingConfig
	err := r.db.Pool.QueryRow(ctx, query).Scan(
		&c.ID,
		&c.PerMemberPriceUSD,
		&c.Currency,
		&c.StripePublishableKeyEncrypted,
		&c.StripeSecretKeyEncrypted,
		&c.StripeWebhookSecretEncrypted,
		&c.GooglePlacesAPIKeyEncrypted,
		&c.StripeProductID,
		&c.StripePriceID,
		&c.IsActive,
		&c.GooglePlacesIsActive,
	)
	if err != nil {
		return nil, fmt.Errorf("getting/creating platform billing config: %w", err)
	}
	return &c, nil
}

// Update updates the config row and returns the updated record.
func (r *PlatformBillingConfigRepo) Update(ctx context.Context, id uuid.UUID, in PlatformBillingConfigUpdateInput) (*PlatformBillingConfig, error) {
	query := `
		UPDATE platform_billing_config SET
			per_member_price_usd = COALESCE($2, per_member_price_usd),
			currency = COALESCE($3, currency),
			stripe_product_id = COALESCE($4, stripe_product_id),
			stripe_price_id = COALESCE($5, stripe_price_id),
			stripe_publishable_key_encrypted = COALESCE($6, stripe_publishable_key_encrypted),
			stripe_secret_key_encrypted = COALESCE($7, stripe_secret_key_encrypted),
			stripe_webhook_secret_encrypted = COALESCE($8, stripe_webhook_secret_encrypted),
			google_places_api_key_encrypted = COALESCE($9, google_places_api_key_encrypted),
			is_active = COALESCE($10, is_active),
			google_places_is_active = COALESCE($11, google_places_is_active),
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, per_member_price_usd, currency,
		          stripe_publishable_key_encrypted, stripe_secret_key_encrypted, stripe_webhook_secret_encrypted, google_places_api_key_encrypted,
		          stripe_product_id, stripe_price_id,
		          is_active, google_places_is_active`

	var c PlatformBillingConfig
	err := r.db.Pool.QueryRow(ctx, query,
		id,
		in.PerMemberPriceUSD,
		in.Currency,
		in.StripeProductID,
		in.StripePriceID,
		in.StripePublishableKeyEncrypted,
		in.StripeSecretKeyEncrypted,
		in.StripeWebhookSecretEncrypted,
		in.GooglePlacesAPIKeyEncrypted,
		in.IsActive,
		in.GooglePlacesIsActive,
	).Scan(
		&c.ID,
		&c.PerMemberPriceUSD,
		&c.Currency,
		&c.StripePublishableKeyEncrypted,
		&c.StripeSecretKeyEncrypted,
		&c.StripeWebhookSecretEncrypted,
		&c.GooglePlacesAPIKeyEncrypted,
		&c.StripeProductID,
		&c.StripePriceID,
		&c.IsActive,
		&c.GooglePlacesIsActive,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("updating platform billing config: %w", err)
	}
	return &c, nil
}

func (c *PlatformBillingConfig) ToPublic() PlatformBillingConfigPublic {
	return PlatformBillingConfigPublic{
		PerMemberPriceUSD:    c.PerMemberPriceUSD,
		Currency:             c.Currency,
		HasPublishableKey:    c.StripePublishableKeyEncrypted != nil && *c.StripePublishableKeyEncrypted != "",
		HasSecretKey:         c.StripeSecretKeyEncrypted != nil && *c.StripeSecretKeyEncrypted != "",
		HasWebhookSecret:     c.StripeWebhookSecretEncrypted != nil && *c.StripeWebhookSecretEncrypted != "",
		HasGooglePlacesKey:   c.GooglePlacesAPIKeyEncrypted != nil && *c.GooglePlacesAPIKeyEncrypted != "",
		StripeProductID:      c.StripeProductID,
		StripePriceID:        c.StripePriceID,
		IsActive:             c.IsActive,
		GooglePlacesIsActive: c.GooglePlacesIsActive,
	}
}
