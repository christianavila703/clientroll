package events

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// GenerateInboundKeyID creates a random public key identifier for inbound webhooks.
func GenerateInboundKeyID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "whk_" + hex.EncodeToString([]byte(GenerateWebhookSecret()))[:24]
	}
	return "whk_" + hex.EncodeToString(buf)
}

// GenerateInboundSigningSecret creates a random signing secret for inbound webhooks.
func GenerateInboundSigningSecret() string {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return GenerateWebhookSecret()
	}
	return "whsec_in_" + hex.EncodeToString(buf)
}

// SignTimestampedPayload computes HMAC-SHA256 over "<timestamp>.<payload>".
func SignTimestampedPayload(timestamp string, payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
