# ClientRoll – Internal Development Documentation

This document describes the ClientRoll codebase for developers working on the project. For deployment and webhooks, see [DEPLOY.md](DEPLOY.md), [WEBHOOKS.md](WEBHOOKS.md), and [GOCARDLESS_DEPLOY.md](GOCARDLESS_DEPLOY.md).

---

## 1. Overview

ClientRoll is a **multi-tenant SaaS** for membership and client management with recurring payments (GoCardless), one-off charges (Stripe), email (MailerSend), and optional inbound/outbound webhooks.

- **Backend:** Single Go API (Chi router) serving JSON and static assets. No server-rendered HTML.
- **Frontend:** React SPA (Rspack, TypeScript, Tailwind) built into `web/` and served by the API or a reverse proxy.
- **Data:** PostgreSQL (with RLS for tenant isolation), Redis (sessions, rate limiting, event queue).
- **Secrets:** One `MASTER_ENCRYPTION_KEY` in env; tenant credentials (e.g. GoCardless) stored encrypted in the DB.

---

## 2. Tech Stack

| Layer | Technologies |
|-------|---------------|
| **Backend** | Go 1.22, Chi v5, pgx/v5 (Postgres), go-redis v9 |
| **Auth** | JWT (golang-jwt/jwt/v5), Redis sessions, bcrypt (golang.org/x/crypto) |
| **Database** | PostgreSQL, migrations in `migrations/*.sql` |
| **Cache / sessions** | Redis |
| **Payments** | GoCardless (OAuth Connect, payments/refunds/mandates), Stripe (webhooks) |
| **Email** | MailerSend |
| **Frontend** | React 18, TypeScript, React Router v6, Rspack (SWC), Tailwind, PostCSS |
| **Frontend UI** | Tremor, Recharts, ECharts, TipTap (rich text) |
| **Testing** | Go: `go test` in `internal/` |
| **Deploy** | Docker (Alpine), Makefile, `deploy.sh` or `scripts/deploy-on-server.sh` |

---

## 3. Repository Structure

```
clientroll/
├── cmd/                    # Go entrypoints
│   ├── server/             # Main API (main.go)
│   ├── migrate/            # DB migrations CLI (up/down)
│   ├── platformadmin/      # One-off: create platform admin
│   └── hashpw/             # CLI: bcrypt hash password
├── internal/               # Backend application code
│   ├── config/             # Env-based config
│   ├── auth/               # JWT, sessions, password hashing
│   ├── database/           # Postgres pool, Redis, Encryptor
│   ├── handlers/           # HTTP handlers (auth, platform, tenant resources)
│   ├── middleware/         # Auth, tenant context, CSRF, audit, rate limit, logging
│   ├── models/             # DTOs / request structs
│   ├── repository/         # Data access (user, tenant, client, membership, etc.)
│   ├── email/              # MailerSend provider and templates
│   ├── events/             # Event dispatcher, worker, inbound webhook auth
│   ├── payment/            # GoCardless client and webhook handling
│   ├── billing/            # Stripe usage/billing (provider interface)
│   ├── storage/            # File store (e.g. ./uploads)
│   └── observability/      # Alerts monitor, payment lifecycle metrics
├── frontend/               # React SPA
│   ├── src/
│   │   ├── main.tsx        # Entry, BrowserRouter, AuthProvider
│   │   ├── App.tsx         # Route definitions
│   │   ├── api/client.ts   # API client (Bearer, CSRF, refresh on 401)
│   │   ├── store/          # AuthContext
│   │   ├── components/     # Pages and shared UI
│   │   ├── hooks/
│   │   └── styles/
│   ├── rspack.config.ts
│   ├── tailwind.config.ts
│   └── package.json
├── migrations/             # Numbered SQL up/down migrations (NNN_*.up.sql/.down.sql)
├── web/                    # Frontend build output (served by API)
├── docs/                   # DEPLOY, WEBHOOKS, GOCARDLESS_DEPLOY, this file
├── scripts/                # deploy-on-server.sh
├── Makefile                # dev, build, test, migrate, frontend, docker
├── Dockerfile              # Multi-stage Go API image
├── docker-compose-dev.yml  # Local Postgres + Redis
├── docker-compose.yml      # Production API container
└── deploy.sh               # Build locally, SCP, migrate, restart
```

---

## 4. Environment and Configuration

Configuration is loaded from environment variables in `internal/config/config.go`. There is no `.env.example` in the repo; required and optional variables are listed below.

### Required (all environments)

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `REDIS_URL` | Redis connection string (e.g. `redis://localhost:6379/2`) |
| `MASTER_ENCRYPTION_KEY` | At least 32 characters; used to encrypt tenant credentials and tokens |

