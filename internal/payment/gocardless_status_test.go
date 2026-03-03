package payment

import "testing"

func TestMapGCPaymentStatusToOurs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "pending submission", in: "pending_submission", want: "pending"},
		{name: "submitted", in: "submitted", want: "pending"},
		{name: "confirmed", in: "confirmed", want: "pending"},
		{name: "paid out", in: "paid_out", want: "succeeded"},
		{name: "failed", in: "failed", want: "failed"},
		{name: "charged back", in: "charged_back", want: "failed"},
		{name: "cancelled", in: "cancelled", want: "cancelled"},
		{name: "unknown defaults pending", in: "something_else", want: "pending"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapGCPaymentStatusToOurs(tt.in)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
