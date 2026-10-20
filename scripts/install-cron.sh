#!/usr/bin/env bash
#
# Install (or update) the daily cron jobs:
#   - database backup        (scripts/db-backup.sh)
#   - platform billing usage (billing-snapshot binary, if deployed)
#
# Re-running replaces existing entries instead of duplicating them.
#
# Optional env:
#   DEPLOY_PATH      (default: /var/www/clientroll)
#   BACKUP_SCHEDULE  (default: "15 2 * * *" — 02:15 daily)
#   BILLING_SCHEDULE (default: "15 3 * * *" — 03:15 daily)
#   LOG_DIR          (default: /var/log)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_PATH="${DEPLOY_PATH:-/var/www/clientroll}"
BACKUP_SCHEDULE="${BACKUP_SCHEDULE:-15 2 * * *}"
BILLING_SCHEDULE="${BILLING_SCHEDULE:-15 3 * * *}"
LOG_DIR="${LOG_DIR:-/var/log}"
BILLING_BIN="$DEPLOY_PATH/billing-snapshot"

BACKUP_MARKER="# clientroll-db-backup"
BILLING_MARKER="# clientroll-billing-snapshot"

lines=("$BACKUP_SCHEDULE $SCRIPT_DIR/db-backup.sh >> $LOG_DIR/clientroll-db-backup.log 2>&1 $BACKUP_MARKER")

if [[ -x "$BILLING_BIN" ]]; then
  if [[ ! -f "$DEPLOY_PATH/.env" ]]; then
    echo "ERROR: $DEPLOY_PATH/.env not found (billing-snapshot needs DATABASE_URL and MASTER_ENCRYPTION_KEY)."
    exit 1
  fi
  lines+=("$BILLING_SCHEDULE cd $DEPLOY_PATH && set -a && . ./.env && set +a && $BILLING_BIN >> $LOG_DIR/clientroll-billing-snapshot.log 2>&1 $BILLING_MARKER")
else
  echo "INFO: $BILLING_BIN not found; skipping billing snapshot job (deploy first to enable it)."
fi

# Drop any previous entries carrying our markers, then append the new ones.
{
  crontab -l 2>/dev/null | grep -vF -e "$BACKUP_MARKER" -e "$BILLING_MARKER" || true
  printf '%s\n' "${lines[@]}"
} | crontab -

echo "Installed cron jobs:"
printf '  %s\n' "${lines[@]}"
