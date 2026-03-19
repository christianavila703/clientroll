package repository

import (
	"context"
	"fmt"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
)

// ReportData holds aggregated data for financial reports.
type MonthlyRevenue struct {
	Month        string  `json:"month"`
	CurrencyCode string  `json:"currency_code,omitempty"`
	Revenue      float64 `json:"revenue"`
	Expected     float64 `json:"expected"`
}

type MembershipBreakdown struct {
	TypeTitle    string  `json:"type_title"`
	CurrencyCode string  `json:"currency_code,omitempty"`
	Revenue      float64 `json:"revenue"`
	Count        int     `json:"count"`
}

type CurrencyTotal struct {
	CurrencyCode string  `json:"currency_code"`
	Revenue      float64 `json:"revenue"`
	Payments     int     `json:"payments"`
}

type DashboardSummary struct {
	ActiveMembers       int     `json:"active_members"`
	MonthRevenue        float64 `json:"month_revenue"`
	FailedPayments      int     `json:"failed_payments"`
	NewSignups          int     `json:"new_signups"`
	OpenDisputes        int     `json:"open_disputes"`
	ActiveMandates      int     `json:"active_mandates"`
	ActiveSubscriptions int     `json:"active_subscriptions"`
	PendingPayments     int     `json:"pending_payments"`
}

// ReportsRepo handles aggregated reporting queries.
type ReportsRepo struct {
	db *database.Postgres
}

// NewReportsRepo creates a new reports repository.
func NewReportsRepo(db *database.Postgres) *ReportsRepo {
	return &ReportsRepo{db: db}
}

// GetDashboardSummary returns summary card data for the dashboard.
func (r *ReportsRepo) GetDashboardSummary(ctx context.Context, tenantID uuid.UUID) (*DashboardSummary, error) {
	var s DashboardSummary

	// Active members: distinct non-deleted clients with at least one active membership.
	r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT m.client_id)
		FROM memberships m
		JOIN clients c ON c.id = m.client_id
		WHERE m.tenant_id = $1
		  AND m.status = 'active'
		  AND c.deleted_at IS NULL
	`, tenantID).Scan(&s.ActiveMembers)

	// This month revenue (membership payments + service charges)
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	r.db.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(rev), 0) FROM (
			SELECT amount AS rev FROM payments WHERE tenant_id=$1 AND status='succeeded' AND paid_date >= $2
			UNION ALL
			SELECT amount + tax_amount AS rev FROM service_charges WHERE tenant_id=$1 AND status='succeeded' AND paid_date >= $2
		) combined
	`, tenantID, monthStart).Scan(&s.MonthRevenue)

	// Failed payments (unresolved)
	r.db.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM payments WHERE tenant_id=$1 AND status='failed'",
		tenantID).Scan(&s.FailedPayments)

	// New signups this month
	r.db.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM memberships WHERE tenant_id=$1 AND created_at >= $2",
		tenantID, monthStart).Scan(&s.NewSignups)

	// Open disputes
	r.db.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM payment_disputes WHERE tenant_id=$1 AND status IN ('open', 'under_review')",
		tenantID).Scan(&s.OpenDisputes)

	// Direct debit stats
	r.db.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM memberships WHERE tenant_id=$1 AND gocardless_mandate_id IS NOT NULL AND status='active'",
		tenantID).Scan(&s.ActiveMandates)

	r.db.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM memberships WHERE tenant_id=$1 AND gocardless_subscription_id IS NOT NULL AND status='active'",
		tenantID).Scan(&s.ActiveSubscriptions)

	r.db.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM payments WHERE tenant_id=$1 AND status IN ('pending', 'submitted')",
		tenantID).Scan(&s.PendingPayments)

	return &s, nil
}

