package handlers

import (
	"log"
	"net/http"

	"clientroll/internal/middleware"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ReportsHandler handles financial reporting endpoints.
type ReportsHandler struct {
	reportsRepo *repository.ReportsRepo
}

// NewReportsHandler creates a new reports handler.
func NewReportsHandler(rr *repository.ReportsRepo) *ReportsHandler {
	return &ReportsHandler{reportsRepo: rr}
}

// RegisterRoutes registers reports routes.
func (h *ReportsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/reports/dashboard", h.Dashboard)
	r.Get("/reports/revenue/monthly", h.MonthlyRevenue)
	r.Get("/reports/memberships/breakdown", h.MembershipBreakdown)
	r.Get("/reports/payments/status", h.PaymentStatusBreakdown)
}

// Dashboard returns summary data for the dashboard.
func (h *ReportsHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))

	summary, err := h.reportsRepo.GetDashboardSummary(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error getting dashboard summary: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get dashboard data")
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// MonthlyRevenue returns monthly revenue for the past 12 months.
func (h *ReportsHandler) MonthlyRevenue(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))

	data, err := h.reportsRepo.GetMonthlyRevenue(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error getting monthly revenue: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get revenue data")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"months": data})
}

// MembershipBreakdown returns revenue per membership type.
func (h *ReportsHandler) MembershipBreakdown(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))

	data, err := h.reportsRepo.GetMembershipBreakdown(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error getting membership breakdown: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get breakdown data")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"breakdown": data})
}

// PaymentStatusBreakdown returns payment counts by status.
func (h *ReportsHandler) PaymentStatusBreakdown(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))

	data, err := h.reportsRepo.GetPaymentStatusBreakdown(r.Context(), tenantID)
	if err != nil {
		log.Printf("Error getting payment status breakdown: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get status data")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"statuses": data})
}
