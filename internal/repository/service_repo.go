package repository

import (
	"context"
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
	ID          uuid.UUID  `json:"id"`
	TenantID    uuid.UUID  `json:"tenant_id"`
	ClientID    uuid.UUID  `json:"client_id"`
	ServiceID   *uuid.UUID `json:"service_id"`
	Description string     `json:"description"`
	Amount      float64    `json:"amount"`
	TaxAmount   float64    `json:"tax_amount"`
	Status      string     `json:"status"`
	PaidDate    *time.Time `json:"paid_date"`
	Notes       *string    `json:"notes"`
	CreatedBy   *uuid.UUID `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
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
		 FROM services WHERE tenant_id=$1 ORDER BY sort_order, name`, tenantID)
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
		 WHERE id=$1 AND tenant_id=$2
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
	_, err := r.db.Pool.Exec(ctx, "DELETE FROM services WHERE id=$1 AND tenant_id=$2", id, tenantID)
	return err
}

// CreateCharge creates a one-off service charge for a client.
func (r *ServiceRepo) CreateCharge(ctx context.Context, tenantID, clientID uuid.UUID, serviceID *uuid.UUID, description string, amount, taxAmount float64, notes *string, createdBy *uuid.UUID) (*ServiceCharge, error) {
	var sc ServiceCharge
	now := time.Now()
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO service_charges (tenant_id, client_id, service_id, description, amount, tax_amount, status, paid_date, notes, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, 'succeeded', $7, $8, $9)
		 RETURNING id, tenant_id, client_id, service_id, description, amount, tax_amount, status, paid_date, notes, created_by, created_at`,
		tenantID, clientID, serviceID, description, amount, taxAmount, &now, notes, createdBy).Scan(
		&sc.ID, &sc.TenantID, &sc.ClientID, &sc.ServiceID, &sc.Description,
		&sc.Amount, &sc.TaxAmount, &sc.Status, &sc.PaidDate, &sc.Notes, &sc.CreatedBy, &sc.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating service charge: %w", err)
	}
	return &sc, nil
}

// ListChargesByClient returns service charges for a client.
func (r *ServiceRepo) ListChargesByClient(ctx context.Context, clientID, tenantID uuid.UUID) ([]ServiceCharge, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, tenant_id, client_id, service_id, description, amount, tax_amount, status, paid_date, notes, created_by, created_at
		 FROM service_charges WHERE client_id=$1 AND tenant_id=$2 ORDER BY created_at DESC`, clientID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ServiceCharge
	for rows.Next() {
		var sc ServiceCharge
		if err := rows.Scan(&sc.ID, &sc.TenantID, &sc.ClientID, &sc.ServiceID, &sc.Description,
			&sc.Amount, &sc.TaxAmount, &sc.Status, &sc.PaidDate, &sc.Notes, &sc.CreatedBy, &sc.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, sc)
	}
	return items, rows.Err()
}

// Needed to satisfy pgx unused import
var _ = pgx.ErrNoRows
