package xero

import (
	"testing"
	"time"
)

func TestRetryAfterDefaultsWhenHeaderMissing(t *testing.T) {
	got := retryAfter("")
	if got != 1500*time.Millisecond {
		t.Fatalf("expected default 1500ms, got %v", got)
	}
}

func TestRetryAfterParsesSeconds(t *testing.T) {
	got := retryAfter("3")
	if got != 3*time.Second {
		t.Fatalf("expected 3s, got %v", got)
	}
}
