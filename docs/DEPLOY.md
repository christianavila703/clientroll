# Deployment

Deployments should follow: **Git → push to GitHub → on server pull from GitHub → deploy on server**.

## 1. Push to GitHub

From your machine:

```bash
git add -A
git commit -m "Your message"
git push origin main
```

## 2. On the server: pull and deploy

SSH into the server, then from the app directory (e.g. `/var/www/clientroll`):

```bash
cd /var/www/clientroll   # or your DEPLOY_PATH
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
ssh root@203.29.242.238 'cd /var/www/clientroll && git pull origin main && ./scripts/deploy-on-server.sh'
```

## Alternative: build locally and upload (existing `deploy.sh`)

If you prefer to build on your machine and upload the binary and frontend:

```bash
DEPLOY_HOST=root@203.29.242.238 DEPLOY_PATH=/var/www/clientroll ./deploy.sh
```

This builds locally, SCPs the binary and `frontend/dist` to the server, runs migrations on the server (if the repo is present there), swaps files, and restarts the service. The server does **not** need to pull from GitHub for this flow.

## Summary

| Flow | When to use |
|------|-------------|
| **Git → GitHub → pull on server → `./scripts/deploy-on-server.sh`** | Standard: single source of truth in GitHub; server always builds from latest pull. |
| **`./deploy.sh`** (build locally, SCP) | When the server has no Go/Node or you want to avoid building on the server. |
