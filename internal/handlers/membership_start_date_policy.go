package handlers

import (
	"fmt"
	"strings"
	"time"
)

const minFutureMembershipStartDays = 2

// validateMinimumFutureStartDate enforces a minimum lead window for future-dated memberships.
// Same-day and past dates are allowed; only future dates are validated.
func validateMinimumFutureStartDate(startDate time.Time, tenantTimezone string, minDays int) error {
	if minDays <= 0 {
		return nil
	}
	loc := time.UTC
	if tz := strings.TrimSpace(tenantTimezone); tz != "" {
		if loaded, err := time.LoadLocation(tz); err == nil {
			loc = loaded
		}
	}
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	start := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, loc)
	if !start.After(today) {
		return nil
	}
	minAllowed := today.AddDate(0, 0, minDays)
	if start.Before(minAllowed) {
		return fmt.Errorf("Start date must be at least %d days in the future", minDays)
	}
	return nil
}
