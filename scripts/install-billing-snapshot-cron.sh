#!/usr/bin/env bash
#
# Install a daily cron job for the platform billing snapshot binary.
#
# Optional env:
#   CRON_SCHEDULE (default: "15 3 * * *" — 03:15 UTC daily)
#   SNAPSHOT_LOG  (default: /var/log/clientroll-billing-snapshot.log)
#   DEPLOY_PATH   (default: /var/www/clientroll)
#
set -euo pipefail

DEPLOY_PATH="${DEPLOY_PATH:-/var/www/clientroll}"
BINARY_PATH="${BINARY_PATH:-$DEPLOY_PATH/billing-snapshot}"
CRON_SCHEDULE="${CRON_SCHEDULE:-15 3 * * *}"
SNAPSHOT_LOG="${SNAPSHOT_LOG:-/var/log/clientroll-billing-snapshot.log}"
MARKER="# clientroll-billing-snapshot"

if [[ ! -x "$BINARY_PATH" ]]; then
  echo "billing-snapshot binary not found or not executable: $BINARY_PATH"
  echo "Build and deploy it first (make build-billing-snapshot or ./scripts/deploy-on-server.sh)."
  exit 1
fi

if [[ ! -f "$DEPLOY_PATH/.env" ]]; then
  echo ".env not found at $DEPLOY_PATH/.env (needs DATABASE_URL and MASTER_ENCRYPTION_KEY)"
  exit 1
fi

CRON_LINE="$CRON_SCHEDULE cd $DEPLOY_PATH && set -a && . ./.env && set +a && $BINARY_PATH >> $SNAPSHOT_LOG 2>&1 $MARKER"

current_cron="$(crontab -l 2>/dev/null || true)"
new_cron="$(printf "%s\n" "$current_cron" | awk -v m="$MARKER" 'index($0,m)==0')"
new_cron="${new_cron}
$CRON_LINE"

printf "%s\n" "$new_cron" | sed '/^[[:space:]]*$/N;/^\n$/D' | crontab -

echo "Installed billing snapshot cron job:"
echo "$CRON_LINE"
