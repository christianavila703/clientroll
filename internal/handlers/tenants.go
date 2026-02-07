package handlers

import (
	"context"
	"log"
	"net/http"
	"regexp"
	"strings"

	"clientroll/internal/auth"
	"clientroll/internal/email"
	"clientroll/internal/models"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
)

// TenantRegistrationHandler handles public tenant self-registration.
type TenantRegistrationHandler struct {
	tenantRepo *repository.TenantRepo
	userRepo   *repository.UserRepo
	jwtManager *auth.JWTManager
	sessionMgr *auth.SessionManager
	emailSvc   email.Provider
	appURL     string
}

// NewTenantRegistrationHandler creates a new handler.
func NewTenantRegistrationHandler(
	tr *repository.TenantRepo,
	ur *repository.UserRepo,
	jm *auth.JWTManager,
	sm *auth.SessionManager,
	es email.Provider,
	appURL string,
) *TenantRegistrationHandler {
	return &TenantRegistrationHandler{
		tenantRepo: tr,
		userRepo:   ur,
		jwtManager: jm,
		sessionMgr: sm,
		emailSvc:   es,
		appURL:     appURL,
	}
}

// RegisterPublicRoutes registers public registration routes.
func (h *TenantRegistrationHandler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/register", h.Register)
	r.Get("/register/check-slug/{slug}", h.CheckSlug)
}

var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$`)

// Register handles new tenant self-registration.
func (h *TenantRegistrationHandler) Register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		// Company
		CompanyName string `json:"company_name"`
		Slug        string `json:"slug"`
		Timezone    string `json:"timezone"`
		// Admin user
		Email     string `json:"email"`
		Password  string `json:"password"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}

	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if input.CompanyName == "" || input.Email == "" || input.Password == "" || input.FirstName == "" {
		writeError(w, http.StatusBadRequest, "Company name, email, password, and first name are required")
		return
	}

	// Generate slug if not provided
	if input.Slug == "" {
		input.Slug = generateSlug(input.CompanyName)
	}
	input.Slug = strings.ToLower(strings.TrimSpace(input.Slug))

	if !slugRegex.MatchString(input.Slug) {
		writeError(w, http.StatusBadRequest, "Slug must be 3-50 characters, lowercase letters, numbers, and hyphens only")
		return
	}

	if input.Timezone == "" {
		input.Timezone = "Australia/Brisbane"
	}

	// Validate password strength
	if err := auth.ValidatePasswordStrength(input.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check slug availability
	existing, _ := h.tenantRepo.FindBySlug(r.Context(), input.Slug)
	if existing != nil {
		writeError(w, http.StatusConflict, "This URL is already taken. Please choose a different one.")
		return
	}

	// Create tenant
	tenant, err := h.tenantRepo.Create(r.Context(), models.CreateTenantInput{
		Name:     input.CompanyName,
		Slug:     input.Slug,
		Timezone: input.Timezone,
	})
	if err != nil {
		log.Printf("Error creating tenant: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create account")
		return
	}

	// Hash password
	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create account")
		return
	}

	// Create admin user
	user, err := h.userRepo.Create(r.Context(), tenant.ID,
		input.Email, hash, input.FirstName, input.LastName,
		models.RoleTenantAdmin,
	)
	if err != nil {
		log.Printf("Error creating admin user: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create admin user")
		return
	}

	// Auto-login: generate tokens
	accessToken, refreshToken, tokenID, err := h.jwtManager.GenerateTokenPair(
		user.ID, tenant.ID, user.Role,
	)
	if err != nil {
		log.Printf("Error generating tokens: %v", err)
		writeError(w, http.StatusInternalServerError, "Account created but login failed")
		return
	}

	// Store session
	h.sessionMgr.CreateSession(r.Context(), tenant.ID, user.ID, user.Role, tokenID)
	h.userRepo.RecordLoginSuccess(r.Context(), user.ID)

	// Send welcome email (async, don't block response)
	go func() {
		emailInput := email.WelcomeEmail(input.FirstName, input.CompanyName, h.appURL+"/dashboard")
		emailInput.To = input.Email
		if _, err := h.emailSvc.Send(context.Background(), emailInput); err != nil {
			log.Printf("Error sending welcome email to %s: %v", input.Email, err)
		}
	}()

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"tenant": tenant,
		"user":   user.ToResponse(),
		"tokens": models.AuthTokens{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    h.jwtManager.AccessDurationSeconds(),
		},
	})
}

// CheckSlug checks if a tenant slug is available.
func (h *TenantRegistrationHandler) CheckSlug(w http.ResponseWriter, r *http.Request) {
	slug := strings.ToLower(chi.URLParam(r, "slug"))

	if !slugRegex.MatchString(slug) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"available": false,
			"reason":    "Invalid format. Use lowercase letters, numbers, and hyphens.",
		})
		return
	}

	existing, _ := h.tenantRepo.FindBySlug(r.Context(), slug)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"slug":      slug,
		"available": existing == nil,
	})
}

func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	var result []byte
	for _, c := range []byte(slug) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result = append(result, c)
		}
	}
	s := string(result)
	// Trim trailing hyphens
	s = strings.Trim(s, "-")
	if len(s) < 3 {
		s = s + "-app"
	}
	return s
}
