package handlers

import (
	"log"
	"net/http"

	"clientroll/internal/auth"
	"clientroll/internal/models"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// PlatformHandler handles super-admin tenant management.
type PlatformHandler struct {
	tenantRepo *repository.TenantRepo
	userRepo   *repository.UserRepo
}

// NewPlatformHandler creates a new platform handler.
func NewPlatformHandler(tr *repository.TenantRepo, ur *repository.UserRepo) *PlatformHandler {
	return &PlatformHandler{tenantRepo: tr, userRepo: ur}
}

// RegisterRoutes registers platform routes.
func (h *PlatformHandler) RegisterRoutes(r chi.Router) {
	r.Get("/tenants", h.ListTenants)
	r.Post("/tenants", h.CreateTenant)
	r.Get("/tenants/{tenantID}", h.GetTenant)
	r.Put("/tenants/{tenantID}", h.UpdateTenant)
}

// ListTenants returns all tenants.
func (h *PlatformHandler) ListTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := h.tenantRepo.List(r.Context())
	if err != nil {
		log.Printf("Error listing tenants: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list tenants")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenants": tenants,
		"count":   len(tenants),
	})
}

// CreateTenant creates a new tenant with an initial admin user.
func (h *PlatformHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var input struct {
		models.CreateTenantInput
		AdminEmail     string `json:"admin_email"`
		AdminPassword  string `json:"admin_password"`
		AdminFirstName string `json:"admin_first_name"`
		AdminLastName  string `json:"admin_last_name"`
	}

	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.Name == "" || input.Slug == "" {
		writeError(w, http.StatusBadRequest, "Name and slug are required")
		return
	}
	if input.Timezone == "" {
		input.Timezone = "Australia/Brisbane"
	}
	if input.AdminEmail == "" || input.AdminPassword == "" {
		writeError(w, http.StatusBadRequest, "Admin email and password are required")
		return
	}

	// Check slug uniqueness
	existing, _ := h.tenantRepo.FindBySlug(r.Context(), input.Slug)
	if existing != nil {
		writeError(w, http.StatusConflict, "Tenant slug already taken")
		return
	}

	// Validate password strength
	if err := auth.ValidatePasswordStrength(input.AdminPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Create tenant
	tenant, err := h.tenantRepo.Create(r.Context(), input.CreateTenantInput)
	if err != nil {
		log.Printf("Error creating tenant: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create tenant")
		return
	}

	// Hash password and create admin user
	hash, err := auth.HashPassword(input.AdminPassword)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create admin user")
		return
	}

	user, err := h.userRepo.Create(r.Context(), tenant.ID,
		input.AdminEmail, hash, input.AdminFirstName, input.AdminLastName,
		models.RoleTenantAdmin,
	)
	if err != nil {
		log.Printf("Error creating admin user: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create admin user")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"tenant": tenant,
		"admin":  user.ToResponse(),
	})
}

// GetTenant returns a single tenant by ID.
func (h *PlatformHandler) GetTenant(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	tenant, err := h.tenantRepo.FindByID(r.Context(), id)
	if err != nil {
		log.Printf("Error fetching tenant: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch tenant")
		return
	}
	if tenant == nil {
		writeError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	writeJSON(w, http.StatusOK, tenant)
}

// UpdateTenant updates a tenant.
func (h *PlatformHandler) UpdateTenant(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant ID")
		return
	}

	var input models.UpdateTenantInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	tenant, err := h.tenantRepo.Update(r.Context(), id, input)
	if err != nil {
		log.Printf("Error updating tenant: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update tenant")
		return
	}

	writeJSON(w, http.StatusOK, tenant)
}
