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

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	userRepo      *repository.UserRepo
	adminRepo     *repository.PlatformAdminRepo
	tenantRepo    *repository.TenantRepo
	jwtManager    *auth.JWTManager
	sessionMgr    *auth.SessionManager
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(
	ur *repository.UserRepo,
	ar *repository.PlatformAdminRepo,
	tr *repository.TenantRepo,
	jm *auth.JWTManager,
	sm *auth.SessionManager,
) *AuthHandler {
	return &AuthHandler{
		userRepo:   ur,
		adminRepo:  ar,
		tenantRepo: tr,
		jwtManager: jm,
		sessionMgr: sm,
	}
}

// RegisterRoutes registers auth routes on the router.
func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	r.Post("/auth/login", h.Login)
	r.Post("/auth/platform-login", h.PlatformLogin)
	r.Post("/auth/refresh", h.RefreshToken)
	r.Post("/auth/logout", h.Logout)
}

// Login handles tenant user login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input models.LoginInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.Email == "" || input.Password == "" {
		writeError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	// Find user by email (across all tenants -- email is unique per tenant)
	user, err := h.userRepo.FindByEmail(r.Context(), input.Email)
	if err != nil {
		log.Printf("Login error: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Check if account is locked
	if user.IsLocked() {
		writeError(w, http.StatusTooManyRequests, "Account is temporarily locked. Try again later.")
		return
	}

	// Verify tenant is active
	tenant, err := h.tenantRepo.FindByID(r.Context(), user.TenantID)
	if err != nil || tenant == nil || !tenant.IsActive {
		writeError(w, http.StatusUnauthorized, "Account is not available")
		return
	}

	// Verify password
	if !auth.CheckPassword(input.Password, user.PasswordHash) {
		h.userRepo.RecordLoginFailure(r.Context(), user.ID)
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Generate tokens
	accessToken, refreshToken, tokenID, err := h.jwtManager.GenerateTokenPair(
		user.ID, user.TenantID, user.Role,
	)
	if err != nil {
		log.Printf("Token generation error: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Store session in Redis
	if err := h.sessionMgr.CreateSession(r.Context(), user.TenantID, user.ID, user.Role, tokenID); err != nil {
		log.Printf("Session creation error: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Record successful login
	h.userRepo.RecordLoginSuccess(r.Context(), user.ID)

	writeJSON(w, http.StatusOK, models.LoginResponse{
		User: user.ToResponse(),
		Tokens: models.AuthTokens{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    h.jwtManager.AccessDurationSeconds(),
		},
	})
}

// PlatformLogin handles platform admin login.
func (h *AuthHandler) PlatformLogin(w http.ResponseWriter, r *http.Request) {
	var input models.LoginInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	admin, err := h.adminRepo.FindByEmail(r.Context(), input.Email)
	if err != nil {
		log.Printf("Platform login error: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	if admin == nil {
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	if !auth.CheckPassword(input.Password, admin.PasswordHash) {
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	accessToken, refreshToken, tokenID, err := h.jwtManager.GeneratePlatformTokenPair(admin.ID)
	if err != nil {
		log.Printf("Token generation error: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	if err := h.sessionMgr.CreateSession(r.Context(), uuid.Nil, admin.ID, models.RolePlatformAdmin, tokenID); err != nil {
		log.Printf("Session creation error: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user": map[string]interface{}{
			"id":    admin.ID,
			"email": admin.Email,
			"name":  admin.Name,
			"role":  models.RolePlatformAdmin,
		},
		"tokens": models.AuthTokens{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    h.jwtManager.AccessDurationSeconds(),
		},
	})
}

// RefreshToken exchanges a refresh token for a new token pair (rotation).
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSON(r, &input); err != nil || input.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "Refresh token is required")
		return
	}

	claims, err := h.jwtManager.ValidateToken(input.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}

	if claims.TokenType != auth.RefreshToken {
		writeError(w, http.StatusUnauthorized, "Invalid token type")
		return
	}

	// Verify old session exists
	tenantID := claims.TenantID
	if tenantID == "" {
		tenantID = "platform"
	}
	_, err = h.sessionMgr.ValidateSession(r.Context(), tenantID, claims.UserID, claims.TokenID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Session expired")
		return
	}

	// Revoke old session (rotation -- old refresh token can't be reused)
	h.sessionMgr.RevokeSession(r.Context(), tenantID, claims.UserID, claims.TokenID)

	// Generate new token pair
	userID, _ := uuid.Parse(claims.UserID)
	parsedTenantID, _ := uuid.Parse(claims.TenantID)

	var accessToken, refreshToken, tokenID string
	if claims.Role == models.RolePlatformAdmin {
		accessToken, refreshToken, tokenID, err = h.jwtManager.GeneratePlatformTokenPair(userID)
	} else {
		accessToken, refreshToken, tokenID, err = h.jwtManager.GenerateTokenPair(userID, parsedTenantID, claims.Role)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Create new session
	h.sessionMgr.CreateSession(r.Context(), parsedTenantID, userID, claims.Role, tokenID)

	writeJSON(w, http.StatusOK, models.AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    h.jwtManager.AccessDurationSeconds(),
	})
}

// Logout revokes the current session.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	userID := middleware.GetUserID(r.Context())
	tokenID := middleware.GetTokenID(r.Context())

	if tenantID == "" {
		tenantID = "platform"
	}

	if userID != "" && tokenID != "" {
		h.sessionMgr.RevokeSession(r.Context(), tenantID, userID, tokenID)
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Logged out"})
}
