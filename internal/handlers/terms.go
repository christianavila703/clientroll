package handlers

import (
	"log"
	"net/http"

	"clientroll/internal/middleware"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// TermsHandler handles terms and conditions CRUD.
type TermsHandler struct {
	termsRepo *repository.TermsRepo
}

// NewTermsHandler creates a new terms handler.
func NewTermsHandler(tr *repository.TermsRepo) *TermsHandler {
	return &TermsHandler{termsRepo: tr}
}

// RegisterRoutes registers terms routes.
func (h *TermsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/terms", h.List)
	r.Post("/terms", h.Create)
	r.Get("/terms/{termsID}", h.Get)
	r.Put("/terms/{termsID}", h.Update)
	r.Delete("/terms/{termsID}", h.Delete)
}

// List returns all T&C documents for the tenant.
func (h *TermsHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	terms, err := h.termsRepo.List(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error listing terms: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list terms")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"terms": terms, "count": len(terms)})
}

// Create creates a new T&C document.
func (h *TermsHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	var input struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Title == "" || input.Content == "" {
		writeError(w, http.StatusBadRequest, "Title and content are required")
		return
	}
	terms, err := h.termsRepo.Create(r.Context(), tenantID, input.Title, input.Content)
	if err != nil {
		log.Printf("Error creating terms: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create terms")
		return
	}
	writeJSON(w, http.StatusCreated, terms)
}

// Get returns a single T&C document.
func (h *TermsHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	termsID, err := uuid.Parse(chi.URLParam(r, "termsID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid terms ID")
		return
	}
	terms, err := h.termsRepo.FindByID(r.Context(), termsID, tenantID)
	if err != nil || terms == nil {
		writeError(w, http.StatusNotFound, "Terms not found")
		return
	}
	writeJSON(w, http.StatusOK, terms)
}

// Update updates a T&C document (increments version).
func (h *TermsHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	termsID, err := uuid.Parse(chi.URLParam(r, "termsID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid terms ID")
		return
	}
	var input struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		IsActive bool   `json:"is_active"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	terms, err := h.termsRepo.Update(r.Context(), termsID, tenantID, input.Title, input.Content, input.IsActive)
	if err != nil {
		log.Printf("Error updating terms: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update terms")
		return
	}
	writeJSON(w, http.StatusOK, terms)
}

// Delete removes a T&C document.
func (h *TermsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	termsID, err := uuid.Parse(chi.URLParam(r, "termsID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid terms ID")
		return
	}
	if err := h.termsRepo.Delete(r.Context(), termsID, tenantID); err != nil {
		log.Printf("Error deleting terms: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete terms")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
