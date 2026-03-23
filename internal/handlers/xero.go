package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"clientroll/internal/accounting/xero"
	"clientroll/internal/accounting/xerosync"
	"clientroll/internal/config"
	"clientroll/internal/database"
	"clientroll/internal/middleware"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type pendingXeroSelection struct {
	TenantID              uuid.UUID
	AccessTokenEncrypted  string
	RefreshTokenEncrypted string
	TokenExpiry           time.Time
	DefaultRevenueCode    string
	DefaultPaymentCode    string
	Connections           []xero.Connection
	ExpiresAt             time.Time
}

type XeroHandler struct {
	cfg            *config.Config
	encryptor      *database.Encryptor
	connectionRepo *repository.XeroConnectionRepo
	syncRepo       *repository.XeroSyncRepo
	paymentRepo    *repository.PaymentRepo
	serviceRepo    *repository.ServiceRepo
	authClient     *xero.AuthClient
	syncService    *xerosync.Service

	mu      sync.Mutex
	pending map[string]pendingXeroSelection
}

func NewXeroHandler(
	cfg *config.Config,
	enc *database.Encryptor,
	connectionRepo *repository.XeroConnectionRepo,
	syncRepo *repository.XeroSyncRepo,
	paymentRepo *repository.PaymentRepo,
	serviceRepo *repository.ServiceRepo,
	syncService *xerosync.Service,
) *XeroHandler {
	return &XeroHandler{
		cfg:            cfg,
		encryptor:      enc,
		connectionRepo: connectionRepo,
		syncRepo:       syncRepo,
		paymentRepo:    paymentRepo,
		serviceRepo:    serviceRepo,
		authClient:     xero.NewAuthClient(nil),
		syncService:    syncService,
		pending:        make(map[string]pendingXeroSelection),
	}
}

func (h *XeroHandler) RegisterTenantRoutes(r chi.Router) {
	r.Get("/xero/connect", h.ConnectURL)
	r.Get("/xero/status", h.GetStatus)
	r.Get("/xero/pending-selection", h.GetPendingSelection)
	r.Put("/xero/config", h.UpdateConfig)
	r.Post("/xero/disconnect", h.Disconnect)
	r.Post("/xero/select-org", h.SelectOrg)
	r.Get("/xero/sync-failures", h.ListTenantSyncFailures)
	r.Post("/xero/sync/retry", h.RetrySyncEvent)
}

func (h *XeroHandler) RegisterPublicRoutes(r chi.Router) {
	r.Get("/xero/oauth/callback", h.OAuthCallback)
}

func (h *XeroHandler) RegisterPlatformRoutes(r chi.Router) {
	r.Get("/xero/sync-failures", h.ListPlatformSyncFailures)
}

func (h *XeroHandler) ConnectURL(w http.ResponseWriter, r *http.Request) {
	if !requireTenantAdminForXero(w, r) {
		return
	}
	if !h.syncService.IsConfigured() {
		writeError(w, http.StatusBadRequest, "Xero OAuth is not configured")
		return
	}
	tenantIDStr := middleware.GetTenantID(r.Context())
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil || tenantID == uuid.Nil {
		writeError(w, http.StatusForbidden, "Tenant context required")
		return
	}

	nonce := uuid.New().String()
	stateData := tenantID.String() + "|" + nonce
	mac := hmac.New(sha256.New, []byte(h.cfg.MasterEncryptionKey))
	mac.Write([]byte(stateData))
	sig := hex.EncodeToString(mac.Sum(nil))
	state := base64.RawURLEncoding.EncodeToString([]byte(stateData + "|" + sig))
	redirectURI := h.redirectURI()
	redirectURL := xero.BuildAuthorizeURL(
		h.cfg.XeroClientID,
		redirectURI,
		state,
		[]string{"openid", "profile", "email", "accounting.contacts", "accounting.transactions", "offline_access"},
	)
	writeJSON(w, http.StatusOK, map[string]string{"redirect_url": redirectURL})
}

