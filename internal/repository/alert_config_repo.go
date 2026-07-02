package repository

import (
	"context"
	"fmt"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AlertConfig holds per-tenant payment monitoring alert preferences.
type AlertConfig struct {
	ID                       uuid.UUID `json:"id"`
	TenantID                 uuid.UUID `json:"tenant_id"`
	AlertEmail               string    `json:"alert_email"`
	ImmediatePaymentFailed   bool      `json:"immediate_payment_failed"`
	ImmediateDispute         bool      `json:"immediate_dispute"`
	ImmediateMandateCancelled bool     `json:"immediate_mandate_cancelled"`
	ImmediatePipelineBreak   bool      `json:"immediate_pipeline_break"`
	DailyDigestEnabled       bool      `json:"daily_digest_enabled"`
	DailyDigestHour          int       `json:"daily_digest_hour"`
	DailyDigestAlways        bool      `json:"daily_digest_always"`
	GraceDaysCard            int       `json:"grace_days_card"`
	GraceDaysDirectDebit     int       `json:"grace_days_direct_debit"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// AlertConfigRepo handles database operations for tenant alert configuration.
type AlertConfigRepo struct {
	db *database.Postgres
}

func NewAlertConfigRepo(db *database.Postgres) *AlertConfigRepo {
	return &AlertConfigRepo{db: db}
}

func (r *AlertConfigRepo) GetByTenant(ctx context.Context, tenantID uuid.UUID) (*AlertConfig, error) {
	var c AlertConfig
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, tenant_id, alert_email,
		       immediate_payment_failed, immediate_dispute, immediate_mandate_cancelled, immediate_pipeline_break,
		       daily_digest_enabled, daily_digest_hour, daily_digest_always,
		       grace_days_card, grace_days_direct_debit,
		       created_at, updated_at
		FROM tenant_alert_config
		WHERE tenant_id = $1
	`, tenantID).Scan(
		&c.ID, &c.TenantID, &c.AlertEmail,
		&c.ImmediatePaymentFailed, &c.ImmediateDispute, &c.ImmediateMandateCancelled, &c.ImmediatePipelineBreak,
		&c.DailyDigestEnabled, &c.DailyDigestHour, &c.DailyDigestAlways,
		&c.GraceDaysCard, &c.GraceDaysDirectDebit,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get alert config: %w", err)
	}
	return &c, nil
}

func (r *AlertConfigRepo) Upsert(ctx context.Context, tenantID uuid.UUID, input AlertConfig) (*AlertConfig, error) {
	var c AlertConfig
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO tenant_alert_config (
			tenant_id, alert_email,
			immediate_payment_failed, immediate_dispute, immediate_mandate_cancelled, immediate_pipeline_break,
			daily_digest_enabled, daily_digest_hour, daily_digest_always,
			grace_days_card, grace_days_direct_debit
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (tenant_id) DO UPDATE SET
			alert_email = EXCLUDED.alert_email,
			immediate_payment_failed = EXCLUDED.immediate_payment_failed,
			immediate_dispute = EXCLUDED.immediate_dispute,
			immediate_mandate_cancelled = EXCLUDED.immediate_mandate_cancelled,
			immediate_pipeline_break = EXCLUDED.immediate_pipeline_break,
			daily_digest_enabled = EXCLUDED.daily_digest_enabled,
			daily_digest_hour = EXCLUDED.daily_digest_hour,
			daily_digest_always = EXCLUDED.daily_digest_always,
			grace_days_card = EXCLUDED.grace_days_card,
			grace_days_direct_debit = EXCLUDED.grace_days_direct_debit,
			updated_at = NOW()
		RETURNING id, tenant_id, alert_email,
		          immediate_payment_failed, immediate_dispute, immediate_mandate_cancelled, immediate_pipeline_break,
		          daily_digest_enabled, daily_digest_hour, daily_digest_always,
		          grace_days_card, grace_days_direct_debit,
		          created_at, updated_at
	`, tenantID, input.AlertEmail,
		input.ImmediatePaymentFailed, input.ImmediateDispute, input.ImmediateMandateCancelled, input.ImmediatePipelineBreak,
		input.DailyDigestEnabled, input.DailyDigestHour, input.DailyDigestAlways,
		input.GraceDaysCard, input.GraceDaysDirectDebit,
	).Scan(
		&c.ID, &c.TenantID, &c.AlertEmail,
		&c.ImmediatePaymentFailed, &c.ImmediateDispute, &c.ImmediateMandateCancelled, &c.ImmediatePipelineBreak,
		&c.DailyDigestEnabled, &c.DailyDigestHour, &c.DailyDigestAlways,
		&c.GraceDaysCard, &c.GraceDaysDirectDebit,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert alert config: %w", err)
	}
	return &c, nil
}

// ListAllWithDigest returns all tenant alert configs that have daily digest enabled.
func (r *AlertConfigRepo) ListAllWithDigest(ctx context.Context) ([]AlertConfig, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, tenant_id, alert_email,
		       immediate_payment_failed, immediate_dispute, immediate_mandate_cancelled, immediate_pipeline_break,
		       daily_digest_enabled, daily_digest_hour, daily_digest_always,
		       grace_days_card, grace_days_direct_debit,
		       created_at, updated_at
		FROM tenant_alert_config
		WHERE daily_digest_enabled = TRUE
	`)
	if err != nil {
		return nil, fmt.Errorf("list digest configs: %w", err)
	}
	defer rows.Close()
	var items []AlertConfig
	for rows.Next() {
		var c AlertConfig
		if err := rows.Scan(
			&c.ID, &c.TenantID, &c.AlertEmail,
			&c.ImmediatePaymentFailed, &c.ImmediateDispute, &c.ImmediateMandateCancelled, &c.ImmediatePipelineBreak,
			&c.DailyDigestEnabled, &c.DailyDigestHour, &c.DailyDigestAlways,
			&c.GraceDaysCard, &c.GraceDaysDirectDebit,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan alert config: %w", err)
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

// ListAll returns every tenant alert config row.
func (r *AlertConfigRepo) ListAll(ctx context.Context) ([]AlertConfig, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, tenant_id, alert_email,
		       immediate_payment_failed, immediate_dispute, immediate_mandate_cancelled, immediate_pipeline_break,
		       daily_digest_enabled, daily_digest_hour, daily_digest_always,
		       grace_days_card, grace_days_direct_debit,
		       created_at, updated_at
		FROM tenant_alert_config
	`)
	if err != nil {
		return nil, fmt.Errorf("list all alert configs: %w", err)
	}
	defer rows.Close()
	var items []AlertConfig
	for rows.Next() {
		var c AlertConfig
		if err := rows.Scan(
			&c.ID, &c.TenantID, &c.AlertEmail,
			&c.ImmediatePaymentFailed, &c.ImmediateDispute, &c.ImmediateMandateCancelled, &c.ImmediatePipelineBreak,
			&c.DailyDigestEnabled, &c.DailyDigestHour, &c.DailyDigestAlways,
			&c.GraceDaysCard, &c.GraceDaysDirectDebit,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan alert config: %w", err)
		}
		items = append(items, c)
	}
	return items, rows.Err()
}
