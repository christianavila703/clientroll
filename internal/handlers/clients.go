package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/email"
	"clientroll/internal/events"
	"clientroll/internal/middleware"
	"clientroll/internal/models"
	"clientroll/internal/payment"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ClientHandler handles client CRUD endpoints.
type ClientHandler struct {
	clientRepo       *repository.ClientRepo
	memberRepo       *repository.MembershipRepo
	mtRepo           *repository.MembershipTypeRepo
	credRepo         *repository.TenantCredentialRepo
	redirectFlowRepo *repository.RedirectFlowSessionRepo
	encryptor        *database.Encryptor
	emailSvc         email.Provider
	dispatcher       *events.Dispatcher
}

// NewClientHandler creates a new client handler.
func NewClientHandler(cr *repository.ClientRepo, mr *repository.MembershipRepo, mtRepo *repository.MembershipTypeRepo, credR *repository.TenantCredentialRepo, rfr *repository.RedirectFlowSessionRepo, enc *database.Encryptor, es email.Provider) *ClientHandler {
	return &ClientHandler{clientRepo: cr, memberRepo: mr, mtRepo: mtRepo, credRepo: credR, redirectFlowRepo: rfr, encryptor: enc, emailSvc: es}
}

// SetDispatcher sets optional outbound webhook event dispatcher.
func (h *ClientHandler) SetDispatcher(dispatcher *events.Dispatcher) {
	h.dispatcher = dispatcher
}

// RegisterRoutes registers client routes.
func (h *ClientHandler) RegisterRoutes(r chi.Router) {
	r.Get("/clients", h.List)
	r.Post("/clients", h.Create)
	r.Get("/clients/{clientID}", h.Get)
	r.Put("/clients/{clientID}", h.Update)
	r.Delete("/clients/{clientID}", h.Delete)
	r.Post("/clients/bulk-delete", h.BulkDelete)
	r.Get("/clients/{clientID}/memberships", h.GetMemberships)
	r.Get("/clients/{clientID}/mandate-status", h.GetMandateStatus)
	r.Post("/clients/{clientID}/send-mandate-link", h.SendMandateLink)
	r.Post("/clients/{clientID}/complete-mandate-link", h.CompleteMandateLink)
}

// List returns all clients with optional search and status filter.
func (h *ClientHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")

	clients, err := h.clientRepo.List(r.Context(), tenantID, search, status)
	if err != nil {
		log.Printf("Error listing clients: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list clients")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"clients": clients, "count": len(clients)})
}

