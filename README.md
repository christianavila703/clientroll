# ClientRoll

Multi-tenant SaaS for membership and client management. Businesses (tenants) publish signup pages, collect signed agreements, and take recurring or one-off payments from their members through their own Stripe or GoCardless accounts.

## Features

- **Multi-tenant** — PostgreSQL row-level security isolates each tenant's data
- **Public signup flow** — membership selection, terms acceptance, signature capture, payment
- **Payments** — Stripe (card, BECS direct debit) and GoCardless (direct debit), connected per tenant
- **Agreements** — signed HTML snapshots stored at acceptance, rendered to PDF on demand
- **Webhooks** — signed inbound (create clients) and outbound (client, membership, payment events)
- **Accounting** — optional Xero sync
- **Platform billing** — the platform owner bills tenants per active member via a separate Stripe account

## Tech stack

| Layer | Technologies |
|-------|--------------|
| Backend | Go 1.24, Chi, pgx (PostgreSQL), go-redis |
| Frontend | React 18, TypeScript, Rspack, Tailwind |
| Auth | JWT + Redis sessions, bcrypt, CSRF tokens |
| Payments | Stripe, GoCardless |
| Email | MailerSend |
| PDF | `chromedp` (requires Chrome/Chromium on the host) |

## Project structure

```
cmd/
  server/              Main API server
  migrate/             Migration CLI (up / down)
  billing-snapshot/    Daily platform-billing usage job
  backfill-agreements/ Backfill missing agreement snapshots
  platformadmin/       Create a platform admin
  hashpw/              Hash a password with bcrypt
internal/
  handlers/            HTTP handlers
  repository/          Data access
  middleware/          Auth, tenant context, CSRF, rate limiting, audit
  payment/             GoCardless client
  billing/             Platform billing (owner → tenant)
  events/              Webhook dispatcher and worker
  email/ pdf/ terms/   Email, PDF rendering, T&C templating
  config/              Environment configuration
frontend/              React SPA (builds into web/)
migrations/            Numbered SQL migrations (NNN_name.up.sql / .down.sql)
scripts/               Server deploy, DB backup, cron install and smoke test
```

## Getting started

**Prerequisites:** Go 1.24+, Node.js + npm, Docker.

```bash
make frontend-install   # Install frontend dependencies
make dev-db             # Start Postgres + Redis in Docker
make migrate-up         # Apply migrations
make dev                # API on http://localhost:8080
make frontend-dev       # Frontend on http://localhost:3000 (separate terminal)
```

Open http://localhost:3000. The frontend dev server proxies `/api` to the backend. `make dev` supplies local database, Redis and encryption-key values, so no `.env` is needed for development.

## Configuration

Copy `.env.example` to `.env` and fill in the values. All settings are read in `internal/config/config.go`.

**Required**

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `REDIS_URL` | Redis connection string |
| `MASTER_ENCRYPTION_KEY` | 32+ characters; encrypts tenant credentials at rest |

**Optional**

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP port |
| `ENVIRONMENT` | `development` | `development`, `staging` or `production` |
| `APP_URL` | `http://localhost:3000` | Public origin, used for OAuth redirects and email links |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Comma-separated list |
| `JWT_ACCESS_DURATION_MINUTES` | `15` | Access token lifetime |
| `JWT_REFRESH_DURATION_DAYS` | `7` | Refresh token lifetime |
| `MAILERSEND_API_KEY` | — | MailerSend API key |
| `MAILERSEND_FROM_EMAIL` | `noreply@example.com` | Sender address |
| `SIGNUP_LINK_SIGNING_SECRET` | `MASTER_ENCRYPTION_KEY` | HMAC secret for signup links |
| `SIGNUP_LINKS_REQUIRE_SIGNATURE` | — | Reject unsigned signup links when `true` |
| `GOCARDLESS_SANDBOX_CLIENT_ID` / `_SECRET` | — | GoCardless Connect (sandbox) |
| `GOCARDLESS_LIVE_CLIENT_ID` / `_SECRET` | — | GoCardless Connect (live) |
| `GOCARDLESS_WEBHOOK_SECRET` | — | GoCardless webhook signature secret |
| `XERO_CLIENT_ID` / `_SECRET` / `_REDIRECT_URI` | — | Xero OAuth |
| `XERO_SYNC_ENABLED` | `false` | Enable Xero sync |
| `ALERTS_WEBHOOK_URL` | — | Receives critical internal alerts as JSON |

