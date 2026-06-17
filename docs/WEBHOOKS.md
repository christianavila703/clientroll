# Webhooks

This project supports both inbound and outbound webhooks on a per-tenant basis.

## Tenant Key Management (Inbound)

Authenticated tenant admins can manage inbound webhook keys:

- `GET /api/tenant/webhooks/inbound/keys`
- `POST /api/tenant/webhooks/inbound/keys`
- `POST /api/tenant/webhooks/inbound/keys/{keyID}/rotate`
- `DELETE /api/tenant/webhooks/inbound/keys/{keyID}`

`POST` and `rotate` return `signing_secret` only once. Store it securely.

## Inbound Webhook Endpoint (Create Client)

- `POST /api/webhooks/inbound/clients`

Headers:

- `X-Webhook-Key-Id`: tenant inbound key id (for example `whk_...`)
- `X-Webhook-Timestamp`: unix epoch seconds
- `X-Webhook-Signature`: hex HMAC-SHA256 of `<timestamp>.<raw_request_body>` using the key secret

Payload:

```json
{
  "event_id": "evt_external_123",
  "event_type": "client.created",
  "data": {
    "name": "Jane Citizen",
    "email": "jane@example.com",
    "phone": "0400000000",
    "birth_date": "1992-04-15"
  }
}
```

Notes:

- `event_id` is required and used for idempotency dedupe.
- Timestamp skew beyond 5 minutes is rejected.
- Body size limit is 1 MiB.

## Signup completion webhook (Settings)

Tenant admins configure a dedicated signup webhook under **Settings → Organisation → Signup webhook**:

- `GET /api/tenant/settings/tenant` — includes `signup_webhook_url`, `signup_webhook_enabled`, `signup_webhook_has_secret`
- `PUT /api/tenant/settings/tenant` — set `signup_webhook_url`, `signup_webhook_enabled`; optional `regenerate_signup_webhook_secret: true`
- `POST /api/tenant/settings/signup-webhook/test` — sends a sample `signup.completed` payload synchronously

When enabled, ClientRoll POSTs once per completed membership signup (after payment succeeds for checkout flows). The body uses flat Ignition-style keys, for example:

- `Membership Name`, `Contact Name`, `Contact Email`, `Start Date`, `End Date`
- `Client Name`, `Client Company Address Lines`, `Client Company Address City`, …
- `Agreement PDF URL`, `Membership Value`, `signatories`

Request headers match outbound webhooks (`X-Webhook-Signature`, `X-Webhook-Event`, `X-Webhook-Event-Id`, `X-Webhook-Timestamp`).

Saving a new URL or enabling the webhook generates a signing secret returned once as `signup_webhook_signing_secret`.

## Outbound Webhook Endpoints

Tenants can already manage outbound endpoints at:

- `GET /api/tenant/webhooks`
- `POST /api/tenant/webhooks`
- `PUT /api/tenant/webhooks/{endpointID}/toggle`
- `DELETE /api/tenant/webhooks/{endpointID}`
- `GET /api/tenant/webhooks/{endpointID}/deliveries`
- `GET /api/tenant/webhooks/event-types`

Outbound events emitted:

- `client.created`
- `membership.created`
- `membership.cancelled`
- `membership.expired`
- `payment.succeeded`
- `payment.failed`

Outbound request headers:

- `X-Webhook-Signature`: hex HMAC-SHA256 of `<timestamp>.<raw_body>`
- `X-Webhook-Event`: event type
- `X-Webhook-Event-Id`: event id
- `X-Webhook-Timestamp`: RFC3339 timestamp

Retry behavior:

- Failed deliveries are retried with exponential backoff.
- Due retry jobs are re-queued automatically by the webhook worker.

## Internal Webhook Ops (Platform Admin Only)

These endpoints are internal reliability tooling and are not tenant-facing:

- `GET /api/platform/webhooks/stripe/failures`
- `POST /api/platform/webhooks/stripe/replay`
- `POST /api/platform/webhooks/stripe/replay-failures`
- `GET /api/platform/webhooks/gocardless/failures`
- `POST /api/platform/webhooks/gocardless/reconcile-payment`
- `GET /api/platform/webhooks/inbound/failures`
- `GET /api/platform/ops/membership-status-drift`
- `POST /api/platform/ops/membership-status-reconcile`
- `GET /api/platform/ops/health-snapshot`
- `POST /api/platform/ops/reconcile-runbook`

Notes:

- Use these for operational monitoring/recovery of failed inbound processing.
- Tenant users should only see friendly payment state messages, never raw webhook internals.
