package handlers

import (
	"log"
	"net/http"

	"clientroll/internal/middleware"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// MembershipTypeHandler handles membership type CRUD.
type MembershipTypeHandler struct {
	mtRepo    *repository.MembershipTypeRepo
	termsRepo *repository.TermsRepo
}

// NewMembershipTypeHandler creates a new handler.
func NewMembershipTypeHandler(mt *repository.MembershipTypeRepo, tr *repository.TermsRepo) *MembershipTypeHandler {
	return &MembershipTypeHandler{mtRepo: mt, termsRepo: tr}
}

// RegisterRoutes registers membership type routes.
func (h *MembershipTypeHandler) RegisterRoutes(r chi.Router) {
	r.Get("/membership-types", h.List)
	r.Post("/membership-types", h.Create)
	r.Get("/membership-types/{mtID}", h.Get)
	r.Post("/membership-types/{mtID}/duplicate", h.Duplicate)
	r.Post("/membership-types/{mtID}/term-options", h.CreateTermOption)
	r.Delete("/term-options/{toID}", h.DeleteTermOption)
	r.Post("/membership-types/{mtID}/terms/{termsID}", h.AttachTerms)
	r.Delete("/membership-types/{mtID}/terms/{termsID}", h.DetachTerms)
	r.Get("/membership-types/{mtID}/terms", h.GetTerms)
}

// List returns all membership types.
func (h *MembershipTypeHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	types, err := h.mtRepo.List(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error listing membership types: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list membership types")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"membership_types": types, "count": len(types)})
}

// Create creates a new membership type.
func (h *MembershipTypeHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	var input struct {
		Title       string  `json:"title"`
		Slug        string  `json:"slug"`
		Description *string `json:"description"`
		Inclusions  *string `json:"inclusions"`
		TopImageURL *string `json:"top_image_url"`
		TotalValue  float64 `json:"total_value"`
		SortOrder   int     `json:"sort_order"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Title == "" {
		writeError(w, http.StatusBadRequest, "Title is required")
		return
	}
	if input.Slug == "" {
		input.Slug = repository.GenerateSlug(input.Title)
	}
	mt, err := h.mtRepo.Create(r.Context(), tenantID, input.Title, input.Slug,
		input.Description, input.Inclusions, input.TopImageURL, input.TotalValue, input.SortOrder)
	if err != nil {
		log.Printf("Error creating membership type: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create membership type")
		return
	}
	writeJSON(w, http.StatusCreated, mt)
}

// Get returns a membership type with term options.
func (h *MembershipTypeHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	mtID, err := uuid.Parse(chi.URLParam(r, "mtID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid membership type ID")
		return
	}
	mt, err := h.mtRepo.FindByID(r.Context(), mtID, tenantID)
	if err != nil || mt == nil {
		writeError(w, http.StatusNotFound, "Membership type not found")
		return
	}
	writeJSON(w, http.StatusOK, mt)
}

// Duplicate deep-copies a membership type.
func (h *MembershipTypeHandler) Duplicate(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	mtID, err := uuid.Parse(chi.URLParam(r, "mtID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid membership type ID")
		return
	}
	mt, err := h.mtRepo.Duplicate(r.Context(), mtID, tenantID)
	if err != nil {
		log.Printf("Error duplicating membership type: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to duplicate membership type")
		return
	}
	writeJSON(w, http.StatusCreated, mt)
}

// CreateTermOption adds a term option to a membership type.
func (h *MembershipTypeHandler) CreateTermOption(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	mtID, err := uuid.Parse(chi.URLParam(r, "mtID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid membership type ID")
		return
	}
	var input struct {
		DurationMonths int      `json:"duration_months"`
		FeeTotal       float64  `json:"fee_total"`
		FeeWeekly      *float64 `json:"fee_weekly"`
		FeeFortnightly *float64 `json:"fee_fortnightly"`
		FeeMonthly     *float64 `json:"fee_monthly"`
		AllowUpfront   bool     `json:"allow_upfront"`
	}
	if err := decodeJSON(r, &input); err != nil || input.DurationMonths <= 0 {
		writeError(w, http.StatusBadRequest, "Duration months must be positive")
		return
	}
	to, err := h.mtRepo.CreateTermOption(r.Context(), tenantID, mtID,
		input.DurationMonths, input.FeeTotal, input.FeeWeekly,
		input.FeeFortnightly, input.FeeMonthly, input.AllowUpfront)
	if err != nil {
		log.Printf("Error creating term option: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create term option")
		return
	}
	writeJSON(w, http.StatusCreated, to)
}

// DeleteTermOption removes a term option.
func (h *MembershipTypeHandler) DeleteTermOption(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	toID, _ := uuid.Parse(chi.URLParam(r, "toID"))
	if err := h.mtRepo.DeleteTermOption(r.Context(), toID, tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete term option")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Term option deleted"})
}

// AttachTerms links a T&C to a membership type.
func (h *MembershipTypeHandler) AttachTerms(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	mtID, _ := uuid.Parse(chi.URLParam(r, "mtID"))
	termsID, _ := uuid.Parse(chi.URLParam(r, "termsID"))
	if err := h.termsRepo.AttachToMembershipType(r.Context(), tenantID, mtID, termsID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to attach terms")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Terms attached"})
}

// DetachTerms removes a T&C link from a membership type.
func (h *MembershipTypeHandler) DetachTerms(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	mtID, _ := uuid.Parse(chi.URLParam(r, "mtID"))
	termsID, _ := uuid.Parse(chi.URLParam(r, "termsID"))
	if err := h.termsRepo.DetachFromMembershipType(r.Context(), tenantID, mtID, termsID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to detach terms")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Terms detached"})
}

// GetTerms returns T&Cs attached to a membership type.
func (h *MembershipTypeHandler) GetTerms(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	mtID, _ := uuid.Parse(chi.URLParam(r, "mtID"))
	terms, err := h.termsRepo.GetForMembershipType(r.Context(), mtID, tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get terms")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"terms": terms})
}
