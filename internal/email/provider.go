package email

import "context"

// Provider defines the interface for email delivery.
type Provider interface {
	Send(ctx context.Context, input SendInput) (string, error)
}

// SendInput holds data for sending an email.
type SendInput struct {
	To           string
	Subject      string
	TemplateName string
	Variables    map[string]string
}

// Template names used across the application.
const (
	TemplateWelcome              = "welcome"
	TemplateMembershipConfirm    = "membership_confirmation"
	TemplatePasswordReset        = "password_reset"
	TemplateStaffInvitation      = "staff_invitation"
	TemplatePaymentReceipt       = "payment_receipt"
	TemplatePaymentFailed        = "payment_failed"
	TemplateDisputeAlert         = "dispute_alert"
	TemplateMembershipExpiring   = "membership_expiring"
)
