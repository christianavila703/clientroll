package xerosync

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDayStringUTC(t *testing.T) {
	ts := time.Date(2026, 3, 12, 22, 30, 0, 0, time.FixedZone("AEST", 10*60*60))
	if got := dayString(ts); got != "2026-03-12" {
		t.Fatalf("unexpected day string: %s", got)
	}
}

func TestShortRef(t *testing.T) {
	id := uuid.MustParse("12345678-1234-1234-1234-1234567890ab")
	ref := shortRef("CR-", id)
	if !strings.HasPrefix(ref, "CR-") {
		t.Fatalf("missing prefix: %s", ref)
	}
	if len(ref) != len("CR-")+8 {
		t.Fatalf("expected 8-char suffix, got %s", ref)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "  ", "foo", "bar"); got != "foo" {
		t.Fatalf("expected foo, got %q", got)
	}
}
