# ClientRoll

Multi-tenant membership and client management with recurring payments (GoCardless), one-off charges (Stripe), and email (MailerSend).

## Quick start (development)

```bash
make dev-db          # Start Postgres + Redis
make migrate-up       # Run migrations
make dev              # Start API on :8080
make frontend-dev     # Start frontend on :3000 (in another terminal)
```

Open http://localhost:3000.

## Documentation

| Doc | Description |
|-----|-------------|
| [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) | **Internal dev guide** — architecture, agreement snapshot lifecycle, API, auth, conventions |
| [docs/IMPLEMENTATION-GAPS.md](docs/IMPLEMENTATION-GAPS.md) | Internal review — placeholders and missing implementations |
| [docs/ENV-REFERENCE.md](docs/ENV-REFERENCE.md) | Environment variables |
| [docs/DEPLOY.md](docs/DEPLOY.md) | Deployment + production checks (includes Chromium requirement for agreement PDFs) |
| [docs/WEBHOOKS.md](docs/WEBHOOKS.md) | Inbound and outbound webhooks |
| [docs/GOCARDLESS_DEPLOY.md](docs/GOCARDLESS_DEPLOY.md) | GoCardless Connect setup |

## Stack

- **Backend:** Go 1.22, Chi, PostgreSQL (pgx), Redis
- **Frontend:** React 18, TypeScript, Rspack, Tailwind
- **Payments:** GoCardless (primary), Stripe (webhooks)
- **Documents/PDF:** Agreement HTML snapshots + on-demand PDF generation (`chromedp`, requires Chromium/Chrome on host)

## Commands

| Command | Description |
|---------|-------------|
| `make help` | List all make targets |
| `make dev` | Run API (dev DB/Redis from Makefile) |
| `make frontend-dev` | Run frontend dev server |
| `make build` | Build Go binary to `bin/clientroll` |
| `make frontend` | Build frontend to `web/` |
| `make test` | Go tests |
| `make migrate-up` / `make migrate-down` | Migrations |
| `go run ./cmd/backfill-agreements --dry-run` | Preview memberships missing agreement snapshots |
