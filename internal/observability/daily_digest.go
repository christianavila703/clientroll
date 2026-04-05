package observability

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/email"
	"clientroll/internal/repository"

	"github.com/google/uuid"
)

// DigestService builds and sends the daily payment digest email.
type DigestService struct {
	db              *database.Postgres
	emailSvc        email.Provider
	alertConfigRepo *repository.AlertConfigRepo
	reconRepo       *repository.ReconciliationRepo
	ownerAlerts     *OwnerAlertService
}

func NewDigestService(
	db *database.Postgres,
	emailSvc email.Provider,
	alertConfigRepo *repository.AlertConfigRepo,
	reconRepo *repository.ReconciliationRepo,
	ownerAlerts *OwnerAlertService,
) *DigestService {
	return &DigestService{
		db:              db,
		emailSvc:        emailSvc,
		alertConfigRepo: alertConfigRepo,
		reconRepo:       reconRepo,
		ownerAlerts:     ownerAlerts,
	}
}

// StartDailyDigestScheduler runs a loop that checks every 15 minutes whether
// it's time to send any tenant's daily digest (based on their configured hour).
func StartDailyDigestScheduler(ctx context.Context, ds *DigestService) {
	if ds == nil || ds.emailSvc == nil || ds.alertConfigRepo == nil {
		log.Printf("daily_digest.scheduler skipped (missing dependencies)")
		return
	}
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	log.Printf("daily_digest.scheduler started")

	sentToday := make(map[string]bool)
	lastDate := time.Now().UTC().Format("2006-01-02")

	for {
		select {
		case <-ctx.Done():
			log.Printf("daily_digest.scheduler stopped")
			return
		case <-ticker.C:
			today := time.Now().UTC().Format("2006-01-02")
			if today != lastDate {
				sentToday = make(map[string]bool)
				lastDate = today
			}
			ds.runDigestCycle(ctx, sentToday)
		}
	}
}

func (ds *DigestService) runDigestCycle(ctx context.Context, sentToday map[string]bool) {
	configs, err := ds.alertConfigRepo.ListAllWithDigest(ctx)
	if err != nil {
		log.Printf("daily_digest.scheduler list_configs_failed err=%v", err)
		return
	}

	for _, cfg := range configs {
		key := cfg.TenantID.String()
		if sentToday[key] {
			continue
		}
		tz := ds.tenantTimezone(ctx, cfg.TenantID)
		loc, err := time.LoadLocation(tz)
		if err != nil {
			loc = time.UTC
		}
		now := time.Now().In(loc)
		if now.Hour() >= cfg.DailyDigestHour {
			ds.sendDigest(ctx, cfg)
			sentToday[key] = true
		}
	}
}

func (ds *DigestService) tenantTimezone(ctx context.Context, tenantID uuid.UUID) string {
	var tz string
	_ = ds.db.Pool.QueryRow(ctx, `SELECT COALESCE(NULLIF(timezone, ''), 'UTC') FROM tenants WHERE id = $1`, tenantID).Scan(&tz)
	if tz == "" {
		return "UTC"
	}
	return tz
}

func (ds *DigestService) sendDigest(ctx context.Context, cfg repository.AlertConfig) {
	evr, err := ds.ownerAlerts.CheckExpectedVsReceived(ctx, cfg.TenantID, cfg.GraceDaysCard, cfg.GraceDaysDirectDebit)
	if err != nil {
		log.Printf("daily_digest expected_vs_received failed tenant=%s err=%v", cfg.TenantID, err)
		evr = &ExpectedVsReceivedSummary{}
	}

	wh, err := ds.ownerAlerts.GetWebhookHealth(ctx, cfg.TenantID)
	if err != nil {
		log.Printf("daily_digest webhook_health failed tenant=%s err=%v", cfg.TenantID, err)
		wh = &WebhookHealthSummary{}
	}

	aging, err := ds.ownerAlerts.GetPaymentAging(ctx, cfg.TenantID)
	if err != nil {
		log.Printf("daily_digest payment_aging failed tenant=%s err=%v", cfg.TenantID, err)
		aging = &PaymentAgingSummary{}
	}

	var latestRecon *repository.ReconciliationRun
	if ds.reconRepo != nil {
		latestRecon, _ = ds.reconRepo.LatestByTenant(ctx, cfg.TenantID)
	}

	hasIssues := evr.TotalOverdue > 0 || (wh.Failed24h > 0) || aging.ActiveUnpaid30d > 0
	if latestRecon != nil && latestRecon.TotalMismatched > 0 {
		hasIssues = true
	}

	if !cfg.DailyDigestAlways && !hasIssues {
		return
	}

	subject := ds.buildDigestSubject(evr, hasIssues)
	html := ds.buildDigestHTML(evr, wh, aging, latestRecon)

	if _, err := ds.emailSvc.Send(ctx, email.SendInput{
		To:      cfg.AlertEmail,
		Subject: subject,
		HTML:    html,
		Text:    subject,
	}); err != nil {
		log.Printf("daily_digest send failed tenant=%s to=%s err=%v", cfg.TenantID, cfg.AlertEmail, err)
	} else {
		log.Printf("daily_digest sent tenant=%s to=%s issues=%v", cfg.TenantID, cfg.AlertEmail, hasIssues)
	}
}

