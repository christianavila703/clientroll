package handlers

import (
	"log"
	"net/http"

	"clientroll/internal/middleware"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// PaymentHandler handles payment tracking endpoints.
type PaymentHandler struct {
	paymentRepo *repository.PaymentRepo
}

// NewPaymentHandler creates a new payment handler.
func NewPaymentHandler(pr *repository.PaymentRepo) *PaymentHandler {
	return &PaymentHandler{paymentRepo: pr}
}

// RegisterRoutes registers payment routes.
func (h *PaymentHandler) RegisterRoutes(r chi.Router) {
	r.Get("/payments", h.List)
	r.Get("/payments/failed", h.ListFailed)
	r.Get("/payments/client/{clientID}", h.ListByClient)
}

// List returns all payments for the tenant.
func (h *PaymentHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	status := r.URL.Query().Get("status")
	limit := 100

	payments, err := h.paymentRepo.ListByTenant(r.Context(), tenantID, status, limit)
	if err != nil {
		log.Printf("Error listing payments: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list payments")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"payments": payments,
		"count":    len(payments),
	})
}

// ListFailed returns failed payments.
func (h *PaymentHandler) ListFailed(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))

	payments, err := h.paymentRepo.ListByTenant(r.Context(), tenantID, "failed", 0)
	if err != nil {
		log.Printf("Error listing failed payments: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list payments")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"payments": payments,
		"count":    len(payments),
	})
}

// ListByClient returns payments for a specific client.
func (h *PaymentHandler) ListByClient(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	clientID, err := uuid.Parse(chi.URLParam(r, "clientID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}

	payments, err := h.paymentRepo.ListByClient(r.Context(), clientID, tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list payments")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"payments": payments,
		"count":    len(payments),
	})
}
