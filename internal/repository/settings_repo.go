package repository

import (
	"context"
	"fmt"

	"clientroll/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AgreementSettings represents Section A agreement details.
type AgreementSettings struct {
	ID           uuid.UUID `json:"id"`
	TenantID     uuid.UUID `json:"tenant_id"`
	CompanyName  *string   `json:"company_name"`
	BusinessName *string   `json:"business_name"`
	ABNACN       *string   `json:"abn_acn"`
	TradingName  *string   `json:"trading_name"`
	Address      *string   `json:"address"`
	State        *string   `json:"state"`
	Postcode     *string   `json:"postcode"`
	Email        *string   `json:"email"`
	Phone        *string   `json:"phone"`
}

// SettingsRepo handles database operations for tenant settings.
type SettingsRepo struct {
	db *database.Postgres
}

// NewSettingsRepo creates a new settings repository.
func NewSettingsRepo(db *database.Postgres) *SettingsRepo {
	return &SettingsRepo{db: db}
}

// GetAgreementSettings returns the agreement settings for a tenant.
func (r *SettingsRepo) GetAgreementSettings(ctx context.Context, tenantID uuid.UUID) (*AgreementSettings, error) {
	query := `
		SELECT id, tenant_id, company_name, business_name, abn_acn,
		       trading_name, address, state, postcode, email, phone
		FROM agreement_settings
		WHERE tenant_id = $1`

	var s AgreementSettings
	err := r.db.Pool.QueryRow(ctx, query, tenantID).Scan(
		&s.ID, &s.TenantID, &s.CompanyName, &s.BusinessName, &s.ABNACN,
		&s.TradingName, &s.Address, &s.State, &s.Postcode, &s.Email, &s.Phone,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting agreement settings: %w", err)
	}
	return &s, nil
}

// UpsertAgreementSettings creates or updates agreement settings.
func (r *SettingsRepo) UpsertAgreementSettings(ctx context.Context, tenantID uuid.UUID, input AgreementSettings) (*AgreementSettings, error) {
	query := `
		INSERT INTO agreement_settings (tenant_id, company_name, business_name, abn_acn,
		    trading_name, address, state, postcode, email, phone)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (tenant_id) DO UPDATE SET
		    company_name = COALESCE($2, agreement_settings.company_name),
		    business_name = COALESCE($3, agreement_settings.business_name),
		    abn_acn = COALESCE($4, agreement_settings.abn_acn),
		    trading_name = COALESCE($5, agreement_settings.trading_name),
		    address = COALESCE($6, agreement_settings.address),
		    state = COALESCE($7, agreement_settings.state),
		    postcode = COALESCE($8, agreement_settings.postcode),
		    email = COALESCE($9, agreement_settings.email),
		    phone = COALESCE($10, agreement_settings.phone),
		    updated_at = NOW()
		RETURNING id, tenant_id, company_name, business_name, abn_acn,
		          trading_name, address, state, postcode, email, phone`

	var s AgreementSettings
	err := r.db.Pool.QueryRow(ctx, query, tenantID,
		input.CompanyName, input.BusinessName, input.ABNACN,
		input.TradingName, input.Address, input.State,
		input.Postcode, input.Email, input.Phone,
	).Scan(
		&s.ID, &s.TenantID, &s.CompanyName, &s.BusinessName, &s.ABNACN,
		&s.TradingName, &s.Address, &s.State, &s.Postcode, &s.Email, &s.Phone,
	)
	if err != nil {
		return nil, fmt.Errorf("upserting agreement settings: %w", err)
	}
	return &s, nil
}
