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
	userRepo   *repository.UserRepo
	adminRepo  *repository.PlatformAdminRepo
	tenantRepo *repository.TenantRepo
	jwtManager *auth.JWTManager
	sessionMgr *auth.SessionManager
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

// RegisterTenantRoutes registers authenticated tenant-scoped auth routes.
func (h *AuthHandler) RegisterTenantRoutes(r chi.Router) {
	r.Post("/auth/change-password", h.ChangePassword)
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

	users, err := h.userRepo.ListActiveByEmail(r.Context(), input.Email)
	if err != nil {
		log.Printf("Login error: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal error")
		return
	}
	if len(users) == 0 {
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Resolve the right tenant user by password match among active tenant users.
	// This avoids false 401s when multiple tenants use the same email address.
	var matched *models.User
	lockedCount := 0
	for i := range users {
		user := users[i]
		if user.IsLocked() {
			lockedCount++
			continue
		}
		tenant, terr := h.tenantRepo.FindByID(r.Context(), user.TenantID)
		if terr != nil || tenant == nil || !tenant.IsActive {
			continue
		}
		if auth.CheckPassword(input.Password, user.PasswordHash) {
			matched = &user
			break
		}
	}
	if matched == nil {
		if lockedCount == len(users) {
			writeError(w, http.StatusTooManyRequests, "Account is temporarily locked. Try again later.")
			return
		}
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Generate tokens
	accessToken, refreshToken, tokenID, err := h.jwtManager.GenerateTokenPair(
		matched.ID, matched.TenantID, matched.Role,
	)
	if err != nil {
		log.Printf("Token generation error: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Store session in Redis
	if err := h.sessionMgr.CreateSession(r.Context(), matched.TenantID, matched.ID, matched.Role, tokenID); err != nil {
		log.Printf("Session creation error: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Record successful login
	h.userRepo.RecordLoginSuccess(r.Context(), matched.ID)

	writeJSON(w, http.StatusOK, models.LoginResponse{
		User: matched.ToResponse(),
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

// ChangePassword allows a logged-in user to change their own password.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(middleware.GetUserID(r.Context()))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	tenantID, err := uuid.Parse(middleware.GetTenantID(r.Context()))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if input.CurrentPassword == "" || input.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "Current password and new password are required")
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), userID, tenantID)
	if err != nil || user == nil {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}

	if !auth.CheckPassword(input.CurrentPassword, user.PasswordHash) {
		writeError(w, http.StatusForbidden, "Current password is incorrect")
		return
	}

	if err := auth.ValidatePasswordStrength(input.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	hash, err := auth.HashPassword(input.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update password")
		return
	}

	if err := h.userRepo.UpdatePassword(r.Context(), userID, tenantID, hash); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Password updated successfully"})
}
