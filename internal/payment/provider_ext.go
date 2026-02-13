package payment

import (
	"context"
	"time"
)

// GoCardlessExtended extends the core Provider with all GoCardless API methods.
// *GoCardlessProvider implements this interface (real GoCardless API; no mocks).
type GoCardlessExtended interface {
	Provider

	// --- Payments ---
	RetryPayment(ctx context.Context, paymentID string) error
	CancelPayment(ctx context.Context, paymentID string) error
	UpdatePayment(ctx context.Context, paymentID string, metadata map[string]string) error
	GetPayment(ctx context.Context, paymentID string) (PaymentDetail, error)

	// --- Refunds ---
	CreateRefund(ctx context.Context, paymentID string, amountMinor int64, reference string) (string, error)
	GetRefund(ctx context.Context, refundID string) (RefundInfo, error)
	ListRefunds(ctx context.Context, paymentID string, limit int) ([]RefundSummary, error)
	UpdateRefund(ctx context.Context, refundID string, metadata map[string]string) error

	// --- Mandates ---
	ReinstateMandate(ctx context.Context, mandateID string) error
	GetMandate(ctx context.Context, mandateID string) (MandateInfo, error)
	UpdateMandate(ctx context.Context, mandateID string, metadata map[string]string) error
	ListMandates(ctx context.Context, customerID string) ([]MandateSummary, error)

	// --- Customers ---
	GetCustomer(ctx context.Context, customerID string) (CustomerDetail, error)
	ListCustomers(ctx context.Context, referencePrefix string) ([]CustomerSummary, error)
	UpdateCustomer(ctx context.Context, customerID, email, name, phone string) error
	RemoveCustomer(ctx context.Context, customerID string) error

	// --- Customer Bank Accounts ---
	CreateCustomerBankAccount(ctx context.Context, customerID, accountHolderName, accountNumber, branchCode, countryCode string) (string, error)
	GetCustomerBankAccount(ctx context.Context, bankAccountID string) (BankAccountInfo, error)
	ListCustomerBankAccounts(ctx context.Context, customerID string) ([]BankAccountSummary, error)
	UpdateCustomerBankAccount(ctx context.Context, bankAccountID string, metadata map[string]string) error
	DisableCustomerBankAccount(ctx context.Context, bankAccountID string) error

	// --- Redirect Flows ---
	CreateRedirectFlow(ctx context.Context, redirectURI, description, sessionToken string) (redirectURL, flowID string, err error)
	GetRedirectFlow(ctx context.Context, flowID string) (RedirectFlowInfo, error)
	CompleteRedirectFlow(ctx context.Context, flowID, sessionToken string) (mandateID, customerID string, err error)

	// --- Subscriptions ---
	CreateSubscription(ctx context.Context, mandateID string, amountMinor int64, currency, interval string, intervalCount int, name, reference string, endDate *time.Time) (string, error)
	GetSubscription(ctx context.Context, subscriptionID string) (SubscriptionInfo, error)
	UpdateSubscription(ctx context.Context, subscriptionID string, amountMinor *int64, name string, endDate *time.Time) error
	CancelSubscription(ctx context.Context, subscriptionID string) error
	PauseSubscription(ctx context.Context, subscriptionID string) error
	ResumeSubscription(ctx context.Context, subscriptionID string) error
	ListSubscriptions(ctx context.Context, customerID, mandateID, status string, limit int) ([]SubscriptionSummary, error)

	// --- Bank Details Lookup ---
	ValidateBankDetails(ctx context.Context, countryCode, accountNumber, branchCode, accountHolderName string) (BankDetailsResult, error)

	// --- Instalment Schedules ---
	CreateInstalmentSchedule(ctx context.Context, mandateID string, totalAmountMinor int64, currency string, instalments []InstalmentDate, name, reference string) (string, error)
	GetInstalmentSchedule(ctx context.Context, scheduleID string) (InstalmentScheduleInfo, error)
	ListInstalmentSchedules(ctx context.Context, mandateID string, limit int) ([]InstalmentScheduleSummary, error)
	UpdateInstalmentSchedule(ctx context.Context, scheduleID string, metadata map[string]string) error
	CancelInstalmentSchedule(ctx context.Context, scheduleID string) error

	// --- Payouts ---
	ListPayouts(ctx context.Context, since, until time.Time) ([]PayoutSummary, error)
	GetPayout(ctx context.Context, payoutID string) (PayoutInfo, error)
	UpdatePayout(ctx context.Context, payoutID string, reference string) error
	ListPayoutItems(ctx context.Context, payoutID string) ([]PayoutItem, error)

	// --- Blocks ---
	CreateBlock(ctx context.Context, customerID, reason string) (string, error)
	GetBlock(ctx context.Context, blockID string) (BlockInfo, error)
	ListBlocks(ctx context.Context, customerID string) ([]BlockSummary, error)
	DisableBlock(ctx context.Context, blockID string) error
	EnableBlock(ctx context.Context, blockID string) error

	// --- Mandate PDFs ---
	CreateMandatePDF(ctx context.Context, mandateID string) (pdfURL string, err error)

	// --- Events ---
	ListEvents(ctx context.Context, resourceType, action string, limit int) ([]EventSummary, error)
	GetEvent(ctx context.Context, eventID string) (EventDetail, error)

	// --- Creditors ---
	ListCreditors(ctx context.Context) ([]CreditorSummary, error)
	GetCreditor(ctx context.Context, creditorID string) (CreditorDetail, error)

	// --- Webhooks (management) ---
	ListWebhooks(ctx context.Context, limit int) ([]WebhookSummary, error)
	GetWebhook(ctx context.Context, webhookID string) (WebhookDetail, error)
	RetryWebhook(ctx context.Context, webhookID string) error

	// --- Payments (list) ---
	ListPayments(ctx context.Context, mandateID, customerID, status string, limit int) ([]PaymentSummary, error)

	// --- Scheme Identifiers ---
	ListSchemeIdentifiers(ctx context.Context) ([]SchemeIdentifierSummary, error)
	GetSchemeIdentifier(ctx context.Context, schemeID string) (SchemeIdentifierDetail, error)
}

