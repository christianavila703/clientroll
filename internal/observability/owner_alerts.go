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

// OwnerAlertService sends immediate and digest alert emails to tenant owners.
type OwnerAlertService struct {
	db                *database.Postgres
	emailSvc          email.Provider
	alertConfigRepo   *repository.AlertConfigRepo
	emailTemplateRepo *repository.TenantEmailTemplateRepo
}

func NewOwnerAlertService(db *database.Postgres, emailSvc email.Provider, alertConfigRepo *repository.AlertConfigRepo) *OwnerAlertService {
	return &OwnerAlertService{db: db, emailSvc: emailSvc, alertConfigRepo: alertConfigRepo}
}

// SetEmailTemplateRepo injects the template repo for customisable alert emails.
func (s *OwnerAlertService) SetEmailTemplateRepo(r *repository.TenantEmailTemplateRepo) {
	s.emailTemplateRepo = r
}

// AlertType classifies an immediate alert.
type AlertType string

const (
	AlertPaymentFailed      AlertType = "payment_failed"
	AlertDispute            AlertType = "dispute"
	AlertMandateCancelled   AlertType = "mandate_cancelled"
	AlertPipelineBreak      AlertType = "pipeline_break"
	AlertWebhookDropped     AlertType = "webhook_dropped"
	AlertWebhookStuck       AlertType = "webhook_stuck"
)

// PaymentAlertData carries context for a payment-related immediate alert.
type PaymentAlertData struct {
	ClientName      string
	MembershipTitle string
	Amount          float64
	CurrencyCode    string
	FailureReason   string
	Provider        string // "stripe" or "gocardless"
	DisputeReason   string
}

// SendRawEmail sends a pre-built email to the tenant's alert email address.
func (s *OwnerAlertService) SendRawEmail(ctx context.Context, tenantID uuid.UUID, subject, html string) {
	if s.emailSvc == nil || s.alertConfigRepo == nil {
		return
	}
	cfg, err := s.alertConfigRepo.GetByTenant(ctx, tenantID)
	if err != nil || cfg == nil || cfg.AlertEmail == "" {
		return
	}
	go func() {
		sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := s.emailSvc.Send(sendCtx, email.SendInput{
			To:      cfg.AlertEmail,
			Subject: subject,
			HTML:    html,
			Text:    subject,
		}); err != nil {
			log.Printf("owner_alert raw send failed tenant=%s to=%s err=%v", tenantID, cfg.AlertEmail, err)
		} else {
			log.Printf("owner_alert raw sent tenant=%s to=%s subject=%q", tenantID, cfg.AlertEmail, subject)
		}
	}()
}

// SendImmediate sends a real-time alert email for a critical payment event.
func (s *OwnerAlertService) SendImmediate(ctx context.Context, tenantID uuid.UUID, alertType AlertType, data PaymentAlertData) {
	if s.emailSvc == nil || s.alertConfigRepo == nil {
		return
	}
	cfg, err := s.alertConfigRepo.GetByTenant(ctx, tenantID)
	if err != nil || cfg == nil || cfg.AlertEmail == "" {
		return
	}
	if !s.isAlertEnabled(cfg, alertType) {
		return
	}

	subject, html := s.buildImmediateEmail(ctx, tenantID, alertType, data)
	go func() {
		sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := s.emailSvc.Send(sendCtx, email.SendInput{
			To:      cfg.AlertEmail,
			Subject: subject,
			HTML:    html,
			Text:    subject,
		}); err != nil {
			log.Printf("owner_alert send failed tenant=%s type=%s to=%s err=%v", tenantID, alertType, cfg.AlertEmail, err)
		} else {
			log.Printf("owner_alert sent tenant=%s type=%s to=%s", tenantID, alertType, cfg.AlertEmail)
		}
	}()
}

func (s *OwnerAlertService) isAlertEnabled(cfg *repository.AlertConfig, alertType AlertType) bool {
	switch alertType {
	case AlertPaymentFailed:
		return cfg.ImmediatePaymentFailed
	case AlertDispute:
		return cfg.ImmediateDispute
	case AlertMandateCancelled:
		return cfg.ImmediateMandateCancelled
	case AlertPipelineBreak, AlertWebhookDropped, AlertWebhookStuck:
		return cfg.ImmediatePipelineBreak
	default:
		return true
	}
}