// Create creates a new client.
func (h *ClientHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))

	var input struct {
		Name                  *string `json:"name"`
		UnitNumber            *string `json:"unit_number"`
		Address               *string `json:"address"`
		Suburb                *string `json:"suburb"`
		State                 *string `json:"state"`
		Postcode              *string `json:"postcode"`
		Email                 *string `json:"email"`
		Phone                 *string `json:"phone"`
		BirthDate             *string `json:"birth_date"`
		Gender                *string `json:"gender"`
		EmergencyContactName  *string `json:"emergency_contact_name"`
		EmergencyContactPhone *string `json:"emergency_contact_phone"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	normalize := func(v *string) *string {
		if v == nil {
			return nil
		}
		t := strings.TrimSpace(*v)
		if t == "" {
			return nil
		}
		return &t
	}
	required := func(v *string) string {
		if v == nil {
			return ""
		}
		return strings.TrimSpace(*v)
	}

	var birthDate *time.Time
	if bd := normalize(input.BirthDate); bd != nil {
		parsed, parseErr := time.Parse(time.RFC3339, *bd)
		if parseErr != nil {
			if d, dayErr := time.Parse("2006-01-02", *bd); dayErr == nil {
				parsed = d
			} else {
				writeError(w, http.StatusBadRequest, "Invalid request body")
				return
			}
		}
		birthDate = &parsed
	}

	create := &models.Client{
		Name:                  required(input.Name),
		UnitNumber:            normalize(input.UnitNumber),
		Address:               normalize(input.Address),
		Suburb:                normalize(input.Suburb),
		State:                 normalize(input.State),
		Postcode:              normalize(input.Postcode),
		Email:                 normalize(input.Email),
		Phone:                 normalize(input.Phone),
		BirthDate:             birthDate,
		Gender:                normalize(input.Gender),
		EmergencyContactName:  normalize(input.EmergencyContactName),
		EmergencyContactPhone: normalize(input.EmergencyContactPhone),
	}

	if create.Name == "" ||
		create.Address == nil ||
		create.Suburb == nil ||
		create.State == nil ||
		create.Postcode == nil ||
		create.Email == nil ||
		create.Phone == nil ||
		create.BirthDate == nil ||
		create.Gender == nil ||
		create.EmergencyContactName == nil ||
		create.EmergencyContactPhone == nil {
		writeError(w, http.StatusBadRequest, "All client fields are required")
		return
	}
	client, err := h.clientRepo.Create(r.Context(), tenantID, create)
	if err != nil {
		log.Printf("Error creating client: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create client")
		return
	}
	if h.dispatcher != nil {
		h.dispatcher.Dispatch(r.Context(), events.NewEvent(events.ClientCreated, tenantID, map[string]interface{}{
			"client_id": client.ID,
			"name":      client.Name,
			"email":     client.Email,
			"phone":     client.Phone,
		}))
	}
	writeJSON(w, http.StatusCreated, client)
}

// Get returns a single client.
func (h *ClientHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	clientID, err := uuid.Parse(chi.URLParam(r, "clientID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}
	client, err := h.clientRepo.FindByID(r.Context(), clientID, tenantID)
	if err != nil || client == nil {
		writeError(w, http.StatusNotFound, "Client not found")
		return
	}
	writeJSON(w, http.StatusOK, client)
}

// Update updates a client.
func (h *ClientHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	clientID, err := uuid.Parse(chi.URLParam(r, "clientID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}

	var input struct {
		Name                  *string `json:"name"`
		UnitNumber            *string `json:"unit_number"`
		Address               *string `json:"address"`
		Suburb                *string `json:"suburb"`
		State                 *string `json:"state"`
		Postcode              *string `json:"postcode"`
		Email                 *string `json:"email"`
		Phone                 *string `json:"phone"`
		BirthDate             *string `json:"birth_date"`
		Gender                *string `json:"gender"`
		EmergencyContactName  *string `json:"emergency_contact_name"`
		EmergencyContactPhone *string `json:"emergency_contact_phone"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	normalize := func(v *string) *string {
		if v == nil {
			return nil
		}
		t := strings.TrimSpace(*v)
		if t == "" {
			return nil
		}
		return &t
	}

	var birthDate *time.Time
	if bd := normalize(input.BirthDate); bd != nil {
		parsed, parseErr := time.Parse(time.RFC3339, *bd)
		if parseErr != nil {
			if d, dayErr := time.Parse("2006-01-02", *bd); dayErr == nil {
				parsed = d
			} else {
				writeError(w, http.StatusBadRequest, "Invalid request body")
				return
			}
		}
		birthDate = &parsed
	}

	update := &models.Client{
		Name: strings.TrimSpace(func() string {
			if input.Name == nil {
				return ""
			}
			return *input.Name
		}()),
		UnitNumber:            normalize(input.UnitNumber),
		Address:               normalize(input.Address),
		Suburb:                normalize(input.Suburb),
		State:                 normalize(input.State),
		Postcode:              normalize(input.Postcode),
		Email:                 normalize(input.Email),
		Phone:                 normalize(input.Phone),
		BirthDate:             birthDate,
		Gender:                normalize(input.Gender),
		EmergencyContactName:  normalize(input.EmergencyContactName),
		EmergencyContactPhone: normalize(input.EmergencyContactPhone),
	}
	if update.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	client, err := h.clientRepo.Update(r.Context(), clientID, tenantID, update)
	if err != nil {
		log.Printf("Error updating client: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update client")
		return
	}
	writeJSON(w, http.StatusOK, client)
}

