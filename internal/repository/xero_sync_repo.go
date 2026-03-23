package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
)

const (
	XeroEntityContact           = "contact"
	XeroEntityMembershipPayment = "membership_payment"
	XeroEntityServiceCharge     = "service_charge"
	XeroEntityCreditNote        = "credit_note"
)

type XeroSyncMapping struct {
	ID             uuid.UUID  `json:"id"`
	TenantID       uuid.UUID  `json:"tenant_id"`
	EntityType     string     `json:"entity_type"`
	EntityID       uuid.UUID  `json:"entity_id"`
	XeroObjectID   string     `json:"xero_object_id"`
	XeroObjectRef  *string    `json:"xero_object_ref,omitempty"`
	SyncState      string     `json:"sync_state"`
	PaidDateAtSync *time.Time `json:"paid_date_at_sync,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	LastSyncedAt   time.Time  `json:"last_synced_at"`
}

type XeroSyncEvent struct {
	ID         uuid.UUID `json:"id"`
	TenantID   uuid.UUID `json:"tenant_id"`
	EntityType string    `json:"entity_type"`
	EntityID   uuid.UUID `json:"entity_id"`
	Action     string    `json:"action"`
	Status     string    `json:"status"`
	ErrorCode  *string   `json:"error_code,omitempty"`
	ErrorBody  *string   `json:"error_body,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type XeroSyncRepo struct {
	db *database.Postgres
}

func NewXeroSyncRepo(db *database.Postgres) *XeroSyncRepo {
	return &XeroSyncRepo{db: db}
}

func (r *XeroSyncRepo) FindMapping(ctx context.Context, tenantID uuid.UUID, entityType string, entityID uuid.UUID) (*XeroSyncMapping, error) {
	query := `SELECT id, tenant_id, entity_type, entity_id, xero_object_id, xero_object_ref, sync_state, paid_date_at_sync, created_at, last_synced_at
	          FROM xero_sync_mappings
	          WHERE tenant_id=$1 AND entity_type=$2 AND entity_id=$3`
	var m XeroSyncMapping
	err := r.db.Pool.QueryRow(ctx, query, tenantID, entityType, entityID).Scan(
		&m.ID, &m.TenantID, &m.EntityType, &m.EntityID, &m.XeroObjectID, &m.XeroObjectRef,
		&m.SyncState, &m.PaidDateAtSync, &m.CreatedAt, &m.LastSyncedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

type UpsertXeroSyncMappingInput struct {
	EntityType     string
	EntityID       uuid.UUID
	XeroObjectID   string
	XeroObjectRef  *string
	SyncState      string
	PaidDateAtSync *time.Time
}

func (r *XeroSyncRepo) UpsertMapping(ctx context.Context, tenantID uuid.UUID, in UpsertXeroSyncMappingInput) (*XeroSyncMapping, error) {
	query := `
		INSERT INTO xero_sync_mappings (
			tenant_id, entity_type, entity_id, xero_object_id, xero_object_ref, sync_state, paid_date_at_sync
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (tenant_id, entity_type, entity_id) DO UPDATE SET
			xero_object_id = EXCLUDED.xero_object_id,
			xero_object_ref = EXCLUDED.xero_object_ref,
			sync_state = EXCLUDED.sync_state,
			paid_date_at_sync = COALESCE(EXCLUDED.paid_date_at_sync, xero_sync_mappings.paid_date_at_sync),
			last_synced_at = NOW()
		RETURNING id, tenant_id, entity_type, entity_id, xero_object_id, xero_object_ref, sync_state, paid_date_at_sync, created_at, last_synced_at`

	var m XeroSyncMapping
	err := r.db.Pool.QueryRow(ctx, query,
		tenantID, in.EntityType, in.EntityID, in.XeroObjectID, in.XeroObjectRef, in.SyncState, in.PaidDateAtSync,
	).Scan(
		&m.ID, &m.TenantID, &m.EntityType, &m.EntityID, &m.XeroObjectID, &m.XeroObjectRef, &m.SyncState, &m.PaidDateAtSync, &m.CreatedAt, &m.LastSyncedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert xero sync mapping: %w", err)
	}
	return &m, nil
}

func (r *XeroSyncRepo) AppendEvent(ctx context.Context, tenantID uuid.UUID, entityType string, entityID uuid.UUID, action, status string, errorCode, errorBody *string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO xero_sync_events (tenant_id, entity_type, entity_id, action, status, error_code, error_body)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, tenantID, entityType, entityID, action, status, errorCode, errorBody)
	return err
}

func (r *XeroSyncRepo) ListFailedEventsByTenant(ctx context.Context, tenantID uuid.UUID, limit int) ([]XeroSyncEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, tenant_id, entity_type, entity_id, action, status, error_code, error_body, created_at
		FROM xero_sync_events
		WHERE tenant_id = $1 AND status = 'failed'
		ORDER BY created_at DESC
		LIMIT $2
	`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []XeroSyncEvent
	for rows.Next() {
		var ev XeroSyncEvent
		if err := rows.Scan(
			&ev.ID, &ev.TenantID, &ev.EntityType, &ev.EntityID, &ev.Action, &ev.Status, &ev.ErrorCode, &ev.ErrorBody, &ev.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (r *XeroSyncRepo) ListFailedEvents(ctx context.Context, limit int) ([]XeroSyncEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, tenant_id, entity_type, entity_id, action, status, error_code, error_body, created_at
		FROM xero_sync_events
		WHERE status = 'failed'
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []XeroSyncEvent
	for rows.Next() {
		var ev XeroSyncEvent
		if err := rows.Scan(
			&ev.ID, &ev.TenantID, &ev.EntityType, &ev.EntityID, &ev.Action, &ev.Status, &ev.ErrorCode, &ev.ErrorBody, &ev.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (r *XeroSyncRepo) GetEventByID(ctx context.Context, id uuid.UUID) (*XeroSyncEvent, error) {
	query := `SELECT id, tenant_id, entity_type, entity_id, action, status, error_code, error_body, created_at
	          FROM xero_sync_events
	          WHERE id = $1`
	var ev XeroSyncEvent
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&ev.ID, &ev.TenantID, &ev.EntityType, &ev.EntityID, &ev.Action, &ev.Status, &ev.ErrorCode, &ev.ErrorBody, &ev.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ev, nil
}

func (r *XeroSyncRepo) CountEventsByStatus(ctx context.Context, tenantID *uuid.UUID, since time.Time) (map[string]int, error) {
	query := `
		SELECT status, COUNT(*)
		FROM xero_sync_events
		WHERE created_at >= $1`
	args := []interface{}{since}
	if tenantID != nil && *tenantID != uuid.Nil {
		query += ` AND tenant_id = $2`
		args = append(args, *tenantID)
	}
	query += ` GROUP BY status`

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]int{"success": 0, "failed": 0}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		out[strings.ToLower(strings.TrimSpace(status))] = count
	}
	return out, rows.Err()
}
