package handlers

import (
	"strconv"
	"testing"
	"time"
)

func TestIsFreshTimestamp(t *testing.T) {
	now := time.Now().Unix()

	if !isFreshTimestamp(strconv.FormatInt(now, 10), 300) {
		t.Fatalf("expected current timestamp to be fresh")
	}
	if !isFreshTimestamp(strconv.FormatInt(now-299, 10), 300) {
		t.Fatalf("expected near-past timestamp to be fresh")
	}
	if isFreshTimestamp(strconv.FormatInt(now-301, 10), 300) {
		t.Fatalf("expected stale timestamp to be rejected")
	}
	if isFreshTimestamp("not-a-number", 300) {
		t.Fatalf("expected invalid timestamp to be rejected")
	}
}
