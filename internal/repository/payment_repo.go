package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Payment represents a payment record.
type Payment struct {
	ID                  uuid.UUID       `json:"id"`
	TenantID            uuid.UUID       `json:"tenant_id"`
	MembershipID        uuid.UUID       `json:"membership_id"`
	ClientID            uuid.UUID       `json:"client_id"`
	Amount              float64         `json:"amount"`
	BaseAmount          float64         `json:"base_amount"`
	TaxAmount           float64         `json:"tax_amount"`
	ProcessingFeeAmount float64         `json:"processing_fee_amount"`
	CurrencyCode        string          `json:"currency_code"`
	Status              string          `json:"status"`
	ProviderReference   *string         `json:"provider_reference"`
	DueDate             time.Time       `json:"due_date"`
	PaidDate            *time.Time      `json:"paid_date"`
	FailureReason       *string         `json:"failure_reason"`
	FailureCode         *string         `json:"failure_code"`
	RetryCount          int             `json:"retry_count"`
	CreatedAt           time.Time       `json:"created_at"`
	StripeDetails       json.RawMessage `json:"stripe_details,omitempty"`
}

// PaymentComponents is an optional base / tax / processing-fee split for the charged amount.
// If nil, the full amount is treated as base (tax and fee zero) — legacy behaviour.
type PaymentComponents struct {
	BaseAmount          float64
	TaxAmount           float64
	ProcessingFeeAmount float64
}

func normalizePaymentLineAmounts(total float64, c *PaymentComponents) (base, tax, fee float64) {
	if c == nil {
		return total, 0, 0
	}
	base, tax, fee = c.BaseAmount, c.TaxAmount, c.ProcessingFeeAmount
	if base < 0 {
		base = 0
	}
	if tax < 0 {
		tax = 0
	}
	if fee < 0 {
		fee = 0
	}
	// Keep ledger consistent with charged total
	sum := base + tax + fee
	if sum <= 0 {
		return total, 0, 0
	}
	if diff := total - sum; diff < -0.02 || diff > 0.02 {
		base = total - tax - fee
		if base < 0 {
			base = 0
		}
	}
	return base, tax, fee
}

// PaymentRepo handles database operations for payments.
type PaymentRepo struct {
	db *database.Postgres
}

// NewPaymentRepo creates a new payment repository.
func NewPaymentRepo(db *database.Postgres) *PaymentRepo {
	return &PaymentRepo{db: db}
}

const paymentFields = `id, tenant_id, membership_id, client_id, amount, base_amount, tax_amount, processing_fee_amount, currency_code, status,
	provider_reference, due_date, paid_date, failure_reason, failure_code,
	retry_count, COALESCE(stripe_details, '{}'::jsonb), created_at`

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
func (r *PaymentRepo) Create(ctx context.Context, tenantID, membershipID, clientID uuid.UUID, amount float64, currencyCode string, dueDate time.Time, providerRef string, components *PaymentComponents) (*Payment, error) {
	base, tax, fee := normalizePaymentLineAmounts(amount, components)
	query := `INSERT INTO payments (tenant_id, membership_id, client_id, amount, base_amount, tax_amount, processing_fee_amount, currency_code, due_date, provider_reference)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING ` + paymentFields

	p, err := r.scanPaymentRow(r.db.Pool.QueryRow(ctx, query, tenantID, membershipID, clientID, amount, base, tax, fee, currencyCode, dueDate, providerRef))
	if err != nil {
		return nil, fmt.Errorf("creating payment: %w", err)
	}
	return p, nil
}

