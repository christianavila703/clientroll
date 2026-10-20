# Deployment

Deployments should follow: **Git → push to GitHub → on server pull from GitHub → deploy on server**.

For high-safety releases, always include:

- Automatic DB migration execution (no manual psql step)
- Production smoke checks immediately after deploy
- A runbook drill path for replay/reconcile endpoints
- Annotated release tag for rollback target

## 1. Push to GitHub

From your machine:

```bash
git add -A
git commit -m "Your message"
git push origin main
```

## 2. On the server: pull and deploy

**Layout (default in `scripts/deploy-on-server.sh`):**

| Path | Role |
|------|------|
| `/var/www/clientroll-repo` | **Git clone** — run `git pull` and `./scripts/deploy-on-server.sh` here (`SOURCE_REPO`) |
| `/var/www/clientroll` | **Runtime deploy** — binary, `web/`, often `.env` (`DEPLOY_PATH`) |

SSH into the server, then from the **repo** directory:

```bash
cd /var/www/clientroll-repo
git pull origin main
chmod +x scripts/deploy-on-server.sh   # first time only
./scripts/deploy-on-server.sh
```

The script will:

- `git pull --ff-only` (if you didn’t pull above)
- Build the Go backend → `server`
- Build the frontend (`npm run build`) → `frontend/dist`
- Run migrations (`go run ./cmd/migrate up`) if `DATABASE_URL` is set in `.env`
- Replace the running binary and frontend with the new build
- Restart the systemd service (`clientroll` by default)

### Environment on the server

- **Agreement PDF downloads** (`/api/tenant/memberships/{id}/agreement.pdf`) use headless **Chromium** via `chromedp`. Install a Chromium/Chrome build on the server and ensure `google-chrome`, `chromium`, or `chromium-browser` is on `PATH`, or PDF generation returns 500. Signed agreement **HTML** is always stored in `membership_agreement_snapshots`; PDFs are rendered on demand.
- **`.env`** in the repo root must define at least `DATABASE_URL`. Other vars (e.g. `APP_URL`, `GOCARDLESS_WEBHOOK_SECRET`) go here too.
- **Frontend deps**: after `package.json` or `package-lock.json` changes, run `cd frontend && npm ci` on the server once so the next `npm run build` uses the right deps.
- **Systemd**: if your service name is not `clientroll`, set `DEPLOY_SERVICE` before running the script:
  ```bash
  export DEPLOY_SERVICE=my-app
  ./scripts/deploy-on-server.sh
  ```

### One-liner (from your machine via SSH)

You can trigger a pull and deploy from your machine:

```bash
ssh root@YOUR_SERVER_IP 'cd /var/www/clientroll && git pull origin main && ./scripts/deploy-on-server.sh'
```

## Alternative: build locally and upload (existing `deploy.sh`)

If you prefer to build on your machine and upload the binary and frontend:

```bash
DEPLOY_HOST=root@YOUR_SERVER_IP DEPLOY_PATH=/var/www/clientroll ./deploy.sh
```

This builds locally, SCPs the binary and `frontend/dist` to the server, runs migrations on the server (if the repo is present there), swaps files, and restarts the service. The server does **not** need to pull from GitHub for this flow.

`deploy.sh` now resolves `DATABASE_URL` from server env and runs `go run ./cmd/migrate up` locally against the production DB **before** the service swap, so migration execution is not dependent on source code existing at `DEPLOY_PATH`.

If you must skip migrations for an emergency binary-only rollback:

```bash
SKIP_MIGRATIONS=1 DEPLOY_HOST=root@YOUR_SERVER_IP DEPLOY_PATH=/var/www/clientroll ./deploy.sh
```

## Production smoke checks

Use the smoke script after every deploy:

```bash
APP_URL=https://app.clientroll.com ./scripts/smoke-prod.sh
```

To also verify signed signup URL enforcement:

```bash
APP_URL=https://app.clientroll.com DEPLOY_HOST=root@YOUR_SERVER_IP TENANT_SLUG=your-tenant ./scripts/smoke-prod.sh
```

