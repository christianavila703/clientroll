package repository

import (
	"context"
	"fmt"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PaymentDispute represents a dispute entry linked to a payment.
type PaymentDispute struct {
	ID                uuid.UUID   `json:"id"`
	TenantID          uuid.UUID   `json:"tenant_id"`
	PaymentID         uuid.UUID   `json:"payment_id"`
	ClientID          uuid.UUID   `json:"client_id"`
	ProviderDisputeID *string     `json:"provider_dispute_id"`
	Reason            *string     `json:"reason"`
	Status            string      `json:"status"`
	Amount            float64     `json:"amount"`
	Notes             *string     `json:"notes"`
	ResolvedBy        *string     `json:"resolved_by"`
	OpenedAt          time.Time   `json:"opened_at"`
	ResolvedAt        *time.Time  `json:"resolved_at"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

// PaymentDisputeRepo handles payment dispute persistence.
type PaymentDisputeRepo struct {
	db *database.Postgres
}

// NewPaymentDisputeRepo creates a new payment dispute repository.
func NewPaymentDisputeRepo(db *database.Postgres) *PaymentDisputeRepo {
	return &PaymentDisputeRepo{db: db}
}

const paymentDisputeFields = `id, tenant_id, payment_id, client_id, provider_dispute_id, reason, status,
	amount, notes, resolved_by, opened_at, resolved_at, created_at, updated_at`

// GetByProviderDisputeID returns a dispute by provider dispute id.
func (r *PaymentDisputeRepo) GetByProviderDisputeID(ctx context.Context, tenantID uuid.UUID, providerDisputeID string) (*PaymentDispute, error) {
	query := `SELECT ` + paymentDisputeFields + ` FROM payment_disputes
		WHERE tenant_id = $1 AND provider_dispute_id = $2
		ORDER BY created_at DESC LIMIT 1`
	var d PaymentDispute
	err := r.db.Pool.QueryRow(ctx, query, tenantID, providerDisputeID).Scan(r.scanFields(&d)...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find dispute by provider id: %w", err)
	}
	return &d, nil
}

// GetOpenByPaymentID returns the latest open/under_review dispute for the payment.
func (r *PaymentDisputeRepo) GetOpenByPaymentID(ctx context.Context, tenantID, paymentID uuid.UUID) (*PaymentDispute, error) {
	query := `SELECT ` + paymentDisputeFields + ` FROM payment_disputes
		WHERE tenant_id = $1 AND payment_id = $2 AND status IN ('open', 'under_review')
		ORDER BY opened_at DESC LIMIT 1`
	var d PaymentDispute
	err := r.db.Pool.QueryRow(ctx, query, tenantID, paymentID).Scan(r.scanFields(&d)...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find open dispute by payment: %w", err)
	}
	return &d, nil
}

// Create inserts a new payment dispute record.
func (r *PaymentDisputeRepo) Create(
	ctx context.Context,
	tenantID, paymentID, clientID uuid.UUID,
	providerDisputeID, reason *string,
	status string,
	amount float64,
	notes *string,
	resolvedBy *string,
	resolvedAt *time.Time,
) (*PaymentDispute, error) {
	query := `INSERT INTO payment_disputes (
		tenant_id, payment_id, client_id, provider_dispute_id, reason, status,
		amount, notes, resolved_by, resolved_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	RETURNING ` + paymentDisputeFields

	var d PaymentDispute
	err := r.db.Pool.QueryRow(
		ctx,
		query,
		tenantID, paymentID, clientID, providerDisputeID, reason, status,
		amount, notes, resolvedBy, resolvedAt,
	).Scan(r.scanFields(&d)...)
	if err != nil {
		return nil, fmt.Errorf("create payment dispute: %w", err)
	}
	return &d, nil
}

// Update updates an existing payment dispute record.
func (r *PaymentDisputeRepo) Update(
	ctx context.Context,
	id, tenantID uuid.UUID,
	reason *string,
	status string,
	amount float64,
	notes *string,
	resolvedBy *string,
	resolvedAt *time.Time,
) (*PaymentDispute, error) {
	query := `UPDATE payment_disputes
		SET reason = $3, status = $4, amount = $5, notes = $6, resolved_by = $7, resolved_at = $8, updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
		RETURNING ` + paymentDisputeFields
	var d PaymentDispute
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID, reason, status, amount, notes, resolvedBy, resolvedAt).Scan(r.scanFields(&d)...)
	if err != nil {
		return nil, fmt.Errorf("update payment dispute: %w", err)
	}
	return &d, nil
}

// UpsertByProviderOrPayment upserts a dispute using provider dispute id when available.
func (r *PaymentDisputeRepo) UpsertByProviderOrPayment(
	ctx context.Context,
	tenantID, paymentID, clientID uuid.UUID,
	providerDisputeID, reason *string,
	status string,
	amount float64,
	notes *string,
	resolvedBy *string,
	resolvedAt *time.Time,
) (*PaymentDispute, error) {
	if providerDisputeID != nil && *providerDisputeID != "" {
		d, err := r.GetByProviderDisputeID(ctx, tenantID, *providerDisputeID)
		if err != nil {
			return nil, err
		}
		if d != nil {
			return r.Update(ctx, d.ID, tenantID, reason, status, amount, notes, resolvedBy, resolvedAt)
		}
		return r.Create(ctx, tenantID, paymentID, clientID, providerDisputeID, reason, status, amount, notes, resolvedBy, resolvedAt)
	}

	d, err := r.GetOpenByPaymentID(ctx, tenantID, paymentID)
	if err != nil {
		return nil, err
	}
	if d != nil {
		return r.Update(ctx, d.ID, tenantID, reason, status, amount, notes, resolvedBy, resolvedAt)
	}
	return r.Create(ctx, tenantID, paymentID, clientID, nil, reason, status, amount, notes, resolvedBy, resolvedAt)
}

func (r *PaymentDisputeRepo) scanFields(d *PaymentDispute) []interface{} {
	return []interface{}{
		&d.ID, &d.TenantID, &d.PaymentID, &d.ClientID, &d.ProviderDisputeID, &d.Reason, &d.Status,
		&d.Amount, &d.Notes, &d.ResolvedBy, &d.OpenedAt, &d.ResolvedAt, &d.CreatedAt, &d.UpdatedAt,
	}
}