// GetMonthlyRevenue returns monthly revenue for the past 12 months (membership payments + service charges).
func (r *ReportsRepo) GetMonthlyRevenue(ctx context.Context, tenantID uuid.UUID) ([]MonthlyRevenue, error) {
	query := `
		SELECT month, COALESCE(SUM(revenue), 0) AS revenue FROM (
			SELECT TO_CHAR(paid_date, 'YYYY-MM') AS month, amount AS revenue
			FROM payments
			WHERE tenant_id = $1 AND status = 'succeeded' AND paid_date >= NOW() - INTERVAL '12 months'
			UNION ALL
			SELECT TO_CHAR(paid_date, 'YYYY-MM') AS month, amount + tax_amount AS revenue
			FROM service_charges
			WHERE tenant_id = $1 AND status = 'succeeded' AND paid_date >= NOW() - INTERVAL '12 months'
		) combined
		GROUP BY month
		ORDER BY month`

	rows, err := r.db.Pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("getting monthly revenue: %w", err)
	}
	defer rows.Close()

	var items []MonthlyRevenue
	for rows.Next() {
		var m MonthlyRevenue
		if err := rows.Scan(&m.Month, &m.Revenue); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

// GetMembershipBreakdown returns revenue and count per membership type.
func (r *ReportsRepo) GetMembershipBreakdown(ctx context.Context, tenantID uuid.UUID) ([]MembershipBreakdown, error) {
	query := `
		SELECT mt.title,
		       COALESCE(mt.currency_code, 'AUD') AS currency_code,
		       COALESCE(SUM(p.amount), 0) AS revenue,
		       COUNT(DISTINCT m.id) AS member_count
		FROM memberships m
		JOIN membership_types mt ON mt.id = m.membership_type_id AND mt.tenant_id = m.tenant_id
		JOIN clients c ON c.id = m.client_id
		LEFT JOIN payments p ON p.membership_id = m.id AND p.status = 'succeeded'
		WHERE mt.tenant_id = $1
		  AND mt.is_active = TRUE
		  AND mt.deleted_at IS NULL
		  AND m.status = 'active'
		  AND c.deleted_at IS NULL
		GROUP BY mt.title, COALESCE(mt.currency_code, 'AUD')
		ORDER BY member_count DESC, revenue DESC, mt.title ASC`

	rows, err := r.db.Pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("getting membership breakdown: %w", err)
	}
	defer rows.Close()

	var items []MembershipBreakdown
	for rows.Next() {
		var b MembershipBreakdown
		if err := rows.Scan(&b.TypeTitle, &b.CurrencyCode, &b.Revenue, &b.Count); err != nil {
			return nil, err
		}
		items = append(items, b)
	}
	return items, rows.Err()
}

// ComprehensiveReport holds all report data.
type ComprehensiveReport struct {
	// KPIs
	ActiveMembers       int     `json:"active_members"`
	TotalClients        int     `json:"total_clients"`
	MonthRevenue        float64 `json:"month_revenue"`
	PrevMonthRevenue    float64 `json:"prev_month_revenue"`
	TotalRevenue        float64 `json:"total_revenue"`
	MRR                 float64 `json:"mrr"`
	ARR                 float64 `json:"arr"`
	AvgRevenuePerMember float64 `json:"avg_revenue_per_member"`
	FailedPayments      int     `json:"failed_payments"`
	NewSignupsMonth     int     `json:"new_signups_month"`
	ChurnedMonth        int     `json:"churned_month"`
	CollectionRate      float64 `json:"collection_rate"`
	AvgMembershipDays   int     `json:"avg_membership_days"`

	// Time series
	MonthlyRevenue []MonthlyRevenue `json:"monthly_revenue"`
	MonthlySignups []MonthlyCount   `json:"monthly_signups"`
	MonthlyChurn   []MonthlyCount   `json:"monthly_churn"`

	// Breakdowns
	MembershipBreakdown []MembershipBreakdown `json:"membership_breakdown"`
	PaymentStatuses     map[string]int        `json:"payment_statuses"`
	FrequencyBreakdown  []FrequencyBreakdown  `json:"frequency_breakdown"`
	RevenueByCurrency   []CurrencyTotal       `json:"revenue_by_currency"`
	TopClients          []TopClient           `json:"top_clients"`
	UpcomingExpirations []UpcomingExpiry      `json:"upcoming_expirations"`
	RecentFailures      []RecentFailure       `json:"recent_failures"`
}

type MonthlyCount struct {
	Month string `json:"month"`
	Count int    `json:"count"`
}

type FrequencyBreakdown struct {
	Frequency string  `json:"frequency"`
	Count     int     `json:"count"`
	Revenue   float64 `json:"revenue"`
}

type TopClient struct {
	ClientID    string  `json:"client_id"`
	ClientName  string  `json:"client_name"`
	TotalPaid   float64 `json:"total_paid"`
	Memberships int     `json:"memberships"`
}

type UpcomingExpiry struct {
	ClientName     string `json:"client_name"`
	MembershipType string `json:"membership_type"`
	EndDate        string `json:"end_date"`
	DaysLeft       int    `json:"days_left"`
}

