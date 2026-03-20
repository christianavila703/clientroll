package handlers

import (
	"math"
	"testing"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.0001
}

func TestSplitAmountByTaxMode_GSTOnInclusive(t *testing.T) {
	base, tax := splitAmountByTaxMode(999, 10, true)
	if !almostEqual(base, 908.18) {
		t.Fatalf("expected base 908.18, got %.2f", base)
	}
	if !almostEqual(tax, 90.82) {
		t.Fatalf("expected tax 90.82, got %.2f", tax)
	}
}

func TestSplitAmountByTaxMode_GSTOff(t *testing.T) {
	base, tax := splitAmountByTaxMode(999, 10, false)
	if !almostEqual(base, 999) {
		t.Fatalf("expected base 999.00, got %.2f", base)
	}
	if !almostEqual(tax, 0) {
		t.Fatalf("expected tax 0.00, got %.2f", tax)
	}
}

func TestPaymentComponentsByTaxMode_WithFee(t *testing.T) {
	components := paymentComponentsByTaxMode(1028, 29, 10, false)
	if !almostEqual(components.BaseAmount, 999) {
		t.Fatalf("expected base 999.00, got %.2f", components.BaseAmount)
	}
	if !almostEqual(components.TaxAmount, 0) {
		t.Fatalf("expected tax 0.00, got %.2f", components.TaxAmount)
	}
	if !almostEqual(components.ProcessingFeeAmount, 29) {
		t.Fatalf("expected fee 29.00, got %.2f", components.ProcessingFeeAmount)
	}
}