func (h *XeroHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if code == "" || state == "" {
		http.Redirect(w, r, h.settingsRedirect("xero=error_missing_params"), http.StatusFound)
		return
	}
	tenantID, ok := h.parseAndVerifyState(state)
	if !ok {
		http.Redirect(w, r, h.settingsRedirect("xero=error_invalid_state"), http.StatusFound)
		return
	}
	token, err := h.authClient.ExchangeCode(r.Context(), h.cfg.XeroClientID, h.cfg.XeroClientSecret, code, h.redirectURI())
	if err != nil {
		log.Printf("xero oauth exchange failed tenant=%s err=%v", tenantID, err)
		http.Redirect(w, r, h.settingsRedirect("xero=error_exchange_failed"), http.StatusFound)
		return
	}
	accessEnc, err := h.encryptor.Encrypt(token.AccessToken)
	if err != nil {
		http.Redirect(w, r, h.settingsRedirect("xero=error_encrypt_failed"), http.StatusFound)
		return
	}
	refreshEnc, err := h.encryptor.Encrypt(token.RefreshToken)
	if err != nil {
		http.Redirect(w, r, h.settingsRedirect("xero=error_encrypt_failed"), http.StatusFound)
		return
	}
	c := xero.NewClient(nil, token.AccessToken, "")
	connections, err := c.ListConnections(r.Context())
	if err != nil {
		log.Printf("xero connections fetch failed tenant=%s err=%v", tenantID, err)
		http.Redirect(w, r, h.settingsRedirect("xero=error_connections_failed"), http.StatusFound)
		return
	}
	if len(connections) == 0 {
		http.Redirect(w, r, h.settingsRedirect("xero=error_no_orgs"), http.StatusFound)
		return
	}
	if len(connections) == 1 {
		connName := connections[0].TenantName
		_, err := h.connectionRepo.UpsertByTenant(r.Context(), tenantID, repository.UpsertXeroConnectionInput{
			AccessTokenEncrypted:      accessEnc,
			RefreshTokenEncrypted:     refreshEnc,
			TokenExpiry:               token.Expiry(time.Now().UTC()),
			XeroTenantID:              connections[0].TenantID,
			XeroTenantName:            &connName,
			DefaultRevenueAccountCode: "200",
			DefaultPaymentAccountCode: "090",
			IsActive:                  true,
			NeedsReauth:               false,
		})
		if err != nil {
			log.Printf("xero connection save failed tenant=%s err=%v", tenantID, err)
			http.Redirect(w, r, h.settingsRedirect("xero=error_save_failed"), http.StatusFound)
			return
		}
		http.Redirect(w, r, h.settingsRedirect("xero=connected"), http.StatusFound)
		return
	}

	tokenID := uuid.New().String()
	h.mu.Lock()
	h.pending[tokenID] = pendingXeroSelection{
		TenantID:              tenantID,
		AccessTokenEncrypted:  accessEnc,
		RefreshTokenEncrypted: refreshEnc,
		TokenExpiry:           token.Expiry(time.Now().UTC()),
		DefaultRevenueCode:    "200",
		DefaultPaymentCode:    "090",
		Connections:           connections,
		ExpiresAt:             time.Now().UTC().Add(15 * time.Minute),
	}
	h.mu.Unlock()
	http.Redirect(w, r, h.settingsRedirect("xero=select_org&selection_token="+url.QueryEscape(tokenID)), http.StatusFound)
}

func (h *XeroHandler) SelectOrg(w http.ResponseWriter, r *http.Request) {
	if !requireTenantAdminForXero(w, r) {
		return
	}
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil || tenantID == uuid.Nil {
		writeError(w, http.StatusForbidden, "Tenant context required")
		return
	}
	var input struct {
		SelectionToken string `json:"selection_token"`
		XeroTenantID   string `json:"xero_tenant_id"`
	}
	if err := decodeJSON(r, &input); err != nil || input.SelectionToken == "" || input.XeroTenantID == "" {
		writeError(w, http.StatusBadRequest, "selection_token and xero_tenant_id are required")
		return
	}
	h.mu.Lock()
	pending, ok := h.pending[input.SelectionToken]
	if ok {
		delete(h.pending, input.SelectionToken)
	}
	h.mu.Unlock()
	if !ok || pending.TenantID != tenantID || time.Now().UTC().After(pending.ExpiresAt) {
		writeError(w, http.StatusBadRequest, "Selection token is invalid or expired")
		return
	}

	var chosen *xero.Connection
	for _, c := range pending.Connections {
		if c.TenantID == input.XeroTenantID {
			cc := c
			chosen = &cc
			break
		}
	}
	if chosen == nil {
		writeError(w, http.StatusBadRequest, "Selected Xero org is not authorized")
		return
	}

	name := chosen.TenantName
	_, err = h.connectionRepo.UpsertByTenant(r.Context(), tenantID, repository.UpsertXeroConnectionInput{
		AccessTokenEncrypted:      pending.AccessTokenEncrypted,
		RefreshTokenEncrypted:     pending.RefreshTokenEncrypted,
		TokenExpiry:               pending.TokenExpiry,
		XeroTenantID:              chosen.TenantID,
		XeroTenantName:            &name,
		DefaultRevenueAccountCode: pending.DefaultRevenueCode,
		DefaultPaymentAccountCode: pending.DefaultPaymentCode,
		IsActive:                  true,
		NeedsReauth:               false,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save selected Xero org")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Xero connected"})
}

