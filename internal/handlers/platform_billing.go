package handlers

import (
	"log"
	"net/http"

	"clientroll/internal/database"
	"clientroll/internal/models"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v82"
	billingPortal "github.com/stripe/stripe-go/v82/billingportal/session"
)

// PlatformBillingHandler handles platform-level Stripe billing configuration and tenant billing visibility.
type PlatformBillingHandler struct {
	appURL string

	enc *database.Encryptor

	cfgRepo    *repository.PlatformBillingConfigRepo
	tenantRepo *repository.TenantRepo
	subRepo    *repository.TenantSubscriptionRepo
	usageRepo  *repository.TenantUsageRepo
	invRepo    *repository.TenantInvoiceRepo
}

func NewPlatformBillingHandler(
	appURL string,
	enc *database.Encryptor,
	cfgRepo *repository.PlatformBillingConfigRepo,
	tenantRepo *repository.TenantRepo,
	subRepo *repository.TenantSubscriptionRepo,
	usageRepo *repository.TenantUsageRepo,
	invRepo *repository.TenantInvoiceRepo,
) *PlatformBillingHandler {
	return &PlatformBillingHandler{
		appURL:     appURL,
		enc:        enc,
		cfgRepo:    cfgRepo,
		tenantRepo: tenantRepo,
		subRepo:    subRepo,
		usageRepo:  usageRepo,
		invRepo:    invRepo,
	}
}

func (h *PlatformBillingHandler) RegisterRoutes(r chi.Router) {
	r.Get("/billing/config", h.GetConfig)
	r.Put("/billing/config", h.UpdateConfig)

	r.Get("/billing/tenants", h.ListTenantBilling)
	r.Get("/billing/tenants/{tenantID}", h.GetTenantBilling)
	r.Post("/billing/tenants/{tenantID}/portal", h.CreatePortalSession)
}

func (h *PlatformBillingHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.cfgRepo.GetOrCreate(r.Context())
	if err != nil {
		log.Printf("platform billing: get config error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to load billing config")
		return
	}

	webhookURL := h.appURL + "/api/webhooks/stripe/platform-billing"
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"config":      cfg.ToPublic(),
		"webhook_url": webhookURL,
		"webhook_events": []string{
			"customer.subscription.created",
			"customer.subscription.updated",
			"customer.subscription.deleted",
			"invoice.created",
			"invoice.finalized",
			"invoice.paid",
			"invoice.payment_failed",
			"invoice.voided",
			"payment_method.attached",
			"payment_method.updated",
			"customer.updated",
		},
	})
}

