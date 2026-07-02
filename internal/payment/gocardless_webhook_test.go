package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestHandleWebhook_VerifiesSignature(t *testing.T) {
	provider := &GoCardlessProvider{apiSecret: "whsec_test"}
	payload := []byte(`{"events":[{"id":"EV123","resource_type":"payments","action":"paid_out","links":{"payment":"PM123"}}]}`)

	mac := hmac.New(sha256.New, []byte("whsec_test"))
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))

	event, err := provider.HandleWebhook(context.Background(), payload, sig)
	if err != nil {
		t.Fatalf("expected signature to pass, got error: %v", err)
	}
	if event == nil || event.ResourceID != "PM123" || event.Type != "payments.paid_out" {
		t.Fatalf("unexpected parsed event: %#v", event)
	}

	if _, err := provider.HandleWebhook(context.Background(), payload, "bad_signature"); err == nil {
		t.Fatal("expected signature mismatch error")
	}
}
