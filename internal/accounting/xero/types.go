package xero

import "time"

const (
	AuthBaseURL       = "https://login.xero.com/identity/connect/authorize"
	TokenURL          = "https://identity.xero.com/connect/token"
	ConnectionsURL    = "https://api.xero.com/connections"
	AccountingBaseURL = "https://api.xero.com/api.xro/2.0"
)

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

func (t TokenResponse) Expiry(now time.Time) time.Time {
	ttl := t.ExpiresIn
	if ttl <= 0 {
		ttl = 1800
	}
	return now.Add(time.Duration(ttl) * time.Second)
}

type Connection struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenantId"`
	TenantType     string    `json:"tenantType"`
	TenantName     string    `json:"tenantName"`
	CreatedDateUTC time.Time `json:"createdDateUtc"`
	UpdatedDateUTC time.Time `json:"updatedDateUtc"`
}

type ContactInput struct {
	Name         string  `json:"Name"`
	EmailAddress *string `json:"EmailAddress,omitempty"`
}

type Contact struct {
	ContactID string  `json:"ContactID"`
	Name      string  `json:"Name"`
	Email     *string `json:"EmailAddress,omitempty"`
}

type InvoiceLine struct {
	Description string  `json:"Description"`
	Quantity    float64 `json:"Quantity"`
	UnitAmount  float64 `json:"UnitAmount"`
	AccountCode string  `json:"AccountCode"`
	TaxType     string  `json:"TaxType"`
}

type InvoiceInput struct {
	Type            string        `json:"Type"`
	ContactID       string        `json:"ContactID"`
	Status          string        `json:"Status"`
	LineAmountTypes string        `json:"LineAmountTypes"`
	LineItems       []InvoiceLine `json:"LineItems"`
	CurrencyCode    string        `json:"CurrencyCode,omitempty"`
	DueDateString   string        `json:"DueDate,omitempty"`
	Reference       string        `json:"Reference,omitempty"`
}

type Invoice struct {
	InvoiceID     string `json:"InvoiceID"`
	InvoiceNumber string `json:"InvoiceNumber"`
	Status        string `json:"Status"`
}

type PaymentInput struct {
	InvoiceID   string  `json:"InvoiceID"`
	AccountCode string  `json:"AccountCode"`
	Amount      float64 `json:"Amount"`
	DateString  string  `json:"Date"`
}

type Payment struct {
	PaymentID string `json:"PaymentID"`
	Status    string `json:"Status"`
}

type CreditNoteLine struct {
	Description string  `json:"Description"`
	Quantity    float64 `json:"Quantity"`
	UnitAmount  float64 `json:"UnitAmount"`
	AccountCode string  `json:"AccountCode"`
	TaxType     string  `json:"TaxType"`
}

type CreditNoteInput struct {
	Type            string           `json:"Type"`
	ContactID       string           `json:"ContactID"`
	Status          string           `json:"Status"`
	LineAmountTypes string           `json:"LineAmountTypes"`
	LineItems       []CreditNoteLine `json:"LineItems"`
	DateString      string           `json:"Date,omitempty"`
	Reference       string           `json:"Reference,omitempty"`
}

type CreditNote struct {
	CreditNoteID string `json:"CreditNoteID"`
	Number       string `json:"CreditNoteNumber"`
	Status       string `json:"Status"`
}
