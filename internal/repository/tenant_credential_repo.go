package repository

import (
	"context"
	"fmt"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
)

// TenantCredential represents stored API credentials.
type TenantCredential struct {
	ID                uuid.UUID `json:"id"`
	TenantID          uuid.UUID `json:"tenant_id"`
	ProviderName      string    `json:"provider_name"`
	APIKeyEncrypted   string    `json:"-"`
	APISecretEncrypted string   `json:"-"`
	Environment       string    `json:"environment"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
}

// TenantCredentialRepo handles DB operations for tenant credentials.
type TenantCredentialRepo struct {
	db *database.Postgres
}

// NewTenantCredentialRepo creates a new repo.
func NewTenantCredentialRepo(db *database.Postgres) *TenantCredentialRepo {
	return &TenantCredentialRepo{db: db}
}

// ListByTenant returns all credentials for a tenant.
func (r *TenantCredentialRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]TenantCredential, error) {
	query := `SELECT id, tenant_id, provider_name, COALESCE(api_key_encrypted,''), COALESCE(api_secret_encrypted,''),
	          environment, is_active, created_at
	          FROM tenant_credentials WHERE tenant_id = $1 ORDER BY provider_name`

	rows, err := r.db.Pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listing credentials: %w", err)
	}
	defer rows.Close()

	var creds []TenantCredential
	for rows.Next() {
		var c TenantCredential
		if err := rows.Scan(&c.ID, &c.TenantID, &c.ProviderName, &c.APIKeyEncrypted,
			&c.APISecretEncrypted, &c.Environment, &c.IsActive, &c.CreatedAt); err != nil {
			return nil, err
		}
		creds = append(creds, c)
	}
	return creds, rows.Err()
}

// Upsert creates or updates credentials for a provider.
func (r *TenantCredentialRepo) Upsert(ctx context.Context, tenantID uuid.UUID, provider, encKey, encSecret, environment string, isActive bool) error {
	query := `
		INSERT INTO tenant_credentials (tenant_id, provider_name, api_key_encrypted, api_secret_encrypted, environment, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tenant_id, provider_name) DO UPDATE SET
			api_key_encrypted = CASE WHEN $3 = '' THEN tenant_credentials.api_key_encrypted ELSE $3 END,
			api_secret_encrypted = CASE WHEN $4 = '' THEN tenant_credentials.api_secret_encrypted ELSE $4 END,
			environment = $5, is_active = $6, updated_at = NOW()`

	_, err := r.db.Pool.Exec(ctx, query, tenantID, provider, encKey, encSecret, environment, isActive)
	return err
}

// Delete removes credentials for a provider.
func (r *TenantCredentialRepo) Delete(ctx context.Context, tenantID uuid.UUID, provider string) error {
	_, err := r.db.Pool.Exec(ctx, "DELETE FROM tenant_credentials WHERE tenant_id=$1 AND provider_name=$2", tenantID, provider)
	return err
}

// GetByProvider returns credentials for a specific provider.
func (r *TenantCredentialRepo) GetByProvider(ctx context.Context, tenantID uuid.UUID, provider string) (*TenantCredential, error) {
	query := `SELECT id, tenant_id, provider_name, COALESCE(api_key_encrypted,''), COALESCE(api_secret_encrypted,''),
	          environment, is_active, created_at
	          FROM tenant_credentials WHERE tenant_id = $1 AND provider_name = $2`

	var c TenantCredential
	err := r.db.Pool.QueryRow(ctx, query, tenantID, provider).Scan(
		&c.ID, &c.TenantID, &c.ProviderName, &c.APIKeyEncrypted,
		&c.APISecretEncrypted, &c.Environment, &c.IsActive, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
