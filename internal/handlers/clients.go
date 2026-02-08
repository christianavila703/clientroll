package handlers

import (
	"log"
	"net/http"

	"clientroll/internal/middleware"
	"clientroll/internal/models"
	"clientroll/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ClientHandler handles client CRUD endpoints.
type ClientHandler struct {
	clientRepo *repository.ClientRepo
}

// NewClientHandler creates a new client handler.
func NewClientHandler(cr *repository.ClientRepo) *ClientHandler {
	return &ClientHandler{clientRepo: cr}
}

// RegisterRoutes registers client routes.
func (h *ClientHandler) RegisterRoutes(r chi.Router) {
	r.Get("/clients", h.List)
	r.Post("/clients", h.Create)
	r.Get("/clients/{clientID}", h.Get)
	r.Put("/clients/{clientID}", h.Update)
	r.Get("/clients/{clientID}/memberships", h.GetMemberships)
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
	var input models.Client
	if err := decodeJSON(r, &input); err != nil || input.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}
	client, err := h.clientRepo.Create(r.Context(), tenantID, &input)
	if err != nil {
		log.Printf("Error creating client: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create client")
		return
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
	var input models.Client
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	client, err := h.clientRepo.Update(r.Context(), clientID, tenantID, &input)
	if err != nil {
		log.Printf("Error updating client: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update client")
		return
	}
	writeJSON(w, http.StatusOK, client)
}

// ClientMembershipRow is the enriched membership data for the client profile.
type ClientMembershipRow struct {
	MembershipID     string   `json:"membership_id"`
	MembershipType   string   `json:"membership_type"`
	MembershipTypeID string   `json:"membership_type_id"`
	TermMonths       int      `json:"term_months"`
	StartDate        string   `json:"start_date"`
	EndDate          string   `json:"end_date"`
	PaymentMethod    *string  `json:"payment_method"`
	PaymentFrequency *string  `json:"payment_frequency"`
	PaymentAmount    *float64 `json:"payment_amount"`
	Status           string   `json:"status"`
	TotalPaid        float64  `json:"total_paid"`
	TotalExpected    float64  `json:"total_expected"`
	FailedCount      int      `json:"failed_count"`
	LastPaymentDate  *string  `json:"last_payment_date"`
	LastPaymentStatus *string `json:"last_payment_status"`
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
			m.payment_amount,
			m.status,
			COALESCE((SELECT SUM(p.amount) FROM payments p WHERE p.membership_id = m.id AND p.status = 'succeeded'), 0),
			COALESCE(m.payment_amount, 0) * mto.duration_months,
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
			&row.PaymentMethod, &row.PaymentFrequency, &row.PaymentAmount,
			&row.Status, &row.TotalPaid, &row.TotalExpected,
			&row.FailedCount, &row.LastPaymentDate, &row.LastPaymentStatus,
		); err != nil {
			log.Printf("Error scanning client membership: %v", err)
			continue
		}
		results = append(results, row)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"memberships": results, "count": len(results)})
}
