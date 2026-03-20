package handlers

import (
	"testing"

	"clientroll/internal/models"
)

func TestDeriveCanonicalMembershipPricing_WeeklyFromConfiguredFee(t *testing.T) {
	weekly := 25.0
	term := &models.TermOption{
		DurationMonths: 12,
		FeeTotal:       1200,
		FeeWeekly:      &weekly,
	}
	got, err := deriveCanonicalMembershipPricing(term, "weekly")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Amount != 25.0 {
		t.Fatalf("expected weekly amount 25.0, got %.2f", got.Amount)
	}
	if got.IntervalUnit == nil || *got.IntervalUnit != "week" || got.IntervalCount != 1 {
		t.Fatalf("expected weekly interval week/1, got unit=%v count=%d", got.IntervalUnit, got.IntervalCount)
	}
}

func TestDeriveCanonicalMembershipPricing_CustomScheduleConfigured(t *testing.T) {
	amt := 99.0
	unit := "month"
	count := 2
	numPayments := 6
	term := &models.TermOption{
		CustomScheduleEnabled:       true,
		CustomScheduleAmount:        &amt,
		CustomScheduleIntervalUnit:  &unit,
		CustomScheduleIntervalCount: &count,
		CustomScheduleNumPayments:   &numPayments,
	}
	got, err := deriveCanonicalMembershipPricing(term, "custom_schedule")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Amount != 99.0 {
		t.Fatalf("expected custom amount 99.0, got %.2f", got.Amount)
	}
	if got.IntervalUnit == nil || *got.IntervalUnit != "month" || got.IntervalCount != 2 {
		t.Fatalf("expected custom interval month/2, got unit=%v count=%d", got.IntervalUnit, got.IntervalCount)
	}
	if got.NumPayments == nil || *got.NumPayments != 6 {
		t.Fatalf("expected custom num_payments=6, got %v", got.NumPayments)
	}
}

func TestDeriveCanonicalMembershipPricing_CustomScheduleDisabled(t *testing.T) {
	term := &models.TermOption{}
	_, err := deriveCanonicalMembershipPricing(term, "custom_schedule")
	if err == nil {
		t.Fatal("expected error when custom schedule is disabled")
	}
}

