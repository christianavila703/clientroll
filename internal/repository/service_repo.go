package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	ID           uuid.UUID `json:"id"`
	TenantID     uuid.UUID `json:"tenant_id"`
	Name         string    `json:"name"`
	Description  *string   `json:"description"`
	Price        float64   `json:"price"`
	CurrencyCode string    `json:"currency_code"`
	TaxRate      float64   `json:"tax_rate"`
	IsActive     bool      `json:"is_active"`
	SortOrder    int       `json:"sort_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ServiceCharge struct {
	ID                  uuid.UUID       `json:"id"`
	TenantID            uuid.UUID       `json:"tenant_id"`
	ClientID            uuid.UUID       `json:"client_id"`
	ServiceID           *uuid.UUID      `json:"service_id"`
	Description         string          `json:"description"`
	Amount              float64         `json:"amount"`
	TaxAmount           float64         `json:"tax_amount"`
	ProcessingFeeAmount float64         `json:"processing_fee_amount"`
	CurrencyCode        string          `json:"currency_code"`
	Status              string          `json:"status"`
	PaidDate            *time.Time      `json:"paid_date"`
	Notes               *string         `json:"notes"`
	CreatedBy           *uuid.UUID      `json:"created_by"`
	ProviderReference   *string         `json:"provider_reference,omitempty"`
	PaymentMethod       *string         `json:"payment_method,omitempty"`
	StripeDetails       json.RawMessage `json:"stripe_details,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
}

type ServiceRepo struct {
	db *database.Postgres
}

func NewServiceRepo(db *database.Postgres) *ServiceRepo {
	return &ServiceRepo{db: db}
}

