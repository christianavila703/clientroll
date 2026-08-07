package repository

import (
	"strings"
	"testing"
)

func strPtr(s string) *string { return &s }

func TestValidateScheduleInputs_DatedInstalments(t *testing.T) {
	valid := PaymentScheduleInput{
		Kind: "instalments",
		Instalments: []InstalmentLineInput{
			{Amount: 5000, OffsetDays: 0},
			{Amount: 8500, OffsetDays: 14},
		},
	}
	if err := ValidateScheduleInputs([]PaymentScheduleInput{valid}); err != nil {
		t.Fatalf("valid dated plan rejected: %v", err)
	}

	cases := []struct {
		name    string
		mutate  func(in *PaymentScheduleInput)
		wantErr string
	}{
		{
			name: "first line must be day 0",
			mutate: func(in *PaymentScheduleInput) {
				in.Instalments[0].OffsetDays = 3
			},
			wantErr: "day 0",
		},
		{
			name: "offsets strictly increasing",
			mutate: func(in *PaymentScheduleInput) {
				in.Instalments[1].OffsetDays = 0
			},
			wantErr: "strictly increasing",
		},
		{
			name: "line amounts positive",
			mutate: func(in *PaymentScheduleInput) {
				in.Instalments[1].Amount = 0
			},
			wantErr: "must be > 0",
		},
		{
			name: "no interval alongside dated lines",
			mutate: func(in *PaymentScheduleInput) {
				in.IntervalUnit = strPtr("week")
			},
			wantErr: "must not carry an interval",
		},
		{
			name: "lines only valid on instalments kind",
			mutate: func(in *PaymentScheduleInput) {
				in.Kind = "recurring"
				in.IntervalUnit = strPtr("week")
				count := 1
				in.IntervalCount = &count
			},
			wantErr: "only valid with kind 'instalments'",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := valid
			in.Instalments = append([]InstalmentLineInput(nil), valid.Instalments...)
			tc.mutate(&in)
			err := ValidateScheduleInputs([]PaymentScheduleInput{in})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestNormalizeDatedInstalmentInput(t *testing.T) {
	unit := "week"
	count := 2
	in := PaymentScheduleInput{
		Kind:          "instalments",
		Amount:        1, // stale client value: overwritten by the line sum
		IntervalUnit:  &unit,
		IntervalCount: &count,
		Instalments: []InstalmentLineInput{
			{Amount: 5000, OffsetDays: 0},
			{Amount: 8500, OffsetDays: 14},
		},
	}
	normalizeDatedInstalmentInput(&in)
	if in.Amount != 13500 {
		t.Errorf("amount = %v, want 13500", in.Amount)
	}
	if in.NumPayments == nil || *in.NumPayments != 2 {
		t.Errorf("num_payments = %v, want 2", in.NumPayments)
	}
	if in.IntervalUnit != nil || in.IntervalCount != nil {
		t.Errorf("interval fields should be cleared, got %v %v", in.IntervalUnit, in.IntervalCount)
	}

	// Interval-based instalments stay untouched.
	legacy := PaymentScheduleInput{Kind: "instalments", Amount: 100, IntervalUnit: &unit, IntervalCount: &count}
	normalizeDatedInstalmentInput(&legacy)
	if legacy.Amount != 100 || legacy.IntervalUnit == nil {
		t.Errorf("legacy instalments input was modified: %+v", legacy)
	}
}