### Optional (with defaults)

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `ENVIRONMENT` | `development` | `development` \| `staging` \| `production` |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Comma-separated origins |
| `JWT_ACCESS_DURATION_MINUTES` | `15` | Access token TTL |
| `JWT_REFRESH_DURATION_DAYS` | `7` | Refresh token TTL |
| `MAILERSEND_API_KEY` | (empty) | MailerSend API key |
| `MAILERSEND_FROM_EMAIL` | `clientroll@app.businessnet.com` | From address |
| `APP_URL` | `https://app.clientroll.com` | App origin (OAuth, emails) |
| `GOCARDLESS_SANDBOX_CLIENT_ID` / `_SECRET` | (empty) | GoCardless sandbox Connect |
| `GOCARDLESS_LIVE_CLIENT_ID` / `_SECRET` | (empty) | GoCardless live Connect |
| `GOCARDLESS_WEBHOOK_SECRET` | (empty) | GoCardless webhook signature verification |

If GoCardless env vars are empty, GoCardless features are disabled. Validation only enforces the three required vars; in development, empty required vars can be used for quicker local startup (e.g. Makefile overrides).

---

## 5. Local Development

### Prerequisites

- Go 1.22+
- Node.js and npm (for frontend)
- Docker and Docker Compose (for Postgres and Redis)

### One-time setup

1. Clone the repo and install frontend deps:
   ```bash
   cd frontend && npm install && cd ..
   ```
2. Start Postgres and Redis:
   ```bash
   make dev-db
   ```
3. Run migrations:
   ```bash
   make migrate-up
   ```

### Running the app

- **Backend:** `make dev` — runs the Go server with dev DB/Redis and a dev `MASTER_ENCRYPTION_KEY`. Listens on port 8080.
- **Frontend:** `make frontend-dev` (or `cd frontend && npm run dev`) — Rspack dev server on port 3000, proxying `/api` to `http://localhost:8080`.

Use the app at `http://localhost:3000`. The API is at `http://localhost:8080`; the frontend uses `/api` which the dev server proxies to the backend.

### Other useful commands

| Command | Description |
|---------|-------------|
| `make dev-db-stop` | Stop Postgres and Redis |
| `make migrate-down` | Roll back last migration |
| `make test` | Run Go tests (`go test ./... -v -race`) |
| `make build` | Build Go binary to `bin/clientroll` |
| `make frontend` | Build frontend into `web/` |
| `make clean` | Remove `bin/`, `web/assets/`, frontend cache |

---

## 6. API Overview

Base path: **`/api`**. The frontend uses `API_BASE = "/api"` in `frontend/src/api/client.ts`.

### Public (no auth)

- **Auth:** `POST /api/auth/login`, `POST /api/auth/platform-login`, `POST /api/auth/refresh`, `POST /api/auth/logout`
- **Files:** Public signup images (see `fileHandler.RegisterPublicRoutes`)
- **Signup:** Public signup flow (see `signupHandler.RegisterPublicRoutes`)
- **GoCardless:** OAuth callback and webhook (see `gocardlessHandler.RegisterPublicRoutes`)
- **Stripe:** Webhook (see `stripeWebhookHandler.RegisterPublicRoutes`)
- **Inbound webhooks:** `POST /api/webhooks/inbound/clients` (see [WEBHOOKS.md](WEBHOOKS.md))
- **Slug check:** `GET /api/register/check-slug/{slug}`
- **Registration / password reset / email verify:** Under rate-limited group (`Register`, password reset, verify routes)

### Payment webhook event matrix

| Provider | Incoming event/action | Internal payment status | Outbound event |
|----------|------------------------|--------------------------|----------------|
| Stripe | `checkout.session.completed` | `succeeded` | `payment.succeeded` |
| Stripe | `checkout.session.expired` | `failed` | `payment.failed` |
| Stripe | `invoice.finalized` | `pending` (open invoice, e.g. BECS still clearing) | — |
| Stripe | `invoice.payment_succeeded` | `succeeded` | `payment.succeeded` |
| Stripe | `invoice.payment_failed` | `failed` | `payment.failed` |
| Stripe | `payment_intent.processing`, `payment_intent.succeeded`, `payment_intent.payment_failed` | (no status change by itself) merges `payments.stripe_details` | — |
| Stripe | `charge.succeeded`, `charge.updated` | merges BECS / mandate / charge id into `payments.stripe_details` | — |
| Stripe | `charge.dispute.created`, `charge.dispute.updated`, `charge.dispute.closed` | `disputed` (or `succeeded` when dispute is won) | `payment.disputed` |
| Stripe | `charge.refunded`, `refund.updated` (succeeded only) | `refunded` | `payment.refunded` |
| Stripe | `customer.subscription.deleted` | membership `cancelled` | `membership.cancelled` |
| GoCardless | `payments.paid_out` | `succeeded` | `payment.succeeded` |
| GoCardless | `payments.failed`, `payments.late_failure_settled` | `failed` | `payment.failed` |
| GoCardless | `payments.charged_back`, `payments.chargeback_settled` | `disputed` | `payment.disputed` |
| GoCardless | `payments.cancelled` | `cancelled` | none |
| GoCardless | `refunds.created`, `refunds.submitted`, `refunds.paid`, `refunds.refunded` | `refunded` | `payment.refunded` |
| GoCardless | `mandates.cancelled`, `mandates.failed`, `mandates.expired` | mandate link cleared on memberships | none |
| GoCardless | `subscriptions.cancelled`, `subscriptions.finished` | membership `cancelled`/`expired` | `membership.cancelled`/`membership.expired` |

