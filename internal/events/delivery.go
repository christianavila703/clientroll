package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
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
	log.Println("webhook.worker started")

	for {
		select {
		case <-ctx.Done():
			log.Println("webhook.worker stopped")
			return
		default:
			w.processNext(ctx)
			w.requeueDueRetries(ctx)
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
		log.Printf("webhook.worker invalid_job err=%v", err)
		return
	}

	w.deliver(ctx, job)
}

func (w *Worker) deliver(ctx context.Context, job DeliveryJob) {
	// Decrypt the webhook secret
	secret, err := w.enc.Decrypt(job.SecretEncrypted)
	if err != nil {
		log.Printf("webhook.worker decrypt_secret_failed endpoint=%s tenant=%s err=%v", job.EndpointID, job.TenantID, err)
		w.recordFailure(ctx, job, 0, "decryption_error", "Failed to decrypt signing secret")
		return
	}

	// Compute HMAC signature
	ts := job.Timestamp
	if ts == "" {
		ts = time.Now().UTC().Format(time.RFC3339)
	}
	signedPayload := append([]byte(ts+"."), []byte(job.Payload)...)
	signature := SignPayload(signedPayload, secret)

	// Make the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", job.URL, bytes.NewReader([]byte(job.Payload)))
	if err != nil {
		w.recordFailure(ctx, job, 0, "", fmt.Sprintf("Invalid URL: %v", err))
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Event", job.EventType)
	req.Header.Set("X-Webhook-Event-Id", job.EventID)
	req.Header.Set("X-Webhook-Timestamp", ts)
	req.Header.Set("User-Agent", "ClientRoll-Webhook/1.0")

	resp, err := w.client.Do(req)
	if err != nil {
		w.handleFailure(ctx, job, 0, "", fmt.Sprintf("Request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		w.recordSuccess(ctx, job, resp.StatusCode, string(body))
	} else {
		w.handleFailure(ctx, job, resp.StatusCode, string(body),
			fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
}

func (w *Worker) recordSuccess(ctx context.Context, job DeliveryJob, httpStatus int, responseBody string) {
	if job.DeliveryKind == "signup" {
		_, err := w.db.Pool.Exec(ctx, `
			UPDATE signup_webhook_deliveries SET status='delivered', http_status=$1,
			       response_body=$2, attempt_count=attempt_count+1
			WHERE id=$3 AND tenant_id=$4`,
			httpStatus, responseBody, job.DeliveryID, job.TenantID)
		if err != nil {
			log.Printf("signup webhook record_success_failed delivery=%s tenant=%s err=%v", job.DeliveryID, job.TenantID, err)
		}
		return
	}
	_, err := w.db.Pool.Exec(ctx, `
		UPDATE webhook_deliveries SET status='delivered', http_status=$1,
		       response_body=$2, attempt_count=attempt_count+1
		WHERE webhook_endpoint_id=$3 AND tenant_id=$4 AND event_type=$5
		  AND status IN ('pending', 'retrying')
		ORDER BY created_at DESC LIMIT 1`,
		httpStatus, responseBody, job.EndpointID, job.TenantID, job.EventType)
	if err != nil {
		log.Printf("webhook.worker record_success_failed endpoint=%s tenant=%s event=%s err=%v", job.EndpointID, job.TenantID, job.EventType, err)
	}
}

func (w *Worker) handleFailure(ctx context.Context, job DeliveryJob, httpStatus int, responseBody, reason string) {
	job.AttemptCount++

	if job.AttemptCount >= MaxRetries {
		w.recordFailure(ctx, job, httpStatus, responseBody, reason)
		return
	}

	delay := NextRetryDelay(job.AttemptCount)
	nextRetry := time.Now().Add(delay)

	if job.DeliveryKind == "signup" {
		w.db.Pool.Exec(ctx, `
			UPDATE signup_webhook_deliveries SET status='retrying', http_status=$1,
			       response_body=$2, attempt_count=$3, next_retry_at=$4
			WHERE id=$5 AND tenant_id=$6`,
			httpStatus, responseBody, job.AttemptCount, nextRetry, job.DeliveryID, job.TenantID)
	} else {
		w.db.Pool.Exec(ctx, `
		UPDATE webhook_deliveries SET status='retrying', http_status=$1,
		       response_body=$2, attempt_count=$3, next_retry_at=$4
		WHERE webhook_endpoint_id=$5 AND tenant_id=$6 AND event_type=$7
		  AND status IN ('pending', 'retrying')
		ORDER BY created_at DESC LIMIT 1`,
			httpStatus, responseBody, job.AttemptCount, nextRetry,
			job.EndpointID, job.TenantID, job.EventType)
	}

	// Schedule retry via Redis delayed queue
	jobBytes, _ := json.Marshal(job)
	w.redis.Client.ZAdd(ctx, w.redis.Key("webhook:retry"), redis.Z{
		Score:  float64(nextRetry.Unix()),
		Member: string(jobBytes),
	})
}

func (w *Worker) recordFailure(ctx context.Context, job DeliveryJob, httpStatus int, responseBody, reason string) {
	if job.DeliveryKind == "signup" {
		w.db.Pool.Exec(ctx, `
			UPDATE signup_webhook_deliveries SET status='failed', http_status=$1,
			       response_body=$2, attempt_count=$3
			WHERE id=$4 AND tenant_id=$5`,
			httpStatus, responseBody, job.AttemptCount, job.DeliveryID, job.TenantID)
		log.Printf("signup webhook delivery_failed delivery=%s tenant=%s attempt_count=%d reason=%s",
			job.DeliveryID, job.TenantID, job.AttemptCount, reason)
		return
	}
	w.db.Pool.Exec(ctx, `
		UPDATE webhook_deliveries SET status='failed', http_status=$1,
		       response_body=$2, attempt_count=$3
		WHERE webhook_endpoint_id=$4 AND tenant_id=$5 AND event_type=$6
		  AND status IN ('pending', 'retrying')
		ORDER BY created_at DESC LIMIT 1`,
		httpStatus, responseBody, job.AttemptCount,
		job.EndpointID, job.TenantID, job.EventType)

	log.Printf("webhook.worker delivery_failed_permanently endpoint=%s tenant=%s event=%s attempt_count=%d reason=%s",
		job.EndpointID, job.TenantID, job.EventType, job.AttemptCount, reason)
}

func (w *Worker) requeueDueRetries(ctx context.Context) {
	now := strconv.FormatInt(time.Now().Unix(), 10)
	due, err := w.redis.Client.ZRangeByScore(ctx, w.redis.Key("webhook:retry"), &redis.ZRangeBy{
		Min:    "-inf",
		Max:    now,
		Offset: 0,
		Count:  50,
	}).Result()
	if err != nil || len(due) == 0 {
		return
	}

	for _, item := range due {
		removed, err := w.redis.Client.ZRem(ctx, w.redis.Key("webhook:retry"), item).Result()
		if err != nil || removed == 0 {
			continue
		}
		if err := w.redis.Client.LPush(ctx, w.redis.Key("webhook:queue"), item).Err(); err != nil {
			// Best effort: put it back in retry set to avoid loss.
			_, _ = w.redis.Client.ZAdd(ctx, w.redis.Key("webhook:retry"), redis.Z{
				Score:  float64(time.Now().Unix() + 5),
				Member: item,
			}).Result()
		}
	}
}
