package repository

import (
	"context"
	"fmt"

	"clientroll/internal/database"
	"clientroll/internal/models"

	"github.com/jackc/pgx/v5"
)

// PlatformAdminRepo handles database operations for platform admins.
type PlatformAdminRepo struct {
	db *database.Postgres
}

// NewPlatformAdminRepo creates a new platform admin repository.
func NewPlatformAdminRepo(db *database.Postgres) *PlatformAdminRepo {
	return &PlatformAdminRepo{db: db}
}

// FindByEmail looks up a platform admin by email.
func (r *PlatformAdminRepo) FindByEmail(ctx context.Context, email string) (*models.PlatformAdmin, error) {
	query := `
		SELECT id, email, password_hash, name, is_active, created_at, updated_at
		FROM platform_admins
		WHERE email = $1 AND is_active = true`

	var a models.PlatformAdmin
	err := r.db.Pool.QueryRow(ctx, query, email).Scan(
		&a.ID, &a.Email, &a.PasswordHash, &a.Name,
		&a.IsActive, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding platform admin: %w", err)
	}
	return &a, nil
}

// Create inserts a new platform admin.
func (r *PlatformAdminRepo) Create(ctx context.Context, email, passwordHash, name string) (*models.PlatformAdmin, error) {
	query := `
		INSERT INTO platform_admins (email, password_hash, name)
		VALUES ($1, $2, $3)
		RETURNING id, email, name, is_active, created_at, updated_at`

	var a models.PlatformAdmin
	err := r.db.Pool.QueryRow(ctx, query, email, passwordHash, name).Scan(
		&a.ID, &a.Email, &a.Name, &a.IsActive, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating platform admin: %w", err)
	}
	return &a, nil
}
