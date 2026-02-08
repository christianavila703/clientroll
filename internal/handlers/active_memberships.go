package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/middleware"
	"clientroll/internal/models"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ActiveMembershipsHandler serves the active memberships view.
type ActiveMembershipsHandler struct {
	db          *database.Postgres
	memberRepo  interface{ Create(ctx context.Context, tenantID, clientID, membershipTypeID, termOptionID uuid.UUID, startDate time.Time, paymentMethod, paymentFrequency string, paymentAmount float64, memberSignature string, staffUserID *uuid.UUID, staffSignature string) (*models.Membership, error) }
}

// NewActiveMembershipsHandler creates a new handler.
func NewActiveMembershipsHandler(db *database.Postgres, mr interface{ Create(ctx context.Context, tenantID, clientID, membershipTypeID, termOptionID uuid.UUID, startDate time.Time, paymentMethod, paymentFrequency string, paymentAmount float64, memberSignature string, staffUserID *uuid.UUID, staffSignature string) (*models.Membership, error) }) *ActiveMembershipsHandler {
	return &ActiveMembershipsHandler{db: db, memberRepo: mr}
}

// RegisterRoutes registers routes.
func (h *ActiveMembershipsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/active-memberships", h.List)
	r.Post("/memberships", h.CreateMembership)
}

// CreateMembership creates a membership for a client (staff-initiated).
func (h *ActiveMembershipsHandler) CreateMembership(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	userID, _ := uuid.Parse(middleware.GetUserID(r.Context()))

	var input struct {
		ClientID         string  `json:"client_id"`
		MembershipTypeID string  `json:"membership_type_id"`
		TermOptionID     string  `json:"term_option_id"`
		StartDate        string  `json:"start_date"`
		PaymentMethod    string  `json:"payment_method"`
		PaymentFrequency string  `json:"payment_frequency"`
		PaymentAmount    float64 `json:"payment_amount"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	clientID, _ := uuid.Parse(input.ClientID)
	mtID, _ := uuid.Parse(input.MembershipTypeID)
	toID, _ := uuid.Parse(input.TermOptionID)
	startDate, _ := time.Parse("2006-01-02", input.StartDate)

	m, err := h.memberRepo.Create(r.Context(), tenantID, clientID, mtID, toID,
		startDate, input.PaymentMethod, input.PaymentFrequency, input.PaymentAmount,
		"", &userID, "")
	if err != nil {
		log.Printf("Error creating membership: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create membership")
		return
	}

	writeJSON(w, http.StatusCreated, m)
}

// ActiveMembershipRow is the joined data for the active memberships view.
type ActiveMembershipRow struct {
	MembershipID     string   `json:"membership_id"`
	ClientID         string   `json:"client_id"`
	ClientName       string   `json:"client_name"`
	ClientEmail      *string  `json:"client_email"`
	ClientPhone      *string  `json:"client_phone"`
	MembershipType   string   `json:"membership_type"`
	TermMonths       int      `json:"term_months"`
	PaymentFrequency *string  `json:"payment_frequency"`
	PaymentAmount    *float64 `json:"payment_amount"`
	Status           string   `json:"status"`
	StartDate        string   `json:"start_date"`
	EndDate          string   `json:"end_date"`
	LastPaymentDate  *string  `json:"last_payment_date"`
	LastPaymentAmt   *float64 `json:"last_payment_amount"`
	LastPaymentStatus *string `json:"last_payment_status"`
	TotalPaid        float64  `json:"total_paid"`
	TotalExpected    float64  `json:"total_expected"`
	FailedCount      int      `json:"failed_count"`
}

// List returns all memberships with payment summary.
func (h *ActiveMembershipsHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	statusFilter := r.URL.Query().Get("status")
	if statusFilter == "" {
		statusFilter = "active"
	}

	query := `
		SELECT
			m.id::text,
			c.id::text,
			c.name,
			c.email,
			c.phone,
			mt.title,
			mto.duration_months,
			m.payment_frequency,
			m.payment_amount,
			m.status,
			TO_CHAR(m.start_date, 'YYYY-MM-DD'),
			TO_CHAR(m.min_term_end_date, 'YYYY-MM-DD'),
			(SELECT TO_CHAR(p.paid_date, 'YYYY-MM-DD') FROM payments p WHERE p.membership_id = m.id AND p.status = 'succeeded' ORDER BY p.paid_date DESC LIMIT 1),
			(SELECT p.amount FROM payments p WHERE p.membership_id = m.id AND p.status = 'succeeded' ORDER BY p.paid_date DESC LIMIT 1),
			(SELECT p.status FROM payments p WHERE p.membership_id = m.id ORDER BY p.created_at DESC LIMIT 1),
			COALESCE((SELECT SUM(p.amount) FROM payments p WHERE p.membership_id = m.id AND p.status = 'succeeded'), 0),
			m.payment_amount * mto.duration_months,
			(SELECT COUNT(*) FROM payments p WHERE p.membership_id = m.id AND p.status = 'failed')
		FROM memberships m
		JOIN clients c ON c.id = m.client_id
		JOIN membership_types mt ON mt.id = m.membership_type_id
		JOIN membership_term_options mto ON mto.id = m.term_option_id
		WHERE m.tenant_id = $1`

	args := []interface{}{tenantID}
	if statusFilter != "all" {
		query += " AND m.status = $2"
		args = append(args, statusFilter)
	}
	query += " ORDER BY m.start_date DESC"

	rows, err := h.db.Pool.Query(r.Context(), query, args...)
	if err != nil {
		log.Printf("Error listing active memberships: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list memberships")
		return
	}
	defer rows.Close()

	var results []ActiveMembershipRow
	for rows.Next() {
		var row ActiveMembershipRow
		if err := rows.Scan(
			&row.MembershipID, &row.ClientID, &row.ClientName, &row.ClientEmail, &row.ClientPhone,
			&row.MembershipType, &row.TermMonths, &row.PaymentFrequency,
			&row.PaymentAmount, &row.Status, &row.StartDate, &row.EndDate,
			&row.LastPaymentDate, &row.LastPaymentAmt, &row.LastPaymentStatus,
			&row.TotalPaid, &row.TotalExpected, &row.FailedCount,
		); err != nil {
			log.Printf("Error scanning active membership: %v", err)
			continue
		}
		results = append(results, row)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"memberships": results,
		"count":       len(results),
	})
}
