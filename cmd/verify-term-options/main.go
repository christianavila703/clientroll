// Dry-run verifier for migration 074_term_options_v2.
//
// Runs the same SQL the real migration runs, inside a transaction that is
// ROLLED BACK at the end. Captures counts of what the migration WOULD do so
// operators can sign off before applying it for real.
//
// Usage:
//
//	DATABASE_URL=... go run ./cmd/verify-term-options [-out unresolved.csv]
//
// Exit code 0 means "verification ran"; the operator should still review
// the printed counts and the optional CSV before applying the migration.
package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationFile = "migrations/074_term_options_v2.up.sql"

func main() {
	var outCSV string
	flag.StringVar(&outCSV, "out", "", "optional CSV path for unresolved memberships")
	flag.Parse()

	dbURL := os.Getenv("DATABASE_URL")
	if strings.TrimSpace(dbURL) == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	migrationPath, err := resolveMigrationPath()
	if err != nil {
		log.Fatalf("locate migration file: %v", err)
	}
	sqlBytes, err := os.ReadFile(migrationPath)
	if err != nil {
		log.Fatalf("read %s: %v", migrationPath, err)
	}
	log.Printf("Using migration source: %s", migrationPath)

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		log.Fatalf("begin tx: %v", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !strings.Contains(err.Error(), "tx is closed") {
			log.Printf("rollback err: %v", err)
		}
	}()

	// Snapshot pre-migration counts.
	preMtoCount, _ := scanInt(ctx, tx, `SELECT COUNT(*) FROM membership_term_options`)
	preMemCount, _ := scanInt(ctx, tx, `SELECT COUNT(*) FROM memberships`)
	preMttCount, _ := scanInt(ctx, tx, `SELECT COUNT(*) FROM membership_type_terms`)

	log.Printf("Pre-migration: %d MTOs, %d memberships, %d (mt → tc) attachments",
		preMtoCount, preMemCount, preMttCount)

	if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
		log.Fatalf("run migration in dry-run tx: %v", err)
	}

	// Post-migration counts (still inside the tx that will roll back).
	schedCount, _ := scanInt(ctx, tx,
		`SELECT COUNT(*) FROM term_option_payment_schedules`)
	totCount, _ := scanInt(ctx, tx,
		`SELECT COUNT(*) FROM term_option_terms`)
	resolved, _ := scanInt(ctx, tx,
		`SELECT COUNT(*) FROM memberships WHERE payment_schedule_id IS NOT NULL`)
	unresolved, _ := scanInt(ctx, tx,
		`SELECT COUNT(*) FROM migration_unresolved_memberships`)
	upfront, _ := scanInt(ctx, tx,
		`SELECT COUNT(*) FROM term_option_payment_schedules WHERE kind = 'upfront'`)
	recurring, _ := scanInt(ctx, tx,
		`SELECT COUNT(*) FROM term_option_payment_schedules WHERE kind = 'recurring'`)
	instalments, _ := scanInt(ctx, tx,
		`SELECT COUNT(*) FROM term_option_payment_schedules WHERE kind = 'instalments'`)
	defaults, _ := scanInt(ctx, tx,
		`SELECT COUNT(*) FROM term_option_payment_schedules WHERE is_default = TRUE`)

	log.Printf("Post-migration (dry-run):")
	log.Printf("  payment schedules created : %d  (upfront=%d, recurring=%d, instalments=%d)",
		schedCount, upfront, recurring, instalments)
	log.Printf("  is_default rows           : %d / %d MTOs",
		defaults, preMtoCount)
	log.Printf("  term_option_terms rows    : %d  (fanned out from %d mt→tc attachments)",
		totCount, preMttCount)
	log.Printf("  memberships resolved      : %d / %d",
		resolved, preMemCount)
	log.Printf("  unresolved memberships    : %d (review before applying)",
		unresolved)

	if unresolved > 0 && outCSV != "" {
		if err := writeUnresolvedCSV(ctx, tx, outCSV); err != nil {
			log.Printf("could not write %s: %v", outCSV, err)
		} else {
			log.Printf("Unresolved details written to %s", outCSV)
		}
	}

	if unresolved == 0 {
		log.Println("OK — every membership maps to a payment_schedule_id.")
	} else {
		log.Println("WARNING — review unresolved memberships before applying the migration.")
	}
}

func resolveMigrationPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := cwd; ; {
		p := filepath.Join(dir, migrationFile)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find %s anywhere from %s upward", migrationFile, cwd)
		}
		dir = parent
	}
}

func scanInt(ctx context.Context, tx pgx.Tx, sql string) (int, error) {
	var n int
	if err := tx.QueryRow(ctx, sql).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func writeUnresolvedCSV(ctx context.Context, tx pgx.Tx, path string) error {
	rows, err := tx.Query(ctx, `
		SELECT tenant_id, membership_id, term_option_id,
		       payment_frequency, payment_amount, reason
		FROM migration_unresolved_memberships
		ORDER BY tenant_id, membership_id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"tenant_id", "membership_id", "term_option_id",
		"payment_frequency", "payment_amount", "reason"})
	for rows.Next() {
		var (
			tenantID, membershipID, termOptionID string
			frequency, reason                    string
			amount                               *float64
		)
		if err := rows.Scan(&tenantID, &membershipID, &termOptionID,
			&frequency, &amount, &reason); err != nil {
			return err
		}
		amt := ""
		if amount != nil {
			amt = fmt.Sprintf("%.2f", *amount)
		}
		if err := w.Write([]string{tenantID, membershipID, termOptionID,
			frequency, amt, reason}); err != nil {
			return err
		}
	}
	return rows.Err()
}