// Delete soft-deletes a client.
func (h *ClientHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	clientID, err := uuid.Parse(chi.URLParam(r, "clientID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}

	if err := h.clientRepo.SoftDelete(r.Context(), clientID, tenantID); err != nil {
		log.Printf("Error deleting client: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete client")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "deleted"})
}

// BulkDelete soft-deletes multiple clients.
func (h *ClientHandler) BulkDelete(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	var input struct {
		ClientIDs []string `json:"client_ids"`
	}
	if err := decodeJSON(r, &input); err != nil || len(input.ClientIDs) == 0 {
		writeError(w, http.StatusBadRequest, "client_ids is required")
		return
	}

	ids := make([]uuid.UUID, 0, len(input.ClientIDs))
	for _, s := range input.ClientIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid client ID")
			return
		}
		ids = append(ids, id)
	}

	deleted, err := h.clientRepo.SoftDeleteMany(r.Context(), tenantID, ids)
	if err != nil {
		log.Printf("Error bulk deleting clients: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete clients")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "deleted", "deleted": deleted})
}

// ClientMembershipRow is the enriched membership data for the client profile.
type ClientMembershipRow struct {
	MembershipID         string   `json:"membership_id"`
	MembershipType       string   `json:"membership_type"`
	MembershipTypeID     string   `json:"membership_type_id"`
	TermMonths           int      `json:"term_months"`
	StartDate            string   `json:"start_date"`
	EndDate              string   `json:"end_date"`
	PaymentMethod        *string  `json:"payment_method"`
	PaymentFrequency     *string  `json:"payment_frequency"`
	PaymentIntervalUnit  *string  `json:"payment_interval_unit"`
	PaymentIntervalCount int      `json:"payment_interval_count"`
	PaymentAmount        *float64 `json:"payment_amount"`
	NumPayments          *int     `json:"num_payments"`
	Status               string   `json:"status"`
	MandateID            *string  `json:"mandate_id"`
	SubscriptionID       *string  `json:"subscription_id"`
	TotalPaid            float64  `json:"total_paid"`
	TotalExpected        float64  `json:"total_expected"`
	FailedCount          int      `json:"failed_count"`
	LastPaymentDate      *string  `json:"last_payment_date"`
	LastPaymentStatus    *string  `json:"last_payment_status"`
}

// GetMemberships returns all memberships for a client with enriched data.
func (h *ClientHandler) GetMemberships(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	clientID, _ := uuid.Parse(chi.URLParam(r, "clientID"))

	query := `
		SELECT
			m.id::text,
			mt.title,
			mt.id::text,
			mto.duration_months,
			TO_CHAR(m.start_date, 'YYYY-MM-DD'),
			TO_CHAR(m.min_term_end_date, 'YYYY-MM-DD'),
			m.payment_method,
			m.payment_frequency,
			m.payment_interval_unit,
			m.payment_interval_count,
			m.payment_amount,
			m.num_payments,
			m.status,
			m.gocardless_mandate_id,
			m.gocardless_subscription_id,
			COALESCE((SELECT SUM(p.amount) FROM payments p WHERE p.membership_id = m.id AND p.status = 'succeeded'), 0),
			CASE
				WHEN m.num_payments IS NOT NULL THEN COALESCE(m.payment_amount, 0) * m.num_payments
				ELSE COALESCE(m.payment_amount, 0) * mto.duration_months
			END,
			(SELECT COUNT(*) FROM payments p WHERE p.membership_id = m.id AND p.status = 'failed'),
			(SELECT TO_CHAR(p.paid_date, 'YYYY-MM-DD') FROM payments p WHERE p.membership_id = m.id ORDER BY p.created_at DESC LIMIT 1),
			(SELECT p.status FROM payments p WHERE p.membership_id = m.id ORDER BY p.created_at DESC LIMIT 1)
		FROM memberships m
		JOIN membership_types mt ON mt.id = m.membership_type_id
		JOIN membership_term_options mto ON mto.id = m.term_option_id
		WHERE m.client_id = $1 AND m.tenant_id = $2
		ORDER BY m.start_date DESC`

	rows, err := h.clientRepo.DB().Pool.Query(r.Context(), query, clientID, tenantID)
	if err != nil {
		log.Printf("Error getting client memberships: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to get memberships")
		return
	}
	defer rows.Close()

	var results []ClientMembershipRow
	for rows.Next() {
		var row ClientMembershipRow
		if err := rows.Scan(
			&row.MembershipID, &row.MembershipType, &row.MembershipTypeID,
			&row.TermMonths, &row.StartDate, &row.EndDate,
			&row.PaymentMethod, &row.PaymentFrequency, &row.PaymentIntervalUnit, &row.PaymentIntervalCount, &row.PaymentAmount, &row.NumPayments,
			&row.Status, &row.MandateID, &row.SubscriptionID,
			&row.TotalPaid, &row.TotalExpected,
			&row.FailedCount, &row.LastPaymentDate, &row.LastPaymentStatus,
		); err != nil {
			log.Printf("Error scanning client membership: %v", err)
			continue
		}
		results = append(results, row)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"memberships": results, "count": len(results)})
}

