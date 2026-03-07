package main

import (
	"context"
	"log"
	"os"
	"time"

	"clientroll/internal/billing"
	"clientroll/internal/database"
	"clientroll/internal/repository"
)

// Daily billing job:
// - snapshots active member counts for all tenants
// - updates Stripe subscription quantities (licensed billing: $/active-member/month)
//
// Intended to be run via systemd timer / cron.
func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	masterKey := os.Getenv("MASTER_ENCRYPTION_KEY")
	if masterKey == "" {
		log.Fatal("MASTER_ENCRYPTION_KEY is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, err := database.NewPostgres(dbURL)
	if err != nil {
		log.Fatalf("failed to connect to postgres: %v", err)
	}
	defer db.Close()

	enc, err := database.NewEncryptor(masterKey)
	if err != nil {
		log.Fatalf("failed to init encryptor: %v", err)
	}

	cfgRepo := repository.NewPlatformBillingConfigRepo(db)
	tenantRepo := repository.NewTenantRepo(db)
	subRepo := repository.NewTenantSubscriptionRepo(db)

	cfg, err := cfgRepo.GetOrCreate(ctx)
	if err != nil {
		log.Fatalf("failed to load platform billing config: %v", err)
	}
	if !cfg.IsActive {
		log.Printf("platform billing is inactive; exiting")
		return
	}
	if cfg.StripeSecretKeyEncrypted == nil || *cfg.StripeSecretKeyEncrypted == "" {
		log.Fatal("platform billing: Stripe secret key is not configured")
	}

	stripeSecret, err := enc.Decrypt(*cfg.StripeSecretKeyEncrypted)
	if err != nil {
		log.Fatalf("failed to decrypt Stripe secret key: %v", err)
	}

	var stripePublishable string
	if cfg.StripePublishableKeyEncrypted != nil && *cfg.StripePublishableKeyEncrypted != "" {
		stripePublishable, _ = enc.Decrypt(*cfg.StripePublishableKeyEncrypted)
	}
	var stripeWebhookSecret string
	if cfg.StripeWebhookSecretEncrypted != nil && *cfg.StripeWebhookSecretEncrypted != "" {
		stripeWebhookSecret, _ = enc.Decrypt(*cfg.StripeWebhookSecretEncrypted)
	}

	usageTracker := billing.NewUsageTracker(db)
	if err := usageTracker.SnapshotAllTenants(ctx, cfg.PerMemberPriceUSD); err != nil {
		log.Fatalf("usage snapshot failed: %v", err)
	}

	provider := billing.NewStripeBillingProvider(stripePublishable, stripeSecret, stripeWebhookSecret)

	tenants, err := tenantRepo.List(ctx)
	if err != nil {
		log.Fatalf("failed to list tenants: %v", err)
	}

	updated := 0
	for _, t := range tenants {
		sub, err := subRepo.GetByTenant(ctx, t.ID)
		if err != nil {
			log.Printf("tenant %s: subscription fetch error: %v", t.ID, err)
			continue
		}
		if sub == nil || sub.StripeSubscriptionID == nil || *sub.StripeSubscriptionID == "" {
			continue
		}
		if sub.Status == "cancelled" || sub.Status == "canceled" {
			continue
		}

		if err := provider.ReportUsage(ctx, *sub.StripeSubscriptionID, sub.ActiveMemberCount); err != nil {
			log.Printf("tenant %s: report usage error: %v", t.ID, err)
			continue
		}
		updated++
	}

	log.Printf("billing snapshot complete: %d tenants reported to Stripe", updated)
}

