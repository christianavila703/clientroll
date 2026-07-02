package payment

import (
	"context"
	"time"
)

// Provider defines the interface for payment providers (GoCardless, etc.).
// Each tenant has their own credentials. The provider is instantiated per-request.
type Provider interface {
	// CreateCustomer registers a client with the payment provider.
	CreateCustomer(ctx context.Context, input CreateCustomerInput) (string, error)

	// SetupDirectDebit creates a direct debit mandate.
	SetupDirectDebit(ctx context.Context, input SetupDirectDebitInput) (string, error)

	// CreatePayment schedules a payment against a mandate.
	CreatePayment(ctx context.Context, input CreatePaymentInput) (string, error)

	// GetPaymentStatus returns the current status of a payment.
	GetPaymentStatus(ctx context.Context, paymentID string) (PaymentStatus, error)

	// CancelMandate cancels a direct debit mandate.
	CancelMandate(ctx context.Context, mandateID string) error

	// HandleWebhook processes an incoming webhook from the provider.
	HandleWebhook(ctx context.Context, payload []byte, signature string) (*WebhookEvent, error)
}

// CreateCustomerInput holds data for creating a customer.
type CreateCustomerInput struct {
	Email     string
	Name      string
	Phone     string
	Reference string // Our client ID
}

// SetupDirectDebitInput holds data for setting up a direct debit.
type SetupDirectDebitInput struct {
	CustomerID        string
	AccountHolderName string
	AccountNumber     string
	BSB               string
	Reference         string // Our membership ID
}

// CreatePaymentInput holds data for creating a payment.
type CreatePaymentInput struct {
	MandateID   string
	Amount      int64 // in cents
	Currency    string
	Description string
	ChargeDate  time.Time
	Reference   string // Our payment ID
}

// PaymentStatus represents the state of a payment.
type PaymentStatus struct {
	ID     string
	Status string // pending, confirmed, paid_out, failed, cancelled
	Amount int64
	Error  string
}

// WebhookEvent represents a parsed webhook event.
type WebhookEvent struct {
	Type       string // payment.confirmed, payment.failed, mandate.cancelled, etc.
	ResourceID string
	Action     string
	Details    map[string]interface{}
}
