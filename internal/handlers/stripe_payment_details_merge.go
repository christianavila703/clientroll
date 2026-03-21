package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v82"
)

// mergeStripeDetailsForInvoice merges Stripe invoice / PI context onto the payment row
// keyed by provider_reference = invoice ID.
func (h *StripeWebhookHandler) mergeStripeDetailsForInvoice(ctx context.Context, tenantID uuid.UUID, invoiceID string, inv *stripe.Invoice, eventCreated int64) error {
	if h.paymentRepo == nil || invoiceID == "" || inv == nil {
		return nil
	}
	patch := buildStripeDetailsPatchFromInvoice(inv, eventCreated)
	return h.paymentRepo.MergeStripeDetails(ctx, tenantID, invoiceID, patch)
}

func (h *StripeWebhookHandler) mergeStripeDetailsFromPaymentIntent(ctx context.Context, tenantID uuid.UUID, raw []byte, eventCreated int64) error {
	if h.paymentRepo == nil {
		return nil
	}
	invoiceID, patch := buildStripeDetailsPatchFromPaymentIntent(raw, eventCreated)
	if invoiceID == "" || len(patch) == 0 {
		return nil
	}
	return h.paymentRepo.MergeStripeDetails(ctx, tenantID, invoiceID, patch)
}

func (h *StripeWebhookHandler) mergeStripeDetailsFromCharge(ctx context.Context, tenantID uuid.UUID, raw []byte, eventCreated int64) error {
	if h.paymentRepo == nil {
		return nil
	}
	invoiceID, patch := buildStripeDetailsPatchFromCharge(raw, eventCreated)
	if invoiceID == "" || len(patch) == 0 {
		return nil
	}
	return h.paymentRepo.MergeStripeDetails(ctx, tenantID, invoiceID, patch)
}

func buildStripeDetailsPatchFromInvoice(inv *stripe.Invoice, eventCreated int64) map[string]interface{} {
	patch := map[string]interface{}{
		"source": "stripe",
	}
	if inv.ID != "" {
		patch["invoice_id"] = inv.ID
	}
	if inv.Number != "" {
		patch["invoice_number"] = inv.Number
	}
	if inv.Status != "" {
		patch["stripe_invoice_status"] = string(inv.Status)
	}
	if inv.Currency != "" {
		patch["currency"] = strings.ToLower(string(inv.Currency))
	}
	if inv.Customer != nil && inv.Customer.ID != "" {
		patch["stripe_customer_id"] = inv.Customer.ID
	}
	if inv.CustomerEmail != "" {
		patch["stripe_customer_email"] = inv.CustomerEmail
	}
	if inv.CustomerName != "" {
		patch["stripe_customer_name"] = inv.CustomerName
	}
	if inv.Parent != nil && inv.Parent.SubscriptionDetails != nil && inv.Parent.SubscriptionDetails.Subscription != nil && inv.Parent.SubscriptionDetails.Subscription.ID != "" {
		patch["stripe_subscription_id"] = inv.Parent.SubscriptionDetails.Subscription.ID
	}
	if inv.Lines != nil && len(inv.Lines.Data) > 0 {
		d := strings.TrimSpace(inv.Lines.Data[0].Description)
		if d != "" {
			patch["stripe_line_description"] = d
		}
	}
	if strings.TrimSpace(inv.Description) != "" {
		patch["stripe_description"] = strings.TrimSpace(inv.Description)
	}
	if inv.StatusTransitions != nil && inv.StatusTransitions.FinalizedAt > 0 {
		patch["invoice_finalized_at"] = time.Unix(inv.StatusTransitions.FinalizedAt, 0).UTC().Format(time.RFC3339)
	}
	if eventCreated > 0 {
		patch["last_stripe_event_at"] = time.Unix(eventCreated, 0).UTC().Format(time.RFC3339)
	}
	return patch
}

func buildStripeDetailsPatchFromPaymentIntent(raw []byte, eventCreated int64) (invoiceID string, patch map[string]interface{}) {
	var pi struct {
		ID            string          `json:"id"`
		Status        string          `json:"status"`
		Description   string          `json:"description"`
		Created       int64           `json:"created"`
		Invoice       json.RawMessage `json:"invoice"`
		LatestCharge  json.RawMessage `json:"latest_charge"`
		PaymentMethod json.RawMessage `json:"payment_method"`
	}
	if err := json.Unmarshal(raw, &pi); err != nil {
		return "", nil
	}
	invoiceID = stripeExpandableStringID(pi.Invoice)
	if invoiceID == "" {
		return "", nil
	}
	patch = map[string]interface{}{
		"source":                "stripe",
		"payment_intent_id":     pi.ID,
		"payment_intent_status": pi.Status,
	}
	if pi.Description != "" {
		patch["stripe_description"] = pi.Description
	}
	if lc := stripeExpandableStringID(pi.LatestCharge); lc != "" {
		patch["stripe_charge_id"] = lc
	}
	if pm := stripeExpandableStringID(pi.PaymentMethod); pm != "" {
		patch["stripe_payment_method_id"] = pm
	}
	mergePaymentMethodObject(pi.PaymentMethod, patch)
	if pi.Created > 0 {
		patch["payment_intent_created_at"] = time.Unix(pi.Created, 0).UTC().Format(time.RFC3339)
	}
	if eventCreated > 0 {
		patch["last_stripe_event_at"] = time.Unix(eventCreated, 0).UTC().Format(time.RFC3339)
	}
	return invoiceID, patch
}