func (r *ServiceRepo) List(ctx context.Context, tenantID uuid.UUID) ([]Service, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, tenant_id, name, description, price, currency_code, tax_rate, is_active, sort_order, created_at, updated_at
		 FROM services WHERE tenant_id=$1 AND deleted_at IS NULL ORDER BY sort_order, name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Service
	for rows.Next() {
		var s Service
		if err := rows.Scan(&s.ID, &s.TenantID, &s.Name, &s.Description, &s.Price, &s.CurrencyCode,
			&s.TaxRate, &s.IsActive, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func (r *ServiceRepo) Create(ctx context.Context, tenantID uuid.UUID, name string, description *string, price float64, currencyCode string, taxRate float64, sortOrder int) (*Service, error) {
	var s Service
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO services (tenant_id, name, description, price, currency_code, tax_rate, sort_order)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, tenant_id, name, description, price, currency_code, tax_rate, is_active, sort_order, created_at, updated_at`,
		tenantID, name, description, price, currencyCode, taxRate, sortOrder).Scan(
		&s.ID, &s.TenantID, &s.Name, &s.Description, &s.Price, &s.CurrencyCode,
		&s.TaxRate, &s.IsActive, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating service: %w", err)
	}
	return &s, nil
}

func (r *ServiceRepo) Update(ctx context.Context, id, tenantID uuid.UUID, name string, description *string, price float64, currencyCode string, taxRate float64, isActive bool, sortOrder int) (*Service, error) {
	var s Service
	err := r.db.Pool.QueryRow(ctx,
		`UPDATE services SET name=$3, description=$4, price=$5, currency_code=$6, tax_rate=$7, is_active=$8, sort_order=$9, updated_at=NOW()
		 WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL
		 RETURNING id, tenant_id, name, description, price, currency_code, tax_rate, is_active, sort_order, created_at, updated_at`,
		id, tenantID, name, description, price, currencyCode, taxRate, isActive, sortOrder).Scan(
		&s.ID, &s.TenantID, &s.Name, &s.Description, &s.Price, &s.CurrencyCode,
		&s.TaxRate, &s.IsActive, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating service: %w", err)
	}
	return &s, nil
}

func (r *ServiceRepo) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE services
		 SET deleted_at = NOW(), is_active = false, updated_at = NOW()
		 WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`,
		id, tenantID,
	)
	return err
}

// CreateCharge creates a one-off service charge for a client.
func (r *ServiceRepo) CreateCharge(ctx context.Context, tenantID, clientID uuid.UUID, serviceID *uuid.UUID, description string, amount, taxAmount float64, notes *string, createdBy *uuid.UUID) (*ServiceCharge, error) {
	now := time.Now()
	return r.createChargeWithState(ctx, tenantID, clientID, serviceID, description, amount, taxAmount, 0, "succeeded", &now, notes, createdBy)
}

// CreatePendingCharge creates a service charge in pending state. Use this before
// redirecting to hosted checkout so reports only include it after successful payment.
func (r *ServiceRepo) CreatePendingCharge(ctx context.Context, tenantID, clientID uuid.UUID, serviceID *uuid.UUID, description string, amount, taxAmount, processingFeeAmount float64, notes *string, createdBy *uuid.UUID) (*ServiceCharge, error) {
	return r.createChargeWithState(ctx, tenantID, clientID, serviceID, description, amount, taxAmount, processingFeeAmount, "pending", nil, notes, createdBy)
}

func (r *ServiceRepo) createChargeWithState(ctx context.Context, tenantID, clientID uuid.UUID, serviceID *uuid.UUID, description string, amount, taxAmount, processingFeeAmount float64, status string, paidDate *time.Time, notes *string, createdBy *uuid.UUID) (*ServiceCharge, error) {
	currencyCode := "AUD"
	if serviceID != nil {
		if err := r.db.Pool.QueryRow(ctx, `SELECT currency_code FROM services WHERE id=$1 AND tenant_id=$2`, *serviceID, tenantID).Scan(&currencyCode); err != nil {
			// Fallback to AUD so ad-hoc service charges remain chargeable even if the linked service was deleted.
			currencyCode = "AUD"
		}
	}
	var sc ServiceCharge
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO service_charges (tenant_id, client_id, service_id, description, amount, tax_amount, processing_fee_amount, currency_code, status, paid_date, notes, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id, tenant_id, client_id, service_id, description, amount, tax_amount, processing_fee_amount, currency_code, status, paid_date, notes, created_by, provider_reference, payment_method, COALESCE(stripe_details, '{}'::jsonb), created_at`,
		tenantID, clientID, serviceID, description, amount, taxAmount, processingFeeAmount, currencyCode, status, paidDate, notes, createdBy).Scan(
		&sc.ID, &sc.TenantID, &sc.ClientID, &sc.ServiceID, &sc.Description,
		&sc.Amount, &sc.TaxAmount, &sc.ProcessingFeeAmount, &sc.CurrencyCode, &sc.Status, &sc.PaidDate, &sc.Notes, &sc.CreatedBy, &sc.ProviderReference, &sc.PaymentMethod, &sc.StripeDetails, &sc.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating service charge: %w", err)
	}
	return &sc, nil
}

// UpdateChargeStatus updates a service charge status and paid_date.
func (r *ServiceRepo) UpdateChargeStatus(ctx context.Context, tenantID, chargeID uuid.UUID, status string, paidDate *time.Time) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE service_charges
		 SET status = $3, paid_date = $4
		 WHERE tenant_id = $1 AND id = $2`,
		tenantID, chargeID, status, paidDate)
	if err != nil {
		return fmt.Errorf("updating service charge status: %w", err)
	}
	return nil
}

// UpdateChargeProcessingFee updates the reconciled processing fee amount for a service charge.
func (r *ServiceRepo) UpdateChargeProcessingFee(ctx context.Context, tenantID, chargeID uuid.UUID, feeAmount float64) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE service_charges
		 SET processing_fee_amount = $3,
		     updated_at = NOW()
		 WHERE tenant_id = $1 AND id = $2`,
		tenantID, chargeID, feeAmount)
	if err != nil {
		return fmt.Errorf("updating service charge processing fee: %w", err)
	}
	return nil
}

// ListChargesByClient returns service charges for a client.
func (r *ServiceRepo) ListChargesByClient(ctx context.Context, clientID, tenantID uuid.UUID) ([]ServiceCharge, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, tenant_id, client_id, service_id, description, amount, tax_amount, processing_fee_amount, currency_code, status, paid_date, notes, created_by, provider_reference, payment_method, COALESCE(stripe_details, '{}'::jsonb), created_at
		 FROM service_charges WHERE client_id=$1 AND tenant_id=$2 ORDER BY created_at DESC`, clientID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ServiceCharge
	for rows.Next() {
		var sc ServiceCharge
		if err := rows.Scan(&sc.ID, &sc.TenantID, &sc.ClientID, &sc.ServiceID, &sc.Description,
			&sc.Amount, &sc.TaxAmount, &sc.ProcessingFeeAmount, &sc.CurrencyCode, &sc.Status, &sc.PaidDate, &sc.Notes, &sc.CreatedBy, &sc.ProviderReference, &sc.PaymentMethod, &sc.StripeDetails, &sc.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, sc)
	}
	return items, rows.Err()
}

func (r *ServiceRepo) FindChargeByID(ctx context.Context, tenantID, chargeID uuid.UUID) (*ServiceCharge, error) {
	query := `SELECT id, tenant_id, client_id, service_id, description, amount, tax_amount, processing_fee_amount, currency_code, status, paid_date, notes, created_by, provider_reference, payment_method, COALESCE(stripe_details, '{}'::jsonb), created_at
	          FROM service_charges
	          WHERE tenant_id=$1 AND id=$2`
	var sc ServiceCharge
	err := r.db.Pool.QueryRow(ctx, query, tenantID, chargeID).Scan(
		&sc.ID, &sc.TenantID, &sc.ClientID, &sc.ServiceID, &sc.Description, &sc.Amount, &sc.TaxAmount, &sc.ProcessingFeeAmount, &sc.CurrencyCode, &sc.Status, &sc.PaidDate, &sc.Notes, &sc.CreatedBy, &sc.ProviderReference, &sc.PaymentMethod, &sc.StripeDetails, &sc.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &sc, nil
}

// UpdateChargeProviderContext stores provider reference/method hints for a service charge.
func (r *ServiceRepo) UpdateChargeProviderContext(ctx context.Context, tenantID, chargeID uuid.UUID, providerRef, paymentMethod *string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE service_charges
		 SET provider_reference = COALESCE(provider_reference, $3),
		     payment_method = COALESCE(payment_method, $4),
		     updated_at = NOW()
		 WHERE tenant_id = $1 AND id = $2`,
		tenantID, chargeID, providerRef, paymentMethod)
	if err != nil {
		return fmt.Errorf("updating service charge provider context: %w", err)
	}
	return nil
}

// MergeStripeDetailsByChargeID deep-merges details into service_charges.stripe_details.
func (r *ServiceRepo) MergeStripeDetailsByChargeID(ctx context.Context, tenantID, chargeID uuid.UUID, patch map[string]interface{}) error {
	if len(patch) == 0 {
		return nil
	}
	payload, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal service stripe_details patch: %w", err)
	}
	ct, err := r.db.Pool.Exec(ctx,
		`UPDATE service_charges
		 SET stripe_details = COALESCE(stripe_details, '{}'::jsonb) || $3::jsonb,
		     updated_at = NOW()
		 WHERE tenant_id = $1 AND id = $2`,
		tenantID, chargeID, payload)
	if err != nil {
		return fmt.Errorf("merge service stripe_details by charge id: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return nil
	}
	return nil
}

// Needed to satisfy pgx unused import
var _ = pgx.ErrNoRows
