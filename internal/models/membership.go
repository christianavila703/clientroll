package models

import (
	"time"

	"github.com/google/uuid"
)

// MembershipType represents a type of membership offered by a tenant.
type MembershipType struct {
	ID                  uuid.UUID `json:"id"`
	TenantID            uuid.UUID `json:"tenant_id"`
	MembershipName      *string   `json:"membership_name"`
	MembershipLevel     *string   `json:"membership_level"`
	Title               string    `json:"title"`
	Slug                string    `json:"slug"`
	Summary             *string   `json:"summary"`
	Description         *string   `json:"description"`
	Inclusions          *string   `json:"inclusions"`
	Benefits            *string   `json:"benefits"`
	TopImageURL         *string   `json:"top_image_url"`
	CurrencyCode        string    `json:"currency_code"`
	IsActive            bool      `json:"is_active"`
	IsMostPopular       bool      `json:"is_most_popular"`
	IntrinsicValue      float64   `json:"intrinsic_value"`
	TotalValue          float64   `json:"total_value"`
	SortOrder           int       `json:"sort_order"`
	UseCustomThankyou   bool      `json:"use_custom_thankyou"`
	ThankyouHeading     *string   `json:"thankyou_heading"`
	ThankyouBody        *string   `json:"thankyou_body"`
	ThankyouImageURL    *string   `json:"thankyou_image_url"`
	ThankyouRedirectURL *string   `json:"thankyou_redirect_url"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	// Joined data
	TermOptions []TermOption `json:"term_options,omitempty"`
}

// TermOption represents a duration/pricing option for a membership type.
type TermOption struct {
	ID                          uuid.UUID `json:"id"`
	TenantID                    uuid.UUID `json:"tenant_id"`
	MembershipTypeID            uuid.UUID `json:"membership_type_id"`
	DurationMonths              int       `json:"duration_months"`
	FeeTotal                    float64   `json:"fee_total"`
	FeeWeekly                   *float64  `json:"fee_weekly"`
	FeeFortnightly              *float64  `json:"fee_fortnightly"`
	FeeMonthly                  *float64  `json:"fee_monthly"`
	FeeQuarterly                *float64  `json:"fee_quarterly"`
	AllowUpfront                bool      `json:"allow_upfront"`
	NumPayments                 *int      `json:"num_payments"` // if set, payments stop after N (membership stays active)
	CustomScheduleEnabled       bool      `json:"custom_schedule_enabled"`
	CustomScheduleLabel         *string   `json:"custom_schedule_label"`
	CustomScheduleAmount        *float64  `json:"custom_schedule_amount"`
	CustomScheduleIntervalUnit  *string   `json:"custom_schedule_interval_unit"`
	CustomScheduleIntervalCount *int      `json:"custom_schedule_interval_count"`
	CustomScheduleNumPayments   *int      `json:"custom_schedule_num_payments"`
	// Joined data
	Bonuses []Bonus `json:"bonuses,omitempty"`
}

// Bonus represents a standalone bonus that can be attached to term options.
type Bonus struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	ImageURL    *string   `json:"image_url"`
	Value       float64   `json:"value"`
	IsActive    bool      `json:"is_active"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TermsAndConditions represents a versioned T&C document.
type TermsAndConditions struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Version   int       `json:"version"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