type RecentFailure struct {
	ClientName    string  `json:"client_name"`
	Amount        float64 `json:"amount"`
	FailureReason string  `json:"failure_reason"`
	DueDate       string  `json:"due_date"`
}

// PaymentConsistencyReport highlights ledger mismatches between service charges and provider payments.
type PaymentConsistencyReport struct {
	ServiceChargesLast24h                   int `json:"service_charges_last_24h"`
	ServiceChargesWithProviderPayment24h    int `json:"service_charges_with_provider_payment_last_24h"`
	ServiceChargesMissingProviderPayment24h int `json:"service_charges_missing_provider_payment_last_24h"`
	ProviderPaymentsMissingServiceCharge24h int `json:"provider_payments_missing_service_charge_last_24h"`
}

// AlertStatus describes threshold-based operational alerts for payment/webhook flows.
type AlertStatus struct {
	RetryQueueDepth            int      `json:"retry_queue_depth"`
	FailedWebhookDeliveries15m int      `json:"failed_webhook_deliveries_15m"`
	StuckRetries               int      `json:"stuck_retries"`
	PaymentFailureRate24h      float64  `json:"payment_failure_rate_24h"`
	OpenFailedPayments         int      `json:"open_failed_payments"`
	Issues                     []string `json:"issues"`
}

