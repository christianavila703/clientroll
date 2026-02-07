package handlers

import (
	"log"
	"net/http"

	"clientroll/internal/auth"
	"clientroll/internal/middleware"
	"clientroll/internal/models"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// StaffHandler handles tenant staff management.
type StaffHandler struct {
	userRepo *repository.UserRepo
}

// NewStaffHandler creates a new staff handler.
func NewStaffHandler(ur *repository.UserRepo) *StaffHandler {
	return &StaffHandler{userRepo: ur}
}

// RegisterRoutes registers staff management routes.
func (h *StaffHandler) RegisterRoutes(r chi.Router) {
	r.Get("/staff", h.ListStaff)
	r.Post("/staff", h.CreateStaff)
	r.Get("/staff/{userID}", h.GetStaff)
	r.Put("/staff/{userID}", h.UpdateStaff)
	r.Delete("/staff/{userID}", h.DeactivateStaff)
}

// ListStaff returns all staff for the tenant.
func (h *StaffHandler) ListStaff(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}

	users, err := h.userRepo.ListByTenant(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error listing staff: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list staff")
		return
	}

	responses := make([]models.UserResponse, len(users))
	for i, u := range users {
		responses[i] = u.ToResponse()
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"staff": responses,
		"count": len(responses),
	})
}

// CreateStaff creates a new staff member (admin only).
func (h *StaffHandler) CreateStaff(w http.ResponseWriter, r *http.Request) {
	role := middleware.GetRole(r.Context())
	if role != models.RoleTenantAdmin {
		writeError(w, http.StatusForbidden, "Only tenant admins can create staff")
		return
	}

	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}

	var input struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Role      string `json:"role"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.Email == "" || input.Password == "" || input.FirstName == "" {
		writeError(w, http.StatusBadRequest, "Email, password and first name are required")
		return
	}

	if input.Role != models.RoleTenantAdmin && input.Role != models.RoleTenantStaff {
		input.Role = models.RoleTenantStaff
	}

	if err := auth.ValidatePasswordStrength(input.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check email uniqueness within tenant
	existing, _ := h.userRepo.FindByEmailAndTenant(r.Context(), input.Email, tenantID)
	if existing != nil {
		writeError(w, http.StatusConflict, "Email already in use")
		return
	}

	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	user, err := h.userRepo.Create(r.Context(), tenantID,
		input.Email, hash, input.FirstName, input.LastName, input.Role,
	)
	if err != nil {
		log.Printf("Error creating staff: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	writeJSON(w, http.StatusCreated, user.ToResponse())
}

// GetStaff returns a single staff member.
func (h *StaffHandler) GetStaff(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid tenant context")
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), userID, tenantID)
	if err != nil || user == nil {
		writeError(w, http.StatusNotFound, "Staff member not found")
		return
	}

	writeJSON(w, http.StatusOK, user.ToResponse())
}

// UpdateStaff updates a staff member (admin only).
func (h *StaffHandler) UpdateStaff(w http.ResponseWriter, r *http.Request) {
	if middleware.GetRole(r.Context()) != models.RoleTenantAdmin {
		writeError(w, http.StatusForbidden, "Only tenant admins can update staff")
		return
	}

	writeError(w, http.StatusNotImplemented, "Staff update coming soon")
}

// DeactivateStaff deactivates a staff member (admin only).
func (h *StaffHandler) DeactivateStaff(w http.ResponseWriter, r *http.Request) {
	if middleware.GetRole(r.Context()) != models.RoleTenantAdmin {
		writeError(w, http.StatusForbidden, "Only tenant admins can deactivate staff")
		return
	}

	writeError(w, http.StatusNotImplemented, "Staff deactivation coming soon")
}