func (s *OwnerAlertService) buildImmediateEmail(ctx context.Context, tenantID uuid.UUID, alertType AlertType, data PaymentAlertData) (string, string) {
	money := func(v float64) string { return fmt.Sprintf("%.2f", v) }
	cur := strings.ToUpper(strings.TrimSpace(data.CurrencyCode))
	if cur == "" {
		cur = "AUD"
	}

	switch alertType {
	case AlertPaymentFailed:
		reason := data.FailureReason
		if reason == "" {
			reason = "No details provided"
		}
		vars := map[string]string{
			"admin_name": "", "client_name": data.ClientName, "amount": money(data.Amount),
			"total_charged": money(data.Amount), "amount_due": money(data.Amount),
			"base_amount": money(data.Amount), "tax_amount": "0.00",
			"processing_fee_amount": "0.00", "to_organisation_amount": money(data.Amount),
			"currency_code": cur, "membership_title": data.MembershipTitle, "failure_reason": reason,
		}
		if subj, html, ok := s.tryTemplate(ctx, tenantID, "payment_failed_alert", vars); ok {
			return subj, html
		}
		subject := fmt.Sprintf("Payment failed: %s — %s $%s", data.ClientName, cur, money(data.Amount))
		html := email.WrapHTML(fmt.Sprintf(`
		<h1 style="font-size:20px;font-weight:600;color:#dc2626;margin:0 0 8px;">Payment Failed</h1>
		<p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 16px;">A payment has failed and needs your attention.</p>
		<div style="background:#fef2f2;border:1px solid #fecaca;border-radius:8px;padding:16px;margin:0 0 24px;">
			<table style="width:100%%;font-size:14px;color:#374151;">
				<tr><td style="padding:6px 0;color:#6b7280;">Client</td><td style="padding:6px 0;font-weight:600;text-align:right;">%s</td></tr>
				<tr><td style="padding:6px 0;color:#6b7280;">Amount</td><td style="padding:6px 0;text-align:right;">%s $%s</td></tr>
				<tr><td style="padding:6px 0;color:#6b7280;">Membership</td><td style="padding:6px 0;text-align:right;">%s</td></tr>
				<tr><td style="padding:6px 0;color:#6b7280;">Reason</td><td style="padding:6px 0;text-align:right;color:#dc2626;">%s</td></tr>
				<tr><td style="padding:6px 0;color:#6b7280;">Provider</td><td style="padding:6px 0;text-align:right;">%s</td></tr>
			</table>
		</div>
		<p style="font-size:13px;color:#9ca3af;">Review this payment in your ClientRoll dashboard under Payments.</p>`,
			data.ClientName, cur, money(data.Amount), data.MembershipTitle, reason, data.Provider))
		return subject, html

	case AlertDispute:
		reason := data.DisputeReason
		if reason == "" {
			reason = "Not specified"
		}
		vars := map[string]string{
			"admin_name": "", "client_name": data.ClientName,
			"amount": money(data.Amount), "reason": reason,
		}
		if subj, html, ok := s.tryTemplate(ctx, tenantID, "dispute_alert", vars); ok {
			return subj, html
		}
		subject := fmt.Sprintf("Dispute opened: %s — %s $%s", data.ClientName, cur, money(data.Amount))
		html := email.WrapHTML(fmt.Sprintf(`
		<h1 style="font-size:20px;font-weight:600;color:#d97706;margin:0 0 8px;">Payment Dispute Opened</h1>
		<p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 16px;">A client has disputed a payment. This requires immediate review.</p>
		<div style="background:#fffbeb;border:1px solid #fde68a;border-radius:8px;padding:16px;margin:0 0 24px;">
			<table style="width:100%%;font-size:14px;color:#374151;">
				<tr><td style="padding:6px 0;color:#6b7280;">Client</td><td style="padding:6px 0;font-weight:600;text-align:right;">%s</td></tr>
				<tr><td style="padding:6px 0;color:#6b7280;">Amount</td><td style="padding:6px 0;text-align:right;">%s $%s</td></tr>
				<tr><td style="padding:6px 0;color:#6b7280;">Reason</td><td style="padding:6px 0;text-align:right;">%s</td></tr>
				<tr><td style="padding:6px 0;color:#6b7280;">Provider</td><td style="padding:6px 0;text-align:right;">%s</td></tr>
			</table>
		</div>
		<p style="font-size:13px;color:#9ca3af;">Review and respond in your ClientRoll dashboard.</p>`,
			data.ClientName, cur, money(data.Amount), reason, data.Provider))
		return subject, html

	case AlertMandateCancelled:
		subject := fmt.Sprintf("Mandate/subscription cancelled: %s", data.ClientName)
		html := email.WrapHTML(fmt.Sprintf(`
		<h1 style="font-size:20px;font-weight:600;color:#d97706;margin:0 0 8px;">Payment Mandate Cancelled</h1>
		<p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 16px;">A client's payment mandate or subscription has been cancelled at the provider level. Future payments for this membership will not be collected automatically.</p>
		<div style="background:#fffbeb;border:1px solid #fde68a;border-radius:8px;padding:16px;margin:0 0 24px;">
			<table style="width:100%%;font-size:14px;color:#374151;">
				<tr><td style="padding:6px 0;color:#6b7280;">Client</td><td style="padding:6px 0;font-weight:600;text-align:right;">%s</td></tr>
				<tr><td style="padding:6px 0;color:#6b7280;">Membership</td><td style="padding:6px 0;text-align:right;">%s</td></tr>
				<tr><td style="padding:6px 0;color:#6b7280;">Provider</td><td style="padding:6px 0;text-align:right;">%s</td></tr>
			</table>
		</div>
		<p style="font-size:13px;color:#9ca3af;">No further payments will be collected for this membership unless a new mandate is set up.</p>`,
			data.ClientName, data.MembershipTitle, data.Provider))
		return subject, html

	case AlertPipelineBreak:
		subject := "Payment pipeline alert — expected payments not arriving"
		html := email.WrapHTML(fmt.Sprintf(`
		<h1 style="font-size:20px;font-weight:600;color:#dc2626;margin:0 0 8px;">Payment Pipeline Alert</h1>
		<p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 16px;">
			Multiple expected payments have not arrived. This may indicate a problem with your Stripe or GoCardless integration.
		</p>
		<div style="background:#fef2f2;border:1px solid #fecaca;border-radius:8px;padding:16px;margin:0 0 24px;">
			<p style="font-size:14px;color:#374151;margin:0;">%s</p>
		</div>
		<p style="font-size:13px;color:#9ca3af;">Check your payment provider settings in ClientRoll and verify your Stripe/GoCardless dashboards.</p>`,
			data.FailureReason))
		return subject, html

	case AlertWebhookDropped:
		subject := "Stripe webhook dropped — tenant misconfigured"
		html := email.WrapHTML(fmt.Sprintf(`
		<h1 style="font-size:20px;font-weight:600;color:#dc2626;margin:0 0 8px;">Webhook Configuration Alert</h1>
		<p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 16px;">
			A Stripe webhook was received but could not be processed because the tenant's Stripe integration is inactive or the webhook secret is missing. The event was dropped.
		</p>
		<div style="background:#fef2f2;border:1px solid #fecaca;border-radius:8px;padding:16px;margin:0 0 24px;">
			<p style="font-size:14px;color:#374151;margin:0;">%s</p>
		</div>
		<p style="font-size:13px;color:#9ca3af;">Payment status updates from Stripe will not be processed until the integration is correctly configured.</p>`,
			data.FailureReason))
		return subject, html

	case AlertWebhookStuck:
		subject := "Webhook events stuck — not processing"
		html := email.WrapHTML(fmt.Sprintf(`
		<h1 style="font-size:20px;font-weight:600;color:#dc2626;margin:0 0 8px;">Stuck Webhook Events Detected</h1>
		<p style="font-size:14px;color:#6b7280;line-height:1.6;margin:0 0 16px;">
			Webhook events have been received but stuck in "received" status for over 10 minutes without being processed. Payment status updates may be delayed.
		</p>
		<div style="background:#fef2f2;border:1px solid #fecaca;border-radius:8px;padding:16px;margin:0 0 24px;">
			<p style="font-size:14px;color:#374151;margin:0;">%s</p>
		</div>
		<p style="font-size:13px;color:#9ca3af;">Check server logs for webhook processing errors. These events may need manual reprocessing.</p>`,
			data.FailureReason))
		return subject, html

	default:
		subject := fmt.Sprintf("ClientRoll Alert: %s", alertType)
		html := email.WrapHTML(fmt.Sprintf(`
		<h1 style="font-size:20px;font-weight:600;color:#111827;margin:0 0 8px;">Alert: %s</h1>
		<p style="font-size:14px;color:#6b7280;line-height:1.6;">%s</p>`, alertType, data.FailureReason))
		return subject, html
	}
}

