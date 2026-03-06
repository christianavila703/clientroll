package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
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
	encryptor   *database.Encryptor
	paymentRepo *repository.PaymentRepo
	dispatcher  *events.Dispatcher
}

// NewStripeWebhookHandler creates a Stripe webhook handler.
func NewStripeWebhookHandler(
	db *database.Postgres,
	tr *repository.TenantRepo,
	credRepo *repository.TenantCredentialRepo,
	encryptor *database.Encryptor,
	paymentRepo *repository.PaymentRepo,
) *StripeWebhookHandler {
	return &StripeWebhookHandler{
		db:          db,
		tenantRepo:  tr,
		credRepo:    credRepo,
		encryptor:   encryptor,
		paymentRepo: paymentRepo,
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
	if err != nil || creds == nil || creds.APISecretEncrypted == "" {
		writeError(w, http.StatusServiceUnavailable, "Stripe webhook not configured")
		return
	}
	secret, err := h.encryptor.Decrypt(creds.APISecretEncrypted)
	if err != nil || secret == "" {
		writeError(w, http.StatusUnauthorized, "Invalid Stripe webhook secret")
		return
	}

	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), secret)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid Stripe signature")
		return
	}

	if err := h.db.SetTenantContext(r.Context(), tenant.ID.String()); err != nil {
		writeError(w, http.StatusInternalServerError, "Tenant context error")
		return
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
	}
	w.WriteHeader(http.StatusOK)
}

func (h *StripeWebhookHandler) handleCheckoutCompleted(ctx context.Context, tenantID uuid.UUID, event stripe.Event) {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		return
	}
	metadata := sess.Metadata
	membershipID, err1 := uuid.Parse(metadata["membership_id"])
	clientID, err2 := uuid.Parse(metadata["client_id"])
	amount, err3 := strconv.ParseFloat(metadata["payment_amount"], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return
	}
	providerRef := sess.ID
	if sess.PaymentIntent != nil && sess.PaymentIntent.ID != "" {
		providerRef = sess.PaymentIntent.ID
	}
	p, _, err := h.paymentRepo.CreateIfNotExists(ctx, tenantID, membershipID, clientID, amount, time.Now().UTC(), providerRef)
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
	membershipID, err1 := uuid.Parse(metadata["membership_id"])
	clientID, err2 := uuid.Parse(metadata["client_id"])
	amount, err3 := strconv.ParseFloat(metadata["payment_amount"], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return
	}
	p, _, err := h.paymentRepo.CreateIfNotExists(ctx, tenantID, membershipID, clientID, amount, time.Now().UTC(), sess.ID)
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
	p, _, err := h.paymentRepo.CreateIfNotExists(ctx, tenantID, membershipID, clientID, amount, time.Now().UTC(), providerRef)
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
