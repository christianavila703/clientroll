package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/events"
	"clientroll/internal/observability"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v82"
	stripeevent "github.com/stripe/stripe-go/v82/event"
	"github.com/stripe/stripe-go/v82/subscription"
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

// RegisterPlatformRoutes registers internal platform-admin webhook ops routes.
func (h *StripeWebhookHandler) RegisterPlatformRoutes(r chi.Router) {
	r.Get("/webhooks/stripe/failures", h.ListFailedStripeWebhookEvents)
	r.Post("/webhooks/stripe/replay", h.ReplayStripeWebhookEvent)
	r.Post("/webhooks/stripe/replay-failures", h.ReplayFailedStripeWebhookEvents)
}

// Webhook handles Stripe events for a specific tenant slug.
func (h *StripeWebhookHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	observability.PaymentLifecycleMetrics.IncWebhookReceived()
	tenantSlug := chi.URLParam(r, "tenantSlug")
	if tenantSlug == "" {
		observability.PaymentLifecycleMetrics.IncWebhookRejected()
		writeError(w, http.StatusBadRequest, "Missing tenant slug")
		return
	}
	tenant, err := h.tenantRepo.FindBySlug(r.Context(), tenantSlug)
	if err != nil || tenant == nil {
		observability.PaymentLifecycleMetrics.IncWebhookRejected()
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
		observability.PaymentLifecycleMetrics.IncWebhookRejected()
		log.Printf("stripe webhook secret decrypt failed tenant=%s slug=%s err=%v", tenant.ID, tenantSlug, err)
		writeError(w, http.StatusUnauthorized, "Invalid Stripe webhook secret")
		return
	}
	secret = strings.TrimSpace(secret)

	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		observability.PaymentLifecycleMetrics.IncWebhookRejected()
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
		observability.PaymentLifecycleMetrics.IncWebhookRejected()
		sigHeader := r.Header.Get("Stripe-Signature")
		log.Printf("stripe webhook signature verify failed tenant=%s slug=%s payload_len=%d sig_header_len=%d err=%v", tenant.ID, tenantSlug, len(payload), len(sigHeader), err)
		writeError(w, http.StatusUnauthorized, "Invalid Stripe signature")
		return
	}

	if err := h.db.SetTenantContext(r.Context(), tenant.ID.String()); err != nil {
		observability.PaymentLifecycleMetrics.IncWebhookRejected()
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
			observability.PaymentLifecycleMetrics.IncWebhookFailed()
			writeError(w, http.StatusInternalServerError, "Webhook dedupe failed")
			return
		}
		if !inserted {
			observability.PaymentLifecycleMetrics.IncWebhookDuplicate()
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	handleErr := h.processStripeEvent(r.Context(), tenant.ID, event)
	if handleErr != nil {
		observability.PaymentLifecycleMetrics.IncWebhookFailed()
		if event.ID != "" && h.eventRepo != nil {
			_ = h.eventRepo.MarkFailed(r.Context(), tenant.ID, "stripe", event.ID, handleErr.Error())
		}
		log.Printf("stripe webhook handler failed tenant=%s event=%s type=%s err=%v", tenant.ID, event.ID, event.Type, handleErr)
		writeError(w, http.StatusInternalServerError, "Webhook processing failed")
		return
	}
	if event.ID != "" && h.eventRepo != nil {
		if err := h.eventRepo.MarkProcessed(r.Context(), tenant.ID, "stripe", event.ID); err != nil {
			observability.PaymentLifecycleMetrics.IncWebhookFailed()
			_ = h.eventRepo.MarkFailed(r.Context(), tenant.ID, "stripe", event.ID, err.Error())
			log.Printf("stripe webhook mark processed failed tenant=%s event=%s type=%s err=%v", tenant.ID, event.ID, event.Type, err)
		}
	}
	observability.PaymentLifecycleMetrics.IncWebhookProcessed()
	w.WriteHeader(http.StatusOK)
}

// shouldSyncMembershipAfterSubscriptionCheckout is true when Checkout completed and either the customer
// paid (or no payment was required), or the subscription is in trial (deferred first charge / future start).
// We do not treat async direct-debit "unpaid + active" as succeeded here — invoice events will settle that.
func shouldSyncMembershipAfterSubscriptionCheckout(sess *stripe.CheckoutSession, raw []byte) bool {
	if sess == nil {
		return false
	}
	if sess.Status != stripe.CheckoutSessionStatusComplete {
		return false
	}
	switch sess.PaymentStatus {
	case stripe.CheckoutSessionPaymentStatusPaid, stripe.CheckoutSessionPaymentStatusNoPaymentRequired:
		return true
	}
	if sess.Subscription != nil && sess.Subscription.Status == stripe.SubscriptionStatusTrialing {
		return true
	}
	return checkoutSessionSubscriptionRawIsTrialing(raw)
}

