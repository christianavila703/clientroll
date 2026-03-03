package handlers

import "testing"

func TestWebhookResourceIDFromLinks(t *testing.T) {
	tests := []struct {
		resourceType string
		paymentID    string
		mandateID    string
		subID        string
		refundID     string
		fallback     string
		want         string
	}{
		{resourceType: "payments", paymentID: "PM1", fallback: "EV1", want: "PM1"},
		{resourceType: "mandates", mandateID: "MD1", fallback: "EV1", want: "MD1"},
		{resourceType: "subscriptions", subID: "SB1", fallback: "EV1", want: "SB1"},
		{resourceType: "refunds", refundID: "RF1", fallback: "EV1", want: "RF1"},
		{resourceType: "organisations", fallback: "EV1", want: "EV1"},
	}

	for _, tt := range tests {
		got := webhookResourceIDFromLinks(tt.resourceType, tt.paymentID, tt.mandateID, tt.subID, tt.refundID, tt.fallback)
		if got != tt.want {
			t.Fatalf("resource=%s expected %q, got %q", tt.resourceType, tt.want, got)
		}
	}
}
