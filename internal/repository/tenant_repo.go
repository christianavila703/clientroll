package repository

import (
	"context"
	"fmt"

	"clientroll/internal/database"
	"clientroll/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// TenantRepo handles database operations for tenants.
type TenantRepo struct {
	db *database.Postgres
}

// NewTenantRepo creates a new tenant repository.
func NewTenantRepo(db *database.Postgres) *TenantRepo {
	return &TenantRepo{db: db}
}

// FindByID returns a tenant by ID.
func (r *TenantRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error) {
	query := `
		SELECT id, name, slug, timezone, is_active, logo_url, primary_colour, secondary_colour, signup_price_display_mode,
		       default_thankyou_heading, default_thankyou_body,
		       default_thankyou_image_url, default_thankyou_redirect_url,
		       created_at, updated_at
		FROM tenants WHERE id = $1`

	var t models.Tenant
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&t.ID, &t.Name, &t.Slug, &t.Timezone, &t.IsActive,
		&t.LogoURL, &t.PrimaryColour, &t.SecondaryColour, &t.SignupPriceDisplayMode,
		&t.DefaultThankyouHeading, &t.DefaultThankyouBody,
		&t.DefaultThankyouImageURL, &t.DefaultThankyouRedirectURL,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding tenant by ID: %w", err)
	}
	return &t, nil
}

// FindBySlug returns a tenant by slug.
func (r *TenantRepo) FindBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	query := `
		SELECT id, name, slug, timezone, is_active, logo_url, primary_colour, secondary_colour, signup_price_display_mode,
		       default_thankyou_heading, default_thankyou_body,
		       default_thankyou_image_url, default_thankyou_redirect_url,
		       created_at, updated_at
		FROM tenants WHERE slug = $1`

	var t models.Tenant
	err := r.db.Pool.QueryRow(ctx, query, slug).Scan(
		&t.ID, &t.Name, &t.Slug, &t.Timezone, &t.IsActive,
		&t.LogoURL, &t.PrimaryColour, &t.SecondaryColour, &t.SignupPriceDisplayMode,
		&t.DefaultThankyouHeading, &t.DefaultThankyouBody,
		&t.DefaultThankyouImageURL, &t.DefaultThankyouRedirectURL,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding tenant by slug: %w", err)
	}
	return &t, nil
}

// Create inserts a new tenant.
func (r *TenantRepo) Create(ctx context.Context, input models.CreateTenantInput) (*models.Tenant, error) {
	query := `
		INSERT INTO tenants (name, slug, timezone)
		VALUES ($1, $2, $3)
		RETURNING id, name, slug, timezone, is_active, logo_url, primary_colour, secondary_colour, signup_price_display_mode,
		          default_thankyou_heading, default_thankyou_body,
		          default_thankyou_image_url, default_thankyou_redirect_url,
		          created_at, updated_at`

	var t models.Tenant
	err := r.db.Pool.QueryRow(ctx, query, input.Name, input.Slug, input.Timezone).Scan(
		&t.ID, &t.Name, &t.Slug, &t.Timezone, &t.IsActive,
		&t.LogoURL, &t.PrimaryColour, &t.SecondaryColour, &t.SignupPriceDisplayMode,
		&t.DefaultThankyouHeading, &t.DefaultThankyouBody,
		&t.DefaultThankyouImageURL, &t.DefaultThankyouRedirectURL,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating tenant: %w", err)
	}
	return &t, nil
}

// List returns all tenants (for platform admin).
func (r *TenantRepo) List(ctx context.Context) ([]models.Tenant, error) {
	query := `
		SELECT id, name, slug, timezone, is_active, logo_url, primary_colour, secondary_colour, signup_price_display_mode,
		       default_thankyou_heading, default_thankyou_body,
		       default_thankyou_image_url, default_thankyou_redirect_url,
		       created_at, updated_at
		FROM tenants ORDER BY created_at DESC`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing tenants: %w", err)
	}
	defer rows.Close()

	var tenants []models.Tenant
	for rows.Next() {
		var t models.Tenant
		if err := rows.Scan(
			&t.ID, &t.Name, &t.Slug, &t.Timezone, &t.IsActive,
			&t.LogoURL, &t.PrimaryColour, &t.SecondaryColour, &t.SignupPriceDisplayMode,
			&t.DefaultThankyouHeading, &t.DefaultThankyouBody,
			&t.DefaultThankyouImageURL, &t.DefaultThankyouRedirectURL,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning tenant: %w", err)
		}
		tenants = append(tenants, t)
	}
	return tenants, rows.Err()
}

// Update updates a tenant.
func (r *TenantRepo) Update(ctx context.Context, id uuid.UUID, input models.UpdateTenantInput) (*models.Tenant, error) {
	query := `
		UPDATE tenants SET
			name = COALESCE($2, name),
			timezone = COALESCE($3, timezone),
			is_active = COALESCE($4, is_active),
			primary_colour = COALESCE($5, primary_colour),
			secondary_colour = COALESCE($6, secondary_colour),
			logo_url = COALESCE($7, logo_url),
			default_thankyou_heading = COALESCE($8, default_thankyou_heading),
			default_thankyou_body = COALESCE($9, default_thankyou_body),
			default_thankyou_image_url = COALESCE($10, default_thankyou_image_url),
			default_thankyou_redirect_url = COALESCE($11, default_thankyou_redirect_url),
			signup_price_display_mode = COALESCE($12, signup_price_display_mode),
			slug = COALESCE($13, slug),
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, slug, timezone, is_active, logo_url, primary_colour, secondary_colour, signup_price_display_mode,
		          default_thankyou_heading, default_thankyou_body,
		          default_thankyou_image_url, default_thankyou_redirect_url,
		          created_at, updated_at`

	var t models.Tenant
	err := r.db.Pool.QueryRow(ctx, query, id,
		input.Name, input.Timezone, input.IsActive, input.PrimaryColour,
		input.SecondaryColour, input.LogoURL,
		input.DefaultThankyouHeading, input.DefaultThankyouBody, input.DefaultThankyouImageURL, input.DefaultThankyouRedirectURL, input.SignupPriceDisplayMode, input.Slug,
	).Scan(
		&t.ID, &t.Name, &t.Slug, &t.Timezone, &t.IsActive,
		&t.LogoURL, &t.PrimaryColour, &t.SecondaryColour, &t.SignupPriceDisplayMode,
		&t.DefaultThankyouHeading, &t.DefaultThankyouBody,
		&t.DefaultThankyouImageURL, &t.DefaultThankyouRedirectURL,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("updating tenant: %w", err)
	}
	return &t, nil
}
