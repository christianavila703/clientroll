package handlers

import (
	"log"
	"net/http"

	"clientroll/internal/middleware"
	"clientroll/internal/models"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ClientHandler handles client CRUD endpoints.
type ClientHandler struct {
	clientRepo *repository.ClientRepo
}

// NewClientHandler creates a new client handler.
func NewClientHandler(cr *repository.ClientRepo) *ClientHandler {
	return &ClientHandler{clientRepo: cr}
}

// RegisterRoutes registers client routes.
func (h *ClientHandler) RegisterRoutes(r chi.Router) {
	r.Get("/clients", h.List)
	r.Post("/clients", h.Create)
	r.Get("/clients/{clientID}", h.Get)
	r.Put("/clients/{clientID}", h.Update)
	r.Get("/clients/{clientID}/memberships", h.GetMemberships)
}

// List returns all clients with optional search.
func (h *ClientHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	search := r.URL.Query().Get("search")

	clients, err := h.clientRepo.List(r.Context(), tenantID, search)
	if err != nil {
		log.Printf("Error listing clients: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list clients")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"clients": clients, "count": len(clients)})
}

// Create creates a new client.
func (h *ClientHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	var input models.Client
	if err := decodeJSON(r, &input); err != nil || input.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}
	client, err := h.clientRepo.Create(r.Context(), tenantID, &input)
	if err != nil {
		log.Printf("Error creating client: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create client")
		return
	}
	writeJSON(w, http.StatusCreated, client)
}

// Get returns a single client.
func (h *ClientHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	clientID, err := uuid.Parse(chi.URLParam(r, "clientID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}
	client, err := h.clientRepo.FindByID(r.Context(), clientID, tenantID)
	if err != nil || client == nil {
		writeError(w, http.StatusNotFound, "Client not found")
		return
	}
	writeJSON(w, http.StatusOK, client)
}

// Update updates a client.
func (h *ClientHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	clientID, err := uuid.Parse(chi.URLParam(r, "clientID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}
	var input models.Client
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	client, err := h.clientRepo.Update(r.Context(), clientID, tenantID, &input)
	if err != nil {
		log.Printf("Error updating client: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update client")
		return
	}
	writeJSON(w, http.StatusOK, client)
}

// GetMemberships returns all memberships for a client.
func (h *ClientHandler) GetMemberships(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	clientID, _ := uuid.Parse(chi.URLParam(r, "clientID"))
	memberships, err := h.clientRepo.GetMemberships(r.Context(), clientID, tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get memberships")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"memberships": memberships, "count": len(memberships)})
}
