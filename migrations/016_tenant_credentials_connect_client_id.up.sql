-- Store per-tenant GoCardless Connect app client_id (no env).
ALTER TABLE tenant_credentials ADD COLUMN IF NOT EXISTS connect_client_id VARCHAR(255);
