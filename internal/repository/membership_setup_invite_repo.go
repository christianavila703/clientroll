package repository

import (
	"context"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type MembershipSetupInvite struct {
	ID                    uuid.UUID  `json:"id"`
	TenantID              uuid.UUID  `json:"tenant_id"`
	Token                 uuid.UUID  `json:"token"`
	ClientID              uuid.UUID  `json:"client_id"`
	MembershipTypeID      uuid.UUID  `json:"membership_type_id"`
	TermOptionID          uuid.UUID  `json:"term_option_id"`
	StartDate             time.Time  `json:"start_date"`
	PaymentFrequency      string     `json:"payment_frequency"`
	PaymentMethod         *string    `json:"payment_method,omitempty"`
	PaymentMethodStrategy string     `json:"payment_method_strategy"`
	PaymentMethodID       *string    `json:"payment_method_id,omitempty"`
	GoCardlessMandateID   *string    `json:"gocardless_mandate_id,omitempty"`
	ExpiresAt             time.Time  `json:"expires_at"`
	CompletedAt           *time.Time `json:"completed_at,omitempty"`
	CreatedByUserID       *uuid.UUID `json:"created_by_user_id,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type MembershipSetupInviteRepo struct {
	db *database.Postgres
}

func NewMembershipSetupInviteRepo(db *database.Postgres) *MembershipSetupInviteRepo {
	return &MembershipSetupInviteRepo{db: db}
}

func (r *MembershipSetupInviteRepo) Create(
	ctx context.Context,
	tenantID uuid.UUID,
	clientID uuid.UUID,
	membershipTypeID uuid.UUID,
	termOptionID uuid.UUID,
	startDate time.Time,
	paymentFrequency string,
	paymentMethod *string,
	paymentMethodStrategy string,
	paymentMethodID *string,
	gocardlessMandateID *string,
	expiresAt time.Time,
	createdByUserID *uuid.UUID,
) (*MembershipSetupInvite, error) {
	var out MembershipSetupInvite
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO membership_setup_invites (
			tenant_id, client_id, membership_type_id, term_option_id, start_date,
			payment_frequency, payment_method, payment_method_strategy, payment_method_id,
			gocardless_mandate_id, expires_at, created_by_user_id
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING
			id, tenant_id, token, client_id, membership_type_id, term_option_id, start_date,
			payment_frequency, payment_method, payment_method_strategy, payment_method_id,
			gocardless_mandate_id, expires_at, completed_at, created_by_user_id, created_at, updated_at
	`, tenantID, clientID, membershipTypeID, termOptionID, startDate, paymentFrequency, paymentMethod, paymentMethodStrategy, paymentMethodID, gocardlessMandateID, expiresAt, createdByUserID).Scan(
		&out.ID, &out.TenantID, &out.Token, &out.ClientID, &out.MembershipTypeID, &out.TermOptionID, &out.StartDate,
		&out.PaymentFrequency, &out.PaymentMethod, &out.PaymentMethodStrategy, &out.PaymentMethodID,
		&out.GoCardlessMandateID, &out.ExpiresAt, &out.CompletedAt, &out.CreatedByUserID, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *MembershipSetupInviteRepo) FindByToken(ctx context.Context, token uuid.UUID) (*MembershipSetupInvite, error) {
	var out MembershipSetupInvite
	err := r.db.Pool.QueryRow(ctx, `
		SELECT
			id, tenant_id, token, client_id, membership_type_id, term_option_id, start_date,
			payment_frequency, payment_method, payment_method_strategy, payment_method_id,
			gocardless_mandate_id, expires_at, completed_at, created_by_user_id, created_at, updated_at
		FROM membership_setup_invites
		WHERE token = $1
	`, token).Scan(
		&out.ID, &out.TenantID, &out.Token, &out.ClientID, &out.MembershipTypeID, &out.TermOptionID, &out.StartDate,
		&out.PaymentFrequency, &out.PaymentMethod, &out.PaymentMethodStrategy, &out.PaymentMethodID,
		&out.GoCardlessMandateID, &out.ExpiresAt, &out.CompletedAt, &out.CreatedByUserID, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

func (r *MembershipSetupInviteRepo) MarkCompleted(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE membership_setup_invites
		SET completed_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND completed_at IS NULL
	`, id)
	return err
}