func (h *PlatformBillingHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.cfgRepo.GetOrCreate(r.Context())
	if err != nil {
		log.Printf("platform billing: get config error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to load billing config")
		return
	}

	var input struct {
		PerMemberPriceUSD    *float64 `json:"per_member_price_usd,omitempty"`
		Currency             *string  `json:"currency,omitempty"`
		IsActive             *bool    `json:"is_active,omitempty"`
		GooglePlacesIsActive *bool    `json:"google_places_is_active,omitempty"`

		StripeProductID *string `json:"stripe_product_id,omitempty"`
		StripePriceID   *string `json:"stripe_price_id,omitempty"`

		StripePublishableKey string `json:"stripe_publishable_key,omitempty"`
		StripeSecretKey      string `json:"stripe_secret_key,omitempty"`
		StripeWebhookSecret  string `json:"stripe_webhook_secret,omitempty"`
		GooglePlacesAPIKey   string `json:"google_places_api_key,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	up := repository.PlatformBillingConfigUpdateInput{
		PerMemberPriceUSD:    input.PerMemberPriceUSD,
		Currency:             input.Currency,
		IsActive:             input.IsActive,
		GooglePlacesIsActive: input.GooglePlacesIsActive,
		StripeProductID:      input.StripeProductID,
		StripePriceID:        input.StripePriceID,
	}

	if input.StripePublishableKey != "" {
		encVal, err := h.enc.Encrypt(input.StripePublishableKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Encryption error")
			return
		}
		up.StripePublishableKeyEncrypted = &encVal
	}
	if input.StripeSecretKey != "" {
		encVal, err := h.enc.Encrypt(input.StripeSecretKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Encryption error")
			return
		}
		up.StripeSecretKeyEncrypted = &encVal
	}
	if input.StripeWebhookSecret != "" {
		encVal, err := h.enc.Encrypt(input.StripeWebhookSecret)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Encryption error")
			return
		}
		up.StripeWebhookSecretEncrypted = &encVal
	}
	if input.GooglePlacesAPIKey != "" {
		encVal, err := h.enc.Encrypt(input.GooglePlacesAPIKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Encryption error")
			return
		}
		up.GooglePlacesAPIKeyEncrypted = &encVal
	}

	updated, err := h.cfgRepo.Update(r.Context(), cfg.ID, up)
	if err != nil {
		log.Printf("platform billing: update config error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update billing config")
		return
	}
	if updated == nil {
		writeError(w, http.StatusNotFound, "Billing config not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"config": updated.ToPublic(),
	})
}

func (h *PlatformBillingHandler) ListTenantBilling(w http.ResponseWriter, r *http.Request) {
	tenants, err := h.tenantRepo.List(r.Context())
	if err != nil {
		log.Printf("platform billing: list tenants error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list tenants")
		return
	}

	type TenantBillingRow struct {
		Tenant       models.Tenant                  `json:"tenant"`
		Subscription *repository.TenantSubscription `json:"subscription,omitempty"`
	}

	rows := make([]TenantBillingRow, 0, len(tenants))
	for _, t := range tenants {
		sub, err := h.subRepo.GetByTenant(r.Context(), t.ID)
		if err != nil {
			// Don’t fail the whole list; show tenant and omit subscription.
			log.Printf("platform billing: get subscription error tenant=%s: %v", t.ID, err)
			sub = nil
		}
		rows = append(rows, TenantBillingRow{
			Tenant:       t,
			Subscription: sub,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenants": rows,
		"count":   len(rows),
	})
}

func (h *PlatformBillingHandler) GetTenantBilling(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	tenant, err := h.tenantRepo.FindByID(r.Context(), tenantID)
	if err != nil {
		log.Printf("platform billing: get tenant error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch tenant")
		return
	}
	if tenant == nil {
		writeError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	sub, err := h.subRepo.GetByTenant(r.Context(), tenantID)
	if err != nil {
		log.Printf("platform billing: get subscription error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch subscription")
		return
	}

	days := 90
	usage, err := h.usageRepo.ListByTenant(r.Context(), tenantID, days)
	if err != nil {
		log.Printf("platform billing: get usage error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch usage")
		return
	}

	invoices, err := h.invRepo.ListByTenant(r.Context(), tenantID, 50)
	if err != nil {
		log.Printf("platform billing: get invoices error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch invoices")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenant":       tenant,
		"subscription": sub,
		"usage":        usage,
		"invoices":     invoices,
	})
}

func (h *PlatformBillingHandler) CreatePortalSession(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	cfg, err := h.cfgRepo.GetOrCreate(r.Context())
	if err != nil {
		log.Printf("platform billing: get config error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to load billing config")
		return
	}
	if cfg.StripeSecretKeyEncrypted == nil || *cfg.StripeSecretKeyEncrypted == "" {
		writeError(w, http.StatusBadRequest, "Stripe secret key is not configured")
		return
	}

	secretKey, err := h.enc.Decrypt(*cfg.StripeSecretKeyEncrypted)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to decrypt Stripe secret key")
		return
	}

	sub, err := h.subRepo.GetByTenant(r.Context(), tenantID)
	if err != nil {
		log.Printf("platform billing: get subscription error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch subscription")
		return
	}
	if sub == nil || sub.StripeCustomerID == nil || *sub.StripeCustomerID == "" {
		writeError(w, http.StatusBadRequest, "Tenant does not have a Stripe customer yet")
		return
	}

	var body struct {
		ReturnURL string `json:"return_url"`
	}
	_ = decodeJSON(r, &body) // optional
	returnURL := body.ReturnURL
	if returnURL == "" {
		returnURL = h.appURL + "/platform/billing"
	}

	stripe.Key = secretKey
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(*sub.StripeCustomerID),
		ReturnURL: stripe.String(returnURL),
	}
	params.Context = r.Context()

	sess, err := billingPortal.New(params)
	if err != nil {
		log.Printf("platform billing: create portal session error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create portal session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"url": sess.URL,
		// Billing Portal sessions do not expose an expires_at field in stripe-go.
	})
}
