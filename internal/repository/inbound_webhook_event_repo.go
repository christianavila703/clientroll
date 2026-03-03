package repository

import (
	"context"
	"errors"
	"fmt"

	"clientroll/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// InboundWebhookEventRepo stores processed inbound webhook events for idempotency.
type InboundWebhookEventRepo struct {
	db *database.Postgres
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
		SET processed_at = NOW()
		WHERE tenant_id = $1 AND provider = $2 AND event_id = $3
	`, tenantID, provider, eventID)
	if err != nil {
		return fmt.Errorf("mark inbound webhook processed: %w", err)
	}
	return nil
}
