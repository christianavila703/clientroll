package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// InboundWebhookEventRepo stores processed inbound webhook events for idempotency.
type InboundWebhookEventRepo struct {
	db *database.Postgres
}

// InboundWebhookEvent is an internal operational record for inbound webhook processing.
type InboundWebhookEvent struct {
	ID          uuid.UUID  `json:"id"`
	TenantID    uuid.UUID  `json:"tenant_id"`
	Provider    string     `json:"provider"`
	EventID     string     `json:"event_id"`
	Resource    string     `json:"resource_type"`
	ResourceID  string     `json:"resource_id"`
	Status      string     `json:"status"`
	RetryCount  int        `json:"retry_count"`
	LastError   *string    `json:"last_error,omitempty"`
	ReceivedAt  time.Time  `json:"received_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
	LastErrorAt *time.Time `json:"last_error_at,omitempty"`
}

// NewInboundWebhookEventRepo creates a new inbound webhook event repository.
func NewInboundWebhookEventRepo(db *database.Postgres) *InboundWebhookEventRepo {
	return &InboundWebhookEventRepo{db: db}
}

// RecordReceived inserts the inbound event. Returns false when the event was already seen.
func (r *InboundWebhookEventRepo) RecordReceived(ctx context.Context, tenantID uuid.UUID, provider, eventID, resourceType, resourceID string) (bool, error) {
	query := `
		INSERT INTO inbound_webhook_events (
			tenant_id, provider, event_id, resource_type, resource_id
		) VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Pool.Exec(ctx, query, tenantID, provider, eventID, resourceType, resourceID)
	if err == nil {
		return true, nil
	}

	var pgErr *pgconn.PgError
	if ok := errors.As(err, &pgErr); ok && pgErr.Code == "23505" {
		return false, nil
	}
	return false, fmt.Errorf("record inbound webhook event: %w", err)
}

// MarkProcessed sets processed_at for an event after successful processing.
func (r *InboundWebhookEventRepo) MarkProcessed(ctx context.Context, tenantID uuid.UUID, provider, eventID string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE inbound_webhook_events
		SET processed_at = NOW(),
		    status = 'processed',
		    last_error = NULL,
		    last_error_at = NULL
		WHERE tenant_id = $1 AND provider = $2 AND event_id = $3
	`, tenantID, provider, eventID)
	if err != nil {
		return fmt.Errorf("mark inbound webhook processed: %w", err)
	}
	return nil
}

// MarkFailed records a processing failure for an already-recorded inbound webhook event.
func (r *InboundWebhookEventRepo) MarkFailed(ctx context.Context, tenantID uuid.UUID, provider, eventID, errMsg string) error {
	if errMsg == "" {
		errMsg = "webhook processing failed"
	}
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE inbound_webhook_events
		SET status = 'failed',
		    retry_count = retry_count + 1,
		    last_error = $4,
		    last_error_at = NOW()
		WHERE tenant_id = $1 AND provider = $2 AND event_id = $3
	`, tenantID, provider, eventID, errMsg)
	if err != nil {
		return fmt.Errorf("mark inbound webhook failed: %w", err)
	}
	return nil
}

// ListFailed returns recent failed inbound webhook events for a provider.
func (r *InboundWebhookEventRepo) ListFailed(ctx context.Context, provider string, limit int) ([]InboundWebhookEvent, error) {
	return r.ListFailedFiltered(ctx, provider, nil, limit)
}

// ListFailedFiltered returns recent failed inbound webhook events for a provider, optionally scoped to a tenant.
func (r *InboundWebhookEventRepo) ListFailedFiltered(ctx context.Context, provider string, tenantID *uuid.UUID, limit int) ([]InboundWebhookEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, tenant_id, provider, event_id, resource_type, resource_id, status,
		       retry_count, last_error, received_at, processed_at, last_error_at
		FROM inbound_webhook_events
		WHERE provider = $1 AND status = 'failed'
		  AND ($2::uuid IS NULL OR tenant_id = $2::uuid)
		ORDER BY last_error_at DESC NULLS LAST, received_at DESC
		LIMIT $3
	`, provider, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("list failed inbound webhook events: %w", err)
	}
	defer rows.Close()

	out := make([]InboundWebhookEvent, 0, limit)
	for rows.Next() {
		var item InboundWebhookEvent
		if scanErr := rows.Scan(
			&item.ID, &item.TenantID, &item.Provider, &item.EventID, &item.Resource, &item.ResourceID,
			&item.Status, &item.RetryCount, &item.LastError, &item.ReceivedAt, &item.ProcessedAt, &item.LastErrorAt,
		); scanErr != nil {
			return nil, fmt.Errorf("scan failed inbound webhook event: %w", scanErr)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
