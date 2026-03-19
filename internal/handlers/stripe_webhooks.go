package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/events"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
)

// StripeWebhookHandler handles public Stripe webhook events.
type StripeWebhookHandler struct {
	db          *database.Postgres
	tenantRepo  *repository.TenantRepo
	credRepo    *repository.TenantCredentialRepo
	eventRepo   *repository.InboundWebhookEventRepo
	encryptor   *database.Encryptor
	paymentRepo *repository.PaymentRepo
	disputeRepo *repository.PaymentDisputeRepo
	memberRepo  *repository.MembershipRepo
	serviceRepo *repository.ServiceRepo
	dispatcher  *events.Dispatcher
}

// NewStripeWebhookHandler creates a Stripe webhook handler.
func NewStripeWebhookHandler(
	db *database.Postgres,
	tr *repository.TenantRepo,
	credRepo *repository.TenantCredentialRepo,
	eventRepo *repository.InboundWebhookEventRepo,
	encryptor *database.Encryptor,
	paymentRepo *repository.PaymentRepo,
	disputeRepo *repository.PaymentDisputeRepo,
	memberRepo *repository.MembershipRepo,
	serviceRepo *repository.ServiceRepo,
) *StripeWebhookHandler {
	return &StripeWebhookHandler{
		db:          db,
		tenantRepo:  tr,
		credRepo:    credRepo,
		eventRepo:   eventRepo,
		encryptor:   encryptor,
		paymentRepo: paymentRepo,
		disputeRepo: disputeRepo,
		memberRepo:  memberRepo,
		serviceRepo: serviceRepo,
	}
}

// SetDispatcher sets optional outbound webhook event dispatcher.
func (h *StripeWebhookHandler) SetDispatcher(dispatcher *events.Dispatcher) {
	h.dispatcher = dispatcher
}

// RegisterPublicRoutes registers public Stripe webhook route.
func (h *StripeWebhookHandler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/webhooks/stripe/{tenantSlug}", h.Webhook)
}

