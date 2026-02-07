package billing

import (
	"context"
	"fmt"
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

func (s *StripeBillingProvider) CreateSubscription(_ context.Context, _ SubscriptionInput) (string, error) {
	return "", fmt.Errorf("Stripe integration not yet implemented")
}

func (s *StripeBillingProvider) ReportUsage(_ context.Context, _ string, _ int) error {
	return fmt.Errorf("Stripe integration not yet implemented")
}

func (s *StripeBillingProvider) GetSubscriptionStatus(_ context.Context, _ string) (SubscriptionStatus, error) {
	return SubscriptionStatus{}, fmt.Errorf("Stripe integration not yet implemented")
}

func (s *StripeBillingProvider) CancelSubscription(_ context.Context, _ string) error {
	return fmt.Errorf("Stripe integration not yet implemented")
}

func (s *StripeBillingProvider) CreatePortalSession(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("Stripe integration not yet implemented")
}
