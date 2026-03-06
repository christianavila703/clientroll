# GoCardless integration – deploy checklist

## 1. GoCardless dashboard (one-time)

- Create a **Partner app** in [GoCardless Developer](https://developer.gocardless.com/) (sandbox and/or live).
- Set **Redirect URL**: `https://app.clientroll.com/api/gocardless/oauth/callback`
- Set **Webhook URL**: `https://app.clientroll.com/api/webhooks/gocardless`
- Note **Client ID**, **Client Secret**, and **Webhook secret** for each environment.

## 2. Server environment

Add to your app’s env (e.g. `/var/www/clientroll/.env` or systemd `EnvironmentFile`):

```bash
# GoCardless Connect (optional; omit to disable)
GOCARDLESS_SANDBOX_CLIENT_ID=...
GOCARDLESS_SANDBOX_CLIENT_SECRET=...
GOCARDLESS_LIVE_CLIENT_ID=...
GOCARDLESS_LIVE_CLIENT_SECRET=...
GOCARDLESS_WEBHOOK_SECRET=...

# Must be the app origin (used for OAuth redirect)
APP_URL=https://app.clientroll.com
```

## 3. Database migration

On the server (or wherever your DB runs), from the repo root:

```bash
export DATABASE_URL="postgres://..."
go run ./cmd/migrate up
```

This applies `015_tenant_credentials_organisation_id.up.sql` (adds `provider_organisation_id` and the webhook lookup function).

## 4. Deploy backend and frontend

- Replace the running API binary with the new build (e.g. `server_linux_amd64` from `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server_linux_amd64 ./cmd/server/`).
- Restart the API service (e.g. `systemctl restart clientroll`).
- Deploy the new frontend build (e.g. copy `frontend/dist/*` to the static root and ensure `index.html` is no-cache if needed).

## 5. Verify

- Log in as a tenant admin → **Settings** → **Integrations**.
- Click **Set up GoCardless** and complete the wizard (intro → environment → connect with GoCardless).
- After redirect back, the card should show **Connected** and **Disconnect**.
