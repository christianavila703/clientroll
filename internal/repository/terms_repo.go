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

// TermsRepo handles database operations for terms and conditions.
type TermsRepo struct {
	db *database.Postgres
}

// NewTermsRepo creates a new terms repository.
func NewTermsRepo(db *database.Postgres) *TermsRepo {
	return &TermsRepo{db: db}
}

// List returns all T&C documents for a tenant. activeOnly filters out
// documents marked is_active=false so admin paths can opt in to either view.
func (r *TermsRepo) List(ctx context.Context, tenantID uuid.UUID, activeOnly bool) ([]models.TermsAndConditions, error) {
	query := `
		SELECT id, tenant_id, title, content, version, is_active, created_at, updated_at
		FROM terms_and_conditions WHERE tenant_id = $1`
	if activeOnly {
		query += ` AND is_active = true`
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.db.Pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listing terms: %w", err)
	}
	defer rows.Close()

	var items []models.TermsAndConditions
	for rows.Next() {
		var t models.TermsAndConditions
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Title, &t.Content,
			&t.Version, &t.IsActive, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning terms: %w", err)
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// FindByID returns a T&C document by ID.
func (r *TermsRepo) FindByID(ctx context.Context, id, tenantID uuid.UUID) (*models.TermsAndConditions, error) {
	query := `
		SELECT id, tenant_id, title, content, version, is_active, created_at, updated_at
		FROM terms_and_conditions WHERE id = $1 AND tenant_id = $2`

	var t models.TermsAndConditions
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID).Scan(
		&t.ID, &t.TenantID, &t.Title, &t.Content,
		&t.Version, &t.IsActive, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding terms: %w", err)
	}
	return &t, nil
}

// Create inserts a new T&C document.
func (r *TermsRepo) Create(ctx context.Context, tenantID uuid.UUID, title, content string) (*models.TermsAndConditions, error) {
	query := `
		INSERT INTO terms_and_conditions (tenant_id, title, content)
		VALUES ($1, $2, $3)
		RETURNING id, tenant_id, title, content, version, is_active, created_at, updated_at`

	var t models.TermsAndConditions
	err := r.db.Pool.QueryRow(ctx, query, tenantID, title, content).Scan(
		&t.ID, &t.TenantID, &t.Title, &t.Content,
		&t.Version, &t.IsActive, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating terms: %w", err)
	}
	// Snapshot v1 into history. Best-effort: a failure here should not
	// roll back the live row, but it does deserve a server log so an admin
	// can replay the snapshot later if needed.
	r.snapshotVersion(ctx, &t, nil)
	return &t, nil
}

// Update updates a T&C document (creates a new version) and writes a history
// entry capturing the new title/content/is_active. changeNote is optional.
func (r *TermsRepo) Update(ctx context.Context, id, tenantID uuid.UUID, title, content string, isActive bool, changeNote *string) (*models.TermsAndConditions, error) {
	query := `
		UPDATE terms_and_conditions SET title=$3, content=$4, is_active=$5,
		       version=version+1, updated_at=NOW()
		WHERE id=$1 AND tenant_id=$2
		RETURNING id, tenant_id, title, content, version, is_active, created_at, updated_at`

	var t models.TermsAndConditions
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID, title, content, isActive).Scan(
		&t.ID, &t.TenantID, &t.Title, &t.Content,
		&t.Version, &t.IsActive, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating terms: %w", err)
	}
	r.snapshotVersion(ctx, &t, changeNote)
	return &t, nil
}

