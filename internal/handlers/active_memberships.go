package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/middleware"
	"clientroll/internal/payment"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ActiveMembershipsHandler serves the active memberships view.
type ActiveMembershipsHandler struct {
	db         *database.Postgres
	memberRepo *repository.MembershipRepo
	mtRepo     *repository.MembershipTypeRepo
	clientRepo *repository.ClientRepo
	credRepo   *repository.TenantCredentialRepo
	encryptor  *database.Encryptor
}

// NewActiveMembershipsHandler creates a new handler.
func NewActiveMembershipsHandler(db *database.Postgres, mr *repository.MembershipRepo, mtRepo *repository.MembershipTypeRepo, cr *repository.ClientRepo, credR *repository.TenantCredentialRepo, enc *database.Encryptor) *ActiveMembershipsHandler {
	return &ActiveMembershipsHandler{db: db, memberRepo: mr, mtRepo: mtRepo, clientRepo: cr, credRepo: credR, encryptor: enc}
}

// RegisterRoutes registers routes.
func (h *ActiveMembershipsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/active-memberships", h.List)
	r.Post("/memberships", h.CreateMembership)
	r.Post("/memberships/{membershipID}/cancel", h.CancelMembership)
}

// CreateMembership creates a membership for a client (staff-initiated).
// If bank details (bsb + account_number) are provided and GoCardless is configured,
// it also creates a GoCardless customer + mandate and links them.
func (h *ActiveMembershipsHandler) CreateMembership(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	userID, _ := uuid.Parse(middleware.GetUserID(r.Context()))

	var input struct {
		ClientID          string  `json:"client_id"`
		MembershipTypeID  string  `json:"membership_type_id"`
		TermOptionID      string  `json:"term_option_id"`
		StartDate         string  `json:"start_date"`
		PaymentMethod     string  `json:"payment_method"`
		PaymentFrequency  string  `json:"payment_frequency"`
		PaymentAmount     float64 `json:"payment_amount"`
		// Bank details for GoCardless mandate setup
		AccountHolderName string `json:"account_holder_name,omitempty"`
		AccountNumber     string `json:"account_number,omitempty"`
		BSB               string `json:"bsb,omitempty"`
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

	// --- GoCardless: create customer + mandate if bank details provided ---
	if input.BSB != "" && input.AccountNumber != "" {
		prov, provErr := h.getGoCardlessProvider(r.Context(), tenantID)
		if provErr == nil && prov != nil {
			client, _ := h.clientRepo.FindByID(r.Context(), clientID, tenantID)
			if client != nil {
				gcCustomerID := ""
				if client.GoCardlessCustomerID != nil && *client.GoCardlessCustomerID != "" {
					gcCustomerID = *client.GoCardlessCustomerID
				} else {
					clientEmail := ""
					if client.Email != nil {
						clientEmail = *client.Email
					}
					clientPhone := ""
					if client.Phone != nil {
						clientPhone = *client.Phone
					}
					cid, err := prov.CreateCustomer(r.Context(), payment.CreateCustomerInput{
						Email:     clientEmail,
						Name:      client.Name,
						Phone:     clientPhone,
						Reference: client.ID.String(),
					})
					if err != nil {
						log.Printf("GoCardless CreateCustomer (staff signup): %v", err)
					} else {
						gcCustomerID = cid
						_ = h.clientRepo.SetGoCardlessCustomerID(r.Context(), client.ID, tenantID, cid)
					}
				}
				if gcCustomerID != "" {
					holderName := input.AccountHolderName
					if holderName == "" {
						holderName = client.Name
					}
					mandateID, err := prov.SetupDirectDebit(r.Context(), payment.SetupDirectDebitInput{
						CustomerID:        gcCustomerID,
						AccountHolderName: holderName,
						AccountNumber:     input.AccountNumber,
						BSB:               input.BSB,
						Reference:         m.ID.String(),
					})
					if err != nil {
						log.Printf("GoCardless SetupDirectDebit (staff signup): %v", err)
					} else {
						_ = h.memberRepo.SetGoCardlessMandateID(r.Context(), m.ID, tenantID, mandateID)
						log.Printf("GoCardless mandate %s created for membership %s", mandateID, m.ID)

						// Create subscription for recurring payments (pass num_payments from term option if set)
						var numPay *int
						if to, _ := h.mtRepo.GetTermOption(r.Context(), toID, tenantID); to != nil && to.NumPayments != nil {
							numPay = to.NumPayments
						}
						createGoCardlessSubscription(r.Context(), prov, h.memberRepo, mandateID, m,
							tenantID, input.PaymentFrequency, input.PaymentAmount, "Membership", m.MinTermEndDate, numPay)
					}
				}
			}
		}
	}

	writeJSON(w, http.StatusCreated, m)
}

// CancelMembership cancels a membership and its GoCardless subscription+mandate.
func (h *ActiveMembershipsHandler) CancelMembership(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	membershipID, err := uuid.Parse(chi.URLParam(r, "membershipID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid membership ID")
		return
	}

	m, err := h.memberRepo.FindByID(r.Context(), membershipID, tenantID)
	if err != nil || m == nil {
		writeError(w, http.StatusNotFound, "Membership not found")
		return
	}

	// Cancel GoCardless subscription if exists
	if m.GoCardlessSubscriptionID != nil && *m.GoCardlessSubscriptionID != "" {
		prov, provErr := h.getGoCardlessProvider(r.Context(), tenantID)
		if provErr == nil && prov != nil {
			if err := prov.CancelSubscription(r.Context(), *m.GoCardlessSubscriptionID); err != nil {
				log.Printf("GoCardless CancelSubscription %s: %v", *m.GoCardlessSubscriptionID, err)
			}
		}
	}

	// Cancel GoCardless mandate if exists
	if m.GoCardlessMandateID != nil && *m.GoCardlessMandateID != "" {
		prov, provErr := h.getGoCardlessProvider(r.Context(), tenantID)
		if provErr == nil && prov != nil {
			if err := prov.CancelMandate(r.Context(), *m.GoCardlessMandateID); err != nil {
				log.Printf("GoCardless CancelMandate %s: %v", *m.GoCardlessMandateID, err)
			}
		}
	}

	// Update membership status
	if err := h.memberRepo.UpdateStatus(r.Context(), membershipID, tenantID, "cancelled"); err != nil {
		log.Printf("Error cancelling membership: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to cancel membership")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// getGoCardlessProvider returns a GoCardless provider for the tenant (if configured).
func (h *ActiveMembershipsHandler) getGoCardlessProvider(ctx context.Context, tenantID uuid.UUID) (payment.GoCardlessExtended, error) {
	if h.credRepo == nil || h.encryptor == nil {
		return nil, nil
	}
	creds, err := h.credRepo.GetByProvider(ctx, tenantID, "gocardless")
	if err != nil || creds == nil || creds.APIKeyEncrypted == "" {
		return nil, err
	}
	apiKey, err := h.encryptor.Decrypt(creds.APIKeyEncrypted)
	if err != nil {
		return nil, err
	}
	env := creds.Environment
	if env == "" {
		env = "sandbox"
	}
	return payment.NewGoCardlessProvider(apiKey, "", env), nil
}

// ActiveMembershipRow is the joined data for the active memberships view.
type ActiveMembershipRow struct {
	MembershipID      string   `json:"membership_id"`
	ClientID          string   `json:"client_id"`
	ClientName        string   `json:"client_name"`
	ClientEmail       *string  `json:"client_email"`
	ClientPhone       *string  `json:"client_phone"`
	MembershipType    string   `json:"membership_type"`
	TermMonths        int      `json:"term_months"`
	PaymentFrequency  *string  `json:"payment_frequency"`
	PaymentAmount     *float64 `json:"payment_amount"`
	PaymentMethod     *string  `json:"payment_method"`
	Status            string   `json:"status"`
	StartDate         string   `json:"start_date"`
	EndDate           string   `json:"end_date"`
	MandateID         *string  `json:"mandate_id"`
	SubscriptionID    *string  `json:"subscription_id"`
	LastPaymentDate   *string  `json:"last_payment_date"`
	LastPaymentAmt    *float64 `json:"last_payment_amount"`
	LastPaymentStatus *string  `json:"last_payment_status"`
	TotalPaid         float64  `json:"total_paid"`
	TotalExpected     float64  `json:"total_expected"`
	FailedCount       int      `json:"failed_count"`
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
			m.payment_method,
			m.status,
			TO_CHAR(m.start_date, 'YYYY-MM-DD'),
			TO_CHAR(m.min_term_end_date, 'YYYY-MM-DD'),
			m.gocardless_mandate_id,
			m.gocardless_subscription_id,
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
			&row.PaymentAmount, &row.PaymentMethod, &row.Status, &row.StartDate, &row.EndDate,
			&row.MandateID, &row.SubscriptionID,
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
