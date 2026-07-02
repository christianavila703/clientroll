package xero

import (
	"context"
	"fmt"
)

func (c *Client) ListConnections(ctx context.Context) ([]Connection, error) {
	type respEnvelope []Connection
	var out respEnvelope
	_, _, err := c.doJSON(ctx, "GET", ConnectionsURL, nil, &out)
	if err != nil {
		return nil, err
	}
	return []Connection(out), nil
}

func (c *Client) UpsertContact(ctx context.Context, in ContactInput) (*Contact, error) {
	type req struct {
		Contacts []ContactInput `json:"Contacts"`
	}
	type resp struct {
		Contacts []Contact `json:"Contacts"`
	}
	var out resp
	_, _, err := c.doJSON(ctx, "PUT", AccountingBaseURL+"/Contacts", req{Contacts: []ContactInput{in}}, &out)
	if err != nil {
		return nil, err
	}
	if len(out.Contacts) == 0 {
		return nil, fmt.Errorf("xero contact response empty")
	}
	return &out.Contacts[0], nil
}

func (c *Client) CreateInvoice(ctx context.Context, in InvoiceInput) (*Invoice, error) {
	type invoiceIn struct {
		Type            string            `json:"Type"`
		Contact         map[string]string `json:"Contact"`
		Status          string            `json:"Status"`
		LineAmountTypes string            `json:"LineAmountTypes"`
		LineItems       []InvoiceLine     `json:"LineItems"`
		CurrencyCode    string            `json:"CurrencyCode,omitempty"`
		DueDateString   string            `json:"DueDate,omitempty"`
		Reference       string            `json:"Reference,omitempty"`
	}
	type req struct {
		Invoices []invoiceIn `json:"Invoices"`
	}
	type resp struct {
		Invoices []Invoice `json:"Invoices"`
	}
	payload := req{
		Invoices: []invoiceIn{{
			Type:            in.Type,
			Contact:         map[string]string{"ContactID": in.ContactID},
			Status:          in.Status,
			LineAmountTypes: in.LineAmountTypes,
			LineItems:       in.LineItems,
			CurrencyCode:    in.CurrencyCode,
			DueDateString:   in.DueDateString,
			Reference:       in.Reference,
		}},
	}
	var out resp
	_, _, err := c.doJSON(ctx, "PUT", AccountingBaseURL+"/Invoices", payload, &out)
	if err != nil {
		return nil, err
	}
	if len(out.Invoices) == 0 {
		return nil, fmt.Errorf("xero invoice response empty")
	}
	return &out.Invoices[0], nil
}

func (c *Client) UpdateInvoiceStatus(ctx context.Context, invoiceID, status string) (*Invoice, error) {
	type invoicePatch struct {
		InvoiceID string `json:"InvoiceID"`
		Status    string `json:"Status"`
	}
	type req struct {
		Invoices []invoicePatch `json:"Invoices"`
	}
	type resp struct {
		Invoices []Invoice `json:"Invoices"`
	}
	payload := req{
		Invoices: []invoicePatch{{
			InvoiceID: invoiceID,
			Status:    status,
		}},
	}
	var out resp
	_, _, err := c.doJSON(ctx, "POST", AccountingBaseURL+"/Invoices", payload, &out)
	if err != nil {
		return nil, err
	}
	if len(out.Invoices) == 0 {
		return nil, fmt.Errorf("xero invoice status update response empty")
	}
	return &out.Invoices[0], nil
}

func (c *Client) CreatePayment(ctx context.Context, in PaymentInput) (*Payment, error) {
	type paymentIn struct {
		Invoice map[string]string `json:"Invoice"`
		Account map[string]string `json:"Account"`
		Amount  float64           `json:"Amount"`
		Date    string            `json:"Date"`
	}
	type req struct {
		Payments []paymentIn `json:"Payments"`
	}
	type resp struct {
		Payments []Payment `json:"Payments"`
	}
	payload := req{
		Payments: []paymentIn{{
			Invoice: map[string]string{"InvoiceID": in.InvoiceID},
			Account: map[string]string{"Code": in.AccountCode},
			Amount:  in.Amount,
			Date:    in.DateString,
		}},
	}
	var out resp
	_, _, err := c.doJSON(ctx, "PUT", AccountingBaseURL+"/Payments", payload, &out)
	if err != nil {
		return nil, err
	}
	if len(out.Payments) == 0 {
		return nil, fmt.Errorf("xero payment response empty")
	}
	return &out.Payments[0], nil
}

func (c *Client) CreateCreditNote(ctx context.Context, in CreditNoteInput) (*CreditNote, error) {
	type creditIn struct {
		Type            string            `json:"Type"`
		Contact         map[string]string `json:"Contact"`
		Status          string            `json:"Status"`
		LineAmountTypes string            `json:"LineAmountTypes"`
		LineItems       []CreditNoteLine  `json:"LineItems"`
		DateString      string            `json:"Date,omitempty"`
		Reference       string            `json:"Reference,omitempty"`
	}
	type req struct {
		CreditNotes []creditIn `json:"CreditNotes"`
	}
	type resp struct {
		CreditNotes []CreditNote `json:"CreditNotes"`
	}
	payload := req{
		CreditNotes: []creditIn{{
			Type:            in.Type,
			Contact:         map[string]string{"ContactID": in.ContactID},
			Status:          in.Status,
			LineAmountTypes: in.LineAmountTypes,
			LineItems:       in.LineItems,
			DateString:      in.DateString,
			Reference:       in.Reference,
		}},
	}
	var out resp
	_, _, err := c.doJSON(ctx, "PUT", AccountingBaseURL+"/CreditNotes", payload, &out)
	if err != nil {
		return nil, err
	}
	if len(out.CreditNotes) == 0 {
		return nil, fmt.Errorf("xero credit note response empty")
	}
	return &out.CreditNotes[0], nil
}
