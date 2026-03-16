#!/usr/bin/env bash
#
# Create a compressed PostgreSQL backup from DATABASE_URL.
# Intended for server use (cron/systemd) but can run manually.
#
# Optional env:
#   SOURCE_REPO   (default: /var/www/clientroll-repo or script parent)
#   DEPLOY_PATH   (default: /var/www/clientroll)
#   BACKUP_DIR    (default: $DEPLOY_PATH/backups/db)
#   RETAIN_DAYS   (default: 30)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_SOURCE_REPO="$(cd "$SCRIPT_DIR/.." && pwd)"

SOURCE_REPO="${SOURCE_REPO:-/var/www/clientroll-repo}"
if [[ ! -d "$SOURCE_REPO" ]]; then
  SOURCE_REPO="$DEFAULT_SOURCE_REPO"
fi

DEPLOY_PATH="${DEPLOY_PATH:-/var/www/clientroll}"
BACKUP_DIR="${BACKUP_DIR:-$DEPLOY_PATH/backups/db}"
RETAIN_DAYS="${RETAIN_DAYS:-30}"

if [[ -f "$DEPLOY_PATH/.env" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$DEPLOY_PATH/.env"
  set +a
elif [[ -f "$SOURCE_REPO/.env" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$SOURCE_REPO/.env"
  set +a
fi

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "ERROR: DATABASE_URL is not set (checked $DEPLOY_PATH/.env then $SOURCE_REPO/.env)."
  exit 1
fi

if ! command -v pg_dump >/dev/null 2>&1; then
  echo "ERROR: pg_dump is required but not installed."
  exit 1
fi

if ! command -v gzip >/dev/null 2>&1; then
  echo "ERROR: gzip is required but not installed."
  exit 1
fi

mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR" 2>/dev/null || true

ts="$(date -u +%Y%m%d-%H%M%S)"
outfile="$BACKUP_DIR/clientroll-db-$ts.sql.gz"
tmpfile="$outfile.tmp"

echo "Creating DB backup at $outfile"
pg_dump "$DATABASE_URL" | gzip -9 > "$tmpfile"
mv "$tmpfile" "$outfile"
chmod 600 "$outfile" 2>/dev/null || true

if command -v shasum >/dev/null 2>&1; then
  shasum -a 256 "$outfile" > "$outfile.sha256"
fi

# Prune old backups.
if [[ "$RETAIN_DAYS" =~ ^[0-9]+$ ]] && [[ "$RETAIN_DAYS" -gt 0 ]]; then
  find "$BACKUP_DIR" -type f \( -name "clientroll-db-*.sql.gz" -o -name "clientroll-db-*.sql.gz.sha256" \) -mtime +"$RETAIN_DAYS" -delete
fi

echo "Backup complete: $outfile"
