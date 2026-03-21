#!/usr/bin/env bash
set -euo pipefail

APP_URL="${APP_URL:-https://app.clientroll.com}"
PLATFORM_TOKEN="${PLATFORM_TOKEN:-}"
TENANT_ID="${TENANT_ID:-}"

if [[ -z "$PLATFORM_TOKEN" ]]; then
  echo "PLATFORM_TOKEN is required"
  exit 1
fi

auth_hdr=(-H "Authorization: Bearer ${PLATFORM_TOKEN}" -H "Content-Type: application/json")

echo "1) Pulling ops health snapshot"
curl -sS "${auth_hdr[@]}" "${APP_URL}/api/platform/ops/health-snapshot" | python3 -m json.tool

echo "2) Running dry-run failed Stripe replay batch"
if [[ -n "$TENANT_ID" ]]; then
  replay_body="{\"tenant_id\":\"${TENANT_ID}\",\"dry_run\":true,\"limit\":25}"
else
  replay_body='{"dry_run":true,"limit":25}'
fi
curl -sS "${auth_hdr[@]}" -X POST "${APP_URL}/api/platform/webhooks/stripe/replay-failures" -d "${replay_body}" | python3 -m json.tool

echo "3) Running membership reconcile runbook"
if [[ -n "$TENANT_ID" ]]; then
  reconcile_body="{\"tenant_id\":\"${TENANT_ID}\"}"
else
  reconcile_body='{}'
fi
curl -sS "${auth_hdr[@]}" -X POST "${APP_URL}/api/platform/ops/reconcile-runbook" -d "${reconcile_body}" | python3 -m json.tool

echo "Runbook drill completed."