func buildStripeDetailsPatchFromCharge(raw []byte, eventCreated int64) (invoiceID string, patch map[string]interface{}) {
	var ch stripe.Charge
	if err := json.Unmarshal(raw, &ch); err != nil {
		return "", nil
	}
	var top map[string]interface{}
	_ = json.Unmarshal(raw, &top)
	invoiceID = stripeInvoiceIDFromMap(top)
	if invoiceID == "" {
		return "", nil
	}
	patch = map[string]interface{}{
		"source":               "stripe",
		"stripe_charge_id":     ch.ID,
		"stripe_charge_status": string(ch.Status),
	}
	if ch.PaymentIntent != nil && ch.PaymentIntent.ID != "" {
		patch["payment_intent_id"] = ch.PaymentIntent.ID
	}
	if pmd := ch.PaymentMethodDetails; pmd != nil && pmd.AUBECSDebit != nil {
		b := pmd.AUBECSDebit
		patch["payment_method_type"] = "au_becs_debit"
		patch["bank_bsb"] = b.BSBNumber
		patch["bank_last4"] = b.Last4
		if b.Mandate != "" {
			patch["stripe_mandate_id"] = b.Mandate
		}
		if b.BSBNumber != "" && b.Last4 != "" {
			patch["payment_method_label"] = fmt.Sprintf("Australia BECS Direct Debit · BSB %s · Last 4 %s", b.BSBNumber, b.Last4)
		}
	}
	if eventCreated > 0 {
		patch["last_stripe_event_at"] = time.Unix(eventCreated, 0).UTC().Format(time.RFC3339)
	}
	return invoiceID, patch
}

func stripeInvoiceIDFromMap(m map[string]interface{}) string {
	if m == nil {
		return ""
	}
	v, ok := m["invoice"]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	if sub, ok := v.(map[string]interface{}); ok {
		if id, _ := sub["id"].(string); id != "" {
			return id
		}
	}
	return ""
}

func stripeExpandableStringID(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return s
	}
	var o struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &o); err == nil && o.ID != "" {
		return o.ID
	}
	return ""
}

// handlePaymentIntentStripe records PI / mandate / bank hints onto the invoice-linked payment row.
func (h *StripeWebhookHandler) handlePaymentIntentStripe(ctx context.Context, tenantID uuid.UUID, event stripe.Event) error {
	return h.mergeStripeDetailsFromPaymentIntent(ctx, tenantID, event.Data.Raw, event.Created)
}

// handleChargeStripe merges charge-level BECS / mandate IDs (Stripe often omits invoice on Charge in older structs; raw JSON still has it).
func (h *StripeWebhookHandler) handleChargeStripe(ctx context.Context, tenantID uuid.UUID, event stripe.Event) error {
	return h.mergeStripeDetailsFromCharge(ctx, tenantID, event.Data.Raw, event.Created)
}

func mergePaymentMethodObject(raw json.RawMessage, patch map[string]interface{}) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var s string
	if json.Unmarshal(raw, &s) == nil && s != "" {
		return
	}
	var pm struct {
		Type       string `json:"type"`
		AUBECS     *struct {
			BSBNumber string `json:"bsb_number"`
			Last4     string `json:"last4"`
		} `json:"au_becs_debit"`
		SEPA      *struct {
			Last4 string `json:"last4"`
		} `json:"sepa_debit"`
		USBank    *struct {
			Last4 string `json:"last4"`
		} `json:"us_bank_account"`
	}
	if err := json.Unmarshal(raw, &pm); err != nil {
		return
	}
	if pm.Type != "" {
		patch["payment_method_type"] = pm.Type
	}
	switch pm.Type {
	case "au_becs_debit":
		if pm.AUBECS != nil {
			patch["bank_bsb"] = pm.AUBECS.BSBNumber
			patch["bank_last4"] = pm.AUBECS.Last4
			if pm.AUBECS.BSBNumber != "" && pm.AUBECS.Last4 != "" {
				patch["payment_method_label"] = fmt.Sprintf("Australia BECS Direct Debit · BSB %s · Last 4 %s", pm.AUBECS.BSBNumber, pm.AUBECS.Last4)
			} else if pm.AUBECS.Last4 != "" {
				patch["payment_method_label"] = fmt.Sprintf("Australia BECS Direct Debit · Last 4 %s", pm.AUBECS.Last4)
			} else {
				patch["payment_method_label"] = "Australia BECS Direct Debit"
			}
		}
	case "sepa_debit":
		if pm.SEPA != nil && pm.SEPA.Last4 != "" {
			patch["payment_method_label"] = fmt.Sprintf("SEPA Direct Debit · Last 4 %s", pm.SEPA.Last4)
		} else {
			patch["payment_method_label"] = "SEPA Direct Debit"
		}
	case "us_bank_account":
		if pm.USBank != nil && pm.USBank.Last4 != "" {
			patch["payment_method_label"] = fmt.Sprintf("US bank account · Last 4 %s", pm.USBank.Last4)
		} else {
			patch["payment_method_label"] = "US bank account (ACH)"
		}
	}
}
