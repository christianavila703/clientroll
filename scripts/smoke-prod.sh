#!/usr/bin/env bash
set -euo pipefail

APP_URL="${APP_URL:-https://app.example.com}"
DEPLOY_HOST="${DEPLOY_HOST:-}"
TENANT_SLUG="${TENANT_SLUG:-}"

pass() { echo "PASS: $*"; }
fail() { echo "FAIL: $*" >&2; exit 1; }

echo "Running production smoke checks against ${APP_URL}"

health_code="$(curl -sS -o /tmp/clientroll_health.json -w "%{http_code}" "${APP_URL}/api/health")"
[[ "$health_code" == "200" ]] || fail "/api/health returned ${health_code}"
grep -q '"status":"ok"' /tmp/clientroll_health.json || fail "/api/health did not report status ok"
pass "health endpoint"

if [[ -n "$DEPLOY_HOST" && -n "$TENANT_SLUG" ]]; then
  echo "Running signed-link smoke check for tenant ${TENANT_SLUG}"
  signed_out="$(
    ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new "$DEPLOY_HOST" "set -euo pipefail; . /var/www/clientroll/.env; exp=\$(( \$(date +%s) + 3600 )); payload=\"${TENANT_SLUG,,}||\$exp\"; sig=\$(printf '%s' \"\$payload\" | openssl dgst -sha256 -hmac \"\$SIGNUP_LINK_SIGNING_SECRET\" -binary | xxd -p -c 256); echo \"\$exp \$sig\""
  )"
  exp="$(printf '%s' "$signed_out" | awk '{print $1}')"
  sig="$(printf '%s' "$signed_out" | awk '{print $2}')"
  [[ -n "$exp" && -n "$sig" ]] || fail "failed to generate signed query for signup check"

  unsigned_code="$(curl -sS -o /dev/null -w "%{http_code}" "${APP_URL}/api/signup/${TENANT_SLUG}")"
  signed_code="$(curl -sS -o /dev/null -w "%{http_code}" "${APP_URL}/api/signup/${TENANT_SLUG}?exp=${exp}&sig=${sig}")"

  [[ "$unsigned_code" == "401" ]] || fail "unsigned signup expected 401, got ${unsigned_code}"
  [[ "$signed_code" == "200" ]] || fail "signed signup expected 200, got ${signed_code}"
  pass "signed signup enforcement"
else
  echo "INFO: skipping signed-link smoke check (set DEPLOY_HOST and TENANT_SLUG to enable)"
fi

echo "Smoke checks complete."
