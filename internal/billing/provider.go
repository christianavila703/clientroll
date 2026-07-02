package billing

import "context"

// PlatformBillingProvider defines the interface for platform billing (Stripe).
// Charges tenants based on active member count.
type PlatformBillingProvider interface {
	// CreateSubscription creates a new subscription for a tenant.
	CreateSubscription(ctx context.Context, input SubscriptionInput) (string, error)

	// ReportUsage reports metered usage (active member count) for billing.
	ReportUsage(ctx context.Context, subscriptionID string, memberCount int) error

	// GetSubscriptionStatus returns the current subscription status.
	GetSubscriptionStatus(ctx context.Context, subscriptionID string) (SubscriptionStatus, error)

	// CancelSubscription cancels a tenant's subscription.
	CancelSubscription(ctx context.Context, subscriptionID string) error

	// CreatePortalSession creates a Stripe billing portal session URL.
	CreatePortalSession(ctx context.Context, customerID string) (string, error)
}

// SubscriptionInput holds data for creating a subscription.
type SubscriptionInput struct {
	TenantID   string
	TenantName string
	AdminEmail string

	// StripePriceID is the recurring price ID (per active member per month).
	// This is platform-configured and stored in platform_billing_config.
	StripePriceID string

	// PricePerMember is retained for reporting/validation, but Stripe billing
	// should rely on StripePriceID as the source of truth.
	PricePerMember float64
}

// SubscriptionStatus represents the current state of a subscription.
type SubscriptionStatus struct {
	ID                string
	Status            string // active, past_due, cancelled, trialing
	CurrentPeriodEnd  string
	MemberCount       int
	MonthlyAmount     float64
}
