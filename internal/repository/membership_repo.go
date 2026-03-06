package repository

import (
	"context"
	"fmt"
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

// Create inserts a new membership from a signup submission.
func (r *MembershipRepo) Create(
	ctx context.Context,
	tenantID, clientID, membershipTypeID, termOptionID uuid.UUID,
	startDate time.Time,
	paymentMethod, paymentFrequency string,
	paymentAmount float64,
	paymentIntervalUnit *string,
	paymentIntervalCount int,
	numPayments *int,
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

	query := `
		INSERT INTO memberships (tenant_id, client_id, membership_type_id, term_option_id,
		    start_date, min_term_end_date, payment_method, payment_frequency, payment_interval_unit, payment_interval_count, payment_amount, num_payments,
		    status, member_signature_data, member_signed_at,
		    staff_user_id, staff_signature_data, staff_signed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'active', $12, $13, $14, $15, $16)
		RETURNING id, tenant_id, client_id, membership_type_id, term_option_id,
		    start_date, min_term_end_date, payment_method, payment_frequency, payment_interval_unit, payment_interval_count, payment_amount, num_payments, payments_made,
		    status, gocardless_mandate_id, member_signature_data, member_signed_at,
		    staff_user_id, staff_signature_data, staff_signed_at, created_at, updated_at`

	var m models.Membership
	err = r.db.Pool.QueryRow(ctx, query,
		tenantID, clientID, membershipTypeID, termOptionID,
		startDate, endDate, paymentMethod, paymentFrequency, paymentIntervalUnit, paymentIntervalCount, paymentAmount, numPayments,
		memberSignature, memberSignedAt, staffUserID, staffSignature, staffSignedAt,
	).Scan(
		&m.ID, &m.TenantID, &m.ClientID, &m.MembershipTypeID, &m.TermOptionID,
		&m.StartDate, &m.MinTermEndDate, &m.PaymentMethod, &m.PaymentFrequency, &m.PaymentIntervalUnit, &m.PaymentIntervalCount,
		&m.PaymentAmount, &m.NumPayments, &m.PaymentsMade, &m.Status, &m.GoCardlessMandateID, &m.MemberSignature, &m.MemberSignedAt,
		&m.StaffUserID, &m.StaffSignature, &m.StaffSignedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating membership: %w", err)
	}
	return &m, nil
}

// FindByID returns a membership by ID.
func (r *MembershipRepo) FindByID(ctx context.Context, id, tenantID uuid.UUID) (*models.Membership, error) {
	query := `
		SELECT id, tenant_id, client_id, membership_type_id, term_option_id,
		    start_date, min_term_end_date, payment_method, payment_frequency, payment_interval_unit, payment_interval_count, payment_amount, num_payments, payments_made,
		    status, gocardless_mandate_id, gocardless_subscription_id, member_signature_data, member_signed_at,
		    staff_user_id, staff_signature_data, staff_signed_at, created_at, updated_at
		FROM memberships WHERE id = $1 AND tenant_id = $2`

	var m models.Membership
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID).Scan(
		&m.ID, &m.TenantID, &m.ClientID, &m.MembershipTypeID, &m.TermOptionID,
		&m.StartDate, &m.MinTermEndDate, &m.PaymentMethod, &m.PaymentFrequency, &m.PaymentIntervalUnit, &m.PaymentIntervalCount,
		&m.PaymentAmount, &m.NumPayments, &m.PaymentsMade, &m.Status, &m.GoCardlessMandateID, &m.GoCardlessSubscriptionID,
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

// FindBySubscriptionID finds a membership by its GoCardless subscription ID.
func (r *MembershipRepo) FindBySubscriptionID(ctx context.Context, tenantID uuid.UUID, subscriptionID string) (*models.Membership, error) {
	query := `
		SELECT id, tenant_id, client_id, membership_type_id, term_option_id,
		    start_date, min_term_end_date, payment_method, payment_frequency, payment_interval_unit, payment_interval_count, payment_amount, num_payments, payments_made,
		    status, gocardless_mandate_id, gocardless_subscription_id, member_signature_data, member_signed_at,
		    staff_user_id, staff_signature_data, staff_signed_at, created_at, updated_at
		FROM memberships WHERE tenant_id = $1 AND gocardless_subscription_id = $2`

	var m models.Membership
	err := r.db.Pool.QueryRow(ctx, query, tenantID, subscriptionID).Scan(
		&m.ID, &m.TenantID, &m.ClientID, &m.MembershipTypeID, &m.TermOptionID,
		&m.StartDate, &m.MinTermEndDate, &m.PaymentMethod, &m.PaymentFrequency, &m.PaymentIntervalUnit, &m.PaymentIntervalCount,
		&m.PaymentAmount, &m.NumPayments, &m.PaymentsMade, &m.Status, &m.GoCardlessMandateID, &m.GoCardlessSubscriptionID,
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
		    start_date, min_term_end_date, payment_method, payment_frequency, payment_interval_unit, payment_interval_count, payment_amount, num_payments, payments_made,
		    status, gocardless_mandate_id, gocardless_subscription_id, member_signature_data, member_signed_at,
		    staff_user_id, staff_signature_data, staff_signed_at, created_at, updated_at
		FROM memberships WHERE tenant_id = $1 AND gocardless_mandate_id = $2`

	var m models.Membership
	err := r.db.Pool.QueryRow(ctx, query, tenantID, mandateID).Scan(
		&m.ID, &m.TenantID, &m.ClientID, &m.MembershipTypeID, &m.TermOptionID,
		&m.StartDate, &m.MinTermEndDate, &m.PaymentMethod, &m.PaymentFrequency, &m.PaymentIntervalUnit, &m.PaymentIntervalCount,
		&m.PaymentAmount, &m.NumPayments, &m.PaymentsMade, &m.Status, &m.GoCardlessMandateID, &m.GoCardlessSubscriptionID,
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
		    start_date, min_term_end_date, payment_method, payment_frequency, payment_interval_unit, payment_interval_count, payment_amount, num_payments, payments_made,
		    status, gocardless_mandate_id, gocardless_subscription_id, member_signature_data, member_signed_at,
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
			&m.StartDate, &m.MinTermEndDate, &m.PaymentMethod, &m.PaymentFrequency, &m.PaymentIntervalUnit, &m.PaymentIntervalCount,
			&m.PaymentAmount, &m.NumPayments, &m.PaymentsMade, &m.Status, &m.GoCardlessMandateID, &m.GoCardlessSubscriptionID,
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
