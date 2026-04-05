package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
)

// ReconciliationMismatch describes a single discrepancy found during reconciliation.
type ReconciliationMismatch struct {
	MembershipID       uuid.UUID `json:"membership_id"`
	ClientID           uuid.UUID `json:"client_id"`
	ClientName         string    `json:"client_name"`
	MembershipTitle    string    `json:"membership_title"`
	LocalStatus        string    `json:"local_status"`
	ProviderStatus     string    `json:"provider_status"`
	ProviderSubID      string    `json:"provider_subscription_id,omitempty"`
	Severity           string    `json:"severity"`
	Category           string    `json:"category"`
	RecommendedAction  string    `json:"recommended_action"`
	AutoFixed          bool      `json:"auto_fixed"`
}

// ReconciliationRun is the stored result of a reconciliation sweep.
type ReconciliationRun struct {
	ID              uuid.UUID                `json:"id"`
	TenantID        uuid.UUID                `json:"tenant_id"`
	RunType         string                   `json:"run_type"`
	StartedAt       time.Time                `json:"started_at"`
	CompletedAt     *time.Time               `json:"completed_at"`
	TotalChecked    int                      `json:"total_checked"`
	TotalMatched    int                      `json:"total_matched"`
	TotalMismatched int                      `json:"total_mismatched"`
	TotalAutoFixed  int                      `json:"total_auto_fixed"`
	Details         []ReconciliationMismatch `json:"details"`
	CreatedAt       time.Time                `json:"created_at"`
}

// ReconciliationRepo handles reconciliation run persistence.
type ReconciliationRepo struct {
	db *database.Postgres
}

func NewReconciliationRepo(db *database.Postgres) *ReconciliationRepo {
	return &ReconciliationRepo{db: db}
}

func (r *ReconciliationRepo) Create(ctx context.Context, tenantID uuid.UUID, runType string) (*ReconciliationRun, error) {
	var run ReconciliationRun
	var detailsRaw []byte
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO reconciliation_runs (tenant_id, run_type)
		VALUES ($1, $2)
		RETURNING id, tenant_id, run_type, started_at, completed_at,
		          total_checked, total_matched, total_mismatched, total_auto_fixed,
		          details, created_at
	`, tenantID, runType).Scan(
		&run.ID, &run.TenantID, &run.RunType, &run.StartedAt, &run.CompletedAt,
		&run.TotalChecked, &run.TotalMatched, &run.TotalMismatched, &run.TotalAutoFixed,
		&detailsRaw, &run.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create reconciliation run: %w", err)
	}
	if len(detailsRaw) > 0 {
		_ = json.Unmarshal(detailsRaw, &run.Details)
	}
	return &run, nil
}

func (r *ReconciliationRepo) Complete(ctx context.Context, id uuid.UUID, checked, matched, mismatched, autoFixed int, details []ReconciliationMismatch) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("marshal reconciliation details: %w", err)
	}
	_, err = r.db.Pool.Exec(ctx, `
		UPDATE reconciliation_runs
		SET completed_at = NOW(),
		    total_checked = $2,
		    total_matched = $3,
		    total_mismatched = $4,
		    total_auto_fixed = $5,
		    details = $6
		WHERE id = $1
	`, id, checked, matched, mismatched, autoFixed, detailsJSON)
	if err != nil {
		return fmt.Errorf("complete reconciliation run: %w", err)
	}
	return nil
}

func (r *ReconciliationRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID, limit int) ([]ReconciliationRun, error) {
	if limit <= 0 {
		limit = 30
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, tenant_id, run_type, started_at, completed_at,
		       total_checked, total_matched, total_mismatched, total_auto_fixed,
		       details, created_at
		FROM reconciliation_runs
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("list reconciliation runs: %w", err)
	}
	defer rows.Close()
	var items []ReconciliationRun
	for rows.Next() {
		var run ReconciliationRun
		var detailsRaw []byte
		if err := rows.Scan(
			&run.ID, &run.TenantID, &run.RunType, &run.StartedAt, &run.CompletedAt,
			&run.TotalChecked, &run.TotalMatched, &run.TotalMismatched, &run.TotalAutoFixed,
			&detailsRaw, &run.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan reconciliation run: %w", err)
		}
		if len(detailsRaw) > 0 {
			_ = json.Unmarshal(detailsRaw, &run.Details)
		}
		items = append(items, run)
	}
	return items, rows.Err()
}

// LatestByTenant returns the most recent run for a tenant, or nil.
func (r *ReconciliationRepo) LatestByTenant(ctx context.Context, tenantID uuid.UUID) (*ReconciliationRun, error) {
	runs, err := r.ListByTenant(ctx, tenantID, 1)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, nil
	}
	return &runs[0], nil
}
