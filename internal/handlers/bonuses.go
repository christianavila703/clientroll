package handlers

import (
	"log"
	"net/http"

	"clientroll/internal/middleware"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// BonusHandler handles bonus CRUD endpoints.
type BonusHandler struct {
	bonusRepo *repository.BonusRepo
}

// NewBonusHandler creates a new bonus handler.
func NewBonusHandler(br *repository.BonusRepo) *BonusHandler {
	return &BonusHandler{bonusRepo: br}
}

// RegisterRoutes registers bonus routes.
func (h *BonusHandler) RegisterRoutes(r chi.Router) {
	r.Get("/bonuses", h.List)
	r.Post("/bonuses", h.Create)
	r.Get("/bonuses/{bonusID}", h.Get)
	r.Put("/bonuses/{bonusID}", h.Update)
	r.Delete("/bonuses/{bonusID}", h.Delete)
	r.Put("/term-options/{termOptionID}/bonuses", h.SetTermOptionBonuses)
	r.Get("/term-options/{termOptionID}/bonuses", h.GetTermOptionBonuses)
}

// List returns all bonuses for the tenant.
func (h *BonusHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	bonuses, err := h.bonusRepo.List(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error listing bonuses: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list bonuses")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"bonuses": bonuses, "count": len(bonuses)})
}

// Create creates a new bonus.
func (h *BonusHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	var input struct {
		Title       string  `json:"title"`
		Description *string `json:"description"`
		ImageURL    *string `json:"image_url"`
		Value       float64 `json:"value"`
		SortOrder   int     `json:"sort_order"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Title == "" {
		writeError(w, http.StatusBadRequest, "Title is required")
		return
	}
	bonus, err := h.bonusRepo.Create(r.Context(), tenantID, input.Title, input.Description, input.ImageURL, input.Value, input.SortOrder)
	if err != nil {
		log.Printf("Error creating bonus: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create bonus")
		return
	}
	writeJSON(w, http.StatusCreated, bonus)
}

// Get returns a single bonus.
func (h *BonusHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	bonusID, err := uuid.Parse(chi.URLParam(r, "bonusID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid bonus ID")
		return
	}
	bonus, err := h.bonusRepo.FindByID(r.Context(), bonusID, tenantID)
	if err != nil || bonus == nil {
		writeError(w, http.StatusNotFound, "Bonus not found")
		return
	}
	writeJSON(w, http.StatusOK, bonus)
}

// Update updates a bonus.
func (h *BonusHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	bonusID, err := uuid.Parse(chi.URLParam(r, "bonusID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid bonus ID")
		return
	}
	var input struct {
		Title       string  `json:"title"`
		Description *string `json:"description"`
		ImageURL    *string `json:"image_url"`
		Value       float64 `json:"value"`
		IsActive    bool    `json:"is_active"`
		SortOrder   int     `json:"sort_order"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	bonus, err := h.bonusRepo.Update(r.Context(), bonusID, tenantID, input.Title, input.Description, input.ImageURL, input.Value, input.IsActive, input.SortOrder)
	if err != nil {
		log.Printf("Error updating bonus: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update bonus")
		return
	}
	writeJSON(w, http.StatusOK, bonus)
}

// Delete removes a bonus.
func (h *BonusHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	bonusID, _ := uuid.Parse(chi.URLParam(r, "bonusID"))
	if err := h.bonusRepo.Delete(r.Context(), bonusID, tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete bonus")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Bonus deleted"})
}

// SetTermOptionBonuses assigns bonuses to a term option.
func (h *BonusHandler) SetTermOptionBonuses(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	termOptionID, err := uuid.Parse(chi.URLParam(r, "termOptionID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid term option ID")
		return
	}
	var input struct {
		BonusIDs []uuid.UUID `json:"bonus_ids"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := h.bonusRepo.SetTermOptionBonuses(r.Context(), termOptionID, tenantID, input.BonusIDs); err != nil {
		log.Printf("Error setting term option bonuses: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to assign bonuses")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Bonuses assigned"})
}

// GetTermOptionBonuses returns bonuses for a term option.
func (h *BonusHandler) GetTermOptionBonuses(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	termOptionID, _ := uuid.Parse(chi.URLParam(r, "termOptionID"))
	bonuses, err := h.bonusRepo.GetBonusesForTermOption(r.Context(), termOptionID, tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get bonuses")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"bonuses": bonuses})
}
