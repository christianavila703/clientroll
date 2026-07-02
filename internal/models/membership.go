package models

import (
	"time"

	"github.com/google/uuid"
)

// MembershipType represents a type of membership offered by a tenant.
type MembershipType struct {
	ID              uuid.UUID `json:"id"`
	TenantID        uuid.UUID `json:"tenant_id"`
	MembershipName  *string   `json:"membership_name"`
	MembershipLevel *string   `json:"membership_level"`
	Title           string    `json:"title"`
	Slug            string    `json:"slug"`
	Summary         *string   `json:"summary"`
	Description     *string   `json:"description"`
	Inclusions      *string   `json:"inclusions"`
	Benefits        *string   `json:"benefits"`
	TopImageURL     *string   `json:"top_image_url"`
	CurrencyCode    string    `json:"currency_code"`
	// If set, this overrides tenant-level enabled payment frequencies for this membership.
	EnabledPaymentFrequencies []string  `json:"enabled_payment_frequencies,omitempty"`
	ApplySalesTax             bool      `json:"apply_sales_tax"`
	IsActive      bool `json:"is_active"`
	IsMostPopular bool `json:"is_most_popular"`
	SortOrder     int  `json:"sort_order"`
	UseCustomThankyou         bool      `json:"use_custom_thankyou"`
	ThankyouHeading           *string   `json:"thankyou_heading"`
	ThankyouBody              *string   `json:"thankyou_body"`
	ThankyouImageURL          *string   `json:"thankyou_image_url"`
	ThankyouRedirectURL       *string   `json:"thankyou_redirect_url"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
	// Joined data
	TermOptions []TermOption `json:"term_options,omitempty"`
}

// TermOption represents a duration/pricing option for a membership type.
//
// After migration 074 the term option only carries duration + display metadata.
// The pricing/schedule rows live in PaymentSchedules (loaded as joined data).
// The legacy Fee*, CustomSchedule*, AllowUpfront, ContinueAfterTerm fields are
// kept here for read-back compatibility during the soak window; new code paths
// should rely on PaymentSchedules instead.
type TermOption struct {
	ID               uuid.UUID `json:"id"`
	TenantID         uuid.UUID `json:"tenant_id"`
	MembershipTypeID uuid.UUID `json:"membership_type_id"`
	// New v2 fields (migration 074).
	DurationValue int        `json:"duration_value"`
	DurationUnit  string     `json:"duration_unit"`
	Label         *string    `json:"label"`
	SortOrder     int        `json:"sort_order"`
	ArchivedAt    *time.Time `json:"archived_at"`
	// Legacy aggregate columns. duration_months mirrors duration_value when
	// duration_unit='month' and is roughly weeks*12/52 otherwise.
	DurationMonths              int      `json:"duration_months"`
	FeeTotal                    float64  `json:"fee_total"`
	FeeWeekly                   *float64 `json:"fee_weekly"`
	FeeFortnightly              *float64 `json:"fee_fortnightly"`
	FeeMonthly                  *float64 `json:"fee_monthly"`
	FeeQuarterly                *float64 `json:"fee_quarterly"`
	AllowUpfront                bool     `json:"allow_upfront"`
	NumPayments                 *int     `json:"num_payments"`
	ContinueAfterTerm           bool     `json:"continue_after_term"`
	CustomScheduleEnabled       bool     `json:"custom_schedule_enabled"`
	CustomScheduleLabel         *string  `json:"custom_schedule_label"`
	CustomScheduleAmount        *float64 `json:"custom_schedule_amount"`
	CustomScheduleIntervalUnit  *string  `json:"custom_schedule_interval_unit"`
	CustomScheduleIntervalCount *int     `json:"custom_schedule_interval_count"`
	CustomScheduleNumPayments   *int     `json:"custom_schedule_num_payments"`
	// Joined data
	Bonuses          []Bonus           `json:"bonuses,omitempty"`
	PaymentSchedules []PaymentSchedule `json:"payment_schedules,omitempty"`
	AttachedTerms    []AttachedTerms   `json:"attached_terms,omitempty"`
}

// PaymentSchedule is one way a TermOption can be paid for.
//
//   - kind = "upfront": a single payment of `Amount`. IntervalUnit/Count are nil.
//   - kind = "recurring": pay `Amount` every `IntervalCount` of `IntervalUnit`.
//     NumPayments=nil means it continues indefinitely (subscription); a set
//     value means it stops after N charges.
//   - kind = "instalments": same shape as recurring but NumPayments is required
//     and `Amount * NumPayments` typically equals the contract total.
type PaymentSchedule struct {
	ID            uuid.UUID  `json:"id"`
	TenantID      uuid.UUID  `json:"tenant_id"`
	TermOptionID  uuid.UUID  `json:"term_option_id"`
	Kind          string     `json:"kind"`
	Label         *string    `json:"label"`
	Amount        float64    `json:"amount"`
	IntervalUnit  *string    `json:"interval_unit"`
	IntervalCount *int       `json:"interval_count"`
	NumPayments   *int       `json:"num_payments"`
	IsDefault     bool       `json:"is_default"`
	SortOrder     int        `json:"sort_order"`
	ArchivedAt    *time.Time `json:"archived_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// AttachedTerms is the projection of a term_option_terms row into the
// loaded view of a TermOption. It mirrors a TermsAndConditions row plus the
// attachment metadata (is_required, sort_order).
type AttachedTerms struct {
	TermsAndConditions
	IsRequired bool `json:"is_required"`
	AttachSort int  `json:"attach_sort_order"`
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
