package events

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
)

// Dispatcher handles publishing events to outbound webhook endpoints.
type Dispatcher struct {
	db    *database.Postgres
	redis *database.Redis
	enc   *database.Encryptor
}

// NewDispatcher creates a new event dispatcher.
func NewDispatcher(db *database.Postgres, redis *database.Redis, enc *database.Encryptor) *Dispatcher {
	return &Dispatcher{db: db, redis: redis, enc: enc}
}

// Dispatch publishes an event to all matching webhook endpoints for the tenant.
func (d *Dispatcher) Dispatch(ctx context.Context, event Event) {
	tenantID, err := uuid.Parse(event.TenantID)
	if err != nil {
		log.Printf("Event dispatch: invalid tenant ID: %s", event.TenantID)
		return
	}

	// Find active webhook endpoints that subscribe to this event type
	rows, err := d.db.Pool.Query(ctx, `
		SELECT id, url, secret_encrypted, event_types
		FROM webhook_endpoints
		WHERE tenant_id = $1 AND is_active = true
		  AND ($2 = ANY(event_types) OR array_length(event_types, 1) IS NULL)`,
		tenantID, event.Type)
	if err != nil {
		log.Printf("Event dispatch: error querying endpoints: %v", err)
		return
	}
	defer rows.Close()

	payload, _ := json.Marshal(event)

	for rows.Next() {
		var endpointID uuid.UUID
		var url, secretEncrypted string
		var eventTypes []string

		if err := rows.Scan(&endpointID, &url, &secretEncrypted, &eventTypes); err != nil {
			log.Printf("Event dispatch: scan error: %v", err)
			continue
		}

		// Create delivery record
		_, err := d.db.Pool.Exec(ctx, `
			INSERT INTO webhook_deliveries
				(tenant_id, webhook_endpoint_id, event_type, payload, status, attempt_count)
			VALUES ($1, $2, $3, $4, 'pending', 0)`,
			tenantID, endpointID, event.Type, payload)
		if err != nil {
			log.Printf("Event dispatch: error creating delivery: %v", err)
			continue
		}

		// Enqueue for async delivery via Redis
		job := DeliveryJob{
			EndpointID:      endpointID.String(),
			TenantID:        tenantID.String(),
			URL:             url,
			SecretEncrypted: secretEncrypted,
			Payload:         string(payload),
			EventType:       event.Type,
			EventID:         event.ID,
			Timestamp:       event.CreatedAt.UTC().Format(time.RFC3339),
			AttemptCount:    0,
		}
		jobBytes, _ := json.Marshal(job)
		d.redis.Client.LPush(ctx, d.redis.Key("webhook:queue"), string(jobBytes))
	}
}

	// DeliveryJob represents a queued webhook delivery.
type DeliveryJob struct {
	DeliveryKind    string `json:"delivery_kind,omitempty"` // "signup" or empty for endpoint webhooks
	DeliveryID      string `json:"delivery_id,omitempty"`
	EndpointID      string `json:"endpoint_id"`
	TenantID        string `json:"tenant_id"`
	URL             string `json:"url"`
	SecretEncrypted string `json:"secret_encrypted"`
	Payload         string `json:"payload"`
	EventType       string `json:"event_type"`
	EventID         string `json:"event_id"`
	Timestamp       string `json:"timestamp"`
	AttemptCount    int    `json:"attempt_count"`
}

// SignPayload computes the HMAC-SHA256 signature for a payload.
func SignPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// RetryDelays returns exponential backoff delays for webhook retries.
func RetryDelays() []time.Duration {
	return []time.Duration{
		1 * time.Minute,
		5 * time.Minute,
		30 * time.Minute,
		2 * time.Hour,
		24 * time.Hour,
	}
}

// MaxRetries is the maximum number of delivery attempts.
const MaxRetries = 5

// NextRetryDelay returns the delay for the next retry attempt.
func NextRetryDelay(attempt int) time.Duration {
	delays := RetryDelays()
	if attempt >= len(delays) {
		return delays[len(delays)-1]
	}
	return delays[attempt]
}

// GenerateWebhookSecret creates a random signing secret.
func GenerateWebhookSecret() string {
	id := uuid.New()
	return "whsec_" + hex.EncodeToString(id[:])
}
