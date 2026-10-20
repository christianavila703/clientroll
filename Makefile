# ClientRoll Makefile

.PHONY: help dev dev-db dev-stop build run test migrate frontend frontend-dev clean

# Default target
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Development
dev-db: ## Start dev PostgreSQL + Redis
	docker compose -f docker-compose-dev.yml up -d

dev-db-stop: ## Stop dev PostgreSQL + Redis
	docker compose -f docker-compose-dev.yml down

dev: ## Run Go server in development mode
	DATABASE_URL="postgresql://clientroll_user:clientroll_dev_password@localhost:5432/clientroll_db?sslmode=disable" \
	REDIS_URL="redis://localhost:6379/2" \
	MASTER_ENCRYPTION_KEY="dev-only-key-must-be-32-chars-long!!" \
	ENVIRONMENT="development" \
	CORS_ALLOWED_ORIGINS="http://localhost:3000,http://localhost:8080" \
	go run ./cmd/server

# Build
build: ## Build Go binary
	CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/clientroll ./cmd/server

# Test
test: ## Run all tests
	go test ./... -v -race

# Database migrations
migrate-up: ## Run all migrations
	@echo "Running migrations..."
	DATABASE_URL="postgresql://clientroll_user:clientroll_dev_password@localhost:5432/clientroll_db?sslmode=disable" \
	go run ./cmd/migrate up

migrate-down: ## Rollback last migration
	@echo "Rolling back migration..."
	DATABASE_URL="postgresql://clientroll_user:clientroll_dev_password@localhost:5432/clientroll_db?sslmode=disable" \
	go run ./cmd/migrate down

# Frontend
frontend: ## Build frontend for production
	cd frontend && npm run build

frontend-dev: ## Start frontend dev server
	cd frontend && npm run dev

frontend-install: ## Install frontend dependencies
	cd frontend && npm install

# Clean
clean: ## Clean build artifacts
	rm -rf bin/
	rm -rf web/assets/
	rm -rf frontend/node_modules/.cache
