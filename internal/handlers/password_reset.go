package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"time"

	"clientroll/internal/auth"
	"clientroll/internal/database"
	"clientroll/internal/email"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// PasswordResetHandler handles forgot password and reset flows.
type PasswordResetHandler struct {
	userRepo  *repository.UserRepo
	db        *database.Postgres
	emailSvc  email.Provider
	appURL    string
}

// NewPasswordResetHandler creates a new password reset handler.
func NewPasswordResetHandler(
	ur *repository.UserRepo,
	db *database.Postgres,
	es email.Provider,
	appURL string,
) *PasswordResetHandler {
	return &PasswordResetHandler{userRepo: ur, db: db, emailSvc: es, appURL: appURL}
}

// RegisterRoutes registers password reset routes (public, rate limited).
func (h *PasswordResetHandler) RegisterRoutes(r chi.Router) {
	r.Post("/auth/forgot-password", h.ForgotPassword)
	r.Post("/auth/reset-password", h.ResetPassword)
}

// ForgotPassword sends a password reset email.
func (h *PasswordResetHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Email == "" {
		writeError(w, http.StatusBadRequest, "Email is required")
		return
	}

	// Always return success to prevent email enumeration
	defer writeJSON(w, http.StatusOK, map[string]string{
		"message": "If an account exists with that email, a password reset link has been sent.",
	})

	// Find user
	user, err := h.userRepo.FindByEmail(r.Context(), input.Email)
	if err != nil || user == nil {
		return // Silent — don't reveal whether email exists
	}

	// Rate limit: max 3 resets per hour per user
	var recentCount int
	h.db.Pool.QueryRow(r.Context(),
		"SELECT COUNT(*) FROM password_reset_tokens WHERE user_id=$1 AND created_at > NOW() - INTERVAL '1 hour'",
		user.ID).Scan(&recentCount)
	if recentCount >= 3 {
		return // Silent rate limit
	}

	// Generate token
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	rawToken := hex.EncodeToString(tokenBytes)
	tokenHash := hashToken(rawToken)

	// Store hashed token (expires in 1 hour)
	_, err = h.db.Pool.Exec(r.Context(),
		`INSERT INTO password_reset_tokens (user_id, tenant_id, token_hash, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		user.ID, user.TenantID, tokenHash, time.Now().Add(1*time.Hour))
	if err != nil {
		log.Printf("Error storing reset token: %v", err)
		return
	}

	// Send email
	resetURL := h.appURL + "/reset-password?token=" + rawToken
	emailInput := email.PasswordResetEmail(user.FirstName, resetURL)
	emailInput.To = user.Email

	_, err = h.emailSvc.Send(r.Context(), emailInput)
	if err != nil {
		log.Printf("Error sending reset email to %s: %v", user.Email, err)
	}
}

// ResetPassword sets a new password using a reset token.
func (h *PasswordResetHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Token == "" || input.Password == "" {
		writeError(w, http.StatusBadRequest, "Token and new password are required")
		return
	}

	// Validate password
	if err := auth.ValidatePasswordStrength(input.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tokenHash := hashToken(input.Token)

	// Find valid token
	var tokenID uuid.UUID
	var userID uuid.UUID
	err := h.db.Pool.QueryRow(r.Context(),
		`SELECT id, user_id FROM password_reset_tokens
		 WHERE token_hash=$1 AND used=false AND expires_at > NOW()`,
		tokenHash).Scan(&tokenID, &userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid or expired reset link. Please request a new one.")
		return
	}

	// Hash new password
	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update password")
		return
	}

	// Update password
	_, err = h.db.Pool.Exec(r.Context(),
		"UPDATE users SET password_hash=$1, failed_login_attempts=0, locked_until=NULL, updated_at=NOW() WHERE id=$2",
		hash, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update password")
		return
	}

	// Mark token as used
	h.db.Pool.Exec(r.Context(),
		"UPDATE password_reset_tokens SET used=true WHERE id=$1", tokenID)

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Password updated successfully. You can now sign in.",
	})
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