func (h *XeroHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil || tenantID == uuid.Nil {
		writeError(w, http.StatusForbidden, "Tenant context required")
		return
	}
	conn, err := h.connectionRepo.GetByTenant(r.Context(), tenantID)
	if err != nil || conn == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"connected":            false,
			"is_active":            false,
			"feature_enabled":      h.cfg != nil && h.cfg.XeroSyncEnabled,
			"oauth_configured":     h.syncService != nil && h.syncService.IsConfigured(),
			"credit_notes_enabled": h.cfg != nil && h.cfg.XeroCreditNotesEnabled,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"connected":                    true,
		"is_active":                    conn.IsActive,
		"feature_enabled":              h.cfg != nil && h.cfg.XeroSyncEnabled,
		"oauth_configured":             h.syncService != nil && h.syncService.IsConfigured(),
		"credit_notes_enabled":         h.cfg != nil && h.cfg.XeroCreditNotesEnabled,
		"needs_reauth":                 conn.NeedsReauth,
		"xero_tenant_id":               conn.XeroTenantID,
		"xero_tenant_name":             conn.XeroTenantName,
		"default_revenue_account_code": conn.DefaultRevenueAccountCode,
		"default_payment_account_code": conn.DefaultPaymentAccountCode,
		"token_expiry":                 conn.TokenExpiry,
	})
}

func (h *XeroHandler) GetPendingSelection(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil || tenantID == uuid.Nil {
		writeError(w, http.StatusForbidden, "Tenant context required")
		return
	}
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}
	h.mu.Lock()
	p, ok := h.pending[token]
	h.mu.Unlock()
	if !ok || p.TenantID != tenantID || time.Now().UTC().After(p.ExpiresAt) {
		writeError(w, http.StatusNotFound, "Selection token not found or expired")
		return
	}
	type row struct {
		TenantID   string `json:"tenant_id"`
		TenantName string `json:"tenant_name"`
	}
	items := make([]row, 0, len(p.Connections))
	for _, c := range p.Connections {
		items = append(items, row{TenantID: c.TenantID, TenantName: c.TenantName})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "expires_at": p.ExpiresAt})
}

