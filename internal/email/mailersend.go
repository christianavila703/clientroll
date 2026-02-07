package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MailerSendProvider implements the email Provider using the MailerSend API.
type MailerSendProvider struct {
	apiKey string
	client *http.Client
}

// NewMailerSendProvider creates a new MailerSend provider.
func NewMailerSendProvider(apiKey string) *MailerSendProvider {
	return &MailerSendProvider{
		apiKey: apiKey,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (m *MailerSendProvider) Send(ctx context.Context, input SendInput) (string, error) {
	if m.apiKey == "" {
		return "", fmt.Errorf("MailerSend API key not configured")
	}

	body := map[string]interface{}{
		"to":      []map[string]string{{"email": input.To}},
		"subject": input.Subject,
		"text":    buildPlainText(input),
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

	// Extract message ID from header
	messageID := resp.Header.Get("X-Message-Id")
	return messageID, nil
}

func buildPlainText(input SendInput) string {
	text := input.Subject + "\n\n"
	for k, v := range input.Variables {
		text += k + ": " + v + "\n"
	}
	return text
}
