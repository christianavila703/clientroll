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

func deriveCanonicalMembershipPricing(term *models.TermOption, frequency string) (*canonicalMembershipPricing, error) {
	if term == nil {
		return nil, fmt.Errorf("term option is required")
	}
	freq := strings.ToLower(strings.TrimSpace(frequency))
	if freq == "" {
		return nil, fmt.Errorf("payment frequency is required")
	}

	round2 := func(v float64) float64 {
		return math.Round(v*100) / 100
	}
	weeklyFallback := func() float64 {
		if term.DurationMonths <= 0 || term.FeeTotal <= 0 {
			return 0
		}
		weeks := (float64(term.DurationMonths) * 52) / 12
		if weeks <= 0 {
			return 0
		}
		return round2(term.FeeTotal / weeks)
	}
	monthlyFallback := func() float64 {
		if term.DurationMonths <= 0 || term.FeeTotal <= 0 {
			return 0
		}
		return round2(term.FeeTotal / float64(term.DurationMonths))
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
		if term.FeeWeekly != nil && *term.FeeWeekly > 0 {
			result.Amount = round2(*term.FeeWeekly)
		} else {
			result.Amount = weeklyFallback()
		}
		withInterval("week", 1)
	case "fortnightly":
		if term.FeeFortnightly != nil && *term.FeeFortnightly > 0 {
			result.Amount = round2(*term.FeeFortnightly)
		} else if weekly := weeklyFallback(); weekly > 0 {
			result.Amount = round2(weekly * 2)
		}
		withInterval("week", 2)
	case "monthly":
		if term.FeeMonthly != nil && *term.FeeMonthly > 0 {
			result.Amount = round2(*term.FeeMonthly)
		} else {
			result.Amount = monthlyFallback()
		}
		withInterval("month", 1)
	case "quarterly":
		if term.FeeQuarterly != nil && *term.FeeQuarterly > 0 {
			result.Amount = round2(*term.FeeQuarterly)
		} else if term.DurationMonths >= 3 && term.FeeTotal > 0 {
			result.Amount = round2(term.FeeTotal / (float64(term.DurationMonths) / 3))
		}
		withInterval("month", 3)
	case "custom_schedule":
		if !term.CustomScheduleEnabled {
			return nil, fmt.Errorf("custom schedule is not enabled for this term option")
		}
		if term.CustomScheduleAmount == nil || *term.CustomScheduleAmount <= 0 {
			return nil, fmt.Errorf("custom schedule amount is not configured for this term option")
		}
		if term.CustomScheduleIntervalUnit == nil || (*term.CustomScheduleIntervalUnit != "week" && *term.CustomScheduleIntervalUnit != "month") {
			return nil, fmt.Errorf("custom schedule interval is not configured for this term option")
		}
		if term.CustomScheduleIntervalCount == nil || *term.CustomScheduleIntervalCount <= 0 {
			return nil, fmt.Errorf("custom schedule interval count is not configured for this term option")
		}
		if term.CustomScheduleNumPayments == nil || *term.CustomScheduleNumPayments <= 0 {
			return nil, fmt.Errorf("custom schedule number of payments is not configured for this term option")
		}
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