// CreateIfNotExists inserts a payment, or returns the existing payment for provider reference.
func (r *PaymentRepo) CreateIfNotExists(ctx context.Context, tenantID, membershipID, clientID uuid.UUID, amount float64, currencyCode string, dueDate time.Time, providerRef string, components *PaymentComponents) (*Payment, bool, error) {
	base, tax, fee := normalizePaymentLineAmounts(amount, components)
	query := `INSERT INTO payments (tenant_id, membership_id, client_id, amount, base_amount, tax_amount, processing_fee_amount, currency_code, due_date, provider_reference)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (tenant_id, provider_reference) WHERE provider_reference IS NOT NULL
		DO UPDATE SET updated_at = NOW()
		RETURNING ` + paymentFields + `, (xmax = 0) AS inserted`

	var inserted bool
	row := r.db.Pool.QueryRow(ctx, query, tenantID, membershipID, clientID, amount, base, tax, fee, currencyCode, dueDate, providerRef)
	var p Payment
	var sd []byte
	err := row.Scan(
		&p.ID, &p.TenantID, &p.MembershipID, &p.ClientID, &p.Amount, &p.BaseAmount, &p.TaxAmount, &p.ProcessingFeeAmount, &p.CurrencyCode, &p.Status,
		&p.ProviderReference, &p.DueDate, &p.PaidDate, &p.FailureReason,
		&p.FailureCode, &p.RetryCount, &sd, &p.CreatedAt,
		&inserted,
	)
	if err != nil {
		return nil, false, fmt.Errorf("create or get payment: %w", err)
	}
	if len(sd) > 0 {
		p.StripeDetails = json.RawMessage(append(json.RawMessage(nil), sd...))
	}
	return &p, inserted, nil
}

