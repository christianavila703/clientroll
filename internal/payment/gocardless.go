package payment

import (
	"context"
	"fmt"
)

// GoCardlessProvider implements the payment Provider interface for GoCardless.
// Per-tenant credentials are injected at construction time.
type GoCardlessProvider struct {
	apiKey      string
	apiSecret   string
	environment string // "sandbox" or "live"
}

// NewGoCardlessProvider creates a new GoCardless provider with tenant credentials.
func NewGoCardlessProvider(apiKey, apiSecret, environment string) *GoCardlessProvider {
	return &GoCardlessProvider{
		apiKey:      apiKey,
		apiSecret:   apiSecret,
		environment: environment,
	}
}

func (g *GoCardlessProvider) CreateCustomer(_ context.Context, _ CreateCustomerInput) (string, error) {
	// TODO: Implement with GoCardless API when credentials are available
	return "", fmt.Errorf("GoCardless integration not yet implemented")
}

func (g *GoCardlessProvider) SetupDirectDebit(_ context.Context, _ SetupDirectDebitInput) (string, error) {
	return "", fmt.Errorf("GoCardless integration not yet implemented")
}

func (g *GoCardlessProvider) CreatePayment(_ context.Context, _ CreatePaymentInput) (string, error) {
	return "", fmt.Errorf("GoCardless integration not yet implemented")
}

func (g *GoCardlessProvider) GetPaymentStatus(_ context.Context, _ string) (PaymentStatus, error) {
	return PaymentStatus{}, fmt.Errorf("GoCardless integration not yet implemented")
}

func (g *GoCardlessProvider) CancelMandate(_ context.Context, _ string) error {
	return fmt.Errorf("GoCardless integration not yet implemented")
}

func (g *GoCardlessProvider) HandleWebhook(_ context.Context, _ []byte, _ string) (*WebhookEvent, error) {
	return nil, fmt.Errorf("GoCardless integration not yet implemented")
}
