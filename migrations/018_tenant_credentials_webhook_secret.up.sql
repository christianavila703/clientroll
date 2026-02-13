-- Per-tenant GoCardless webhook secret (optional; fall back to global if not set)
ALTER TABLE tenant_credentials ADD COLUMN IF NOT EXISTS webhook_secret_encrypted TEXT;