func checkoutSessionSubscriptionRawIsTrialing(raw []byte) bool {
	var payload struct {
		Subscription json.RawMessage `json:"subscription"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || len(payload.Subscription) == 0 {
		return false
	}
	var subObj struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(payload.Subscription, &subObj); err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(subObj.Status), string(stripe.SubscriptionStatusTrialing))
}

func invoiceSubscriptionIDFromPayload(inv *stripe.Invoice, raw []byte) string {
	if inv != nil && inv.Parent != nil && inv.Parent.SubscriptionDetails != nil &&
		inv.Parent.SubscriptionDetails.Subscription != nil &&
		strings.TrimSpace(inv.Parent.SubscriptionDetails.Subscription.ID) != "" {
		return strings.TrimSpace(inv.Parent.SubscriptionDetails.Subscription.ID)
	}
	var payload struct {
		Subscription interface{} `json:"subscription"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	switch v := payload.Subscription.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]interface{}:
		if id, _ := v["id"].(string); strings.TrimSpace(id) != "" {
			return strings.TrimSpace(id)
		}
	}
	return ""
}

// subscriptionMetadataForStripeAPI loads subscription metadata (e.g. membership_id) using the tenant secret key.
func (h *StripeWebhookHandler) subscriptionMetadataForStripeAPI(ctx context.Context, tenantID uuid.UUID, subscriptionID string) (map[string]string, error) {
	if h.credRepo == nil || h.encryptor == nil {
		return nil, fmt.Errorf("stripe unavailable")
	}
	creds, err := h.credRepo.GetByProvider(ctx, tenantID, "stripe")
	if err != nil || creds == nil || strings.TrimSpace(creds.APIKeyEncrypted) == "" {
		return nil, fmt.Errorf("stripe not configured")
	}
	key, err := h.encryptor.Decrypt(creds.APIKeyEncrypted)
	if err != nil || strings.TrimSpace(key) == "" {
		return nil, fmt.Errorf("stripe key invalid")
	}
	stripe.Key = strings.TrimSpace(key)
	sub, err := subscription.Get(strings.TrimSpace(subscriptionID), nil)
	if err != nil {
		return nil, fmt.Errorf("subscription get: %w", err)
	}
	if sub == nil {
		return nil, fmt.Errorf("subscription missing")
	}
	return sub.Metadata, nil
}

func (h *StripeWebhookHandler) processStripeEvent(ctx context.Context, tenantID uuid.UUID, event stripe.Event) error {
	switch event.Type {
	case "checkout.session.completed":
		return h.handleCheckoutCompleted(ctx, tenantID, event)
	case "checkout.session.expired":
		return h.handleCheckoutFailed(ctx, tenantID, event)
	case "invoice.payment_succeeded":
		return h.handleInvoicePayment(ctx, tenantID, event, true)
	case "invoice.payment_failed":
		return h.handleInvoicePayment(ctx, tenantID, event, false)
	case "invoice.finalized":
		return h.handleInvoiceFinalized(ctx, tenantID, event)
	case "payment_intent.processing", "payment_intent.succeeded", "payment_intent.payment_failed":
		return h.handlePaymentIntentStripe(ctx, tenantID, event)
	case "charge.succeeded", "charge.updated":
		return h.handleChargeStripe(ctx, tenantID, event)
	case "customer.subscription.deleted":
		return h.handleSubscriptionDeleted(ctx, tenantID, event)
	case "charge.dispute.created", "charge.dispute.updated", "charge.dispute.closed":
		return h.handleDisputeEvent(ctx, tenantID, event)
	case "charge.refunded", "refund.updated":
		return h.handleRefundEvent(ctx, tenantID, event)
	default:
		return nil
	}
}

