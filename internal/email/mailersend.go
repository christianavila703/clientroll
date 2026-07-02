package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// MailerSendProvider implements the email Provider using the MailerSend API.
type MailerSendProvider struct {
	apiKey    string
	fromEmail string
	fromName  string
	client    *http.Client
}

// NewMailerSendProvider creates a new MailerSend provider.
func NewMailerSendProvider(apiKey, fromEmail, fromName string) *MailerSendProvider {
	return &MailerSendProvider{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		fromName:  fromName,
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (m *MailerSendProvider) Send(ctx context.Context, input SendInput) (string, error) {
	if m.apiKey == "" {
		log.Printf("MailerSend: skipping send (no API key) to=%s subject=%s", input.To, input.Subject)
		return "skipped_no_key", nil
	}

	body := map[string]interface{}{
		"from": map[string]string{
			"email": m.fromEmail,
			"name":  m.fromName,
		},
		"to": []map[string]string{
			{"email": input.To},
		},
		"subject": input.Subject,
		"html":    input.HTML,
		"text":    input.Text,
	}

	if len(input.Attachments) > 0 {
		var atts []map[string]string
		for _, a := range input.Attachments {
			atts = append(atts, map[string]string{
				"filename":    a.Filename,
				"content":     base64.StdEncoding.EncodeToString(a.Content),
				"disposition": "attachment",
			})
		}
		body["attachments"] = atts
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshalling email body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.mailersend.com/v1/email", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)

	resp, err := m.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending email: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("MailerSend error %d: %s", resp.StatusCode, string(respBody))
	}

	messageID := resp.Header.Get("X-Message-Id")
	log.Printf("MailerSend: sent to=%s subject=%s messageId=%s", input.To, input.Subject, messageID)
	return messageID, nil
}