// Webhook handles Stripe events for a specific tenant slug.
func (h *StripeWebhookHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")
	if tenantSlug == "" {
		writeError(w, http.StatusBadRequest, "Missing tenant slug")
		return
	}
	tenant, err := h.tenantRepo.FindBySlug(r.Context(), tenantSlug)
	if err != nil || tenant == nil {
		writeError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	creds, err := h.credRepo.GetByProvider(r.Context(), tenant.ID, "stripe")
	if err != nil || creds == nil || !creds.IsActive || creds.APISecretEncrypted == "" {
		// Acknowledge unconfigured/inactive tenant webhooks to avoid provider retry storms.
		// This can happen when an endpoint exists in Stripe but tenant setup is incomplete.
		w.WriteHeader(http.StatusOK)
		return
	}
	secret, err := h.encryptor.Decrypt(creds.APISecretEncrypted)
	if err != nil || secret == "" {
		log.Printf("stripe webhook secret decrypt failed tenant=%s slug=%s err=%v", tenant.ID, tenantSlug, err)
		writeError(w, http.StatusUnauthorized, "Invalid Stripe webhook secret")
		return
	}
	secret = strings.TrimSpace(secret)

	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	event, err := webhook.ConstructEventWithOptions(
		payload,
		r.Header.Get("Stripe-Signature"),
		secret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true},
	)
	if err != nil {
		sigHeader := r.Header.Get("Stripe-Signature")
		log.Printf("stripe webhook signature verify failed tenant=%s slug=%s payload_len=%d sig_header_len=%d err=%v", tenant.ID, tenantSlug, len(payload), len(sigHeader), err)
		writeError(w, http.StatusUnauthorized, "Invalid Stripe signature")
		return
	}

	if err := h.db.SetTenantContext(r.Context(), tenant.ID.String()); err != nil {
		writeError(w, http.StatusInternalServerError, "Tenant context error")
		return
	}

	if event.ID != "" && h.eventRepo != nil {
		inserted, recErr := h.eventRepo.RecordReceived(
			r.Context(),
			tenant.ID,
			"stripe",
			event.ID,
			stripeResourceType(event.Type),
			stripeResourceIDFromRaw(event.Data.Raw, event.ID),
		)
		if recErr != nil {
			writeError(w, http.StatusInternalServerError, "Webhook dedupe failed")
			return
		}
		if !inserted {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	switch event.Type {
	case "checkout.session.completed":
		h.handleCheckoutCompleted(r.Context(), tenant.ID, event)
	case "checkout.session.expired":
		h.handleCheckoutFailed(r.Context(), tenant.ID, event)
	case "invoice.payment_succeeded":
		h.handleInvoicePayment(r.Context(), tenant.ID, event, true)
	case "invoice.payment_failed":
		h.handleInvoicePayment(r.Context(), tenant.ID, event, false)
	case "customer.subscription.deleted":
		h.handleSubscriptionDeleted(r.Context(), tenant.ID, event)
	case "charge.dispute.created", "charge.dispute.updated", "charge.dispute.closed":
		h.handleDisputeEvent(r.Context(), tenant.ID, event)
	case "charge.refunded", "refund.updated":
		h.handleRefundEvent(r.Context(), tenant.ID, event)
	}
	if event.ID != "" && h.eventRepo != nil {
		_ = h.eventRepo.MarkProcessed(r.Context(), tenant.ID, "stripe", event.ID)
	}
	w.WriteHeader(http.StatusOK)
}

func (h *StripeWebhookHandler) handleCheckoutCompleted(ctx context.Context, tenantID uuid.UUID, event stripe.Event) {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		return
	}
	metadata := sess.Metadata
	if metadata["type"] == "service_charge" {
		if h.serviceRepo == nil {
			return
		}
		chargeID, err := uuid.Parse(strings.TrimSpace(metadata["charge_id"]))
		if err != nil {
			return
		}
		now := time.Now().UTC()
		if err := h.serviceRepo.UpdateChargeStatus(ctx, tenantID, chargeID, "succeeded", &now); err != nil {
			log.Printf("stripe webhook update service charge succeeded failed: %v", err)
		}
		return
	}
	// For subscription checkouts, rely on invoice events for ledger entries.
	// This avoids double-counting the initial payment from both
	// checkout.session.completed and invoice.payment_succeeded.
	if sess.Mode == stripe.CheckoutSessionModeSubscription {
		return
	}
	membershipID, err1 := uuid.Parse(metadata["membership_id"])
	clientID, err2 := uuid.Parse(metadata["client_id"])
	amount, err3 := strconv.ParseFloat(metadata["payment_amount"], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return
	}
	currencyCode := strings.ToUpper(strings.TrimSpace(metadata["currency_code"]))
	if currencyCode == "" {
		currencyCode = "AUD"
	}
	providerRef := sess.ID
	if sess.PaymentIntent != nil && sess.PaymentIntent.ID != "" {
		providerRef = sess.PaymentIntent.ID
	}
	p, _, err := h.paymentRepo.CreateIfNotExists(ctx, tenantID, membershipID, clientID, amount, currencyCode, time.Now().UTC(), providerRef)
	if err != nil {
		log.Printf("stripe webhook create payment failed: %v", err)
		return
	}
	now := time.Now().UTC()
	_ = h.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, "succeeded", &now, nil, nil)
	if h.dispatcher != nil {
		h.dispatcher.Dispatch(ctx, events.NewEvent(events.PaymentSucceeded, tenantID, map[string]interface{}{
			"payment_id":    p.ID,
			"membership_id": membershipID,
			"client_id":     clientID,
			"amount":        amount,
			"status":        "succeeded",
			"provider_ref":  providerRef,
			"source":        "stripe_webhook",
		}))
	}
}

func (h *StripeWebhookHandler) handleCheckoutFailed(ctx context.Context, tenantID uuid.UUID, event stripe.Event) {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		return
	}
	metadata := sess.Metadata
	if metadata["type"] == "service_charge" {
		if h.serviceRepo == nil {
			return
		}
		chargeID, err := uuid.Parse(strings.TrimSpace(metadata["charge_id"]))
		if err != nil {
			return
		}
		if err := h.serviceRepo.UpdateChargeStatus(ctx, tenantID, chargeID, "failed", nil); err != nil {
			log.Printf("stripe webhook update service charge failed failed: %v", err)
		}
		return
	}
	membershipID, err1 := uuid.Parse(metadata["membership_id"])
	clientID, err2 := uuid.Parse(metadata["client_id"])
	amount, err3 := strconv.ParseFloat(metadata["payment_amount"], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return
	}
	currencyCode := strings.ToUpper(strings.TrimSpace(metadata["currency_code"]))
	if currencyCode == "" {
		currencyCode = "AUD"
	}
	p, _, err := h.paymentRepo.CreateIfNotExists(ctx, tenantID, membershipID, clientID, amount, currencyCode, time.Now().UTC(), sess.ID)
	if err != nil {
		return
	}
	reason := "checkout expired"
	_ = h.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, "failed", nil, &reason, nil)
	if h.dispatcher != nil {
		h.dispatcher.Dispatch(ctx, events.NewEvent(events.PaymentFailed, tenantID, map[string]interface{}{
			"payment_id":    p.ID,
			"membership_id": membershipID,
			"client_id":     clientID,
			"amount":        amount,
			"status":        "failed",
			"provider_ref":  sess.ID,
			"source":        "stripe_webhook",
		}))
	}
}