// --- Types ---

// PaymentDetail holds full payment details from GetPayment.
type PaymentDetail struct {
	ID          string
	Status      string
	Amount      int64
	Currency    string
	ChargeDate  string
	Description string
	Reference   string
	MandateID   string
	Metadata    map[string]string
}

// RefundInfo holds refund details from GetRefund.
type RefundInfo struct {
	ID        string
	PaymentID string
	Amount    int64
	Status    string
	Reference string
}

// RefundSummary is a list item for ListRefunds.
type RefundSummary struct {
	ID        string
	PaymentID string
	Amount    int64
	Status    string
}

// MandateInfo holds mandate details from GetMandate.
type MandateInfo struct {
	ID         string
	Status     string
	Scheme     string
	Reference  string
	CustomerID string
}

// MandateSummary is a list item for ListMandates.
type MandateSummary struct {
	ID     string
	Status string
}

// CustomerDetail holds full customer details from GetCustomer.
type CustomerDetail struct {
	ID         string
	Email      string
	GivenName  string
	FamilyName string
	Phone      string
	Metadata   map[string]string
}

// CustomerSummary is a list item for ListCustomers.
type CustomerSummary struct {
	ID         string
	Email      string
	GivenName  string
	FamilyName string
}

// BankAccountInfo holds customer bank account details.
type BankAccountInfo struct {
	ID                string
	AccountHolderName string
	AccountNumberEnd  string
	BankName          string
	CountryCode       string
	Currency          string
	Enabled           bool
}

// BankAccountSummary is a list item for ListCustomerBankAccounts.
type BankAccountSummary struct {
	ID                string
	AccountHolderName string
	AccountNumberEnd  string
	BankName          string
	Enabled           bool
}