GoCardless and Xero are disabled when their variables are empty. Tenants enter their own Stripe and GoCardless account credentials in the app; these are stored encrypted.

## Commands

| Command | Description |
|---------|-------------|
| `make help` | List all targets |
| `make dev` / `make frontend-dev` | Run API / frontend in development |
| `make dev-db` / `make dev-db-stop` | Start / stop local Postgres + Redis |
| `make migrate-up` / `make migrate-down` | Apply / roll back migrations |
| `make test` | Run Go tests |
| `make build` | Build API binary to `bin/clientroll` |
| `make frontend` | Build frontend to `web/` |
| `make docker-build` / `docker-up` / `docker-down` | Production Docker image and containers |
| `make clean` | Remove build artifacts |

## Deployment

The API serves the built frontend from `web/`. Two deploy scripts are included:

- **Build on the server** — clone the repo on the server, then run `./scripts/deploy-on-server.sh`. It pulls, builds the backend and frontend, backs up the database, runs migrations, swaps in the new release and restarts the systemd service (`clientroll` by default; override with `DEPLOY_SERVICE`).
- **Build locally** — `DEPLOY_HOST=user@host DEPLOY_PATH=/var/www/clientroll ./deploy.sh` builds on your machine, uploads over SCP, runs migrations and restarts the service.

After deploying, run `APP_URL=https://your-domain ./scripts/smoke-prod.sh` to check health and signup-link enforcement.

**Host requirements**

- Chrome or Chromium on `PATH` (`google-chrome`, `chromium` or `chromium-browser`) for agreement PDFs
- A `.env` file in the deploy path with at least the required variables

**Platform billing job** — `cmd/billing-snapshot` counts active members per tenant and updates Stripe subscription quantities. Run `./scripts/install-cron.sh` on the server to schedule it daily alongside the database backup. Use `go run ./cmd/billing-snapshot -dry-run` to check the configuration.

**Payment provider setup**

- **Stripe** — point each tenant's webhook at `/api/webhooks/stripe/{slug}`. Subscribe to the `checkout.session.*`, `invoice.*`, `payment_intent.*`, `charge.*` and `customer.subscription.deleted` events.
- **GoCardless** — create a Partner app and set the redirect URL to `{APP_URL}/api/gocardless/oauth/callback` and the webhook URL to `{APP_URL}/api/webhooks/gocardless`.

## Webhooks

**Inbound** — `POST /api/webhooks/inbound/clients` creates a client. Tenant admins create keys under **Settings**. Each request must include:

| Header | Value |
|--------|-------|
| `X-Webhook-Key-Id` | Inbound key ID |
| `X-Webhook-Timestamp` | Unix seconds (rejected if more than 5 minutes off) |
| `X-Webhook-Signature` | Hex HMAC-SHA256 of `<timestamp>.<raw_body>` |

```json
{
  "event_id": "evt_123",
  "event_type": "client.created",
  "data": { "name": "Jane Citizen", "email": "jane@example.com", "phone": "0400000000" }
}
```

`event_id` is used for deduplication. The body limit is 1 MiB.

**Outbound** — tenants register endpoints for `client.created`, `membership.created`, `membership.cancelled`, `membership.expired`, `payment.succeeded` and `payment.failed`. Each delivery is signed the same way in `X-Webhook-Signature`, with `X-Webhook-Event`, `X-Webhook-Event-Id` and `X-Webhook-Timestamp` headers. Failed deliveries are retried with exponential backoff.

A separate **signup webhook** (Settings → Organisation) sends one `signup.completed` payload per completed membership signup.
