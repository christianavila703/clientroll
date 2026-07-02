package repository

import (
	"context"
	"fmt"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// WebhookEndpoint represents a configured webhook endpoint.
type WebhookEndpoint struct {
	ID              uuid.UUID `json:"id"`
	TenantID        uuid.UUID `json:"tenant_id"`
	URL             string    `json:"url"`
	SecretEncrypted string    `json:"-"`
	EventTypes      []string  `json:"event_types"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
}

// WebhookDelivery represents a delivery attempt.
type WebhookDelivery struct {
	ID              uuid.UUID  `json:"id"`
	TenantID        uuid.UUID  `json:"tenant_id"`
	EndpointID      uuid.UUID  `json:"webhook_endpoint_id"`
	EventType       string     `json:"event_type"`
	Payload         string     `json:"payload"`
	HTTPStatus      *int       `json:"http_status"`
	ResponseBody    *string    `json:"response_body"`
	Status          string     `json:"status"`
	AttemptCount    int        `json:"attempt_count"`
	NextRetryAt     *time.Time `json:"next_retry_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

// WebhookRepo handles database operations for webhooks.
type WebhookRepo struct {
	db *database.Postgres
}

// NewWebhookRepo creates a new webhook repository.
func NewWebhookRepo(db *database.Postgres) *WebhookRepo {
	return &WebhookRepo{db: db}
}

// ListEndpoints returns all webhook endpoints for a tenant.
func (r *WebhookRepo) ListEndpoints(ctx context.Context, tenantID uuid.UUID) ([]WebhookEndpoint, error) {
	query := `SELECT id, tenant_id, url, secret_encrypted, event_types, is_active, created_at
		FROM webhook_endpoints WHERE tenant_id = $1 ORDER BY created_at DESC`

	rows, err := r.db.Pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listing webhook endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []WebhookEndpoint
	for rows.Next() {
		var e WebhookEndpoint
		if err := rows.Scan(&e.ID, &e.TenantID, &e.URL, &e.SecretEncrypted,
			&e.EventTypes, &e.IsActive, &e.CreatedAt); err != nil {
			return nil, err
		}
		endpoints = append(endpoints, e)
	}
	return endpoints, rows.Err()
}

// CreateEndpoint creates a new webhook endpoint.
func (r *WebhookRepo) CreateEndpoint(ctx context.Context, tenantID uuid.UUID, url, secretEncrypted string, eventTypes []string) (*WebhookEndpoint, error) {
	query := `INSERT INTO webhook_endpoints (tenant_id, url, secret_encrypted, event_types)
		VALUES ($1, $2, $3, $4)
		RETURNING id, tenant_id, url, secret_encrypted, event_types, is_active, created_at`

	var e WebhookEndpoint
	err := r.db.Pool.QueryRow(ctx, query, tenantID, url, secretEncrypted, eventTypes).Scan(
		&e.ID, &e.TenantID, &e.URL, &e.SecretEncrypted, &e.EventTypes, &e.IsActive, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating webhook endpoint: %w", err)
	}
	return &e, nil
}

// DeleteEndpoint removes a webhook endpoint.
func (r *WebhookRepo) DeleteEndpoint(ctx context.Context, id, tenantID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx,
		"DELETE FROM webhook_endpoints WHERE id=$1 AND tenant_id=$2", id, tenantID)
	return err
}

// ToggleEndpoint enables or disables a webhook endpoint.
func (r *WebhookRepo) ToggleEndpoint(ctx context.Context, id, tenantID uuid.UUID, isActive bool) error {
	_, err := r.db.Pool.Exec(ctx,
		"UPDATE webhook_endpoints SET is_active=$3, updated_at=NOW() WHERE id=$1 AND tenant_id=$2",
		id, tenantID, isActive)
	return err
}

// ListDeliveries returns recent deliveries for an endpoint.
func (r *WebhookRepo) ListDeliveries(ctx context.Context, endpointID, tenantID uuid.UUID, limit int) ([]WebhookDelivery, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT id, tenant_id, webhook_endpoint_id, event_type, payload::text,
		http_status, response_body, status, attempt_count, next_retry_at, created_at
		FROM webhook_deliveries WHERE webhook_endpoint_id=$1 AND tenant_id=$2
		ORDER BY created_at DESC LIMIT $3`

	rows, err := r.db.Pool.Query(ctx, query, endpointID, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deliveries []WebhookDelivery
	for rows.Next() {
		var d WebhookDelivery
		if err := rows.Scan(&d.ID, &d.TenantID, &d.EndpointID, &d.EventType, &d.Payload,
			&d.HTTPStatus, &d.ResponseBody, &d.Status, &d.AttemptCount,
			&d.NextRetryAt, &d.CreatedAt); err != nil {
			return nil, err
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}

// FindEndpointByID returns a webhook endpoint by ID.
func (r *WebhookRepo) FindEndpointByID(ctx context.Context, id, tenantID uuid.UUID) (*WebhookEndpoint, error) {
	query := `SELECT id, tenant_id, url, secret_encrypted, event_types, is_active, created_at
		FROM webhook_endpoints WHERE id=$1 AND tenant_id=$2`

	var e WebhookEndpoint
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID).Scan(
		&e.ID, &e.TenantID, &e.URL, &e.SecretEncrypted, &e.EventTypes, &e.IsActive, &e.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}
