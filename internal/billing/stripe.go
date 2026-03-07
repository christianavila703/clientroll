package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/billingportal/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/subscription"
)

// StripeBillingProvider implements PlatformBillingProvider for Stripe.
type StripeBillingProvider struct {
	publishableKey string
	secretKey      string
	webhookSecret  string
}

// NewStripeBillingProvider creates a new Stripe billing provider.
func NewStripeBillingProvider(publishableKey, secretKey, webhookSecret string) *StripeBillingProvider {
	return &StripeBillingProvider{
		publishableKey: publishableKey,
		secretKey:      secretKey,
		webhookSecret:  webhookSecret,
	}
}

func (s *StripeBillingProvider) CreateSubscription(ctx context.Context, input SubscriptionInput) (string, error) {
	if input.StripePriceID == "" {
		return "", fmt.Errorf("StripePriceID is required")
	}
	if input.AdminEmail == "" {
		return "", fmt.Errorf("admin email is required")
	}

	stripe.Key = s.secretKey

	// Create customer (store tenant_id in metadata for webhook routing).
	cParams := &stripe.CustomerParams{
		Email: stripe.String(input.AdminEmail),
		Name:  stripe.String(input.TenantName),
		Metadata: map[string]string{
			"tenant_id": input.TenantID,
		},
	}
	cParams.Context = ctx

	c, err := customer.New(cParams)
	if err != nil {
		return "", fmt.Errorf("creating Stripe customer: %w", err)
	}

	// Create subscription. This does NOT collect a payment method by itself.
	// The application should prefer Checkout for initial card collection;
	// this exists for administrative/backfill use.
	sParams := &stripe.SubscriptionParams{
		Customer: stripe.String(c.ID),
		Items: []*stripe.SubscriptionItemsParams{
			{Price: stripe.String(input.StripePriceID), Quantity: stripe.Int64(0)},
		},
		Metadata: map[string]string{
			"tenant_id": input.TenantID,
		},
	}
	sParams.Context = ctx
	sParams.AddExpand("items.data.price")
	sParams.AddExpand("latest_invoice.payment_intent")

	sub, err := subscription.New(sParams)
	if err != nil {
		return "", fmt.Errorf("creating Stripe subscription: %w", err)
	}
	return sub.ID, nil
}

func (s *StripeBillingProvider) ReportUsage(ctx context.Context, subscriptionID string, memberCount int) error {
	if subscriptionID == "" {
		return fmt.Errorf("subscriptionID is required")
	}
	if memberCount < 0 {
		return fmt.Errorf("memberCount cannot be negative")
	}

	stripe.Key = s.secretKey

	// Update subscription quantity for the first item (single-item subscription model).
	// This supports “$X per active member per month” licensed billing.
	getParams := &stripe.SubscriptionParams{}
	getParams.Context = ctx
	getParams.AddExpand("items.data.price")
	sub, err := subscription.Get(subscriptionID, getParams)
	if err != nil {
		return fmt.Errorf("fetching Stripe subscription: %w", err)
	}
	if sub.Items == nil || len(sub.Items.Data) == 0 {
		return fmt.Errorf("subscription has no items")
	}
	itemID := sub.Items.Data[0].ID

	up := &stripe.SubscriptionParams{
		Items: []*stripe.SubscriptionItemsParams{
			{ID: stripe.String(itemID), Quantity: stripe.Int64(int64(memberCount))},
		},
	}
	up.Context = ctx
	_, err = subscription.Update(subscriptionID, up)
	if err != nil {
		return fmt.Errorf("updating subscription quantity: %w", err)
	}
	return nil
}

func (s *StripeBillingProvider) GetSubscriptionStatus(ctx context.Context, subscriptionID string) (SubscriptionStatus, error) {
	if subscriptionID == "" {
		return SubscriptionStatus{}, fmt.Errorf("subscriptionID is required")
	}

	stripe.Key = s.secretKey

	params := &stripe.SubscriptionParams{}
	params.Context = ctx
	params.AddExpand("items.data.price")
	sub, err := subscription.Get(subscriptionID, params)
	if err != nil {
		return SubscriptionStatus{}, fmt.Errorf("fetching Stripe subscription: %w", err)
	}

	st := SubscriptionStatus{
		ID:     sub.ID,
		Status: string(sub.Status),
	}

	if sub.CurrentPeriodEnd > 0 {
		st.CurrentPeriodEnd = time.Unix(sub.CurrentPeriodEnd, 0).UTC().Format("2006-01-02")
	}

	if sub.Items != nil && len(sub.Items.Data) > 0 {
		item := sub.Items.Data[0]
		if item.Quantity != nil {
			st.MemberCount = int(*item.Quantity)
		}
		// Calculate monthly amount from price unit amount * quantity.
		if item.Price != nil && item.Price.UnitAmount != 0 {
			st.MonthlyAmount = (float64(item.Price.UnitAmount) / 100.0) * float64(st.MemberCount)
		}
	}

	return st, nil
}

func (s *StripeBillingProvider) CancelSubscription(ctx context.Context, subscriptionID string) error {
	if subscriptionID == "" {
		return fmt.Errorf("subscriptionID is required")
	}
	stripe.Key = s.secretKey

	params := &stripe.SubscriptionCancelParams{}
	params.Context = ctx
	_, err := subscription.Cancel(subscriptionID, params)
	if err != nil {
		return fmt.Errorf("canceling Stripe subscription: %w", err)
	}
	return nil
}

func (s *StripeBillingProvider) CreatePortalSession(ctx context.Context, customerID string) (string, error) {
	if customerID == "" {
		return "", fmt.Errorf("customerID is required")
	}
	stripe.Key = s.secretKey

	params := &stripe.BillingPortalSessionParams{
		Customer: stripe.String(customerID),
	}
	params.Context = ctx
	// Return URL is set by caller (preferred). If left empty, Stripe uses default portal settings.

	sess, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("creating Stripe billing portal session: %w", err)
	}
	return sess.URL, nil
}
