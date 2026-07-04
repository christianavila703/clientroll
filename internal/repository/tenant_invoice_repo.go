package repository

import (
	"context"
	"fmt"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
)

type TenantInvoice struct {
	ID            uuid.UUID  `json:"id"`
	TenantID      uuid.UUID  `json:"tenant_id"`
	StripeInvoiceID *string  `json:"stripe_invoice_id,omitempty"`
	AmountUSD     float64    `json:"amount_usd"`
	MemberCount   int        `json:"member_count"`
	PeriodStart   time.Time  `json:"period_start"`
	PeriodEnd     time.Time  `json:"period_end"`
	Status        string     `json:"status"`
	PaidAt        *time.Time `json:"paid_at,omitempty"`
	HostedInvoiceURL *string `json:"hosted_invoice_url,omitempty"`
	InvoicePDFURL    *string `json:"invoice_pdf_url,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type TenantInvoiceRepo struct {
	db *database.Postgres
}

func NewTenantInvoiceRepo(db *database.Postgres) *TenantInvoiceRepo {
	return &TenantInvoiceRepo{db: db}
}

func (r *TenantInvoiceRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID, limit int) ([]TenantInvoice, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rlsID, tenantUUID := database.TenantRLSScope(tenantID)
	query := `
		WITH _ AS (
			SELECT set_config('app.current_tenant_id', $1, true)
		)
		SELECT id, tenant_id, stripe_invoice_id, amount_usd, member_count, period_start, period_end,
		       status, paid_at, hosted_invoice_url, invoice_pdf_url, created_at
		FROM tenant_invoices
		WHERE tenant_id = $2
		ORDER BY created_at DESC
		LIMIT $3`

	rows, err := r.db.Pool.Query(ctx, query, rlsID, tenantUUID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing tenant invoices: %w", err)
	}
	defer rows.Close()

	var out []TenantInvoice
	for rows.Next() {
		var inv TenantInvoice
		if err := rows.Scan(
			&inv.ID, &inv.TenantID, &inv.StripeInvoiceID, &inv.AmountUSD, &inv.MemberCount,
			&inv.PeriodStart, &inv.PeriodEnd, &inv.Status, &inv.PaidAt, &inv.HostedInvoiceURL, &inv.InvoicePDFURL, &inv.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

type UpsertTenantInvoiceInput struct {
	StripeInvoiceID  string
	AmountUSD        float64
	MemberCount      int
	PeriodStart      time.Time
	PeriodEnd        time.Time
	Status           string
	PaidAt           *time.Time
	HostedInvoiceURL *string
	InvoicePDFURL    *string
}

func (r *TenantInvoiceRepo) UpsertByStripeInvoiceID(ctx context.Context, tenantID uuid.UUID, in UpsertTenantInvoiceInput) (*TenantInvoice, error) {
	rlsID, tenantUUID := database.TenantRLSScope(tenantID)
	query := `
		WITH _ AS (
			SELECT set_config('app.current_tenant_id', $1, true)
		)
		INSERT INTO tenant_invoices (tenant_id, stripe_invoice_id, amount_usd, member_count, period_start, period_end, status, paid_at, hosted_invoice_url, invoice_pdf_url)
		VALUES ($2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (stripe_invoice_id) DO UPDATE SET
			amount_usd = EXCLUDED.amount_usd,
			member_count = EXCLUDED.member_count,
			period_start = EXCLUDED.period_start,
			period_end = EXCLUDED.period_end,
			status = EXCLUDED.status,
			paid_at = EXCLUDED.paid_at,
			hosted_invoice_url = EXCLUDED.hosted_invoice_url,
			invoice_pdf_url = EXCLUDED.invoice_pdf_url
		RETURNING id, tenant_id, stripe_invoice_id, amount_usd, member_count, period_start, period_end, status, paid_at, hosted_invoice_url, invoice_pdf_url, created_at`

	var inv TenantInvoice
	err := r.db.Pool.QueryRow(ctx, query,
		rlsID,
		tenantUUID,
		in.StripeInvoiceID,
		in.AmountUSD,
		in.MemberCount,
		in.PeriodStart,
		in.PeriodEnd,
		in.Status,
		in.PaidAt,
		in.HostedInvoiceURL,
		in.InvoicePDFURL,
	).Scan(
		&inv.ID, &inv.TenantID, &inv.StripeInvoiceID, &inv.AmountUSD, &inv.MemberCount,
		&inv.PeriodStart, &inv.PeriodEnd, &inv.Status, &inv.PaidAt, &inv.HostedInvoiceURL, &inv.InvoicePDFURL, &inv.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upserting tenant invoice: %w", err)
	}
	return &inv, nil
}

