package handlers

import (
	"log"
	"net/http"
	"time"

	"clientroll/internal/middleware"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// TermsHandler handles terms and conditions CRUD.
type TermsHandler struct {
	termsRepo  *repository.TermsRepo
	inviteRepo *repository.TermsReacceptanceInviteRepo
}

// NewTermsHandler creates a new terms handler.
func NewTermsHandler(tr *repository.TermsRepo) *TermsHandler {
	return &TermsHandler{termsRepo: tr}
}

// WithReacceptanceInvites wires the optional invite repo so that flagging a
// T&C as "requires re-acceptance" also issues a shareable signing link per
// affected membership.
func (h *TermsHandler) WithReacceptanceInvites(repo *repository.TermsReacceptanceInviteRepo) *TermsHandler {
	h.inviteRepo = repo
	return h
}

// RegisterRoutes registers terms routes.
func (h *TermsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/terms", h.List)
	r.Post("/terms", h.Create)
	r.Get("/terms/{termsID}", h.Get)
	r.Put("/terms/{termsID}", h.Update)
	r.Get("/terms/{termsID}/versions", h.ListVersions)
	r.Delete("/terms/{termsID}", h.Delete)
	r.Get("/memberships/{membershipID}/pending-reacceptance", h.PendingReacceptance)
	r.Post("/memberships/{membershipID}/terms/{termsID}/acknowledge", h.AcknowledgeReacceptance)
}

// List returns all T&C documents for the tenant.
func (h *TermsHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	terms, err := h.termsRepo.List(r.Context(), tenantID, false)
	if err != nil {
		log.Printf("Error listing terms: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list terms")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"terms": terms, "count": len(terms)})
}

// Create creates a new T&C document.
func (h *TermsHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	var input struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Title == "" || input.Content == "" {
		writeError(w, http.StatusBadRequest, "Title and content are required")
		return
	}
	terms, err := h.termsRepo.Create(r.Context(), tenantID, input.Title, input.Content)
	if err != nil {
		log.Printf("Error creating terms: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create terms")
		return
	}
	writeJSON(w, http.StatusCreated, terms)
}

// Get returns a single T&C document.
func (h *TermsHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	termsID, err := uuid.Parse(chi.URLParam(r, "termsID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid terms ID")
		return
	}
	terms, err := h.termsRepo.FindByID(r.Context(), termsID, tenantID)
	if err != nil || terms == nil {
		writeError(w, http.StatusNotFound, "Terms not found")
		return
	}
	writeJSON(w, http.StatusOK, terms)
}

// Update updates a T&C document (increments version).
func (h *TermsHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	termsID, err := uuid.Parse(chi.URLParam(r, "termsID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid terms ID")
		return
	}
	var input struct {
		Title                 string  `json:"title"`
		Content               string  `json:"content"`
		IsActive              bool    `json:"is_active"`
		ChangeNote            *string `json:"change_note,omitempty"`
		RequiresReacceptance  bool    `json:"requires_reacceptance,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	terms, err := h.termsRepo.Update(r.Context(), termsID, tenantID, input.Title, input.Content, input.IsActive, input.ChangeNote)
	if err != nil {
		log.Printf("Error updating terms: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update terms")
		return
	}
	flaggedMembers := int64(0)
	invitesIssued := 0
	if input.RequiresReacceptance {
		// Best-effort: failing to flag re-acceptance should not roll back
		// the successful T&C save. The admin can re-publish the same
		// version explicitly if they need to retry the flag.
		userID := middleware.GetUserID(r.Context())
		var triggeredBy *uuid.UUID
		if uid, err := uuid.Parse(userID); err == nil {
			triggeredBy = &uid
		}
		flagged, ferr := h.termsRepo.FlagReacceptanceRequired(r.Context(), tenantID, termsID, terms.Version, triggeredBy)
		if ferr != nil {
			log.Printf("Failed to flag re-acceptance terms=%s tenant=%s err=%v", termsID, tenantID, ferr)
		} else {
			flaggedMembers = flagged
		}
		// Issue one tokenised re-acceptance invite per flagged membership.
		// Failure here is logged but does not block the save — staff can
		// rerun by publishing the same version again.
		if h.inviteRepo != nil {
			issued, ierr := h.inviteRepo.IssueForFlaggedMemberships(
				r.Context(), tenantID, termsID, terms.Version, triggeredBy, reacceptanceInviteTTL,
			)
			if ierr != nil {
				log.Printf("Failed to issue re-acceptance invites terms=%s tenant=%s err=%v", termsID, tenantID, ierr)
			} else {
				invitesIssued = issued
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"terms":                terms,
		"reacceptance_flagged": flaggedMembers,
		"invites_issued":       invitesIssued,
	})
}

// reacceptanceInviteTTL controls how long a member has to follow the link
// after the admin flags the T&C. 90 days strikes a balance: long enough that
// the member can sign on their next business interaction, short enough that
// stale tokens age out of the active set.
const reacceptanceInviteTTL = 90 * 24 * time.Hour

// ListVersions returns the change history for a T&C document.
func (h *TermsHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	termsID, err := uuid.Parse(chi.URLParam(r, "termsID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid terms ID")
		return
	}
	versions, err := h.termsRepo.ListVersions(r.Context(), termsID, tenantID)
	if err != nil {
		log.Printf("Error listing T&C versions: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to load history")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"versions": versions})
}

// PendingReacceptance lists the outstanding re-acceptance requirements for
// one membership. Staff dashboards use this to flag a "needs re-acceptance"
// badge on the member's record; member portals (when they exist) will hit
// the same shape.
func (h *TermsHandler) PendingReacceptance(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	membershipID, err := uuid.Parse(chi.URLParam(r, "membershipID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid membership ID")
		return
	}
	pending, err := h.termsRepo.PendingReacceptanceForMembership(r.Context(), membershipID, tenantID)
	if err != nil {
		log.Printf("Error listing pending reacceptance: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to load pending re-acceptance")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"pending": pending})
}

// AcknowledgeReacceptance marks one (membership, terms_id) pair as
// re-accepted. Idempotent. Today this is staff-driven; a future member
// portal can call the same endpoint with the member's session.
func (h *TermsHandler) AcknowledgeReacceptance(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	membershipID, err := uuid.Parse(chi.URLParam(r, "membershipID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid membership ID")
		return
	}
	termsID, err := uuid.Parse(chi.URLParam(r, "termsID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid terms ID")
		return
	}
	if err := h.termsRepo.AcknowledgeReacceptance(r.Context(), membershipID, tenantID, termsID); err != nil {
		log.Printf("Error acknowledging reacceptance: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to acknowledge")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})
}

// Delete removes a T&C document.
func (h *TermsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	termsID, err := uuid.Parse(chi.URLParam(r, "termsID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid terms ID")
		return
	}
	if err := h.termsRepo.Delete(r.Context(), termsID, tenantID); err != nil {
		log.Printf("Error deleting terms: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete terms")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
