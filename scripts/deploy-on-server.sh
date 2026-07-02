#!/usr/bin/env bash
# Build from one repo path and deploy into another server path.
# Hardcoded server layout:
#   Source repo: /var/www/clientroll-repo
#   Deploy path: /var/www/clientroll
#
# Optional env:
#   SOURCE_REPO     (override source repo path)
#   DEPLOY_PATH     (override deploy destination path)
#   DEPLOY_SERVICE  (default: clientroll)
#   SKIP_PULL=1     (skip git pull in source repo)

set -euo pipefail

SOURCE_REPO="${SOURCE_REPO:-/var/www/clientroll-repo}"
DEPLOY_PATH="${DEPLOY_PATH:-/var/www/clientroll}"
SERVICE="${DEPLOY_SERVICE:-clientroll}"
BUILD_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$BUILD_DIR" 2>/dev/null || true
}
trap cleanup EXIT

echo "Source repo: $SOURCE_REPO"
echo "Deploy path: $DEPLOY_PATH"
echo "Service: $SERVICE"
echo "Build dir: $BUILD_DIR"

cd "$SOURCE_REPO"

if [[ "${SKIP_PULL:-0}" != "1" ]]; then
  echo "Pulling latest code in source repo..."
  git pull --ff-only
fi

GIT_SHA="$(git rev-parse --short HEAD)"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Building from git SHA: $GIT_SHA ($BUILD_TIME)"

echo "Building backend binary..."
CGO_ENABLED=0 go build -o "$BUILD_DIR/server_new" ./cmd/server/

echo "Building frontend assets..."
(
  cd frontend
  APP_BUILD_SHA="$GIT_SHA" APP_BUILD_TIME="$BUILD_TIME" npm run build
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

if [[ -n "${DATABASE_URL:-}" ]] && [[ -f "$SOURCE_REPO/scripts/db-backup.sh" ]]; then
  echo "Creating pre-deploy DB backup..."
  SOURCE_REPO="$SOURCE_REPO" DEPLOY_PATH="$DEPLOY_PATH" "$SOURCE_REPO/scripts/db-backup.sh" || true
fi

echo "Running migrations..."
if [[ -n "${DATABASE_URL:-}" ]]; then
  go run ./cmd/migrate up
else
  echo "DATABASE_URL is required for migrations."
  exit 1
fi

echo "Preparing deploy directories..."
mkdir -p "$DEPLOY_PATH"
mkdir -p "$DEPLOY_PATH/web_new"
rm -rf "$DEPLOY_PATH/web_new"/*

echo "Copying backend binary..."
cp -f "$BUILD_DIR/server_new" "$DEPLOY_PATH/server_new"

echo "Staging frontend bundle from web/..."
cp -a "$SOURCE_REPO/web/." "$DEPLOY_PATH/web_new/"

# Write a build marker into the deployed frontend so you can verify what is live.
echo "$GIT_SHA $BUILD_TIME" > "$DEPLOY_PATH/web_new/build.txt"
echo "$GIT_SHA $BUILD_TIME" > "$DEPLOY_PATH/BUILD_SHA"

echo "Switching release files..."
if sudo systemctl is-active --quiet "$SERVICE"; then
  sudo systemctl stop "$SERVICE"
fi
mv -f "$DEPLOY_PATH/server_new" "$DEPLOY_PATH/server"
chmod +x "$DEPLOY_PATH/server"
rm -rf "$DEPLOY_PATH/web_old"
[[ -d "$DEPLOY_PATH/web" ]] && mv "$DEPLOY_PATH/web" "$DEPLOY_PATH/web_old"
mv -f "$DEPLOY_PATH/web_new" "$DEPLOY_PATH/web"

# If your systemd unit points at bin/clientroll, keep it in sync.
if [[ -d "$DEPLOY_PATH/bin" ]]; then
  cp -f "$DEPLOY_PATH/server" "$DEPLOY_PATH/bin/clientroll"
  chmod +x "$DEPLOY_PATH/bin/clientroll"
fi

echo "Service ExecStart (for verification):"
systemctl show -p ExecStart "$SERVICE" 2>/dev/null || true

echo "Starting $SERVICE..."
sudo systemctl start "$SERVICE"
if ! sudo systemctl is-active --quiet "$SERVICE"; then
  echo "Service $SERVICE failed to start."
  sudo systemctl status "$SERVICE" --no-pager -l || true
  exit 1
fi

echo "Deploy complete."
