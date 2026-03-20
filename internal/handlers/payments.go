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
	PaymentID         string  `json:"payment_id"`
	ClientID          string  `json:"client_id"`
	ClientName        string  `json:"client_name"`
	ClientEmail       *string `json:"client_email"`
	MembershipType    string  `json:"membership_type"`
	MembershipID      string  `json:"membership_id"`
	PaymentSource     string  `json:"payment_source"`
	Description       *string `json:"description"`
	TermMonths        int     `json:"term_months"`
	Amount            float64 `json:"amount"`
	BaseAmount        float64 `json:"base_amount"`
	TaxAmount         float64 `json:"tax_amount"`
	ProcessingFeeAmount float64 `json:"processing_fee_amount"`
	CurrencyCode      string  `json:"currency_code"`
	Status            string  `json:"status"`
	DueDate           string  `json:"due_date"`
	PaidDate          *string `json:"paid_date"`
	FailureReason     *string `json:"failure_reason"`
	RetryCount        int     `json:"retry_count"`
	ProviderReference *string `json:"provider_reference"`
	PaymentMethod     *string `json:"payment_method"`
	TotalPaid         float64 `json:"total_paid"`
	TotalExpected     float64 `json:"total_expected"`
	MembershipEnd     string  `json:"membership_end"`
	MembershipStatus  string  `json:"membership_status"`
	PaymentFrequency  *string `json:"payment_frequency"`
}

// List returns all payments for the tenant with enriched data.
func (h *PaymentHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	status := r.URL.Query().Get("status")

	query := `
		WITH all_rows AS (
			SELECT
				p.id::text AS payment_id,
				c.id::text AS client_id,
				c.name AS client_name,
				c.email AS client_email,
				mt.title AS membership_type,
				m.id::text AS membership_id,
				'membership' AS payment_source,
				NULL::text AS description,
				mto.duration_months AS term_months,
				p.amount AS amount,
				p.base_amount AS base_amount,
				p.tax_amount AS tax_amount,
				p.processing_fee_amount AS processing_fee_amount,
				p.currency_code AS currency_code,
				p.status AS status,
				TO_CHAR(p.due_date, 'YYYY-MM-DD') AS due_date,
				CASE WHEN p.paid_date IS NOT NULL THEN TO_CHAR(p.paid_date, 'YYYY-MM-DD') ELSE NULL END AS paid_date,
				p.failure_reason AS failure_reason,
				p.retry_count AS retry_count,
				p.provider_reference AS provider_reference,
				m.payment_method AS payment_method,
				COALESCE((SELECT SUM(p2.amount) FROM payments p2 WHERE p2.membership_id = m.id AND p2.status = 'succeeded'), 0) AS total_paid,
				membership_expected_total(
					COALESCE(m.payment_amount, 0),
					m.num_payments,
					m.payment_frequency,
					m.payment_interval_unit,
					m.payment_interval_count,
					m.start_date,
					m.min_term_end_date,
					mto.duration_months
				) AS total_expected,
				TO_CHAR(m.min_term_end_date, 'YYYY-MM-DD') AS membership_end,
				m.status AS membership_status,
				m.payment_frequency AS payment_frequency,
				p.created_at AS created_at
			FROM payments p
			JOIN clients c ON c.id = p.client_id
			JOIN memberships m ON m.id = p.membership_id
			JOIN membership_types mt ON mt.id = m.membership_type_id
			JOIN membership_term_options mto ON mto.id = m.term_option_id
			WHERE p.tenant_id = $1

			UNION ALL

			SELECT
				sc.id::text AS payment_id,
				c.id::text AS client_id,
				c.name AS client_name,
				c.email AS client_email,
				COALESCE(s.name, sc.description, 'Service Charge') AS membership_type,
				'' AS membership_id,
				'service' AS payment_source,
				sc.description AS description,
				0 AS term_months,
				(sc.amount + sc.tax_amount + sc.processing_fee_amount) AS amount,
				sc.amount AS base_amount,
				sc.tax_amount AS tax_amount,
				sc.processing_fee_amount AS processing_fee_amount,
				COALESCE(s.currency_code, 'AUD') AS currency_code,
				sc.status AS status,
				TO_CHAR(sc.created_at, 'YYYY-MM-DD') AS due_date,
				CASE WHEN sc.paid_date IS NOT NULL THEN TO_CHAR(sc.paid_date, 'YYYY-MM-DD') ELSE NULL END AS paid_date,
				NULL::text AS failure_reason,
				0 AS retry_count,
				NULL::text AS provider_reference,
				NULL::text AS payment_method,
				CASE WHEN sc.status = 'succeeded' THEN (sc.amount + sc.tax_amount + sc.processing_fee_amount) ELSE 0 END AS total_paid,
				(sc.amount + sc.tax_amount + sc.processing_fee_amount) AS total_expected,
				TO_CHAR(sc.created_at, 'YYYY-MM-DD') AS membership_end,
				sc.status AS membership_status,
				NULL::text AS payment_frequency,
				sc.created_at AS created_at
			FROM service_charges sc
			JOIN clients c ON c.id = sc.client_id
			LEFT JOIN services s ON s.id = sc.service_id
			WHERE sc.tenant_id = $1
		)
		SELECT
			payment_id, client_id, client_name, client_email,
			membership_type, membership_id, payment_source, description, term_months,
			amount, base_amount, tax_amount, processing_fee_amount, currency_code, status, due_date, paid_date,
			failure_reason, retry_count, provider_reference, payment_method,
			total_paid, total_expected, membership_end, membership_status, payment_frequency
		FROM all_rows
		WHERE 1=1`

	args := []interface{}{tenantID}
	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC LIMIT 200"

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
			&row.MembershipType, &row.MembershipID, &row.PaymentSource, &row.Description, &row.TermMonths,
			&row.Amount, &row.BaseAmount, &row.TaxAmount, &row.ProcessingFeeAmount, &row.CurrencyCode, &row.Status, &row.DueDate, &row.PaidDate,
			&row.FailureReason, &row.RetryCount,
			&row.ProviderReference, &row.PaymentMethod,
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
