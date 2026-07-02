package repository

import (
	"context"
	"fmt"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
)

type XeroConnection struct {
	ID                        uuid.UUID `json:"id"`
	TenantID                  uuid.UUID `json:"tenant_id"`
	AccessTokenEncrypted      string    `json:"-"`
	RefreshTokenEncrypted     string    `json:"-"`
	TokenExpiry               time.Time `json:"token_expiry"`
	XeroTenantID              string    `json:"xero_tenant_id"`
	XeroTenantName            *string   `json:"xero_tenant_name,omitempty"`
	DefaultRevenueAccountCode string    `json:"default_revenue_account_code"`
	DefaultPaymentAccountCode string    `json:"default_payment_account_code"`
	IsActive                  bool      `json:"is_active"`
	NeedsReauth               bool      `json:"needs_reauth"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

type XeroConnectionRepo struct {
	db *database.Postgres
}

func NewXeroConnectionRepo(db *database.Postgres) *XeroConnectionRepo {
	return &XeroConnectionRepo{db: db}
}

func (r *XeroConnectionRepo) GetByTenant(ctx context.Context, tenantID uuid.UUID) (*XeroConnection, error) {
	query := `SELECT id, tenant_id, access_token_encrypted, refresh_token_encrypted, token_expiry,
	          xero_tenant_id, xero_tenant_name, default_revenue_account_code, default_payment_account_code,
	          is_active, needs_reauth, created_at, updated_at
	          FROM xero_connections
	          WHERE tenant_id = $1`
	var c XeroConnection
	err := r.db.Pool.QueryRow(ctx, query, tenantID).Scan(
		&c.ID, &c.TenantID, &c.AccessTokenEncrypted, &c.RefreshTokenEncrypted, &c.TokenExpiry,
		&c.XeroTenantID, &c.XeroTenantName, &c.DefaultRevenueAccountCode, &c.DefaultPaymentAccountCode,
		&c.IsActive, &c.NeedsReauth, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

type UpsertXeroConnectionInput struct {
	AccessTokenEncrypted      string
	RefreshTokenEncrypted     string
	TokenExpiry               time.Time
	XeroTenantID              string
	XeroTenantName            *string
	DefaultRevenueAccountCode string
	DefaultPaymentAccountCode string
	IsActive                  bool
	NeedsReauth               bool
}

func (r *XeroConnectionRepo) UpsertByTenant(ctx context.Context, tenantID uuid.UUID, in UpsertXeroConnectionInput) (*XeroConnection, error) {
	if in.DefaultRevenueAccountCode == "" {
		in.DefaultRevenueAccountCode = "200"
	}
	if in.DefaultPaymentAccountCode == "" {
		in.DefaultPaymentAccountCode = "090"
	}
	query := `
		INSERT INTO xero_connections (
			tenant_id, access_token_encrypted, refresh_token_encrypted, token_expiry,
			xero_tenant_id, xero_tenant_name, default_revenue_account_code, default_payment_account_code,
			is_active, needs_reauth
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (tenant_id) DO UPDATE SET
			access_token_encrypted = EXCLUDED.access_token_encrypted,
			refresh_token_encrypted = EXCLUDED.refresh_token_encrypted,
			token_expiry = EXCLUDED.token_expiry,
			xero_tenant_id = EXCLUDED.xero_tenant_id,
			xero_tenant_name = EXCLUDED.xero_tenant_name,
			default_revenue_account_code = EXCLUDED.default_revenue_account_code,
			default_payment_account_code = EXCLUDED.default_payment_account_code,
			is_active = EXCLUDED.is_active,
			needs_reauth = EXCLUDED.needs_reauth,
			updated_at = NOW()
		RETURNING id, tenant_id, access_token_encrypted, refresh_token_encrypted, token_expiry,
		          xero_tenant_id, xero_tenant_name, default_revenue_account_code, default_payment_account_code,
		          is_active, needs_reauth, created_at, updated_at`

	var c XeroConnection
	err := r.db.Pool.QueryRow(ctx, query,
		tenantID,
		in.AccessTokenEncrypted,
		in.RefreshTokenEncrypted,
		in.TokenExpiry,
		in.XeroTenantID,
		in.XeroTenantName,
		in.DefaultRevenueAccountCode,
		in.DefaultPaymentAccountCode,
		in.IsActive,
		in.NeedsReauth,
	).Scan(
		&c.ID, &c.TenantID, &c.AccessTokenEncrypted, &c.RefreshTokenEncrypted, &c.TokenExpiry,
		&c.XeroTenantID, &c.XeroTenantName, &c.DefaultRevenueAccountCode, &c.DefaultPaymentAccountCode,
		&c.IsActive, &c.NeedsReauth, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert xero connection: %w", err)
	}
	return &c, nil
}

func (r *XeroConnectionRepo) UpdateConfig(ctx context.Context, tenantID uuid.UUID, revenueCode, paymentCode string) error {
	query := `UPDATE xero_connections
	          SET default_revenue_account_code = $2,
	              default_payment_account_code = $3,
	              updated_at = NOW()
	          WHERE tenant_id = $1`
	_, err := r.db.Pool.Exec(ctx, query, tenantID, revenueCode, paymentCode)
	if err != nil {
		return fmt.Errorf("update xero config: %w", err)
	}
	return nil
}

func (r *XeroConnectionRepo) SetNeedsReauth(ctx context.Context, tenantID uuid.UUID, needsReauth bool) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE xero_connections SET needs_reauth=$2, updated_at=NOW() WHERE tenant_id=$1`,
		tenantID, needsReauth,
	)
	return err
}

func (r *XeroConnectionRepo) Deactivate(ctx context.Context, tenantID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE xero_connections
		 SET is_active = false,
		     needs_reauth = false,
		     access_token_encrypted = '',
		     refresh_token_encrypted = '',
		     updated_at = NOW()
		 WHERE tenant_id = $1`,
		tenantID,
	)
	return err
}
