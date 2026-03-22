package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"clientroll/internal/database"
)

const (
	alertRetryQueueDepthThreshold    = 50
	alertFailedDeliveries15mThresh   = 10
	alertStuckRetriesThreshold       = 1
	alertServiceLedgerMismatchThresh = 5
	alertMembershipStatusDriftThresh = 5
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
	activateScheduledMemberships(ctx, db)
	reconcileMembershipStatusDrift(ctx, db)

	var retryQueueDepth int
	var failedDeliveries15m int
	var stuckRetries int
	var serviceMismatch24h int
	var membershipStatusDrift int

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

	_ = db.Pool.QueryRow(ctx, `
		WITH latest_payment AS (
			SELECT DISTINCT ON (p.tenant_id, p.membership_id)
				p.tenant_id, p.membership_id, p.status
			FROM payments p
			ORDER BY p.tenant_id, p.membership_id, COALESCE(p.paid_date, p.due_date, p.created_at) DESC, p.created_at DESC
		),
		targets AS (
			SELECT
				m.status AS current_status,
				CASE
					WHEN m.start_date > ((NOW() AT TIME ZONE COALESCE(NULLIF(t.timezone, ''), 'UTC'))::date) THEN 'scheduled'
					WHEN lp.status IN ('failed', 'disputed') THEN 'past_due'
					WHEN lp.status = 'succeeded' THEN 'active'
					ELSE m.status
				END AS target_status
			FROM memberships m
			JOIN tenants t ON t.id = m.tenant_id
			LEFT JOIN latest_payment lp
				ON lp.tenant_id = m.tenant_id
				AND lp.membership_id = m.id
			WHERE m.status NOT IN ('cancelled', 'expired')
		)
		SELECT COUNT(*)
		FROM targets
		WHERE current_status <> target_status
	`).Scan(&membershipStatusDrift)

	if retryQueueDepth >= alertRetryQueueDepthThreshold {
		emitAlert("retry_queue_depth_high", map[string]interface{}{
			"depth":     retryQueueDepth,
			"threshold": alertRetryQueueDepthThreshold,
		})
	}
	if failedDeliveries15m >= alertFailedDeliveries15mThresh {
		emitAlert("failed_webhook_deliveries_spike", map[string]interface{}{
			"count":     failedDeliveries15m,
			"threshold": alertFailedDeliveries15mThresh,
			"window":    "15m",
		})
	}
	if stuckRetries >= alertStuckRetriesThreshold {
		emitAlert("stuck_webhook_retries", map[string]interface{}{
			"count": stuckRetries,
		})
	}
	if serviceMismatch24h >= alertServiceLedgerMismatchThresh {
		emitAlert("service_payment_ledger_mismatch", map[string]interface{}{
			"count":     serviceMismatch24h,
			"threshold": alertServiceLedgerMismatchThresh,
			"window":    "24h",
		})
	}
	if membershipStatusDrift >= alertMembershipStatusDriftThresh {
		emitAlert("membership_status_drift", map[string]interface{}{
			"count":     membershipStatusDrift,
			"threshold": alertMembershipStatusDriftThresh,
		})
	}
}

func activateScheduledMemberships(ctx context.Context, db *database.Postgres) {
	tag, err := db.Pool.Exec(ctx, `
		UPDATE memberships m
		SET status = 'active',
		    updated_at = NOW()
		FROM tenants t
		WHERE m.tenant_id = t.id
		  AND m.status = 'scheduled'
		  AND m.start_date <= ((NOW() AT TIME ZONE COALESCE(NULLIF(t.timezone, ''), 'UTC'))::date)
	`)
	if err != nil {
		log.Printf("alerts.monitor activate_scheduled_memberships_failed err=%v", err)
		return
	}
	if tag.RowsAffected() > 0 {
		log.Printf("alerts.monitor activated_scheduled_memberships count=%d", tag.RowsAffected())
	}
}

func reconcileMembershipStatusDrift(ctx context.Context, db *database.Postgres) {
	tag, err := db.Pool.Exec(ctx, `
		WITH latest_payment AS (
			SELECT DISTINCT ON (p.tenant_id, p.membership_id)
				p.tenant_id, p.membership_id, p.status
			FROM payments p
			ORDER BY p.tenant_id, p.membership_id, COALESCE(p.paid_date, p.due_date, p.created_at) DESC, p.created_at DESC
		),
		targets AS (
			SELECT
				m.id,
				m.tenant_id,
				CASE
					WHEN m.start_date > ((NOW() AT TIME ZONE COALESCE(NULLIF(t.timezone, ''), 'UTC'))::date) THEN 'scheduled'
					WHEN lp.status IN ('failed', 'disputed') THEN 'past_due'
					WHEN lp.status = 'succeeded' THEN 'active'
					ELSE m.status
				END AS target_status
			FROM memberships m
			JOIN tenants t ON t.id = m.tenant_id
			LEFT JOIN latest_payment lp
				ON lp.tenant_id = m.tenant_id
				AND lp.membership_id = m.id
			WHERE m.status NOT IN ('cancelled', 'expired')
		)
		UPDATE memberships m
		SET status = t.target_status,
		    updated_at = NOW()
		FROM targets t
		WHERE m.id = t.id
		  AND m.tenant_id = t.tenant_id
		  AND m.status <> t.target_status
	`)
	if err != nil {
		log.Printf("alerts.monitor reconcile_membership_status_drift_failed err=%v", err)
		return
	}
	if tag.RowsAffected() > 0 {
		log.Printf("alerts.monitor reconciled_membership_status_drift count=%d", tag.RowsAffected())
	}
}

func emitAlert(alertType string, payload map[string]interface{}) {
	body := map[string]interface{}{
		"alert_type": alertType,
		"source":     "alerts.monitor",
		"at":         time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range payload {
		body[k] = v
	}
	b, err := json.Marshal(body)
	if err != nil {
		log.Printf("ALERT %s payload_marshal_failed err=%v", alertType, err)
		return
	}
	log.Printf("ALERT %s", string(b))
	postAlertWebhook(b)
}

func postAlertWebhook(payload []byte) {
	webhookURL := os.Getenv("ALERTS_WEBHOOK_URL")
	if webhookURL == "" {
		return
	}
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		log.Printf("alerts.monitor webhook request build failed err=%v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("alerts.monitor webhook delivery failed err=%v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("alerts.monitor webhook non-2xx status=%d", resp.StatusCode)
	}
}
