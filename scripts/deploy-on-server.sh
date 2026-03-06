#!/usr/bin/env bash
# Run this on the server after pulling from GitHub.
# Usage: from repo root on server: ./scripts/deploy-on-server.sh
# Or:    cd /var/www/clientroll && git pull origin main && ./scripts/deploy-on-server.sh
#
# Expects: repo cloned from GitHub at DEPLOY_PATH (default: current dir).
# Uses: .env (DATABASE_URL, etc.), systemd unit DEPLOY_SERVICE (default: clientroll).

set -e
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"
DEPLOY_PATH="${DEPLOY_PATH:-$REPO_ROOT}"
SERVICE="${DEPLOY_SERVICE:-clientroll}"

[[ -f .env ]] && set -a && source .env && set +a

echo "Pulling from GitHub..."
git pull --ff-only || { echo "Warning: git pull had conflicts or failed. Fix and re-run."; exit 1; }

echo "Building backend..."
CGO_ENABLED=0 go build -o server_new ./cmd/server/

echo "Building frontend..."
cd frontend && npm run build && cd ..

echo "Running migrations..."
if [[ -n "$DATABASE_URL" ]]; then
  go run ./cmd/migrate up || true
else
  echo "Skipping migration (no DATABASE_URL in .env)."
fi

echo "Swapping binary and frontend..."
mv -f server_new server 2>/dev/null || true
chmod +x server 2>/dev/null || true
rm -rf frontend_dist_old
[[ -d frontend_dist ]] && mv frontend_dist frontend_dist_old
mv frontend/dist frontend_dist

echo "Restarting $SERVICE..."
sudo systemctl restart "$SERVICE" 2>/dev/null || true

echo "Deploy complete."
