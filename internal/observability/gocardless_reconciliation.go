package observability

import (
	"context"
	"fmt"
	"log"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/payment"
	"clientroll/internal/repository"

	"github.com/google/uuid"
)

// GCReconciler verifies local membership/payment state against the GoCardless API.
type GCReconciler struct {
	db          *database.Postgres
	encryptor   *database.Encryptor
	credRepo    *repository.TenantCredentialRepo
	paymentRepo *repository.PaymentRepo
	memberRepo  *repository.MembershipRepo
	reconRepo   *repository.ReconciliationRepo
}

func NewGCReconciler(
	db *database.Postgres,
	enc *database.Encryptor,
	credRepo *repository.TenantCredentialRepo,
	paymentRepo *repository.PaymentRepo,
	memberRepo *repository.MembershipRepo,
	reconRepo *repository.ReconciliationRepo,
) *GCReconciler {
	return &GCReconciler{
		db:          db,
		encryptor:   enc,
		credRepo:    credRepo,
		paymentRepo: paymentRepo,
		memberRepo:  memberRepo,
		reconRepo:   reconRepo,
	}
}

type membershipGCInfo struct {
	MembershipID     uuid.UUID
	ClientID         uuid.UUID
	ClientName       string
	MembershipTitle  string
	MembershipStatus string
	GCSubscriptionID string
	GCMandateID      string
	PaymentAmount    float64
}

// RunForTenant performs the full GoCardless reconciliation sweep for a single tenant.
func (gr *GCReconciler) RunForTenant(ctx context.Context, tenantID uuid.UUID) (*repository.ReconciliationRun, error) {
	run, err := gr.reconRepo.Create(ctx, tenantID, "gocardless")
	if err != nil {
		return nil, fmt.Errorf("create gc reconciliation run: %w", err)
	}

	prov, err := gr.getProvider(ctx, tenantID)
	if err != nil || prov == nil {
		log.Printf("gc_reconciler no_provider tenant=%s", tenantID)
		_ = gr.reconRepo.Complete(ctx, run.ID, 0, 0, 0, 0, nil)
		return run, nil
	}

	memberships, err := gr.getMembershipsWithGC(ctx, tenantID)
	if err != nil {
		log.Printf("gc_reconciler list_memberships failed tenant=%s err=%v", tenantID, err)
		_ = gr.reconRepo.Complete(ctx, run.ID, 0, 0, 0, 0, nil)
		return run, err
	}

	var mismatches []repository.ReconciliationMismatch
	checked, matched := 0, 0

	for _, m := range memberships {
		checked++

		if m.GCSubscriptionID != "" {
			sub, subErr := prov.GetSubscription(ctx, m.GCSubscriptionID)
			if subErr != nil {
				log.Printf("gc_reconciler sub_get failed tenant=%s sub=%s err=%v", tenantID, m.GCSubscriptionID, subErr)
				continue
			}

			mismatch := gr.compareSubscriptionStatuses(m, sub)
			if mismatch != nil {
				autoFixed := gr.attemptAutoFix(ctx, tenantID, m, sub.Status)
				mismatch.AutoFixed = autoFixed
				mismatches = append(mismatches, *mismatch)
			} else {
				matched++
			}

			if m.PaymentAmount > 0 && sub.Amount > 0 {
				gcAmount := float64(sub.Amount) / 100.0
				if amountMismatch := gr.checkAmountMismatch(m, gcAmount); amountMismatch != nil {
					mismatches = append(mismatches, *amountMismatch)
				}
			}
		} else {
			matched++
		}
	}

	autoFixed := 0
	for _, mm := range mismatches {
		if mm.AutoFixed {
			autoFixed++
		}
	}

	if err := gr.reconRepo.Complete(ctx, run.ID, checked, matched, len(mismatches), autoFixed, mismatches); err != nil {
		log.Printf("gc_reconciler complete_run failed tenant=%s err=%v", tenantID, err)
	}

	run.TotalChecked = checked
	run.TotalMatched = matched
	run.TotalMismatched = len(mismatches)
	run.TotalAutoFixed = autoFixed
	run.Details = mismatches
	now := time.Now().UTC()
	run.CompletedAt = &now

	log.Printf("gc_reconciler completed tenant=%s checked=%d matched=%d mismatched=%d auto_fixed=%d",
		tenantID, checked, matched, len(mismatches), autoFixed)
	return run, nil
}

func (gr *GCReconciler) getProvider(ctx context.Context, tenantID uuid.UUID) (*payment.GoCardlessProvider, error) {
	creds, err := gr.credRepo.GetByProvider(ctx, tenantID, "gocardless")
	if err != nil || creds == nil || !creds.IsActive || creds.APIKeyEncrypted == "" {
		return nil, nil
	}
	apiKey, err := gr.encryptor.Decrypt(creds.APIKeyEncrypted)
	if err != nil || apiKey == "" {
		return nil, fmt.Errorf("decrypt gc key: %w", err)
	}
	env := creds.Environment
	if env == "" {
		env = "live"
	}
	return payment.NewGoCardlessProvider(apiKey, "", env), nil
}

