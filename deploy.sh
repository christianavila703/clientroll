#!/usr/bin/env bash
# Deploy ClientRoll to app.clientroll.com (or DEPLOY_HOST).
# Requires: DEPLOY_HOST (e.g. user@server), DEPLOY_PATH (e.g. /var/www/clientroll)
# Optional: DEPLOY_SSH_KEY (path to private key), DEPLOY_SERVICE (systemd unit, default clientroll)

set -e
cd "$(dirname "$0")"

if [[ -z "$DEPLOY_HOST" || -z "$DEPLOY_PATH" ]]; then
  echo "Usage: DEPLOY_HOST=user@host DEPLOY_PATH=/var/www/clientroll [DEPLOY_SSH_KEY=path] ./deploy.sh"
  echo "Example: DEPLOY_HOST=root@203.29.242.238 DEPLOY_PATH=/var/www/clientroll ./deploy.sh"
  exit 1
fi

SERVICE="${DEPLOY_SERVICE:-clientroll}"
SSH_OPTS=(-o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new)
[[ -n "$DEPLOY_SSH_KEY" ]] && SSH_OPTS+=(-i "$DEPLOY_SSH_KEY")

echo "Building Linux binary..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server_linux_amd64 ./cmd/server/

echo "Building frontend..."
cd frontend && npm run build && cd ..

echo "Uploading binary..."
scp "${SSH_OPTS[@]}" server_linux_amd64 "$DEPLOY_HOST:$DEPLOY_PATH/server_new"

echo "Preparing remote dirs..."
ssh "${SSH_OPTS[@]}" "$DEPLOY_HOST" "mkdir -p $DEPLOY_PATH/web_new"
echo "Uploading frontend (web/)..."
scp "${SSH_OPTS[@]}" -r web/* "$DEPLOY_HOST:$DEPLOY_PATH/web_new/"

echo "Running migration (if repo present) and switching binary + frontend on server..."
ssh "${SSH_OPTS[@]}" "$DEPLOY_HOST" "bash -s -- '$DEPLOY_PATH' '$SERVICE'" << 'REMOTE'
  set -e
  DEPLOY_PATH="$1"
  SERVICE="$2"
  cd "$DEPLOY_PATH" || exit 1
  [[ -f .env ]] && set -a && source .env && set +a
  [[ -z "$DATABASE_URL" && -f .env.production ]] && set -a && source .env.production && set +a
  if [[ -n "$DATABASE_URL" && -f cmd/migrate/main.go ]]; then
    echo "Running migrations..."
    go run ./cmd/migrate up || true
  else
    echo "Skipping migration: DATABASE_URL not set in $DEPLOY_PATH/.env (or no repo at $DEPLOY_PATH)."
  fi
  # Stop service before replacing binary (avoid "Text file busy")
  sudo systemctl stop "$SERVICE" 2>/dev/null || true
  mv -f server_new server 2>/dev/null || true
  chmod +x server 2>/dev/null || true
  # Also copy to bin/clientroll if that's what systemd uses
  if [[ -d "$DEPLOY_PATH/bin" ]]; then
    cp -f server "$DEPLOY_PATH/bin/clientroll" 2>/dev/null || true
    chmod +x "$DEPLOY_PATH/bin/clientroll" 2>/dev/null || true
  fi
  rm -rf web_old
  [[ -d web ]] && mv web web_old
  mv -f web_new web
  echo "Starting $SERVICE..."
  sudo systemctl start "$SERVICE" 2>/dev/null || true
  echo "Deploy done."
REMOTE

echo "Deploy complete."
