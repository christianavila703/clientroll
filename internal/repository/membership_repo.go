package repository

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MembershipRepo handles database operations for memberships (agreements).
type MembershipRepo struct {
	db *database.Postgres
}

// NewMembershipRepo creates a new membership repository.
func NewMembershipRepo(db *database.Postgres) *MembershipRepo {
	return &MembershipRepo{db: db}
}

// SetPaymentScheduleID stamps the v2 payment_schedule_id onto a membership.
// Safe to call when the column does not yet exist (migrations not applied):
// the UPDATE returns a "column does not exist" error which the caller can
// log as non-fatal during the soak window.
func (r *MembershipRepo) SetPaymentScheduleID(ctx context.Context, membershipID, tenantID, scheduleID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE memberships SET payment_schedule_id = $3, updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2`,
		membershipID, tenantID, scheduleID)
	return err
}

// Create inserts a new membership from a signup submission.
func (r *MembershipRepo) Create(
	ctx context.Context,
	tenantID, clientID, membershipTypeID, termOptionID uuid.UUID,
	startDate time.Time,
	paymentMethod, paymentFrequency string,
	currencyCode string,
	paymentAmount float64,
	paymentIntervalUnit *string,
	paymentIntervalCount int,
	numPayments *int,
	continueAfterTerm bool,
	memberSignature string,
	staffUserID *uuid.UUID,
	staffSignature string,
) (*models.Membership, error) {
	// Calculate end date from term option
	var durationMonths int
	err := r.db.Pool.QueryRow(ctx,
		"SELECT duration_months FROM membership_term_options WHERE id=$1 AND tenant_id=$2",
		termOptionID, tenantID).Scan(&durationMonths)
	if err != nil {
		return nil, fmt.Errorf("looking up term option: %w", err)
	}

	endDate := startDate.AddDate(0, durationMonths, 0)
	now := time.Now()

	var memberSignedAt, staffSignedAt *time.Time
	if memberSignature != "" {
		memberSignedAt = &now
	}
	if staffSignature != "" {
		staffSignedAt = &now
	}

	status := "active"
	var startsInFuture bool
	if err := r.db.Pool.QueryRow(ctx, `
		SELECT $1::date > ((NOW() AT TIME ZONE COALESCE(NULLIF(timezone, ''), 'UTC'))::date)
		FROM tenants
		WHERE id = $2
	`, startDate, tenantID).Scan(&startsInFuture); err == nil {
		if startsInFuture {
			status = "scheduled"
		}
	} else if startDate.UTC().After(time.Now().UTC()) {
		// Fallback keeps previous behavior if tenant timezone lookup fails unexpectedly.
		status = "scheduled"
	}

	query := `
		INSERT INTO memberships (tenant_id, client_id, membership_type_id, term_option_id,
		    start_date, min_term_end_date, payment_method, payment_frequency, currency_code, payment_interval_unit, payment_interval_count, payment_amount, num_payments,
		    continue_after_term, status, member_signature_data, member_signed_at,
		    staff_user_id, staff_signature_data, staff_signed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		RETURNING id, tenant_id, client_id, membership_type_id, term_option_id,
		    start_date, min_term_end_date, payment_method, payment_frequency, currency_code, payment_interval_unit, payment_interval_count, payment_amount, num_payments, payments_made,
		    continue_after_term, status, gocardless_mandate_id, gocardless_subscription_id, member_signature_data, member_signed_at,
		    staff_user_id, staff_signature_data, staff_signed_at, created_at, updated_at`

	var m models.Membership
	err = r.db.Pool.QueryRow(ctx, query,
		tenantID, clientID, membershipTypeID, termOptionID,
		startDate, endDate, paymentMethod, paymentFrequency, currencyCode, paymentIntervalUnit, paymentIntervalCount, paymentAmount, numPayments,
		continueAfterTerm, status, memberSignature, memberSignedAt, staffUserID, staffSignature, staffSignedAt,
	).Scan(
		&m.ID, &m.TenantID, &m.ClientID, &m.MembershipTypeID, &m.TermOptionID,
		&m.StartDate, &m.MinTermEndDate, &m.PaymentMethod, &m.PaymentFrequency, &m.CurrencyCode, &m.PaymentIntervalUnit, &m.PaymentIntervalCount,
		&m.PaymentAmount, &m.NumPayments, &m.PaymentsMade, &m.ContinueAfterTerm, &m.Status, &m.GoCardlessMandateID, &m.GoCardlessSubscriptionID, &m.MemberSignature, &m.MemberSignedAt,
		&m.StaffUserID, &m.StaffSignature, &m.StaffSignedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating membership: %w", err)
	}
	return &m, nil
}

// FindUnpaidDuplicateCandidate returns an existing non-cancelled membership that matches
// the requested signup payload, has no payment rows yet, and was created very recently.
// This blocks near-immediate accidental retries while still allowing legitimate same-day
// or same-client additional memberships created after a short gap.
func (r *MembershipRepo) FindUnpaidDuplicateCandidate(
	ctx context.Context,
	tenantID, clientID, membershipTypeID, termOptionID uuid.UUID,
	startDate time.Time,
	paymentMethod, paymentFrequency string,
	paymentAmount float64,
	paymentIntervalUnit *string,
	paymentIntervalCount int,
) (*models.Membership, error) {
	paymentMethod = strings.ToLower(strings.TrimSpace(paymentMethod))
	paymentFrequency = strings.ToLower(strings.TrimSpace(paymentFrequency))
	unit := ""
	if paymentIntervalUnit != nil {
		unit = strings.ToLower(strings.TrimSpace(*paymentIntervalUnit))
	}
	if paymentIntervalCount <= 0 {
		paymentIntervalCount = 1
	}

	query := `
		SELECT m.id, m.tenant_id, m.client_id, m.membership_type_id, m.term_option_id,
		    m.start_date, m.min_term_end_date, m.payment_method, m.payment_frequency, m.currency_code, m.payment_interval_unit, m.payment_interval_count, m.payment_amount, m.num_payments, m.payments_made,
		    m.continue_after_term, m.status, m.gocardless_mandate_id, m.gocardless_subscription_id, m.member_signature_data, m.member_signed_at,
		    m.staff_user_id, m.staff_signature_data, m.staff_signed_at, m.created_at, m.updated_at
		FROM memberships m
		WHERE m.tenant_id = $1
		  AND m.client_id = $2
		  AND m.membership_type_id = $3
		  AND m.term_option_id = $4
		  AND m.start_date = $5::date
		  AND LOWER(COALESCE(m.payment_method, '')) = $6
		  AND LOWER(COALESCE(m.payment_frequency, '')) = $7
		  AND COALESCE(m.payment_interval_count, 1) = $8
		  AND LOWER(COALESCE(m.payment_interval_unit, '')) = $9
		  AND ABS(COALESCE(m.payment_amount, 0) - $10) <= 0.01
		  AND m.status IN ('pending', 'scheduled', 'active', 'past_due')
		  AND NOT EXISTS (SELECT 1 FROM payments p WHERE p.membership_id = m.id)
		  AND m.created_at >= (NOW() - INTERVAL '5 minutes')
		ORDER BY m.created_at DESC
		LIMIT 1`

	var m models.Membership
	err := r.db.Pool.QueryRow(
		ctx,
		query,
		tenantID, clientID, membershipTypeID, termOptionID,
		startDate, paymentMethod, paymentFrequency, paymentIntervalCount, unit, math.Round(paymentAmount*100)/100,
	).Scan(
		&m.ID, &m.TenantID, &m.ClientID, &m.MembershipTypeID, &m.TermOptionID,
		&m.StartDate, &m.MinTermEndDate, &m.PaymentMethod, &m.PaymentFrequency, &m.CurrencyCode, &m.PaymentIntervalUnit, &m.PaymentIntervalCount,
		&m.PaymentAmount, &m.NumPayments, &m.PaymentsMade, &m.ContinueAfterTerm, &m.Status, &m.GoCardlessMandateID, &m.GoCardlessSubscriptionID,
		&m.MemberSignature, &m.MemberSignedAt,
		&m.StaffUserID, &m.StaffSignature, &m.StaffSignedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding duplicate candidate membership: %w", err)
	}
	return &m, nil
}

// FindByID returns a membership by ID.
func (r *MembershipRepo) FindByID(ctx context.Context, id, tenantID uuid.UUID) (*models.Membership, error) {
	query := `
		SELECT id, tenant_id, client_id, membership_type_id, term_option_id,
		    start_date, min_term_end_date, payment_method, payment_frequency, currency_code, payment_interval_unit, payment_interval_count, payment_amount, num_payments, payments_made,
		    continue_after_term, status, gocardless_mandate_id, gocardless_subscription_id, member_signature_data, member_signed_at,
		    staff_user_id, staff_signature_data, staff_signed_at, created_at, updated_at
		FROM memberships WHERE id = $1 AND tenant_id = $2`

	var m models.Membership
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID).Scan(
		&m.ID, &m.TenantID, &m.ClientID, &m.MembershipTypeID, &m.TermOptionID,
		&m.StartDate, &m.MinTermEndDate, &m.PaymentMethod, &m.PaymentFrequency, &m.CurrencyCode, &m.PaymentIntervalUnit, &m.PaymentIntervalCount,
		&m.PaymentAmount, &m.NumPayments, &m.PaymentsMade, &m.ContinueAfterTerm, &m.Status, &m.GoCardlessMandateID, &m.GoCardlessSubscriptionID,
		&m.MemberSignature, &m.MemberSignedAt,
		&m.StaffUserID, &m.StaffSignature, &m.StaffSignedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding membership: %w", err)
	}
	return &m, nil
}

// SetGoCardlessMandateID stores the GoCardless mandate ID for a membership.
func (r *MembershipRepo) SetGoCardlessMandateID(ctx context.Context, membershipID, tenantID uuid.UUID, mandateID string) error {
	_, err := r.db.Pool.Exec(ctx,
		"UPDATE memberships SET gocardless_mandate_id = $3, updated_at = NOW() WHERE id = $1 AND tenant_id = $2",
		membershipID, tenantID, mandateID)
	return err
}

// ClearGoCardlessMandateByProviderID clears gocardless_mandate_id for all memberships with the given provider mandate ID (e.g. after mandate.cancelled webhook).
func (r *MembershipRepo) ClearGoCardlessMandateByProviderID(ctx context.Context, tenantID uuid.UUID, providerMandateID string) error {
	_, err := r.db.Pool.Exec(ctx,
		"UPDATE memberships SET gocardless_mandate_id = NULL, gocardless_subscription_id = NULL, updated_at = NOW() WHERE tenant_id = $1 AND gocardless_mandate_id = $2",
		tenantID, providerMandateID)
	return err
}

// SetGoCardlessSubscriptionID stores the GoCardless subscription ID for a membership.
func (r *MembershipRepo) SetGoCardlessSubscriptionID(ctx context.Context, membershipID, tenantID uuid.UUID, subscriptionID string) error {
	_, err := r.db.Pool.Exec(ctx,
		"UPDATE memberships SET gocardless_subscription_id = $3, updated_at = NOW() WHERE id = $1 AND tenant_id = $2",
		membershipID, tenantID, subscriptionID)
	return err
}

// UpdateStatus updates the status of a membership.
func (r *MembershipRepo) UpdateStatus(ctx context.Context, membershipID, tenantID uuid.UUID, status string) error {
	_, err := r.db.Pool.Exec(ctx,
		"UPDATE memberships SET status = $3, updated_at = NOW() WHERE id = $1 AND tenant_id = $2",
		membershipID, tenantID, status)
	return err
}

// UpdateContinueAfterTerm updates the continue_after_term toggle for a membership.
func (r *MembershipRepo) UpdateContinueAfterTerm(ctx context.Context, membershipID, tenantID uuid.UUID, continueAfterTerm bool) error {
	_, err := r.db.Pool.Exec(ctx,
		"UPDATE memberships SET continue_after_term = $3, updated_at = NOW() WHERE id = $1 AND tenant_id = $2",
		membershipID, tenantID, continueAfterTerm)
	return err
}

// FindBySubscriptionID finds a membership by its GoCardless subscription ID.
func (r *MembershipRepo) FindBySubscriptionID(ctx context.Context, tenantID uuid.UUID, subscriptionID string) (*models.Membership, error) {
	query := `
		SELECT id, tenant_id, client_id, membership_type_id, term_option_id,
		    start_date, min_term_end_date, payment_method, payment_frequency, currency_code, payment_interval_unit, payment_interval_count, payment_amount, num_payments, payments_made,
		    continue_after_term, status, gocardless_mandate_id, gocardless_subscription_id, member_signature_data, member_signed_at,
		    staff_user_id, staff_signature_data, staff_signed_at, created_at, updated_at
		FROM memberships WHERE tenant_id = $1 AND gocardless_subscription_id = $2`

	var m models.Membership
	err := r.db.Pool.QueryRow(ctx, query, tenantID, subscriptionID).Scan(
		&m.ID, &m.TenantID, &m.ClientID, &m.MembershipTypeID, &m.TermOptionID,
		&m.StartDate, &m.MinTermEndDate, &m.PaymentMethod, &m.PaymentFrequency, &m.CurrencyCode, &m.PaymentIntervalUnit, &m.PaymentIntervalCount,
		&m.PaymentAmount, &m.NumPayments, &m.PaymentsMade, &m.ContinueAfterTerm, &m.Status, &m.GoCardlessMandateID, &m.GoCardlessSubscriptionID,
		&m.MemberSignature, &m.MemberSignedAt,
		&m.StaffUserID, &m.StaffSignature, &m.StaffSignedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// FindByMandateID finds a membership by its GoCardless mandate ID.
func (r *MembershipRepo) FindByMandateID(ctx context.Context, tenantID uuid.UUID, mandateID string) (*models.Membership, error) {
	query := `
		SELECT id, tenant_id, client_id, membership_type_id, term_option_id,
		    start_date, min_term_end_date, payment_method, payment_frequency, currency_code, payment_interval_unit, payment_interval_count, payment_amount, num_payments, payments_made,
		    continue_after_term, status, gocardless_mandate_id, gocardless_subscription_id, member_signature_data, member_signed_at,
		    staff_user_id, staff_signature_data, staff_signed_at, created_at, updated_at
		FROM memberships WHERE tenant_id = $1 AND gocardless_mandate_id = $2`

	var m models.Membership
	err := r.db.Pool.QueryRow(ctx, query, tenantID, mandateID).Scan(
		&m.ID, &m.TenantID, &m.ClientID, &m.MembershipTypeID, &m.TermOptionID,
		&m.StartDate, &m.MinTermEndDate, &m.PaymentMethod, &m.PaymentFrequency, &m.CurrencyCode, &m.PaymentIntervalUnit, &m.PaymentIntervalCount,
		&m.PaymentAmount, &m.NumPayments, &m.PaymentsMade, &m.ContinueAfterTerm, &m.Status, &m.GoCardlessMandateID, &m.GoCardlessSubscriptionID,
		&m.MemberSignature, &m.MemberSignedAt,
		&m.StaffUserID, &m.StaffSignature, &m.StaffSignedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// ListByTenant returns all memberships for a tenant.
func (r *MembershipRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID, status string) ([]models.Membership, error) {
	query := `
		SELECT id, tenant_id, client_id, membership_type_id, term_option_id,
		    start_date, min_term_end_date, payment_method, payment_frequency, currency_code, payment_interval_unit, payment_interval_count, payment_amount, num_payments, payments_made,
		    continue_after_term, status, gocardless_mandate_id, gocardless_subscription_id, member_signature_data, member_signed_at,
		    staff_user_id, staff_signature_data, staff_signed_at, created_at, updated_at
		FROM memberships WHERE tenant_id = $1`
	args := []interface{}{tenantID}

	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing memberships: %w", err)
	}
	defer rows.Close()

	var memberships []models.Membership
	for rows.Next() {
		var m models.Membership
		if err := rows.Scan(
			&m.ID, &m.TenantID, &m.ClientID, &m.MembershipTypeID, &m.TermOptionID,
			&m.StartDate, &m.MinTermEndDate, &m.PaymentMethod, &m.PaymentFrequency, &m.CurrencyCode, &m.PaymentIntervalUnit, &m.PaymentIntervalCount,
			&m.PaymentAmount, &m.NumPayments, &m.PaymentsMade, &m.ContinueAfterTerm, &m.Status, &m.GoCardlessMandateID, &m.GoCardlessSubscriptionID,
			&m.MemberSignature, &m.MemberSignedAt,
			&m.StaffUserID, &m.StaffSignature, &m.StaffSignedAt, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		memberships = append(memberships, m)
	}
	return memberships, rows.Err()
}

// CountActive returns the count of active memberships for a tenant.
func (r *MembershipRepo) CountActive(ctx context.Context, tenantID uuid.UUID) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM memberships WHERE tenant_id=$1 AND status='active'",
		tenantID).Scan(&count)
	return count, err
}

// MembershipTypeApplySalesTax returns the tax toggle for a membership type.
// Defaults to true when not found, preserving historical behavior.
func (r *MembershipRepo) MembershipTypeApplySalesTax(ctx context.Context, membershipTypeID, tenantID uuid.UUID) (bool, error) {
	var applySalesTax bool
	err := r.db.Pool.QueryRow(ctx,
		"SELECT apply_sales_tax FROM membership_types WHERE id=$1 AND tenant_id=$2",
		membershipTypeID, tenantID).Scan(&applySalesTax)
	if err != nil {
		if err == pgx.ErrNoRows {
			return true, nil
		}
		return true, err
	}
	return applySalesTax, nil
}

// ReconcileLifecycleStatuses normalizes membership status from start_date + latest payment status.
// If tenantID is nil, it reconciles across all tenants.
func (r *MembershipRepo) ReconcileLifecycleStatuses(ctx context.Context, tenantID *uuid.UUID) (int64, error) {
	query := `
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
			  AND ($1::uuid IS NULL OR m.tenant_id = $1::uuid)
		)
		UPDATE memberships m
		SET status = t.target_status,
		    updated_at = NOW()
		FROM targets t
		WHERE m.id = t.id
		  AND m.tenant_id = t.tenant_id
		  AND m.status <> t.target_status
	`
	cmd, err := r.db.Pool.Exec(ctx, query, tenantID)
	if err != nil {
		return 0, fmt.Errorf("reconcile membership lifecycle statuses: %w", err)
	}
	return cmd.RowsAffected(), nil
}

// CountLifecycleStatusDrift counts memberships whose current status differs from canonical target status.
// If tenantID is nil, it counts drift across all tenants.
func (r *MembershipRepo) CountLifecycleStatusDrift(ctx context.Context, tenantID *uuid.UUID) (int64, error) {
	query := `
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
			  AND ($1::uuid IS NULL OR m.tenant_id = $1::uuid)
		)
		SELECT COUNT(*)
		FROM targets
		WHERE current_status <> target_status
	`
	var count int64
	if err := r.db.Pool.QueryRow(ctx, query, tenantID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count membership lifecycle drift: %w", err)
	}
	return count, nil
}

// ActivateScheduledDue marks scheduled memberships as active when start_date is due.
// If tenantID is nil, it applies across all tenants.
func (r *MembershipRepo) ActivateScheduledDue(ctx context.Context, tenantID *uuid.UUID) (int64, error) {
	cmd, err := r.db.Pool.Exec(ctx, `
		UPDATE memberships m
		SET status = 'active',
		    updated_at = NOW()
		FROM tenants t
		WHERE m.tenant_id = t.id
		  AND m.status = 'scheduled'
		  AND m.start_date <= ((NOW() AT TIME ZONE COALESCE(NULLIF(t.timezone, ''), 'UTC'))::date)
		  AND ($1::uuid IS NULL OR m.tenant_id = $1::uuid)
	`, tenantID)
	if err != nil {
		return 0, fmt.Errorf("activate scheduled due memberships: %w", err)
	}
	return cmd.RowsAffected(), nil
}