func (h *XeroHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	if !requireTenantAdminForXero(w, r) {
		return
	}
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil || tenantID == uuid.Nil {
		writeError(w, http.StatusForbidden, "Tenant context required")
		return
	}
	var input struct {
		DefaultRevenueAccountCode string `json:"default_revenue_account_code"`
		DefaultPaymentAccountCode string `json:"default_payment_account_code"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	input.DefaultRevenueAccountCode = strings.TrimSpace(input.DefaultRevenueAccountCode)
	input.DefaultPaymentAccountCode = strings.TrimSpace(input.DefaultPaymentAccountCode)
	if input.DefaultRevenueAccountCode == "" || input.DefaultPaymentAccountCode == "" {
		writeError(w, http.StatusBadRequest, "Both account codes are required")
		return
	}
	if err := h.connectionRepo.UpdateConfig(r.Context(), tenantID, input.DefaultRevenueAccountCode, input.DefaultPaymentAccountCode); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update Xero config")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Xero config updated"})
}

func (h *XeroHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	if !requireTenantAdminForXero(w, r) {
		return
	}
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil || tenantID == uuid.Nil {
		writeError(w, http.StatusForbidden, "Tenant context required")
		return
	}
	if err := h.connectionRepo.Deactivate(r.Context(), tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to disconnect Xero")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Xero disconnected"})
}

func (h *XeroHandler) ListTenantSyncFailures(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil || tenantID == uuid.Nil {
		writeError(w, http.StatusForbidden, "Tenant context required")
		return
	}
	rows, err := h.syncRepo.ListFailedEventsByTenant(r.Context(), tenantID, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list Xero sync failures")
		return
	}
	since := time.Now().UTC().Add(-24 * time.Hour)
	summary, _ := h.syncRepo.CountEventsByStatus(r.Context(), &tenantID, since)
	writeJSON(w, http.StatusOK, map[string]any{
		"items":       rows,
		"summary_24h": summary,
	})
}

func (h *XeroHandler) ListPlatformSyncFailures(w http.ResponseWriter, r *http.Request) {
	rows, err := h.syncRepo.ListFailedEvents(r.Context(), 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list Xero sync failures")
		return
	}
	since := time.Now().UTC().Add(-24 * time.Hour)
	summary, _ := h.syncRepo.CountEventsByStatus(r.Context(), nil, since)
	writeJSON(w, http.StatusOK, map[string]any{
		"items":       rows,
		"summary_24h": summary,
	})
}

func (h *XeroHandler) RetrySyncEvent(w http.ResponseWriter, r *http.Request) {
	if !requireTenantAdminForXero(w, r) {
		return
	}
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil || tenantID == uuid.Nil {
		writeError(w, http.StatusForbidden, "Tenant context required")
		return
	}
	var input struct {
		EventID string `json:"event_id"`
	}
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.EventID) == "" {
		writeError(w, http.StatusBadRequest, "event_id is required")
		return
	}
	eventID, err := uuid.Parse(strings.TrimSpace(input.EventID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid event_id")
		return
	}
	ev, err := h.syncRepo.GetEventByID(r.Context(), eventID)
	if err != nil || ev == nil || ev.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "Sync event not found")
		return
	}
	if err := h.retryEvent(r.Context(), ev); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Retry completed"})
}

func (h *XeroHandler) retryEvent(ctx context.Context, ev *repository.XeroSyncEvent) error {
	switch ev.EntityType {
	case repository.XeroEntityMembershipPayment:
		p, err := h.paymentRepo.GetByID(ctx, ev.EntityID, ev.TenantID)
		if err != nil || p == nil {
			return fmt.Errorf("payment not found for retry")
		}
		switch ev.Action {
		case "ensure_invoice":
			_, err = h.syncService.EnsureInvoiceForPayment(ctx, p)
			return err
		case "apply_payment":
			return h.syncService.ApplyPaymentForPayment(ctx, p)
		default:
			return fmt.Errorf("unsupported retry action")
		}
	case repository.XeroEntityServiceCharge:
		ch, err := h.serviceRepo.FindChargeByID(ctx, ev.TenantID, ev.EntityID)
		if err != nil || ch == nil {
			return fmt.Errorf("service charge not found for retry")
		}
		switch ev.Action {
		case "ensure_invoice":
			_, err = h.syncService.EnsureInvoiceForServiceCharge(ctx, ch)
			return err
		case "apply_payment":
			return h.syncService.ApplyPaymentForServiceCharge(ctx, ch)
		default:
			return fmt.Errorf("unsupported retry action")
		}
	default:
		return fmt.Errorf("unsupported retry entity")
	}
}

func (h *XeroHandler) redirectURI() string {
	if strings.TrimSpace(h.cfg.XeroRedirectURI) != "" {
		return strings.TrimSpace(h.cfg.XeroRedirectURI)
	}
	return strings.TrimSuffix(h.cfg.AppURL, "/") + "/api/xero/oauth/callback"
}

func (h *XeroHandler) settingsRedirect(query string) string {
	return strings.TrimSuffix(h.cfg.AppURL, "/") + "/settings?tab=integrations&" + query
}

func (h *XeroHandler) parseAndVerifyState(encoded string) (uuid.UUID, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return uuid.Nil, false
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 {
		return uuid.Nil, false
	}
	tenantID, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, false
	}
	stateData := parts[0] + "|" + parts[1]
	mac := hmac.New(sha256.New, []byte(h.cfg.MasterEncryptionKey))
	mac.Write([]byte(stateData))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return uuid.Nil, false
	}
	return tenantID, true
}

func requireTenantAdminForXero(w http.ResponseWriter, r *http.Request) bool {
	role := strings.TrimSpace(middleware.GetRole(r.Context()))
	if role != "tenant_admin" {
		writeError(w, http.StatusForbidden, "Only admins can manage Xero integration")
		return false
	}
	return true
}
