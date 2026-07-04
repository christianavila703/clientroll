package repository

import (
	"context"
	"fmt"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// TenantSubscription represents the platform billing subscription for a tenant.
type TenantSubscription struct {
	ID       uuid.UUID `json:"id"`
	TenantID uuid.UUID `json:"tenant_id"`

	StripeCustomerID        *string `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID    *string `json:"stripe_subscription_id,omitempty"`
	StripeSubscriptionItemID *string `json:"stripe_subscription_item_id,omitempty"`

	Status string `json:"status"`

	ActiveMemberCount      int     `json:"active_member_count"`
	CurrentMonthlyAmountUSD float64 `json:"current_monthly_amount_usd"`
	CurrentPeriodStart     *time.Time `json:"current_period_start,omitempty"`
	CurrentPeriodEnd       *time.Time `json:"current_period_end,omitempty"`

	PastDueSince     *time.Time `json:"past_due_since,omitempty"`
	BillingGraceUntil *time.Time `json:"billing_grace_until,omitempty"`

	HasValidPaymentMethod bool    `json:"has_valid_payment_method"`
	CardBrand             *string `json:"card_brand,omitempty"`
	CardLast4             *string `json:"card_last4,omitempty"`
	CardExpMonth          *int    `json:"card_exp_month,omitempty"`
	CardExpYear           *int    `json:"card_exp_year,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TenantSubscriptionRepo struct {
	db *database.Postgres
}

func NewTenantSubscriptionRepo(db *database.Postgres) *TenantSubscriptionRepo {
	return &TenantSubscriptionRepo{db: db}
}

// GetTenantIDByStripeCustomerID resolves tenant_id from a Stripe customer ID.
// Uses a SECURITY DEFINER function so it works without tenant context (e.g. webhooks).
func (r *TenantSubscriptionRepo) GetTenantIDByStripeCustomerID(ctx context.Context, stripeCustomerID string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.Pool.QueryRow(ctx, `SELECT get_tenant_id_by_stripe_customer_id($1)`, stripeCustomerID).Scan(&id)
	return id, err
}

// GetTenantIDByStripeSubscriptionID resolves tenant_id from a Stripe subscription ID.
// Uses a SECURITY DEFINER function so it works without tenant context (e.g. webhooks).
func (r *TenantSubscriptionRepo) GetTenantIDByStripeSubscriptionID(ctx context.Context, stripeSubscriptionID string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.Pool.QueryRow(ctx, `SELECT get_tenant_id_by_stripe_subscription_id($1)`, stripeSubscriptionID).Scan(&id)
	return id, err
}

// GetByTenant returns the subscription record for a tenant.
func (r *TenantSubscriptionRepo) GetByTenant(ctx context.Context, tenantID uuid.UUID) (*TenantSubscription, error) {
	rlsID, tenantUUID := database.TenantRLSScope(tenantID)
	query := `
		WITH _ AS (
			SELECT set_config('app.current_tenant_id', $1, true)
		)
		SELECT id, tenant_id,
		       stripe_customer_id, stripe_subscription_id, stripe_subscription_item_id,
		       status, active_member_count, current_monthly_amount_usd,
		       current_period_start, current_period_end,
		       past_due_since, billing_grace_until,
		       has_valid_payment_method, card_brand, card_last4, card_exp_month, card_exp_year,
		       created_at, updated_at
		FROM tenant_subscriptions
		WHERE tenant_id = $2`

	var s TenantSubscription
	err := r.db.Pool.QueryRow(ctx, query, rlsID, tenantUUID).Scan(
		&s.ID, &s.TenantID,
		&s.StripeCustomerID, &s.StripeSubscriptionID, &s.StripeSubscriptionItemID,
		&s.Status, &s.ActiveMemberCount, &s.CurrentMonthlyAmountUSD,
		&s.CurrentPeriodStart, &s.CurrentPeriodEnd,
		&s.PastDueSince, &s.BillingGraceUntil,
		&s.HasValidPaymentMethod, &s.CardBrand, &s.CardLast4, &s.CardExpMonth, &s.CardExpYear,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting tenant subscription: %w", err)
	}
	return &s, nil
}

// UpsertPending ensures a subscription row exists for the tenant.
func (r *TenantSubscriptionRepo) UpsertPending(ctx context.Context, tenantID uuid.UUID) (*TenantSubscription, error) {
	rlsID, tenantUUID := database.TenantRLSScope(tenantID)
	query := `
		WITH _ AS (
			SELECT set_config('app.current_tenant_id', $1, true)
		)
		INSERT INTO tenant_subscriptions (tenant_id, status)
		VALUES ($2, 'pending')
		ON CONFLICT (tenant_id) DO UPDATE SET
			updated_at = NOW()
		RETURNING id, tenant_id,
		          stripe_customer_id, stripe_subscription_id, stripe_subscription_item_id,
		          status, active_member_count, current_monthly_amount_usd,
		          current_period_start, current_period_end,
		          past_due_since, billing_grace_until,
		          has_valid_payment_method, card_brand, card_last4, card_exp_month, card_exp_year,
		          created_at, updated_at`

	var s TenantSubscription
	err := r.db.Pool.QueryRow(ctx, query, rlsID, tenantUUID).Scan(
		&s.ID, &s.TenantID,
		&s.StripeCustomerID, &s.StripeSubscriptionID, &s.StripeSubscriptionItemID,
		&s.Status, &s.ActiveMemberCount, &s.CurrentMonthlyAmountUSD,
		&s.CurrentPeriodStart, &s.CurrentPeriodEnd,
		&s.PastDueSince, &s.BillingGraceUntil,
		&s.HasValidPaymentMethod, &s.CardBrand, &s.CardLast4, &s.CardExpMonth, &s.CardExpYear,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upserting pending subscription: %w", err)
	}
	return &s, nil
}

type TenantSubscriptionUpdate struct {
	StripeCustomerID         *string
	StripeSubscriptionID     *string
	StripeSubscriptionItemID *string

	Status *string

	ActiveMemberCount       *int
	CurrentMonthlyAmountUSD *float64
	CurrentPeriodStart      *time.Time
	CurrentPeriodEnd        *time.Time

	PastDueSince      *time.Time
	BillingGraceUntil *time.Time

	HasValidPaymentMethod *bool
	CardBrand             *string
	CardLast4             *string
	CardExpMonth          *int
	CardExpYear           *int
}

// UpdateByTenant updates the subscription row for a tenant.
func (r *TenantSubscriptionRepo) UpdateByTenant(ctx context.Context, tenantID uuid.UUID, u TenantSubscriptionUpdate) (*TenantSubscription, error) {
	rlsID, tenantUUID := database.TenantRLSScope(tenantID)
	query := `
		WITH _ AS (
			SELECT set_config('app.current_tenant_id', $1, true)
		)
		UPDATE tenant_subscriptions SET
			stripe_customer_id = COALESCE($3, stripe_customer_id),
			stripe_subscription_id = COALESCE($4, stripe_subscription_id),
			stripe_subscription_item_id = COALESCE($5, stripe_subscription_item_id),
			status = COALESCE($6, status),
			active_member_count = COALESCE($7, active_member_count),
			current_monthly_amount_usd = COALESCE($8, current_monthly_amount_usd),
			current_period_start = COALESCE($9, current_period_start),
			current_period_end = COALESCE($10, current_period_end),
			past_due_since = COALESCE($11, past_due_since),
			billing_grace_until = COALESCE($12, billing_grace_until),
			has_valid_payment_method = COALESCE($13, has_valid_payment_method),
			card_brand = COALESCE($14, card_brand),
			card_last4 = COALESCE($15, card_last4),
			card_exp_month = COALESCE($16, card_exp_month),
			card_exp_year = COALESCE($17, card_exp_year),
			updated_at = NOW()
		WHERE tenant_id = $2
		RETURNING id, tenant_id,
		          stripe_customer_id, stripe_subscription_id, stripe_subscription_item_id,
		          status, active_member_count, current_monthly_amount_usd,
		          current_period_start, current_period_end,
		          past_due_since, billing_grace_until,
		          has_valid_payment_method, card_brand, card_last4, card_exp_month, card_exp_year,
		          created_at, updated_at`

	var s TenantSubscription
	err := r.db.Pool.QueryRow(ctx, query,
		rlsID,
		tenantUUID,
		u.StripeCustomerID,
		u.StripeSubscriptionID,
		u.StripeSubscriptionItemID,
		u.Status,
		u.ActiveMemberCount,
		u.CurrentMonthlyAmountUSD,
		u.CurrentPeriodStart,
		u.CurrentPeriodEnd,
		u.PastDueSince,
		u.BillingGraceUntil,
		u.HasValidPaymentMethod,
		u.CardBrand,
		u.CardLast4,
		u.CardExpMonth,
		u.CardExpYear,
	).Scan(
		&s.ID, &s.TenantID,
		&s.StripeCustomerID, &s.StripeSubscriptionID, &s.StripeSubscriptionItemID,
		&s.Status, &s.ActiveMemberCount, &s.CurrentMonthlyAmountUSD,
		&s.CurrentPeriodStart, &s.CurrentPeriodEnd,
		&s.PastDueSince, &s.BillingGraceUntil,
		&s.HasValidPaymentMethod, &s.CardBrand, &s.CardLast4, &s.CardExpMonth, &s.CardExpYear,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("updating tenant subscription: %w", err)
	}
	return &s, nil
}

