package handlers

import (
	"context"
	"time"

	"clientroll/internal/repository"

	"github.com/google/uuid"
)

// syncMembershipStatusAfterPayment keeps membership lifecycle aligned with payment outcomes.
// It is intentionally conservative: cancelled/expired memberships are never auto-mutated.
func syncMembershipStatusAfterPayment(
	ctx context.Context,
	memberRepo *repository.MembershipRepo,
	tenantID uuid.UUID,
	membershipID uuid.UUID,
	paymentStatus string,
	now time.Time,
) error {
	if memberRepo == nil || membershipID == uuid.Nil {
		return nil
	}
	m, err := memberRepo.FindByID(ctx, membershipID, tenantID)
	if err != nil || m == nil {
		return err
	}
	if m.Status == "cancelled" || m.Status == "expired" {
		return nil
	}

	// Future-start memberships stay scheduled even if setup/early provider events occur.
	isFutureStart := m.StartDate.After(now.UTC())
	targetStatus := ""

	switch paymentStatus {
	case "succeeded":
		if isFutureStart {
			targetStatus = "scheduled"
		} else {
			targetStatus = "active"
		}
	case "failed", "disputed":
		if isFutureStart {
			targetStatus = "scheduled"
		} else {
			targetStatus = "past_due"
		}
	default:
		return nil
	}
	if targetStatus == "" || targetStatus == m.Status {
		return nil
	}
	return memberRepo.UpdateStatus(ctx, membershipID, tenantID, targetStatus)
}
