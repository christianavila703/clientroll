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
		    start_date, min_term_end_date, payment_method, payment_frequency, payment_amount,
		    status, member_signature_data, member_signed_at,
		    staff_user_id, staff_signature_data, staff_signed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'active', $10, $11, $12, $13, $14)
		RETURNING id, tenant_id, client_id, membership_type_id, term_option_id,
		    start_date, min_term_end_date, payment_method, payment_frequency, payment_amount,
		    status, member_signature_data, member_signed_at,
		    staff_user_id, staff_signature_data, staff_signed_at, created_at, updated_at`

	var m models.Membership
	err = r.db.Pool.QueryRow(ctx, query,
		tenantID, clientID, membershipTypeID, termOptionID,
		startDate, endDate, paymentMethod, paymentFrequency, paymentAmount,
		memberSignature, memberSignedAt, staffUserID, staffSignature, staffSignedAt,
	).Scan(
		&m.ID, &m.TenantID, &m.ClientID, &m.MembershipTypeID, &m.TermOptionID,
		&m.StartDate, &m.MinTermEndDate, &m.PaymentMethod, &m.PaymentFrequency,
		&m.PaymentAmount, &m.Status, &m.MemberSignature, &m.MemberSignedAt,
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
		    start_date, min_term_end_date, payment_method, payment_frequency, payment_amount,
		    status, member_signature_data, member_signed_at,
		    staff_user_id, staff_signature_data, staff_signed_at, created_at, updated_at
		FROM memberships WHERE id = $1 AND tenant_id = $2`

	var m models.Membership
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID).Scan(
		&m.ID, &m.TenantID, &m.ClientID, &m.MembershipTypeID, &m.TermOptionID,
		&m.StartDate, &m.MinTermEndDate, &m.PaymentMethod, &m.PaymentFrequency,
		&m.PaymentAmount, &m.Status, &m.MemberSignature, &m.MemberSignedAt,
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

// ListByTenant returns all memberships for a tenant.
func (r *MembershipRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID, status string) ([]models.Membership, error) {
	query := `
		SELECT id, tenant_id, client_id, membership_type_id, term_option_id,
		    start_date, min_term_end_date, payment_method, payment_frequency, payment_amount,
		    status, member_signature_data, member_signed_at,
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
			&m.StartDate, &m.MinTermEndDate, &m.PaymentMethod, &m.PaymentFrequency,
			&m.PaymentAmount, &m.Status, &m.MemberSignature, &m.MemberSignedAt,
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