func (gr *GCReconciler) getMembershipsWithGC(ctx context.Context, tenantID uuid.UUID) ([]membershipGCInfo, error) {
	rows, err := gr.db.Pool.Query(ctx, `
		SELECT m.id, m.client_id, c.name, mt.title, m.status,
		       COALESCE(m.gocardless_subscription_id, ''),
		       COALESCE(m.gocardless_mandate_id, ''),
		       COALESCE(m.payment_amount, 0)
		FROM memberships m
		JOIN clients c ON c.id = m.client_id AND c.tenant_id = m.tenant_id
		JOIN membership_types mt ON mt.id = m.membership_type_id AND mt.tenant_id = m.tenant_id
		WHERE m.tenant_id = $1
		  AND m.status NOT IN ('cancelled', 'expired')
		  AND (m.gocardless_subscription_id IS NOT NULL OR m.gocardless_mandate_id IS NOT NULL)
		ORDER BY m.id
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("query gc memberships: %w", err)
	}
	defer rows.Close()

	var items []membershipGCInfo
	for rows.Next() {
		var m membershipGCInfo
		if err := rows.Scan(&m.MembershipID, &m.ClientID, &m.ClientName, &m.MembershipTitle,
			&m.MembershipStatus, &m.GCSubscriptionID, &m.GCMandateID, &m.PaymentAmount); err != nil {
			return nil, fmt.Errorf("scan membership gc info: %w", err)
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

func (gr *GCReconciler) compareSubscriptionStatuses(m membershipGCInfo, sub payment.SubscriptionInfo) *repository.ReconciliationMismatch {
	localActive := m.MembershipStatus == "active" || m.MembershipStatus == "scheduled"
	localPastDue := m.MembershipStatus == "past_due"

	switch sub.Status {
	case "active":
		if localActive {
			return nil
		}
		if localPastDue {
			return &repository.ReconciliationMismatch{
				MembershipID:      m.MembershipID,
				ClientID:          m.ClientID,
				ClientName:        m.ClientName,
				MembershipTitle:   m.MembershipTitle,
				LocalStatus:       m.MembershipStatus,
				ProviderStatus:    sub.Status,
				ProviderSubID:     m.GCSubscriptionID,
				Severity:          "warning",
				Category:          "missed_success",
				RecommendedAction: "GoCardless says active but we show past_due — may have missed a success webhook",
			}
		}
		return nil

	case "cancelled", "finished":
		if localActive || localPastDue {
			return &repository.ReconciliationMismatch{
				MembershipID:      m.MembershipID,
				ClientID:          m.ClientID,
				ClientName:        m.ClientName,
				MembershipTitle:   m.MembershipTitle,
				LocalStatus:       m.MembershipStatus,
				ProviderStatus:    sub.Status,
				ProviderSubID:     m.GCSubscriptionID,
				Severity:          "critical",
				Category:          "cancelled_at_provider",
				RecommendedAction: "GoCardless subscription " + sub.Status + " but still active locally",
			}
		}
		return nil

	case "paused":
		if localActive {
			return &repository.ReconciliationMismatch{
				MembershipID:      m.MembershipID,
				ClientID:          m.ClientID,
				ClientName:        m.ClientName,
				MembershipTitle:   m.MembershipTitle,
				LocalStatus:       m.MembershipStatus,
				ProviderStatus:    sub.Status,
				ProviderSubID:     m.GCSubscriptionID,
				Severity:          "warning",
				Category:          "paused_at_provider",
				RecommendedAction: "GoCardless subscription paused — membership may need suspension",
			}
		}
		return nil

	default:
		return nil
	}
}

func (gr *GCReconciler) checkAmountMismatch(m membershipGCInfo, gcAmount float64) *repository.ReconciliationMismatch {
	diff := m.PaymentAmount - gcAmount
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
		ProviderStatus:    fmt.Sprintf("$%.2f", gcAmount),
		ProviderSubID:     m.GCSubscriptionID,
		Severity:          "warning",
		Category:          "amount_mismatch",
		RecommendedAction: fmt.Sprintf("Local amount $%.2f vs GoCardless $%.2f", m.PaymentAmount, gcAmount),
	}
}

func (gr *GCReconciler) attemptAutoFix(ctx context.Context, tenantID uuid.UUID, m membershipGCInfo, gcStatus string) bool {
	switch {
	case gcStatus == "active" && m.MembershipStatus == "past_due":
		_, err := gr.db.Pool.Exec(ctx,
			`UPDATE memberships SET status = 'active', updated_at = NOW() WHERE id = $1 AND tenant_id = $2`,
			m.MembershipID, tenantID)
		if err != nil {
			log.Printf("gc_reconciler auto_fix_active failed membership=%s err=%v", m.MembershipID, err)
			return false
		}
		log.Printf("gc_reconciler auto_fixed_active tenant=%s membership=%s gc_status=%s", tenantID, m.MembershipID, gcStatus)
		return true
	}
	return false
}

// StartGCReconciliationScheduler runs the GoCardless reconciliation daily for all configured tenants.
func StartGCReconciliationScheduler(ctx context.Context, gr *GCReconciler, alertConfigRepo *repository.AlertConfigRepo) {
	if gr == nil || alertConfigRepo == nil {
		log.Printf("gc_reconciler.scheduler skipped (missing dependencies)")
		return
	}
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	log.Printf("gc_reconciler.scheduler started")

	lastRunDate := ""
	for {
		select {
		case <-ctx.Done():
			log.Printf("gc_reconciler.scheduler stopped")
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
			log.Printf("gc_reconciler.scheduler running daily sweep")
			configs, err := alertConfigRepo.ListAll(ctx)
			if err != nil {
				log.Printf("gc_reconciler.scheduler list_configs failed err=%v", err)
				continue
			}
			for _, cfg := range configs {
				if _, err := gr.RunForTenant(ctx, cfg.TenantID); err != nil {
					log.Printf("gc_reconciler.scheduler tenant=%s err=%v", cfg.TenantID, err)
				}
			}
			lastRunDate = today
		}
	}
}
