package repository

import (
	"context"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// TermsReacceptanceInvite is a single tokenised link a member follows to
// re-accept an updated T&C document. The token field is the public secret
// that authorises the bearer to view + sign.
type TermsReacceptanceInvite struct {
	ID                  uuid.UUID  `json:"id"`
	TenantID            uuid.UUID  `json:"tenant_id"`
	Token               uuid.UUID  `json:"token"`
	MembershipID        uuid.UUID  `json:"membership_id"`
	TermsID             uuid.UUID  `json:"terms_id"`
	TermsVersion        int        `json:"terms_version"`
	ExpiresAt           time.Time  `json:"expires_at"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
	SignatureTyped      *string    `json:"signature_typed,omitempty"`
	SignatureImageData  *string    `json:"signature_image_data,omitempty"`
	SignatureIP         *string    `json:"signature_ip,omitempty"`
	SignatureUserAgent  *string    `json:"signature_user_agent,omitempty"`
	TriggeredByUserID   *uuid.UUID `json:"triggered_by_user_id,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// ReacceptanceInviteDetail is the rich payload returned to the public page —
// it carries everything the React component needs to render the page without
// further round-trips (member name, org branding, T&C content, etc).
type ReacceptanceInviteDetail struct {
	Invite           TermsReacceptanceInvite `json:"invite"`
	TenantSlug       string                  `json:"tenant_slug"`
	OrgName          string                  `json:"org_name"`
	OrgLogoURL       *string                 `json:"org_logo_url,omitempty"`
	ClientFirstName  string                  `json:"client_first_name"`
	ClientLastName   string                  `json:"client_last_name"`
	ClientEmail      string                  `json:"client_email"`
	ClientID         uuid.UUID               `json:"client_id"`
	MembershipTitle  string                  `json:"membership_title"`
	TermsID          uuid.UUID               `json:"terms_id"`
	TermsTitle       string                  `json:"terms_title"`
	TermsContent     string                  `json:"terms_content"`
	TermsChangeNote  *string                 `json:"terms_change_note,omitempty"`
	TermsEffectiveAt *time.Time              `json:"terms_effective_at,omitempty"`
	PreviousVersion  *int                    `json:"previous_version,omitempty"`
}

type TermsReacceptanceInviteRepo struct {
	db *database.Postgres
}

func NewTermsReacceptanceInviteRepo(db *database.Postgres) *TermsReacceptanceInviteRepo {
	return &TermsReacceptanceInviteRepo{db: db}
}

// Issue creates (or rotates) an invite for the given membership + terms pair.
// Re-flagging the same T&C overwrites the previous unused invite — we always
// want the member following the most recent link to land on the live version.
func (r *TermsReacceptanceInviteRepo) Issue(
	ctx context.Context,
	tenantID, membershipID, termsID uuid.UUID,
	termsVersion int,
	triggeredBy *uuid.UUID,
	ttl time.Duration,
) (*TermsReacceptanceInvite, error) {
	expiresAt := time.Now().UTC().Add(ttl)
	var out TermsReacceptanceInvite
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO terms_reacceptance_invites
			(tenant_id, membership_id, terms_id, terms_version, expires_at, triggered_by_user_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (membership_id, terms_id) DO UPDATE
			SET token = gen_random_uuid(),
			    terms_version = EXCLUDED.terms_version,
			    expires_at = EXCLUDED.expires_at,
			    completed_at = NULL,
			    signature_typed = NULL,
			    signature_image_data = NULL,
			    signature_ip = NULL,
			    signature_user_agent = NULL,
			    triggered_by_user_id = EXCLUDED.triggered_by_user_id,
			    updated_at = NOW()
		RETURNING id, tenant_id, token, membership_id, terms_id, terms_version,
		          expires_at, completed_at, signature_typed, signature_image_data,
		          signature_ip, signature_user_agent, triggered_by_user_id,
		          created_at, updated_at`,
		tenantID, membershipID, termsID, termsVersion, expiresAt, triggeredBy,
	).Scan(
		&out.ID, &out.TenantID, &out.Token, &out.MembershipID, &out.TermsID, &out.TermsVersion,
		&out.ExpiresAt, &out.CompletedAt, &out.SignatureTyped, &out.SignatureImageData,
		&out.SignatureIP, &out.SignatureUserAgent, &out.TriggeredByUserID,
		&out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// IssueForFlaggedMemberships looks at every membership that was just flagged
// for re-acceptance of termsID at termsVersion and creates / rotates an
// invite for each. Returns the count of invites issued. Idempotent.
func (r *TermsReacceptanceInviteRepo) IssueForFlaggedMemberships(
	ctx context.Context,
	tenantID, termsID uuid.UUID,
	termsVersion int,
	triggeredBy *uuid.UUID,
	ttl time.Duration,
) (int, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT membership_id
		FROM membership_reacceptance_required
		WHERE tenant_id = $1 AND terms_id = $2 AND acknowledged_at IS NULL`,
		tenantID, termsID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var membershipIDs []uuid.UUID
	for rows.Next() {
		var mid uuid.UUID
		if err := rows.Scan(&mid); err != nil {
			return 0, err
		}
		membershipIDs = append(membershipIDs, mid)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	issued := 0
	for _, mid := range membershipIDs {
		if _, err := r.Issue(ctx, tenantID, mid, termsID, termsVersion, triggeredBy, ttl); err != nil {
			return issued, err
		}
		issued++
	}
	return issued, nil
}

// FindDetailByToken returns the full rich payload for the public page —
// member name, org branding, latest T&C content. The bearer of the token is
// trusted to be the right member (tokens are UUIDs and only sent via secure
// channels). RLS is set by the caller before invoking.
func (r *TermsReacceptanceInviteRepo) FindDetailByToken(ctx context.Context, token uuid.UUID) (*ReacceptanceInviteDetail, error) {
	var d ReacceptanceInviteDetail
	err := r.db.Pool.QueryRow(ctx, `
		SELECT
			i.id, i.tenant_id, i.token, i.membership_id, i.terms_id, i.terms_version,
			i.expires_at, i.completed_at, i.signature_typed, i.signature_image_data,
			i.signature_ip, i.signature_user_agent, i.triggered_by_user_id,
			i.created_at, i.updated_at,
			ten.slug, ten.name, ten.logo_url,
			c.first_name, c.last_name, c.email, c.id,
			mt.title,
			t.title, t.content, v.change_note, v.effective_from,
			(SELECT MAX(version) FROM terms_and_conditions_versions
			 WHERE terms_id = t.id AND tenant_id = i.tenant_id AND version < i.terms_version)
		FROM terms_reacceptance_invites i
		JOIN tenants ten ON ten.id = i.tenant_id
		JOIN memberships m ON m.id = i.membership_id
		JOIN clients c ON c.id = m.client_id
		JOIN membership_types mt ON mt.id = m.membership_type_id
		JOIN terms_and_conditions t ON t.id = i.terms_id
		LEFT JOIN terms_and_conditions_versions v
		       ON v.terms_id = i.terms_id AND v.tenant_id = i.tenant_id AND v.version = i.terms_version
		WHERE i.token = $1`,
		token,
	).Scan(
		&d.Invite.ID, &d.Invite.TenantID, &d.Invite.Token, &d.Invite.MembershipID, &d.Invite.TermsID, &d.Invite.TermsVersion,
		&d.Invite.ExpiresAt, &d.Invite.CompletedAt, &d.Invite.SignatureTyped, &d.Invite.SignatureImageData,
		&d.Invite.SignatureIP, &d.Invite.SignatureUserAgent, &d.Invite.TriggeredByUserID,
		&d.Invite.CreatedAt, &d.Invite.UpdatedAt,
		&d.TenantSlug, &d.OrgName, &d.OrgLogoURL,
		&d.ClientFirstName, &d.ClientLastName, &d.ClientEmail, &d.ClientID,
		&d.MembershipTitle,
		&d.TermsTitle, &d.TermsContent, &d.TermsChangeNote, &d.TermsEffectiveAt,
		&d.PreviousVersion,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	d.TermsID = d.Invite.TermsID
	return &d, nil
}

// MarkCompleted records the member's signature and stamps completed_at.
// Idempotent: re-submitting the same token is rejected upstream (the GET
// returns 410 Gone for a completed invite).
func (r *TermsReacceptanceInviteRepo) MarkCompleted(
	ctx context.Context,
	id uuid.UUID,
	signatureTyped *string,
	signatureImageData *string,
	ip *string,
	userAgent *string,
) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE terms_reacceptance_invites
		SET completed_at = NOW(),
		    signature_typed = $2,
		    signature_image_data = $3,
		    signature_ip = $4,
		    signature_user_agent = $5,
		    updated_at = NOW()
		WHERE id = $1 AND completed_at IS NULL`,
		id, signatureTyped, signatureImageData, ip, userAgent)
	return err
}

// StaffLinkRow is a flattened invite + status row for the staff
// "re-acceptance links" panel.
type StaffLinkRow struct {
	ID           uuid.UUID  `json:"id"`
	Token        uuid.UUID  `json:"token"`
	TermsID      uuid.UUID  `json:"terms_id"`
	TermsTitle   string     `json:"terms_title"`
	TermsVersion int        `json:"terms_version"`
	ExpiresAt    time.Time  `json:"expires_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// ListByMembership returns the invite links visible to staff for one
// membership. Includes both pending and completed rows so the admin can see
// when the member actually signed.
func (r *TermsReacceptanceInviteRepo) ListByMembership(ctx context.Context, membershipID, tenantID uuid.UUID) ([]StaffLinkRow, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT i.id, i.token, i.terms_id, t.title, i.terms_version,
		       i.expires_at, i.completed_at, i.created_at
		FROM terms_reacceptance_invites i
		JOIN terms_and_conditions t ON t.id = i.terms_id
		WHERE i.membership_id = $1 AND i.tenant_id = $2
		ORDER BY i.created_at DESC`,
		membershipID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StaffLinkRow{}
	for rows.Next() {
		var row StaffLinkRow
		if err := rows.Scan(&row.ID, &row.Token, &row.TermsID, &row.TermsTitle, &row.TermsVersion,
			&row.ExpiresAt, &row.CompletedAt, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