// tryTemplate attempts to render an alert using the customisable template system.
// Returns false if the template repo is not configured or the template is disabled.
func (s *OwnerAlertService) tryTemplate(ctx context.Context, tenantID uuid.UUID, templateKey string, vars map[string]string) (string, string, bool) {
	if s.emailTemplateRepo == nil {
		return "", "", false
	}
	def, ok := email.OperationalTemplateByKey(templateKey)
	if !ok {
		return "", "", false
	}
	enabled := def.DefaultEnabled
	subject := def.Subject
	body := def.Body
	override, err := s.emailTemplateRepo.GetByKey(ctx, tenantID, templateKey)
	if err != nil {
		log.Printf("owner_alert template lookup failed tenant=%s template=%s err=%v", tenantID, templateKey, err)
		return "", "", false
	}
	if override != nil {
		enabled = override.Enabled
		if override.SubjectOverride != nil && strings.TrimSpace(*override.SubjectOverride) != "" {
			subject = *override.SubjectOverride
		}
		if override.BodyOverride != nil && strings.TrimSpace(*override.BodyOverride) != "" {
			body = *override.BodyOverride
		}
	}
	if !enabled {
		return "", "", false
	}
	render := func(input string) string {
		out := input
		for k, v := range vars {
			out = strings.ReplaceAll(out, "{{"+k+"}}", v)
		}
		return out
	}
	return render(subject), email.WrapHTML(render(body)), true
}

