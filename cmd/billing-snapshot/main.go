package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"clientroll/internal/billing"
	"clientroll/internal/database"
	"clientroll/internal/repository"
)

// Daily billing job:
// - snapshots active member counts for all tenants
// - updates Stripe subscription quantities (licensed billing: $/active-member/month)
//
// Schedule via scripts/install-billing-snapshot-cron.sh or systemd timer.
// Requires DATABASE_URL and MASTER_ENCRYPTION_KEY.
func main() {
	dryRun := flag.Bool("dry-run", false, "load config and log whether the job would run, without writing")
	flag.Parse()

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	masterKey := strings.TrimSpace(os.Getenv("MASTER_ENCRYPTION_KEY"))
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

	if *dryRun {
		cfg, err := cfgRepo.GetOrCreate(ctx)
		if err != nil {
			log.Fatalf("failed to load platform billing config: %v", err)
		}
		if !cfg.IsActive {
			log.Println("dry-run: platform billing is inactive; job would exit without changes")
			return
		}
		log.Printf("dry-run: platform billing is active (price=%.2f %s); job would run snapshot + Stripe updates", cfg.PerMemberPriceUSD, cfg.Currency)
		return
	}

	runner := billing.NewSnapshotRunner(db, enc, cfgRepo, tenantRepo, subRepo)
	result, err := runner.Run(ctx)
	if result.BillingInactive {
		return
	}
	if err != nil {
		log.Fatalf("billing snapshot failed: %v", err)
	}
	log.Printf(
		"billing snapshot success: snapshotted=%d stripe_updated=%d stripe_skipped=%d",
		result.SnapshottedTenants,
		result.StripeUpdated,
		result.StripeSkipped,
	)
}
