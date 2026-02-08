package handlers

import (
	"log"
	"net/http"

	"clientroll/internal/database"
	"clientroll/internal/middleware"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// PaymentHandler handles payment tracking endpoints.
type PaymentHandler struct {
	paymentRepo *repository.PaymentRepo
	db          *database.Postgres
}

// NewPaymentHandler creates a new payment handler.
func NewPaymentHandler(pr *repository.PaymentRepo, db *database.Postgres) *PaymentHandler {
	return &PaymentHandler{paymentRepo: pr, db: db}
}

// RegisterRoutes registers payment routes.
func (h *PaymentHandler) RegisterRoutes(r chi.Router) {
	r.Get("/payments", h.List)
	r.Get("/payments/failed", h.ListFailed)
	r.Get("/payments/client/{clientID}", h.ListByClient)
}

// EnrichedPayment is a payment row with joined client/membership data.
type EnrichedPayment struct {
	PaymentID       string   `json:"payment_id"`
	ClientID        string   `json:"client_id"`
	ClientName      string   `json:"client_name"`
	ClientEmail     *string  `json:"client_email"`
	MembershipType  string   `json:"membership_type"`
	MembershipID    string   `json:"membership_id"`
	TermMonths      int      `json:"term_months"`
	Amount          float64  `json:"amount"`
	Status          string   `json:"status"`
	DueDate         string   `json:"due_date"`
	PaidDate        *string  `json:"paid_date"`
	FailureReason   *string  `json:"failure_reason"`
	RetryCount      int      `json:"retry_count"`
	TotalPaid       float64  `json:"total_paid"`
	TotalExpected   float64  `json:"total_expected"`
	MembershipEnd   string   `json:"membership_end"`
	MembershipStatus string  `json:"membership_status"`
	PaymentFrequency *string `json:"payment_frequency"`
}

// List returns all payments for the tenant with enriched data.
func (h *PaymentHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	status := r.URL.Query().Get("status")

	query := `
		SELECT
			p.id::text,
			c.id::text,
			c.name,
			c.email,
			mt.title,
			m.id::text,
			mto.duration_months,
			p.amount,
			p.status,
			TO_CHAR(p.due_date, 'YYYY-MM-DD'),
			CASE WHEN p.paid_date IS NOT NULL THEN TO_CHAR(p.paid_date, 'YYYY-MM-DD') ELSE NULL END,
			p.failure_reason,
			p.retry_count,
			COALESCE((SELECT SUM(p2.amount) FROM payments p2 WHERE p2.membership_id = m.id AND p2.status = 'succeeded'), 0),
			COALESCE(m.payment_amount, 0) * mto.duration_months,
			TO_CHAR(m.min_term_end_date, 'YYYY-MM-DD'),
			m.status,
			m.payment_frequency
		FROM payments p
		JOIN clients c ON c.id = p.client_id
		JOIN memberships m ON m.id = p.membership_id
		JOIN membership_types mt ON mt.id = m.membership_type_id
		JOIN membership_term_options mto ON mto.id = m.term_option_id
		WHERE p.tenant_id = $1`

	args := []interface{}{tenantID}
	if status != "" {
		query += " AND p.status = $2"
		args = append(args, status)
	}
	query += " ORDER BY p.created_at DESC LIMIT 200"

	rows, err := h.db.Pool.Query(r.Context(), query, args...)
	if err != nil {
		log.Printf("Error listing enriched payments: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list payments")
		return
	}
	defer rows.Close()

	var results []EnrichedPayment
	for rows.Next() {
		var row EnrichedPayment
		if err := rows.Scan(
			&row.PaymentID, &row.ClientID, &row.ClientName, &row.ClientEmail,
			&row.MembershipType, &row.MembershipID, &row.TermMonths,
			&row.Amount, &row.Status, &row.DueDate, &row.PaidDate,
			&row.FailureReason, &row.RetryCount,
			&row.TotalPaid, &row.TotalExpected,
			&row.MembershipEnd, &row.MembershipStatus, &row.PaymentFrequency,
		); err != nil {
			log.Printf("Error scanning enriched payment: %v", err)
			continue
		}
		results = append(results, row)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"payments": results,
		"count":    len(results),
	})
}

// ListFailed returns failed payments.
func (h *PaymentHandler) ListFailed(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))

	payments, err := h.paymentRepo.ListByTenant(r.Context(), tenantID, "failed", 0)
	if err != nil {
		log.Printf("Error listing failed payments: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list payments")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"payments": payments,
		"count":    len(payments),
	})
}

// ListByClient returns payments for a specific client.
func (h *PaymentHandler) ListByClient(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	clientID, err := uuid.Parse(chi.URLParam(r, "clientID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}

	payments, err := h.paymentRepo.ListByClient(r.Context(), clientID, tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list payments")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"payments": payments,
		"count":    len(payments),
	})
}
