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
	Month    string  `json:"month"`
	Revenue  float64 `json:"revenue"`
	Expected float64 `json:"expected"`
}

type MembershipBreakdown struct {
	TypeTitle string  `json:"type_title"`
	Revenue   float64 `json:"revenue"`
	Count     int     `json:"count"`
}

type DashboardSummary struct {
	ActiveMembers    int     `json:"active_members"`
	MonthRevenue     float64 `json:"month_revenue"`
	FailedPayments   int     `json:"failed_payments"`
	NewSignups       int     `json:"new_signups"`
	OpenDisputes     int     `json:"open_disputes"`
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

	// Active members
	r.db.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM memberships WHERE tenant_id=$1 AND status='active'",
		tenantID).Scan(&s.ActiveMembers)

	// This month revenue
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	r.db.Pool.QueryRow(ctx,
		"SELECT COALESCE(SUM(amount), 0) FROM payments WHERE tenant_id=$1 AND status='succeeded' AND paid_date >= $2",
		tenantID, monthStart).Scan(&s.MonthRevenue)

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

	return &s, nil
}

// GetMonthlyRevenue returns monthly revenue for the past 12 months.
func (r *ReportsRepo) GetMonthlyRevenue(ctx context.Context, tenantID uuid.UUID) ([]MonthlyRevenue, error) {
	query := `
		SELECT TO_CHAR(paid_date, 'YYYY-MM') AS month,
		       COALESCE(SUM(amount), 0) AS revenue
		FROM payments
		WHERE tenant_id = $1 AND status = 'succeeded'
		  AND paid_date >= NOW() - INTERVAL '12 months'
		GROUP BY TO_CHAR(paid_date, 'YYYY-MM')
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
		       COALESCE(SUM(p.amount), 0) AS revenue,
		       COUNT(DISTINCT m.id) AS member_count
		FROM membership_types mt
		LEFT JOIN memberships m ON m.membership_type_id = mt.id AND m.tenant_id = mt.tenant_id
		LEFT JOIN payments p ON p.membership_id = m.id AND p.status = 'succeeded'
		WHERE mt.tenant_id = $1
		GROUP BY mt.title
		ORDER BY revenue DESC`

	rows, err := r.db.Pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("getting membership breakdown: %w", err)
	}
	defer rows.Close()

	var items []MembershipBreakdown
	for rows.Next() {
		var b MembershipBreakdown
		if err := rows.Scan(&b.TypeTitle, &b.Revenue, &b.Count); err != nil {
			return nil, err
		}
		items = append(items, b)
	}
	return items, rows.Err()
}

// GetPaymentStatusBreakdown returns payment counts by status.
func (r *ReportsRepo) GetPaymentStatusBreakdown(ctx context.Context, tenantID uuid.UUID) (map[string]int, error) {
	query := `SELECT status, COUNT(*) FROM payments WHERE tenant_id = $1 GROUP BY status`
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