// GetMandateStatus checks whether a client has a GoCardless mandate on any active membership.
func (h *ClientHandler) GetMandateStatus(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	clientID, err := uuid.Parse(chi.URLParam(r, "clientID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}

	client, err := h.clientRepo.FindByID(r.Context(), clientID, tenantID)
	if err != nil || client == nil {
		writeError(w, http.StatusNotFound, "Client not found")
		return
	}

	result := map[string]interface{}{
		"has_mandate":            false,
		"mandate_id":             nil,
		"membership_id":          nil,
		"gocardless_customer_id": nil,
		"client_email":           nil,
	}

	if client.GoCardlessCustomerID != nil && *client.GoCardlessCustomerID != "" {
		result["gocardless_customer_id"] = *client.GoCardlessCustomerID
	}
	if client.Email != nil && *client.Email != "" {
		result["client_email"] = *client.Email
	}

	// Check memberships for an active mandate
	memberships, err := h.clientRepo.GetMemberships(r.Context(), clientID, tenantID)
	if err == nil {
		for _, m := range memberships {
			if m.Status == "active" && m.GoCardlessMandateID != nil && *m.GoCardlessMandateID != "" {
				result["has_mandate"] = true
				result["mandate_id"] = *m.GoCardlessMandateID
				result["membership_id"] = m.ID.String()
				break
			}
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// SendMandateLink creates a GoCardless redirect flow and emails the link to the client.
func (h *ClientHandler) SendMandateLink(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	clientID, err := uuid.Parse(chi.URLParam(r, "clientID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}

	client, err := h.clientRepo.FindByID(r.Context(), clientID, tenantID)
	if err != nil || client == nil {
		writeError(w, http.StatusNotFound, "Client not found")
		return
	}

	var input struct {
		RedirectURI string `json:"redirect_uri"`
	}
	_ = decodeJSON(r, &input) // optional body
	if input.RedirectURI == "" {
		input.RedirectURI = r.Header.Get("Origin")
		if input.RedirectURI == "" {
			input.RedirectURI = "https://app.clientroll.com"
		}
		input.RedirectURI += "/clients/" + clientID.String()
	}

	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil || prov == nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured for this tenant")
		return
	}

	sessionToken := fmt.Sprintf("mandate-%s-%s", clientID, uuid.New().String()[:8])
	// Append session_token so when GoCardless redirects back (with redirect_flow_id), the client profile has both params to call complete-mandate-link.
	if strings.Contains(input.RedirectURI, "?") {
		input.RedirectURI += "&session_token=" + sessionToken
	} else {
		input.RedirectURI += "?session_token=" + sessionToken
	}
	redirectURL, flowID, err := prov.CreateRedirectFlow(r.Context(), input.RedirectURI, "Set up Direct Debit for "+client.Name, sessionToken)
	if err != nil {
		log.Printf("GoCardless CreateRedirectFlow for client %s: %v", clientID, err)
		writeProviderError(w, err)
		return
	}

	// Email the link to the client
	emailSent := false
	if client.Email != nil && *client.Email != "" && h.emailSvc != nil {
		html := fmt.Sprintf(`
			<h2 style="font-size:18px;font-weight:600;color:#111827;margin:0 0 12px;">Set Up Direct Debit</h2>
			<p style="font-size:14px;color:#6b7280;line-height:1.6;">Hi %s,</p>
			<p style="font-size:14px;color:#6b7280;line-height:1.6;">Please click the link below to securely set up your direct debit mandate:</p>
			<p style="margin:24px 0;"><a href="%s" style="display:inline-block;padding:12px 24px;background-color:#4f46e5;color:#ffffff;text-decoration:none;border-radius:8px;font-weight:600;font-size:14px;">Set Up Direct Debit</a></p>
			<p style="font-size:12px;color:#9ca3af;">This link will take you to GoCardless to securely enter your bank details.</p>
		`, client.Name, redirectURL)
		_, sendErr := h.emailSvc.Send(context.Background(), email.SendInput{
			To:      *client.Email,
			Subject: "Set Up Your Direct Debit",
			HTML:    html,
		})
		if sendErr != nil {
			log.Printf("Error sending mandate link email to %s: %v", *client.Email, sendErr)
		} else {
			emailSent = true
		}
	}

	// Store session for completion callback
	if h.redirectFlowRepo != nil {
		_ = h.redirectFlowRepo.Create(r.Context(), tenantID, clientID, sessionToken, flowID)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"redirect_url":  redirectURL,
		"flow_id":       flowID,
		"session_token": sessionToken,
		"email_sent":    emailSent,
	})
}

// CompleteMandateLink completes a redirect flow after the client returns from GoCardless.
// Links the resulting mandate to the client's active membership and creates a subscription.
func (h *ClientHandler) CompleteMandateLink(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := uuid.Parse(middleware.GetTenantID(r.Context()))
	clientID, err := uuid.Parse(chi.URLParam(r, "clientID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid client ID")
		return
	}

	var input struct {
		FlowID       string `json:"flow_id"`
		SessionToken string `json:"session_token"`
	}
	if err := decodeJSON(r, &input); err != nil || input.FlowID == "" || input.SessionToken == "" {
		writeError(w, http.StatusBadRequest, "flow_id and session_token required")
		return
	}

	prov, err := h.getGoCardlessProvider(r.Context(), tenantID)
	if err != nil || prov == nil {
		writeError(w, http.StatusBadRequest, "GoCardless not configured")
		return
	}

	// Complete the redirect flow with GoCardless
	mandateID, customerID, err := prov.CompleteRedirectFlow(r.Context(), input.FlowID, input.SessionToken)
	if err != nil {
		log.Printf("GoCardless CompleteRedirectFlow for client %s: %v", clientID, err)
		writeProviderError(w, err)
		return
	}

	// Store customer ID on client
	_ = h.clientRepo.SetGoCardlessCustomerID(r.Context(), clientID, tenantID, customerID)

	// Find active membership and link mandate
	memberships, _ := h.clientRepo.GetMemberships(r.Context(), clientID, tenantID)
	var linkedMembershipID string
	var subscriptionID string
	for _, m := range memberships {
		if m.Status == "active" && (m.GoCardlessMandateID == nil || *m.GoCardlessMandateID == "") {
			_ = h.memberRepo.SetGoCardlessMandateID(r.Context(), m.ID, tenantID, mandateID)
			linkedMembershipID = m.ID.String()

			// Create subscription for recurring payments (pass num_payments from term option if set)
			if m.PaymentFrequency != nil && *m.PaymentFrequency != "upfront" && m.PaymentAmount != nil {
				var numPay *int
				if h.mtRepo != nil {
					if to, _ := h.mtRepo.GetTermOption(r.Context(), m.TermOptionID, tenantID); to != nil && to.NumPayments != nil {
						numPay = to.NumPayments
					}
				}
				subID, _ := createGoCardlessSubscription(r.Context(), prov, h.memberRepo, mandateID, &m,
					tenantID, *m.PaymentFrequency, *m.PaymentAmount, m.PaymentIntervalUnit, m.PaymentIntervalCount, "Membership", m.MinTermEndDate, numPay)
				subscriptionID = subID
			}
			break
		}
	}

	// Clean up the session
	if h.redirectFlowRepo != nil {
		session, _ := h.redirectFlowRepo.FindBySessionToken(r.Context(), input.SessionToken)
		if session != nil {
			_ = h.redirectFlowRepo.Delete(r.Context(), session.ID)
		}
	}

	log.Printf("Redirect flow completed: client=%s mandate=%s customer=%s membership=%s subscription=%s",
		clientID, mandateID, customerID, linkedMembershipID, subscriptionID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"mandate_id":      mandateID,
		"customer_id":     customerID,
		"membership_id":   linkedMembershipID,
		"subscription_id": subscriptionID,
	})
}

// getGoCardlessProvider returns a GoCardless provider for the tenant (if configured).
func (h *ClientHandler) getGoCardlessProvider(ctx context.Context, tenantID uuid.UUID) (payment.GoCardlessExtended, error) {
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
