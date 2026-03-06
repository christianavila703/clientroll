#!/usr/bin/env bash
# Build from one repo path and deploy into another server path.
# Hardcoded server layout:
#   Source repo: /var/www/clientroll-repo
#   Deploy path: /var/www/clientroll
#
# Optional env:
#   DEPLOY_SERVICE  (default: clientroll)
#   SKIP_PULL=1     (skip git pull in source repo)

set -euo pipefail

SOURCE_REPO="/var/www/clientroll-repo"
DEPLOY_PATH="/var/www/clientroll"
SERVICE="${DEPLOY_SERVICE:-clientroll}"

echo "Source repo: $SOURCE_REPO"
echo "Deploy path: $DEPLOY_PATH"
echo "Service: $SERVICE"

cd "$SOURCE_REPO"

if [[ "${SKIP_PULL:-0}" != "1" ]]; then
  echo "Pulling latest code in source repo..."
  git pull --ff-only
fi

echo "Building backend binary..."
CGO_ENABLED=0 go build -o server_new ./cmd/server/

echo "Building frontend assets..."
(
  cd frontend
  npm run build
)

# Load env for migration from deploy path first, then source repo as fallback.
if [[ -f "$DEPLOY_PATH/.env" ]]; then
  set -a
  source "$DEPLOY_PATH/.env"
  set +a
elif [[ -f "$SOURCE_REPO/.env" ]]; then
  set -a
  source "$SOURCE_REPO/.env"
  set +a
fi

echo "Running migrations..."
if [[ -n "${DATABASE_URL:-}" ]]; then
  go run ./cmd/migrate up || true
else
  echo "Skipping migration (DATABASE_URL not set)."
fi

echo "Preparing deploy directories..."
mkdir -p "$DEPLOY_PATH"
mkdir -p "$DEPLOY_PATH/frontend_dist_new"
rm -rf "$DEPLOY_PATH/frontend_dist_new"/*

echo "Copying backend binary..."
cp -f "$SOURCE_REPO/server_new" "$DEPLOY_PATH/server_new"

echo "Copying frontend bundle from web/..."
cp -a "$SOURCE_REPO/web/." "$DEPLOY_PATH/frontend_dist_new/"

echo "Switching release files..."
sudo systemctl stop "$SERVICE" 2>/dev/null || true
mv -f "$DEPLOY_PATH/server_new" "$DEPLOY_PATH/server"
chmod +x "$DEPLOY_PATH/server" 2>/dev/null || true
rm -rf "$DEPLOY_PATH/frontend_dist_old"
[[ -d "$DEPLOY_PATH/frontend_dist" ]] && mv "$DEPLOY_PATH/frontend_dist" "$DEPLOY_PATH/frontend_dist_old"
mv -f "$DEPLOY_PATH/frontend_dist_new" "$DEPLOY_PATH/frontend_dist"

# If your systemd unit points at bin/clientroll, keep it in sync.
if [[ -d "$DEPLOY_PATH/bin" ]]; then
  cp -f "$DEPLOY_PATH/server" "$DEPLOY_PATH/bin/clientroll" 2>/dev/null || true
  chmod +x "$DEPLOY_PATH/bin/clientroll" 2>/dev/null || true
fi

echo "Starting $SERVICE..."
sudo systemctl start "$SERVICE" 2>/dev/null || true

echo "Deploy complete."
