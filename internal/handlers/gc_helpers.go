package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"clientroll/internal/models"
	"clientroll/internal/payment"
	"clientroll/internal/repository"

	"github.com/google/uuid"
)

// gcSubscriptionParams converts membership payment frequency to GoCardless subscription parameters.
func gcSubscriptionParams(paymentFrequency string) (interval string, intervalCount int) {
	switch paymentFrequency {
	case "weekly":
		return "weekly", 1
	case "fortnightly":
		return "weekly", 2
	case "monthly":
		return "monthly", 1
	case "quarterly":
		return "monthly", 3
	default:
		return "monthly", 1
	}
}

func normalizeIntervalUnit(unit string) string {
	switch unit {
	case "week", "weekly":
		return "weekly"
	case "month", "monthly":
		return "monthly"
	default:
		return ""
	}
}

// subscriptionEndDateFromNumPayments computes end date when num_payments is set:
// end = membership_start + (num_payments * period), so schedules align to term start.
func subscriptionEndDateFromNumPayments(membershipStart time.Time, interval string, intervalCount, numPayments int) time.Time {
	anchor := membershipStart
	if anchor.IsZero() {
		anchor = time.Now()
	}
	switch interval {
	case "weekly":
		return anchor.AddDate(0, 0, 7*intervalCount*numPayments)
	case "monthly":
		return anchor.AddDate(0, intervalCount*numPayments, 0)
	default:
		return anchor.AddDate(0, intervalCount*numPayments, 0)
	}
}

// createGoCardlessSubscription creates a GoCardless subscription for a membership after mandate setup.
// If numPayments is set, the subscription end_date is computed so that exactly that many payments are taken.
// Returns (subscriptionID, error). Logs errors but doesn't fail the caller.
func createGoCardlessSubscription(
	ctx context.Context,
	prov payment.GoCardlessExtended,
	memberRepo *repository.MembershipRepo,
	mandateID string,
	membership interface{ GetID() uuid.UUID },
	tenantID uuid.UUID,
	paymentFrequency string,
	paymentAmount float64,
	paymentIntervalUnit *string,
	paymentIntervalCount int,
	membershipName string,
	membershipStart time.Time,
	endDate time.Time,
	numPayments *int,
	continueAfterTerm bool,
) (string, error) {
	if paymentFrequency == "upfront" || paymentFrequency == "" {
		return "", nil // No subscription needed for upfront payments
	}
	if paymentFrequency == "custom_schedule" && (paymentIntervalUnit == nil || *paymentIntervalUnit == "") {
		// A custom_schedule membership without an interval is a dated
		// split-payment plan; falling through would bill the plan TOTAL on the
		// default monthly cadence. Callers must use
		// createGoCardlessInstalmentSchedule for these.
		return "", fmt.Errorf("membership %s uses a dated instalment plan; refusing to create a recurring subscription", membership.GetID())
	}

	interval := ""
	intervalCount := paymentIntervalCount
	if paymentIntervalUnit != nil && *paymentIntervalUnit != "" {
		interval = normalizeIntervalUnit(*paymentIntervalUnit)
	}
	if interval == "" {
		interval, intervalCount = gcSubscriptionParams(paymentFrequency)
	}
	if intervalCount <= 0 {
		intervalCount = 1
	}
	amountMinor := int64(paymentAmount * 100)
	if amountMinor <= 0 {
		return "", nil
	}

	var ed *time.Time
	if numPayments != nil && *numPayments > 0 {
		computed := subscriptionEndDateFromNumPayments(membershipStart, interval, intervalCount, *numPayments)
		ed = &computed
	} else if !continueAfterTerm && !endDate.IsZero() {
		ed = &endDate
	}

	membershipID := membership.GetID()
	subID, err := prov.CreateSubscription(ctx, mandateID, amountMinor, "AUD", interval, intervalCount, membershipName, membershipID.String(), ed)
	if err != nil {
		log.Printf("GoCardless CreateSubscription for membership %s: %v", membershipID, err)
		return "", err
	}

	if memberRepo != nil {
		_ = memberRepo.SetGoCardlessSubscriptionID(ctx, membershipID, tenantID, subID)
	}
	log.Printf("GoCardless subscription %s created for membership %s (mandate %s)", subID, membershipID, mandateID)
	return subID, nil
}

// createGoCardlessInstalmentSchedule creates a native GoCardless instalment
// schedule from a dated split-payment plan (all lines, including day 0 —
// GoCardless charges it on the next available collection date).
func createGoCardlessInstalmentSchedule(
	ctx context.Context,
	prov payment.GoCardlessExtended,
	memberRepo *repository.MembershipRepo,
	mandateID string,
	membership interface{ GetID() uuid.UUID },
	tenantID uuid.UUID,
	lines []models.InstalmentLine,
	name string,
	currencyCode string,
	membershipStart time.Time,
) (string, error) {
	if len(lines) == 0 {
		return "", fmt.Errorf("instalment plan has no lines")
	}
	anchor := membershipStart
	now := time.Now().UTC()
	if anchor.IsZero() || anchor.Before(now) {
		anchor = now
	}
	total := int64(0)
	instalments := make([]payment.InstalmentDate, 0, len(lines))
	for _, l := range lines {
		amountMinor := int64(l.Amount * 100)
		if amountMinor <= 0 {
			return "", fmt.Errorf("instalment line amount must be positive")
		}
		total += amountMinor
		chargeDate := anchor.AddDate(0, 0, l.OffsetDays)
		if chargeDate.Before(now) {
			chargeDate = now
		}
		instalments = append(instalments, payment.InstalmentDate{
			ChargeDate: chargeDate.Format("2006-01-02"),
			Amount:     amountMinor,
		})
	}
	if currencyCode == "" {
		currencyCode = "AUD"
	}
	membershipID := membership.GetID()
	scheduleID, err := prov.CreateInstalmentSchedule(ctx, mandateID, total, currencyCode, instalments, name, membershipID.String())
	if err != nil {
		log.Printf("GoCardless CreateInstalmentSchedule for membership %s: %v", membershipID, err)
		return "", err
	}
	log.Printf("GoCardless instalment schedule %s created for membership %s (mandate %s, %d instalments)", scheduleID, membershipID, mandateID, len(instalments))
	return scheduleID, nil
}
