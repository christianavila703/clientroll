package handlers

import (
	"log"
	"net/http"

	"clientroll/internal/middleware"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ServiceHandler struct {
	serviceRepo *repository.ServiceRepo
}

func NewServiceHandler(sr *repository.ServiceRepo) *ServiceHandler {
	return &ServiceHandler{serviceRepo: sr}
}

func (h *ServiceHandler) RegisterRoutes(r chi.Router) {
	r.Get("/services", h.List)
	r.Post("/services", h.Create)
	r.Put("/services/{serviceID}", h.Update)
	r.Delete("/services/{serviceID}", h.Delete)
	r.Post("/services/charge", h.TakePayment)
	r.Get("/services/charges/{clientID}", h.ListCharges)
}

func (h *ServiceHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	services, err := h.serviceRepo.List(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error listing services: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list services")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"services": services, "count": len(services)})
}

func (h *ServiceHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	var input struct {
		Name        string   `json:"name"`
		Description *string  `json:"description"`
		Price       float64  `json:"price"`
		TaxRate     float64  `json:"tax_rate"`
		SortOrder   int      `json:"sort_order"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}
	svc, err := h.serviceRepo.Create(r.Context(), tenantID, input.Name, input.Description, input.Price, input.TaxRate, input.SortOrder)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create service")
		return
	}
	writeJSON(w, http.StatusCreated, svc)
}

func (h *ServiceHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	svcID, _ := uuid.Parse(chi.URLParam(r, "serviceID"))
	var input struct {
		Name        string   `json:"name"`
		Description *string  `json:"description"`
		Price       float64  `json:"price"`
		TaxRate     float64  `json:"tax_rate"`
		IsActive    bool     `json:"is_active"`
		SortOrder   int      `json:"sort_order"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	svc, err := h.serviceRepo.Update(r.Context(), svcID, tenantID, input.Name, input.Description, input.Price, input.TaxRate, input.IsActive, input.SortOrder)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update service")
		return
	}
	writeJSON(w, http.StatusOK, svc)
}

func (h *ServiceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	svcID, _ := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err := h.serviceRepo.Delete(r.Context(), svcID, tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete service")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// TakePayment charges a client for a service (one-off payment).
func (h *ServiceHandler) TakePayment(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	userID, _ := uuid.Parse(middleware.GetUserID(r.Context()))
	var input struct {
		ClientID    string   `json:"client_id"`
		ServiceID   *string  `json:"service_id"`
		Description string   `json:"description"`
		Amount      float64  `json:"amount"`
		TaxAmount   float64  `json:"tax_amount"`
		Notes       *string  `json:"notes"`
	}
	if err := decodeJSON(r, &input); err != nil || input.ClientID == "" || input.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "Client ID and positive amount required")
		return
	}
	clientID, _ := uuid.Parse(input.ClientID)
	var serviceID *uuid.UUID
	if input.ServiceID != nil && *input.ServiceID != "" {
		sid, _ := uuid.Parse(*input.ServiceID)
		serviceID = &sid
	}
	charge, err := h.serviceRepo.CreateCharge(r.Context(), tenantID, clientID, serviceID, input.Description, input.Amount, input.TaxAmount, input.Notes, &userID)
	if err != nil {
		log.Printf("Error creating service charge: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create charge")
		return
	}
	writeJSON(w, http.StatusCreated, charge)
}

func (h *ServiceHandler) ListCharges(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	clientID, _ := uuid.Parse(chi.URLParam(r, "clientID"))
	charges, err := h.serviceRepo.ListChargesByClient(r.Context(), clientID, tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list charges")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"charges": charges, "count": len(charges)})
}
