package observability

import (
	"context"
	"log"
	"time"

	"clientroll/internal/database"
)

const (
	alertRetryQueueDepthThreshold    = 50
	alertFailedDeliveries15mThresh   = 10
	alertStuckRetriesThreshold       = 1
	alertServiceLedgerMismatchThresh = 5
)

// StartAlertsMonitor runs periodic operational alert checks and logs alert events.
// This is a baseline "always-on" monitor so ops can wire log-based alerts immediately.
func StartAlertsMonitor(ctx context.Context, db *database.Postgres, interval time.Duration) {
	if db == nil || interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("alerts.monitor started interval=%s", interval)
	for {
		select {
		case <-ctx.Done():
			log.Printf("alerts.monitor stopped")
			return
		case <-ticker.C:
			runAlertChecks(ctx, db)
		}
	}
}

func runAlertChecks(ctx context.Context, db *database.Postgres) {
	var retryQueueDepth int
	var failedDeliveries15m int
	var stuckRetries int
	var serviceMismatch24h int

	_ = db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM webhook_deliveries
		WHERE status = 'retrying'
	`).Scan(&retryQueueDepth)

	_ = db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM webhook_deliveries
		WHERE status = 'failed'
		  AND created_at >= NOW() - INTERVAL '15 minutes'
	`).Scan(&failedDeliveries15m)

	_ = db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM webhook_deliveries
		WHERE status = 'retrying'
		  AND next_retry_at IS NOT NULL
		  AND next_retry_at < NOW() - INTERVAL '10 minutes'
	`).Scan(&stuckRetries)

	_ = db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM service_charges sc
		WHERE sc.created_at >= NOW() - INTERVAL '24 hours'
		  AND NOT EXISTS (
		    SELECT 1 FROM payments p
		    WHERE p.tenant_id = sc.tenant_id
		      AND p.provider_reference = ('charge-' || sc.id::text)
		  )
	`).Scan(&serviceMismatch24h)

	if retryQueueDepth >= alertRetryQueueDepthThreshold {
		log.Printf("ALERT retry_queue_depth_high depth=%d threshold=%d", retryQueueDepth, alertRetryQueueDepthThreshold)
	}
	if failedDeliveries15m >= alertFailedDeliveries15mThresh {
		log.Printf("ALERT failed_webhook_deliveries_spike count=%d threshold=%d window=15m", failedDeliveries15m, alertFailedDeliveries15mThresh)
	}
	if stuckRetries >= alertStuckRetriesThreshold {
		log.Printf("ALERT stuck_webhook_retries count=%d", stuckRetries)
	}
	if serviceMismatch24h >= alertServiceLedgerMismatchThresh {
		log.Printf("ALERT service_payment_ledger_mismatch count=%d threshold=%d window=24h", serviceMismatch24h, alertServiceLedgerMismatchThresh)
	}
}
