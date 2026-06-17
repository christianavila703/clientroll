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

	"clientroll/internal/accounting/xerosync"
	"clientroll/internal/database"
	"clientroll/internal/email"
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
	inviteRepo  *repository.MembershipSetupInviteRepo
	dispatcher  *events.Dispatcher
	xeroSync    *xerosync.Service
	ownerAlerts *observability.OwnerAlertService

	signupWebhookSvc *events.SignupWebhookService
	appURL           string

	clientRepo        *repository.ClientRepo
	mtRepo            *repository.MembershipTypeRepo
	emailSvc          email.Provider
	emailTemplateRepo *repository.TenantEmailTemplateRepo
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
	inviteRepo *repository.MembershipSetupInviteRepo,
	xeroSync *xerosync.Service,
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
		inviteRepo:  inviteRepo,
		xeroSync:    xeroSync,
	}
}

// SetDispatcher sets optional outbound webhook event dispatcher.
func (h *StripeWebhookHandler) SetDispatcher(dispatcher *events.Dispatcher) {
	h.dispatcher = dispatcher
}

// SetSignupWebhookService wires signup completion webhooks for Stripe checkout flows.
func (h *StripeWebhookHandler) SetSignupWebhookService(svc *events.SignupWebhookService, appURL string) {
	h.signupWebhookSvc = svc
	h.appURL = strings.TrimSpace(appURL)
}

func (h *StripeWebhookHandler) notifySignupCompleted(ctx context.Context, tenantID, membershipID uuid.UUID) {
	if h.signupWebhookSvc == nil {
		return
	}
	appURL := h.appURL
	if appURL == "" {
		appURL = "https://app.clientroll.com"
	}
	h.signupWebhookSvc.NotifyCompleted(ctx, tenantID, membershipID, appURL)
}

// SetOwnerAlerts sets optional owner alert service for immediate payment failure emails.
func (h *StripeWebhookHandler) SetOwnerAlerts(oa *observability.OwnerAlertService) {
	h.ownerAlerts = oa
}

// SetEmailDeps injects dependencies needed for client-facing payment emails.
func (h *StripeWebhookHandler) SetEmailDeps(clientRepo *repository.ClientRepo, mtRepo *repository.MembershipTypeRepo, emailSvc email.Provider, emailTemplateRepo *repository.TenantEmailTemplateRepo) {
	h.clientRepo = clientRepo
	h.mtRepo = mtRepo
	h.emailSvc = emailSvc
	h.emailTemplateRepo = emailTemplateRepo
}

