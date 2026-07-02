package email

import "context"

// Provider defines the interface for email delivery.
type Provider interface {
	Send(ctx context.Context, input SendInput) (string, error)
}

// Attachment represents an email file attachment.
type Attachment struct {
	Filename string
	Content  []byte
	MimeType string
}

// SendInput holds data for sending an email.
type SendInput struct {
	To          string
	Subject     string
	HTML        string
	Text        string
	Attachments []Attachment
}
