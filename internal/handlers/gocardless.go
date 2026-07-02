package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"clientroll/internal/accounting/xerosync"
	"clientroll/internal/config"
	"clientroll/internal/database"
	"clientroll/internal/email"
	"clientroll/internal/events"
	"clientroll/internal/middleware"
	"clientroll/internal/observability"
	"clientroll/internal/payment"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	gocardlessProvider  = "gocardless"
	connectSandboxAuth  = "https://connect-sandbox.gocardless.com/oauth/authorize"
	connectSandboxToken = "https://connect-sandbox.gocardless.com/oauth/access_token"
	connectLiveAuth     = "https://connect.gocardless.com/oauth/authorize"
	connectLiveToken    = "https://connect.gocardless.com/oauth/access_token"
)

// writeProviderError writes an error response; if err is a GoCardless API error (GCError),
// it uses that status code and message so clients get correct 4xx (e.g. validation, not found).
func writeProviderError(w http.ResponseWriter, err error) {
	var gce *payment.GCError
	if errors.As(err, &gce) {
		status := gce.StatusCode
		if status < 400 || status >= 600 {
			status = http.StatusInternalServerError
		}
		msg := gce.Message
		if msg == "" {
			msg = gce.Raw
		}
		if msg == "" {
			msg = err.Error()
		}
		writeError(w, status, msg)
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

// GoCardlessHandler handles GoCardless Connect OAuth, webhooks, and payment creation.
type GoCardlessHandler struct {
	cfg               *config.Config
	tenantRepo        *repository.TenantRepo
	credRepo          *repository.TenantCredentialRepo
	eventRepo         *repository.InboundWebhookEventRepo
	paymentRepo       *repository.PaymentRepo
	disputeRepo       *repository.PaymentDisputeRepo
	memberRepo        *repository.MembershipRepo
	mtRepo            *repository.MembershipTypeRepo
	clientRepo        *repository.ClientRepo
	emailTemplateRepo *repository.TenantEmailTemplateRepo
	encryptor         *database.Encryptor
	emailSvc          email.Provider
	dispatcher        *events.Dispatcher
	xeroSync          *xerosync.Service
	ownerAlerts       *observability.OwnerAlertService
}

// NewGoCardlessHandler creates a new GoCardless handler.
func NewGoCardlessHandler(cfg *config.Config, tenantRepo *repository.TenantRepo, cr *repository.TenantCredentialRepo, eventRepo *repository.InboundWebhookEventRepo, paymentRepo *repository.PaymentRepo, disputeRepo *repository.PaymentDisputeRepo, memberRepo *repository.MembershipRepo, mtRepo *repository.MembershipTypeRepo, clientRepo *repository.ClientRepo, etr *repository.TenantEmailTemplateRepo, enc *database.Encryptor, es email.Provider, xeroSync *xerosync.Service) *GoCardlessHandler {
	return &GoCardlessHandler{
		cfg:               cfg,
		tenantRepo:        tenantRepo,
		credRepo:          cr,
		eventRepo:         eventRepo,
		paymentRepo:       paymentRepo,
		disputeRepo:       disputeRepo,
		memberRepo:        memberRepo,
		mtRepo:            mtRepo,
		clientRepo:        clientRepo,
		emailTemplateRepo: etr,
		encryptor:         enc,
		emailSvc:          es,
		xeroSync:          xeroSync,
	}
}

func (h *GoCardlessHandler) tenantTaxPercent(ctx context.Context, tenantID uuid.UUID) float64 {
	if h.tenantRepo == nil {
		return 10
	}
	tenant, err := h.tenantRepo.FindByID(ctx, tenantID)
	if err != nil || tenant == nil {
		return 10
	}
	return tenantSalesTaxPercent(tenant)
}

func (h *GoCardlessHandler) membershipAppliesSalesTax(ctx context.Context, tenantID uuid.UUID, membershipTypeID uuid.UUID) bool {
	if h.memberRepo == nil {
		return true
	}
	applySalesTax, err := h.memberRepo.MembershipTypeApplySalesTax(ctx, membershipTypeID, tenantID)
	if err != nil {
		log.Printf("gocardless.tax_toggle lookup failed tenant=%s membership_type=%s err=%v", tenantID, membershipTypeID, err)
		return true
	}
	return applySalesTax
}

// SetDispatcher sets optional outbound webhook event dispatcher.
func (h *GoCardlessHandler) SetDispatcher(dispatcher *events.Dispatcher) {
	h.dispatcher = dispatcher
}

// SetOwnerAlerts sets optional owner alert service for immediate payment failure emails.
func (h *GoCardlessHandler) SetOwnerAlerts(oa *observability.OwnerAlertService) {
	h.ownerAlerts = oa
}

// linkGCAudit records which payment/membership a GoCardless webhook event affected.
func (h *GoCardlessHandler) linkGCAudit(ctx context.Context, tenantID uuid.UUID, eventID string, paymentID *uuid.UUID, membershipID *uuid.UUID, expectedOutcome string) {
	if h.eventRepo == nil || eventID == "" {
		return
	}
	h.eventRepo.LinkPayment(ctx, tenantID, gocardlessProvider, eventID, paymentID, membershipID, expectedOutcome)
}

// RegisterTenantRoutes registers tenant-scoped routes (require auth + tenant).
// Every GoCardless API is exposed here so it can be called from inside Client Roll.
func (h *GoCardlessHandler) RegisterTenantRoutes(r chi.Router) {
	// OAuth connect
	r.Get("/gocardless/connect", h.ConnectURL)

	// Payments
	r.Post("/gocardless/payments", h.CreatePayment)
	r.Get("/gocardless/payments", h.ListPayments) // list must be before {paymentID}
	r.Get("/gocardless/payments/{paymentID}", h.GetPaymentHandler)
	r.Put("/gocardless/payments/{paymentID}", h.UpdatePaymentHandler)
	r.Post("/gocardless/payments/{paymentID}/retry", h.RetryPayment)
	r.Post("/gocardless/payments/{paymentID}/cancel", h.CancelPayment)

	// Refunds
	r.Post("/gocardless/refunds", h.CreateRefund)
	r.Get("/gocardless/refunds", h.ListRefundsHandler) // list before {refundID}
	r.Get("/gocardless/refunds/{refundID}", h.GetRefund)
	r.Put("/gocardless/refunds/{refundID}", h.UpdateRefundHandler)

	// Mandates
	r.Get("/gocardless/mandates", h.ListMandates) // list before {mandateID}
	r.Get("/gocardless/mandates/{mandateID}", h.GetMandate)
	r.Put("/gocardless/mandates/{mandateID}", h.UpdateMandateHandler)
	r.Post("/gocardless/mandates/{mandateID}/reinstate", h.ReinstateMandate)
	r.Post("/gocardless/mandates/{mandateID}/cancel", h.CancelMandateHandler)
	r.Post("/gocardless/mandates/{mandateID}/pdf", h.CreateMandatePDFHandler)

	// Customers
	r.Get("/gocardless/customers", h.ListCustomers) // list before {customerID}
	r.Get("/gocardless/customers/{customerID}", h.GetCustomerHandler)
	r.Put("/gocardless/customers/{customerID}", h.UpdateCustomerHandler)
	r.Delete("/gocardless/customers/{customerID}", h.RemoveCustomerHandler)

	// Customer bank accounts
	r.Post("/gocardless/customer-bank-accounts", h.CreateCustomerBankAccountHandler)
	r.Get("/gocardless/customer-bank-accounts", h.ListCustomerBankAccountsHandler)
	r.Get("/gocardless/customer-bank-accounts/{bankAccountID}", h.GetCustomerBankAccountHandler)
	r.Put("/gocardless/customer-bank-accounts/{bankAccountID}", h.UpdateCustomerBankAccountHandler)
	r.Post("/gocardless/customer-bank-accounts/{bankAccountID}/disable", h.DisableCustomerBankAccountHandler)

	// Redirect flows
	r.Post("/gocardless/redirect-flows", h.CreateRedirectFlow)
	r.Post("/gocardless/redirect-flows/complete", h.CompleteRedirectFlow)
	r.Get("/gocardless/redirect-flows/{flowID}", h.GetRedirectFlowHandler)

	// Subscriptions
	r.Post("/gocardless/subscriptions", h.CreateSubscriptionHandler)
	r.Get("/gocardless/subscriptions", h.ListSubscriptionsHandler)
	r.Get("/gocardless/subscriptions/{subscriptionID}", h.GetSubscriptionHandler)
	r.Put("/gocardless/subscriptions/{subscriptionID}", h.UpdateSubscriptionHandler)
	r.Post("/gocardless/subscriptions/{subscriptionID}/cancel", h.CancelSubscriptionHandler)
	r.Post("/gocardless/subscriptions/{subscriptionID}/pause", h.PauseSubscriptionHandler)
	r.Post("/gocardless/subscriptions/{subscriptionID}/resume", h.ResumeSubscriptionHandler)

	// Bank details lookup
	r.Post("/gocardless/bank-details-lookup", h.ValidateBankDetailsHandler)

	// Instalment schedules
	r.Post("/gocardless/instalment-schedules", h.CreateInstalmentScheduleHandler)
	r.Get("/gocardless/instalment-schedules", h.ListInstalmentSchedulesHandler)
	r.Get("/gocardless/instalment-schedules/{scheduleID}", h.GetInstalmentScheduleHandler)
	r.Put("/gocardless/instalment-schedules/{scheduleID}", h.UpdateInstalmentScheduleHandler)
	r.Post("/gocardless/instalment-schedules/{scheduleID}/cancel", h.CancelInstalmentScheduleHandler)

	// Payouts
	r.Get("/gocardless/payouts", h.ListPayoutsHandler)
	r.Get("/gocardless/payouts/{payoutID}", h.GetPayoutHandler)
	r.Put("/gocardless/payouts/{payoutID}", h.UpdatePayoutHandler)
	r.Get("/gocardless/payouts/{payoutID}/items", h.ListPayoutItemsHandler)

	// Blocks
	r.Post("/gocardless/blocks", h.CreateBlockHandler)
	r.Get("/gocardless/blocks", h.ListBlocksHandler)
	r.Get("/gocardless/blocks/{blockID}", h.GetBlockHandler)
	r.Post("/gocardless/blocks/{blockID}/disable", h.DisableBlockHandler)
	r.Post("/gocardless/blocks/{blockID}/enable", h.EnableBlockHandler)

	// Events
	r.Get("/gocardless/events", h.ListEventsHandler)
	r.Get("/gocardless/events/{eventID}", h.GetEventHandler)

	// Creditors
	r.Get("/gocardless/creditors", h.ListCreditorsHandler)
	r.Get("/gocardless/creditors/{creditorID}", h.GetCreditorHandler)

	// Webhooks management
	r.Get("/gocardless/gc-webhooks", h.ListWebhooksHandler)
	r.Get("/gocardless/gc-webhooks/{webhookID}", h.GetWebhookHandler)
	r.Post("/gocardless/gc-webhooks/{webhookID}/retry", h.RetryWebhookHandler)

	// Scheme identifiers
	r.Get("/gocardless/scheme-identifiers", h.ListSchemeIdentifiersHandler)
	r.Get("/gocardless/scheme-identifiers/{schemeID}", h.GetSchemeIdentifierHandler)

	// Setup direct debit (full mandate creation)
	r.Post("/gocardless/setup-direct-debit", h.SetupDirectDebitHandler)

	// Get payment status
	r.Get("/gocardless/payment-status/{paymentID}", h.GetPaymentStatusHandler)
}

// RegisterPlatformRoutes registers internal platform-admin operational routes.
func (h *GoCardlessHandler) RegisterPlatformRoutes(r chi.Router) {
	r.Get("/webhooks/gocardless/failures", h.ListFailedGoCardlessWebhookEvents)
	r.Post("/webhooks/gocardless/reconcile-payment", h.ReconcileGoCardlessPayment)
}

// RegisterPublicRoutes registers public routes (no auth: OAuth callback, webhook).
func (h *GoCardlessHandler) RegisterPublicRoutes(r chi.Router) {
	r.Get("/gocardless/oauth/callback", h.OAuthCallback)
	r.Post("/webhooks/gocardless", h.Webhook)
}

// ConnectURL returns the GoCardless OAuth authorize URL for the current tenant.
// Uses the tenant's stored Connect app client_id (no env).
func (h *GoCardlessHandler) ConnectURL(w http.ResponseWriter, r *http.Request) {
	tenantIDStr := middleware.GetTenantID(r.Context())
	if tenantIDStr == "" {
		writeError(w, http.StatusForbidden, "Tenant context required")
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		writeError(w, http.StatusForbidden, "Invalid tenant")
		return
	}

	env := strings.ToLower(r.URL.Query().Get("environment"))
	if env != "live" && env != "sandbox" {
		env = "sandbox"
	}

	creds, err := h.credRepo.GetByProvider(r.Context(), tenantID, gocardlessProvider)
	if err != nil || creds == nil {
		writeError(w, http.StatusBadRequest, "Add your GoCardless app credentials first")
		return
	}
	if creds.ConnectClientID == "" {
		writeError(w, http.StatusBadRequest, "Add your GoCardless app credentials first")
		return
	}
	// Use tenant's environment if it matches; otherwise use requested env
	authURL := connectSandboxAuth
	if env == "live" {
		authURL = connectLiveAuth
	}

	redirectURI := strings.TrimSuffix(h.cfg.AppURL, "/") + "/api/gocardless/oauth/callback"
	params := url.Values{
		"client_id":     {creds.ConnectClientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {"read_write"},
		"state":         {tenantID.String() + "|" + env},
	}
	redirectURL := authURL + "?" + params.Encode()
	writeJSON(w, http.StatusOK, map[string]string{"redirect_url": redirectURL})
}

// OAuthCallback handles the redirect from GoCardless after user authorizes.
// Uses the tenant's stored Connect app client_id and client_secret from DB (no env).
func (h *GoCardlessHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		http.Redirect(w, r, redirectToIntegrations(h.cfg.AppURL, "error=missing_params"), http.StatusFound)
		return
	}

	parts := strings.SplitN(state, "|", 2)
	tenantID, err := uuid.Parse(parts[0])
	if err != nil {
		http.Redirect(w, r, redirectToIntegrations(h.cfg.AppURL, "error=invalid_state"), http.StatusFound)
		return
	}
	env := "sandbox"
	if len(parts) == 2 && parts[1] == "live" {
		env = "live"
	}

	creds, err := h.credRepo.GetByProvider(r.Context(), tenantID, gocardlessProvider)
	if err != nil || creds == nil || creds.ConnectClientID == "" || creds.APISecretEncrypted == "" {
		http.Redirect(w, r, redirectToIntegrations(h.cfg.AppURL, "error=not_configured"), http.StatusFound)
		return
	}
	clientSecret, err := h.encryptor.Decrypt(creds.APISecretEncrypted)
	if err != nil {
		log.Printf("GoCardless decrypt client secret error: %v", err)
		http.Redirect(w, r, redirectToIntegrations(h.cfg.AppURL, "error=exchange_failed"), http.StatusFound)
		return
	}
	clientID := creds.ConnectClientID

	tokenURL := connectSandboxToken
	if env == "live" {
		tokenURL = connectLiveToken
	}

	redirectURI := strings.TrimSuffix(h.cfg.AppURL, "/") + "/api/gocardless/oauth/callback"
	body := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}.Encode()

	req, err := http.NewRequestWithContext(r.Context(), "POST", tokenURL, strings.NewReader(body))
	if err != nil {
		log.Printf("GoCardless token request build error: %v", err)
		http.Redirect(w, r, redirectToIntegrations(h.cfg.AppURL, "error=exchange_failed"), http.StatusFound)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("GoCardless token request error: %v", err)
		http.Redirect(w, r, redirectToIntegrations(h.cfg.AppURL, "error=exchange_failed"), http.StatusFound)
		return
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Printf("GoCardless token error %d: %s", resp.StatusCode, string(raw))
		http.Redirect(w, r, redirectToIntegrations(h.cfg.AppURL, "error=exchange_failed"), http.StatusFound)
		return
	}

	var tokenResp struct {
		AccessToken    string `json:"access_token"`
		OrganisationID string `json:"organisation_id"`
	}
	if err := json.Unmarshal(raw, &tokenResp); err != nil {
		log.Printf("GoCardless token parse error: %v", err)
		http.Redirect(w, r, redirectToIntegrations(h.cfg.AppURL, "error=exchange_failed"), http.StatusFound)
		return
	}

	encAccess, err := h.encryptor.Encrypt(tokenResp.AccessToken)
	if err != nil {
		log.Printf("GoCardless encrypt token error: %v", err)
		http.Redirect(w, r, redirectToIntegrations(h.cfg.AppURL, "error=save_failed"), http.StatusFound)
		return
	}

	err = h.credRepo.UpsertWithOrganisation(r.Context(), tenantID, gocardlessProvider, encAccess, "", tokenResp.OrganisationID, env, true)
	if err != nil {
		log.Printf("GoCardless save credentials error: %v", err)
		http.Redirect(w, r, redirectToIntegrations(h.cfg.AppURL, "error=save_failed"), http.StatusFound)
		return
	}

	http.Redirect(w, r, redirectToIntegrations(h.cfg.AppURL, "gocardless=connected"), http.StatusFound)
}

func redirectToIntegrations(appURL string, q string) string {
	base := strings.TrimSuffix(appURL, "/")
	return base + "/settings?tab=integrations&" + q
}

// Webhook handles incoming GoCardless webhooks.
// Uses per-tenant webhook secret when set (resolve tenant from first event's organisation); otherwise falls back to global GoCardlessWebhookSecret.
func (h *GoCardlessHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	observability.PaymentLifecycleMetrics.IncWebhookReceived()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		observability.PaymentLifecycleMetrics.IncWebhookRejected()
		writeError(w, http.StatusBadRequest, "Invalid body")
		return
	}

	signature := r.Header.Get("Webhook-Signature")
	if signature == "" {
		observability.PaymentLifecycleMetrics.IncWebhookRejected()
		writeError(w, http.StatusUnauthorized, "Missing signature")
		return
	}

	// Parse minimally to get organisation from first event so we can resolve tenant and per-tenant secret
	var parseOrg struct {
		Events []struct {
			Links struct {
				Organisation string `json:"organisation"`
			} `json:"links"`
		} `json:"events"`
	}
	if err := json.Unmarshal(body, &parseOrg); err != nil || len(parseOrg.Events) == 0 {
		observability.PaymentLifecycleMetrics.IncWebhookRejected()
		writeError(w, http.StatusBadRequest, "Invalid webhook payload")
		return
	}
	orgID := parseOrg.Events[0].Links.Organisation
	if orgID == "" {
		observability.PaymentLifecycleMetrics.IncWebhookRejected()
		writeError(w, http.StatusBadRequest, "Missing organisation in webhook")
		return
	}

	webhookSecret := h.cfg.GoCardlessWebhookSecret
	if creds, err := h.credRepo.GetByOrganisation(r.Context(), gocardlessProvider, orgID); err == nil && creds != nil && creds.WebhookSecretEncrypted != "" {
		if dec, err := h.encryptor.Decrypt(creds.WebhookSecretEncrypted); err == nil && dec != "" {
			webhookSecret = dec
		}
	}
	if webhookSecret == "" {
		observability.PaymentLifecycleMetrics.IncWebhookRejected()
		writeError(w, http.StatusServiceUnavailable, "Webhooks not configured")
		return
	}

	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		observability.PaymentLifecycleMetrics.IncWebhookRejected()
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var payload struct {
		Events []struct {
			ID           string `json:"id"`
			ResourceType string `json:"resource_type"`
			Action       string `json:"action"`
			Links        struct {
				Organisation string `json:"organisation"`
				Payment      string `json:"payment"`
				Mandate      string `json:"mandate"`
				Subscription string `json:"subscription"`
				Refund       string `json:"refund"`
			} `json:"links"`
			Details struct {
				Reason string `json:"reason"`
				Cause  string `json:"cause"`
			} `json:"details"`
		} `json:"events"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("GoCardless webhook parse error: %v", err)
		observability.PaymentLifecycleMetrics.IncWebhookRejected()
		writeError(w, http.StatusBadRequest, "Invalid webhook payload")
		return
	}

	processingFailed := false
	for _, ev := range payload.Events {
		orgID := ev.Links.Organisation
		if orgID == "" {
			continue
		}

		tenantID, err := h.credRepo.GetTenantIDByOrganisation(r.Context(), gocardlessProvider, orgID)
		if err != nil {
			log.Printf("gocardless.webhook tenant_lookup_failed org=%s event=%s err=%v", orgID, ev.ID, err)
			processingFailed = true
			continue
		}

		if ev.ID != "" && h.eventRepo != nil {
			inserted, err := h.eventRepo.RecordReceived(
				r.Context(),
				tenantID,
				gocardlessProvider,
				ev.ID,
				ev.ResourceType,
				webhookResourceIDFromLinks(ev.ResourceType, ev.Links.Payment, ev.Links.Mandate, ev.Links.Subscription, ev.Links.Refund, ev.ID),
			)
			if err != nil {
				log.Printf("gocardless.webhook dedupe_store_failed tenant=%s event=%s err=%v", tenantID, ev.ID, err)
				processingFailed = true
				continue
			}
			if !inserted {
				observability.PaymentLifecycleMetrics.IncWebhookDuplicate()
				log.Printf("gocardless.webhook duplicate_event tenant=%s event=%s resource=%s", tenantID, ev.ID, ev.ResourceType)
				continue
			}
		}

		var processErr error
		switch ev.ResourceType {
		case "organisations":
			if ev.Action == "disconnected" {
				_ = h.credRepo.UpsertWithOrganisation(r.Context(), tenantID, gocardlessProvider, "", "", orgID, "", false)
			}
		case "payments":
			processErr = h.handlePaymentEvent(r.Context(), tenantID, ev.ID, ev.Links.Payment, ev.Action, ev.Details.Reason, ev.Details.Cause)
		case "mandates":
			h.handleMandateEvent(r.Context(), tenantID, ev.ID, ev.Links.Mandate, ev.Action, ev.Details.Reason, ev.Details.Cause)
		case "subscriptions":
			h.handleSubscriptionEvent(r.Context(), tenantID, ev.ID, ev.Action, ev.Links.Subscription, ev.Links.Payment, ev.Details.Reason, ev.Details.Cause)
		case "refunds":
			processErr = h.handleRefundEvent(r.Context(), tenantID, ev.ID, ev.Links.Refund, ev.Action, ev.Details.Reason, ev.Details.Cause)
		default:
			// other resource types (e.g. creditor, payout, instalment_schedule, block) — log only
			log.Printf("gocardless.webhook ignored_resource tenant=%s org=%s resource=%s action=%s event=%s", tenantID, orgID, ev.ResourceType, ev.Action, ev.ID)
		}

		if processErr != nil {
			observability.PaymentLifecycleMetrics.IncWebhookFailed()
			if ev.ID != "" && h.eventRepo != nil {
				_ = h.eventRepo.MarkFailed(r.Context(), tenantID, gocardlessProvider, ev.ID, processErr.Error())
			}
			log.Printf("gocardless.webhook process_failed tenant=%s resource=%s action=%s event=%s err=%v", tenantID, ev.ResourceType, ev.Action, ev.ID, processErr)
			processingFailed = true
			continue
		}
		observability.PaymentLifecycleMetrics.IncWebhookProcessed()
		if ev.ID != "" && h.eventRepo != nil {
			if err := h.eventRepo.MarkProcessed(r.Context(), tenantID, gocardlessProvider, ev.ID); err != nil {
				_ = h.eventRepo.MarkFailed(r.Context(), tenantID, gocardlessProvider, ev.ID, err.Error())
				log.Printf("gocardless.webhook mark_processed_failed tenant=%s event=%s err=%v", tenantID, ev.ID, err)
				processingFailed = true
			}
		}
	}

	if processingFailed {
		observability.PaymentLifecycleMetrics.IncWebhookFailed()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ListFailedGoCardlessWebhookEvents returns recent failed GoCardless inbound webhook processing events.
func (h *GoCardlessHandler) ListFailedGoCardlessWebhookEvents(w http.ResponseWriter, r *http.Request) {
	if h.eventRepo == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"events": []interface{}{}, "count": 0})
		return
	}
	limit := 50
	if s := strings.TrimSpace(r.URL.Query().Get("limit")); s != "" {
		if parsed, err := strconv.Atoi(s); err == nil {
			limit = parsed
		}
	}
	events, err := h.eventRepo.ListFailed(r.Context(), gocardlessProvider, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list failed GoCardless events")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}

// ReconcileGoCardlessPayment re-fetches a provider payment and re-applies canonical payment/membership status mapping.
// Platform-admin only route for operational recovery.
func (h *GoCardlessHandler) ReconcileGoCardlessPayment(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TenantID  string `json:"tenant_id"`
		PaymentID string `json:"payment_id"`
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
	paymentID := strings.TrimSpace(input.PaymentID)
	if paymentID == "" {
		writeError(w, http.StatusBadRequest, "payment_id is required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil || prov == nil {
		writeError(w, http.StatusBadGateway, "GoCardless not configured for tenant")
		return
	}
	info, err := prov.GetPayment(r.Context(), paymentID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "Failed to fetch GoCardless payment")
		return
	}
	action := reconcileActionFromGoCardlessStatus(info.Status)
	if action == "" {
		writeError(w, http.StatusBadRequest, "Unsupported payment status for reconcile")
		return
	}
	if err := h.handlePaymentEvent(r.Context(), tenantID, "", paymentID, action, "manual_reconcile", "manual_reconcile"); err != nil {
		writeError(w, http.StatusBadGateway, "Failed to reconcile payment")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "reconciled",
		"tenant_id":   tenantID,
		"payment_id":  paymentID,
		"gc_status":   info.Status,
		"gc_action":   action,
		"charge_date": info.ChargeDate,
	})
}

func reconcileActionFromGoCardlessStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "paid_out":
		return "paid_out"
	case "confirmed":
		return "confirmed"
	case "submitted":
		return "submitted"
	case "pending_submission":
		return "pending_submission"
	case "failed":
		return "failed"
	case "late_failure_settled":
		return "late_failure_settled"
	case "charged_back":
		return "charged_back"
	case "chargeback_settled":
		return "chargeback_settled"
	case "cancelled":
		return "cancelled"
	default:
		return ""
	}
}

func (h *GoCardlessHandler) handleMandateEvent(ctx context.Context, tenantID uuid.UUID, eventID, mandateID, action, reason, cause string) {
	if mandateID == "" {
		return
	}
	switch action {
	case "cancelled", "failed", "expired":
		if h.memberRepo != nil {
			_ = h.memberRepo.ClearGoCardlessMandateByProviderID(ctx, tenantID, mandateID)
		}
		if h.ownerAlerts != nil {
			h.ownerAlerts.SendImmediate(ctx, tenantID, observability.AlertMandateCancelled, observability.PaymentAlertData{
				ClientName:    "Mandate " + mandateID,
				FailureReason: fmt.Sprintf("GoCardless mandate %s: reason=%s cause=%s", action, reason, cause),
				Provider:      "GoCardless",
			})
		}
		log.Printf("GoCardless mandate.%s: cleared mandate %s (reason=%s cause=%s)", action, mandateID, reason, cause)
	case "active":
		log.Printf("GoCardless mandate.active: mandate %s is now active", mandateID)
	case "pending_submission", "submitted", "reinstated":
		log.Printf("GoCardless mandate.%s: mandate=%s", action, mandateID)
	default:
		log.Printf("GoCardless webhook mandate.%s mandate=%s reason=%s cause=%s", action, mandateID, reason, cause)
	}
}

func (h *GoCardlessHandler) handleSubscriptionEvent(ctx context.Context, tenantID uuid.UUID, eventID, action, subscriptionID, paymentID, reason, cause string) {
	log.Printf("GoCardless webhook subscription.%s subscription=%s payment=%s reason=%s cause=%s", action, subscriptionID, paymentID, reason, cause)

	switch action {
	case "cancelled", "finished":
		if h.memberRepo != nil && subscriptionID != "" {
			m, _ := h.memberRepo.FindBySubscriptionID(ctx, tenantID, subscriptionID)
			if m != nil {
				newStatus := "expired"
				if action == "cancelled" {
					newStatus = "cancelled"
				}
				_ = h.memberRepo.UpdateStatus(ctx, m.ID, tenantID, newStatus)
				h.linkGCAudit(ctx, tenantID, eventID, nil, &m.ID, newStatus)
				log.Printf("Membership %s set to %s (subscription %s %s)", m.ID, newStatus, subscriptionID, action)
				if h.xeroSync != nil && h.paymentRepo != nil {
					if openPayment, pErr := h.paymentRepo.FindLatestOpenByMembership(ctx, tenantID, m.ID); pErr == nil && openPayment != nil {
						_ = h.xeroSync.VoidInvoiceForPaymentIfUnpaid(ctx, openPayment, "Subscription "+action)
					}
				}
				if h.dispatcher != nil {
					eventType := events.MembershipExpired
					if newStatus == "cancelled" {
						eventType = events.MembershipCancelled
					}
					h.dispatcher.Dispatch(ctx, events.NewEvent(eventType, tenantID, map[string]interface{}{
						"membership_id":   m.ID,
						"client_id":       m.ClientID,
						"subscription_id": subscriptionID,
						"status":          newStatus,
						"source":          "gocardless_webhook",
					}))
				}
			}
		}
	case "payment_created":
		// Payment created by subscription - leave status update to payment webhooks
		log.Printf("GoCardless subscription.payment_created: subscription=%s payment=%s", subscriptionID, paymentID)
	}
}

func (h *GoCardlessHandler) handleRefundEvent(ctx context.Context, tenantID uuid.UUID, eventID, refundID, action, reason, cause string) error {
	log.Printf("GoCardless webhook refund.%s refund=%s reason=%s cause=%s", action, refundID, reason, cause)
	if refundID == "" || h.paymentRepo == nil {
		return nil
	}
	if !isGoCardlessRefundActionSupported(action) {
		return nil
	}

	prov, err := h.getGoCardlessProvider(ctx, tenantID)
	if err != nil {
		return err
	}
	if prov == nil {
		return nil
	}
	refundInfo, err := prov.GetRefund(ctx, refundID)
	if err != nil {
		return err
	}
	if refundInfo.PaymentID == "" {
		return nil
	}
	p, err := h.paymentRepo.GetByProviderReference(ctx, tenantID, refundInfo.PaymentID)
	if err != nil {
		return err
	}
	if p == nil {
		p, err = h.reconcileExternalPayment(ctx, tenantID, refundInfo.PaymentID)
		if err != nil || p == nil {
			return err
		}
	}
	if err := h.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, "refunded", nil, nil, nil); err != nil {
		return err
	}
	h.linkGCAudit(ctx, tenantID, eventID, &p.ID, &p.MembershipID, "refunded")
	p.Status = "refunded"
	p.PaidDate = nil
	if h.xeroSync != nil {
		refundAmount := float64(refundInfo.Amount) / 100.0
		if refundAmount <= 0 {
			refundAmount = p.Amount
		}
		_ = h.xeroSync.CreateCreditNoteForMembershipPayment(ctx, p, refundAmount, "Payment refunded")
	}
	if h.dispatcher != nil {
		h.dispatcher.Dispatch(ctx, events.NewEvent(events.PaymentRefunded, tenantID, map[string]interface{}{
			"payment_id":      p.ID,
			"membership_id":   p.MembershipID,
			"client_id":       p.ClientID,
			"amount":          float64(refundInfo.Amount) / 100.0,
			"status":          "refunded",
			"provider_ref":    refundInfo.PaymentID,
			"provider_refund": refundID,
			"source":          "gocardless_webhook",
		}))
	}
	return nil
}

// handlePaymentEvent updates the payments table from a GoCardless payment webhook event.
// If the payment doesn't exist in our system (created externally), it creates a record.
// Sends email notifications for payment success/failure.
func (h *GoCardlessHandler) handlePaymentEvent(ctx context.Context, tenantID uuid.UUID, eventID, gcPaymentID, action, reason, cause string) error {
	if gcPaymentID == "" || h.paymentRepo == nil {
		return nil
	}
	p, err := h.paymentRepo.GetByProviderReference(ctx, tenantID, gcPaymentID)
	if err != nil {
		return err
	}

	if p == nil {
		p, err = h.reconcileExternalPayment(ctx, tenantID, gcPaymentID)
		if err != nil {
			return err
		}
		if p == nil {
			return errors.New("payment not linked to a membership")
		}
	}

	var status string
	var paidDate *time.Time
	var failureReason, failureCode *string
	isDispute := false
	mappedStatus, mappedDispute, handled := mapGoCardlessPaymentAction(action)
	if !handled {
		log.Printf("gocardless.webhook payment_unhandled action=%s payment=%s", action, gcPaymentID)
		return nil
	}
	if mappedStatus == "" {
		log.Printf("gocardless.webhook payment_approval action=%s payment=%s", action, gcPaymentID)
		return nil
	}
	status = mappedStatus
	isDispute = mappedDispute
	switch status {
	case "succeeded":
		status = "succeeded"
		now := time.Now().UTC()
		paidDate = &now
	case "pending":
		// payments table currently persists processing states as pending until settled.
		status = "pending"
	case "failed":
		status = "failed"
		if reason != "" {
			failureReason = &reason
		}
		if cause != "" {
			failureCode = &cause
		}
	case "disputed":
		status = "disputed"
		if reason != "" {
			failureReason = &reason
		}
		if cause != "" {
			failureCode = &cause
		}
	}

	if err := h.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, status, paidDate, failureReason, failureCode); err != nil {
		return err
	}
	h.linkGCAudit(ctx, tenantID, eventID, &p.ID, &p.MembershipID, status)
	p.Status = status
	p.PaidDate = paidDate
	if h.xeroSync != nil {
		switch status {
		case "pending":
			_, _ = h.xeroSync.EnsureInvoiceForPayment(ctx, p)
		case "succeeded":
			_, _ = h.xeroSync.EnsureInvoiceForPayment(ctx, p)
			_ = h.xeroSync.ApplyPaymentForPayment(ctx, p)
		case "disputed":
			_ = h.xeroSync.CreateCreditNoteForMembershipPayment(ctx, p, p.Amount, "Payment disputed")
		case "cancelled", "failed":
			_ = h.xeroSync.VoidInvoiceForPaymentIfUnpaid(ctx, p, "Payment "+status)
		}
	}
	if p.MembershipID != uuid.Nil {
		if err := syncMembershipStatusAfterPayment(ctx, h.memberRepo, tenantID, p.MembershipID, status, time.Now().UTC()); err != nil {
			log.Printf("gocardless membership status sync failed tenant=%s membership=%s payment=%s status=%s err=%v", tenantID, p.MembershipID, p.ID, status, err)
		}
	}
	if status == "disputed" && isDispute && h.disputeRepo != nil {
		var providerDisputeID *string
		synthetic := gcPaymentID + ":chargeback"
		providerDisputeID = &synthetic
		_, _ = h.disputeRepo.UpsertByProviderOrPayment(
			ctx,
			tenantID,
			p.ID,
			p.ClientID,
			providerDisputeID,
			failureReason,
			"open",
			p.Amount,
			nil,
			nil,
			nil,
		)
	}
	if h.dispatcher != nil {
		if status == "succeeded" || status == "failed" || status == "disputed" {
			eventType := events.PaymentFailed
			if status == "succeeded" {
				eventType = events.PaymentSucceeded
			} else if status == "disputed" {
				eventType = events.PaymentDisputed
			}
			h.dispatcher.Dispatch(ctx, events.NewEvent(eventType, tenantID, map[string]interface{}{
				"payment_id":     p.ID,
				"membership_id":  p.MembershipID,
				"client_id":      p.ClientID,
				"amount":         p.Amount,
				"status":         status,
				"provider_ref":   gcPaymentID,
				"failure_reason": reason,
				"failure_code":   cause,
				"source":         "gocardless_webhook",
			}))
		}
	}

	h.sendPaymentEmail(ctx, tenantID, p, status, reason)
	if h.ownerAlerts != nil && (status == "failed" || status == "disputed") {
		clientName, membershipTitle := h.resolveClientAndMembershipForAlert(ctx, tenantID, p)
		alertType := observability.AlertPaymentFailed
		if status == "disputed" {
			alertType = observability.AlertDispute
		}
		h.ownerAlerts.SendImmediate(ctx, tenantID, alertType, observability.PaymentAlertData{
			ClientName:      clientName,
			MembershipTitle: membershipTitle,
			Amount:          p.Amount,
			CurrencyCode:    p.CurrencyCode,
			FailureReason:   reason,
			DisputeReason:   reason,
			Provider:        "GoCardless",
		})
	}
	log.Printf("gocardless.webhook payment_updated tenant=%s payment=%s provider_ref=%s status=%s", tenantID, p.ID, gcPaymentID, status)
	return nil
}

func mapGoCardlessPaymentAction(action string) (status string, isDispute bool, handled bool) {
	switch action {
	case "paid_out":
		return "succeeded", false, true
	case "confirmed", "submitted", "pending_submission":
		return "pending", false, true
	case "failed", "late_failure_settled":
		return "failed", false, true
	case "charged_back", "chargeback_settled":
		return "disputed", true, true
	case "cancelled":
		return "cancelled", false, true
	case "customer_approval_granted", "customer_approval_denied":
		return "", false, true
	default:
		return "", false, false
	}
}

func isGoCardlessRefundActionSupported(action string) bool {
	switch action {
	case "created", "submitted", "paid", "refunded":
		return true
	default:
		return false
	}
}

func webhookResourceIDFromLinks(resourceType, paymentID, mandateID, subscriptionID, refundID, fallback string) string {
	switch resourceType {
	case "payments":
		return paymentID
	case "mandates":
		return mandateID
	case "subscriptions":
		return subscriptionID
	case "refunds":
		return refundID
	default:
		return fallback
	}
}

func (h *GoCardlessHandler) reconcileExternalPayment(ctx context.Context, tenantID uuid.UUID, gcPaymentID string) (*repository.Payment, error) {
	if h.memberRepo == nil {
		return nil, nil
	}
	prov, err := h.getGoCardlessProvider(ctx, tenantID)
	if err != nil || prov == nil {
		return nil, err
	}

	paymentInfo, err := prov.GetPayment(ctx, gcPaymentID)
	if err != nil {
		return nil, err
	}
	if paymentInfo.MandateID == "" {
		return nil, nil
	}

	membership, err := h.memberRepo.FindByMandateID(ctx, tenantID, paymentInfo.MandateID)
	if err != nil || membership == nil {
		return nil, err
	}

	dueDate := time.Now().UTC()
	if paymentInfo.ChargeDate != "" {
		if parsed, parseErr := time.Parse("2006-01-02", paymentInfo.ChargeDate); parseErr == nil {
			dueDate = parsed
		}
	}
	amount := float64(paymentInfo.Amount) / 100.0

	currencyCode := strings.ToUpper(strings.TrimSpace(membership.CurrencyCode))
	if currencyCode == "" {
		currencyCode = "AUD"
	}
	baseAmount, taxAmount := splitAmountByTaxMode(
		amount,
		h.tenantTaxPercent(ctx, tenantID),
		h.membershipAppliesSalesTax(ctx, tenantID, membership.MembershipTypeID),
	)
	p, _, err := h.paymentRepo.CreateIfNotExists(ctx, tenantID, membership.ID, membership.ClientID, amount, currencyCode, dueDate, gcPaymentID, &repository.PaymentComponents{
		BaseAmount:          baseAmount,
		TaxAmount:           taxAmount,
		ProcessingFeeAmount: 0,
	})
	if err != nil {
		return nil, err
	}
	observability.PaymentLifecycleMetrics.IncPaymentReconciled()
	log.Printf("gocardless.webhook reconciled_external_payment tenant=%s membership=%s provider_ref=%s", tenantID, membership.ID, gcPaymentID)
	return p, nil
}

// sendPaymentEmail sends payment success/failure emails to the client.
func (h *GoCardlessHandler) sendPaymentEmail(ctx context.Context, tenantID uuid.UUID, p *repository.Payment, status, failureReason string) {
	if h.emailSvc == nil || h.clientRepo == nil {
		return
	}
	if p == nil {
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
		"amount":                 money(p.Amount), // backward compatibility in existing templates
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
		switch status {
		case "succeeded", "confirmed", "paid_out":
			input, ok := h.buildOperationalTemplateEmail(context.Background(), tenantID, "payment_receipt", vars)
			if !ok {
				return
			}
			input.To = *client.Email
			if _, err := h.emailSvc.Send(context.Background(), *input); err != nil {
				log.Printf("Error sending payment receipt to %s: %v", *client.Email, err)
			}
		case "failed":
			input, ok := h.buildOperationalTemplateEmail(context.Background(), tenantID, "payment_failed", vars)
			if !ok {
				return
			}
			input.To = *client.Email
			if _, err := h.emailSvc.Send(context.Background(), *input); err != nil {
				log.Printf("Error sending payment failed email to %s: %v", *client.Email, err)
			}
		}
	}()
}

func (h *GoCardlessHandler) buildOperationalTemplateEmail(ctx context.Context, tenantID uuid.UUID, templateKey string, vars map[string]string) (*email.SendInput, bool) {
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
			log.Printf("gocardless.email template lookup failed tenant=%s template=%s err=%v", tenantID, templateKey, err)
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

func (h *GoCardlessHandler) resolveClientAndMembershipForAlert(ctx context.Context, tenantID uuid.UUID, p *repository.Payment) (string, string) {
	clientName := "Unknown"
	membershipTitle := "Membership"
	if p == nil {
		return clientName, membershipTitle
	}
	if h.clientRepo != nil {
		if client, err := h.clientRepo.FindByID(ctx, p.ClientID, tenantID); err == nil && client != nil {
			clientName = client.Name
		}
	}
	if h.memberRepo != nil && h.mtRepo != nil && p.MembershipID != uuid.Nil {
		if membership, err := h.memberRepo.FindByID(ctx, p.MembershipID, tenantID); err == nil && membership != nil {
			if mt, err := h.mtRepo.FindByID(ctx, membership.MembershipTypeID, tenantID); err == nil && mt != nil {
				membershipTitle = mt.Title
			}
		}
	}
	return clientName, membershipTitle
}

func (h *GoCardlessHandler) isEmailTemplateEnabled(ctx context.Context, tenantID uuid.UUID, templateKey string) bool {
	if h.emailTemplateRepo == nil {
		return true
	}
	enabled, err := h.emailTemplateRepo.IsEnabled(ctx, tenantID, templateKey)
	if err != nil {
		log.Printf("gocardless.email toggle check failed tenant=%s template=%s err=%v", tenantID, templateKey, err)
		return true
	}
	return enabled
}

// CreatePayment creates a payment in GoCardless and in the DB (production flow).
// Body: { "membership_id": "uuid", "amount": 50.00, "due_date": "2025-03-01" }.
// Membership must have gocardless_mandate_id set (e.g. from Connect or SetupDirectDebit).
func (h *GoCardlessHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	tenantIDStr := middleware.GetTenantID(r.Context())
	if tenantIDStr == "" {
		writeError(w, http.StatusForbidden, "Tenant context required")
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		writeError(w, http.StatusForbidden, "Invalid tenant")
		return
	}

	var input struct {
		MembershipID string  `json:"membership_id"`
		Amount       float64 `json:"amount"`
		DueDate      string  `json:"due_date"`
	}
	if err := decodeJSON(r, &input); err != nil || input.MembershipID == "" || input.Amount <= 0 || input.DueDate == "" {
		writeError(w, http.StatusBadRequest, "membership_id, amount and due_date are required")
		return
	}
	membershipID, err := uuid.Parse(input.MembershipID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid membership_id")
		return
	}
	dueDate, err := time.Parse("2006-01-02", input.DueDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "due_date must be YYYY-MM-DD")
		return
	}

	creds, err := h.credRepo.GetByProvider(r.Context(), tenantID, gocardlessProvider)
	if err != nil || creds == nil || creds.APIKeyEncrypted == "" {
		writeError(w, http.StatusBadRequest, "GoCardless not configured for this tenant")
		return
	}
	apiKey, err := h.encryptor.Decrypt(creds.APIKeyEncrypted)
	if err != nil {
		log.Printf("GoCardless decrypt API key: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to use payment credentials")
		return
	}
	env := creds.Environment
	if env == "" {
		env = "sandbox"
	}
	prov := payment.NewGoCardlessProvider(apiKey, "", env)

	membership, err := h.memberRepo.FindByID(r.Context(), membershipID, tenantID)
	if err != nil || membership == nil {
		writeError(w, http.StatusNotFound, "Membership not found")
		return
	}
	if membership.GoCardlessMandateID == nil || *membership.GoCardlessMandateID == "" {
		writeError(w, http.StatusBadRequest, "Membership has no direct debit mandate; set up GoCardless first")
		return
	}

	client, err := h.clientRepo.FindByID(r.Context(), membership.ClientID, tenantID)
	if err != nil || client == nil {
		writeError(w, http.StatusNotFound, "Client not found")
		return
	}
	if client.GoCardlessCustomerID == nil || *client.GoCardlessCustomerID == "" {
		// Create customer and save
		name := ""
		if client.Name != "" {
			name = client.Name
		}
		email, phone := "", ""
		if client.Email != nil {
			email = *client.Email
		}
		if client.Phone != nil {
			phone = *client.Phone
		}
		customerID, err := prov.CreateCustomer(r.Context(), payment.CreateCustomerInput{
			Email:     email,
			Name:      name,
			Phone:     phone,
			Reference: client.ID.String(),
		})
		if err != nil {
			log.Printf("GoCardless CreateCustomer: %v", err)
			writeProviderError(w, err)
			return
		}
		if err := h.clientRepo.SetGoCardlessCustomerID(r.Context(), client.ID, tenantID, customerID); err != nil {
			log.Printf("SetGoCardlessCustomerID: %v", err)
		}
		// use for payment below
		client.GoCardlessCustomerID = &customerID
	}

	amountMinor := int64(input.Amount * 100) // dollars to cents
	if amountMinor <= 0 {
		writeError(w, http.StatusBadRequest, "amount must be positive")
		return
	}
	currencyCode := strings.ToUpper(strings.TrimSpace(membership.CurrencyCode))
	if currencyCode == "" {
		currencyCode = "AUD"
	}
	gcPaymentID, err := prov.CreatePayment(r.Context(), payment.CreatePaymentInput{
		MandateID:   *membership.GoCardlessMandateID,
		Amount:      amountMinor,
		Currency:    currencyCode,
		Description: "Membership payment",
		ChargeDate:  dueDate,
		Reference:   membershipID.String() + "-" + dueDate.Format("20060102"),
	})
	if err != nil {
		log.Printf("GoCardless CreatePayment: %v", err)
		writeProviderError(w, err)
		return
	}

	baseAmount, taxAmount := splitAmountByTaxMode(
		input.Amount,
		h.tenantTaxPercent(r.Context(), tenantID),
		h.membershipAppliesSalesTax(r.Context(), tenantID, membership.MembershipTypeID),
	)
	pay, err := h.paymentRepo.Create(r.Context(), tenantID, membershipID, membership.ClientID, input.Amount, currencyCode, dueDate, gcPaymentID, &repository.PaymentComponents{
		BaseAmount:          baseAmount,
		TaxAmount:           taxAmount,
		ProcessingFeeAmount: 0,
	})
	if err != nil {
		log.Printf("PaymentRepo.Create: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to save payment")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"payment_id":         pay.ID.String(),
		"provider_reference": gcPaymentID,
		"status":             pay.Status,
		"amount":             pay.Amount,
		"due_date":           pay.DueDate.Format("2006-01-02"),
	})
}

// getGoCardlessProvider returns a GoCardless provider for the tenant (for extended API calls).
func (h *GoCardlessHandler) getGoCardlessProvider(ctx context.Context, tenantID uuid.UUID) (payment.GoCardlessExtended, error) {
	creds, err := h.credRepo.GetByProvider(ctx, tenantID, gocardlessProvider)
	if err != nil || creds == nil || creds.APIKeyEncrypted == "" {
		return nil, err
	}
	apiKey, err := h.encryptor.Decrypt(creds.APIKeyEncrypted)
	if err != nil {
		return nil, err
	}
	env := creds.Environment
	if env == "" {
		env = "sandbox"
	}
	return payment.NewGoCardlessProvider(apiKey, "", env), nil
}

func (h *GoCardlessHandler) requireTenant(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	tenantIDStr := middleware.GetTenantID(r.Context())
	if tenantIDStr == "" {
		writeError(w, http.StatusForbidden, "Tenant context required")
		return uuid.Nil, false
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		writeError(w, http.StatusForbidden, "Invalid tenant")
		return uuid.Nil, false
	}
	return tenantID, true
}

func (h *GoCardlessHandler) RetryPayment(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	paymentIDStr := chi.URLParam(r, "paymentID")
	paymentID, err := uuid.Parse(paymentIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid payment_id")
		return
	}
	p, err := h.paymentRepo.GetByID(r.Context(), paymentID, tenantID)
	if err != nil || p == nil || p.ProviderReference == nil || *p.ProviderReference == "" {
		writeError(w, http.StatusNotFound, "Payment not found")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.RetryPayment(r.Context(), *p.ProviderReference); err != nil {
		log.Printf("GoCardless RetryPayment: %v", err)
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "retry_submitted"})
}

func (h *GoCardlessHandler) CancelPayment(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	paymentIDStr := chi.URLParam(r, "paymentID")
	paymentID, err := uuid.Parse(paymentIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid payment_id")
		return
	}
	p, err := h.paymentRepo.GetByID(r.Context(), paymentID, tenantID)
	if err != nil || p == nil || p.ProviderReference == nil || *p.ProviderReference == "" {
		writeError(w, http.StatusNotFound, "Payment not found")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.CancelPayment(r.Context(), *p.ProviderReference); err != nil {
		log.Printf("GoCardless CancelPayment: %v", err)
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *GoCardlessHandler) CreateRefund(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	var input struct {
		PaymentID string  `json:"payment_id"` // our DB payment UUID
		Amount    float64 `json:"amount"`
		Reference string  `json:"reference"`
	}
	if err := decodeJSON(r, &input); err != nil || input.PaymentID == "" || input.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "payment_id and amount are required")
		return
	}
	paymentID, err := uuid.Parse(input.PaymentID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid payment_id")
		return
	}
	p, err := h.paymentRepo.GetByID(r.Context(), paymentID, tenantID)
	if err != nil || p == nil || p.ProviderReference == nil || *p.ProviderReference == "" {
		writeError(w, http.StatusNotFound, "Payment not found")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	amountMinor := int64(input.Amount * 100)
	refID, err := prov.CreateRefund(r.Context(), *p.ProviderReference, amountMinor, input.Reference)
	if err != nil {
		log.Printf("GoCardless CreateRefund: %v", err)
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"refund_id": refID})
}

func (h *GoCardlessHandler) GetRefund(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	refundID := chi.URLParam(r, "refundID")
	if refundID == "" {
		writeError(w, http.StatusBadRequest, "refund_id required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	info, err := prov.GetRefund(r.Context(), refundID)
	if err != nil {
		log.Printf("GoCardless GetRefund: %v", err)
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id": info.ID, "payment_id": info.PaymentID, "amount": info.Amount,
		"status": info.Status, "reference": info.Reference,
	})
}

func (h *GoCardlessHandler) GetMandate(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	mandateID := chi.URLParam(r, "mandateID")
	if mandateID == "" {
		writeError(w, http.StatusBadRequest, "mandate_id required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	info, err := prov.GetMandate(r.Context(), mandateID)
	if err != nil {
		log.Printf("GoCardless GetMandate: %v", err)
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id": info.ID, "status": info.Status, "scheme": info.Scheme,
		"reference": info.Reference, "customer_id": info.CustomerID,
	})
}

func (h *GoCardlessHandler) ReinstateMandate(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	mandateID := chi.URLParam(r, "mandateID")
	if mandateID == "" {
		writeError(w, http.StatusBadRequest, "mandate_id required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.ReinstateMandate(r.Context(), mandateID); err != nil {
		log.Printf("GoCardless ReinstateMandate: %v", err)
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "reinstated"})
}

func (h *GoCardlessHandler) ListMandates(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	customerID := r.URL.Query().Get("customer_id")
	if customerID == "" {
		writeError(w, http.StatusBadRequest, "customer_id query required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	list, err := prov.ListMandates(r.Context(), customerID)
	if err != nil {
		log.Printf("GoCardless ListMandates: %v", err)
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"mandates": list})
}

func (h *GoCardlessHandler) ListCustomers(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	referencePrefix := r.URL.Query().Get("reference_prefix")
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	list, err := prov.ListCustomers(r.Context(), referencePrefix)
	if err != nil {
		log.Printf("GoCardless ListCustomers: %v", err)
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"customers": list})
}

func (h *GoCardlessHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	mandateID := r.URL.Query().Get("mandate_id")
	customerID := r.URL.Query().Get("customer_id")
	status := r.URL.Query().Get("status")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := parseInt(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	list, err := prov.ListPayments(r.Context(), mandateID, customerID, status, limit)
	if err != nil {
		log.Printf("GoCardless ListPayments: %v", err)
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"payments": list})
}

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

func (h *GoCardlessHandler) CreateRedirectFlow(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	var input struct {
		RedirectURI  string `json:"redirect_uri"`
		Description  string `json:"description"`
		SessionToken string `json:"session_token"`
	}
	if err := decodeJSON(r, &input); err != nil || input.RedirectURI == "" || input.SessionToken == "" {
		writeError(w, http.StatusBadRequest, "redirect_uri and session_token are required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	redirectURL, flowID, err := prov.CreateRedirectFlow(r.Context(), input.RedirectURI, input.Description, input.SessionToken)
	if err != nil {
		log.Printf("GoCardless CreateRedirectFlow: %v", err)
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"redirect_url": redirectURL, "flow_id": flowID})
}

func (h *GoCardlessHandler) CompleteRedirectFlow(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	var input struct {
		FlowID       string `json:"flow_id"`
		SessionToken string `json:"session_token"`
	}
	if err := decodeJSON(r, &input); err != nil || input.FlowID == "" || input.SessionToken == "" {
		writeError(w, http.StatusBadRequest, "flow_id and session_token are required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	mandateID, customerID, err := prov.CompleteRedirectFlow(r.Context(), input.FlowID, input.SessionToken)
	if err != nil {
		log.Printf("GoCardless CompleteRedirectFlow: %v", err)
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"mandate_id": mandateID, "customer_id": customerID})
}

// ===== All remaining GoCardless handler methods =====

// --- Payments: Get, Update ---

func (h *GoCardlessHandler) GetPaymentHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	gcPaymentID := chi.URLParam(r, "paymentID")
	if gcPaymentID == "" {
		writeError(w, http.StatusBadRequest, "paymentID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	info, err := prov.GetPayment(r.Context(), gcPaymentID)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *GoCardlessHandler) UpdatePaymentHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	gcPaymentID := chi.URLParam(r, "paymentID")
	if gcPaymentID == "" {
		writeError(w, http.StatusBadRequest, "paymentID required")
		return
	}
	var input struct {
		Metadata map[string]string `json:"metadata"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid body")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.UpdatePayment(r.Context(), gcPaymentID, input.Metadata); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// --- Refunds: List, Update ---

func (h *GoCardlessHandler) ListRefundsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	paymentID := r.URL.Query().Get("payment_id")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := parseInt(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	list, err := prov.ListRefunds(r.Context(), paymentID, limit)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"refunds": list})
}

