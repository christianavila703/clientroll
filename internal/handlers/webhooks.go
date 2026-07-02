package handlers

import (
	"log"
	"net/http"

	"clientroll/internal/database"
	"clientroll/internal/events"
	"clientroll/internal/middleware"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// WebhookHandler handles webhook endpoint management.
type WebhookHandler struct {
	webhookRepo *repository.WebhookRepo
	encryptor   *database.Encryptor
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(wr *repository.WebhookRepo, enc *database.Encryptor) *WebhookHandler {
	return &WebhookHandler{webhookRepo: wr, encryptor: enc}
}

// RegisterRoutes registers webhook management routes.
func (h *WebhookHandler) RegisterRoutes(r chi.Router) {
	r.Get("/webhooks", h.ListEndpoints)
	r.Post("/webhooks", h.CreateEndpoint)
	r.Delete("/webhooks/{endpointID}", h.DeleteEndpoint)
	r.Put("/webhooks/{endpointID}/toggle", h.ToggleEndpoint)
	r.Get("/webhooks/{endpointID}/deliveries", h.ListDeliveries)
	r.Get("/webhooks/event-types", h.ListEventTypes)
}

// ListEndpoints returns all webhook endpoints for the tenant.
func (h *WebhookHandler) ListEndpoints(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))

	endpoints, err := h.webhookRepo.ListEndpoints(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error listing webhook endpoints: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list endpoints")
		return
	}

	// Don't expose encrypted secrets — only show masked versions
	type safeEndpoint struct {
		ID         uuid.UUID `json:"id"`
		URL        string    `json:"url"`
		EventTypes []string  `json:"event_types"`
		IsActive   bool      `json:"is_active"`
		CreatedAt  string    `json:"created_at"`
	}
	safe := make([]safeEndpoint, len(endpoints))
	for i, e := range endpoints {
		safe[i] = safeEndpoint{
			ID: e.ID, URL: e.URL, EventTypes: e.EventTypes,
			IsActive: e.IsActive, CreatedAt: e.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"endpoints": safe, "count": len(safe)})
}

// CreateEndpoint creates a new webhook endpoint with an auto-generated signing secret.
func (h *WebhookHandler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))

	var input struct {
		URL        string   `json:"url"`
		EventTypes []string `json:"event_types"`
	}
	if err := decodeJSON(r, &input); err != nil || input.URL == "" {
		writeError(w, http.StatusBadRequest, "URL is required")
		return
	}

	// Generate and encrypt the signing secret
	secret := events.GenerateWebhookSecret()
	encryptedSecret, err := h.encryptor.Encrypt(secret)
	if err != nil {
		log.Printf("Error encrypting webhook secret: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create endpoint")
		return
	}

	endpoint, err := h.webhookRepo.CreateEndpoint(r.Context(), tenantID, input.URL, encryptedSecret, input.EventTypes)
	if err != nil {
		log.Printf("Error creating webhook endpoint: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create endpoint")
		return
	}

	// Return the plain secret ONCE so the user can copy it — never shown again
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"endpoint": map[string]interface{}{
			"id":          endpoint.ID,
			"url":         endpoint.URL,
			"event_types": endpoint.EventTypes,
			"is_active":   endpoint.IsActive,
			"created_at":  endpoint.CreatedAt,
		},
		"signing_secret": secret, // Only returned on creation
	})
}

// DeleteEndpoint removes a webhook endpoint.
func (h *WebhookHandler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	endpointID, err := uuid.Parse(chi.URLParam(r, "endpointID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid endpoint ID")
		return
	}

	if err := h.webhookRepo.DeleteEndpoint(r.Context(), endpointID, tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete endpoint")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Endpoint deleted"})
}

// ToggleEndpoint enables or disables an endpoint.
func (h *WebhookHandler) ToggleEndpoint(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	endpointID, err := uuid.Parse(chi.URLParam(r, "endpointID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid endpoint ID")
		return
	}

	var input struct {
		IsActive bool `json:"is_active"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if err := h.webhookRepo.ToggleEndpoint(r.Context(), endpointID, tenantID, input.IsActive); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to toggle endpoint")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Endpoint updated"})
}

// ListDeliveries returns recent deliveries for an endpoint.
func (h *WebhookHandler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	endpointID, _ := uuid.Parse(chi.URLParam(r, "endpointID"))

	deliveries, err := h.webhookRepo.ListDeliveries(r.Context(), endpointID, tenantID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list deliveries")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deliveries": deliveries, "count": len(deliveries),
	})
}

// ListEventTypes returns all available event types.
func (h *WebhookHandler) ListEventTypes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"event_types": events.AllEventTypes(),
	})
}
