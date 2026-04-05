package observability

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/repository"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/invoice"
	"github.com/stripe/stripe-go/v82/subscription"
)

// StripeReconciler verifies local membership/payment state against the Stripe API.
type StripeReconciler struct {
	db          *database.Postgres
	encryptor   *database.Encryptor
	credRepo    *repository.TenantCredentialRepo
	paymentRepo *repository.PaymentRepo
	memberRepo  *repository.MembershipRepo
	reconRepo   *repository.ReconciliationRepo
}

func NewStripeReconciler(
	db *database.Postgres,
	enc *database.Encryptor,
	credRepo *repository.TenantCredentialRepo,
	paymentRepo *repository.PaymentRepo,
	memberRepo *repository.MembershipRepo,
	reconRepo *repository.ReconciliationRepo,
) *StripeReconciler {
	return &StripeReconciler{
		db:          db,
		encryptor:   enc,
		credRepo:    credRepo,
		paymentRepo: paymentRepo,
		memberRepo:  memberRepo,
		reconRepo:   reconRepo,
	}
}

type membershipStripeInfo struct {
	MembershipID        uuid.UUID
	ClientID            uuid.UUID
	ClientName          string
	MembershipTitle     string
	MembershipStatus    string
	StripeSubID         string
	LatestPaymentStatus string
	PaymentAmount       float64
}

// RunForTenant performs the full Stripe reconciliation sweep for a single tenant.
func (sr *StripeReconciler) RunForTenant(ctx context.Context, tenantID uuid.UUID) (*repository.ReconciliationRun, error) {
	run, err := sr.reconRepo.Create(ctx, tenantID, "daily")
	if err != nil {
		return nil, fmt.Errorf("create reconciliation run: %w", err)
	}

	apiKey, err := sr.getStripeKey(ctx, tenantID)
	if err != nil || apiKey == "" {
		log.Printf("stripe_reconciler no_api_key tenant=%s", tenantID)
		_ = sr.reconRepo.Complete(ctx, run.ID, 0, 0, 0, 0, nil)
		return run, nil
	}

	memberships, err := sr.getMembershipsWithStripe(ctx, tenantID)
	if err != nil {
		log.Printf("stripe_reconciler list_memberships failed tenant=%s err=%v", tenantID, err)
		_ = sr.reconRepo.Complete(ctx, run.ID, 0, 0, 0, 0, nil)
		return run, err
	}

	var mismatches []repository.ReconciliationMismatch
	checked, matched := 0, 0

	for _, m := range memberships {
		checked++

		sub, subErr := sr.getSubscription(apiKey, m.StripeSubID)
		if subErr != nil {
			log.Printf("stripe_reconciler sub_get failed tenant=%s sub=%s err=%v", tenantID, m.StripeSubID, subErr)
			continue
		}
		if sub == nil {
			mismatches = append(mismatches, repository.ReconciliationMismatch{
				MembershipID:      m.MembershipID,
				ClientID:          m.ClientID,
				ClientName:        m.ClientName,
				MembershipTitle:   m.MembershipTitle,
				LocalStatus:       m.MembershipStatus,
				ProviderStatus:    "not_found",
				ProviderSubID:     m.StripeSubID,
				Severity:          "critical",
				Category:          "subscription_missing",
				RecommendedAction: "Verify subscription in Stripe dashboard",
			})
			continue
		}

		stripeStatus := string(sub.Status)
		mismatch := sr.compareStatuses(m, stripeStatus, sub)
		if mismatch != nil {
			autoFixed := sr.attemptAutoFix(ctx, tenantID, m, stripeStatus, sub)
			mismatch.AutoFixed = autoFixed
			mismatches = append(mismatches, *mismatch)
		} else {
			matched++
		}

		if m.PaymentAmount > 0 && sub.LatestInvoice != nil {
			if amtMismatch := sr.checkInvoiceAmount(m, sub.LatestInvoice); amtMismatch != nil {
				mismatches = append(mismatches, *amtMismatch)
			}
		}
	}

	autoFixed := 0
	for _, mm := range mismatches {
		if mm.AutoFixed {
			autoFixed++
		}
	}

	if err := sr.reconRepo.Complete(ctx, run.ID, checked, matched, len(mismatches), autoFixed, mismatches); err != nil {
		log.Printf("stripe_reconciler complete_run failed tenant=%s err=%v", tenantID, err)
	}

	run.TotalChecked = checked
	run.TotalMatched = matched
	run.TotalMismatched = len(mismatches)
	run.TotalAutoFixed = autoFixed
	run.Details = mismatches
	now := time.Now().UTC()
	run.CompletedAt = &now

	log.Printf("stripe_reconciler completed tenant=%s checked=%d matched=%d mismatched=%d auto_fixed=%d",
		tenantID, checked, matched, len(mismatches), autoFixed)
	return run, nil
}

