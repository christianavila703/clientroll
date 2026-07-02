package handlers

import (
	"fmt"
	"log"
	"net/http"

	"clientroll/internal/middleware"
	pdfpkg "clientroll/internal/pdf"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AgreementHandler struct {
	agreementRepo *repository.AgreementSnapshotRepo
}

func NewAgreementHandler(ar *repository.AgreementSnapshotRepo) *AgreementHandler {
	return &AgreementHandler{agreementRepo: ar}
}

func (h *AgreementHandler) RegisterRoutes(r chi.Router) {
	r.Get("/memberships/{membershipID}/agreement", h.GetAgreement)
	r.Get("/memberships/{membershipID}/agreement.pdf", h.DownloadAgreementPDF)
}

func (h *AgreementHandler) GetAgreement(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	membershipID, err := uuid.Parse(chi.URLParam(r, "membershipID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid membership ID")
		return
	}

	snap, err := h.agreementRepo.FindByMembership(r.Context(), membershipID, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "No agreement found for this membership")
		return
	}

	writeJSON(w, http.StatusOK, snap)
}

func (h *AgreementHandler) DownloadAgreementPDF(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	membershipID, err := uuid.Parse(chi.URLParam(r, "membershipID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid membership ID")
		return
	}

	snap, err := h.agreementRepo.FindByMembership(r.Context(), membershipID, tenantID)
	if err != nil {
		log.Printf("agreement.pdf: snapshot not found membership=%s tenant=%s err=%v", membershipID, tenantID, err)
		writeError(w, http.StatusNotFound, "No agreement document found for this membership. The agreement may not have been captured during signup.")
		return
	}
	if snap.AgreementHTML == nil || *snap.AgreementHTML == "" {
		log.Printf("agreement.pdf: snapshot exists but agreement_html empty membership=%s tenant=%s snapshot=%s", membershipID, tenantID, snap.ID)
		writeError(w, http.StatusNotFound, "Agreement snapshot exists but the HTML content is empty. Please contact support.")
		return
	}

	pdfBytes, err := pdfpkg.HTMLToPDF(*snap.AgreementHTML)
	if err != nil {
		log.Printf("agreement.pdf: chromedp PDF generation failed membership=%s tenant=%s err=%v", membershipID, tenantID, err)
		writeError(w, http.StatusInternalServerError, "Failed to generate PDF. This usually means headless Chromium is not installed on the server.")
		return
	}

	filename := fmt.Sprintf("Agreement-%s-%s.pdf", snap.ClientName, snap.CreatedAt.Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
	w.WriteHeader(http.StatusOK)
	w.Write(pdfBytes)
}
