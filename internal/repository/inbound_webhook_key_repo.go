package repository

import (
	"context"
	"fmt"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// InboundWebhookKey is a tenant-scoped API key used for inbound webhook auth.
type InboundWebhookKey struct {
	ID              uuid.UUID  `json:"id"`
	TenantID        uuid.UUID  `json:"tenant_id"`
	KeyID           string     `json:"key_id"`
	SecretEncrypted string     `json:"-"`
	Label           *string    `json:"label,omitempty"`
	IsActive        bool       `json:"is_active"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	RevokedAt       *time.Time `json:"revoked_at,omitempty"`
}

// InboundWebhookKeyRepo handles DB operations for inbound webhook keys.
type InboundWebhookKeyRepo struct {
	db *database.Postgres
}

// NewInboundWebhookKeyRepo creates a new inbound webhook key repository.
func NewInboundWebhookKeyRepo(db *database.Postgres) *InboundWebhookKeyRepo {
	return &InboundWebhookKeyRepo{db: db}
}

// Create inserts a new inbound webhook key for the tenant.
func (r *InboundWebhookKeyRepo) Create(ctx context.Context, tenantID uuid.UUID, keyID, secretEncrypted string, label *string) (*InboundWebhookKey, error) {
	query := `
		INSERT INTO inbound_webhook_keys (tenant_id, key_id, secret_encrypted, label, is_active)
		VALUES ($1, $2, $3, $4, true)
		RETURNING id, tenant_id, key_id, secret_encrypted, label, is_active, last_used_at, created_at, revoked_at
	`

	var out InboundWebhookKey
	err := r.db.Pool.QueryRow(ctx, query, tenantID, keyID, secretEncrypted, label).Scan(
		&out.ID, &out.TenantID, &out.KeyID, &out.SecretEncrypted, &out.Label,
		&out.IsActive, &out.LastUsedAt, &out.CreatedAt, &out.RevokedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create inbound webhook key: %w", err)
	}
	return &out, nil
}

// ListByTenant returns all inbound webhook keys for a tenant.
func (r *InboundWebhookKeyRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]InboundWebhookKey, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, tenant_id, key_id, secret_encrypted, label, is_active, last_used_at, created_at, revoked_at
		FROM inbound_webhook_keys
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list inbound webhook keys: %w", err)
	}
	defer rows.Close()

	var keys []InboundWebhookKey
	for rows.Next() {
		var k InboundWebhookKey
		if err := rows.Scan(
			&k.ID, &k.TenantID, &k.KeyID, &k.SecretEncrypted, &k.Label,
			&k.IsActive, &k.LastUsedAt, &k.CreatedAt, &k.RevokedAt,
		); err != nil {
			return nil, fmt.Errorf("scan inbound webhook key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// FindByTenantAndKeyID returns a key for the tenant by key_id.
func (r *InboundWebhookKeyRepo) FindByTenantAndKeyID(ctx context.Context, tenantID uuid.UUID, keyID string) (*InboundWebhookKey, error) {
	var out InboundWebhookKey
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, tenant_id, key_id, secret_encrypted, label, is_active, last_used_at, created_at, revoked_at
		FROM inbound_webhook_keys
		WHERE tenant_id = $1 AND key_id = $2
	`, tenantID, keyID).Scan(
		&out.ID, &out.TenantID, &out.KeyID, &out.SecretEncrypted, &out.Label,
		&out.IsActive, &out.LastUsedAt, &out.CreatedAt, &out.RevokedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find inbound webhook key by tenant/key id: %w", err)
	}
	return &out, nil
}

// FindActiveByKeyID returns an active key by key_id (public lookup path).
func (r *InboundWebhookKeyRepo) FindActiveByKeyID(ctx context.Context, keyID string) (*InboundWebhookKey, error) {
	var out InboundWebhookKey
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, tenant_id, key_id, secret_encrypted, label, is_active, last_used_at, created_at, revoked_at
		FROM inbound_webhook_keys
		WHERE key_id = $1 AND is_active = true AND revoked_at IS NULL
	`, keyID).Scan(
		&out.ID, &out.TenantID, &out.KeyID, &out.SecretEncrypted, &out.Label,
		&out.IsActive, &out.LastUsedAt, &out.CreatedAt, &out.RevokedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find active inbound webhook key by key id: %w", err)
	}
	return &out, nil
}

// RevokeByKeyID deactivates and revokes a key for a tenant.
func (r *InboundWebhookKeyRepo) RevokeByKeyID(ctx context.Context, tenantID uuid.UUID, keyID string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE inbound_webhook_keys
		SET is_active = false, revoked_at = NOW()
		WHERE tenant_id = $1 AND key_id = $2 AND revoked_at IS NULL
	`, tenantID, keyID)
	if err != nil {
		return fmt.Errorf("revoke inbound webhook key: %w", err)
	}
	return nil
}

// TouchLastUsed sets last_used_at for a key.
func (r *InboundWebhookKeyRepo) TouchLastUsed(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE inbound_webhook_keys
		SET last_used_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("update inbound webhook key last used timestamp: %w", err)
	}
	return nil
}