func (h *GoCardlessHandler) UpdateRefundHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	refundID := chi.URLParam(r, "refundID")
	if refundID == "" {
		writeError(w, http.StatusBadRequest, "refundID required")
		return
	}
	var input struct {
		Metadata map[string]string `json:"metadata"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid body")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.UpdateRefund(r.Context(), refundID, input.Metadata); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// --- Mandates: Update, Cancel, PDF ---

func (h *GoCardlessHandler) UpdateMandateHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	mandateID := chi.URLParam(r, "mandateID")
	if mandateID == "" {
		writeError(w, http.StatusBadRequest, "mandateID required")
		return
	}
	var input struct {
		Metadata map[string]string `json:"metadata"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid body")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.UpdateMandate(r.Context(), mandateID, input.Metadata); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *GoCardlessHandler) CancelMandateHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	mandateID := chi.URLParam(r, "mandateID")
	if mandateID == "" {
		writeError(w, http.StatusBadRequest, "mandateID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.CancelMandate(r.Context(), mandateID); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *GoCardlessHandler) CreateMandatePDFHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	mandateID := chi.URLParam(r, "mandateID")
	if mandateID == "" {
		writeError(w, http.StatusBadRequest, "mandateID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	pdfURL, err := prov.CreateMandatePDF(r.Context(), mandateID)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": pdfURL})
}

// --- Customers: Get, Update, Remove ---

func (h *GoCardlessHandler) GetCustomerHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	customerID := chi.URLParam(r, "customerID")
	if customerID == "" {
		writeError(w, http.StatusBadRequest, "customerID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	info, err := prov.GetCustomer(r.Context(), customerID)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *GoCardlessHandler) UpdateCustomerHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	customerID := chi.URLParam(r, "customerID")
	if customerID == "" {
		writeError(w, http.StatusBadRequest, "customerID required")
		return
	}
	var input struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid body")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.UpdateCustomer(r.Context(), customerID, input.Email, input.Name, input.Phone); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *GoCardlessHandler) RemoveCustomerHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	customerID := chi.URLParam(r, "customerID")
	if customerID == "" {
		writeError(w, http.StatusBadRequest, "customerID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.RemoveCustomer(r.Context(), customerID); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// --- Customer Bank Accounts ---

func (h *GoCardlessHandler) CreateCustomerBankAccountHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	var input struct {
		CustomerID        string `json:"customer_id"`
		AccountHolderName string `json:"account_holder_name"`
		AccountNumber     string `json:"account_number"`
		BranchCode        string `json:"branch_code"`
		CountryCode       string `json:"country_code"`
	}
	if err := decodeJSON(r, &input); err != nil || input.CustomerID == "" || input.AccountNumber == "" {
		writeError(w, http.StatusBadRequest, "customer_id and account_number required")
		return
	}
	if input.CountryCode == "" {
		input.CountryCode = "AU"
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	id, err := prov.CreateCustomerBankAccount(r.Context(), input.CustomerID, input.AccountHolderName, input.AccountNumber, input.BranchCode, input.CountryCode)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"bank_account_id": id})
}

func (h *GoCardlessHandler) GetCustomerBankAccountHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	bankAccountID := chi.URLParam(r, "bankAccountID")
	if bankAccountID == "" {
		writeError(w, http.StatusBadRequest, "bankAccountID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	info, err := prov.GetCustomerBankAccount(r.Context(), bankAccountID)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *GoCardlessHandler) ListCustomerBankAccountsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	customerID := r.URL.Query().Get("customer_id")
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	list, err := prov.ListCustomerBankAccounts(r.Context(), customerID)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"customer_bank_accounts": list})
}

func (h *GoCardlessHandler) UpdateCustomerBankAccountHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	bankAccountID := chi.URLParam(r, "bankAccountID")
	if bankAccountID == "" {
		writeError(w, http.StatusBadRequest, "bankAccountID required")
		return
	}
	var input struct {
		Metadata map[string]string `json:"metadata"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid body")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.UpdateCustomerBankAccount(r.Context(), bankAccountID, input.Metadata); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *GoCardlessHandler) DisableCustomerBankAccountHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	bankAccountID := chi.URLParam(r, "bankAccountID")
	if bankAccountID == "" {
		writeError(w, http.StatusBadRequest, "bankAccountID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.DisableCustomerBankAccount(r.Context(), bankAccountID); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

// --- Redirect Flows: Get ---

func (h *GoCardlessHandler) GetRedirectFlowHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	flowID := chi.URLParam(r, "flowID")
	if flowID == "" {
		writeError(w, http.StatusBadRequest, "flowID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	info, err := prov.GetRedirectFlow(r.Context(), flowID)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// --- Subscriptions ---

func (h *GoCardlessHandler) CreateSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	var input struct {
		MandateID     string  `json:"mandate_id"`
		Amount        float64 `json:"amount"`
		Currency      string  `json:"currency"`
		Interval      string  `json:"interval"`
		IntervalCount int     `json:"interval_count"`
		Name          string  `json:"name"`
		Reference     string  `json:"reference"`
		EndDate       string  `json:"end_date"`
	}
	if err := decodeJSON(r, &input); err != nil || input.MandateID == "" || input.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "mandate_id and amount required")
		return
	}
	if input.Currency == "" {
		input.Currency = "AUD"
	}
	if input.Interval == "" {
		input.Interval = "month"
	}
	if input.IntervalCount <= 0 {
		input.IntervalCount = 1
	}
	var endDate *time.Time
	if input.EndDate != "" {
		t, err := time.Parse("2006-01-02", input.EndDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, "end_date must be YYYY-MM-DD")
			return
		}
		endDate = &t
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	amountMinor := int64(input.Amount * 100)
	id, err := prov.CreateSubscription(r.Context(), input.MandateID, amountMinor, input.Currency, input.Interval, input.IntervalCount, input.Name, input.Reference, endDate)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"subscription_id": id})
}

func (h *GoCardlessHandler) GetSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	subID := chi.URLParam(r, "subscriptionID")
	if subID == "" {
		writeError(w, http.StatusBadRequest, "subscriptionID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	info, err := prov.GetSubscription(r.Context(), subID)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *GoCardlessHandler) UpdateSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	subID := chi.URLParam(r, "subscriptionID")
	if subID == "" {
		writeError(w, http.StatusBadRequest, "subscriptionID required")
		return
	}
	var input struct {
		Amount  *float64 `json:"amount"`
		Name    string   `json:"name"`
		EndDate string   `json:"end_date"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid body")
		return
	}
	var amountMinor *int64
	if input.Amount != nil {
		v := int64(*input.Amount * 100)
		amountMinor = &v
	}
	var endDate *time.Time
	if input.EndDate != "" {
		t, err := time.Parse("2006-01-02", input.EndDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, "end_date must be YYYY-MM-DD")
			return
		}
		endDate = &t
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.UpdateSubscription(r.Context(), subID, amountMinor, input.Name, endDate); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *GoCardlessHandler) CancelSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	subID := chi.URLParam(r, "subscriptionID")
	if subID == "" {
		writeError(w, http.StatusBadRequest, "subscriptionID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.CancelSubscription(r.Context(), subID); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *GoCardlessHandler) PauseSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	subID := chi.URLParam(r, "subscriptionID")
	if subID == "" {
		writeError(w, http.StatusBadRequest, "subscriptionID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.PauseSubscription(r.Context(), subID); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "paused"})
}

func (h *GoCardlessHandler) ResumeSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	subID := chi.URLParam(r, "subscriptionID")
	if subID == "" {
		writeError(w, http.StatusBadRequest, "subscriptionID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.ResumeSubscription(r.Context(), subID); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resumed"})
}

func (h *GoCardlessHandler) ListSubscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	customerID := r.URL.Query().Get("customer_id")
	mandateID := r.URL.Query().Get("mandate_id")
	status := r.URL.Query().Get("status")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := parseInt(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	list, err := prov.ListSubscriptions(r.Context(), customerID, mandateID, status, limit)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"subscriptions": list})
}

// --- Bank Details Lookup ---

func (h *GoCardlessHandler) ValidateBankDetailsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	var input struct {
		CountryCode       string `json:"country_code"`
		AccountNumber     string `json:"account_number"`
		BranchCode        string `json:"branch_code"`
		AccountHolderName string `json:"account_holder_name"`
	}
	if err := decodeJSON(r, &input); err != nil || input.AccountNumber == "" {
		writeError(w, http.StatusBadRequest, "account_number required")
		return
	}
	if input.CountryCode == "" {
		input.CountryCode = "AU"
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	result, err := prov.ValidateBankDetails(r.Context(), input.CountryCode, input.AccountNumber, input.BranchCode, input.AccountHolderName)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// --- Instalment Schedules ---

func (h *GoCardlessHandler) CreateInstalmentScheduleHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	var input struct {
		MandateID   string `json:"mandate_id"`
		TotalAmount int64  `json:"total_amount"`
		Currency    string `json:"currency"`
		Name        string `json:"name"`
		Reference   string `json:"reference"`
		Instalments []struct {
			ChargeDate string `json:"charge_date"`
			Amount     int64  `json:"amount"`
		} `json:"instalments"`
	}
	if err := decodeJSON(r, &input); err != nil || input.MandateID == "" || len(input.Instalments) == 0 {
		writeError(w, http.StatusBadRequest, "mandate_id and instalments required")
		return
	}
	if input.Currency == "" {
		input.Currency = "AUD"
	}
	instalments := make([]payment.InstalmentDate, len(input.Instalments))
	for i, inst := range input.Instalments {
		instalments[i] = payment.InstalmentDate{ChargeDate: inst.ChargeDate, Amount: inst.Amount}
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	id, err := prov.CreateInstalmentSchedule(r.Context(), input.MandateID, input.TotalAmount, input.Currency, instalments, input.Name, input.Reference)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"instalment_schedule_id": id})
}

func (h *GoCardlessHandler) GetInstalmentScheduleHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	scheduleID := chi.URLParam(r, "scheduleID")
	if scheduleID == "" {
		writeError(w, http.StatusBadRequest, "scheduleID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	info, err := prov.GetInstalmentSchedule(r.Context(), scheduleID)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *GoCardlessHandler) ListInstalmentSchedulesHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	mandateID := r.URL.Query().Get("mandate_id")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := parseInt(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	list, err := prov.ListInstalmentSchedules(r.Context(), mandateID, limit)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"instalment_schedules": list})
}

func (h *GoCardlessHandler) UpdateInstalmentScheduleHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	scheduleID := chi.URLParam(r, "scheduleID")
	if scheduleID == "" {
		writeError(w, http.StatusBadRequest, "scheduleID required")
		return
	}
	var input struct {
		Metadata map[string]string `json:"metadata"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid body")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.UpdateInstalmentSchedule(r.Context(), scheduleID, input.Metadata); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *GoCardlessHandler) CancelInstalmentScheduleHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	scheduleID := chi.URLParam(r, "scheduleID")
	if scheduleID == "" {
		writeError(w, http.StatusBadRequest, "scheduleID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.CancelInstalmentSchedule(r.Context(), scheduleID); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// --- Payouts ---

func (h *GoCardlessHandler) ListPayoutsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	var since, until time.Time
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			since = t
		}
	}
	if u := r.URL.Query().Get("until"); u != "" {
		if t, err := time.Parse("2006-01-02", u); err == nil {
			until = t
		}
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	list, err := prov.ListPayouts(r.Context(), since, until)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"payouts": list})
}

func (h *GoCardlessHandler) GetPayoutHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	payoutID := chi.URLParam(r, "payoutID")
	if payoutID == "" {
		writeError(w, http.StatusBadRequest, "payoutID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	info, err := prov.GetPayout(r.Context(), payoutID)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *GoCardlessHandler) UpdatePayoutHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	payoutID := chi.URLParam(r, "payoutID")
	if payoutID == "" {
		writeError(w, http.StatusBadRequest, "payoutID required")
		return
	}
	var input struct {
		Reference string `json:"reference"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid body")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.UpdatePayout(r.Context(), payoutID, input.Reference); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *GoCardlessHandler) ListPayoutItemsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	payoutID := chi.URLParam(r, "payoutID")
	if payoutID == "" {
		writeError(w, http.StatusBadRequest, "payoutID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	items, err := prov.ListPayoutItems(r.Context(), payoutID)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"payout_items": items})
}

// --- Blocks ---

func (h *GoCardlessHandler) CreateBlockHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	var input struct {
		CustomerID string `json:"customer_id"`
		Reason     string `json:"reason"`
	}
	if err := decodeJSON(r, &input); err != nil || input.CustomerID == "" {
		writeError(w, http.StatusBadRequest, "customer_id required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	id, err := prov.CreateBlock(r.Context(), input.CustomerID, input.Reason)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"block_id": id})
}

func (h *GoCardlessHandler) GetBlockHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	blockID := chi.URLParam(r, "blockID")
	if blockID == "" {
		writeError(w, http.StatusBadRequest, "blockID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	info, err := prov.GetBlock(r.Context(), blockID)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *GoCardlessHandler) ListBlocksHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	customerID := r.URL.Query().Get("customer_id")
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	list, err := prov.ListBlocks(r.Context(), customerID)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"blocks": list})
}

func (h *GoCardlessHandler) DisableBlockHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	blockID := chi.URLParam(r, "blockID")
	if blockID == "" {
		writeError(w, http.StatusBadRequest, "blockID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.DisableBlock(r.Context(), blockID); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

func (h *GoCardlessHandler) EnableBlockHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	blockID := chi.URLParam(r, "blockID")
	if blockID == "" {
		writeError(w, http.StatusBadRequest, "blockID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.EnableBlock(r.Context(), blockID); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

// --- Events ---

func (h *GoCardlessHandler) ListEventsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	resourceType := r.URL.Query().Get("resource_type")
	action := r.URL.Query().Get("action")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := parseInt(l); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	list, err := prov.ListEvents(r.Context(), resourceType, action, limit)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"events": list})
}

func (h *GoCardlessHandler) GetEventHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	eventID := chi.URLParam(r, "eventID")
	if eventID == "" {
		writeError(w, http.StatusBadRequest, "eventID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	info, err := prov.GetEvent(r.Context(), eventID)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// --- Creditors ---

func (h *GoCardlessHandler) ListCreditorsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	list, err := prov.ListCreditors(r.Context())
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"creditors": list})
}

func (h *GoCardlessHandler) GetCreditorHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	creditorID := chi.URLParam(r, "creditorID")
	if creditorID == "" {
		writeError(w, http.StatusBadRequest, "creditorID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	info, err := prov.GetCreditor(r.Context(), creditorID)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// --- Webhooks management ---

func (h *GoCardlessHandler) ListWebhooksHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := parseInt(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	list, err := prov.ListWebhooks(r.Context(), limit)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"webhooks": list})
}

func (h *GoCardlessHandler) GetWebhookHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	webhookID := chi.URLParam(r, "webhookID")
	if webhookID == "" {
		writeError(w, http.StatusBadRequest, "webhookID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	info, err := prov.GetWebhook(r.Context(), webhookID)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *GoCardlessHandler) RetryWebhookHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	webhookID := chi.URLParam(r, "webhookID")
	if webhookID == "" {
		writeError(w, http.StatusBadRequest, "webhookID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	if err := prov.RetryWebhook(r.Context(), webhookID); err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "retried"})
}

// --- Scheme Identifiers ---

func (h *GoCardlessHandler) ListSchemeIdentifiersHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	list, err := prov.ListSchemeIdentifiers(r.Context())
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"scheme_identifiers": list})
}

func (h *GoCardlessHandler) GetSchemeIdentifierHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	schemeID := chi.URLParam(r, "schemeID")
	if schemeID == "" {
		writeError(w, http.StatusBadRequest, "schemeID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	info, err := prov.GetSchemeIdentifier(r.Context(), schemeID)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// --- Setup Direct Debit (full mandate creation) ---

func (h *GoCardlessHandler) SetupDirectDebitHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	var input struct {
		CustomerID        string `json:"customer_id"`
		AccountHolderName string `json:"account_holder_name"`
		AccountNumber     string `json:"account_number"`
		BSB               string `json:"bsb"`
		Reference         string `json:"reference"`
	}
	if err := decodeJSON(r, &input); err != nil || input.CustomerID == "" || input.AccountNumber == "" {
		writeError(w, http.StatusBadRequest, "customer_id and account_number required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	mandateID, err := prov.SetupDirectDebit(r.Context(), payment.SetupDirectDebitInput{
		CustomerID:        input.CustomerID,
		AccountHolderName: input.AccountHolderName,
		AccountNumber:     input.AccountNumber,
		BSB:               input.BSB,
		Reference:         input.Reference,
	})
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"mandate_id": mandateID})
}

// --- Payment Status ---

func (h *GoCardlessHandler) GetPaymentStatusHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	gcPaymentID := chi.URLParam(r, "paymentID")
	if gcPaymentID == "" {
		writeError(w, http.StatusBadRequest, "paymentID required")
		return
	}
	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}
	status, err := prov.GetPaymentStatus(r.Context(), gcPaymentID)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}
