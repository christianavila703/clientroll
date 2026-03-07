package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v82/webhook"
)

// PlatformBillingWebhookHandler handles Stripe webhooks for platform billing.
// This is separate from tenant Stripe webhooks used for card payments.
type PlatformBillingWebhookHandler struct {
	enc      *database.Encryptor
	cfgRepo  *repository.PlatformBillingConfigRepo
	subRepo  *repository.TenantSubscriptionRepo
	invRepo  *repository.TenantInvoiceRepo
}

func NewPlatformBillingWebhookHandler(
	enc *database.Encryptor,
	cfgRepo *repository.PlatformBillingConfigRepo,
	subRepo *repository.TenantSubscriptionRepo,
	invRepo *repository.TenantInvoiceRepo,
) *PlatformBillingWebhookHandler {
	return &PlatformBillingWebhookHandler{
		enc:     enc,
		cfgRepo: cfgRepo,
		subRepo: subRepo,
		invRepo: invRepo,
	}
}

func (h *PlatformBillingWebhookHandler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/webhooks/stripe/platform-billing", h.Handle)
}

func (h *PlatformBillingWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.cfgRepo.GetOrCreate(r.Context())
	if err != nil {
		log.Printf("platform billing webhook: config load error: %v", err)
		writeError(w, http.StatusInternalServerError, "Config error")
		return
	}
	if cfg.StripeWebhookSecretEncrypted == nil || *cfg.StripeWebhookSecretEncrypted == "" {
		writeError(w, http.StatusServiceUnavailable, "Webhook secret not configured")
		return
	}
	secret, err := h.enc.Decrypt(*cfg.StripeWebhookSecretEncrypted)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Webhook secret decrypt failed")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid body")
		return
	}

	sig := r.Header.Get("Stripe-Signature")
	if sig == "" {
		writeError(w, http.StatusBadRequest, "Missing Stripe-Signature header")
		return
	}

	event, err := webhook.ConstructEvent(body, sig, secret)
	if err != nil {
		log.Printf("platform billing webhook: signature verification failed: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid signature")
		return
	}

	switch event.Type {
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		if err := h.handleSubscriptionEvent(r, event.Data.Raw, event.Type); err != nil {
			log.Printf("platform billing webhook: subscription handler error: %v", err)
			writeError(w, http.StatusInternalServerError, "Webhook processing error")
			return
		}
	case "invoice.created", "invoice.finalized", "invoice.paid", "invoice.payment_failed", "invoice.voided":
		if err := h.handleInvoiceEvent(r, event.Data.Raw, event.Type); err != nil {
			log.Printf("platform billing webhook: invoice handler error: %v", err)
			writeError(w, http.StatusInternalServerError, "Webhook processing error")
			return
		}
	case "payment_method.attached", "payment_method.updated":
		if err := h.handlePaymentMethodEvent(r, event.Data.Raw); err != nil {
			log.Printf("platform billing webhook: payment method handler error: %v", err)
			writeError(w, http.StatusInternalServerError, "Webhook processing error")
			return
		}
	default:
		// Ignore other events.
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type stripeSubscriptionPayload struct {
	ID                 string            `json:"id"`
	Customer           interface{}       `json:"customer"`
	Status             string            `json:"status"`
	CurrentPeriodStart int64             `json:"current_period_start"`
	CurrentPeriodEnd   int64             `json:"current_period_end"`
	Items              struct {
		Data []struct {
			ID       string `json:"id"`
			Quantity *int64 `json:"quantity"`
			Price    struct {
				UnitAmount int64 `json:"unit_amount"`
			} `json:"price"`
		} `json:"data"`
	} `json:"items"`
	Metadata map[string]string `json:"metadata"`
}

func (h *PlatformBillingWebhookHandler) handleSubscriptionEvent(r *http.Request, raw json.RawMessage, eventType string) error {
	var p stripeSubscriptionPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}

	tenantID, err := h.resolveTenantIDFromMetadataOrCustomer(r.Context(), p.Metadata, p.Customer)
	if err != nil {
		return err
	}

	_, _ = h.subRepo.UpsertPending(r.Context(), tenantID)

	status := p.Status
	if eventType == "customer.subscription.deleted" {
		status = "cancelled"
	}

	var itemID *string
	var qty int
	var monthly float64
	if len(p.Items.Data) > 0 {
		item := p.Items.Data[0]
		if item.ID != "" {
			itemID = &item.ID
		}
		if item.Quantity != nil {
			qty = int(*item.Quantity)
		}
		if item.Price.UnitAmount != 0 {
			monthly = (float64(item.Price.UnitAmount) / 100.0) * float64(qty)
		}
	}

	var cps, cpe *time.Time
	if p.CurrentPeriodStart > 0 {
		t := time.Unix(p.CurrentPeriodStart, 0).UTC()
		cps = &t
	}
	if p.CurrentPeriodEnd > 0 {
		t := time.Unix(p.CurrentPeriodEnd, 0).UTC()
		cpe = &t
	}

	// Grace policy: 7 days after first entering past_due.
	var pastDueSince, graceUntil *time.Time
	if status == "past_due" {
		existing, _ := h.subRepo.GetByTenant(r.Context(), tenantID)
		if existing != nil && existing.PastDueSince != nil && existing.BillingGraceUntil != nil {
			pastDueSince = existing.PastDueSince
			graceUntil = existing.BillingGraceUntil
		} else {
			now := time.Now().UTC()
			pastDueSince = &now
			gu := now.Add(7 * 24 * time.Hour)
			graceUntil = &gu
		}
	}

	subID := p.ID
	customerID := firstString(p.Customer)

	u := repository.TenantSubscriptionUpdate{
		StripeSubscriptionID:     &subID,
		StripeCustomerID:         &customerID,
		StripeSubscriptionItemID: itemID,
		Status:                   &status,
		ActiveMemberCount:        &qty,
		CurrentMonthlyAmountUSD:  &monthly,
		CurrentPeriodStart:       cps,
		CurrentPeriodEnd:         cpe,
	}
	if status == "past_due" {
		u.PastDueSince = pastDueSince
		u.BillingGraceUntil = graceUntil
	} else {
		// Clear grace when not past_due.
		nullTime := (*time.Time)(nil)
		u.PastDueSince = nullTime
		u.BillingGraceUntil = nullTime
	}

	_, err = h.subRepo.UpdateByTenant(r.Context(), tenantID, u)
	return err
}

