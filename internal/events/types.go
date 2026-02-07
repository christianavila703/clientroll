package events

import (
	"time"

	"github.com/google/uuid"
)

// Event types for outbound webhooks.
const (
	ClientCreated       = "client.created"
	ClientUpdated       = "client.updated"
	MembershipCreated   = "membership.created"
	MembershipActivated = "membership.activated"
	MembershipCancelled = "membership.cancelled"
	MembershipExpired   = "membership.expired"
	PaymentSucceeded    = "payment.succeeded"
	PaymentFailed       = "payment.failed"
	PaymentDisputed     = "payment.disputed"
	PaymentRefunded     = "payment.refunded"
	SignupStarted       = "signup.started"
	SignupCompleted     = "signup.completed"
)

// AllEventTypes returns all valid event types.
func AllEventTypes() []string {
	return []string{
		ClientCreated, ClientUpdated,
		MembershipCreated, MembershipActivated, MembershipCancelled, MembershipExpired,
		PaymentSucceeded, PaymentFailed, PaymentDisputed, PaymentRefunded,
		SignupStarted, SignupCompleted,
	}
}

// Event represents an outbound webhook event.
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	TenantID  string                 `json:"tenant_id"`
	CreatedAt time.Time              `json:"created_at"`
	Data      map[string]interface{} `json:"data"`
}

// NewEvent creates a new event with a unique ID.
func NewEvent(eventType string, tenantID uuid.UUID, data map[string]interface{}) Event {
	return Event{
		ID:        "evt_" + uuid.New().String()[:12],
		Type:      eventType,
		TenantID:  tenantID.String(),
		CreatedAt: time.Now().UTC(),
		Data:      data,
	}
}