// GetComprehensiveReport returns all report data in one call.
func (r *ReportsRepo) GetComprehensiveReport(ctx context.Context, tenantID uuid.UUID) (*ComprehensiveReport, error) {
	rpt := &ComprehensiveReport{
		PaymentStatuses: make(map[string]int),
	}
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	prevMonthStart := monthStart.AddDate(0, -1, 0)

	// Active members: distinct non-deleted clients with at least one active membership.
	r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT m.client_id)
		FROM memberships m
		JOIN clients c ON c.id = m.client_id
		WHERE m.tenant_id = $1
		  AND m.status = 'active'
		  AND c.deleted_at IS NULL
	`, tenantID).Scan(&rpt.ActiveMembers)

	// Total clients
	r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM clients WHERE tenant_id=$1", tenantID).Scan(&rpt.TotalClients)

	// This month revenue (membership + service)
	r.db.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(rev),0) FROM (
			SELECT amount AS rev FROM payments WHERE tenant_id=$1 AND status='succeeded' AND paid_date >= $2
			UNION ALL
			SELECT amount + tax_amount AS rev FROM service_charges WHERE tenant_id=$1 AND status='succeeded' AND paid_date >= $2
		) x`, tenantID, monthStart).Scan(&rpt.MonthRevenue)

	// Previous month revenue
	r.db.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(rev),0) FROM (
			SELECT amount AS rev FROM payments WHERE tenant_id=$1 AND status='succeeded' AND paid_date >= $2 AND paid_date < $3
			UNION ALL
			SELECT amount + tax_amount AS rev FROM service_charges WHERE tenant_id=$1 AND status='succeeded' AND paid_date >= $2 AND paid_date < $3
		) x`, tenantID, prevMonthStart, monthStart).Scan(&rpt.PrevMonthRevenue)

	// Total revenue all time
	r.db.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(rev),0) FROM (
			SELECT amount AS rev FROM payments WHERE tenant_id=$1 AND status='succeeded'
			UNION ALL
			SELECT amount + tax_amount AS rev FROM service_charges WHERE tenant_id=$1 AND status='succeeded'
		) x`, tenantID).Scan(&rpt.TotalRevenue)

	// MRR (sum of monthly payment amounts for active memberships)
	r.db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(
		CASE WHEN payment_frequency='monthly' THEN payment_amount
		     WHEN payment_frequency='fortnightly' THEN payment_amount * 2.17
		     WHEN payment_frequency='weekly' THEN payment_amount * 4.33
		     ELSE payment_amount END
	), 0) FROM memberships WHERE tenant_id=$1 AND status='active'`, tenantID).Scan(&rpt.MRR)
	rpt.ARR = rpt.MRR * 12

	// Avg revenue per member
	if rpt.ActiveMembers > 0 {
		rpt.AvgRevenuePerMember = rpt.TotalRevenue / float64(rpt.ActiveMembers)
	}

	// Failed payments
	r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM payments WHERE tenant_id=$1 AND status='failed'", tenantID).Scan(&rpt.FailedPayments)

	// New signups this month
	r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM memberships WHERE tenant_id=$1 AND created_at >= $2", tenantID, monthStart).Scan(&rpt.NewSignupsMonth)

	// Churned this month (expired or cancelled)
	r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM memberships WHERE tenant_id=$1 AND status IN ('expired','cancelled') AND updated_at >= $2", tenantID, monthStart).Scan(&rpt.ChurnedMonth)

	// Collection rate
	var totalDue, totalCollected float64
	r.db.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(rev),0) FROM (
			SELECT amount AS rev FROM payments WHERE tenant_id=$1
			UNION ALL
			SELECT amount + tax_amount AS rev FROM service_charges WHERE tenant_id=$1
		) x`, tenantID).Scan(&totalDue)
	r.db.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(rev),0) FROM (
			SELECT amount AS rev FROM payments WHERE tenant_id=$1 AND status='succeeded'
			UNION ALL
			SELECT amount + tax_amount AS rev FROM service_charges WHERE tenant_id=$1 AND status='succeeded'
		) x`, tenantID).Scan(&totalCollected)
	if totalDue > 0 {
		rpt.CollectionRate = (totalCollected / totalDue) * 100
	}

	// Avg membership duration
	r.db.Pool.QueryRow(ctx, `SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (min_term_end_date - start_date)) / 86400), 0)::int FROM memberships WHERE tenant_id=$1`, tenantID).Scan(&rpt.AvgMembershipDays)

	// Monthly revenue (12 months)
	rpt.MonthlyRevenue, _ = r.GetMonthlyRevenue(ctx, tenantID)

	// Revenue totals by currency (membership + service).
	rowsCurrency, err := r.db.Pool.Query(ctx, `
		SELECT currency_code, COALESCE(SUM(revenue), 0) AS revenue, SUM(payments)::int AS payments FROM (
			SELECT COALESCE(currency_code, 'AUD') AS currency_code, amount AS revenue, 1 AS payments
			FROM payments WHERE tenant_id = $1 AND status = 'succeeded'
			UNION ALL
			SELECT 'AUD' AS currency_code, amount + tax_amount AS revenue, 1 AS payments
			FROM service_charges WHERE tenant_id = $1 AND status = 'succeeded'
		) combined
		GROUP BY currency_code
		ORDER BY currency_code
	`, tenantID)
	if err == nil {
		defer rowsCurrency.Close()
		for rowsCurrency.Next() {
			var c CurrencyTotal
			rowsCurrency.Scan(&c.CurrencyCode, &c.Revenue, &c.Payments)
			rpt.RevenueByCurrency = append(rpt.RevenueByCurrency, c)
		}
	}

	// Monthly signups (12 months)
	rows1, err := r.db.Pool.Query(ctx, `SELECT TO_CHAR(created_at, 'YYYY-MM'), COUNT(*) FROM memberships WHERE tenant_id=$1 AND created_at >= NOW() - INTERVAL '12 months' GROUP BY TO_CHAR(created_at, 'YYYY-MM') ORDER BY 1`, tenantID)
	if err == nil {
		defer rows1.Close()
		for rows1.Next() {
			var mc MonthlyCount
			rows1.Scan(&mc.Month, &mc.Count)
			rpt.MonthlySignups = append(rpt.MonthlySignups, mc)
		}
	}

	// Monthly churn
	rows2, err := r.db.Pool.Query(ctx, `SELECT TO_CHAR(updated_at, 'YYYY-MM'), COUNT(*) FROM memberships WHERE tenant_id=$1 AND status IN ('expired','cancelled') AND updated_at >= NOW() - INTERVAL '12 months' GROUP BY TO_CHAR(updated_at, 'YYYY-MM') ORDER BY 1`, tenantID)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var mc MonthlyCount
			rows2.Scan(&mc.Month, &mc.Count)
			rpt.MonthlyChurn = append(rpt.MonthlyChurn, mc)
		}
	}

	// Membership breakdown
	rpt.MembershipBreakdown, _ = r.GetMembershipBreakdown(ctx, tenantID)

	// Payment statuses
	rpt.PaymentStatuses, _ = r.GetPaymentStatusBreakdown(ctx, tenantID)

	// Frequency breakdown
	rows3, err := r.db.Pool.Query(ctx, `SELECT COALESCE(payment_frequency,'unknown'), COUNT(*), COALESCE(SUM(payment_amount),0) FROM memberships WHERE tenant_id=$1 AND status='active' GROUP BY payment_frequency ORDER BY 2 DESC`, tenantID)
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var fb FrequencyBreakdown
			rows3.Scan(&fb.Frequency, &fb.Count, &fb.Revenue)
			rpt.FrequencyBreakdown = append(rpt.FrequencyBreakdown, fb)
		}
	}

	// Top 10 clients by revenue (membership payments + service charges)
	rows4, err := r.db.Pool.Query(ctx, `
		SELECT c.id::text, c.name,
			COALESCE((SELECT SUM(p.amount) FROM payments p WHERE p.client_id=c.id AND p.status='succeeded'), 0)
			+ COALESCE((SELECT SUM(sc.amount + sc.tax_amount) FROM service_charges sc WHERE sc.client_id=c.id AND sc.status='succeeded'), 0) AS total_rev,
			(SELECT COUNT(DISTINCT m.id) FROM memberships m WHERE m.client_id=c.id) AS memberships
		FROM clients c
		WHERE c.tenant_id=$1 AND c.deleted_at IS NULL
		GROUP BY c.id, c.name
		ORDER BY total_rev DESC LIMIT 10`, tenantID)
	if err == nil {
		defer rows4.Close()
		for rows4.Next() {
			var tc TopClient
			rows4.Scan(&tc.ClientID, &tc.ClientName, &tc.TotalPaid, &tc.Memberships)
			rpt.TopClients = append(rpt.TopClients, tc)
		}
	}

	// Upcoming expirations (next 60 days)
	rows5, err := r.db.Pool.Query(ctx, `SELECT c.name, mt.title, TO_CHAR(m.min_term_end_date, 'YYYY-MM-DD'), EXTRACT(DAY FROM m.min_term_end_date - NOW())::int FROM memberships m JOIN clients c ON c.id=m.client_id JOIN membership_types mt ON mt.id=m.membership_type_id WHERE m.tenant_id=$1 AND m.status='active' AND m.min_term_end_date <= NOW() + INTERVAL '60 days' AND m.min_term_end_date > NOW() ORDER BY m.min_term_end_date LIMIT 10`, tenantID)
	if err == nil {
		defer rows5.Close()
		for rows5.Next() {
			var ue UpcomingExpiry
			rows5.Scan(&ue.ClientName, &ue.MembershipType, &ue.EndDate, &ue.DaysLeft)
			rpt.UpcomingExpirations = append(rpt.UpcomingExpirations, ue)
		}
	}

	// Recent failures
	rows6, err := r.db.Pool.Query(ctx, `SELECT c.name, p.amount, COALESCE(p.failure_reason,'Unknown'), TO_CHAR(p.due_date, 'YYYY-MM-DD') FROM payments p JOIN clients c ON c.id=p.client_id WHERE p.tenant_id=$1 AND p.status='failed' ORDER BY p.created_at DESC LIMIT 10`, tenantID)
	if err == nil {
		defer rows6.Close()
		for rows6.Next() {
			var rf RecentFailure
			rows6.Scan(&rf.ClientName, &rf.Amount, &rf.FailureReason, &rf.DueDate)
			rpt.RecentFailures = append(rpt.RecentFailures, rf)
		}
	}

	return rpt, nil
}

// GetPaymentStatusBreakdown returns payment counts by status (membership payments + service charges).
func (r *ReportsRepo) GetPaymentStatusBreakdown(ctx context.Context, tenantID uuid.UUID) (map[string]int, error) {
	query := `
		SELECT status, SUM(cnt)::int FROM (
			SELECT status, COUNT(*) AS cnt FROM payments WHERE tenant_id = $1 GROUP BY status
			UNION ALL
			SELECT status, COUNT(*) AS cnt FROM service_charges WHERE tenant_id = $1 GROUP BY status
		) combined
		GROUP BY status`
	rows, err := r.db.Pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		rows.Scan(&status, &count)
		counts[status] = count
	}
	return counts, rows.Err()
}

// GetPaymentConsistencyReport returns consistency metrics for service charges vs provider payments.
func (r *ReportsRepo) GetPaymentConsistencyReport(ctx context.Context, tenantID uuid.UUID) (*PaymentConsistencyReport, error) {
	out := &PaymentConsistencyReport{}

	if err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM service_charges
		WHERE tenant_id = $1
		  AND created_at >= NOW() - INTERVAL '24 hours'
	`, tenantID).Scan(&out.ServiceChargesLast24h); err != nil {
		return nil, fmt.Errorf("count service charges: %w", err)
	}

	if err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM service_charges sc
		WHERE sc.tenant_id = $1
		  AND sc.created_at >= NOW() - INTERVAL '24 hours'
		  AND EXISTS (
		    SELECT 1 FROM payments p
		    WHERE p.tenant_id = sc.tenant_id
		      AND p.provider_reference = ('charge-' || sc.id::text)
		  )
	`, tenantID).Scan(&out.ServiceChargesWithProviderPayment24h); err != nil {
		return nil, fmt.Errorf("count matched service charges: %w", err)
	}

	out.ServiceChargesMissingProviderPayment24h = out.ServiceChargesLast24h - out.ServiceChargesWithProviderPayment24h
	if out.ServiceChargesMissingProviderPayment24h < 0 {
		out.ServiceChargesMissingProviderPayment24h = 0
	}

	if err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM payments p
		WHERE p.tenant_id = $1
		  AND p.created_at >= NOW() - INTERVAL '24 hours'
		  AND p.provider_reference LIKE 'charge-%'
		  AND NOT EXISTS (
		    SELECT 1 FROM service_charges sc
		    WHERE sc.tenant_id = p.tenant_id
		      AND p.provider_reference = ('charge-' || sc.id::text)
		  )
	`, tenantID).Scan(&out.ProviderPaymentsMissingServiceCharge24h); err != nil {
		return nil, fmt.Errorf("count orphan provider payments: %w", err)
	}

	return out, nil
}