// OverduePayment represents a payment that was expected but hasn't arrived.
type OverduePayment struct {
	ClientName      string    `json:"client_name"`
	MembershipTitle string    `json:"membership_title"`
	Amount          float64   `json:"amount"`
	CurrencyCode    string    `json:"currency_code"`
	DueDate         time.Time `json:"due_date"`
	DaysOverdue     int       `json:"days_overdue"`
	PaymentMethod   string    `json:"payment_method"`
}

// ExpectedVsReceivedSummary is the output of the expected payment calendar check.
type ExpectedVsReceivedSummary struct {
	TotalExpected  int              `json:"total_expected"`
	TotalSucceeded int              `json:"total_succeeded"`
	TotalPending   int              `json:"total_pending"`
	TotalOverdue   int              `json:"total_overdue"`
	OverdueItems   []OverduePayment `json:"overdue_items"`
}

// CheckExpectedVsReceived calculates which payments were expected by now and flags missing ones.
func (s *OwnerAlertService) CheckExpectedVsReceived(ctx context.Context, tenantID uuid.UUID, graceDaysCard, graceDaysDD int) (*ExpectedVsReceivedSummary, error) {
	if graceDaysCard <= 0 {
		graceDaysCard = 1
	}
	if graceDaysDD <= 0 {
		graceDaysDD = 3
	}

	rows, err := s.db.Pool.Query(ctx, `
		WITH latest_payment AS (
			SELECT DISTINCT ON (p.membership_id)
				p.membership_id,
				p.due_date AS last_due_date,
				p.status AS last_status
			FROM payments p
			WHERE p.tenant_id = $1
			ORDER BY p.membership_id, COALESCE(p.paid_date, p.due_date, p.created_at) DESC, p.created_at DESC
		),
		expected AS (
			SELECT
				m.id AS membership_id,
				m.client_id,
				c.name AS client_name,
				mt.title AS membership_title,
				COALESCE(m.payment_amount, 0) AS amount,
				COALESCE(m.currency_code, 'AUD') AS currency_code,
				COALESCE(m.payment_method, 'card') AS payment_method,
				lp.last_due_date,
				CASE
					WHEN m.payment_frequency = 'weekly' THEN lp.last_due_date + INTERVAL '7 days'
					WHEN m.payment_frequency = 'fortnightly' THEN lp.last_due_date + INTERVAL '14 days'
					WHEN m.payment_frequency = 'monthly' THEN lp.last_due_date + INTERVAL '1 month'
					WHEN m.payment_frequency = 'quarterly' THEN lp.last_due_date + INTERVAL '3 months'
					WHEN m.payment_interval_unit = 'week' THEN lp.last_due_date + (m.payment_interval_count * INTERVAL '7 days')
					WHEN m.payment_interval_unit = 'month' THEN lp.last_due_date + (m.payment_interval_count * INTERVAL '1 month')
					ELSE lp.last_due_date + INTERVAL '1 month'
				END AS next_expected_date
			FROM memberships m
			JOIN clients c ON c.id = m.client_id AND c.tenant_id = m.tenant_id
			JOIN membership_types mt ON mt.id = m.membership_type_id AND mt.tenant_id = m.tenant_id
			LEFT JOIN latest_payment lp ON lp.membership_id = m.id
			WHERE m.tenant_id = $1
			  AND m.status IN ('active', 'past_due')
			  AND m.payment_frequency IS NOT NULL
			  AND m.payment_frequency <> 'upfront'
			  AND lp.last_due_date IS NOT NULL
		),
		with_grace AS (
			SELECT *,
				next_expected_date + CASE
					WHEN payment_method IN ('direct_debit', 'gocardless') THEN ($3 * INTERVAL '1 day')
					ELSE ($2 * INTERVAL '1 day')
				END AS grace_deadline
			FROM expected
		)
		SELECT
			client_name, membership_title, amount, currency_code, payment_method,
			next_expected_date, grace_deadline,
			(CURRENT_DATE - next_expected_date::date) AS days_overdue
		FROM with_grace
		WHERE grace_deadline::date <= CURRENT_DATE
		  AND NOT EXISTS (
			SELECT 1 FROM payments p2
			WHERE p2.tenant_id = $1
			  AND p2.membership_id = with_grace.membership_id
			  AND p2.due_date >= with_grace.next_expected_date::date
			  AND p2.status IN ('succeeded', 'pending')
		  )
		ORDER BY days_overdue DESC
	`, tenantID, graceDaysCard, graceDaysDD)
	if err != nil {
		return nil, fmt.Errorf("check expected vs received: %w", err)
	}
	defer rows.Close()

	var overdue []OverduePayment
	for rows.Next() {
		var item OverduePayment
		var graceDeadline time.Time
		if err := rows.Scan(
			&item.ClientName, &item.MembershipTitle, &item.Amount, &item.CurrencyCode,
			&item.PaymentMethod, &item.DueDate, &graceDeadline, &item.DaysOverdue,
		); err != nil {
			return nil, fmt.Errorf("scan overdue payment: %w", err)
		}
		overdue = append(overdue, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var totalExpected int
	_ = s.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM memberships
		WHERE tenant_id = $1 AND status IN ('active', 'past_due')
		  AND payment_frequency IS NOT NULL AND payment_frequency <> 'upfront'
	`, tenantID).Scan(&totalExpected)

	var totalSucceeded, totalPending int
	_ = s.db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE latest_status = 'succeeded'),
			COUNT(*) FILTER (WHERE latest_status = 'pending')
		FROM (
			SELECT DISTINCT ON (p.membership_id)
				p.membership_id,
				p.status AS latest_status
			FROM payments p
			JOIN memberships m ON m.id = p.membership_id AND m.tenant_id = p.tenant_id
			WHERE p.tenant_id = $1
			  AND m.status IN ('active', 'past_due')
			  AND m.payment_frequency IS NOT NULL
			  AND m.payment_frequency <> 'upfront'
			ORDER BY p.membership_id, COALESCE(p.paid_date, p.due_date, p.created_at) DESC, p.created_at DESC
		) latest
	`, tenantID).Scan(&totalSucceeded, &totalPending)

	return &ExpectedVsReceivedSummary{
		TotalExpected:  totalExpected,
		TotalSucceeded: totalSucceeded,
		TotalPending:   totalPending,
		TotalOverdue:   len(overdue),
		OverdueItems:   overdue,
	}, nil
}

// WebhookHealthSummary provides pipeline health metrics.
type WebhookHealthSummary struct {
	LastStripeWebhook     *time.Time `json:"last_stripe_webhook"`
	LastGoCardlessWebhook *time.Time `json:"last_gocardless_webhook"`
	Processed24h          int        `json:"processed_24h"`
	Failed24h             int        `json:"failed_24h"`
	Processed7d           int        `json:"processed_7d"`
	Failed7d              int        `json:"failed_7d"`
	Stuck                 int        `json:"stuck"`
}

// GetWebhookHealth returns webhook pipeline health metrics for a tenant.
func (s *OwnerAlertService) GetWebhookHealth(ctx context.Context, tenantID uuid.UUID) (*WebhookHealthSummary, error) {
	var wh WebhookHealthSummary

	_ = s.db.Pool.QueryRow(ctx, `
		SELECT MAX(received_at)
		FROM inbound_webhook_events
		WHERE tenant_id = $1 AND provider = 'stripe'
	`, tenantID).Scan(&wh.LastStripeWebhook)

	_ = s.db.Pool.QueryRow(ctx, `
		SELECT MAX(received_at)
		FROM inbound_webhook_events
		WHERE tenant_id = $1 AND provider = 'gocardless'
	`, tenantID).Scan(&wh.LastGoCardlessWebhook)

	_ = s.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM inbound_webhook_events
		WHERE tenant_id = $1 AND status = 'processed' AND received_at >= NOW() - INTERVAL '24 hours'
	`, tenantID).Scan(&wh.Processed24h)

	_ = s.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM inbound_webhook_events
		WHERE tenant_id = $1 AND status = 'failed' AND received_at >= NOW() - INTERVAL '24 hours'
	`, tenantID).Scan(&wh.Failed24h)

	_ = s.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM inbound_webhook_events
		WHERE tenant_id = $1 AND status = 'processed' AND received_at >= NOW() - INTERVAL '7 days'
	`, tenantID).Scan(&wh.Processed7d)

	_ = s.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM inbound_webhook_events
		WHERE tenant_id = $1 AND status = 'failed' AND received_at >= NOW() - INTERVAL '7 days'
	`, tenantID).Scan(&wh.Failed7d)

	_ = s.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM inbound_webhook_events
		WHERE tenant_id = $1 AND status = 'received' AND received_at < NOW() - INTERVAL '10 minutes'
	`, tenantID).Scan(&wh.Stuck)

	return &wh, nil
}

// PaymentAgingBucket groups failed payments by age.
type PaymentAgingBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// PaymentAgingSummary shows failed payment aging plus stale active memberships.
type PaymentAgingSummary struct {
	Buckets          []PaymentAgingBucket `json:"buckets"`
	ActiveUnpaid30d  int                  `json:"active_unpaid_30d"`
}

// GetPaymentAging returns failed payment aging buckets for a tenant.
func (s *OwnerAlertService) GetPaymentAging(ctx context.Context, tenantID uuid.UUID) (*PaymentAgingSummary, error) {
	var b1, b2, b3, b4 int
	_ = s.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE tenant_id = $1 AND status = 'failed' AND due_date >= CURRENT_DATE - 3`, tenantID).Scan(&b1)
	_ = s.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE tenant_id = $1 AND status = 'failed' AND due_date BETWEEN CURRENT_DATE - 7 AND CURRENT_DATE - 4`, tenantID).Scan(&b2)
	_ = s.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE tenant_id = $1 AND status = 'failed' AND due_date BETWEEN CURRENT_DATE - 14 AND CURRENT_DATE - 8`, tenantID).Scan(&b3)
	_ = s.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE tenant_id = $1 AND status = 'failed' AND due_date < CURRENT_DATE - 14`, tenantID).Scan(&b4)

	var activeUnpaid int
	_ = s.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM memberships m
		WHERE m.tenant_id = $1
		  AND m.status = 'active'
		  AND NOT EXISTS (
			SELECT 1 FROM payments p
			WHERE p.tenant_id = m.tenant_id
			  AND p.membership_id = m.id
			  AND p.status = 'succeeded'
			  AND p.paid_date >= CURRENT_DATE - 30
		  )
		  AND m.payment_frequency IS NOT NULL
		  AND m.payment_frequency <> 'upfront'
	`, tenantID).Scan(&activeUnpaid)

	return &PaymentAgingSummary{
		Buckets: []PaymentAgingBucket{
			{Label: "1-3 days", Count: b1},
			{Label: "4-7 days", Count: b2},
			{Label: "8-14 days", Count: b3},
			{Label: "14+ days", Count: b4},
		},
		ActiveUnpaid30d: activeUnpaid,
	}, nil
}

