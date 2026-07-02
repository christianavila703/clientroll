package handlers

import (
	"log"
	"net/http"
	"strings"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/middleware"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ReacceptanceHandler serves the public T&C re-acceptance flow and the
// staff-side links panel. Public endpoints are tokenised — possession of
// the token UUID grants the bearer the right to view + sign the named T&C
// for the named membership. No member login is required.
//
// The staff-side endpoint lives on the tenant router so admins can see
// every outstanding link for a member (e.g. to copy/share via SMS).
type ReacceptanceHandler struct {
	tenantRepo *repository.TenantRepo
	termsRepo  *repository.TermsRepo
	inviteRepo *repository.TermsReacceptanceInviteRepo
	db         *database.Postgres
}

// NewReacceptanceHandler wires the dependencies. db is used to set the
// per-request RLS tenant context on the public path (where there is no
// authenticated user).
func NewReacceptanceHandler(
	tr *repository.TenantRepo,
	terms *repository.TermsRepo,
	inv *repository.TermsReacceptanceInviteRepo,
	db *database.Postgres,
) *ReacceptanceHandler {
	return &ReacceptanceHandler{tenantRepo: tr, termsRepo: terms, inviteRepo: inv, db: db}
}

// RegisterPublicRoutes mounts the unauthenticated, tokenised re-acceptance
// routes under the existing /signup tree so the public DNS surface stays the
// same as the new-signup flow.
func (h *ReacceptanceHandler) RegisterPublicRoutes(r chi.Router) {
	r.Get("/signup/{tenantSlug}/reaccept/{token}", h.GetReacceptance)
	r.Post("/signup/{tenantSlug}/reaccept/{token}", h.SubmitReacceptance)
}

// RegisterTenantRoutes mounts the staff-facing endpoints for listing /
// re-issuing invite links for a membership.
func (h *ReacceptanceHandler) RegisterTenantRoutes(r chi.Router) {
	r.Get("/memberships/{membershipID}/reacceptance-links", h.ListLinks)
}

// GetReacceptance returns the rich payload the public page needs to render
// (member, org, T&C content, prior version pointer for diff). Returns:
//   - 404 if the token does not exist or does not match the URL tenant
//   - 410 Gone if the invite was already completed
//   - 410 Gone if the invite expired
//   - 200 with the detail payload otherwise
func (h *ReacceptanceHandler) GetReacceptance(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")
	tokenRaw := strings.TrimSpace(chi.URLParam(r, "token"))
	if tenantSlug == "" || tokenRaw == "" {
		writeError(w, http.StatusBadRequest, "Invalid link")
		return
	}
	token, err := uuid.Parse(tokenRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid link")
		return
	}
	tenant, err := h.tenantRepo.FindBySlug(r.Context(), tenantSlug)
	if err != nil || tenant == nil {
		writeError(w, http.StatusNotFound, "Tenant not found")
		return
	}
	if err := EnsureSignupTenantDBContext(r.Context(), h.db, tenant.ID); err != nil {
		log.Printf("GetReacceptance EnsureSignupTenantDBContext failed tenant=%s err=%v", tenant.ID, err)
		writeError(w, http.StatusInternalServerError, "Server error")
		return
	}
	detail, err := h.inviteRepo.FindDetailByToken(r.Context(), token)
	if err != nil {
		log.Printf("GetReacceptance lookup error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to load invite")
		return
	}
	if detail == nil || detail.Invite.TenantID != tenant.ID {
		writeError(w, http.StatusNotFound, "Link not found")
		return
	}
	if detail.Invite.CompletedAt != nil {
		writeJSON(w, http.StatusGone, map[string]interface{}{
			"error":         "Link already used",
			"completed_at":  detail.Invite.CompletedAt,
			"tenant_slug":   detail.TenantSlug,
			"org_name":      detail.OrgName,
			"client_name":   strings.TrimSpace(detail.ClientFirstName + " " + detail.ClientLastName),
		})
		return
	}
	if time.Now().UTC().After(detail.Invite.ExpiresAt) {
		writeJSON(w, http.StatusGone, map[string]interface{}{
			"error":         "Link expired",
			"expires_at":    detail.Invite.ExpiresAt,
			"tenant_slug":   detail.TenantSlug,
			"org_name":      detail.OrgName,
		})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// SubmitReacceptance records the member's signature, calls the existing
// AcknowledgeReacceptance to clear the pending flag, and marks the invite
// completed. Re-submitting the same token returns 410 Gone.
func (h *ReacceptanceHandler) SubmitReacceptance(w http.ResponseWriter, r *http.Request) {
	tenantSlug := chi.URLParam(r, "tenantSlug")
	tokenRaw := strings.TrimSpace(chi.URLParam(r, "token"))
	if tenantSlug == "" || tokenRaw == "" {
		writeError(w, http.StatusBadRequest, "Invalid link")
		return
	}
	token, err := uuid.Parse(tokenRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid link")
		return
	}
	tenant, err := h.tenantRepo.FindBySlug(r.Context(), tenantSlug)
	if err != nil || tenant == nil {
		writeError(w, http.StatusNotFound, "Tenant not found")
		return
	}
	if err := EnsureSignupTenantDBContext(r.Context(), h.db, tenant.ID); err != nil {
		log.Printf("SubmitReacceptance EnsureSignupTenantDBContext failed tenant=%s err=%v", tenant.ID, err)
		writeError(w, http.StatusInternalServerError, "Server error")
		return
	}

	var input struct {
		SignatureTyped     string `json:"signature_typed"`
		SignatureImageData string `json:"signature_image_data"`
		Confirmed          bool   `json:"confirmed"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if !input.Confirmed {
		writeError(w, http.StatusBadRequest, "Confirmation required")
		return
	}
	typed := strings.TrimSpace(input.SignatureTyped)
	if typed == "" {
		writeError(w, http.StatusBadRequest, "Please type your name to sign")
		return
	}

	detail, err := h.inviteRepo.FindDetailByToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load invite")
		return
	}
	if detail == nil || detail.Invite.TenantID != tenant.ID {
		writeError(w, http.StatusNotFound, "Link not found")
		return
	}
	if detail.Invite.CompletedAt != nil {
		writeError(w, http.StatusGone, "Link already used")
		return
	}
	if time.Now().UTC().After(detail.Invite.ExpiresAt) {
		writeError(w, http.StatusGone, "Link expired")
		return
	}

	// 1. Mark the membership_reacceptance_required row as acknowledged
	//    (idempotent — running twice keeps the earlier acknowledged_at).
	if err := h.termsRepo.AcknowledgeReacceptance(r.Context(), detail.Invite.MembershipID, tenant.ID, detail.Invite.TermsID); err != nil {
		log.Printf("SubmitReacceptance AcknowledgeReacceptance err=%v", err)
		writeError(w, http.StatusInternalServerError, "Failed to record acceptance")
		return
	}

	// 2. Stamp the invite as completed, capturing the signature inputs +
	//    audit fields (IP / UA) for the legal record. These are kept in the
	//    invite row itself rather than in agreement_snapshots because a
	//    re-acceptance is a delta on top of the original signed agreement,
	//    not a fresh contract.
	ip := clientIP(r)
	ua := r.Header.Get("User-Agent")
	var imagePtr *string
	if strings.TrimSpace(input.SignatureImageData) != "" {
		imgVal := input.SignatureImageData
		imagePtr = &imgVal
	}
	if err := h.inviteRepo.MarkCompleted(r.Context(), detail.Invite.ID, &typed, imagePtr, &ip, &ua); err != nil {
		log.Printf("SubmitReacceptance MarkCompleted err=%v", err)
		writeError(w, http.StatusInternalServerError, "Failed to store signature")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":         "accepted",
		"completed_at":   time.Now().UTC().Format(time.RFC3339),
		"terms_id":       detail.Invite.TermsID,
		"terms_version":  detail.Invite.TermsVersion,
		"membership_id":  detail.Invite.MembershipID,
	})
}

// ListLinks returns every re-acceptance invite (pending + completed) for one
// membership, so the staff UI can show "outstanding signatures" and copy the
// shareable URL.
func (h *ReacceptanceHandler) ListLinks(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	membershipID, err := uuid.Parse(chi.URLParam(r, "membershipID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid membership ID")
		return
	}
	rows, err := h.inviteRepo.ListByMembership(r.Context(), membershipID, tenantID)
	if err != nil {
		log.Printf("ListLinks err=%v", err)
		writeError(w, http.StatusInternalServerError, "Failed to load links")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"links": rows})
}

// clientIP is a best-effort extraction. We don't depend on it for auth —
// the token already authorises the request — so X-Forwarded-For is fine
// even though it could be spoofed.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}
	return r.RemoteAddr
}
