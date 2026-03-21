package repository

import (
	"context"
	"fmt"

	"clientroll/internal/database"
	"clientroll/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ClientRepo handles database operations for clients.
type ClientRepo struct {
	db *database.Postgres
}

// NewClientRepo creates a new client repository.
func NewClientRepo(db *database.Postgres) *ClientRepo {
	return &ClientRepo{db: db}
}

// DB returns the underlying database connection.
func (r *ClientRepo) DB() *database.Postgres {
	return r.db
}

const clientFields = `id, tenant_id, name, unit_number, address, suburb, state, postcode,
	email, phone, birth_date, gender, emergency_contact_name,
	emergency_contact_phone, gocardless_customer_id, stripe_customer_id, deleted_at, created_at, updated_at`

// List returns all clients for a tenant with optional search and status filter.
// status: "active" = has active membership, "inactive" = no active membership, "" = all
func (r *ClientRepo) List(ctx context.Context, tenantID uuid.UUID, search, status string) ([]models.Client, error) {
	query := `SELECT ` + clientFields + ` FROM clients WHERE tenant_id = $1 AND deleted_at IS NULL`
	args := []interface{}{tenantID}
	argN := 2

	if search != "" {
		query += fmt.Sprintf(` AND (name ILIKE $%d OR email ILIKE $%d OR phone ILIKE $%d)`, argN, argN, argN)
		args = append(args, "%"+search+"%")
		argN++
	}

	if status == "active" {
		query += ` AND EXISTS (SELECT 1 FROM memberships m WHERE m.client_id = clients.id AND m.tenant_id = clients.tenant_id AND m.status = 'active')`
	} else if status == "inactive" {
		query += ` AND NOT EXISTS (SELECT 1 FROM memberships m WHERE m.client_id = clients.id AND m.tenant_id = clients.tenant_id AND m.status = 'active')`
	}

	query += ` ORDER BY name`

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing clients: %w", err)
	}
	defer rows.Close()
	return r.scanMany(rows)
}

// FindByID returns a client by ID.
func (r *ClientRepo) FindByID(ctx context.Context, id, tenantID uuid.UUID) (*models.Client, error) {
	query := `SELECT ` + clientFields + ` FROM clients WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`

	var c models.Client
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID).Scan(r.scanFields(&c)...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding client: %w", err)
	}
	return &c, nil
}

// FindByEmail returns an existing client matching email within a tenant.
func (r *ClientRepo) FindByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*models.Client, error) {
	query := `SELECT ` + clientFields + ` FROM clients WHERE tenant_id = $1 AND LOWER(email) = LOWER($2) AND deleted_at IS NULL LIMIT 1`

	var c models.Client
	err := r.db.Pool.QueryRow(ctx, query, tenantID, email).Scan(r.scanFields(&c)...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding client by email: %w", err)
	}
	return &c, nil
}

// Create inserts a new client.
func (r *ClientRepo) Create(ctx context.Context, tenantID uuid.UUID, c *models.Client) (*models.Client, error) {
	query := `
		INSERT INTO clients (tenant_id, name, unit_number, address, suburb, state, postcode,
		    email, phone, birth_date, gender, emergency_contact_name, emergency_contact_phone)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING ` + clientFields

	var out models.Client
	err := r.db.Pool.QueryRow(ctx, query, tenantID, c.Name, c.UnitNumber, c.Address, c.Suburb,
		c.State, c.Postcode, c.Email, c.Phone, c.BirthDate, c.Gender,
		c.EmergencyContactName, c.EmergencyContactPhone).Scan(r.scanFields(&out)...)
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}
	return &out, nil
}

// Update updates a client.
func (r *ClientRepo) Update(ctx context.Context, id, tenantID uuid.UUID, c *models.Client) (*models.Client, error) {
	query := `
		UPDATE clients SET name=$3, unit_number=$4, address=$5, suburb=$6, state=$7, postcode=$8,
		    email=$9, phone=$10, birth_date=$11, gender=$12,
		    emergency_contact_name=$13, emergency_contact_phone=$14, updated_at=NOW()
		WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL
		RETURNING ` + clientFields

	var out models.Client
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID, c.Name, c.UnitNumber, c.Address, c.Suburb,
		c.State, c.Postcode, c.Email, c.Phone, c.BirthDate, c.Gender,
		c.EmergencyContactName, c.EmergencyContactPhone).Scan(r.scanFields(&out)...)
	if err != nil {
		return nil, fmt.Errorf("updating client: %w", err)
	}
	return &out, nil
}

// SetGoCardlessCustomerID stores the GoCardless customer ID for a client.
func (r *ClientRepo) SetGoCardlessCustomerID(ctx context.Context, clientID, tenantID uuid.UUID, customerID string) error {
	_, err := r.db.Pool.Exec(ctx,
		"UPDATE clients SET gocardless_customer_id = $3, updated_at = NOW() WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL",
		clientID, tenantID, customerID)
	return err
}

