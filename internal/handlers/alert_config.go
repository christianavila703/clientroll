package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"clientroll/internal/middleware"
	"clientroll/internal/observability"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// AlertConfigHandler manages tenant alert configuration and reconciliation report endpoints.
type AlertConfigHandler struct {
	alertConfigRepo *repository.AlertConfigRepo
	reconRepo       *repository.ReconciliationRepo
	ownerAlerts     *observability.OwnerAlertService
}

func NewAlertConfigHandler(acr *repository.AlertConfigRepo, rr *repository.ReconciliationRepo, oa *observability.OwnerAlertService) *AlertConfigHandler {
	return &AlertConfigHandler{alertConfigRepo: acr, reconRepo: rr, ownerAlerts: oa}
}

func (h *AlertConfigHandler) RegisterRoutes(r chi.Router) {
	r.Get("/settings/alert-config", h.GetAlertConfig)
	r.Put("/settings/alert-config", h.UpdateAlertConfig)
	r.Get("/reports/payment-reconciliation", h.GetReconciliationReport)
	r.Get("/reports/expected-vs-received", h.GetExpectedVsReceived)
	r.Get("/reports/webhook-health", h.GetWebhookHealth)
	r.Get("/reports/payment-aging", h.GetPaymentAging)
}

func (h *AlertConfigHandler) GetAlertConfig(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant")
		return
	}
	cfg, err := h.alertConfigRepo.GetByTenant(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load alert config")
		return
	}
	if cfg == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"configured": false,
		})
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *AlertConfigHandler) UpdateAlertConfig(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant")
		return
	}
	var input repository.AlertConfig
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(input.AlertEmail) == "" {
		writeError(w, http.StatusBadRequest, "alert_email is required")
		return
	}
	cfg, err := h.alertConfigRepo.Upsert(r.Context(), tenantID, input)
	if err != nil {
		log.Printf("alert_config upsert failed tenant=%s err=%v", tenantID, err)
		writeError(w, http.StatusInternalServerError, "Failed to save alert config")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *AlertConfigHandler) GetReconciliationReport(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant")
		return
	}
	runs, err := h.reconRepo.ListByTenant(r.Context(), tenantID, 30)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load reconciliation report")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"runs": runs,
	})
}

func (h *AlertConfigHandler) GetExpectedVsReceived(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant")
		return
	}
	graceDaysCard, graceDaysDD := 1, 3
	cfg, _ := h.alertConfigRepo.GetByTenant(r.Context(), tenantID)
	if cfg != nil {
		graceDaysCard = cfg.GraceDaysCard
		graceDaysDD = cfg.GraceDaysDirectDebit
	}
	result, err := h.ownerAlerts.CheckExpectedVsReceived(r.Context(), tenantID, graceDaysCard, graceDaysDD)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to check expected vs received")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *AlertConfigHandler) GetWebhookHealth(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant")
		return
	}
	result, err := h.ownerAlerts.GetWebhookHealth(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to check webhook health")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *AlertConfigHandler) GetPaymentAging(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant")
		return
	}
	result, err := h.ownerAlerts.GetPaymentAging(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get payment aging")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
