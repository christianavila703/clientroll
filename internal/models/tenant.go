package models

import (
	"time"

	"github.com/google/uuid"
)

// Tenant represents a tenant/organisation in the platform.
type Tenant struct {
	ID                         uuid.UUID  `json:"id"`
	Name                       string     `json:"name"`
	Slug                       string     `json:"slug"`
	Timezone                   string     `json:"timezone"`
	IsActive                   bool       `json:"is_active"`
	LogoURL                    *string    `json:"logo_url"`
	PrimaryColour              string     `json:"primary_colour"`
	SecondaryColour            *string    `json:"secondary_colour"`
	SignupPriceDisplayMode     string     `json:"signup_price_display_mode"`
	DefaultThankyouHeading     *string    `json:"default_thankyou_heading"`
	DefaultThankyouBody        *string    `json:"default_thankyou_body"`
	DefaultThankyouImageURL    *string    `json:"default_thankyou_image_url"`
	DefaultThankyouRedirectURL *string    `json:"default_thankyou_redirect_url"`
	SetupCompletedAt           *time.Time `json:"setup_completed_at"`
	CreatedAt                  time.Time  `json:"created_at"`
	UpdatedAt                  time.Time  `json:"updated_at"`
}

// CreateTenantInput is the input for creating a new tenant.
type CreateTenantInput struct {
	Name     string `json:"name" validate:"required,min=2,max=255"`
	Slug     string `json:"slug" validate:"required,min=2,max=100,alphanumdash"`
	Timezone string `json:"timezone" validate:"required"`
}

// UpdateTenantInput is the input for updating a tenant.
type UpdateTenantInput struct {
	Name                       *string `json:"name,omitempty"`
	Slug                       *string `json:"slug,omitempty"`
	Timezone                   *string `json:"timezone,omitempty"`
	IsActive                   *bool   `json:"is_active,omitempty"`
	PrimaryColour              *string `json:"primary_colour,omitempty"`
	SecondaryColour            *string `json:"secondary_colour,omitempty"`
	SignupPriceDisplayMode     *string `json:"signup_price_display_mode,omitempty"`
	LogoURL                    *string `json:"logo_url,omitempty"`
	DefaultThankyouHeading     *string `json:"default_thankyou_heading,omitempty"`
	DefaultThankyouBody        *string `json:"default_thankyou_body,omitempty"`
	DefaultThankyouImageURL    *string `json:"default_thankyou_image_url,omitempty"`
	DefaultThankyouRedirectURL *string `json:"default_thankyou_redirect_url,omitempty"`
}
