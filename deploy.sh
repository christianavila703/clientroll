#!/usr/bin/env bash
#
# Build locally and deploy to a remote server over SSH.
# Use this when the server has no Go/Node toolchain; otherwise prefer
# scripts/deploy-on-server.sh.
#
# Required env:
#   DEPLOY_HOST     e.g. user@your-server
#   DEPLOY_PATH     e.g. /var/www/clientroll (must contain .env with DATABASE_URL)
# Optional env:
#   DEPLOY_SSH_KEY  path to SSH private key
#   DEPLOY_SERVICE  systemd unit (default: clientroll)
#   SKIP_MIGRATIONS=1  skip migrations (emergency binary-only rollback)
#
set -euo pipefail
cd "$(dirname "$0")"

DEPLOY_HOST="${DEPLOY_HOST:-}"
DEPLOY_PATH="${DEPLOY_PATH:-}"
SERVICE="${DEPLOY_SERVICE:-clientroll}"

if [[ -z "$DEPLOY_HOST" || -z "$DEPLOY_PATH" ]]; then
  echo "Usage: DEPLOY_HOST=user@host DEPLOY_PATH=/var/www/clientroll [DEPLOY_SSH_KEY=path] ./deploy.sh"
  exit 1
fi

SSH_OPTS=(-o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new)
[[ -n "${DEPLOY_SSH_KEY:-}" ]] && SSH_OPTS+=(-i "$DEPLOY_SSH_KEY")
remote() { ssh "${SSH_OPTS[@]}" "$DEPLOY_HOST" "$@"; }

BUILD_DIR="$(mktemp -d)"
trap 'rm -rf "$BUILD_DIR"' EXIT

GIT_SHA="$(git rev-parse --short HEAD)"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Building $GIT_SHA ($BUILD_TIME)..."

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$BUILD_DIR/server_new" ./cmd/server/
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$BUILD_DIR/billing-snapshot_new" ./cmd/billing-snapshot/
(cd frontend && APP_BUILD_SHA="$GIT_SHA" APP_BUILD_TIME="$BUILD_TIME" npm run build)
echo "$GIT_SHA $BUILD_TIME" > web/build.txt

if [[ "${SKIP_MIGRATIONS:-0}" == "1" ]]; then
  echo "Skipping migrations (SKIP_MIGRATIONS=1)."
else
  echo "Running migrations against the server's DATABASE_URL..."
  REMOTE_DATABASE_URL="$(remote "set -a; . '$DEPLOY_PATH/.env'; printf '%s' \"\${DATABASE_URL:-}\"")"
  if [[ -z "$REMOTE_DATABASE_URL" ]]; then
    echo "DATABASE_URL not found in $DEPLOY_PATH/.env on $DEPLOY_HOST."
    exit 1
  fi
  DATABASE_URL="$REMOTE_DATABASE_URL" go run ./cmd/migrate up
fi

echo "Uploading..."
remote "rm -rf '$DEPLOY_PATH/web_new' && mkdir -p '$DEPLOY_PATH/web_new'"
scp "${SSH_OPTS[@]}" "$BUILD_DIR/server_new" "$BUILD_DIR/billing-snapshot_new" "$DEPLOY_HOST:$DEPLOY_PATH/"
scp "${SSH_OPTS[@]}" -r web/. "$DEPLOY_HOST:$DEPLOY_PATH/web_new/"

echo "Switching release and restarting $SERVICE..."
remote "bash -s -- '$DEPLOY_PATH' '$SERVICE' '$GIT_SHA $BUILD_TIME'" << 'REMOTE'
set -euo pipefail
DEPLOY_PATH="$1"; SERVICE="$2"; BUILD="$3"
cd "$DEPLOY_PATH"
sudo systemctl stop "$SERVICE" || true
mv -f server_new server && chmod +x server
mv -f billing-snapshot_new billing-snapshot && chmod +x billing-snapshot
# Keep bin/clientroll in sync if the systemd unit runs it from there.
[[ -d bin ]] && cp -f server bin/clientroll
rm -rf web_old
[[ -d web ]] && mv web web_old
mv web_new web
echo "$BUILD" > BUILD_SHA
sudo systemctl start "$SERVICE"
if ! sudo systemctl is-active --quiet "$SERVICE"; then
  sudo systemctl status "$SERVICE" --no-pager -l || true
  echo "Service $SERVICE failed to start."
  exit 1
fi
REMOTE

echo "Deploy complete: $GIT_SHA"