// GetByID returns a payment by ID and tenant.
func (r *PaymentRepo) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*Payment, error) {
	query := `SELECT ` + paymentFields + ` FROM payments WHERE id = $1 AND tenant_id = $2`
	p, err := r.scanPaymentRow(r.db.Pool.QueryRow(ctx, query, id, tenantID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding payment by id: %w", err)
	}
	return p, nil
}

// GetByProviderReference returns a payment by tenant and GoCardless payment ID (provider_reference).
func (r *PaymentRepo) GetByProviderReference(ctx context.Context, tenantID uuid.UUID, providerRef string) (*Payment, error) {
	query := `SELECT ` + paymentFields + ` FROM payments WHERE tenant_id = $1 AND provider_reference = $2`
	p, err := r.scanPaymentRow(r.db.Pool.QueryRow(ctx, query, tenantID, providerRef))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding payment by provider reference: %w", err)
	}
	return p, nil
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

// FindLatestStripeCustomerIDByClient returns the most recent Stripe customer id seen for a client.
// It searches both membership payments and service charges stripe_details snapshots.
func (r *PaymentRepo) FindLatestStripeCustomerIDByClient(ctx context.Context, tenantID, clientID uuid.UUID) (string, error) {
	var customerID string
	err := r.db.Pool.QueryRow(ctx, `
		WITH candidates AS (
			SELECT p.stripe_details->>'stripe_customer_id' AS customer_id, p.created_at
			FROM payments p
			WHERE p.tenant_id = $1
			  AND p.client_id = $2
			  AND COALESCE(p.stripe_details->>'stripe_customer_id', '') <> ''
			UNION ALL
			SELECT sc.stripe_details->>'stripe_customer_id' AS customer_id, sc.created_at
			FROM service_charges sc
			WHERE sc.tenant_id = $1
			  AND sc.client_id = $2
			  AND COALESCE(sc.stripe_details->>'stripe_customer_id', '') <> ''
		)
		SELECT customer_id
		FROM candidates
		ORDER BY created_at DESC
		LIMIT 1
	`, tenantID, clientID).Scan(&customerID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("find latest stripe customer id by client: %w", err)
	}
	return strings.TrimSpace(customerID), nil
}

// FindLatestStripeSubscriptionIDByMembership returns the most recent Stripe subscription id seen for a membership.
func (r *PaymentRepo) FindLatestStripeSubscriptionIDByMembership(ctx context.Context, tenantID, membershipID uuid.UUID) (string, error) {
	var subscriptionID string
	err := r.db.Pool.QueryRow(ctx, `
		SELECT p.stripe_details->>'stripe_subscription_id'
		FROM payments p
		WHERE p.tenant_id = $1
		  AND p.membership_id = $2
		  AND COALESCE(p.stripe_details->>'stripe_subscription_id', '') <> ''
		ORDER BY p.created_at DESC
		LIMIT 1
	`, tenantID, membershipID).Scan(&subscriptionID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("find latest stripe subscription id by membership: %w", err)
	}
	return strings.TrimSpace(subscriptionID), nil
}

// FindLatestOpenByMembership returns the most recent unpaid payment for a membership.
func (r *PaymentRepo) FindLatestOpenByMembership(ctx context.Context, tenantID, membershipID uuid.UUID) (*Payment, error) {
	query := `SELECT ` + paymentFields + ` FROM payments
	          WHERE tenant_id = $1
	            AND membership_id = $2
	            AND status IN ('pending','failed')
	          ORDER BY due_date DESC, created_at DESC
	          LIMIT 1`
	p, err := r.scanPaymentRow(r.db.Pool.QueryRow(ctx, query, tenantID, membershipID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find latest open payment by membership: %w", err)
	}
	return p, nil
}

// MergeStripeDetails deep-merges a JSON patch into payments.stripe_details for the row
// matched by tenant + provider_reference (Stripe invoice id for subscription flows).
func (r *PaymentRepo) MergeStripeDetails(ctx context.Context, tenantID uuid.UUID, providerRef string, patch map[string]interface{}) error {
	if providerRef == "" || patch == nil || len(patch) == 0 {
		return nil
	}
	b, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal stripe_details patch: %w", err)
	}
	ct, err := r.db.Pool.Exec(ctx,
		`UPDATE payments SET stripe_details = COALESCE(stripe_details, '{}'::jsonb) || $1::jsonb, updated_at = NOW()
		 WHERE tenant_id = $2 AND provider_reference = $3`,
		b, tenantID, providerRef,
	)
	if err != nil {
		return fmt.Errorf("merge stripe_details: %w", err)
	}
	if ct.RowsAffected() == 0 {
		log.Printf("MergeStripeDetails: no payment for tenant=%s provider_reference=%s", tenantID, providerRef)
	}
	return nil
}

// UpdateStatus updates a payment's status.
func (r *PaymentRepo) UpdateStatus(ctx context.Context, id, tenantID uuid.UUID, status string, paidDate *time.Time, failureReason, failureCode *string) error {
	query := `UPDATE payments SET status=$3::text, paid_date=$4, failure_reason=$5, failure_code=$6,
		retry_count = CASE WHEN $3::text = 'failed' THEN retry_count + 1 ELSE retry_count END,
		updated_at=NOW() WHERE id=$1 AND tenant_id=$2`
	_, err := r.db.Pool.Exec(ctx, query, id, tenantID, status, paidDate, failureReason, failureCode)
	return err
}

// UpdateProcessingFeeByProviderReference sets the processing fee for a payment matched by provider reference.
func (r *PaymentRepo) UpdateProcessingFeeByProviderReference(ctx context.Context, tenantID uuid.UUID, providerRef string, feeAmount float64) error {
	if strings.TrimSpace(providerRef) == "" {
		return nil
	}
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE payments
		 SET processing_fee_amount = $3,
		     updated_at = NOW()
		 WHERE tenant_id = $1 AND provider_reference = $2`,
		tenantID, providerRef, feeAmount)
	if err != nil {
		return fmt.Errorf("update payment processing fee: %w", err)
	}
	return nil
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

func (r *PaymentRepo) scanPaymentRow(row pgx.Row) (*Payment, error) {
	var p Payment
	var sd []byte
	err := row.Scan(
		&p.ID, &p.TenantID, &p.MembershipID, &p.ClientID, &p.Amount, &p.BaseAmount, &p.TaxAmount, &p.ProcessingFeeAmount, &p.CurrencyCode, &p.Status,
		&p.ProviderReference, &p.DueDate, &p.PaidDate, &p.FailureReason,
		&p.FailureCode, &p.RetryCount, &sd, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(sd) > 0 {
		p.StripeDetails = json.RawMessage(append(json.RawMessage(nil), sd...))
	}
	return &p, nil
}

func (r *PaymentRepo) scanMany(rows pgx.Rows) ([]Payment, error) {
	var items []Payment
	for rows.Next() {
		p, err := r.scanPaymentRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *p)
	}
	return items, rows.Err()
}
