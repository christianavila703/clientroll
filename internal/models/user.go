package models

import (
	"time"

	"github.com/google/uuid"
)

// User roles
const (
	RolePlatformAdmin = "platform_admin"
	RoleTenantAdmin   = "tenant_admin"
	RoleTenantStaff   = "tenant_staff"
)

// User represents a tenant user (admin or staff).
type User struct {
	ID                  uuid.UUID  `json:"id"`
	TenantID            uuid.UUID  `json:"tenant_id"`
	Email               string     `json:"email"`
	PasswordHash        string     `json:"-"` // Never serialise
	FirstName           string     `json:"first_name"`
	LastName            string     `json:"last_name"`
	Role                string     `json:"role"`
	IsActive            bool       `json:"is_active"`
	FailedLoginAttempts int        `json:"-"`
	LockedUntil         *time.Time `json:"-"`
	LastLoginAt         *time.Time `json:"last_login_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// FullName returns the user's full name.
func (u *User) FullName() string {
	return u.FirstName + " " + u.LastName
}

// IsLocked returns true if the account is currently locked.
func (u *User) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LockedUntil)
}

// PlatformAdmin represents a platform-level super admin.
type PlatformAdmin struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// LoginInput is the input for login requests.
type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=10"`
}

// UserResponse is the safe representation of a user for API responses.
type UserResponse struct {
	ID        uuid.UUID  `json:"id"`
	TenantID  uuid.UUID  `json:"tenant_id"`
	Email     string     `json:"email"`
	FirstName string     `json:"first_name"`
	LastName  string     `json:"last_name"`
	Role      string     `json:"role"`
	IsActive  bool       `json:"is_active"`
	LastLogin *time.Time `json:"last_login_at"`
}

// ToResponse converts a User to a safe UserResponse.
func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:        u.ID,
		TenantID:  u.TenantID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Role:      u.Role,
		IsActive:  u.IsActive,
		LastLogin: u.LastLoginAt,
	}
}

// AuthTokens holds the access and refresh token pair.
type AuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // seconds
}

// LoginResponse is returned after successful login.
type LoginResponse struct {
	User   UserResponse `json:"user"`
	Tokens AuthTokens   `json:"tokens"`
}
