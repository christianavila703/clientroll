package repository

import (
	"context"
	"fmt"

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

// List returns all T&C documents for a tenant.
func (r *TermsRepo) List(ctx context.Context, tenantID uuid.UUID) ([]models.TermsAndConditions, error) {
	query := `
		SELECT id, tenant_id, title, content, version, is_active, created_at, updated_at
		FROM terms_and_conditions WHERE tenant_id = $1
		ORDER BY created_at DESC`

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
	return &t, nil
}

// Update updates a T&C document (creates a new version).
func (r *TermsRepo) Update(ctx context.Context, id, tenantID uuid.UUID, title, content string, isActive bool) (*models.TermsAndConditions, error) {
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
	return &t, nil
}

// Delete removes a T&C document.
func (r *TermsRepo) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx,
		"DELETE FROM terms_and_conditions WHERE id = $1 AND tenant_id = $2",
		id, tenantID,
	)
	return err
}

// AttachToMembershipType links a T&C to a membership type.
func (r *TermsRepo) AttachToMembershipType(ctx context.Context, tenantID, membershipTypeID, termsID uuid.UUID) error {
	query := `
		INSERT INTO membership_type_terms (tenant_id, membership_type_id, terms_and_conditions_id)
		VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	_, err := r.db.Pool.Exec(ctx, query, tenantID, membershipTypeID, termsID)
	return err
}

// DetachFromMembershipType removes a T&C link from a membership type.
func (r *TermsRepo) DetachFromMembershipType(ctx context.Context, tenantID, membershipTypeID, termsID uuid.UUID) error {
	query := `
		DELETE FROM membership_type_terms
		WHERE tenant_id=$1 AND membership_type_id=$2 AND terms_and_conditions_id=$3`
	_, err := r.db.Pool.Exec(ctx, query, tenantID, membershipTypeID, termsID)
	return err
}

// GetForMembershipType returns T&Cs attached to a membership type.
func (r *TermsRepo) GetForMembershipType(ctx context.Context, membershipTypeID, tenantID uuid.UUID) ([]models.TermsAndConditions, error) {
	query := `
		SELECT t.id, t.tenant_id, t.title, t.content, t.version, t.is_active,
		       t.created_at, t.updated_at
		FROM terms_and_conditions t
		JOIN membership_type_terms mtt ON mtt.terms_and_conditions_id = t.id
		WHERE mtt.membership_type_id = $1 AND mtt.tenant_id = $2
		ORDER BY t.title`

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
