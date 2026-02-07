package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run ./cmd/migrate [up|down]")
	}

	direction := os.Args[1]
	if direction != "up" && direction != "down" {
		log.Fatal("Direction must be 'up' or 'down'")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		log.Fatalf("Failed to create migrations table: %v", err)
	}

	if direction == "up" {
		if err := migrateUp(ctx, pool); err != nil {
			log.Fatalf("Migration up failed: %v", err)
		}
	} else {
		if err := migrateDown(ctx, pool); err != nil {
			log.Fatalf("Migration down failed: %v", err)
		}
	}
}

func ensureMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func migrateUp(ctx context.Context, pool *pgxpool.Pool) error {
	files, err := getMigrationFiles("up")
	if err != nil {
		return err
	}

	applied, err := getAppliedMigrations(ctx, pool)
	if err != nil {
		return err
	}

	for _, f := range files {
		version := extractVersion(f)
		if applied[version] {
			log.Printf("Skipping %s (already applied)", version)
			continue
		}

		log.Printf("Applying %s...", f)
		sql, err := os.ReadFile(filepath.Join("migrations", f))
		if err != nil {
			return fmt.Errorf("reading %s: %w", f, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("beginning transaction: %w", err)
		}

		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("executing %s: %w", f, err)
		}

		if _, err := tx.Exec(ctx,
			"INSERT INTO schema_migrations (version) VALUES ($1)", version,
		); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("recording migration %s: %w", version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("committing %s: %w", f, err)
		}

		log.Printf("Applied %s", f)
	}

	log.Println("All migrations applied successfully")
	return nil
}

func migrateDown(ctx context.Context, pool *pgxpool.Pool) error {
	applied, err := getAppliedMigrations(ctx, pool)
	if err != nil {
		return err
	}

	if len(applied) == 0 {
		log.Println("No migrations to roll back")
		return nil
	}

	// Find the latest applied migration
	var latest string
	for v := range applied {
		if v > latest {
			latest = v
		}
	}

	downFile := latest + ".down.sql"
	log.Printf("Rolling back %s...", latest)

	sql, err := os.ReadFile(filepath.Join("migrations", downFile))
	if err != nil {
		return fmt.Errorf("reading %s: %w", downFile, err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		tx.Rollback(ctx)
		return fmt.Errorf("executing %s: %w", downFile, err)
	}

	if _, err := tx.Exec(ctx,
		"DELETE FROM schema_migrations WHERE version = $1", latest,
	); err != nil {
		tx.Rollback(ctx)
		return fmt.Errorf("removing migration record %s: %w", latest, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing rollback: %w", err)
	}

	log.Printf("Rolled back %s", latest)
	return nil
}

func getMigrationFiles(direction string) ([]string, error) {
	entries, err := os.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("reading migrations directory: %w", err)
	}

	var files []string
	suffix := "." + direction + ".sql"
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), suffix) {
			files = append(files, e.Name())
		}
	}

	sort.Strings(files)
	return files, nil
}

func getAppliedMigrations(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func extractVersion(filename string) string {
	// "001_create_extensions.up.sql" -> "001_create_extensions"
	name := strings.TrimSuffix(filename, ".up.sql")
	name = strings.TrimSuffix(name, ".down.sql")
	return name
}
