package handlers

import (
	"os"
	"strings"
	"testing"
)

func TestSignupAndStaffCreationUseCanonicalPricing(t *testing.T) {
	signupSrc, err := os.ReadFile("signup.go")
	if err != nil {
		t.Fatalf("read signup.go: %v", err)
	}
	activeSrc, err := os.ReadFile("active_memberships.go")
	if err != nil {
		t.Fatalf("read active_memberships.go: %v", err)
	}
	for _, src := range []string{string(signupSrc), string(activeSrc)} {
		if !strings.Contains(src, "deriveCanonicalMembershipPricing(") {
			t.Fatal("expected canonical pricing derivation in both signup and active membership creation handlers")
		}
		if !strings.Contains(src, "payment_amount does not match selected term option") {
			t.Fatal("expected request mismatch validation for payment_amount")
		}
	}
}

func TestStripeWebhookMarksProcessedAfterHandlerSuccess(t *testing.T) {
	src, err := os.ReadFile("stripe_webhooks.go")
	if err != nil {
		t.Fatalf("read stripe_webhooks.go: %v", err)
	}
	s := string(src)
	handleCheck := strings.Index(s, "if handleErr != nil")
	markProcessed := strings.Index(s, "MarkProcessed(")
	if handleCheck < 0 || markProcessed < 0 {
		t.Fatal("expected handleErr check and MarkProcessed call in stripe webhook")
	}
	if markProcessed < handleCheck {
		t.Fatal("expected MarkProcessed to execute only after handler error check")
	}
}

func TestMembershipExpectedTotalFunctionUsedAcrossHandlers(t *testing.T) {
	files := []string{"active_memberships.go", "clients.go", "payments.go"}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if !strings.Contains(string(src), "membership_expected_total(") {
			t.Fatalf("expected %s to use membership_expected_total SQL function", f)
		}
	}
}