// GetOperationalAlertStatus returns threshold-based alerts for key payment/webhook risks.
func (r *ReportsRepo) GetOperationalAlertStatus(ctx context.Context, tenantID uuid.UUID) (*AlertStatus, error) {
	out := &AlertStatus{Issues: []string{}}

	if err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM webhook_deliveries
		WHERE tenant_id = $1 AND status = 'retrying'
	`, tenantID).Scan(&out.RetryQueueDepth); err != nil {
		return nil, fmt.Errorf("count retrying deliveries: %w", err)
	}

	if err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM webhook_deliveries
		WHERE tenant_id = $1
		  AND status = 'failed'
		  AND created_at >= NOW() - INTERVAL '15 minutes'
	`, tenantID).Scan(&out.FailedWebhookDeliveries15m); err != nil {
		return nil, fmt.Errorf("count failed deliveries: %w", err)
	}

	if err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM webhook_deliveries
		WHERE tenant_id = $1
		  AND status = 'retrying'
		  AND next_retry_at IS NOT NULL
		  AND next_retry_at < NOW() - INTERVAL '10 minutes'
	`, tenantID).Scan(&out.StuckRetries); err != nil {
		return nil, fmt.Errorf("count stuck retries: %w", err)
	}

	var totalPayments24h int
	var failedPayments24h int
	if err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM payments
		WHERE tenant_id = $1
		  AND created_at >= NOW() - INTERVAL '24 hours'
	`, tenantID).Scan(&totalPayments24h); err != nil {
		return nil, fmt.Errorf("count payments 24h: %w", err)
	}
	if err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM payments
		WHERE tenant_id = $1
		  AND status = 'failed'
		  AND created_at >= NOW() - INTERVAL '24 hours'
	`, tenantID).Scan(&failedPayments24h); err != nil {
		return nil, fmt.Errorf("count failed payments 24h: %w", err)
	}
	if totalPayments24h > 0 {
		out.PaymentFailureRate24h = float64(failedPayments24h) / float64(totalPayments24h) * 100
	}

	if err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM payments
		WHERE tenant_id = $1
		  AND status = 'failed'
	`, tenantID).Scan(&out.OpenFailedPayments); err != nil {
		return nil, fmt.Errorf("count open failed payments: %w", err)
	}

	// Baseline thresholds; can be moved to env config later.
	if out.RetryQueueDepth >= 25 {
		out.Issues = append(out.Issues, "retry_queue_depth_high")
	}
	if out.FailedWebhookDeliveries15m >= 5 {
		out.Issues = append(out.Issues, "failed_webhook_deliveries_spike")
	}
	if out.StuckRetries > 0 {
		out.Issues = append(out.Issues, "stuck_webhook_retries")
	}
	if out.PaymentFailureRate24h >= 15 {
		out.Issues = append(out.Issues, "payment_failure_rate_high")
	}
	if out.OpenFailedPayments >= 20 {
		out.Issues = append(out.Issues, "open_failed_payments_high")
	}

	return out, nil
}
