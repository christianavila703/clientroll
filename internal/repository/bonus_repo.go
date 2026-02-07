package repository

import (
	"context"
	"fmt"

	"clientroll/internal/database"
	"clientroll/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// BonusRepo handles database operations for bonuses.
type BonusRepo struct {
	db *database.Postgres
}

// NewBonusRepo creates a new bonus repository.
func NewBonusRepo(db *database.Postgres) *BonusRepo {
	return &BonusRepo{db: db}
}

// List returns all bonuses for a tenant.
func (r *BonusRepo) List(ctx context.Context, tenantID uuid.UUID) ([]models.Bonus, error) {
	query := `
		SELECT id, tenant_id, title, description, image_url, value,
		       is_active, sort_order, created_at, updated_at
		FROM bonuses WHERE tenant_id = $1
		ORDER BY sort_order, created_at`

	rows, err := r.db.Pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listing bonuses: %w", err)
	}
	defer rows.Close()

	var bonuses []models.Bonus
	for rows.Next() {
		var b models.Bonus
		if err := rows.Scan(&b.ID, &b.TenantID, &b.Title, &b.Description,
			&b.ImageURL, &b.Value, &b.IsActive, &b.SortOrder,
			&b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning bonus: %w", err)
		}
		bonuses = append(bonuses, b)
	}
	return bonuses, rows.Err()
}

// FindByID returns a bonus by ID.
func (r *BonusRepo) FindByID(ctx context.Context, id, tenantID uuid.UUID) (*models.Bonus, error) {
	query := `
		SELECT id, tenant_id, title, description, image_url, value,
		       is_active, sort_order, created_at, updated_at
		FROM bonuses WHERE id = $1 AND tenant_id = $2`

	var b models.Bonus
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID).Scan(
		&b.ID, &b.TenantID, &b.Title, &b.Description,
		&b.ImageURL, &b.Value, &b.IsActive, &b.SortOrder,
		&b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding bonus: %w", err)
	}
	return &b, nil
}

// Create inserts a new bonus.
func (r *BonusRepo) Create(ctx context.Context, tenantID uuid.UUID, title string, description *string, imageURL *string, value float64, sortOrder int) (*models.Bonus, error) {
	query := `
		INSERT INTO bonuses (tenant_id, title, description, image_url, value, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, tenant_id, title, description, image_url, value,
		          is_active, sort_order, created_at, updated_at`

	var b models.Bonus
	err := r.db.Pool.QueryRow(ctx, query, tenantID, title, description, imageURL, value, sortOrder).Scan(
		&b.ID, &b.TenantID, &b.Title, &b.Description,
		&b.ImageURL, &b.Value, &b.IsActive, &b.SortOrder,
		&b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating bonus: %w", err)
	}
	return &b, nil
}

// Update updates a bonus.
func (r *BonusRepo) Update(ctx context.Context, id, tenantID uuid.UUID, title string, description *string, imageURL *string, value float64, isActive bool, sortOrder int) (*models.Bonus, error) {
	query := `
		UPDATE bonuses SET title=$3, description=$4, image_url=$5, value=$6,
		       is_active=$7, sort_order=$8, updated_at=NOW()
		WHERE id=$1 AND tenant_id=$2
		RETURNING id, tenant_id, title, description, image_url, value,
		          is_active, sort_order, created_at, updated_at`

	var b models.Bonus
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID, title, description, imageURL, value, isActive, sortOrder).Scan(
		&b.ID, &b.TenantID, &b.Title, &b.Description,
		&b.ImageURL, &b.Value, &b.IsActive, &b.SortOrder,
		&b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating bonus: %w", err)
	}
	return &b, nil
}

// Delete removes a bonus.
func (r *BonusRepo) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, "DELETE FROM bonuses WHERE id=$1 AND tenant_id=$2", id, tenantID)
	return err
}

// GetBonusesForTermOption returns bonuses attached to a term option.
func (r *BonusRepo) GetBonusesForTermOption(ctx context.Context, termOptionID, tenantID uuid.UUID) ([]models.Bonus, error) {
	query := `
		SELECT b.id, b.tenant_id, b.title, b.description, b.image_url, b.value,
		       b.is_active, b.sort_order, b.created_at, b.updated_at
		FROM bonuses b
		JOIN term_option_bonuses tob ON tob.bonus_id = b.id
		WHERE tob.term_option_id = $1 AND tob.tenant_id = $2
		ORDER BY b.sort_order`

	rows, err := r.db.Pool.Query(ctx, query, termOptionID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("getting bonuses for term option: %w", err)
	}
	defer rows.Close()

	var bonuses []models.Bonus
	for rows.Next() {
		var b models.Bonus
		if err := rows.Scan(&b.ID, &b.TenantID, &b.Title, &b.Description,
			&b.ImageURL, &b.Value, &b.IsActive, &b.SortOrder,
			&b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning bonus: %w", err)
		}
		bonuses = append(bonuses, b)
	}
	return bonuses, rows.Err()
}

// SetTermOptionBonuses replaces all bonus assignments for a term option.
func (r *BonusRepo) SetTermOptionBonuses(ctx context.Context, termOptionID, tenantID uuid.UUID, bonusIDs []uuid.UUID) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Remove existing
	_, err = tx.Exec(ctx, "DELETE FROM term_option_bonuses WHERE term_option_id=$1 AND tenant_id=$2", termOptionID, tenantID)
	if err != nil {
		return fmt.Errorf("clearing existing bonuses: %w", err)
	}

	// Insert new
	for _, bonusID := range bonusIDs {
		_, err = tx.Exec(ctx,
			"INSERT INTO term_option_bonuses (tenant_id, term_option_id, bonus_id) VALUES ($1, $2, $3)",
			tenantID, termOptionID, bonusID)
		if err != nil {
			return fmt.Errorf("assigning bonus: %w", err)
		}
	}

	return tx.Commit(ctx)
}