// ListFailedStripeWebhookEvents returns internal failed Stripe webhook processing events.
func (h *StripeWebhookHandler) ListFailedStripeWebhookEvents(w http.ResponseWriter, r *http.Request) {
	if h.eventRepo == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"events": []interface{}{}, "count": 0})
		return
	}
	limit := 50
	var tenantID *uuid.UUID
	if rawTenant := strings.TrimSpace(r.URL.Query().Get("tenant_id")); rawTenant != "" {
		id, err := uuid.Parse(rawTenant)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid tenant_id")
			return
		}
		tenantID = &id
	}
	if s := strings.TrimSpace(r.URL.Query().Get("limit")); s != "" {
		if parsed, err := strconv.Atoi(s); err == nil {
			limit = parsed
		}
	}
	events, err := h.eventRepo.ListFailedFiltered(r.Context(), "stripe", tenantID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list failed Stripe events")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}

// ReplayFailedStripeWebhookEvents reprocesses failed Stripe inbound events in batch.
// Platform-admin only route; dry_run defaults to true for safety.
func (h *StripeWebhookHandler) ReplayFailedStripeWebhookEvents(w http.ResponseWriter, r *http.Request) {
	if h.eventRepo == nil {
		writeError(w, http.StatusInternalServerError, "Webhook event repository unavailable")
		return
	}
	var input struct {
		TenantID string `json:"tenant_id"`
		Limit    *int   `json:"limit"`
		DryRun   *bool  `json:"dry_run"`
	}
	_ = decodeJSON(r, &input) // optional

	var tenantFilter *uuid.UUID
	if strings.TrimSpace(input.TenantID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(input.TenantID))
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid tenant_id")
			return
		}
		tenantFilter = &id
	}
	limit := 50
	if input.Limit != nil {
		limit = *input.Limit
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	dryRun := true
	if input.DryRun != nil {
		dryRun = *input.DryRun
	}

	failedEvents, err := h.eventRepo.ListFailedFiltered(r.Context(), "stripe", tenantFilter, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list failed Stripe events")
		return
	}
	type replayResult struct {
		TenantID uuid.UUID `json:"tenant_id"`
		EventID  string    `json:"event_id"`
		Type     string    `json:"type,omitempty"`
		Status   string    `json:"status"`
		Error    string    `json:"error,omitempty"`
	}
	results := make([]replayResult, 0, len(failedEvents))

	for _, item := range failedEvents {
		if dryRun {
			results = append(results, replayResult{
				TenantID: item.TenantID,
				EventID:  item.EventID,
				Status:   "would_replay",
			})
			continue
		}
		evType, replayErr := h.replayStripeEventByID(r.Context(), item.TenantID, item.EventID)
		if replayErr != nil {
			_ = h.eventRepo.MarkFailed(r.Context(), item.TenantID, "stripe", item.EventID, replayErr.Error())
			results = append(results, replayResult{
				TenantID: item.TenantID,
				EventID:  item.EventID,
				Type:     string(evType),
				Status:   "failed",
				Error:    replayErr.Error(),
			})
			continue
		}
		results = append(results, replayResult{
			TenantID: item.TenantID,
			EventID:  item.EventID,
			Type:     string(evType),
			Status:   "replayed",
		})
	}

	replayed := 0
	failed := 0
	for _, r := range results {
		if r.Status == "replayed" {
			replayed++
		}
		if r.Status == "failed" {
			failed++
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "ok",
		"dry_run":      dryRun,
		"requested":    len(failedEvents),
		"replayed":     replayed,
		"failed":       failed,
		"results":      results,
		"tenant_scope": tenantFilter,
	})
}

