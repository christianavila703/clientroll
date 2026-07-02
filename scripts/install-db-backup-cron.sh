#!/usr/bin/env bash
#
# Install a daily cron job for db-backup.sh.
#
# Optional env:
#   CRON_SCHEDULE (default: "15 2 * * *")
#   BACKUP_LOG    (default: /var/log/clientroll-db-backup.log)
#   SCRIPT_PATH   (default: /var/www/clientroll-repo/scripts/db-backup.sh or local repo path)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_SCRIPT_PATH="$SCRIPT_DIR/db-backup.sh"

SCRIPT_PATH="${SCRIPT_PATH:-/var/www/clientroll-repo/scripts/db-backup.sh}"
if [[ ! -f "$SCRIPT_PATH" ]]; then
  SCRIPT_PATH="$LOCAL_SCRIPT_PATH"
fi

CRON_SCHEDULE="${CRON_SCHEDULE:-15 2 * * *}"
BACKUP_LOG="${BACKUP_LOG:-/var/log/clientroll-db-backup.log}"
MARKER="# clientroll-db-backup"
CRON_LINE="$CRON_SCHEDULE $SCRIPT_PATH >> $BACKUP_LOG 2>&1 $MARKER"

current_cron="$(crontab -l 2>/dev/null || true)"

# Remove any old backup line carrying marker, then append new one.
new_cron="$(printf "%s\n" "$current_cron" | awk -v m="$MARKER" 'index($0,m)==0')"
new_cron="${new_cron}
$CRON_LINE"

printf "%s\n" "$new_cron" | sed '/^[[:space:]]*$/N;/^\n$/D' | crontab -

echo "Installed cron job:"
echo "$CRON_LINE"
