# Environment Variables Reference

Single source of truth for behaviour is `internal/config/config.go`. This file is a quick reference for local and production.

## Required

| Variable | Example | Notes |
|----------|---------|--------|
| `DATABASE_URL` | `postgresql://user:pass@localhost:5432/clientroll_db?sslmode=disable` | PostgreSQL connection string |
| `REDIS_URL` | `redis://localhost:6379/2` | Redis connection string |
| `MASTER_ENCRYPTION_KEY` | (32+ chars) | Encrypts tenant credentials; min 32 characters |

## Optional (with defaults)

| Variable | Default | Notes |
|----------|---------|--------|
| `PORT` | `8080` | HTTP server port |
| `ENVIRONMENT` | `development` | `development`, `staging`, or `production` |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Comma-separated list |
| `JWT_ACCESS_DURATION_MINUTES` | `15` | Access token lifetime |
| `JWT_REFRESH_DURATION_DAYS` | `7` | Refresh token lifetime |
| `MAILERSEND_API_KEY` | (empty) | If empty, email sending may be no-op or fail |
| `MAILERSEND_FROM_EMAIL` | `clientroll@app.businessnet.com` | From address for transactional email |
| `APP_URL` | `https://app.clientroll.com` | Used in OAuth redirects and email links |
| `GOCARDLESS_SANDBOX_CLIENT_ID` | (empty) | GoCardless Connect sandbox |
| `GOCARDLESS_SANDBOX_CLIENT_SECRET` | (empty) | |
| `GOCARDLESS_LIVE_CLIENT_ID` | (empty) | GoCardless Connect live |
| `GOCARDLESS_LIVE_CLIENT_SECRET` | (empty) | |
| `GOCARDLESS_WEBHOOK_SECRET` | (empty) | Webhook signature verification |

If all GoCardless variables are empty, GoCardless integration is disabled.

## Local development (Makefile)

`make dev` injects:

- `DATABASE_URL="postgresql://clientroll_user:clientroll_dev_password@localhost:5432/clientroll_db?sslmode=disable"`
- `REDIS_URL="redis://localhost:6379/2"`
- `MASTER_ENCRYPTION_KEY="dev-only-key-must-be-32-chars-long!!"`
- `ENVIRONMENT="development"`
- `CORS_ALLOWED_ORIGINS="http://localhost:3000,http://localhost:8080"`

Use `make dev-db` first so Postgres and Redis match these URLs.