// ReplayStripeWebhookEvent re-fetches and reprocesses a Stripe event (platform-admin only).
func (h *StripeWebhookHandler) ReplayStripeWebhookEvent(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TenantID string `json:"tenant_id"`
		EventID  string `json:"event_id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	tenantID, err := uuid.Parse(strings.TrimSpace(input.TenantID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant_id")
		return
	}
	eventID := strings.TrimSpace(input.EventID)
	if eventID == "" {
		writeError(w, http.StatusBadRequest, "event_id is required")
		return
	}
	evType, replayErr := h.replayStripeEventByID(r.Context(), tenantID, eventID)
	if replayErr != nil {
		if h.eventRepo != nil {
			_ = h.eventRepo.MarkFailed(r.Context(), tenantID, "stripe", eventID, replayErr.Error())
		}
		writeError(w, http.StatusBadGateway, "Stripe replay processing failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "replayed",
		"tenant_id": tenantID,
		"event_id":  eventID,
		"type":      evType,
	})
}

func (h *StripeWebhookHandler) replayStripeEventByID(ctx context.Context, tenantID uuid.UUID, eventID string) (stripe.EventType, error) {
	tenant, err := h.tenantRepo.FindByID(ctx, tenantID)
	if err != nil || tenant == nil {
		return "", fmt.Errorf("tenant not found")
	}
	creds, err := h.credRepo.GetByProvider(ctx, tenantID, "stripe")
	if err != nil || creds == nil || !creds.IsActive || strings.TrimSpace(creds.APIKeyEncrypted) == "" {
		return "", fmt.Errorf("stripe not configured for tenant")
	}
	secretKey, err := h.encryptor.Decrypt(creds.APIKeyEncrypted)
	if err != nil || strings.TrimSpace(secretKey) == "" {
		return "", fmt.Errorf("stripe credentials invalid")
	}
	stripe.Key = secretKey
	ev, err := stripeevent.Get(eventID, nil)
	if err != nil || ev == nil {
		return "", fmt.Errorf("failed to fetch stripe event")
	}
	if !stripeEventBelongsToTenant(ev.Data.Raw, tenantID, tenant.Slug) {
		return ev.Type, fmt.Errorf("stripe event does not belong to tenant")
	}
	if err := h.db.SetTenantContext(ctx, tenantID.String()); err != nil {
		return ev.Type, fmt.Errorf("tenant context error")
	}
	if processErr := h.processStripeEvent(ctx, tenantID, *ev); processErr != nil {
		return ev.Type, processErr
	}
	if h.eventRepo != nil && strings.TrimSpace(ev.ID) != "" {
		_, _ = h.eventRepo.RecordReceived(
			ctx,
			tenantID,
			"stripe",
			ev.ID,
			stripeResourceType(ev.Type),
			stripeResourceIDFromRaw(ev.Data.Raw, ev.ID),
		)
		_ = h.eventRepo.MarkProcessed(ctx, tenantID, "stripe", ev.ID)
	}
	return ev.Type, nil
}

func (h *StripeWebhookHandler) handleCheckoutCompleted(ctx context.Context, tenantID uuid.UUID, event stripe.Event) error {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		return fmt.Errorf("decode checkout.session.completed: %w", err)
	}
	metadata := sess.Metadata
	if metadata["type"] == "service_charge" {
		if h.serviceRepo == nil {
			return nil
		}
		chargeID, err := uuid.Parse(strings.TrimSpace(metadata["charge_id"]))
		if err != nil {
			return nil
		}
		// Store provider linkage + Stripe snapshot for service charges too.
		providerRef := sess.ID
		paymentMethod := strings.TrimSpace(metadata["payment_method"])
		if paymentMethod == "" {
			paymentMethod = "card"
		}
		if err := h.serviceRepo.UpdateChargeProviderContext(ctx, tenantID, chargeID, &providerRef, &paymentMethod); err != nil {
			log.Printf("stripe service_charge provider context tenant=%s charge=%s err=%v", tenantID, chargeID, err)
		}
		if err := h.mergeStripeDetailsForServiceCheckout(ctx, tenantID, chargeID, &sess, event.Created); err != nil {
			log.Printf("stripe service_charge checkout details tenant=%s charge=%s err=%v", tenantID, chargeID, err)
		}
		// Direct-debit checkout.session.completed can still be unpaid/processing.
		if strings.EqualFold(string(sess.PaymentStatus), "paid") {
			now := time.Now().UTC()
			if err := h.serviceRepo.UpdateChargeStatus(ctx, tenantID, chargeID, "succeeded", &now); err != nil {
				return fmt.Errorf("update service charge succeeded: %w", err)
			}
		} else {
			if err := h.serviceRepo.UpdateChargeStatus(ctx, tenantID, chargeID, "pending", nil); err != nil {
				return fmt.Errorf("update service charge pending: %w", err)
			}
		}
		return nil
	}
	// For subscription checkouts: set cancel_at when term has an end (continue_after_term false).
	// For BECS/async methods we also create a pending ledger row now (invoice-linked),
	// then later invoice/payment_intent events enrich and settle it.
	if sess.Mode == stripe.CheckoutSessionModeSubscription {
		if err := h.setSubscriptionCancelAtFromCheckout(ctx, tenantID, event.Data.Raw, metadata); err != nil {
			log.Printf("stripe setSubscriptionCancelAtFromCheckout tenant=%s err=%v", tenantID, err)
			// non-fatal: subscription is still created, just won't auto-end
		}
		if err := h.recordPendingFromSubscriptionCheckout(ctx, tenantID, event.Data.Raw, &sess, metadata, event.Created); err != nil {
			log.Printf("stripe recordPendingFromSubscriptionCheckout tenant=%s err=%v", tenantID, err)
			// non-fatal: invoice/payment events can still create/update this ledger row later.
		}
		// Align membership with billing: subscription-mode checkout used to skip the payment-mode path
		// below, so we never called syncMembershipStatusAfterPayment. Invoice webhooks also often lack
		// membership_id on the Invoice object (metadata lives on the Subscription), so invoice handlers
		// could no-op. Session metadata always has membership_id from signup.
		if h.memberRepo != nil {
			if mid, err := uuid.Parse(strings.TrimSpace(metadata["membership_id"])); err == nil {
				now := time.Now().UTC()
				if shouldSyncMembershipAfterSubscriptionCheckout(&sess, event.Data.Raw) {
					if err := syncMembershipStatusAfterPayment(ctx, h.memberRepo, tenantID, mid, "succeeded", now); err != nil {
						log.Printf("stripe checkout.subscription membership sync tenant=%s membership=%s err=%v", tenantID, mid, err)
					}
				}
			}
		}
		return nil
	}
	membershipID, err1 := uuid.Parse(metadata["membership_id"])
	clientID, err2 := uuid.Parse(metadata["client_id"])
	amount, err3 := strconv.ParseFloat(metadata["payment_amount"], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return nil
	}
	currencyCode := strings.ToUpper(strings.TrimSpace(metadata["currency_code"]))
	if currencyCode == "" {
		currencyCode = "AUD"
	}
	providerRef := sess.ID
	if sess.PaymentIntent != nil && sess.PaymentIntent.ID != "" {
		providerRef = sess.PaymentIntent.ID
	}
	components := parseMetadataPaymentComponents(metadata, amount)
	p, _, err := h.paymentRepo.CreateIfNotExists(ctx, tenantID, membershipID, clientID, amount, currencyCode, time.Now().UTC(), providerRef, components)
	if err != nil {
		return fmt.Errorf("create payment: %w", err)
	}
	now := time.Now().UTC()
	if err := h.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, "succeeded", &now, nil, nil); err != nil {
		return fmt.Errorf("update payment succeeded status: %w", err)
	}
	if err := syncMembershipStatusAfterPayment(ctx, h.memberRepo, tenantID, membershipID, "succeeded", now); err != nil {
		log.Printf("stripe membership status sync failed tenant=%s membership=%s err=%v", tenantID, membershipID, err)
	}
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
	return nil
}

func (h *StripeWebhookHandler) recordPendingFromSubscriptionCheckout(
	ctx context.Context,
	tenantID uuid.UUID,
	raw []byte,
	sess *stripe.CheckoutSession,
	metadata map[string]string,
	eventCreated int64,
) error {
	if h.paymentRepo == nil || sess == nil {
		return nil
	}
	// If already paid instantly (e.g. some card flows), invoice.payment_succeeded will handle status.
	if strings.EqualFold(string(sess.PaymentStatus), "paid") {
		return nil
	}
	invoiceID := stripeCheckoutInvoiceID(raw)
	if invoiceID == "" {
		return nil
	}
	membershipID, err1 := uuid.Parse(metadata["membership_id"])
	clientID, err2 := uuid.Parse(metadata["client_id"])
	amount, err3 := strconv.ParseFloat(metadata["payment_amount"], 64)
	if err1 != nil || err2 != nil || err3 != nil || amount <= 0 {
		return nil
	}
	currencyCode := strings.ToUpper(strings.TrimSpace(metadata["currency_code"]))
	if currencyCode == "" {
		currencyCode = "AUD"
	}
	components := parseMetadataPaymentComponents(metadata, amount)
	p, _, err := h.paymentRepo.CreateIfNotExists(ctx, tenantID, membershipID, clientID, amount, currencyCode, time.Now().UTC(), invoiceID, components)
	if err != nil {
		return fmt.Errorf("create pending payment from checkout: %w", err)
	}
	if p.Status != "succeeded" && p.Status != "failed" && p.Status != "refunded" && p.Status != "pending" {
		if err := h.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, "pending", nil, nil, nil); err != nil {
			return fmt.Errorf("set pending from checkout: %w", err)
		}
	}
	patch := map[string]interface{}{
		"source":                "stripe",
		"invoice_id":            invoiceID,
		"checkout_session_id":   sess.ID,
		"stripe_invoice_status": "open",
	}
	if sess.Subscription != nil && sess.Subscription.ID != "" {
		patch["stripe_subscription_id"] = sess.Subscription.ID
	}
	if sess.Customer != nil && sess.Customer.ID != "" {
		patch["stripe_customer_id"] = sess.Customer.ID
	}
	if sess.CustomerEmail != "" {
		patch["stripe_customer_email"] = sess.CustomerEmail
	}
	if sess.CustomerDetails != nil && strings.TrimSpace(sess.CustomerDetails.Name) != "" {
		patch["stripe_customer_name"] = strings.TrimSpace(sess.CustomerDetails.Name)
	}
	if sess.Currency != "" {
		patch["currency"] = strings.ToLower(string(sess.Currency))
	}
	if sess.PaymentStatus != "" {
		patch["checkout_payment_status"] = strings.ToLower(string(sess.PaymentStatus))
	}
	if eventCreated > 0 {
		patch["last_stripe_event_at"] = time.Unix(eventCreated, 0).UTC().Format(time.RFC3339)
	}
	if err := h.paymentRepo.MergeStripeDetails(ctx, tenantID, invoiceID, patch); err != nil {
		return fmt.Errorf("merge checkout stripe details: %w", err)
	}
	return nil
}

func stripeCheckoutInvoiceID(raw []byte) string {
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	v, ok := payload["invoice"]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	if o, ok := v.(map[string]interface{}); ok {
		if id, _ := o["id"].(string); id != "" {
			return strings.TrimSpace(id)
		}
	}
	return ""
}

// setSubscriptionCancelAtFromCheckout sets cancel_at on the Stripe subscription when the
// membership term has an end (continue_after_term is false). Raw is the checkout.session object JSON.
func (h *StripeWebhookHandler) setSubscriptionCancelAtFromCheckout(ctx context.Context, tenantID uuid.UUID, raw []byte, metadata map[string]string) error {
	if metadata["continue_after_term"] == "true" {
		return nil
	}
	cancelAtStr := strings.TrimSpace(metadata["cancel_at_timestamp"])
	if cancelAtStr == "" {
		return nil
	}
	cancelAt, err := strconv.ParseInt(cancelAtStr, 10, 64)
	if err != nil || cancelAt <= 0 {
		return nil
	}
	var sessRaw struct {
		Subscription interface{} `json:"subscription"`
	}
	if err := json.Unmarshal(raw, &sessRaw); err != nil {
		return fmt.Errorf("parse session: %w", err)
	}
	if sessRaw.Subscription == nil {
		return nil
	}
	var subID string
	switch v := sessRaw.Subscription.(type) {
	case string:
		subID = v
	case map[string]interface{}:
		if id, _ := v["id"].(string); id != "" {
			subID = id
		}
	}
	if subID == "" {
		return fmt.Errorf("missing subscription id in session")
	}
	creds, err := h.credRepo.GetByProvider(ctx, tenantID, "stripe")
	if err != nil || creds == nil || creds.APIKeyEncrypted == "" {
		return fmt.Errorf("stripe not configured: %w", err)
	}
	secretKey, err := h.encryptor.Decrypt(creds.APIKeyEncrypted)
	if err != nil || secretKey == "" {
		return fmt.Errorf("stripe credentials: %w", err)
	}
	stripe.Key = secretKey
	_, err = subscription.Update(subID, &stripe.SubscriptionParams{
		CancelAt:          stripe.Int64(cancelAt),
		CancelAtPeriodEnd: stripe.Bool(false),
	})
	return err
}

func (h *StripeWebhookHandler) handleCheckoutFailed(ctx context.Context, tenantID uuid.UUID, event stripe.Event) error {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		return fmt.Errorf("decode checkout.session.expired: %w", err)
	}
	metadata := sess.Metadata
	if metadata["type"] == "service_charge" {
		if h.serviceRepo == nil {
			return nil
		}
		chargeID, err := uuid.Parse(strings.TrimSpace(metadata["charge_id"]))
		if err != nil {
			return nil
		}
		if err := h.serviceRepo.UpdateChargeStatus(ctx, tenantID, chargeID, "failed", nil); err != nil {
			return fmt.Errorf("update service charge failed status: %w", err)
		}
		providerRef := sess.ID
		paymentMethod := strings.TrimSpace(metadata["payment_method"])
		if paymentMethod == "" {
			paymentMethod = "card"
		}
		if err := h.serviceRepo.UpdateChargeProviderContext(ctx, tenantID, chargeID, &providerRef, &paymentMethod); err != nil {
			log.Printf("stripe service_charge failed provider context tenant=%s charge=%s err=%v", tenantID, chargeID, err)
		}
		if err := h.mergeStripeDetailsForServiceCheckout(ctx, tenantID, chargeID, &sess, event.Created); err != nil {
			log.Printf("stripe service_charge checkout failed details tenant=%s charge=%s err=%v", tenantID, chargeID, err)
		}
		return nil
	}
	membershipID, err1 := uuid.Parse(metadata["membership_id"])
	clientID, err2 := uuid.Parse(metadata["client_id"])
	amount, err3 := strconv.ParseFloat(metadata["payment_amount"], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return nil
	}
	currencyCode := strings.ToUpper(strings.TrimSpace(metadata["currency_code"]))
	if currencyCode == "" {
		currencyCode = "AUD"
	}
	components := parseMetadataPaymentComponents(metadata, amount)
	p, _, err := h.paymentRepo.CreateIfNotExists(ctx, tenantID, membershipID, clientID, amount, currencyCode, time.Now().UTC(), sess.ID, components)
	if err != nil {
		return fmt.Errorf("create failed payment: %w", err)
	}
	reason := "checkout expired"
	if err := h.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, "failed", nil, &reason, nil); err != nil {
		return fmt.Errorf("update payment failed status: %w", err)
	}
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
	return nil
}

func (h *StripeWebhookHandler) handleInvoicePayment(ctx context.Context, tenantID uuid.UUID, event stripe.Event, succeeded bool) error {
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return fmt.Errorf("decode invoice event: %w", err)
	}
	metadata := make(map[string]string)
	for k, v := range inv.Metadata {
		metadata[k] = v
	}
	// Stripe v82+ attaches a snapshot of subscription metadata under parent.subscription_details (June 2023+).
	if inv.Parent != nil && inv.Parent.SubscriptionDetails != nil && inv.Parent.SubscriptionDetails.Metadata != nil {
		for k, v := range inv.Parent.SubscriptionDetails.Metadata {
			if strings.TrimSpace(metadata[k]) == "" && strings.TrimSpace(v) != "" {
				metadata[k] = v
			}
		}
	}
	membershipID, err1 := uuid.Parse(strings.TrimSpace(metadata["membership_id"]))
	clientID, err2 := uuid.Parse(strings.TrimSpace(metadata["client_id"]))
	if err1 != nil || err2 != nil {
		// Older payloads / edge cases: subscription metadata lives only on the Subscription object.
		subID := invoiceSubscriptionIDFromPayload(&inv, event.Data.Raw)
		if subID != "" {
			subMeta, metaErr := h.subscriptionMetadataForStripeAPI(ctx, tenantID, subID)
			if metaErr != nil {
				log.Printf("stripe invoice webhook subscription metadata tenant=%s invoice=%s sub=%s err=%v", tenantID, inv.ID, subID, metaErr)
				return nil
			}
			for k, v := range subMeta {
				if strings.TrimSpace(metadata[k]) == "" && strings.TrimSpace(v) != "" {
					metadata[k] = v
				}
			}
			membershipID, err1 = uuid.Parse(strings.TrimSpace(metadata["membership_id"]))
			clientID, err2 = uuid.Parse(strings.TrimSpace(metadata["client_id"]))
		}
	}
	if err1 != nil || err2 != nil {
		return nil
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
	components := parseMetadataPaymentComponents(metadata, amount)
	if components == nil {
		components = paymentComponentsFromInvoiceRaw(event.Data.Raw, amount)
	}
	if components == nil || components.ProcessingFeeAmount <= 0.004 {
		if fallback := h.invoicePaymentFallbackComponents(ctx, tenantID, membershipID, amount, components); fallback != nil {
			components = fallback
		}
	}
	p, _, err := h.paymentRepo.CreateIfNotExists(ctx, tenantID, membershipID, clientID, amount, currencyCode, time.Now().UTC(), providerRef, components)
	if err != nil {
		return fmt.Errorf("create invoice payment: %w", err)
	}
	if err := h.mergeStripeDetailsForInvoice(ctx, tenantID, inv.ID, &inv, event.Created); err != nil {
		log.Printf("stripe merge invoice details tenant=%s invoice=%s err=%v", tenantID, inv.ID, err)
	}
	if succeeded {
		now := time.Now().UTC()
		if err := h.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, "succeeded", &now, nil, nil); err != nil {
			return fmt.Errorf("update invoice payment succeeded status: %w", err)
		}
		if err := syncMembershipStatusAfterPayment(ctx, h.memberRepo, tenantID, membershipID, "succeeded", now); err != nil {
			log.Printf("stripe membership status sync failed tenant=%s membership=%s err=%v", tenantID, membershipID, err)
		}
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
		return nil
	}
	reason := "invoice payment failed"
	if err := h.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, "failed", nil, &reason, nil); err != nil {
		return fmt.Errorf("update invoice payment failed status: %w", err)
	}
	if err := syncMembershipStatusAfterPayment(ctx, h.memberRepo, tenantID, membershipID, "failed", time.Now().UTC()); err != nil {
		log.Printf("stripe membership status sync failed tenant=%s membership=%s err=%v", tenantID, membershipID, err)
	}
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
	return nil
}

// handleInvoiceFinalized records membership subscription invoices while payment is still in flight
// (e.g. BECS direct debit). Without this, payments only appear after invoice.payment_succeeded.
func (h *StripeWebhookHandler) handleInvoiceFinalized(ctx context.Context, tenantID uuid.UUID, event stripe.Event) error {
	if h.paymentRepo == nil {
		return nil
	}
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return fmt.Errorf("decode invoice.finalized: %w", err)
	}
	// Only unpaid open invoices; paid/void/draft are not "pending collection".
	if inv.Status != stripe.InvoiceStatusOpen {
		return nil
	}
	metadata := make(map[string]string)
	for k, v := range inv.Metadata {
		metadata[k] = v
	}
	if inv.Parent != nil && inv.Parent.SubscriptionDetails != nil && inv.Parent.SubscriptionDetails.Metadata != nil {
		for k, v := range inv.Parent.SubscriptionDetails.Metadata {
			if strings.TrimSpace(metadata[k]) == "" && strings.TrimSpace(v) != "" {
				metadata[k] = v
			}
		}
	}
	if metadata["type"] == "service_charge" {
		return nil
	}
	membershipID, err1 := uuid.Parse(strings.TrimSpace(metadata["membership_id"]))
	clientID, err2 := uuid.Parse(strings.TrimSpace(metadata["client_id"]))
	if err1 != nil || err2 != nil {
		subID := invoiceSubscriptionIDFromPayload(&inv, event.Data.Raw)
		if subID != "" {
			subMeta, metaErr := h.subscriptionMetadataForStripeAPI(ctx, tenantID, subID)
			if metaErr != nil {
				log.Printf("stripe invoice.finalized subscription metadata tenant=%s invoice=%s sub=%s err=%v", tenantID, inv.ID, subID, metaErr)
				return nil
			}
			for k, v := range subMeta {
				if strings.TrimSpace(metadata[k]) == "" && strings.TrimSpace(v) != "" {
					metadata[k] = v
				}
			}
			membershipID, err1 = uuid.Parse(strings.TrimSpace(metadata["membership_id"]))
			clientID, err2 = uuid.Parse(strings.TrimSpace(metadata["client_id"]))
		}
	}
	if err1 != nil || err2 != nil {
		return nil
	}
	amountDue := float64(inv.AmountDue) / 100.0
	if amountDue <= 0 {
		return nil
	}
	providerRef := inv.ID
	currencyCode := strings.ToUpper(string(inv.Currency))
	if currencyCode == "" {
		currencyCode = "AUD"
	}
	components := parseMetadataPaymentComponents(metadata, amountDue)
	if components == nil {
		components = paymentComponentsFromInvoiceRaw(event.Data.Raw, amountDue)
	}
	if components == nil || components.ProcessingFeeAmount <= 0.004 {
		if fallback := h.invoicePaymentFallbackComponents(ctx, tenantID, membershipID, amountDue, components); fallback != nil {
			components = fallback
		}
	}
	p, _, err := h.paymentRepo.CreateIfNotExists(ctx, tenantID, membershipID, clientID, amountDue, currencyCode, time.Now().UTC(), providerRef, components)
	if err != nil {
		return fmt.Errorf("create pending invoice payment: %w", err)
	}
	if err := h.mergeStripeDetailsForInvoice(ctx, tenantID, inv.ID, &inv, event.Created); err != nil {
		log.Printf("stripe merge invoice details (finalized) tenant=%s invoice=%s err=%v", tenantID, inv.ID, err)
	}
	// Do not downgrade a payment that already settled (webhook ordering).
	if p.Status == "succeeded" || p.Status == "failed" || p.Status == "refunded" {
		return nil
	}
	if p.Status != "pending" {
		if err := h.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, "pending", nil, nil, nil); err != nil {
			return fmt.Errorf("set invoice payment pending: %w", err)
		}
	}
	return nil
}

// invoicePaymentFallbackComponents derives base/tax/fee when Stripe webhook payload omits split details.
// It uses membership payment_amount as the organisation amount and treats the remaining delta as processing fee.
func (h *StripeWebhookHandler) invoicePaymentFallbackComponents(
	ctx context.Context,
	tenantID uuid.UUID,
	membershipID uuid.UUID,
	total float64,
	existing *repository.PaymentComponents,
) *repository.PaymentComponents {
	if h.memberRepo == nil {
		return existing
	}
	membership, err := h.memberRepo.FindByID(ctx, membershipID, tenantID)
	if err != nil || membership == nil || membership.PaymentAmount == nil {
		return existing
	}
	orgAmount := *membership.PaymentAmount
	if orgAmount <= 0 || orgAmount-total > 0.02 {
		return existing
	}

	fee := total - orgAmount
	if fee < 0 {
		fee = 0
	}
	if existing != nil && existing.ProcessingFeeAmount > 0.004 {
		fee = existing.ProcessingFeeAmount
	}

	taxPercent := 10.0
	if h.tenantRepo != nil {
		if tenant, tErr := h.tenantRepo.FindByID(ctx, tenantID); tErr == nil && tenant != nil {
			taxPercent = tenantSalesTaxPercent(tenant)
		}
	}
	applySalesTax := true
	if membership != nil && h.memberRepo != nil {
		if v, mtErr := h.memberRepo.MembershipTypeApplySalesTax(ctx, membership.MembershipTypeID, tenantID); mtErr == nil {
			applySalesTax = v
		}
	}
	base, tax := splitAmountByTaxMode(orgAmount, taxPercent, applySalesTax)
	out := &repository.PaymentComponents{
		BaseAmount:          base,
		TaxAmount:           tax,
		ProcessingFeeAmount: fee,
	}
	if math.Abs((out.BaseAmount+out.TaxAmount+out.ProcessingFeeAmount)-total) > 0.05 {
		return existing
	}
	return out
}

func (h *StripeWebhookHandler) handleSubscriptionDeleted(ctx context.Context, tenantID uuid.UUID, event stripe.Event) error {
	if h.memberRepo == nil {
		return nil
	}
	var sub struct {
		ID       string            `json:"id"`
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return fmt.Errorf("decode subscription.deleted: %w", err)
	}
	membershipID, err := uuid.Parse(sub.Metadata["membership_id"])
	if err != nil {
		log.Printf("stripe webhook subscription.deleted missing membership metadata subscription=%s", sub.ID)
		return nil
	}
	m, err := h.memberRepo.FindByID(ctx, membershipID, tenantID)
	if err != nil || m == nil {
		return nil
	}
	if err := h.memberRepo.UpdateStatus(ctx, membershipID, tenantID, "cancelled"); err != nil {
		return fmt.Errorf("cancel membership from subscription.deleted: %w", err)
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
	return nil
}

func (h *StripeWebhookHandler) handleDisputeEvent(ctx context.Context, tenantID uuid.UUID, event stripe.Event) error {
	if h.disputeRepo == nil || h.paymentRepo == nil {
		return nil
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
		return fmt.Errorf("decode dispute event: %w", err)
	}

	chargeID := stripeExpandableID(dispute.Charge)
	paymentIntentID := stripeExpandableID(dispute.PaymentIntent)
	p, err := h.paymentRepo.GetByProviderReferences(ctx, tenantID, paymentIntentID, chargeID)
	if err != nil || p == nil {
		return nil
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
	if _, err := h.disputeRepo.UpsertByProviderOrPayment(
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
	); err != nil {
		return fmt.Errorf("upsert dispute: %w", err)
	}

	if err := h.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, paymentStatus, nil, nil, nil); err != nil {
		return fmt.Errorf("update payment dispute status: %w", err)
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
	return nil
}

func (h *StripeWebhookHandler) handleRefundEvent(ctx context.Context, tenantID uuid.UUID, event stripe.Event) error {
	if h.paymentRepo == nil {
		return nil
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
			return fmt.Errorf("decode charge.refunded: %w", err)
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
			return fmt.Errorf("decode refund.updated: %w", err)
		}
		if refund.Status != "" && refund.Status != "succeeded" {
			return nil
		}
		refs = append(refs, stripeExpandableID(refund.PaymentIntent), stripeExpandableID(refund.Charge))
		if refund.Amount > 0 {
			amount = float64(refund.Amount) / 100.0
		}
	}

	p, err := h.paymentRepo.GetByProviderReferences(ctx, tenantID, refs...)
	if err != nil || p == nil {
		return nil
	}
	if err := h.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, "refunded", nil, nil, nil); err != nil {
		return fmt.Errorf("update payment refunded status: %w", err)
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
	return nil
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

func stripeEventBelongsToTenant(raw []byte, tenantID uuid.UUID, tenantSlug string) bool {
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	mdRaw, ok := payload["metadata"]
	if !ok || mdRaw == nil {
		return false
	}
	md, ok := mdRaw.(map[string]interface{})
	if !ok {
		return false
	}
	if tid, _ := md["tenant_id"].(string); strings.TrimSpace(tid) != "" && strings.EqualFold(strings.TrimSpace(tid), tenantID.String()) {
		return true
	}
	if slug, _ := md["tenant_slug"].(string); strings.TrimSpace(slug) != "" && strings.EqualFold(strings.TrimSpace(slug), strings.TrimSpace(tenantSlug)) {
		return true
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