func (h *StripeWebhookHandler) handleInvoicePayment(ctx context.Context, tenantID uuid.UUID, event stripe.Event, succeeded bool) {
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return
	}
	metadata := inv.Metadata
	membershipID, err1 := uuid.Parse(metadata["membership_id"])
	clientID, err2 := uuid.Parse(metadata["client_id"])
	if err1 != nil || err2 != nil {
		return
	}
	amount := float64(inv.AmountPaid) / 100.0
	if !succeeded {
		amount = float64(inv.AmountDue) / 100.0
	}
	providerRef := inv.ID
	currencyCode := strings.ToUpper(string(inv.Currency))
	if currencyCode == "" {
		currencyCode = "AUD"
	}
	p, _, err := h.paymentRepo.CreateIfNotExists(ctx, tenantID, membershipID, clientID, amount, currencyCode, time.Now().UTC(), providerRef)
	if err != nil {
		return
	}
	if succeeded {
		now := time.Now().UTC()
		_ = h.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, "succeeded", &now, nil, nil)
		if h.dispatcher != nil {
			h.dispatcher.Dispatch(ctx, events.NewEvent(events.PaymentSucceeded, tenantID, map[string]interface{}{
				"payment_id":    p.ID,
				"membership_id": membershipID,
				"client_id":     clientID,
				"amount":        amount,
				"status":        "succeeded",
				"provider_ref":  providerRef,
				"source":        "stripe_webhook",
			}))
		}
		return
	}
	reason := "invoice payment failed"
	_ = h.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, "failed", nil, &reason, nil)
	if h.dispatcher != nil {
		h.dispatcher.Dispatch(ctx, events.NewEvent(events.PaymentFailed, tenantID, map[string]interface{}{
			"payment_id":    p.ID,
			"membership_id": membershipID,
			"client_id":     clientID,
			"amount":        amount,
			"status":        "failed",
			"provider_ref":  providerRef,
			"source":        "stripe_webhook",
		}))
	}
}

func (h *StripeWebhookHandler) handleSubscriptionDeleted(ctx context.Context, tenantID uuid.UUID, event stripe.Event) {
	if h.memberRepo == nil {
		return
	}
	var sub struct {
		ID       string            `json:"id"`
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return
	}
	membershipID, err := uuid.Parse(sub.Metadata["membership_id"])
	if err != nil {
		log.Printf("stripe webhook subscription.deleted missing membership metadata subscription=%s", sub.ID)
		return
	}
	m, err := h.memberRepo.FindByID(ctx, membershipID, tenantID)
	if err != nil || m == nil {
		return
	}
	if err := h.memberRepo.UpdateStatus(ctx, membershipID, tenantID, "cancelled"); err != nil {
		return
	}
	if h.dispatcher != nil {
		h.dispatcher.Dispatch(ctx, events.NewEvent(events.MembershipCancelled, tenantID, map[string]interface{}{
			"membership_id":   m.ID,
			"client_id":       m.ClientID,
			"subscription_id": sub.ID,
			"status":          "cancelled",
			"source":          "stripe_webhook",
		}))
	}
}

func (h *StripeWebhookHandler) handleDisputeEvent(ctx context.Context, tenantID uuid.UUID, event stripe.Event) {
	if h.disputeRepo == nil || h.paymentRepo == nil {
		return
	}
	var dispute struct {
		ID            string      `json:"id"`
		Amount        int64       `json:"amount"`
		Reason        string      `json:"reason"`
		Status        string      `json:"status"`
		PaymentIntent interface{} `json:"payment_intent"`
		Charge        interface{} `json:"charge"`
	}
	if err := json.Unmarshal(event.Data.Raw, &dispute); err != nil {
		return
	}

	chargeID := stripeExpandableID(dispute.Charge)
	paymentIntentID := stripeExpandableID(dispute.PaymentIntent)
	p, err := h.paymentRepo.GetByProviderReferences(ctx, tenantID, paymentIntentID, chargeID)
	if err != nil || p == nil {
		return
	}

	disputeStatus, paymentStatus, resolvedAt := mapStripeDisputeState(event.Type, dispute.Status)
	reason := strings.TrimSpace(dispute.Reason)
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	disputeID := strings.TrimSpace(dispute.ID)
	var disputeIDPtr *string
	if disputeID != "" {
		disputeIDPtr = &disputeID
	}
	amount := p.Amount
	if dispute.Amount > 0 {
		amount = float64(dispute.Amount) / 100.0
	}
	_, _ = h.disputeRepo.UpsertByProviderOrPayment(
		ctx,
		tenantID,
		p.ID,
		p.ClientID,
		disputeIDPtr,
		reasonPtr,
		disputeStatus,
		amount,
		nil,
		nil,
		resolvedAt,
	)

	if err := h.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, paymentStatus, nil, nil, nil); err != nil {
		return
	}
	if h.dispatcher != nil {
		h.dispatcher.Dispatch(ctx, events.NewEvent(events.PaymentDisputed, tenantID, map[string]interface{}{
			"payment_id":      p.ID,
			"membership_id":   p.MembershipID,
			"client_id":       p.ClientID,
			"amount":          amount,
			"status":          paymentStatus,
			"dispute_id":      dispute.ID,
			"dispute_status":  disputeStatus,
			"dispute_reason":  dispute.Reason,
			"provider_ref":    firstNonEmpty(paymentIntentID, chargeID),
			"provider_charge": chargeID,
			"source":          "stripe_webhook",
		}))
	}
}