// SetStripeCustomerID stores the Stripe customer ID for a client.
func (r *ClientRepo) SetStripeCustomerID(ctx context.Context, clientID, tenantID uuid.UUID, customerID string) error {
	_, err := r.db.Pool.Exec(ctx,
		"UPDATE clients SET stripe_customer_id = $3, updated_at = NOW() WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL",
		clientID, tenantID, customerID)
	return err
}

// FindLatestStripeCustomerIDFromHistory inspects payment snapshots for the newest Stripe customer id for a client.
func (r *ClientRepo) FindLatestStripeCustomerIDFromHistory(ctx context.Context, clientID, tenantID uuid.UUID) (string, error) {
	var customerID string
	err := r.db.Pool.QueryRow(ctx, `
		WITH candidates AS (
			SELECT p.stripe_details->>'stripe_customer_id' AS customer_id, p.created_at
			FROM payments p
			WHERE p.client_id = $1 AND p.tenant_id = $2
			  AND COALESCE(p.stripe_details->>'stripe_customer_id', '') <> ''
			UNION ALL
			SELECT sc.stripe_details->>'stripe_customer_id' AS customer_id, sc.created_at
			FROM service_charges sc
			WHERE sc.client_id = $1 AND sc.tenant_id = $2
			  AND COALESCE(sc.stripe_details->>'stripe_customer_id', '') <> ''
		)
		SELECT customer_id
		FROM candidates
		ORDER BY created_at DESC
		LIMIT 1
	`, clientID, tenantID).Scan(&customerID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("find latest stripe customer id from history: %w", err)
	}
	return customerID, nil
}

// GetMemberships returns all memberships for a client.
func (r *ClientRepo) GetMemberships(ctx context.Context, clientID, tenantID uuid.UUID) ([]models.Membership, error) {
	query := `
		SELECT id, tenant_id, client_id, membership_type_id, term_option_id,
		       start_date, min_term_end_date, payment_method, payment_frequency, currency_code, payment_interval_unit, payment_interval_count,
		       payment_amount, num_payments, payments_made, status, gocardless_mandate_id, gocardless_subscription_id,
		       member_signature_data, member_signed_at,
		       staff_user_id, staff_signature_data, staff_signed_at, created_at, updated_at
		FROM memberships WHERE client_id = $1 AND tenant_id = $2
		ORDER BY created_at DESC`

	rows, err := r.db.Pool.Query(ctx, query, clientID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("getting client memberships: %w", err)
	}
	defer rows.Close()

	var memberships []models.Membership
	for rows.Next() {
		var m models.Membership
		if err := rows.Scan(&m.ID, &m.TenantID, &m.ClientID, &m.MembershipTypeID,
			&m.TermOptionID, &m.StartDate, &m.MinTermEndDate, &m.PaymentMethod,
			&m.PaymentFrequency, &m.CurrencyCode, &m.PaymentIntervalUnit, &m.PaymentIntervalCount, &m.PaymentAmount, &m.NumPayments, &m.PaymentsMade, &m.Status,
			&m.GoCardlessMandateID, &m.GoCardlessSubscriptionID,
			&m.MemberSignature, &m.MemberSignedAt, &m.StaffUserID, &m.StaffSignature, &m.StaffSignedAt,
			&m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		memberships = append(memberships, m)
	}
	return memberships, rows.Err()
}

// SoftDelete marks a client as deleted (soft delete).
func (r *ClientRepo) SoftDelete(ctx context.Context, clientID, tenantID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx,
		"UPDATE clients SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL",
		clientID, tenantID,
	)
	return err
}

// SoftDeleteMany marks multiple clients as deleted (soft delete).
func (r *ClientRepo) SoftDeleteMany(ctx context.Context, tenantID uuid.UUID, clientIDs []uuid.UUID) (int64, error) {
	if len(clientIDs) == 0 {
		return 0, nil
	}
	tag, err := r.db.Pool.Exec(ctx,
		"UPDATE clients SET deleted_at = NOW(), updated_at = NOW() WHERE tenant_id = $1 AND id = ANY($2) AND deleted_at IS NULL",
		tenantID, clientIDs,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *ClientRepo) scanFields(c *models.Client) []interface{} {
	return []interface{}{
		&c.ID, &c.TenantID, &c.Name, &c.UnitNumber, &c.Address, &c.Suburb, &c.State,
		&c.Postcode, &c.Email, &c.Phone, &c.BirthDate, &c.Gender,
		&c.EmergencyContactName, &c.EmergencyContactPhone, &c.GoCardlessCustomerID, &c.StripeCustomerID, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt,
	}
}

func (r *ClientRepo) scanMany(rows pgx.Rows) ([]models.Client, error) {
	var items []models.Client
	for rows.Next() {
		var c models.Client
		if err := rows.Scan(r.scanFields(&c)...); err != nil {
			return nil, fmt.Errorf("scanning client: %w", err)
		}
		items = append(items, c)
	}
	return items, rows.Err()
}
