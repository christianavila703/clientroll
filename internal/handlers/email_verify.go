package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/email"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// EmailVerifyHandler handles email verification for registration.
type EmailVerifyHandler struct {
	db       *database.Postgres
	emailSvc email.Provider
	appURL   string
}

// NewEmailVerifyHandler creates a new email verification handler.
func NewEmailVerifyHandler(db *database.Postgres, es email.Provider, appURL string) *EmailVerifyHandler {
	return &EmailVerifyHandler{db: db, emailSvc: es, appURL: appURL}
}

// RegisterRoutes registers public verification routes.
func (h *EmailVerifyHandler) RegisterRoutes(r chi.Router) {
	r.Get("/auth/verify-email", h.VerifyEmail)
	r.Post("/auth/resend-verification", h.ResendVerification)
}

// SendVerification creates a token and sends the verification email.
// Called internally after registration — not an HTTP handler.
func (h *EmailVerifyHandler) SendVerification(tenantID, userID uuid.UUID, userEmail, firstName string) error {
	ctx := context.Background()

	// Generate token
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	rawToken := hex.EncodeToString(tokenBytes)
	tokenHash := sha256Hash(rawToken)

	// Store (24hr expiry)
	_, err := h.db.Pool.Exec(ctx,
		`INSERT INTO user_email_verifications (tenant_id, user_id, email, token_hash, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		tenantID, userID, userEmail, tokenHash, time.Now().Add(24*time.Hour))
	if err != nil {
		return err
	}

	// Send email
	verifyURL := h.appURL + "/verify-email?token=" + rawToken
	emailInput := email.VerifyEmail(firstName, verifyURL)
	emailInput.To = userEmail

	_, err = h.emailSvc.Send(ctx, emailInput)
	return err
}

// VerifyEmail handles the verification link click.
func (h *EmailVerifyHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "Verification token is required")
		return
	}

	tokenHash := sha256Hash(token)

	// Find valid token
	var verifID uuid.UUID
	var userID uuid.UUID
	err := h.db.Pool.QueryRow(r.Context(),
		`SELECT id, user_id FROM user_email_verifications
		 WHERE token_hash=$1 AND verified=false AND expires_at > NOW()`,
		tokenHash).Scan(&verifID, &userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid or expired verification link. Please request a new one.")
		return
	}

	// Mark verified
	_, err = h.db.Pool.Exec(r.Context(),
		`UPDATE user_email_verifications SET verified=true, verified_at=NOW() WHERE id=$1`,
		verifID)
	if err != nil {
		log.Printf("Error marking email verified: %v", err)
		writeError(w, http.StatusInternalServerError, "Verification failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"verified": true,
		"message":  "Email verified successfully. You can now sign in.",
	})
}

// ResendVerification sends a new verification email.
func (h *EmailVerifyHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Email == "" {
		writeError(w, http.StatusBadRequest, "Email is required")
		return
	}

	// Always return success to prevent enumeration
	defer writeJSON(w, http.StatusOK, map[string]string{
		"message": "If an unverified account exists, a new verification email has been sent.",
	})

	// Find unverified record
	var tenantID, userID uuid.UUID
	var firstName string
	err := h.db.Pool.QueryRow(r.Context(),
		`SELECT ev.tenant_id, ev.user_id, u.first_name
		 FROM user_email_verifications ev
		 JOIN users u ON u.id = ev.user_id
		 WHERE ev.email=$1 AND ev.verified=false
		 ORDER BY ev.created_at DESC LIMIT 1`,
		input.Email).Scan(&tenantID, &userID, &firstName)
	if err != nil {
		return
	}

	// Rate limit: max 3 per hour
	var count int
	h.db.Pool.QueryRow(r.Context(),
		"SELECT COUNT(*) FROM user_email_verifications WHERE user_id=$1 AND created_at > NOW() - INTERVAL '1 hour'",
		userID).Scan(&count)
	if count >= 3 {
		return
	}

	go func() {
		if err := h.SendVerification(tenantID, userID, input.Email, firstName); err != nil {
			log.Printf("Error resending verification to %s: %v", input.Email, err)
		}
	}()
}

// IsEmailVerified checks if a user's email has been verified.
func (h *EmailVerifyHandler) IsEmailVerified(userID uuid.UUID) bool {
	var verified bool
	err := h.db.Pool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM user_email_verifications WHERE user_id=$1 AND verified=true)",
		userID).Scan(&verified)
	if err != nil {
		return false
	}
	return verified
}

func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
