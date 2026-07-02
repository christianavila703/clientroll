package handlers

import (
	"encoding/json"
	"testing"

	"github.com/stripe/stripe-go/v82"
)

func TestMapGoCardlessPaymentAction(t *testing.T) {
	tests := []struct {
		name      string
		action    string
		status    string
		dispute   bool
		handled   bool
	}{
		{name: "success", action: "paid_out", status: "succeeded", dispute: false, handled: true},
		{name: "pending", action: "submitted", status: "pending", dispute: false, handled: true},
		{name: "failed", action: "failed", status: "failed", dispute: false, handled: true},
		{name: "chargeback", action: "charged_back", status: "disputed", dispute: true, handled: true},
		{name: "cancelled", action: "cancelled", status: "cancelled", dispute: false, handled: true},
		{name: "approval", action: "customer_approval_granted", status: "", dispute: false, handled: true},
		{name: "unknown", action: "some_new_action", status: "", dispute: false, handled: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, dispute, handled := mapGoCardlessPaymentAction(tt.action)
			if status != tt.status || dispute != tt.dispute || handled != tt.handled {
				t.Fatalf("mapGoCardlessPaymentAction(%q) = (%q,%v,%v), want (%q,%v,%v)", tt.action, status, dispute, handled, tt.status, tt.dispute, tt.handled)
			}
		})
	}
}

func TestIsGoCardlessRefundActionSupported(t *testing.T) {
	for _, action := range []string{"created", "submitted", "paid", "refunded"} {
		if !isGoCardlessRefundActionSupported(action) {
			t.Fatalf("expected action %q to be supported", action)
		}
	}
	if isGoCardlessRefundActionSupported("cancelled") {
		t.Fatal("did not expect cancelled refund action to be supported")
	}
}

func TestMapStripeDisputeState(t *testing.T) {
	tests := []struct {
		name        string
		eventType   stripe.EventType
		stripeState string
		wantDispute string
		wantPayment string
		wantTime    bool
	}{
		{name: "under review", eventType: "charge.dispute.updated", stripeState: "warning_under_review", wantDispute: "under_review", wantPayment: "disputed", wantTime: false},
		{name: "won", eventType: "charge.dispute.closed", stripeState: "won", wantDispute: "won", wantPayment: "succeeded", wantTime: true},
		{name: "lost", eventType: "charge.dispute.closed", stripeState: "lost", wantDispute: "lost", wantPayment: "disputed", wantTime: true},
		{name: "closed fallback", eventType: "charge.dispute.closed", stripeState: "", wantDispute: "resolved", wantPayment: "disputed", wantTime: true},
		{name: "default open", eventType: "charge.dispute.created", stripeState: "", wantDispute: "open", wantPayment: "disputed", wantTime: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			disputeStatus, paymentStatus, resolvedAt := mapStripeDisputeState(tt.eventType, tt.stripeState)
			if disputeStatus != tt.wantDispute || paymentStatus != tt.wantPayment {
				t.Fatalf("mapStripeDisputeState(%q,%q) = (%q,%q), want (%q,%q)", tt.eventType, tt.stripeState, disputeStatus, paymentStatus, tt.wantDispute, tt.wantPayment)
			}
			if (resolvedAt != nil) != tt.wantTime {
				t.Fatalf("resolvedAt presence mismatch: got %v want %v", resolvedAt != nil, tt.wantTime)
			}
		})
	}
}

func TestStripeResourceHelpers(t *testing.T) {
	raw := json.RawMessage(`{"id":"ch_123"}`)
	if got := stripeResourceIDFromRaw(raw, "evt_1"); got != "ch_123" {
		t.Fatalf("stripeResourceIDFromRaw returned %q", got)
	}
	if got := stripeResourceIDFromRaw(json.RawMessage(`{}`), "evt_1"); got != "evt_1" {
		t.Fatalf("stripeResourceIDFromRaw fallback returned %q", got)
	}
	if got := stripeResourceType("charge.dispute.created"); got != "charge" {
		t.Fatalf("stripeResourceType returned %q", got)
	}
	if got := firstNonEmpty("", "pi_1", "ch_1"); got != "pi_1" {
		t.Fatalf("firstNonEmpty returned %q", got)
	}
}
