package billing

import (
	"context"
	"fmt"
	"log"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
)

// UsageTracker handles daily active member count snapshots for billing.
type UsageTracker struct {
	db *database.Postgres
}

// NewUsageTracker creates a new usage tracker.
func NewUsageTracker(db *database.Postgres) *UsageTracker {
	return &UsageTracker{db: db}
}

// SnapshotAllTenants takes a daily snapshot of active member counts for all tenants.
// This should be called by a daily cron job.
func (u *UsageTracker) SnapshotAllTenants(ctx context.Context, pricePerMember float64) error {
	rows, err := u.db.Pool.Query(ctx, `
		SELECT t.id, COUNT(m.id) AS active_count
		FROM tenants t
		LEFT JOIN memberships m ON m.tenant_id = t.id AND m.status = 'active'
		WHERE t.is_active = true
		GROUP BY t.id`)
	if err != nil {
		return fmt.Errorf("querying tenant member counts: %w", err)
	}
	defer rows.Close()

	today := time.Now().UTC().Truncate(24 * time.Hour)
	count := 0

	for rows.Next() {
		var tenantID uuid.UUID
		var activeCount int
		if err := rows.Scan(&tenantID, &activeCount); err != nil {
			log.Printf("Usage snapshot: scan error: %v", err)
			continue
		}

		amountUSD := float64(activeCount) * pricePerMember

		_, err := u.db.Pool.Exec(ctx, `
			INSERT INTO tenant_usage_snapshots (tenant_id, active_member_count, amount_usd, snapshot_date)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (tenant_id, snapshot_date) DO UPDATE SET
				active_member_count = $2, amount_usd = $3`,
			tenantID, activeCount, amountUSD, today)
		if err != nil {
			log.Printf("Usage snapshot: insert error for tenant %s: %v", tenantID, err)
			continue
		}

		// Update the subscription record
		u.db.Pool.Exec(ctx, `
			UPDATE tenant_subscriptions SET active_member_count = $2,
				current_monthly_amount_usd = $3, updated_at = NOW()
			WHERE tenant_id = $1`,
			tenantID, activeCount, amountUSD)

		count++
	}

	log.Printf("Usage snapshots completed: %d tenants processed", count)
	return rows.Err()
}

// GetTenantUsageHistory returns usage snapshots for a tenant.
func (u *UsageTracker) GetTenantUsageHistory(ctx context.Context, tenantID uuid.UUID, days int) ([]UsageSnapshot, error) {
	if days <= 0 {
		days = 30
	}
	query := `
		SELECT tenant_id, active_member_count, amount_usd, snapshot_date, invoiced, created_at
		FROM tenant_usage_snapshots
		WHERE tenant_id = $1 AND snapshot_date >= NOW() - ($2 || ' days')::INTERVAL
		ORDER BY snapshot_date DESC`

	rows, err := u.db.Pool.Query(ctx, query, tenantID, fmt.Sprintf("%d", days))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []UsageSnapshot
	for rows.Next() {
		var s UsageSnapshot
		if err := rows.Scan(&s.TenantID, &s.ActiveMemberCount, &s.AmountUSD,
			&s.SnapshotDate, &s.Invoiced, &s.CreatedAt); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, s)
	}
	return snapshots, rows.Err()
}

// UsageSnapshot represents a daily usage record.
type UsageSnapshot struct {
	TenantID          uuid.UUID `json:"tenant_id"`
	ActiveMemberCount int       `json:"active_member_count"`
	AmountUSD         float64   `json:"amount_usd"`
	SnapshotDate      time.Time `json:"snapshot_date"`
	Invoiced          bool      `json:"invoiced"`
	CreatedAt         time.Time `json:"created_at"`
}