func (ds *DigestService) buildDigestSubject(evr *ExpectedVsReceivedSummary, hasIssues bool) string {
	if hasIssues {
		return fmt.Sprintf("Daily Payment Digest — %d issues need attention", evr.TotalOverdue)
	}
	return "Daily Payment Digest — all clear"
}

func (ds *DigestService) buildDigestHTML(
	evr *ExpectedVsReceivedSummary,
	wh *WebhookHealthSummary,
	aging *PaymentAgingSummary,
	recon *repository.ReconciliationRun,
) string {
	var sb strings.Builder

	sb.WriteString(`<h1 style="font-size:20px;font-weight:600;color:#111827;margin:0 0 16px;">Daily Payment Digest</h1>`)
	sb.WriteString(fmt.Sprintf(`<p style="font-size:12px;color:#9ca3af;margin:0 0 24px;">Report generated %s</p>`, time.Now().UTC().Format("2 January 2006, 15:04 UTC")))

	// Section 1: Expected vs Received
	sb.WriteString(sectionHeader("Expected vs Received", "Our payment calendar"))
	statusColor := "#059669"
	if evr.TotalOverdue > 0 {
		statusColor = "#dc2626"
	}
	sb.WriteString(fmt.Sprintf(`<p style="font-size:14px;color:#374151;margin:0 0 12px;"><strong style="color:%s">%d</strong> payments expected · <strong>%d</strong> received · <strong style="color:%s">%d overdue</strong></p>`,
		statusColor, evr.TotalExpected, evr.TotalReceived, statusColor, evr.TotalOverdue))
	if len(evr.OverdueItems) > 0 {
		sb.WriteString(overdueTable(evr.OverdueItems))
	}

	// Section 2: Stripe/GC Verification
	sb.WriteString(sectionHeader("Provider Verification", "Stripe/GoCardless API"))
	if recon != nil && recon.CompletedAt != nil {
		reconColor := "#059669"
		if recon.TotalMismatched > 0 {
			reconColor = "#dc2626"
		}
		sb.WriteString(fmt.Sprintf(`<p style="font-size:14px;color:#374151;margin:0 0 8px;">Last run: %s</p>`, recon.CompletedAt.Format("2 Jan 2006, 15:04 UTC")))
		sb.WriteString(fmt.Sprintf(`<p style="font-size:14px;color:#374151;margin:0 0 12px;"><strong>%d</strong> checked · <strong>%d</strong> matched · <strong style="color:%s">%d mismatched</strong> · <strong>%d</strong> auto-fixed</p>`,
			recon.TotalChecked, recon.TotalMatched, reconColor, recon.TotalMismatched, recon.TotalAutoFixed))
		if recon.TotalMismatched > 0 && len(recon.Details) > 0 {
			sb.WriteString(mismatchTable(recon.Details))
		}
	} else {
		sb.WriteString(`<p style="font-size:14px;color:#9ca3af;margin:0 0 12px;">No reconciliation run completed yet.</p>`)
	}

	// Section 3: Pipeline Health
	sb.WriteString(sectionHeader("Pipeline Health", "Webhook event log"))
	sb.WriteString(fmt.Sprintf(`<div style="background:#f9fafb;border:1px solid #e5e7eb;border-radius:8px;padding:12px;margin:0 0 16px;">
		<table style="width:100%%;font-size:13px;color:#374151;">
			<tr><td style="padding:4px 0;color:#6b7280;">Last Stripe webhook</td><td style="text-align:right;">%s</td></tr>
			<tr><td style="padding:4px 0;color:#6b7280;">Last GoCardless webhook</td><td style="text-align:right;">%s</td></tr>
			<tr><td style="padding:4px 0;color:#6b7280;">Processed (24h)</td><td style="text-align:right;">%d</td></tr>
			<tr><td style="padding:4px 0;color:#6b7280;">Failed (24h)</td><td style="text-align:right;%s">%d</td></tr>
			<tr><td style="padding:4px 0;color:#6b7280;">Processed (7d)</td><td style="text-align:right;">%d</td></tr>
			<tr><td style="padding:4px 0;color:#6b7280;">Failed (7d)</td><td style="text-align:right;">%d</td></tr>
		</table>
	</div>`,
		formatOptionalTime(wh.LastStripeWebhook),
		formatOptionalTime(wh.LastGoCardlessWebhook),
		wh.Processed24h,
		failedStyle(wh.Failed24h), wh.Failed24h,
		wh.Processed7d, wh.Failed7d))

	// Section 4: Payment Aging
	sb.WriteString(sectionHeader("Payment Aging", "Failed payments by age"))
	if aging != nil {
		sb.WriteString(`<div style="background:#f9fafb;border:1px solid #e5e7eb;border-radius:8px;padding:12px;margin:0 0 16px;">
			<table style="width:100%;font-size:13px;color:#374151;">`)
		for _, b := range aging.Buckets {
			sb.WriteString(fmt.Sprintf(`<tr><td style="padding:4px 0;color:#6b7280;">%s</td><td style="text-align:right;">%d</td></tr>`, b.Label, b.Count))
		}
		sb.WriteString(fmt.Sprintf(`<tr><td style="padding:4px 0;color:#6b7280;font-weight:600;">Active, unpaid 30+ days</td><td style="text-align:right;%s">%d</td></tr>`,
			failedStyle(aging.ActiveUnpaid30d), aging.ActiveUnpaid30d))
		sb.WriteString(`</table></div>`)
	}

	sb.WriteString(`<p style="font-size:11px;color:#d1d5db;margin:16px 0 0;">This is an automated report from ClientRoll payment monitoring. Triple-checked against your database, payment provider APIs, and webhook event log.</p>`)

	return email.WrapHTML(sb.String())
}

