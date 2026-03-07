package billing

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// MockBillingProvider simulates Stripe billing for development.
type MockBillingProvider struct{}

// NewMockBillingProvider creates a new mock billing provider.
func NewMockBillingProvider() *MockBillingProvider {
	return &MockBillingProvider{}
}

func (m *MockBillingProvider) CreateSubscription(_ context.Context, input SubscriptionInput) (string, error) {
	_ = input // included for parity with real provider
	return fmt.Sprintf("mock_sub_%s", uuid.New().String()[:8]), nil
}

func (m *MockBillingProvider) ReportUsage(_ context.Context, _ string, _ int) error {
	return nil
}

func (m *MockBillingProvider) GetSubscriptionStatus(_ context.Context, subID string) (SubscriptionStatus, error) {
	return SubscriptionStatus{
		ID: subID, Status: "active", CurrentPeriodEnd: "2026-03-07",
		MemberCount: 10, MonthlyAmount: 20.00,
	}, nil
}

func (m *MockBillingProvider) CancelSubscription(_ context.Context, _ string) error {
	return nil
}

func (m *MockBillingProvider) CreatePortalSession(_ context.Context, _ string) (string, error) {
	return "https://billing.stripe.com/mock-portal", nil
}