// WebhookAuditMismatch records a single webhook-to-DB verification failure.
type WebhookAuditMismatch struct {
	EventID         string    `json:"event_id"`
	Provider        string    `json:"provider"`
	ResourceType    string    `json:"resource_type"`
	ExpectedOutcome string    `json:"expected_outcome"`
	ActualStatus    string    `json:"actual_status"`
	PaymentID       *string   `json:"payment_id,omitempty"`
	MembershipID    *string   `json:"membership_id,omitempty"`
	ReceivedAt      time.Time `json:"received_at"`
	Category        string    `json:"category"`
}

// WebhookAuditSummary groups all webhook audit findings.
type WebhookAuditSummary struct {
	TotalAudited      int                    `json:"total_audited"`
	TotalVerified     int                    `json:"total_verified"`
	OutcomeMismatches []WebhookAuditMismatch `json:"outcome_mismatches"`
	StuckEvents       int                    `json:"stuck_events"`
	UnlinkedProcessed int                    `json:"unlinked_processed"`
}

// AuditWebhookOutcomes verifies that processed webhooks actually produced the expected DB changes.
func (s *OwnerAlertService) AuditWebhookOutcomes(ctx context.Context, tenantID uuid.UUID) (*WebhookAuditSummary, error) {
	summary := &WebhookAuditSummary{}

	// 1. Outcome mismatches: webhook says expected X but payment is actually Y
	rows, err := s.db.Pool.Query(ctx, `
		SELECT w.event_id, w.provider, w.resource_type, w.expected_outcome,
		       COALESCE(p.status, ''), COALESCE(p.id::text, ''), COALESCE(m.status, ''),
		       COALESCE(w.membership_id::text, ''), w.received_at
		FROM inbound_webhook_events w
		LEFT JOIN payments p ON p.id = w.payment_id
		LEFT JOIN memberships m ON m.id = w.membership_id
		WHERE w.tenant_id = $1
		  AND w.status = 'processed'
		  AND w.expected_outcome IS NOT NULL
		  AND w.received_at >= NOW() - INTERVAL '7 days'
		ORDER BY w.received_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("audit webhook outcomes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var eventID, provider, resourceType, expected, payStatus, payID, memStatus, memID string
		var receivedAt time.Time
		if err := rows.Scan(&eventID, &provider, &resourceType, &expected, &payStatus, &payID, &memStatus, &memID, &receivedAt); err != nil {
			return nil, fmt.Errorf("scan audit row: %w", err)
		}
		summary.TotalAudited++

		actualStatus := payStatus
		if actualStatus == "" {
			actualStatus = memStatus
		}

		if actualStatus == expected {
			summary.TotalVerified++
			continue
		}

		mm := WebhookAuditMismatch{
			EventID:         eventID,
			Provider:        provider,
			ResourceType:    resourceType,
			ExpectedOutcome: expected,
			ActualStatus:    actualStatus,
			ReceivedAt:      receivedAt,
			Category:        "outcome_mismatch",
		}
		if payID != "" {
			mm.PaymentID = &payID
		}
		if memID != "" {
			mm.MembershipID = &memID
		}
		summary.OutcomeMismatches = append(summary.OutcomeMismatches, mm)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 2. Stuck events: received but never processed (older than 10 minutes)
	_ = s.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM inbound_webhook_events
		WHERE tenant_id = $1 AND status = 'received'
		  AND received_at < NOW() - INTERVAL '10 minutes'
	`, tenantID).Scan(&summary.StuckEvents)

	// 3. Unlinked processed: marked processed but no payment/membership linked
	//    (only for resource types that should affect a payment)
	_ = s.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM inbound_webhook_events
		WHERE tenant_id = $1 AND status = 'processed'
		  AND payment_id IS NULL AND membership_id IS NULL
		  AND resource_type IN ('invoice', 'checkout', 'charge', 'payments', 'refunds', 'subscriptions')
		  AND received_at >= NOW() - INTERVAL '7 days'
	`, tenantID).Scan(&summary.UnlinkedProcessed)

	return summary, nil
}

// CountStuckWebhookEvents returns the count of events stuck in 'received' status globally (all tenants).
func (s *OwnerAlertService) CountStuckWebhookEvents(ctx context.Context) int {
	var count int
	_ = s.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM inbound_webhook_events
		WHERE status = 'received' AND received_at < NOW() - INTERVAL '10 minutes'
	`).Scan(&count)
	return count
}