type stripeInvoicePayload struct {
	ID              string            `json:"id"`
	Customer        interface{}       `json:"customer"`
	Subscription    interface{}       `json:"subscription"`
	Status          string            `json:"status"`
	HostedInvoiceURL *string           `json:"hosted_invoice_url"`
	InvoicePDF      *string           `json:"invoice_pdf"`
	AmountPaid      int64             `json:"amount_paid"`
	AmountDue       int64             `json:"amount_due"`
	StatusTransitions struct {
		PaidAt int64 `json:"paid_at"`
	} `json:"status_transitions"`
	Lines struct {
		Data []struct {
			Period struct {
				Start int64 `json:"start"`
				End   int64 `json:"end"`
			} `json:"period"`
			Quantity *int64 `json:"quantity"`
		} `json:"data"`
	} `json:"lines"`
	Metadata map[string]string `json:"metadata"`
}

func (h *PlatformBillingWebhookHandler) handleInvoiceEvent(r *http.Request, raw json.RawMessage, eventType string) error {
	var p stripeInvoicePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}

	// Resolve tenant from metadata or customer/subscription.
	tenantID, err := h.resolveTenantIDFromMetadataOrCustomer(r.Context(), p.Metadata, p.Customer)
	if err != nil {
		// Try subscription lookup if customer is missing.
		subID := firstString(p.Subscription)
		if subID != "" {
			id, subErr := h.subRepo.GetTenantIDByStripeSubscriptionID(r.Context(), subID)
			if subErr == nil {
				tenantID = id
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}

	periodStart := time.Now().UTC()
	periodEnd := periodStart
	memberCount := 0
	if len(p.Lines.Data) > 0 {
		ln := p.Lines.Data[0]
		if ln.Period.Start > 0 {
			periodStart = time.Unix(ln.Period.Start, 0).UTC()
		}
		if ln.Period.End > 0 {
			periodEnd = time.Unix(ln.Period.End, 0).UTC()
		}
		if ln.Quantity != nil {
			memberCount = int(*ln.Quantity)
		}
	}

	amountUSD := float64(0)
	if p.AmountPaid > 0 {
		amountUSD = float64(p.AmountPaid) / 100.0
	} else if p.AmountDue > 0 {
		amountUSD = float64(p.AmountDue) / 100.0
	}

	var paidAt *time.Time
	if p.StatusTransitions.PaidAt > 0 {
		t := time.Unix(p.StatusTransitions.PaidAt, 0).UTC()
		paidAt = &t
	}

	_, err = h.invRepo.UpsertByStripeInvoiceID(r.Context(), tenantID, repository.UpsertTenantInvoiceInput{
		StripeInvoiceID:  p.ID,
		AmountUSD:        amountUSD,
		MemberCount:      memberCount,
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		Status:           p.Status,
		PaidAt:           paidAt,
		HostedInvoiceURL: p.HostedInvoiceURL,
		InvoicePDFURL:    p.InvoicePDF,
	})
	return err
}

type stripePaymentMethodPayload struct {
	ID       string      `json:"id"`
	Customer interface{} `json:"customer"`
	Card     struct {
		Brand    string `json:"brand"`
		Last4    string `json:"last4"`
		ExpMonth int    `json:"exp_month"`
		ExpYear  int    `json:"exp_year"`
	} `json:"card"`
}

func (h *PlatformBillingWebhookHandler) handlePaymentMethodEvent(r *http.Request, raw json.RawMessage) error {
	var p stripePaymentMethodPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	customerID := firstString(p.Customer)
	if customerID == "" {
		return nil
	}

	tenantID, err := h.subRepo.GetTenantIDByStripeCustomerID(r.Context(), customerID)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	hasValid := isCardNotExpired(p.Card.ExpMonth, p.Card.ExpYear, now)

	brand := p.Card.Brand
	last4 := p.Card.Last4
	expMonth := p.Card.ExpMonth
	expYear := p.Card.ExpYear

	u := repository.TenantSubscriptionUpdate{
		HasValidPaymentMethod: &hasValid,
		CardBrand:             &brand,
		CardLast4:             &last4,
		CardExpMonth:          &expMonth,
		CardExpYear:           &expYear,
	}
	_, err = h.subRepo.UpdateByTenant(r.Context(), tenantID, u)
	return err
}

func (h *PlatformBillingWebhookHandler) resolveTenantIDFromMetadataOrCustomer(ctx context.Context, metadata map[string]string, customer interface{}) (uuid.UUID, error) {
	if metadata != nil {
		if tid := strings.TrimSpace(metadata["tenant_id"]); tid != "" {
			return uuid.Parse(tid)
		}
	}

	custID := firstString(customer)
	if custID == "" {
		return uuid.Nil, fmt.Errorf("missing tenant_id metadata and customer id")
	}
	return h.subRepo.GetTenantIDByStripeCustomerID(ctx, custID)
}

func firstString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]interface{}:
		if id, ok := t["id"].(string); ok {
			return id
		}
	case map[string]string:
		if id, ok := t["id"]; ok {
			return id
		}
	}
	return ""
}

func isCardNotExpired(expMonth, expYear int, now time.Time) bool {
	if expMonth <= 0 || expYear <= 0 {
		return false
	}
	// Card is valid through the end of the expiration month.
	// Compare to the first day of the next month.
	year := now.Year()
	month := int(now.Month())
	if expYear > year {
		return true
	}
	if expYear < year {
		return false
	}
	return expMonth >= month
}