**Stripe setup:** In **Developers → Webhooks** for your tenant endpoint (`/api/webhooks/stripe/{slug}`), ensure **`invoice.finalized`** is selected. Without it, subscription invoices that are still collecting (e.g. BECS) only appear in ClientRoll after **`invoice.payment_succeeded`**. For the **Payments → Stripe details** panel (invoice #, PaymentIntent, BSB/last4 when Stripe sends them), also send **`payment_intent.processing`**, **`payment_intent.succeeded`**, **`payment_intent.payment_failed`**, **`charge.succeeded`**, and **`charge.updated`**.

### Authenticated

- **Current user:** `GET /api/me` — returns `user_id`, `tenant_id`, `role` (works for tenant and platform admin).
- **Tenant-scoped:** All under `GET /api/tenant/*` with `RequireTenant`:
  - Settings, staff, bonuses, membership types, terms, clients, payments, reports, active memberships, services, integrations, GoCardless tenant routes, webhooks, inbound webhook keys, file uploads.
- **Platform admin:** Under `GET /api/platform/*` with `RequirePlatformAdmin`:
  - Tenants CRUD (list, create, get, update). Phase 8 billing is a placeholder.

### Health

- **`GET /api/health`** — Returns JSON with `status`, `service`, and `payment_metrics` (webhook and reconciliation counters).

### Handler pattern

- Handlers live in `internal/handlers/`.
- Each handler is constructed with repos and dependencies, then registers routes via `RegisterRoutes(r chi.Router)` (some also have `RegisterPublicRoutes`, `RegisterTenantRoutes`, or `RegisterAuthRoutes`).
- Shared helpers: `writeJSON`, `writeError`, `decodeJSON` in `handlers/helpers.go`.
- Tenant ID and user ID come from context set by `authMW.RequireAuth` and `tenantMW.SetTenantContext` / `tenantMW.RequireTenant`.

---

## 7. Auth and Multi-Tenancy

- **JWT:** Access token (short-lived) and refresh token (long-lived). Refresh tokens are stored in Redis and validated on refresh.
- **Sessions:** Session manager in `internal/auth/sessions.go` uses Redis.
- **CSRF:** State-changing requests require `X-CSRF-Token` header; token in cookie. See `middleware/csrf.go`.
- **Rate limiting:** Login has a strict limit (e.g. 10/15min); registration/password reset/email verify use a looser limit (e.g. 120/15min). See `middleware/ratelimit.go`.
- **Tenant context:** After auth, `tenantMW.SetTenantContext` sets tenant from JWT/session; routes under `/api/tenant` use `tenantMW.RequireTenant`. Tenant ID is used for all tenant-scoped repos and RLS.
- **Platform admin:** Separate role; `/api/platform` routes require `RequirePlatformAdmin`. Platform admins can manage tenants without a tenant context.

---

## 8. Backend Conventions

- **Repositories:** Data access in `internal/repository/`. Each entity has a repo (e.g. `UserRepo`, `TenantRepo`, `ClientRepo`). Repos take `*database.Postgres` or `*database.Redis` and return errors for the handler to translate to HTTP.
- **Models:** Request/response DTOs and shared structs in `internal/models/`.
- **Events:** `internal/events/dispatcher.go` dispatches events; worker in `events/` processes jobs (e.g. outbound webhook delivery). Event types in `events/types.go` (e.g. `client.created`, `membership.created`, `payment.succeeded`). Handlers receive the dispatcher via setters (e.g. `SetDispatcher`) and enqueue work.
- **Encryption:** Tenant secrets (e.g. GoCardless tokens) are encrypted at rest with `database.Encryptor` using `MASTER_ENCRYPTION_KEY`.
- **Migrations:** Numbered `NNN_description.up.sql` and `.down.sql`. Apply with `go run ./cmd/migrate up` (or `make migrate-up`). Schema version tracked in `schema_migrations`.

### Agreement snapshots and PDF retrieval

- **Snapshot lifecycle:** A signed agreement HTML snapshot is created when membership acceptance happens in signup flows and for staff-created memberships. Historical memberships can be populated with `cmd/backfill-agreements`.
- **Tenant-scoped storage:** Agreement data is stored in `membership_agreement_snapshots` and accepted terms in `membership_accepted_terms`; both are tenant-scoped and covered by RLS policy migration updates.
- **Render model:** The system stores canonical agreement HTML at acceptance time, then renders PDF on demand from that stored HTML instead of rebuilding from current membership settings.
- **Download route:** `GET /api/tenant/memberships/{id}/agreement.pdf` returns the PDF (and should align with the agreement details UI route behavior).
- **Runtime dependency:** PDF generation uses `chromedp`; production hosts must have `google-chrome`, `chromium`, or `chromium-browser` available on `PATH`.
- **Failure mode:** If Chromium is unavailable, PDF generation fails while the stored HTML snapshot still exists. Troubleshooting starts with server logs and host Chromium availability checks.
- **Backfill runbook:** Use `go run ./cmd/backfill-agreements --dry-run` first, then rerun without `--dry-run`; scope with `--tenant <slug>` and `--limit <n>` when needed.

---

## 9. Frontend Conventions

- **Path alias:** `@/` maps to `src/` (see `frontend/tsconfig.json` and Rspack).
- **API client:** `frontend/src/api/client.ts` — adds Bearer token and CSRF for non-GET requests, retries on 401 via refresh, redirects to `/login` on auth failure for non-auth endpoints. Exposes typed functions for each API area.
- **Auth state:** `store/AuthContext.tsx` provides user/tenant/role and login/logout.
- **Routing:** React Router v6. `AppShell` wraps authenticated app routes; `RequirePlatformAdmin` wraps platform routes. Default `/` → `/dashboard`; catch-all → `/login`.
- **Build:** Rspack builds from `frontend/src/main.tsx` into `../web` with hashed assets. Production build: `make frontend` or `cd frontend && npm run build`.

---

## 10. Database and Migrations

- **Migrations:** Numbered migration pairs live in `migrations/` (extensions, platform, tenant, client, membership, payments, webhooks, RLS, email verifications, services, GoCardless, agreement snapshots, inbound webhook keys, idempotency, and flexible payment schedules).
- **RLS:** Migration `009_enable_rls.up.sql` enables row-level security for tenant isolation where applicable.
- **Run migrations:** `make migrate-up` (dev) or on server with `DATABASE_URL` set: `go run ./cmd/migrate up`. Rollback: `make migrate-down` or `go run ./cmd/migrate down`.

---

## 11. Testing

- **Go:** Unit and integration tests in `internal/` (e.g. `events/inbound_auth_test.go`, `payment/gocardless_*_test.go`, `handlers/inbound_webhooks_test.go`, `handlers/gocardless_helpers_test.go`). Run: `make test` or `go test ./... -v -race`.

---

## 12. Build and Deploy

- **Build backend:** `make build` → `bin/clientroll`.
- **Build frontend:** `make frontend` → output in `web/`.
- **Docker:** `make docker-build` / `docker-up` / `docker-down`. Production compose uses host DB/Redis via env.
- **Deploy:** Prefer **Git → push to GitHub → on server pull → `./scripts/deploy-on-server.sh`** (see [DEPLOY.md](DEPLOY.md)). Alternative: build locally and SCP with `DEPLOY_HOST=... DEPLOY_PATH=... ./deploy.sh`.
- **Server layout (deploy script):** Source repo e.g. `/var/www/clientroll-repo`, deploy path e.g. `/var/www/clientroll`. Script builds backend and frontend, runs migrations if `DATABASE_URL` is set, swaps binary and `frontend_dist`, restarts systemd service `clientroll` (or `DEPLOY_SERVICE`).

---

## 13. Key Documentation Index

| Document | Purpose |
|----------|---------|
| **DEVELOPMENT.md** (this file) | Architecture, setup, conventions, API overview |
| [IMPLEMENTATION-GAPS.md](IMPLEMENTATION-GAPS.md) | Internal review of placeholders / missing implementations |
| [ENV-REFERENCE.md](ENV-REFERENCE.md) | Environment variables quick reference |
| [DEPLOY.md](DEPLOY.md) | Deployment flows (Git + server script vs local build + SCP) |
| [WEBHOOKS.md](WEBHOOKS.md) | Inbound (keys, endpoint, payload) and outbound (events, retries) |
| [GOCARDLESS_DEPLOY.md](GOCARDLESS_DEPLOY.md) | GoCardless Connect setup and env checklist |

---

## 14. Phases (product context)

Comments in code and frontend reference phases (e.g. Phase 3 settings/staff, Phase 4 core entities, Phase 6 payments/reports, Phase 7 webhooks, Phase 8 platform billing). These are product milestones; Phase 8 platform billing is currently a placeholder UI.