// RedirectFlowInfo holds redirect flow details from GetRedirectFlow.
type RedirectFlowInfo struct {
	ID          string
	RedirectURL string
	Scheme      string
	MandateID   string
	CustomerID  string
}

// PaymentSummary is a list item for ListPayments.
type PaymentSummary struct {
	ID         string
	Status     string
	Amount     int64
	ChargeDate string
	Reference  string
}

// SubscriptionInfo holds subscription details from GetSubscription.
type SubscriptionInfo struct {
	ID             string
	Status         string
	Amount         int64
	Currency       string
	Interval       string
	IntervalCount  int
	NextChargeDate string
	Name           string
}

// SubscriptionSummary is a list item for ListSubscriptions.
type SubscriptionSummary struct {
	ID             string
	Status         string
	Amount         int64
	NextChargeDate string
}

// BankDetailsResult is the result of ValidateBankDetails.
type BankDetailsResult struct {
	Valid         bool
	BankName      string
	AccountHolder string
}

// InstalmentDate is one instalment in CreateInstalmentSchedule (date + amount in minor units).
type InstalmentDate struct {
	ChargeDate string // YYYY-MM-DD
	Amount     int64
}

// InstalmentScheduleInfo holds instalment schedule details.
type InstalmentScheduleInfo struct {
	ID        string
	Status    string
	Currency  string
	Name      string
	MandateID string
}

// InstalmentScheduleSummary is a list item for ListInstalmentSchedules.
type InstalmentScheduleSummary struct {
	ID     string
	Status string
	Name   string
}

// PayoutSummary is a list item for ListPayouts.
type PayoutSummary struct {
	ID          string
	Amount      int64
	Currency    string
	ArrivalDate string
	Status      string
}

// PayoutInfo holds payout details from GetPayout.
type PayoutInfo struct {
	ID          string
	Amount      int64
	Currency    string
	ArrivalDate string
	Status      string
}

// PayoutItem represents one item within a payout.
type PayoutItem struct {
	Type   string
	Amount int64
	Links  map[string]string
}

// BlockInfo holds block details.
type BlockInfo struct {
	ID         string
	BlockType  string
	Reason     string
	CustomerID string
	Active     bool
}

// BlockSummary is a list item for ListBlocks.
type BlockSummary struct {
	ID        string
	BlockType string
	Reason    string
	Active    bool
}

// EventSummary is a list item for ListEvents.
type EventSummary struct {
	ID           string
	ResourceType string
	Action       string
	CreatedAt    string
}

// EventDetail holds full event details.
type EventDetail struct {
	ID           string
	ResourceType string
	Action       string
	CreatedAt    string
	ResourceID   string
	Details      map[string]string
}

// CreditorSummary is a list item for ListCreditors.
type CreditorSummary struct {
	ID   string
	Name string
}

// CreditorDetail holds full creditor details.
type CreditorDetail struct {
	ID                  string
	Name                string
	AddressLine1        string
	City                string
	PostalCode          string
	CountryCode         string
	VerificationStatus  string
	SchemeIdentifiers   []string
}

// WebhookSummary is a list item for ListWebhooks.
type WebhookSummary struct {
	ID             string
	URL            string
	IsTestEndpoint bool
	SuccessCount   int
}

// WebhookDetail holds full webhook details.
type WebhookDetail struct {
	ID             string
	URL            string
	IsTestEndpoint bool
	ResponseCode   int
	ResponseBody   string
}

// SchemeIdentifierSummary is a list item for ListSchemeIdentifiers.
type SchemeIdentifierSummary struct {
	ID     string
	Name   string
	Scheme string
	Status string
}

// SchemeIdentifierDetail holds full scheme identifier details.
type SchemeIdentifierDetail struct {
	ID              string
	Name            string
	Scheme          string
	Status          string
	Reference       string
	MinAdvanceNotice int
}