This checks:

- `GET /api/health` returns `200` and `status=ok`
- Public signup blocks unsigned requests and accepts signed requests

## Runbook drill (replay/reconcile)

Exercise internal recovery endpoints with:

```bash
APP_URL=https://app.clientroll.com PLATFORM_TOKEN=<platform-jwt> ./scripts/runbook-drill.sh
```

Optional tenant scope:

```bash
APP_URL=https://app.clientroll.com PLATFORM_TOKEN=<platform-jwt> TENANT_ID=<uuid> ./scripts/runbook-drill.sh
```

This validates operational control-plane access and output for:

- `GET /api/platform/ops/health-snapshot`
- `POST /api/platform/webhooks/stripe/replay-failures` (dry run)
- `POST /api/platform/ops/reconcile-runbook`

## Release hygiene

Before production deployment:

1. Ensure branch is reviewed and up to date with remote.
2. Use a clean working tree for the release commit.
3. Create an annotated tag after successful smoke + runbook drill:

```bash
git tag -a vYYYY.MM.DD-1 -m "ClientRoll production release"
git push origin vYYYY.MM.DD-1
```

## Summary

| Flow | When to use |
|------|-------------|
| **Git → GitHub → pull on server → `./scripts/deploy-on-server.sh`** | Standard: single source of truth in GitHub; server always builds from latest pull. |
| **`./deploy.sh`** (build locally, SCP) | When the server has no Go/Node or you want to avoid building on the server. |

---

## Platform billing cron (daily usage snapshot)

Platform billing counts each tenant's active members daily and pushes the quantity to Stripe so invoices reflect current usage.

**Binary:** `cmd/billing-snapshot` (deployed to `$DEPLOY_PATH/billing-snapshot`)

**Required env** (same as the API):

| Variable | Purpose |
|----------|---------|
| `DATABASE_URL` | PostgreSQL |
| `MASTER_ENCRYPTION_KEY` | Decrypt platform Stripe keys |

**Each run:**

1. Loads `platform_billing_config` — exits cleanly if billing is inactive
2. Counts active memberships per active tenant → `tenant_usage_snapshots`
3. Updates `tenant_subscriptions.active_member_count` locally
4. For each tenant with a Stripe subscription: `subscription.Update(quantity = active members)`
5. Stripe invoices at period end based on quantity

Runs in ~30 seconds; safe to retry. Exits non-zero if any Stripe update fails (for cron alerting).

### Manual run (development)

```bash
make billing-snapshot
# or
make build-billing-snapshot && DATABASE_URL=... MASTER_ENCRYPTION_KEY=... ./bin/billing-snapshot
go run ./cmd/billing-snapshot -dry-run   # check config without writing
```

### Production install (cron)

After deploy (`./scripts/deploy-on-server.sh` copies `billing-snapshot` to `/var/www/clientroll/`):

```bash
chmod +x scripts/install-billing-snapshot-cron.sh
DEPLOY_PATH=/var/www/clientroll ./scripts/install-billing-snapshot-cron.sh
```

Default schedule: **03:15 UTC daily**. Override with `CRON_SCHEDULE="0 4 * * *"`.

Logs: `/var/log/clientroll-billing-snapshot.log`

### Alternative: systemd timer

If you prefer systemd over cron, create `/etc/systemd/system/clientroll-billing-snapshot.service`:

```ini
[Unit]
Description=ClientRoll platform billing snapshot
After=network-online.target

[Service]
Type=oneshot
WorkingDirectory=/var/www/clientroll
EnvironmentFile=/var/www/clientroll/.env
ExecStart=/var/www/clientroll/billing-snapshot
```

Timer `/etc/systemd/system/clientroll-billing-snapshot.timer`:

```ini
[Unit]
Description=Run ClientRoll billing snapshot daily

[Timer]
OnCalendar=*-*-* 03:15:00
Persistent=true
RandomizedDelaySec=300

[Install]
WantedBy=timers.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now clientroll-billing-snapshot.timer
```