func (sr *StripeReconciler) getStripeKey(ctx context.Context, tenantID uuid.UUID) (string, error) {
	creds, err := sr.credRepo.GetByProvider(ctx, tenantID, "stripe")
	if err != nil || creds == nil || !creds.IsActive || creds.APIKeyEncrypted == "" {
		return "", nil
	}
	key, err := sr.encryptor.Decrypt(creds.APIKeyEncrypted)
	if err != nil {
		return "", fmt.Errorf("decrypt stripe key: %w", err)
	}
	return key, nil
}

func (sr *StripeReconciler) getMembershipsWithStripe(ctx context.Context, tenantID uuid.UUID) ([]membershipStripeInfo, error) {
	rows, err := sr.db.Pool.Query(ctx, `
		SELECT DISTINCT ON (m.id)
			m.id, m.client_id, c.name, mt.title, m.status,
			p.stripe_details->>'stripe_subscription_id' AS stripe_sub_id,
			p.status AS payment_status,
			COALESCE(m.payment_amount, 0)
		FROM memberships m
		JOIN clients c ON c.id = m.client_id AND c.tenant_id = m.tenant_id
		JOIN membership_types mt ON mt.id = m.membership_type_id AND mt.tenant_id = m.tenant_id
		JOIN payments p ON p.tenant_id = m.tenant_id AND p.membership_id = m.id
		WHERE m.tenant_id = $1
		  AND m.status NOT IN ('cancelled', 'expired')
		  AND COALESCE(p.stripe_details->>'stripe_subscription_id', '') <> ''
		ORDER BY m.id, p.created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("query stripe memberships: %w", err)
	}
	defer rows.Close()

	var items []membershipStripeInfo
	for rows.Next() {
		var m membershipStripeInfo
		if err := rows.Scan(&m.MembershipID, &m.ClientID, &m.ClientName, &m.MembershipTitle, &m.MembershipStatus, &m.StripeSubID, &m.LatestPaymentStatus, &m.PaymentAmount); err != nil {
			return nil, fmt.Errorf("scan membership stripe info: %w", err)
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

func (sr *StripeReconciler) getSubscription(apiKey, subID string) (*stripe.Subscription, error) {
	stripe.Key = apiKey
	sub, err := subscription.Get(subID, nil)
	if err != nil {
		stripeErr, ok := err.(*stripe.Error)
		if ok && stripeErr.HTTPStatusCode == 404 {
			return nil, nil
		}
		return nil, err
	}
	return sub, nil
}

func (sr *StripeReconciler) compareStatuses(m membershipStripeInfo, stripeStatus string, sub *stripe.Subscription) *repository.ReconciliationMismatch {
	localActive := m.MembershipStatus == "active" || m.MembershipStatus == "scheduled"
	localPastDue := m.MembershipStatus == "past_due"

	switch stripeStatus {
	case "active", "trialing":
		if localActive {
			return nil // match
		}
		if localPastDue {
			return &repository.ReconciliationMismatch{
				MembershipID:      m.MembershipID,
				ClientID:          m.ClientID,
				ClientName:        m.ClientName,
				MembershipTitle:   m.MembershipTitle,
				LocalStatus:       m.MembershipStatus,
				ProviderStatus:    stripeStatus,
				ProviderSubID:     m.StripeSubID,
				Severity:          "warning",
				Category:          "missed_success",
				RecommendedAction: "Stripe says active but we show past_due — may have missed a success webhook",
			}
		}
		return nil

	case "past_due":
		if localPastDue {
			return nil // match
		}
		if localActive {
			return &repository.ReconciliationMismatch{
				MembershipID:      m.MembershipID,
				ClientID:          m.ClientID,
				ClientName:        m.ClientName,
				MembershipTitle:   m.MembershipTitle,
				LocalStatus:       m.MembershipStatus,
				ProviderStatus:    stripeStatus,
				ProviderSubID:     m.StripeSubID,
				Severity:          "critical",
				Category:          "missed_failure",
				RecommendedAction: "Stripe says past_due but we show active — missed failure webhook",
			}
		}
		return nil

	case "canceled", "cancelled":
		if localActive || localPastDue {
			return &repository.ReconciliationMismatch{
				MembershipID:      m.MembershipID,
				ClientID:          m.ClientID,
				ClientName:        m.ClientName,
				MembershipTitle:   m.MembershipTitle,
				LocalStatus:       m.MembershipStatus,
				ProviderStatus:    stripeStatus,
				ProviderSubID:     m.StripeSubID,
				Severity:          "critical",
				Category:          "cancelled_at_provider",
				RecommendedAction: "Subscription cancelled in Stripe but still active locally",
			}
		}
		return nil

	case "unpaid", "incomplete_expired":
		if localActive {
			return &repository.ReconciliationMismatch{
				MembershipID:      m.MembershipID,
				ClientID:          m.ClientID,
				ClientName:        m.ClientName,
				MembershipTitle:   m.MembershipTitle,
				LocalStatus:       m.MembershipStatus,
				ProviderStatus:    stripeStatus,
				ProviderSubID:     m.StripeSubID,
				Severity:          "critical",
				Category:          "missed_failure",
				RecommendedAction: "Stripe says " + stripeStatus + " but we show active",
			}
		}
		return nil

	default:
		return nil
	}
}

func (sr *StripeReconciler) checkInvoiceAmount(m membershipStripeInfo, inv *stripe.Invoice) *repository.ReconciliationMismatch {
	stripeAmount := float64(inv.AmountPaid) / 100.0
	if stripeAmount == 0 {
		stripeAmount = float64(inv.AmountDue) / 100.0
	}
	if stripeAmount == 0 {
		return nil
	}
	diff := m.PaymentAmount - stripeAmount
	if diff < 0 {
		diff = -diff
	}
	if diff < 0.01 {
		return nil
	}
	return &repository.ReconciliationMismatch{
		MembershipID:      m.MembershipID,
		ClientID:          m.ClientID,
		ClientName:        m.ClientName,
		MembershipTitle:   m.MembershipTitle,
		LocalStatus:       fmt.Sprintf("$%.2f", m.PaymentAmount),
		ProviderStatus:    fmt.Sprintf("$%.2f", stripeAmount),
		ProviderSubID:     m.StripeSubID,
		Severity:          "warning",
		Category:          "amount_mismatch",
		RecommendedAction: fmt.Sprintf("Local amount $%.2f vs Stripe invoice $%.2f", m.PaymentAmount, stripeAmount),
	}
}

// attemptAutoFix applies safe-direction corrections only.
func (sr *StripeReconciler) attemptAutoFix(ctx context.Context, tenantID uuid.UUID, m membershipStripeInfo, stripeStatus string, sub *stripe.Subscription) bool {
	switch {
	case (stripeStatus == "active" || stripeStatus == "trialing") && m.MembershipStatus == "past_due":
		// Stripe says paid/active, we say past_due — safe to fix to active
		_, err := sr.db.Pool.Exec(ctx,
			`UPDATE memberships SET status = 'active', updated_at = NOW() WHERE id = $1 AND tenant_id = $2`,
			m.MembershipID, tenantID)
		if err != nil {
			log.Printf("stripe_reconciler auto_fix_active failed membership=%s err=%v", m.MembershipID, err)
			return false
		}

		// Also try to reconcile the latest invoice as succeeded
		sr.reconcileLatestInvoice(ctx, tenantID, m, sub)
		log.Printf("stripe_reconciler auto_fixed_active tenant=%s membership=%s stripe_status=%s", tenantID, m.MembershipID, stripeStatus)
		return true

	case stripeStatus == "past_due" && m.MembershipStatus == "active":
		// Stripe says past_due, we say active — safe to fix to past_due
		_, err := sr.db.Pool.Exec(ctx,
			`UPDATE memberships SET status = 'past_due', updated_at = NOW() WHERE id = $1 AND tenant_id = $2`,
			m.MembershipID, tenantID)
		if err != nil {
			log.Printf("stripe_reconciler auto_fix_past_due failed membership=%s err=%v", m.MembershipID, err)
			return false
		}
		log.Printf("stripe_reconciler auto_fixed_past_due tenant=%s membership=%s stripe_status=%s", tenantID, m.MembershipID, stripeStatus)
		return true
	}

	// Never auto-cancel — requires human review
	return false
}

func (sr *StripeReconciler) reconcileLatestInvoice(ctx context.Context, tenantID uuid.UUID, m membershipStripeInfo, sub *stripe.Subscription) {
	if sub.LatestInvoice == nil {
		return
	}
	invoiceID := sub.LatestInvoice.ID
	if invoiceID == "" || !strings.HasPrefix(invoiceID, "in_") {
		return
	}

	apiKey, _ := sr.getStripeKey(ctx, tenantID)
	if apiKey == "" {
		return
	}
	stripe.Key = apiKey

	inv, err := invoice.Get(invoiceID, nil)
	if err != nil || inv == nil {
		return
	}
	if inv.Status != stripe.InvoiceStatusPaid {
		return
	}

	p, _ := sr.paymentRepo.GetByProviderReference(ctx, tenantID, invoiceID)
	if p != nil && p.Status != "succeeded" {
		now := time.Now().UTC()
		if err := sr.paymentRepo.UpdateStatus(ctx, p.ID, tenantID, "succeeded", &now, nil, nil); err != nil {
			log.Printf("stripe_reconciler invoice_reconcile update_status failed payment=%s err=%v", p.ID, err)
		}
	}
}

// StartStripeReconciliationScheduler runs the Stripe reconciliation daily for all configured tenants.
func StartStripeReconciliationScheduler(ctx context.Context, sr *StripeReconciler, alertConfigRepo *repository.AlertConfigRepo) {
	if sr == nil || alertConfigRepo == nil {
		log.Printf("stripe_reconciler.scheduler skipped (missing dependencies)")
		return
	}
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	log.Printf("stripe_reconciler.scheduler started")

	lastRunDate := ""
	for {
		select {
		case <-ctx.Done():
			log.Printf("stripe_reconciler.scheduler stopped")
			return
		case <-ticker.C:
			today := time.Now().UTC().Format("2006-01-02")
			if today == lastRunDate {
				continue
			}
			hour := time.Now().UTC().Hour()
			if hour < 4 || hour > 5 {
				continue
			}
			log.Printf("stripe_reconciler.scheduler running daily sweep")
			configs, err := alertConfigRepo.ListAll(ctx)
			if err != nil {
				log.Printf("stripe_reconciler.scheduler list_configs failed err=%v", err)
				continue
			}
			for _, cfg := range configs {
				if _, err := sr.RunForTenant(ctx, cfg.TenantID); err != nil {
					log.Printf("stripe_reconciler.scheduler tenant=%s err=%v", cfg.TenantID, err)
				}
			}
			lastRunDate = today
		}
	}
}
