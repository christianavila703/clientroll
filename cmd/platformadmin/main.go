package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"clientroll/internal/auth"
	"clientroll/internal/config"
	"clientroll/internal/database"
	"clientroll/internal/repository"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: go run ./cmd/platformadmin <email> <password> <name>")
		os.Exit(1)
	}
	email := os.Args[1]
	password := os.Args[2]
	name := os.Args[3]

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	db, err := database.NewPostgres(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()

	if err := auth.ValidatePasswordStrength(password); err != nil {
		log.Fatalf("invalid password: %v", err)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	repo := repository.NewPlatformAdminRepo(db)
	existing, err := repo.FindByEmail(context.Background(), email)
	if err != nil {
		log.Fatalf("check existing admin: %v", err)
	}
	if existing != nil {
		log.Fatalf("platform admin already exists for email %s", email)
	}

	admin, err := repo.Create(context.Background(), email, hash, name)
	if err != nil {
		log.Fatalf("create platform admin: %v", err)
	}
	fmt.Printf("Platform admin created: id=%s email=%s name=%s\n", admin.ID, admin.Email, admin.Name)
}
