package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Payment represents a payment record.
type Payment struct {
	ID                uuid.UUID  `json:"id"`
	TenantID          uuid.UUID  `json:"tenant_id"`
	MembershipID      uuid.UUID  `json:"membership_id"`
	ClientID          uuid.UUID  `json:"client_id"`
	Amount            float64    `json:"amount"`
	Status            string     `json:"status"`
	ProviderReference *string    `json:"provider_reference"`
	DueDate           time.Time  `json:"due_date"`
	PaidDate          *time.Time `json:"paid_date"`
	FailureReason     *string    `json:"failure_reason"`
	FailureCode       *string    `json:"failure_code"`
	RetryCount        int        `json:"retry_count"`
	CreatedAt         time.Time  `json:"created_at"`
}

// PaymentRepo handles database operations for payments.
type PaymentRepo struct {
	db *database.Postgres
}

// NewPaymentRepo creates a new payment repository.
func NewPaymentRepo(db *database.Postgres) *PaymentRepo {
	return &PaymentRepo{db: db}
}

const paymentFields = `id, tenant_id, membership_id, client_id, amount, status,
	provider_reference, due_date, paid_date, failure_reason, failure_code,
	retry_count, created_at`

// ListByTenant returns payments for a tenant with optional status filter.
func (r *PaymentRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID, status string, limit int) ([]Payment, error) {
	query := `SELECT ` + paymentFields + ` FROM payments WHERE tenant_id = $1`
	args := []interface{}{tenantID}
	argN := 2

	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argN)
		args = append(args, status)
		argN++
	}
	query += " ORDER BY created_at DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argN)
		args = append(args, limit)
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing payments: %w", err)
	}
	defer rows.Close()
	return r.scanMany(rows)
}

// ListByClient returns payments for a specific client.
func (r *PaymentRepo) ListByClient(ctx context.Context, clientID, tenantID uuid.UUID) ([]Payment, error) {
	query := `SELECT ` + paymentFields + ` FROM payments WHERE client_id = $1 AND tenant_id = $2 ORDER BY due_date DESC`
	rows, err := r.db.Pool.Query(ctx, query, clientID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanMany(rows)
}

// Create inserts a payment record.
func (r *PaymentRepo) Create(ctx context.Context, tenantID, membershipID, clientID uuid.UUID, amount float64, dueDate time.Time, providerRef string) (*Payment, error) {
	query := `INSERT INTO payments (tenant_id, membership_id, client_id, amount, due_date, provider_reference)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING ` + paymentFields

	var p Payment
	err := r.db.Pool.QueryRow(ctx, query, tenantID, membershipID, clientID, amount, dueDate, providerRef).Scan(r.scanFields(&p)...)
	if err != nil {
		return nil, fmt.Errorf("creating payment: %w", err)
	}
	return &p, nil
}

// CreateIfNotExists inserts a payment, or returns the existing payment for provider reference.
func (r *PaymentRepo) CreateIfNotExists(ctx context.Context, tenantID, membershipID, clientID uuid.UUID, amount float64, dueDate time.Time, providerRef string) (*Payment, bool, error) {
	query := `INSERT INTO payments (tenant_id, membership_id, client_id, amount, due_date, provider_reference)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tenant_id, provider_reference) WHERE provider_reference IS NOT NULL
		DO UPDATE SET updated_at = NOW()
		RETURNING ` + paymentFields + `, (xmax = 0) AS inserted`

	var p Payment
	var inserted bool
	err := r.db.Pool.QueryRow(ctx, query, tenantID, membershipID, clientID, amount, dueDate, providerRef).Scan(append(r.scanFields(&p), &inserted)...)
	if err != nil {
		return nil, false, fmt.Errorf("create or get payment: %w", err)
	}
	return &p, inserted, nil
}

// GetByID returns a payment by ID and tenant.
func (r *PaymentRepo) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*Payment, error) {
	query := `SELECT ` + paymentFields + ` FROM payments WHERE id = $1 AND tenant_id = $2`
	var p Payment
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID).Scan(r.scanFields(&p)...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding payment by id: %w", err)
	}
	return &p, nil
}

// GetByProviderReference returns a payment by tenant and GoCardless payment ID (provider_reference).
func (r *PaymentRepo) GetByProviderReference(ctx context.Context, tenantID uuid.UUID, providerRef string) (*Payment, error) {
	query := `SELECT ` + paymentFields + ` FROM payments WHERE tenant_id = $1 AND provider_reference = $2`
	var p Payment
	err := r.db.Pool.QueryRow(ctx, query, tenantID, providerRef).Scan(r.scanFields(&p)...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding payment by provider reference: %w", err)
	}
	return &p, nil
}

// GetByProviderReferences returns the first payment matched by any provider reference.
func (r *PaymentRepo) GetByProviderReferences(ctx context.Context, tenantID uuid.UUID, providerRefs ...string) (*Payment, error) {
	refs := make([]string, 0, len(providerRefs))
	for _, ref := range providerRefs {
		ref = strings.TrimSpace(ref)
		if ref != "" {
			refs = append(refs, ref)
		}
	}
	if len(refs) == 0 {
		return nil, nil
	}

	for _, ref := range refs {
		p, err := r.GetByProviderReference(ctx, tenantID, ref)
		if err != nil {
			return nil, err
		}
		if p != nil {
			return p, nil
		}
	}
	return nil, nil
}

// UpdateStatus updates a payment's status.
func (r *PaymentRepo) UpdateStatus(ctx context.Context, id, tenantID uuid.UUID, status string, paidDate *time.Time, failureReason, failureCode *string) error {
	query := `UPDATE payments SET status=$3, paid_date=$4, failure_reason=$5, failure_code=$6,
		retry_count = CASE WHEN $3 = 'failed' THEN retry_count + 1 ELSE retry_count END,
		updated_at=NOW() WHERE id=$1 AND tenant_id=$2`
	_, err := r.db.Pool.Exec(ctx, query, id, tenantID, status, paidDate, failureReason, failureCode)
	return err
}

// CountByStatus returns payment counts grouped by status for a tenant.
func (r *PaymentRepo) CountByStatus(ctx context.Context, tenantID uuid.UUID) (map[string]int, error) {
	query := `SELECT status, COUNT(*) FROM payments WHERE tenant_id = $1 GROUP BY status`
	rows, err := r.db.Pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	return counts, rows.Err()
}

func (r *PaymentRepo) scanFields(p *Payment) []interface{} {
	return []interface{}{
		&p.ID, &p.TenantID, &p.MembershipID, &p.ClientID, &p.Amount, &p.Status,
		&p.ProviderReference, &p.DueDate, &p.PaidDate, &p.FailureReason,
		&p.FailureCode, &p.RetryCount, &p.CreatedAt,
	}
}

func (r *PaymentRepo) scanMany(rows pgx.Rows) ([]Payment, error) {
	var items []Payment
	for rows.Next() {
		var p Payment
		if err := rows.Scan(r.scanFields(&p)...); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}
