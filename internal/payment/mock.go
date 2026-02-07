package payment

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// MockProvider simulates a payment provider for development and testing.
type MockProvider struct{}

// NewMockProvider creates a new mock payment provider.
func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (m *MockProvider) CreateCustomer(_ context.Context, input CreateCustomerInput) (string, error) {
	return fmt.Sprintf("mock_cust_%s", uuid.New().String()[:8]), nil
}

func (m *MockProvider) SetupDirectDebit(_ context.Context, input SetupDirectDebitInput) (string, error) {
	return fmt.Sprintf("mock_mandate_%s", uuid.New().String()[:8]), nil
}

func (m *MockProvider) CreatePayment(_ context.Context, input CreatePaymentInput) (string, error) {
	return fmt.Sprintf("mock_pay_%s", uuid.New().String()[:8]), nil
}

func (m *MockProvider) GetPaymentStatus(_ context.Context, paymentID string) (PaymentStatus, error) {
	return PaymentStatus{
		ID:     paymentID,
		Status: "confirmed",
		Amount: 10000,
	}, nil
}

func (m *MockProvider) CancelMandate(_ context.Context, _ string) error {
	return nil
}

func (m *MockProvider) HandleWebhook(_ context.Context, _ []byte, _ string) (*WebhookEvent, error) {
	return &WebhookEvent{Type: "mock.event", Action: "mock"}, nil
}