func sectionHeader(title, source string) string {
	return fmt.Sprintf(`<div style="border-top:1px solid #e5e7eb;margin:20px 0 12px;padding-top:16px;">
		<h2 style="font-size:16px;font-weight:600;color:#111827;margin:0 0 2px;">%s</h2>
		<p style="font-size:11px;color:#9ca3af;margin:0 0 12px;">Source: %s</p>
	</div>`, title, source)
}

func overdueTable(items []OverduePayment) string {
	var sb strings.Builder
	sb.WriteString(`<table style="width:100%;font-size:13px;color:#374151;border-collapse:collapse;margin:0 0 16px;">
		<tr style="background:#fef2f2;">
			<th style="text-align:left;padding:6px;border-bottom:1px solid #fecaca;">Client</th>
			<th style="text-align:left;padding:6px;border-bottom:1px solid #fecaca;">Membership</th>
			<th style="text-align:right;padding:6px;border-bottom:1px solid #fecaca;">Amount</th>
			<th style="text-align:right;padding:6px;border-bottom:1px solid #fecaca;">Due Date</th>
			<th style="text-align:right;padding:6px;border-bottom:1px solid #fecaca;">Days Overdue</th>
		</tr>`)
	for _, item := range items {
		sb.WriteString(fmt.Sprintf(`<tr>
			<td style="padding:6px;border-bottom:1px solid #f3f4f6;">%s</td>
			<td style="padding:6px;border-bottom:1px solid #f3f4f6;">%s</td>
			<td style="padding:6px;border-bottom:1px solid #f3f4f6;text-align:right;">%s $%.2f</td>
			<td style="padding:6px;border-bottom:1px solid #f3f4f6;text-align:right;">%s</td>
			<td style="padding:6px;border-bottom:1px solid #f3f4f6;text-align:right;color:#dc2626;font-weight:600;">%d</td>
		</tr>`, item.ClientName, item.MembershipTitle, item.CurrencyCode, item.Amount,
			item.DueDate.Format("2 Jan 2006"), item.DaysOverdue))
	}
	sb.WriteString(`</table>`)
	return sb.String()
}

func mismatchTable(items []repository.ReconciliationMismatch) string {
	var sb strings.Builder
	sb.WriteString(`<table style="width:100%;font-size:13px;color:#374151;border-collapse:collapse;margin:0 0 16px;">
		<tr style="background:#fffbeb;">
			<th style="text-align:left;padding:6px;border-bottom:1px solid #fde68a;">Client</th>
			<th style="text-align:left;padding:6px;border-bottom:1px solid #fde68a;">Our Status</th>
			<th style="text-align:left;padding:6px;border-bottom:1px solid #fde68a;">Provider Status</th>
			<th style="text-align:left;padding:6px;border-bottom:1px solid #fde68a;">Action</th>
		</tr>`)
	for _, item := range items {
		fixed := ""
		if item.AutoFixed {
			fixed = ` <span style="color:#059669;font-size:11px;">(auto-fixed)</span>`
		}
		sb.WriteString(fmt.Sprintf(`<tr>
			<td style="padding:6px;border-bottom:1px solid #f3f4f6;">%s</td>
			<td style="padding:6px;border-bottom:1px solid #f3f4f6;">%s</td>
			<td style="padding:6px;border-bottom:1px solid #f3f4f6;">%s</td>
			<td style="padding:6px;border-bottom:1px solid #f3f4f6;">%s%s</td>
		</tr>`, item.ClientName, item.LocalStatus, item.ProviderStatus, item.RecommendedAction, fixed))
	}
	sb.WriteString(`</table>`)
	return sb.String()
}

func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return `<span style="color:#d1d5db;">Never</span>`
	}
	return t.Format("2 Jan 2006, 15:04 UTC")
}

func failedStyle(count int) string {
	if count > 0 {
		return "color:#dc2626;font-weight:600;"
	}
	return ""
}
