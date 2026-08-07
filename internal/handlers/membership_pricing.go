package handlers

import (
	"fmt"
	"math"
	"strings"

	"clientroll/internal/models"
)

type canonicalMembershipPricing struct {
	Frequency     string
	Amount        float64
	IntervalUnit  *string
	IntervalCount int
	NumPayments   *int
}

func termOffersPaymentFrequency(term *models.TermOption, frequency string) bool {
	if term == nil {
		return false
	}
	freq := strings.ToLower(strings.TrimSpace(frequency))
	switch freq {
	case "upfront":
		return term.AllowUpfront && term.FeeTotal > 0
	case "weekly":
		return term.FeeWeekly != nil && *term.FeeWeekly > 0
	case "fortnightly":
		return term.FeeFortnightly != nil && *term.FeeFortnightly > 0
	case "monthly":
		return term.FeeMonthly != nil && *term.FeeMonthly > 0
	case "quarterly":
		return term.FeeQuarterly != nil && *term.FeeQuarterly > 0
	case "custom_schedule":
		return term.CustomScheduleEnabled &&
			term.CustomScheduleAmount != nil && *term.CustomScheduleAmount > 0 &&
			term.CustomScheduleIntervalUnit != nil &&
			term.CustomScheduleIntervalCount != nil && *term.CustomScheduleIntervalCount > 0 &&
			term.CustomScheduleNumPayments != nil && *term.CustomScheduleNumPayments > 0
	default:
		return false
	}
}

func deriveCanonicalMembershipPricing(term *models.TermOption, frequency string) (*canonicalMembershipPricing, error) {
	if term == nil {
		return nil, fmt.Errorf("term option is required")
	}
	freq := strings.ToLower(strings.TrimSpace(frequency))
	if freq == "" {
		return nil, fmt.Errorf("payment frequency is required")
	}
	if !termOffersPaymentFrequency(term, freq) {
		return nil, fmt.Errorf("payment frequency %q is not available for this term option", freq)
	}

	round2 := func(v float64) float64 {
		return math.Round(v*100) / 100
	}

	result := &canonicalMembershipPricing{
		Frequency:     freq,
		IntervalCount: 1,
		NumPayments:   term.NumPayments,
	}

	withInterval := func(unit string, count int) {
		u := unit
		result.IntervalUnit = &u
		result.IntervalCount = count
	}

	switch freq {
	case "upfront":
		result.Amount = round2(term.FeeTotal)
		result.IntervalUnit = nil
		result.IntervalCount = 1
		return result, nil
	case "weekly":
		result.Amount = round2(*term.FeeWeekly)
		withInterval("week", 1)
	case "fortnightly":
		result.Amount = round2(*term.FeeFortnightly)
		withInterval("week", 2)
	case "monthly":
		result.Amount = round2(*term.FeeMonthly)
		withInterval("month", 1)
	case "quarterly":
		result.Amount = round2(*term.FeeQuarterly)
		withInterval("month", 3)
	case "custom_schedule":
		result.Amount = round2(*term.CustomScheduleAmount)
		result.IntervalUnit = term.CustomScheduleIntervalUnit
		result.IntervalCount = *term.CustomScheduleIntervalCount
		result.NumPayments = term.CustomScheduleNumPayments
	default:
		return nil, fmt.Errorf("unsupported payment frequency")
	}

	if result.Amount <= 0 {
		return nil, fmt.Errorf("payment amount is not configured for selected frequency")
	}
	return result, nil
}

// derivePricingFromSchedule converts a stored PaymentSchedule row into the
// same canonical pricing shape used downstream. Returned `frequency` is the
// legacy string ('upfront', 'weekly', 'fortnightly', 'monthly', 'quarterly',
// or 'custom_schedule') so existing rules (CHECK constraints, snapshot fields,
// invite payloads) keep working during the v2 soak.
func derivePricingFromSchedule(s *models.PaymentSchedule) (*canonicalMembershipPricing, error) {
	if s == nil {
		return nil, fmt.Errorf("payment schedule is required")
	}
	round2 := func(v float64) float64 { return math.Round(v*100) / 100 }
	result := &canonicalMembershipPricing{
		Amount:      round2(s.Amount),
		NumPayments: s.NumPayments,
	}
	switch s.Kind {
	case "upfront":
		result.Frequency = "upfront"
		result.IntervalCount = 1
	case "recurring":
		if s.IntervalUnit == nil || s.IntervalCount == nil {
			return nil, fmt.Errorf("recurring schedule missing interval")
		}
		unit := *s.IntervalUnit
		count := *s.IntervalCount
		result.IntervalUnit = &unit
		result.IntervalCount = count
		switch {
		case unit == "week" && count == 1:
			result.Frequency = "weekly"
		case unit == "week" && count == 2:
			result.Frequency = "fortnightly"
		case unit == "month" && count == 1:
			result.Frequency = "monthly"
		case unit == "month" && count == 3:
			result.Frequency = "quarterly"
		default:
			// Non-standard recurring cadence — fall back to the catch-all
			// 'custom_schedule' frequency so legacy CHECKs accept it.
			result.Frequency = "custom_schedule"
		}
	case "instalments":
		if len(s.Instalments) > 0 {
			// Dated-lines plan: no interval; amount is the plan total and
			// num_payments is the line count (both derived at save time).
			// The provider-side schedule is built from the lines directly.
			result.Frequency = "custom_schedule"
			n := len(s.Instalments)
			result.NumPayments = &n
			result.IntervalCount = 1
			break
		}
		if s.IntervalUnit == nil || s.IntervalCount == nil {
			return nil, fmt.Errorf("instalments schedule missing interval")
		}
		unit := *s.IntervalUnit
		count := *s.IntervalCount
		result.IntervalUnit = &unit
		result.IntervalCount = count
		result.Frequency = "custom_schedule"
	default:
		return nil, fmt.Errorf("unsupported schedule kind %q", s.Kind)
	}
	if result.Amount <= 0 {
		return nil, fmt.Errorf("schedule amount is not configured")
	}
	return result, nil
}
