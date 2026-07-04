package billing

import (
	"context"
	"fmt"
	"log"
	"strings"

	"clientroll/internal/database"
	"clientroll/internal/repository"

	"github.com/google/uuid"
)

// SnapshotRunner runs the daily platform billing snapshot job.
type SnapshotRunner struct {
	db         *database.Postgres
	enc        *database.Encryptor
	cfgRepo    *repository.PlatformBillingConfigRepo
	tenantRepo *repository.TenantRepo
	subRepo    *repository.TenantSubscriptionRepo
}

// NewSnapshotRunner creates a snapshot runner.
func NewSnapshotRunner(
	db *database.Postgres,
	enc *database.Encryptor,
	cfgRepo *repository.PlatformBillingConfigRepo,
	tenantRepo *repository.TenantRepo,
	subRepo *repository.TenantSubscriptionRepo,
) *SnapshotRunner {
	return &SnapshotRunner{
		db:         db,
		enc:        enc,
		cfgRepo:    cfgRepo,
		tenantRepo: tenantRepo,
		subRepo:    subRepo,
	}
}

// SnapshotRunResult summarizes a billing snapshot run.
type SnapshotRunResult struct {
	BillingInactive    bool     `json:"billing_inactive"`
	SnapshottedTenants int      `json:"snapshotted_tenants"`
	StripeUpdated      int      `json:"stripe_updated"`
	StripeSkipped      int      `json:"stripe_skipped"`
	StripeFailed       int      `json:"stripe_failed"`
	Errors             []string `json:"errors,omitempty"`
}

// Run executes the daily billing snapshot:
//  1. Loads platform_billing_config — no-ops when billing is inactive
//  2. Snapshots active member counts for all active tenants
//  3. Reports quantity to Stripe for each tenant with an active subscription
func (r *SnapshotRunner) Run(ctx context.Context) (SnapshotRunResult, error) {
	result := SnapshotRunResult{}

	cfg, err := r.cfgRepo.GetOrCreate(ctx)
	if err != nil {
		return result, fmt.Errorf("load platform billing config: %w", err)
	}
	if !cfg.IsActive {
		log.Printf("billing snapshot: platform billing is inactive; exiting")
		result.BillingInactive = true
		return result, nil
	}
	if cfg.StripeSecretKeyEncrypted == nil || strings.TrimSpace(*cfg.StripeSecretKeyEncrypted) == "" {
		return result, fmt.Errorf("platform billing: Stripe secret key is not configured")
	}
	if cfg.StripePriceID == nil || strings.TrimSpace(*cfg.StripePriceID) == "" {
		return result, fmt.Errorf("platform billing: Stripe price ID is not configured")
	}

	stripeSecret, err := r.enc.Decrypt(*cfg.StripeSecretKeyEncrypted)
	if err != nil {
		return result, fmt.Errorf("decrypt Stripe secret key: %w", err)
	}

	var stripePublishable string
	if cfg.StripePublishableKeyEncrypted != nil && *cfg.StripePublishableKeyEncrypted != "" {
		stripePublishable, _ = r.enc.Decrypt(*cfg.StripePublishableKeyEncrypted)
	}
	var stripeWebhookSecret string
	if cfg.StripeWebhookSecretEncrypted != nil && *cfg.StripeWebhookSecretEncrypted != "" {
		stripeWebhookSecret, _ = r.enc.Decrypt(*cfg.StripeWebhookSecretEncrypted)
	}

	usageTracker := NewUsageTracker(r.db)
	counts, err := usageTracker.SnapshotAllTenants(ctx, cfg.PerMemberPriceUSD)
	if err != nil {
		return result, fmt.Errorf("usage snapshot: %w", err)
	}
	result.SnapshottedTenants = len(counts)

	provider := NewStripeBillingProvider(stripePublishable, stripeSecret, stripeWebhookSecret)

	tenants, err := r.tenantRepo.List(ctx)
	if err != nil {
		return result, fmt.Errorf("list tenants: %w", err)
	}

	for _, tenant := range tenants {
		sub, err := r.subRepo.GetByTenant(ctx, tenant.ID)
		if err != nil {
			result.StripeFailed++
			msg := fmt.Sprintf("tenant %s: subscription fetch: %v", tenant.Slug, err)
			result.Errors = append(result.Errors, msg)
			log.Printf("billing snapshot: %s", msg)
			continue
		}
		if sub == nil || sub.StripeSubscriptionID == nil || strings.TrimSpace(*sub.StripeSubscriptionID) == "" {
			result.StripeSkipped++
			continue
		}
		status := strings.ToLower(strings.TrimSpace(sub.Status))
		if status == "cancelled" || status == "canceled" {
			result.StripeSkipped++
			continue
		}

		memberCount := counts[tenant.ID]
		subscriptionID := strings.TrimSpace(*sub.StripeSubscriptionID)
		if err := provider.ReportUsage(ctx, subscriptionID, memberCount); err != nil {
			result.StripeFailed++
			msg := fmt.Sprintf("tenant %s (%s): report usage: %v", tenant.Slug, subscriptionID, err)
			result.Errors = append(result.Errors, msg)
			log.Printf("billing snapshot: %s", msg)
			continue
		}
		result.StripeUpdated++
	}

	log.Printf(
		"billing snapshot complete: snapshotted=%d stripe_updated=%d stripe_skipped=%d stripe_failed=%d",
		result.SnapshottedTenants,
		result.StripeUpdated,
		result.StripeSkipped,
		result.StripeFailed,
	)

	if result.StripeFailed > 0 {
		return result, fmt.Errorf("%d tenant(s) failed Stripe quantity update", result.StripeFailed)
	}
	return result, nil
}

// TenantActiveCounts maps tenant ID to active member count from the latest snapshot run.
type TenantActiveCounts map[uuid.UUID]int
