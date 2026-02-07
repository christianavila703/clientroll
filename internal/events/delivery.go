package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"clientroll/internal/database"

	"github.com/redis/go-redis/v9"
)

// Worker processes queued webhook deliveries.
type Worker struct {
	db     *database.Postgres
	redis  *database.Redis
	enc    *database.Encryptor
	client *http.Client
}

// NewWorker creates a new delivery worker.
func NewWorker(db *database.Postgres, redis *database.Redis, enc *database.Encryptor) *Worker {
	return &Worker{
		db:    db,
		redis: redis,
		enc:   enc,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Start begins processing the webhook delivery queue.
func (w *Worker) Start(ctx context.Context) {
	log.Println("Webhook delivery worker started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Webhook delivery worker stopped")
			return
		default:
			w.processNext(ctx)
		}
	}
}

func (w *Worker) processNext(ctx context.Context) {
	// Blocking pop from queue (5 second timeout)
	result, err := w.redis.Client.BRPop(ctx, 5*time.Second, w.redis.Key("webhook:queue")).Result()
	if err != nil {
		return // Timeout or shutdown
	}

	if len(result) < 2 {
		return
	}

	var job DeliveryJob
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		log.Printf("Webhook worker: invalid job: %v", err)
		return
	}

	w.deliver(ctx, job)
}

func (w *Worker) deliver(ctx context.Context, job DeliveryJob) {
	// Decrypt the webhook secret
	secret, err := w.enc.Decrypt(job.SecretEncrypted)
	if err != nil {
		log.Printf("Webhook worker: failed to decrypt secret for endpoint %s: %v", job.EndpointID, err)
		w.recordFailure(ctx, job, 0, "decryption_error", "Failed to decrypt signing secret")
		return
	}

	// Compute HMAC signature
	signature := SignPayload([]byte(job.Payload), secret)

	// Make the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", job.URL, bytes.NewReader([]byte(job.Payload)))
	if err != nil {
		w.recordFailure(ctx, job, 0, "", fmt.Sprintf("Invalid URL: %v", err))
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Event", job.EventType)
	req.Header.Set("User-Agent", "ClientRoll-Webhook/1.0")

	resp, err := w.client.Do(req)
	if err != nil {
		w.handleFailure(ctx, job, 0, "", fmt.Sprintf("Request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Success
		w.recordSuccess(ctx, job, resp.StatusCode, string(body))
	} else {
		// HTTP error — may retry
		w.handleFailure(ctx, job, resp.StatusCode, string(body),
			fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
}

func (w *Worker) recordSuccess(ctx context.Context, job DeliveryJob, httpStatus int, responseBody string) {
	_, err := w.db.Pool.Exec(ctx, `
		UPDATE webhook_deliveries SET status='delivered', http_status=$1,
		       response_body=$2, attempt_count=attempt_count+1
		WHERE webhook_endpoint_id=$3 AND tenant_id=$4 AND event_type=$5
		  AND status IN ('pending', 'retrying')
		ORDER BY created_at DESC LIMIT 1`,
		httpStatus, responseBody, job.EndpointID, job.TenantID, job.EventType)
	if err != nil {
		log.Printf("Webhook worker: error recording success: %v", err)
	}
}

func (w *Worker) handleFailure(ctx context.Context, job DeliveryJob, httpStatus int, responseBody, reason string) {
	job.AttemptCount++

	if job.AttemptCount >= MaxRetries {
		w.recordFailure(ctx, job, httpStatus, responseBody, reason)
		return
	}

	// Re-enqueue with delay
	delay := NextRetryDelay(job.AttemptCount)
	nextRetry := time.Now().Add(delay)

	// Update delivery record
	w.db.Pool.Exec(ctx, `
		UPDATE webhook_deliveries SET status='retrying', http_status=$1,
		       response_body=$2, attempt_count=$3, next_retry_at=$4
		WHERE webhook_endpoint_id=$5 AND tenant_id=$6 AND event_type=$7
		  AND status IN ('pending', 'retrying')
		ORDER BY created_at DESC LIMIT 1`,
		httpStatus, responseBody, job.AttemptCount, nextRetry,
		job.EndpointID, job.TenantID, job.EventType)

	// Schedule retry via Redis delayed queue
	jobBytes, _ := json.Marshal(job)
	w.redis.Client.ZAdd(ctx, w.redis.Key("webhook:retry"), redis.Z{
		Score:  float64(nextRetry.Unix()),
		Member: string(jobBytes),
	})
}

func (w *Worker) recordFailure(ctx context.Context, job DeliveryJob, httpStatus int, responseBody, reason string) {
	w.db.Pool.Exec(ctx, `
		UPDATE webhook_deliveries SET status='failed', http_status=$1,
		       response_body=$2, attempt_count=$3
		WHERE webhook_endpoint_id=$4 AND tenant_id=$5 AND event_type=$6
		  AND status IN ('pending', 'retrying')
		ORDER BY created_at DESC LIMIT 1`,
		httpStatus, responseBody, job.AttemptCount,
		job.EndpointID, job.TenantID, job.EventType)

	log.Printf("Webhook delivery failed permanently: endpoint=%s event=%s reason=%s",
		job.EndpointID, job.EventType, reason)
}
