package models

import (
	"time"

	"github.com/google/uuid"
)

// Client represents a member/client of a tenant.
type Client struct {
	ID                    uuid.UUID  `json:"id"`
	TenantID              uuid.UUID  `json:"tenant_id"`
	Name                  string     `json:"name"`
	Address               *string    `json:"address"`
	Suburb                *string    `json:"suburb"`
	State                 *string    `json:"state"`
	Postcode              *string    `json:"postcode"`
	Email                 *string    `json:"email"`
	Phone                 *string    `json:"phone"`
	BirthDate             *time.Time `json:"birth_date"`
	Gender                *string    `json:"gender"`
	EmergencyContactName  *string    `json:"emergency_contact_name"`
	EmergencyContactPhone *string    `json:"emergency_contact_phone"`
	GoCardlessCustomerID  *string    `json:"gocardless_customer_id,omitempty"`
	DeletedAt             *time.Time `json:"deleted_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// Membership represents a signed membership agreement.
type Membership struct {
	ID                   uuid.UUID  `json:"id"`
	TenantID             uuid.UUID  `json:"tenant_id"`
	ClientID             uuid.UUID  `json:"client_id"`
	MembershipTypeID     uuid.UUID  `json:"membership_type_id"`
	TermOptionID         uuid.UUID  `json:"term_option_id"`
	StartDate            time.Time  `json:"start_date"`
	MinTermEndDate       time.Time  `json:"min_term_end_date"`
	PaymentMethod        *string    `json:"payment_method"`
	PaymentFrequency     *string    `json:"payment_frequency"`
	PaymentIntervalUnit  *string    `json:"payment_interval_unit"`
	PaymentIntervalCount int        `json:"payment_interval_count"`
	PaymentAmount        *float64   `json:"payment_amount"`
	NumPayments          *int       `json:"num_payments"`
	PaymentsMade         int        `json:"payments_made"`
	Status               string     `json:"status"`
	GoCardlessMandateID      *string    `json:"gocardless_mandate_id,omitempty"`
	GoCardlessSubscriptionID *string    `json:"gocardless_subscription_id,omitempty"`
	MemberSignature    *string    `json:"member_signature_data"`
	MemberSignedAt       *time.Time `json:"member_signed_at"`
	StaffUserID          *uuid.UUID `json:"staff_user_id"`
	StaffSignature       *string    `json:"staff_signature_data"`
	StaffSignedAt        *time.Time `json:"staff_signed_at"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// GetID returns the membership ID (implements interface for gc_helpers).
func (m *Membership) GetID() uuid.UUID { return m.ID }
