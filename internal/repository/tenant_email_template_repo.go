package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type TenantEmailTemplate struct {
	TenantID        uuid.UUID `json:"tenant_id"`
	TemplateKey     string    `json:"template_key"`
	Category        string    `json:"category"`
	Enabled         bool      `json:"enabled"`
	SubjectOverride *string   `json:"subject_override,omitempty"`
	BodyOverride    *string   `json:"body_override,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type TenantEmailTemplateRepo struct {
	db *database.Postgres
}

func NewTenantEmailTemplateRepo(db *database.Postgres) *TenantEmailTemplateRepo {
	return &TenantEmailTemplateRepo{db: db}
}

func (r *TenantEmailTemplateRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]TenantEmailTemplate, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT tenant_id, template_key, category, enabled, subject_override, body_override, created_at, updated_at
		FROM tenant_email_templates
		WHERE tenant_id = $1
		ORDER BY category, template_key
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list tenant email templates: %w", err)
	}
	defer rows.Close()

	items := make([]TenantEmailTemplate, 0)
	for rows.Next() {
		var item TenantEmailTemplate
		if err := rows.Scan(
			&item.TenantID,
			&item.TemplateKey,
			&item.Category,
			&item.Enabled,
			&item.SubjectOverride,
			&item.BodyOverride,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan tenant email template: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tenant email templates: %w", err)
	}
	return items, nil
}

func (r *TenantEmailTemplateRepo) Upsert(
	ctx context.Context,
	tenantID uuid.UUID,
	templateKey string,
	category string,
	enabled bool,
	subjectOverride *string,
	bodyOverride *string,
) (*TenantEmailTemplate, error) {
	var out TenantEmailTemplate
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO tenant_email_templates (tenant_id, template_key, category, enabled, subject_override, body_override)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tenant_id, template_key) DO UPDATE SET
			category = EXCLUDED.category,
			enabled = EXCLUDED.enabled,
			subject_override = EXCLUDED.subject_override,
			body_override = EXCLUDED.body_override,
			updated_at = NOW()
		RETURNING tenant_id, template_key, category, enabled, subject_override, body_override, created_at, updated_at
	`, tenantID, templateKey, category, enabled, subjectOverride, bodyOverride).Scan(
		&out.TenantID,
		&out.TemplateKey,
		&out.Category,
		&out.Enabled,
		&out.SubjectOverride,
		&out.BodyOverride,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert tenant email template: %w", err)
	}
	return &out, nil
}

func (r *TenantEmailTemplateRepo) ToggleCategory(
	ctx context.Context,
	tenantID uuid.UUID,
	category string,
	templateKeys []string,
	enabled bool,
) error {
	if len(templateKeys) == 0 {
		return nil
	}
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO tenant_email_templates (tenant_id, template_key, category, enabled)
		SELECT $1, key_name, $2, $3
		FROM unnest($4::text[]) AS key_name
		ON CONFLICT (tenant_id, template_key) DO UPDATE SET
			category = EXCLUDED.category,
			enabled = EXCLUDED.enabled,
			updated_at = NOW()
	`, tenantID, category, enabled, templateKeys)
	if err != nil {
		return fmt.Errorf("toggle tenant email template category: %w", err)
	}
	return nil
}

func (r *TenantEmailTemplateRepo) IsEnabled(ctx context.Context, tenantID uuid.UUID, templateKey string) (bool, error) {
	var enabled bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT enabled
		FROM tenant_email_templates
		WHERE tenant_id = $1 AND template_key = $2
	`, tenantID, templateKey).Scan(&enabled)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// no row => default enabled
			return true, nil
		}
		return false, fmt.Errorf("get tenant email template enabled: %w", err)
	}
	return enabled, nil
}