// linkAudit records which payment/membership a webhook event affected.
func (h *StripeWebhookHandler) linkAudit(ctx context.Context, tenantID uuid.UUID, eventID string, paymentID *uuid.UUID, membershipID *uuid.UUID, expectedOutcome string) {
	if h.eventRepo == nil || eventID == "" {
		return
	}
	h.eventRepo.LinkPayment(ctx, tenantID, "stripe", eventID, paymentID, membershipID, expectedOutcome)
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
		log.Printf("ALERT stripe_webhook_dropped tenant=%s slug=%s reason=inactive_or_missing_secret", tenant.ID, tenantSlug)
		if h.ownerAlerts != nil {
			h.ownerAlerts.SendImmediate(r.Context(), tenant.ID, observability.AlertWebhookDropped, observability.PaymentAlertData{
				FailureReason: fmt.Sprintf("Stripe webhook received for tenant %s but integration is inactive or webhook secret is missing. Events are being dropped.", tenantSlug),
			})
		}
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

// markSignupInviteCompletedFromMetadata sets completed_at on a membership setup invite when Stripe
// checkout or invoice metadata includes signup_invite_id (idempotent).
func (h *StripeWebhookHandler) markSignupInviteCompletedFromMetadata(ctx context.Context, metadata map[string]string) {
	if h.inviteRepo == nil || len(metadata) == 0 {
		return
	}
	raw := strings.TrimSpace(metadata["signup_invite_id"])
	if raw == "" {
		return
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return
	}
	if err := h.inviteRepo.MarkCompleted(ctx, id); err != nil {
		log.Printf("stripe signup_invite mark completed invite=%s err=%v", id, err)
	}
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

		clientID, err := uuid.Parse(strings.TrimSpace(metadata["client_id"]))
		if err != nil {
			return fmt.Errorf("parse client_id from service_charge metadata: %w", err)
		}

		var serviceID *uuid.UUID
		if sid := strings.TrimSpace(metadata["service_id"]); sid != "" {
			parsed, err := uuid.Parse(sid)
			if err == nil {
				serviceID = &parsed
			}
		}

		description := metadata["description"]
		amount, _ := strconv.ParseFloat(metadata["amount"], 64)
		taxAmount, _ := strconv.ParseFloat(metadata["tax_amount"], 64)
		feeAmount, _ := strconv.ParseFloat(metadata["fee_amount"], 64)
		var notes *string
		if n := metadata["notes"]; n != "" {
			notes = &n
		}
		var createdBy *uuid.UUID
		if cb := strings.TrimSpace(metadata["created_by"]); cb != "" {
			if parsed, err := uuid.Parse(cb); err == nil {
				createdBy = &parsed
			}
		}

		paymentMethod := strings.TrimSpace(metadata["payment_method"])
		if paymentMethod == "" {
			paymentMethod = "card"
		}

		// Determine initial status: "paid" means card settled immediately, otherwise pending (e.g. direct debit processing).
		status := "pending"
		var paidDate *time.Time
		if strings.EqualFold(string(sess.PaymentStatus), "paid") {
			status = "succeeded"
			now := time.Now().UTC()
			paidDate = &now
		}

		// If checkout had a pre-created charge (legacy), look it up first.
		var charge *repository.ServiceCharge
		if cid := strings.TrimSpace(metadata["charge_id"]); cid != "" {
			if parsed, err := uuid.Parse(cid); err == nil {
				charge, _ = h.serviceRepo.FindChargeByID(ctx, tenantID, parsed)
			}
		}

		if charge == nil {
			var createErr error
			if status == "succeeded" {
				charge, createErr = h.serviceRepo.CreateCharge(ctx, tenantID, clientID, serviceID, description, amount, taxAmount, notes, createdBy)
			} else {
				charge, createErr = h.serviceRepo.CreatePendingCharge(ctx, tenantID, clientID, serviceID, description, amount, taxAmount, feeAmount, notes, createdBy)
			}
			if createErr != nil {
				return fmt.Errorf("create service charge from checkout webhook: %w", createErr)
			}
		} else if status == "succeeded" {
			now := time.Now().UTC()
			_ = h.serviceRepo.UpdateChargeStatus(ctx, tenantID, charge.ID, "succeeded", &now)
		}

		providerRef := sess.ID
		_ = h.serviceRepo.UpdateChargeProviderContext(ctx, tenantID, charge.ID, &providerRef, &paymentMethod)
		_ = h.mergeStripeDetailsForServiceCheckout(ctx, tenantID, charge.ID, &sess, event.Created)

		if status == "succeeded" {
			if h.dispatcher != nil {
				h.dispatcher.Dispatch(ctx, events.NewEvent(events.ServiceChargeSucceeded, tenantID, map[string]interface{}{
					"charge_id": charge.ID.String(),
					"status":    "succeeded",
					"source":    "stripe_webhook",
				}))
			}
			if h.xeroSync != nil {
				if refreshed, ferr := h.serviceRepo.FindChargeByID(ctx, tenantID, charge.ID); ferr == nil && refreshed != nil {
					_, _ = h.xeroSync.EnsureInvoiceForServiceCharge(ctx, refreshed)
					_ = h.xeroSync.ApplyPaymentForServiceCharge(ctx, refreshed)
				}
			}
		} else if h.xeroSync != nil {
			if refreshed, ferr := h.serviceRepo.FindChargeByID(ctx, tenantID, charge.ID); ferr == nil && refreshed != nil {
				_, _ = h.xeroSync.EnsureInvoiceForServiceCharge(ctx, refreshed)
			}
		}

		h.sendServiceChargeEmails(ctx, tenantID, charge, status, "")

		_ = paidDate
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
					} else {
						h.markSignupInviteCompletedFromMetadata(ctx, metadata)
						h.notifySignupCompleted(ctx, tenantID, mid)
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
	p.Status = "succeeded"
	p.PaidDate = &now
	h.linkAudit(ctx, tenantID, event.ID, &p.ID, &membershipID, "succeeded")
	if h.xeroSync != nil {
		_, _ = h.xeroSync.EnsureInvoiceForPayment(ctx, p)
		_ = h.xeroSync.ApplyPaymentForPayment(ctx, p)
	}
	if err := syncMembershipStatusAfterPayment(ctx, h.memberRepo, tenantID, membershipID, "succeeded", now); err != nil {
		log.Printf("stripe membership status sync failed tenant=%s membership=%s err=%v", tenantID, membershipID, err)
	} else {
		h.markSignupInviteCompletedFromMetadata(ctx, metadata)
		h.notifySignupCompleted(ctx, tenantID, membershipID)
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
		// No charge record was created for checkout-based service charges, so nothing to clean up.
		log.Printf("stripe service_charge checkout expired/cancelled tenant=%s session=%s", tenantID, sess.ID)
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
	h.linkAudit(ctx, tenantID, event.ID, &p.ID, &membershipID, "failed")
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
		p.Status = "succeeded"
		p.PaidDate = &now
		h.linkAudit(ctx, tenantID, event.ID, &p.ID, &membershipID, "succeeded")
		if h.xeroSync != nil {
			_, _ = h.xeroSync.EnsureInvoiceForPayment(ctx, p)
			_ = h.xeroSync.ApplyPaymentForPayment(ctx, p)
		}
		if err := syncMembershipStatusAfterPayment(ctx, h.memberRepo, tenantID, membershipID, "succeeded", now); err != nil {
			log.Printf("stripe membership status sync failed tenant=%s membership=%s err=%v", tenantID, membershipID, err)
		} else {
			h.markSignupInviteCompletedFromMetadata(ctx, metadata)
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
		h.sendPaymentEmail(ctx, tenantID, p, "succeeded", "")
		return nil
	}
	reason := "invoice payment failed"
	if err := h.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, "failed", nil, &reason, nil); err != nil {
		return fmt.Errorf("update invoice payment failed status: %w", err)
	}
	h.linkAudit(ctx, tenantID, event.ID, &p.ID, &membershipID, "failed")
	h.logPaymentFailure(ctx, tenantID, p.ID, nil, &reason, nil)
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
	if h.ownerAlerts != nil {
		clientName, membershipTitle := h.resolveClientAndMembership(ctx, tenantID, clientID, membershipID)
		h.ownerAlerts.SendImmediate(ctx, tenantID, observability.AlertPaymentFailed, observability.PaymentAlertData{
			ClientName:      clientName,
			MembershipTitle: membershipTitle,
			Amount:          amount,
			CurrencyCode:    currencyCode,
			FailureReason:   reason,
			Provider:        "Stripe",
		})
	}
	h.sendPaymentEmail(ctx, tenantID, p, "failed", reason)
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
	if h.xeroSync != nil {
		_, _ = h.xeroSync.EnsureInvoiceForPayment(ctx, p)
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
	h.linkAudit(ctx, tenantID, event.ID, nil, &membershipID, "cancelled")
	if h.xeroSync != nil && h.paymentRepo != nil {
		if openPayment, pErr := h.paymentRepo.FindLatestOpenByMembership(ctx, tenantID, membershipID); pErr == nil && openPayment != nil {
			_ = h.xeroSync.VoidInvoiceForPaymentIfUnpaid(ctx, openPayment, "Membership cancelled")
		}
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
	h.linkAudit(ctx, tenantID, event.ID, &p.ID, &p.MembershipID, paymentStatus)
	p.Status = paymentStatus
	p.PaidDate = nil
	if h.xeroSync != nil && paymentStatus == "disputed" {
		_ = h.xeroSync.CreateCreditNoteForMembershipPayment(ctx, p, amount, "Payment disputed")
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
	if h.ownerAlerts != nil && paymentStatus == "disputed" {
		clientName, membershipTitle := h.resolveClientAndMembership(ctx, tenantID, p.ClientID, p.MembershipID)
		h.ownerAlerts.SendImmediate(ctx, tenantID, observability.AlertDispute, observability.PaymentAlertData{
			ClientName:      clientName,
			MembershipTitle: membershipTitle,
			Amount:          amount,
			CurrencyCode:    p.CurrencyCode,
			DisputeReason:   dispute.Reason,
			Provider:        "Stripe",
		})
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
	h.linkAudit(ctx, tenantID, event.ID, &p.ID, &p.MembershipID, "refunded")
	p.Status = "refunded"
	p.PaidDate = nil
	if amount <= 0 {
		amount = p.Amount
	}
	if h.xeroSync != nil {
		_ = h.xeroSync.CreateCreditNoteForMembershipPayment(ctx, p, amount, "Payment refunded")
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

// resolveClientAndMembership fetches client name and membership title for alert emails.
func (h *StripeWebhookHandler) resolveClientAndMembership(ctx context.Context, tenantID, clientID, membershipID uuid.UUID) (string, string) {
	clientName := "Unknown"
	membershipTitle := "Membership"
	if clientID == uuid.Nil || tenantID == uuid.Nil {
		return clientName, membershipTitle
	}
	var name string
	if err := h.db.Pool.QueryRow(ctx, `SELECT name FROM clients WHERE id = $1 AND tenant_id = $2`, clientID, tenantID).Scan(&name); err == nil {
		clientName = name
	}
	if membershipID != uuid.Nil {
		var title string
		if err := h.db.Pool.QueryRow(ctx, `
			SELECT mt.title FROM memberships m
			JOIN membership_types mt ON mt.id = m.membership_type_id AND mt.tenant_id = m.tenant_id
			WHERE m.id = $1 AND m.tenant_id = $2
		`, membershipID, tenantID).Scan(&title); err == nil && strings.TrimSpace(title) != "" {
			membershipTitle = title
		}
	}
	return clientName, membershipTitle
}

// logPaymentFailure writes to the payment_failure_log table for audit history.
func (h *StripeWebhookHandler) logPaymentFailure(ctx context.Context, tenantID, paymentID uuid.UUID, failureCode, failureReason, providerDetail *string) {
	_, err := h.db.Pool.Exec(ctx, `
		INSERT INTO payment_failure_log (tenant_id, payment_id, failure_code, failure_reason, provider_detail)
		VALUES ($1, $2, $3, $4, $5)
	`, tenantID, paymentID, failureCode, failureReason, providerDetail)
	if err != nil {
		log.Printf("payment_failure_log insert failed tenant=%s payment=%s err=%v", tenantID, paymentID, err)
	}
}

// sendPaymentEmail sends payment success/failure emails to the client for Stripe payments.
func (h *StripeWebhookHandler) sendPaymentEmail(ctx context.Context, tenantID uuid.UUID, p *repository.Payment, status, failureReason string) {
	if h.emailSvc == nil || h.clientRepo == nil || p == nil {
		return
	}
	client, err := h.clientRepo.FindByID(ctx, p.ClientID, tenantID)
	if err != nil || client == nil || client.Email == nil || *client.Email == "" {
		return
	}
	tenantName := ""
	if h.tenantRepo != nil {
		if tenant, tErr := h.tenantRepo.FindByID(ctx, tenantID); tErr == nil && tenant != nil {
			tenantName = tenant.Name
		}
	}
	membershipTitle := "Membership"
	if h.memberRepo != nil && h.mtRepo != nil {
		if membership, mErr := h.memberRepo.FindByID(ctx, p.MembershipID, tenantID); mErr == nil && membership != nil {
			if mt, mtErr := h.mtRepo.FindByID(ctx, membership.MembershipTypeID, tenantID); mtErr == nil && mt != nil && strings.TrimSpace(mt.Title) != "" {
				membershipTitle = mt.Title
			}
		}
	}
	currencyCode := strings.ToUpper(strings.TrimSpace(p.CurrencyCode))
	if currencyCode == "" {
		currencyCode = "AUD"
	}
	reason := strings.TrimSpace(failureReason)
	if reason == "" && p.FailureReason != nil {
		reason = strings.TrimSpace(*p.FailureReason)
	}
	if reason == "" {
		reason = "payment could not be processed"
	}
	money := func(v float64) string { return fmt.Sprintf("%.2f", v) }
	vars := map[string]string{
		"admin_name":             "",
		"client_name":            client.Name,
		"amount":                 money(p.Amount),
		"total_charged":          money(p.Amount),
		"amount_due":             money(p.Amount),
		"base_amount":            money(p.BaseAmount),
		"tax_amount":             money(p.TaxAmount),
		"processing_fee_amount":  money(p.ProcessingFeeAmount),
		"to_organisation_amount": money(p.BaseAmount + p.TaxAmount),
		"currency_code":          currencyCode,
		"membership_title":       membershipTitle,
		"payment_date":           time.Now().Format("2 January 2006"),
		"failure_reason":         reason,
		"company_name":           tenantName,
	}

	go func() {
		var templateKey string
		switch status {
		case "succeeded":
			templateKey = "payment_receipt"
		case "failed":
			templateKey = "payment_failed"
		default:
			return
		}
		input, ok := h.buildOperationalTemplateEmail(context.Background(), tenantID, templateKey, vars)
		if !ok {
			return
		}
		input.To = *client.Email
		if _, err := h.emailSvc.Send(context.Background(), *input); err != nil {
			log.Printf("stripe.%s email send failed to=%s err=%v", templateKey, *client.Email, err)
		} else {
			log.Printf("stripe.%s email sent to=%s", templateKey, *client.Email)
		}
	}()
}

func (h *StripeWebhookHandler) buildOperationalTemplateEmail(ctx context.Context, tenantID uuid.UUID, templateKey string, vars map[string]string) (*email.SendInput, bool) {
	def, ok := email.OperationalTemplateByKey(templateKey)
	if !ok {
		return nil, false
	}
	enabled := def.DefaultEnabled
	subject := def.Subject
	body := def.Body
	if h.emailTemplateRepo != nil {
		override, err := h.emailTemplateRepo.GetByKey(ctx, tenantID, templateKey)
		if err != nil {
			log.Printf("stripe.email template lookup failed tenant=%s template=%s err=%v", tenantID, templateKey, err)
		} else if override != nil {
			enabled = override.Enabled
			if override.SubjectOverride != nil && strings.TrimSpace(*override.SubjectOverride) != "" {
				subject = *override.SubjectOverride
			}
			if override.BodyOverride != nil && strings.TrimSpace(*override.BodyOverride) != "" {
				body = *override.BodyOverride
			}
		}
	}
	if !enabled {
		return nil, false
	}
	render := func(input string) string {
		output := input
		for k, v := range vars {
			output = strings.ReplaceAll(output, "{{"+k+"}}", v)
		}
		return output
	}
	renderedSubject := render(subject)
	renderedBody := render(body)
	emailInput := &email.SendInput{
		Subject: renderedSubject,
		HTML:    email.WrapHTML(renderedBody),
		Text:    renderedSubject,
	}
	return emailInput, true
}

// sendServiceChargeEmails sends receipt/failure emails to the client and staff alert emails for service charges.
func (h *StripeWebhookHandler) sendServiceChargeEmails(ctx context.Context, tenantID uuid.UUID, charge *repository.ServiceCharge, status, failureReason string) {
	if h.emailSvc == nil || charge == nil {
		return
	}

	client, err := h.clientRepo.FindByID(ctx, charge.ClientID, tenantID)
	if err != nil || client == nil {
		return
	}
	clientEmail := ""
	if client.Email != nil {
		clientEmail = strings.TrimSpace(*client.Email)
	}

	tenantName := ""
	if h.tenantRepo != nil {
		if tenant, tErr := h.tenantRepo.FindByID(ctx, tenantID); tErr == nil && tenant != nil {
			tenantName = tenant.Name
		}
	}

	currencyCode := strings.ToUpper(strings.TrimSpace(charge.CurrencyCode))
	if currencyCode == "" {
		currencyCode = "AUD"
	}
	reason := strings.TrimSpace(failureReason)
	if reason == "" {
		reason = "payment could not be processed"
	}
	money := func(v float64) string { return fmt.Sprintf("%.2f", v) }
	total := charge.Amount + charge.TaxAmount + charge.ProcessingFeeAmount

	vars := map[string]string{
		"admin_name":            "",
		"client_name":           client.Name,
		"description":           charge.Description,
		"base_amount":           money(charge.Amount),
		"tax_amount":            money(charge.TaxAmount),
		"processing_fee_amount": money(charge.ProcessingFeeAmount),
		"total_charged":         money(total),
		"currency_code":         currencyCode,
		"payment_date":          time.Now().Format("2 January 2006"),
		"failure_reason":        reason,
		"company_name":          tenantName,
	}

	go func() {
		bgCtx := context.Background()

		// Client email
		if clientEmail != "" {
			var clientTemplate string
			switch status {
			case "succeeded":
				clientTemplate = "service_charge_receipt"
			case "failed":
				clientTemplate = "service_charge_failed"
			}
			if clientTemplate != "" {
				if input, ok := h.buildOperationalTemplateEmail(bgCtx, tenantID, clientTemplate, vars); ok {
					input.To = clientEmail
					if _, err := h.emailSvc.Send(bgCtx, *input); err != nil {
						log.Printf("stripe.%s email send failed to=%s err=%v", clientTemplate, clientEmail, err)
					} else {
						log.Printf("stripe.%s email sent to=%s", clientTemplate, clientEmail)
					}
				}
			}
		}

		// Staff alert email
		var staffTemplate string
		switch status {
		case "succeeded":
			staffTemplate = "service_charge_receipt_alert"
		case "failed":
			staffTemplate = "service_charge_failed_alert"
		}
		if staffTemplate != "" {
			if input, ok := h.buildOperationalTemplateEmail(bgCtx, tenantID, staffTemplate, vars); ok {
				if h.ownerAlerts != nil {
					h.ownerAlerts.SendRawEmail(bgCtx, tenantID, input.Subject, input.HTML)
				}
			}
		}
	}()
}
