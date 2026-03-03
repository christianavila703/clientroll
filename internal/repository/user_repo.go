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

// UserRepo handles database operations for users.
type UserRepo struct {
	db *database.Postgres
}

// NewUserRepo creates a new user repository.
func NewUserRepo(db *database.Postgres) *UserRepo {
	return &UserRepo{db: db}
}

// FindByEmailAndTenant looks up a user by email within a specific tenant.
func (r *UserRepo) FindByEmailAndTenant(ctx context.Context, email string, tenantID uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, tenant_id, email, password_hash, first_name, last_name, role,
		       is_active, failed_login_attempts, locked_until, last_login_at,
		       created_at, updated_at
		FROM users
		WHERE email = $1 AND tenant_id = $2`

	var u models.User
	err := r.db.Pool.QueryRow(ctx, query, email, tenantID).Scan(
		&u.ID, &u.TenantID, &u.Email, &u.PasswordHash,
		&u.FirstName, &u.LastName, &u.Role, &u.IsActive,
		&u.FailedLoginAttempts, &u.LockedUntil, &u.LastLoginAt,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding user by email: %w", err)
	}
	return &u, nil
}

// FindByEmail looks up a user by email across all tenants (for login flow).
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, tenant_id, email, password_hash, first_name, last_name, role,
		       is_active, failed_login_attempts, locked_until, last_login_at,
		       created_at, updated_at
		FROM users
		WHERE email = $1 AND is_active = true
		LIMIT 1`

	var u models.User
	err := r.db.Pool.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.TenantID, &u.Email, &u.PasswordHash,
		&u.FirstName, &u.LastName, &u.Role, &u.IsActive,
		&u.FailedLoginAttempts, &u.LockedUntil, &u.LastLoginAt,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding user by email: %w", err)
	}
	return &u, nil
}

// ListActiveByEmail returns all active users matching an email across tenants.
// This is used by login to avoid false 401s when duplicate emails exist across tenants.
func (r *UserRepo) ListActiveByEmail(ctx context.Context, email string) ([]models.User, error) {
	query := `
		SELECT id, tenant_id, email, password_hash, first_name, last_name, role,
		       is_active, failed_login_attempts, locked_until, last_login_at,
		       created_at, updated_at
		FROM users
		WHERE email = $1 AND is_active = true
		ORDER BY created_at DESC`

	rows, err := r.db.Pool.Query(ctx, query, email)
	if err != nil {
		return nil, fmt.Errorf("listing users by email: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(
			&u.ID, &u.TenantID, &u.Email, &u.PasswordHash,
			&u.FirstName, &u.LastName, &u.Role, &u.IsActive,
			&u.FailedLoginAttempts, &u.LockedUntil, &u.LastLoginAt,
			&u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning users by email: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// FindByID returns a user by ID within a tenant.
func (r *UserRepo) FindByID(ctx context.Context, id, tenantID uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, tenant_id, email, password_hash, first_name, last_name, role,
		       is_active, failed_login_attempts, locked_until, last_login_at,
		       created_at, updated_at
		FROM users
		WHERE id = $1 AND tenant_id = $2`

	var u models.User
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID).Scan(
		&u.ID, &u.TenantID, &u.Email, &u.PasswordHash,
		&u.FirstName, &u.LastName, &u.Role, &u.IsActive,
		&u.FailedLoginAttempts, &u.LockedUntil, &u.LastLoginAt,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding user by ID: %w", err)
	}
	return &u, nil
}

// Create inserts a new user.
func (r *UserRepo) Create(ctx context.Context, tenantID uuid.UUID, email, passwordHash, firstName, lastName, role string) (*models.User, error) {
	query := `
		INSERT INTO users (tenant_id, email, password_hash, first_name, last_name, role)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, tenant_id, email, first_name, last_name, role, is_active, created_at, updated_at`

	var u models.User
	err := r.db.Pool.QueryRow(ctx, query,
		tenantID, email, passwordHash, firstName, lastName, role,
	).Scan(
		&u.ID, &u.TenantID, &u.Email, &u.FirstName, &u.LastName,
		&u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}
	return &u, nil
}

// RecordLoginSuccess resets failed attempts and updates last login time.
func (r *UserRepo) RecordLoginSuccess(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE users
		SET failed_login_attempts = 0, locked_until = NULL, last_login_at = $2, updated_at = $2
		WHERE id = $1`

	_, err := r.db.Pool.Exec(ctx, query, userID, time.Now())
	return err
}

// RecordLoginFailure increments failed attempts and locks if threshold reached.
func (r *UserRepo) RecordLoginFailure(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE users
		SET failed_login_attempts = failed_login_attempts + 1,
		    locked_until = CASE
		        WHEN failed_login_attempts + 1 >= 5 THEN NOW() + INTERVAL '30 minutes'
		        ELSE locked_until
		    END,
		    updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.Pool.Exec(ctx, query, userID)
	return err
}

// ListByTenant returns all users for a tenant.
func (r *UserRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.User, error) {
	query := `
		SELECT id, tenant_id, email, password_hash, first_name, last_name, role,
		       is_active, failed_login_attempts, locked_until, last_login_at,
		       created_at, updated_at
		FROM users
		WHERE tenant_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.Pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(
			&u.ID, &u.TenantID, &u.Email, &u.PasswordHash,
			&u.FirstName, &u.LastName, &u.Role, &u.IsActive,
			&u.FailedLoginAttempts, &u.LockedUntil, &u.LastLoginAt,
			&u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// UpdateStaffProfile updates editable fields for a tenant staff user.
func (r *UserRepo) UpdateStaffProfile(ctx context.Context, userID, tenantID uuid.UUID, firstName, lastName, role string) (*models.User, error) {
	query := `
		UPDATE users
		SET first_name = $3, last_name = $4, role = $5, updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND role IN ('tenant_admin', 'tenant_staff')
		RETURNING id, tenant_id, email, password_hash, first_name, last_name, role,
		       is_active, failed_login_attempts, locked_until, last_login_at,
		       created_at, updated_at`

	var u models.User
	err := r.db.Pool.QueryRow(ctx, query, userID, tenantID, firstName, lastName, role).Scan(
		&u.ID, &u.TenantID, &u.Email, &u.PasswordHash,
		&u.FirstName, &u.LastName, &u.Role, &u.IsActive,
		&u.FailedLoginAttempts, &u.LockedUntil, &u.LastLoginAt,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("updating staff profile: %w", err)
	}
	return &u, nil
}

// DeactivateStaff disables a tenant staff/admin user account.
func (r *UserRepo) DeactivateStaff(ctx context.Context, userID, tenantID uuid.UUID) error {
	query := `
		UPDATE users
		SET is_active = false, updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND role IN ('tenant_admin', 'tenant_staff')`
	tag, err := r.db.Pool.Exec(ctx, query, userID, tenantID)
	if err != nil {
		return fmt.Errorf("deactivating staff: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