// snapshotVersion writes one row to terms_and_conditions_versions describing
// the document's current state. Run after the live row has been UPDATEd /
// INSERTed so version + updated_at are correct. Errors are swallowed so an
// audit-table failure cannot cascade into a 500 on the live save.
func (r *TermsRepo) snapshotVersion(ctx context.Context, t *models.TermsAndConditions, changeNote *string) {
	if t == nil {
		return
	}
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO terms_and_conditions_versions (
			tenant_id, terms_id, version, title, content, is_active, change_note, effective_from
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (terms_id, version) DO NOTHING`,
		t.TenantID, t.ID, t.Version, t.Title, t.Content, t.IsActive, changeNote, t.UpdatedAt)
	if err != nil {
		// snapshotVersion intentionally non-fatal; logged only.
		fmt.Printf("terms_repo.snapshotVersion: terms=%s tenant=%s err=%v\n", t.ID, t.TenantID, err)
	}
}

// VersionRow is the projection of one terms_and_conditions_versions row used
// by the history list endpoint.
type VersionRow struct {
	ID            uuid.UUID `json:"id"`
	Version       int       `json:"version"`
	Title         string    `json:"title"`
	Content       string    `json:"content"`
	IsActive      bool      `json:"is_active"`
	ChangeNote    *string   `json:"change_note,omitempty"`
	EffectiveFrom time.Time `json:"effective_from"`
}

// FlagReacceptanceRequired marks every active membership that has already
// accepted termsID as needing to re-accept the current live version. Called
// when an admin saves a T&C with requires_reacceptance=true. Returns the
// number of memberships flagged (so the UI can show "X members will see a
// re-acceptance prompt"). Triggered rows are idempotent: re-flagging the
// same membership updates the required_version and clears acknowledged_at.
func (r *TermsRepo) FlagReacceptanceRequired(ctx context.Context, tenantID, termsID uuid.UUID, version int, triggeredBy *uuid.UUID) (int64, error) {
	tag, err := r.db.Pool.Exec(ctx, `
		INSERT INTO membership_reacceptance_required
			(tenant_id, membership_id, terms_id, required_version, triggered_by_user_id, triggered_at, acknowledged_at)
		SELECT mat.tenant_id, mat.membership_id, mat.terms_id, $3, $4, NOW(), NULL
		FROM membership_accepted_terms mat
		JOIN memberships m ON m.id = mat.membership_id
		WHERE mat.tenant_id = $1 AND mat.terms_id = $2
		  AND m.status IN ('active', 'paused')
		ON CONFLICT (membership_id, terms_id) DO UPDATE
			SET required_version = EXCLUDED.required_version,
			    triggered_by_user_id = EXCLUDED.triggered_by_user_id,
			    triggered_at = EXCLUDED.triggered_at,
			    acknowledged_at = NULL`,
		tenantID, termsID, version, triggeredBy)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// PendingReacceptance is one outstanding re-acceptance requirement for a member.
type PendingReacceptance struct {
	TermsID         uuid.UUID `json:"terms_id"`
	Title           string    `json:"title"`
	RequiredVersion int       `json:"required_version"`
	Content         string    `json:"content"`
	TriggeredAt     time.Time `json:"triggered_at"`
}

// PendingReacceptanceForMembership returns the un-acknowledged re-acceptance
// items for a single membership. Empty result == nothing to do.
func (r *TermsRepo) PendingReacceptanceForMembership(ctx context.Context, membershipID, tenantID uuid.UUID) ([]PendingReacceptance, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT mrr.terms_id, t.title, mrr.required_version, t.content, mrr.triggered_at
		FROM membership_reacceptance_required mrr
		JOIN terms_and_conditions t ON t.id = mrr.terms_id
		WHERE mrr.membership_id = $1 AND mrr.tenant_id = $2 AND mrr.acknowledged_at IS NULL
		ORDER BY mrr.triggered_at ASC`,
		membershipID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PendingReacceptance{}
	for rows.Next() {
		var p PendingReacceptance
		if err := rows.Scan(&p.TermsID, &p.Title, &p.RequiredVersion, &p.Content, &p.TriggeredAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AcknowledgeReacceptance marks one re-acceptance row as accepted by the
// member. Idempotent: calling twice keeps the first acknowledgment timestamp.
func (r *TermsRepo) AcknowledgeReacceptance(ctx context.Context, membershipID, tenantID, termsID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE membership_reacceptance_required
		SET acknowledged_at = COALESCE(acknowledged_at, NOW())
		WHERE membership_id = $1 AND tenant_id = $2 AND terms_id = $3`,
		membershipID, tenantID, termsID)
	return err
}

// ListVersions returns every captured version of a T&C document, newest first.
func (r *TermsRepo) ListVersions(ctx context.Context, termsID, tenantID uuid.UUID) ([]VersionRow, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, version, title, content, is_active, change_note, effective_from
		FROM terms_and_conditions_versions
		WHERE terms_id = $1 AND tenant_id = $2
		ORDER BY version DESC`,
		termsID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []VersionRow{}
	for rows.Next() {
		var v VersionRow
		if err := rows.Scan(&v.ID, &v.Version, &v.Title, &v.Content, &v.IsActive, &v.ChangeNote, &v.EffectiveFrom); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Delete removes a T&C document.
func (r *TermsRepo) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx,
		"DELETE FROM terms_and_conditions WHERE id = $1 AND tenant_id = $2",
		id, tenantID,
	)
	return err
}

// AttachToMembershipType links a T&C to a membership type. It also fans the
// attachment out to every term option under that membership type via
// term_option_terms so per-term reads stay coherent during the soak window.
func (r *TermsRepo) AttachToMembershipType(ctx context.Context, tenantID, membershipTypeID, termsID uuid.UUID) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO membership_type_terms (tenant_id, membership_type_id, terms_and_conditions_id)
		VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, tenantID, membershipTypeID, termsID); err != nil {
		return fmt.Errorf("attach mt terms: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO term_option_terms (tenant_id, term_option_id, terms_and_conditions_id, is_required, sort_order)
		SELECT $1, mto.id, $3, TRUE, 0
		FROM   membership_term_options mto
		WHERE  mto.membership_type_id = $2
		  AND  mto.tenant_id = $1
		  AND  mto.archived_at IS NULL
		ON CONFLICT (term_option_id, terms_and_conditions_id) DO NOTHING
	`, tenantID, membershipTypeID, termsID); err != nil {
		return fmt.Errorf("attach term option terms: %w", err)
	}
	return tx.Commit(ctx)
}

// DetachFromMembershipType removes a T&C link from a membership type and the
// corresponding fan-out rows in term_option_terms.
func (r *TermsRepo) DetachFromMembershipType(ctx context.Context, tenantID, membershipTypeID, termsID uuid.UUID) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		DELETE FROM membership_type_terms
		WHERE tenant_id=$1 AND membership_type_id=$2 AND terms_and_conditions_id=$3
	`, tenantID, membershipTypeID, termsID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM term_option_terms tot
		USING membership_term_options mto
		WHERE tot.term_option_id = mto.id
		  AND mto.membership_type_id = $2
		  AND tot.tenant_id = $1
		  AND tot.terms_and_conditions_id = $3
	`, tenantID, membershipTypeID, termsID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReplaceForTermOption sets the full set of (TC, required, sort_order) tuples
// attached to a single term option. Atomic: clears then re-inserts.
type AttachedTermInput struct {
	TermsID    uuid.UUID `json:"terms_id"`
	IsRequired bool      `json:"is_required"`
	SortOrder  int       `json:"sort_order"`
}

func (r *TermsRepo) ReplaceForTermOption(ctx context.Context, tenantID, termOptionID uuid.UUID, inputs []AttachedTermInput) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx,
		`DELETE FROM term_option_terms WHERE term_option_id=$1 AND tenant_id=$2`,
		termOptionID, tenantID); err != nil {
		return err
	}
	for _, in := range inputs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO term_option_terms
				(tenant_id, term_option_id, terms_and_conditions_id, is_required, sort_order)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (term_option_id, terms_and_conditions_id) DO UPDATE
			   SET is_required = EXCLUDED.is_required, sort_order = EXCLUDED.sort_order
		`, tenantID, termOptionID, in.TermsID, in.IsRequired, in.SortOrder); err != nil {
			return fmt.Errorf("insert attached term: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// AttachedTermsForTermOption returns the active T&Cs attached to a single term
// option, ordered by sort_order then title. Inactive T&Cs are dropped so the
// signup view and the eventual PDF agree on what the member is signing.
func (r *TermsRepo) AttachedTermsForTermOption(ctx context.Context, termOptionID, tenantID uuid.UUID) ([]models.AttachedTerms, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT t.id, t.tenant_id, t.title, t.content, t.version, t.is_active,
		       t.created_at, t.updated_at, tot.is_required, tot.sort_order
		FROM   term_option_terms tot
		JOIN   terms_and_conditions t ON t.id = tot.terms_and_conditions_id
		WHERE  tot.term_option_id = $1 AND tot.tenant_id = $2
		  AND  t.is_active = true
		ORDER BY tot.sort_order, t.title
	`, termOptionID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAttachedTerms(rows)
}

// AttachedTermsForTenant batch-loads attached T&Cs for every term option in a
// tenant, keyed by term_option_id. Used by membership_type_repo's enrichment
// pass when returning a list of MembershipTypes.
func (r *TermsRepo) AttachedTermsForTenant(ctx context.Context, tenantID uuid.UUID) (map[uuid.UUID][]models.AttachedTerms, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT t.id, t.tenant_id, t.title, t.content, t.version, t.is_active,
		       t.created_at, t.updated_at, tot.is_required, tot.sort_order,
		       tot.term_option_id
		FROM   term_option_terms tot
		JOIN   terms_and_conditions t ON t.id = tot.terms_and_conditions_id
		WHERE  tot.tenant_id = $1 AND t.is_active = true
		ORDER BY tot.term_option_id, tot.sort_order, t.title
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[uuid.UUID][]models.AttachedTerms)
	for rows.Next() {
		var (
			at        models.AttachedTerms
			termOptID uuid.UUID
		)
		if err := rows.Scan(&at.ID, &at.TenantID, &at.Title, &at.Content,
			&at.Version, &at.IsActive, &at.CreatedAt, &at.UpdatedAt,
			&at.IsRequired, &at.AttachSort, &termOptID); err != nil {
			return nil, err
		}
		out[termOptID] = append(out[termOptID], at)
	}
	return out, rows.Err()
}

func scanAttachedTerms(rows pgx.Rows) ([]models.AttachedTerms, error) {
	var out []models.AttachedTerms
	for rows.Next() {
		var at models.AttachedTerms
		if err := rows.Scan(&at.ID, &at.TenantID, &at.Title, &at.Content,
			&at.Version, &at.IsActive, &at.CreatedAt, &at.UpdatedAt,
			&at.IsRequired, &at.AttachSort); err != nil {
			return nil, err
		}
		out = append(out, at)
	}
	return out, rows.Err()
}

// GetForMembershipType returns T&Cs attached to a membership type.
// activeOnly=true filters out documents marked is_active=false. Public signup
// paths and any path that writes a binding agreement should pass true so retired
// documents never make it into a new signing flow; admin/listing paths pass
// false so attached-but-inactive docs are still visible for cleanup.
func (r *TermsRepo) GetForMembershipType(ctx context.Context, membershipTypeID, tenantID uuid.UUID, activeOnly bool) ([]models.TermsAndConditions, error) {
	query := `
		SELECT t.id, t.tenant_id, t.title, t.content, t.version, t.is_active,
		       t.created_at, t.updated_at
		FROM terms_and_conditions t
		JOIN membership_type_terms mtt ON mtt.terms_and_conditions_id = t.id
		WHERE mtt.membership_type_id = $1 AND mtt.tenant_id = $2`
	if activeOnly {
		query += ` AND t.is_active = true`
	}
	query += ` ORDER BY t.title`

	rows, err := r.db.Pool.Query(ctx, query, membershipTypeID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.TermsAndConditions
	for rows.Next() {
		var t models.TermsAndConditions
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Title, &t.Content,
			&t.Version, &t.IsActive, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}