func (h *StripeWebhookHandler) handleRefundEvent(ctx context.Context, tenantID uuid.UUID, event stripe.Event) {
	if h.paymentRepo == nil {
		return
	}

	refs := []string{}
	amount := 0.0

	switch event.Type {
	case "charge.refunded":
		var charge struct {
			ID             string      `json:"id"`
			PaymentIntent  interface{} `json:"payment_intent"`
			Invoice        interface{} `json:"invoice"`
			AmountRefunded int64       `json:"amount_refunded"`
		}
		if err := json.Unmarshal(event.Data.Raw, &charge); err != nil {
			return
		}
		refs = append(refs, charge.ID, stripeExpandableID(charge.PaymentIntent), stripeExpandableID(charge.Invoice))
		if charge.AmountRefunded > 0 {
			amount = float64(charge.AmountRefunded) / 100.0
		}
	case "refund.updated":
		var refund struct {
			ID            string      `json:"id"`
			Status        string      `json:"status"`
			Amount        int64       `json:"amount"`
			Charge        interface{} `json:"charge"`
			PaymentIntent interface{} `json:"payment_intent"`
		}
		if err := json.Unmarshal(event.Data.Raw, &refund); err != nil {
			return
		}
		if refund.Status != "" && refund.Status != "succeeded" {
			return
		}
		refs = append(refs, stripeExpandableID(refund.PaymentIntent), stripeExpandableID(refund.Charge))
		if refund.Amount > 0 {
			amount = float64(refund.Amount) / 100.0
		}
	}

	p, err := h.paymentRepo.GetByProviderReferences(ctx, tenantID, refs...)
	if err != nil || p == nil {
		return
	}
	if err := h.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, "refunded", nil, nil, nil); err != nil {
		return
	}
	if amount <= 0 {
		amount = p.Amount
	}
	if h.dispatcher != nil {
		h.dispatcher.Dispatch(ctx, events.NewEvent(events.PaymentRefunded, tenantID, map[string]interface{}{
			"payment_id":    p.ID,
			"membership_id": p.MembershipID,
			"client_id":     p.ClientID,
			"amount":        amount,
			"status":        "refunded",
			"provider_ref":  firstNonEmpty(refs...),
			"source":        "stripe_webhook",
		}))
	}
}

func stripeResourceType(eventType stripe.EventType) string {
	parts := strings.SplitN(string(eventType), ".", 2)
	if len(parts) == 0 || parts[0] == "" {
		return "stripe_event"
	}
	return parts[0]
}

func stripeResourceIDFromRaw(raw json.RawMessage, fallback string) string {
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fallback
	}
	if id, ok := payload["id"].(string); ok && id != "" {
		return id
	}
	return fallback
}

func stripeExpandableID(v interface{}) string {
	switch vv := v.(type) {
	case string:
		return vv
	case map[string]interface{}:
		if id, ok := vv["id"].(string); ok {
			return id
		}
	}
	return ""
}

func mapStripeDisputeState(eventType stripe.EventType, stripeStatus string) (string, string, *time.Time) {
	status := strings.ToLower(strings.TrimSpace(stripeStatus))
	switch status {
	case "warning_needs_response", "warning_under_review":
		return "under_review", "disputed", nil
	case "won":
		now := time.Now().UTC()
		return "won", "succeeded", &now
	case "lost":
		now := time.Now().UTC()
		return "lost", "disputed", &now
	}
	if eventType == "charge.dispute.closed" {
		now := time.Now().UTC()
		return "resolved", "disputed", &now
	}
	return "open", "disputed", nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
