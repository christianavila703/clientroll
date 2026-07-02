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
	ID                     uuid.UUID `json:"id"`
	TenantID               uuid.UUID `json:"tenant_id"`
	ProviderName           string    `json:"provider_name"`
	APIKeyEncrypted        string    `json:"-"`
	APISecretEncrypted     string    `json:"-"`
	ConnectClientID        string    `json:"-"` // e.g. GoCardless Connect app client_id (per-tenant)
	ProviderOrganisationID string    `json:"provider_organisation_id,omitempty"`
	WebhookSecretEncrypted string    `json:"-"` // per-tenant webhook secret (e.g. GoCardless)
	Environment            string    `json:"environment"`
	IsActive               bool      `json:"is_active"`
	CreatedAt              time.Time `json:"created_at"`
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
	          COALESCE(connect_client_id,''), COALESCE(provider_organisation_id,''), COALESCE(webhook_secret_encrypted,''),
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
			&c.APISecretEncrypted, &c.ConnectClientID, &c.ProviderOrganisationID, &c.WebhookSecretEncrypted, &c.Environment, &c.IsActive, &c.CreatedAt); err != nil {
			return nil, err
		}
		creds = append(creds, c)
	}
	return creds, rows.Err()
}

// Upsert creates or updates credentials for a provider.
func (r *TenantCredentialRepo) Upsert(ctx context.Context, tenantID uuid.UUID, provider, encKey, encSecret, environment string, isActive bool) error {
	return r.UpsertWithOrganisation(ctx, tenantID, provider, encKey, encSecret, "", environment, isActive)
}

// UpsertWithOrganisation creates or updates credentials including provider_organisation_id (e.g. GoCardless org id).
// Does not overwrite connect_client_id or api_secret_encrypted when encSecret is empty (preserves tenant's Connect app credentials).
func (r *TenantCredentialRepo) UpsertWithOrganisation(ctx context.Context, tenantID uuid.UUID, provider, encKey, encSecret, organisationID, environment string, isActive bool) error {
	query := `
		INSERT INTO tenant_credentials (tenant_id, provider_name, api_key_encrypted, api_secret_encrypted, connect_client_id, provider_organisation_id, environment, is_active)
		VALUES ($1, $2, $3, $4, NULL, NULLIF($5,''), $6, $7)
		ON CONFLICT (tenant_id, provider_name) DO UPDATE SET
			api_key_encrypted = CASE WHEN $3 = '' THEN tenant_credentials.api_key_encrypted ELSE $3 END,
			api_secret_encrypted = CASE WHEN $4 = '' THEN tenant_credentials.api_secret_encrypted ELSE $4 END,
			provider_organisation_id = COALESCE(NULLIF($5,''), tenant_credentials.provider_organisation_id),
			environment = $6, is_active = $7, updated_at = NOW()`

	_, err := r.db.Pool.Exec(ctx, query, tenantID, provider, encKey, encSecret, organisationID, environment, isActive)
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
	          COALESCE(connect_client_id,''), COALESCE(provider_organisation_id,''), COALESCE(webhook_secret_encrypted,''),
	          environment, is_active, created_at
	          FROM tenant_credentials WHERE tenant_id = $1 AND provider_name = $2`

	var c TenantCredential
	err := r.db.Pool.QueryRow(ctx, query, tenantID, provider).Scan(
		&c.ID, &c.TenantID, &c.ProviderName, &c.APIKeyEncrypted,
		&c.APISecretEncrypted, &c.ConnectClientID, &c.ProviderOrganisationID, &c.WebhookSecretEncrypted, &c.Environment, &c.IsActive, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// SetWebhookSecret stores the encrypted webhook secret for a provider (e.g. GoCardless per-tenant).
func (r *TenantCredentialRepo) SetWebhookSecret(ctx context.Context, tenantID uuid.UUID, provider, encWebhookSecret string) error {
	query := `UPDATE tenant_credentials SET webhook_secret_encrypted = $3, updated_at = NOW()
	          WHERE tenant_id = $1 AND provider_name = $2`
	_, err := r.db.Pool.Exec(ctx, query, tenantID, provider, encWebhookSecret)
	return err
}

// GetByOrganisation returns credentials for a provider by organisation ID (e.g. webhook: resolve tenant then get creds).
func (r *TenantCredentialRepo) GetByOrganisation(ctx context.Context, provider, organisationID string) (*TenantCredential, error) {
	tenantID, err := r.GetTenantIDByOrganisation(ctx, provider, organisationID)
	if err != nil {
		return nil, err
	}
	return r.GetByProvider(ctx, tenantID, provider)
}

// SetConnectAppCredentials stores a tenant's Connect app client_id and encrypted client_secret (e.g. GoCardless).
// Used before OAuth; access token is stored later via UpsertWithOrganisation.
func (r *TenantCredentialRepo) SetConnectAppCredentials(ctx context.Context, tenantID uuid.UUID, provider, connectClientID, encClientSecret, environment string) error {
	query := `
		INSERT INTO tenant_credentials (tenant_id, provider_name, api_key_encrypted, api_secret_encrypted, connect_client_id, provider_organisation_id, environment, is_active)
		VALUES ($1, $2, '', $3, NULLIF($4,''), '', $5, true)
		ON CONFLICT (tenant_id, provider_name) DO UPDATE SET
			connect_client_id = NULLIF($4,''),
			api_secret_encrypted = CASE WHEN $3 = '' THEN tenant_credentials.api_secret_encrypted ELSE $3 END,
			environment = $5, updated_at = NOW()`
	_, err := r.db.Pool.Exec(ctx, query, tenantID, provider, encClientSecret, connectClientID, environment)
	return err
}

// GetTenantIDByOrganisation returns tenant_id for a provider's organisation_id (e.g. webhook routing).
// Uses a SECURITY DEFINER function so it works when no tenant context is set (e.g. webhook handler).
func (r *TenantCredentialRepo) GetTenantIDByOrganisation(ctx context.Context, provider, organisationID string) (uuid.UUID, error) {
	query := `SELECT get_tenant_id_by_provider_organisation($1, $2)`
	var id uuid.UUID
	err := r.db.Pool.QueryRow(ctx, query, provider, organisationID).Scan(&id)
	return id, err
}
