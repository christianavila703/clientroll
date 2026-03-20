package handlers

import (
	"context"
	"log"
	"time"

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

// subscriptionEndDateFromNumPayments computes the subscription end date when num_payments is set:
// end = now + (num_payments * period). GoCardless will stop after that date.
func subscriptionEndDateFromNumPayments(interval string, intervalCount, numPayments int) time.Time {
	now := time.Now()
	switch interval {
	case "weekly":
		return now.AddDate(0, 0, 7*intervalCount*numPayments)
	case "monthly":
		return now.AddDate(0, intervalCount*numPayments, 0)
	default:
		return now.AddDate(0, intervalCount*numPayments, 0)
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
	endDate time.Time,
	numPayments *int,
	continueAfterTerm bool,
) (string, error) {
	if paymentFrequency == "upfront" || paymentFrequency == "" {
		return "", nil // No subscription needed for upfront payments
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
		computed := subscriptionEndDateFromNumPayments(interval, intervalCount, *numPayments)
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
