package repository

import (
	"context"
	"fmt"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
)

type TenantUsageSnapshot struct {
	ID               uuid.UUID `json:"id"`
	TenantID         uuid.UUID `json:"tenant_id"`
	ActiveMemberCount int      `json:"active_member_count"`
	AmountUSD        float64   `json:"amount_usd"`
	SnapshotDate     time.Time `json:"snapshot_date"`
	Invoiced         bool      `json:"invoiced"`
	CreatedAt        time.Time `json:"created_at"`
}

type TenantUsageRepo struct {
	db *database.Postgres
}

func NewTenantUsageRepo(db *database.Postgres) *TenantUsageRepo {
	return &TenantUsageRepo{db: db}
}

func (r *TenantUsageRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID, days int) ([]TenantUsageSnapshot, error) {
	if days <= 0 {
		days = 30
	}
	rlsID, tenantUUID := database.TenantRLSScope(tenantID)
	query := `
		WITH _ AS (
			SELECT set_config('app.current_tenant_id', $1, true)
		)
		SELECT id, tenant_id, active_member_count, amount_usd, snapshot_date, invoiced, created_at
		FROM tenant_usage_snapshots
		WHERE tenant_id = $2 AND snapshot_date >= NOW() - ($3 || ' days')::INTERVAL
		ORDER BY snapshot_date DESC`

	rows, err := r.db.Pool.Query(ctx, query, rlsID, tenantUUID, fmt.Sprintf("%d", days))
	if err != nil {
		return nil, fmt.Errorf("listing tenant usage: %w", err)
	}
	defer rows.Close()

	var out []TenantUsageSnapshot
	for rows.Next() {
		var s TenantUsageSnapshot
		if err := rows.Scan(&s.ID, &s.TenantID, &s.ActiveMemberCount, &s.AmountUSD, &s.SnapshotDate, &s.Invoiced, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *TenantUsageRepo) MarkInvoicedUpTo(ctx context.Context, tenantID uuid.UUID, throughDate time.Time) error {
	rlsID, tenantUUID := database.TenantRLSScope(tenantID)
	_, err := r.db.Pool.Exec(ctx, `
		WITH _ AS (
			SELECT set_config('app.current_tenant_id', $1, true)
		)
		UPDATE tenant_usage_snapshots
		SET invoiced = true
		WHERE tenant_id = $2 AND snapshot_date <= $3`, rlsID, tenantUUID, throughDate)
	if err != nil {
		return fmt.Errorf("marking usage invoiced: %w", err)
	}
	return nil
}

