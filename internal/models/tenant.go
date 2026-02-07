package models

import (
	"time"

	"github.com/google/uuid"
)

// Tenant represents a tenant/organisation in the platform.
type Tenant struct {
	ID                      uuid.UUID `json:"id"`
	Name                    string    `json:"name"`
	Slug                    string    `json:"slug"`
	Timezone                string    `json:"timezone"`
	IsActive                bool      `json:"is_active"`
	LogoURL                 *string   `json:"logo_url"`
	PrimaryColour           string    `json:"primary_colour"`
	DefaultThankyouHeading  *string   `json:"default_thankyou_heading"`
	DefaultThankyouBody     *string   `json:"default_thankyou_body"`
	DefaultThankyouImageURL *string   `json:"default_thankyou_image_url"`
	DefaultThankyouRedirectURL *string `json:"default_thankyou_redirect_url"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// CreateTenantInput is the input for creating a new tenant.
type CreateTenantInput struct {
	Name     string `json:"name" validate:"required,min=2,max=255"`
	Slug     string `json:"slug" validate:"required,min=2,max=100,alphanumdash"`
	Timezone string `json:"timezone" validate:"required"`
}

// UpdateTenantInput is the input for updating a tenant.
type UpdateTenantInput struct {
	Name          *string `json:"name,omitempty"`
	Timezone      *string `json:"timezone,omitempty"`
	IsActive      *bool   `json:"is_active,omitempty"`
	PrimaryColour *string `json:"primary_colour,omitempty"`
}
