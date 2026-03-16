package handlers

import (
	"log"
	"net/http"
	"regexp"
	"strings"

	"clientroll/internal/middleware"
	"clientroll/internal/models"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var tenantSlugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$`)

// SettingsHandler handles tenant settings endpoints.
type SettingsHandler struct {
	settingsRepo *repository.SettingsRepo
	tenantRepo   *repository.TenantRepo
}

// NewSettingsHandler creates a new settings handler.
func NewSettingsHandler(sr *repository.SettingsRepo, tr *repository.TenantRepo) *SettingsHandler {
	return &SettingsHandler{settingsRepo: sr, tenantRepo: tr}
}

// RegisterRoutes registers settings routes.
func (h *SettingsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/settings/agreement", h.GetAgreementSettings)
	r.Put("/settings/agreement", h.UpdateAgreementSettings)
	r.Get("/settings/tenant", h.GetTenantSettings)
	r.Put("/settings/tenant", h.UpdateTenantSettings)
	r.Post("/complete-setup", h.CompleteSetup)
}

// GetAgreementSettings returns the agreement settings (Section A).
func (h *SettingsHandler) GetAgreementSettings(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}

	settings, err := h.settingsRepo.GetAgreementSettings(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error getting agreement settings: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get settings")
		return
	}

	if settings == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{})
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// UpdateAgreementSettings updates the agreement settings (Section A).
func (h *SettingsHandler) UpdateAgreementSettings(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}

	var input repository.AgreementSettings
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	settings, err := h.settingsRepo.UpsertAgreementSettings(r.Context(), tenantID, input)
	if err != nil {
		log.Printf("Error updating agreement settings: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update settings")
		return
	}

	writeJSON(w, http.StatusOK, settings)
}

// GetTenantSettings returns the tenant's own configuration.
func (h *SettingsHandler) GetTenantSettings(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}

	tenant, err := h.tenantRepo.FindByID(r.Context(), tenantID)
	if err != nil || tenant == nil {
		writeError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	writeJSON(w, http.StatusOK, tenant)
}

// UpdateTenantSettings updates the tenant's own configuration.
func (h *SettingsHandler) UpdateTenantSettings(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}

	role := middleware.GetRole(r.Context())
	if role != "tenant_admin" {
		writeError(w, http.StatusForbidden, "Only tenant admins can update settings")
		return
	}

	var input struct {
		Timezone                   *string `json:"timezone,omitempty"`
		Slug                       *string `json:"slug,omitempty"`
		PrimaryColour              *string `json:"primary_colour,omitempty"`
		SecondaryColour            *string `json:"secondary_colour,omitempty"`
		SignupPriceDisplayMode     *string `json:"signup_price_display_mode,omitempty"`
		LogoURL                    *string `json:"logo_url,omitempty"`
		DefaultThankyouHeading     *string `json:"default_thankyou_heading,omitempty"`
		DefaultThankyouBody        *string `json:"default_thankyou_body,omitempty"`
		DefaultThankyouImageURL    *string `json:"default_thankyou_image_url,omitempty"`
		DefaultThankyouRedirectURL *string `json:"default_thankyou_redirect_url,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var nextSlug *string
	if input.Slug != nil {
		normalized := strings.ToLower(strings.TrimSpace(*input.Slug))
		if !tenantSlugRegex.MatchString(normalized) {
			writeError(w, http.StatusBadRequest, "Slug must be 3-50 characters, lowercase letters, numbers, and hyphens only")
			return
		}
		if existing, _ := h.tenantRepo.FindBySlug(r.Context(), normalized); existing != nil && existing.ID != tenantID {
			writeError(w, http.StatusConflict, "This URL is already taken. Please choose a different one.")
			return
		}
		nextSlug = &normalized
	}

	var nextDisplayMode *string
	if input.SignupPriceDisplayMode != nil {
		mode := strings.ToLower(strings.TrimSpace(*input.SignupPriceDisplayMode))
		if mode != "total" && mode != "weekly" && mode != "monthly" {
			writeError(w, http.StatusBadRequest, "signup_price_display_mode must be 'total', 'weekly', or 'monthly'")
			return
		}
		nextDisplayMode = &mode
	}

	updateInput := models.UpdateTenantInput{
		Timezone:                   input.Timezone,
		Slug:                       nextSlug,
		PrimaryColour:              input.PrimaryColour,
		SecondaryColour:            input.SecondaryColour,
		SignupPriceDisplayMode:     nextDisplayMode,
		LogoURL:                    input.LogoURL,
		DefaultThankyouHeading:     input.DefaultThankyouHeading,
		DefaultThankyouBody:        input.DefaultThankyouBody,
		DefaultThankyouImageURL:    input.DefaultThankyouImageURL,
		DefaultThankyouRedirectURL: input.DefaultThankyouRedirectURL,
	}

	tenant, err := h.tenantRepo.Update(r.Context(), tenantID, updateInput)
	if err != nil {
		log.Printf("Error updating tenant settings: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update settings")
		return
	}

	writeJSON(w, http.StatusOK, tenant)
}

// CompleteSetup marks the tenant's onboarding wizard as done.
func (h *SettingsHandler) CompleteSetup(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}

	tenant, err := h.tenantRepo.CompleteSetup(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error completing tenant setup: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to complete setup")
		return
	}

	writeJSON(w, http.StatusOK, tenant)
}
