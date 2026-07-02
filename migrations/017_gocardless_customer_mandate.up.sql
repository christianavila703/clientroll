-- Store GoCardless customer and mandate IDs for payment flows
ALTER TABLE clients ADD COLUMN IF NOT EXISTS gocardless_customer_id VARCHAR(255);
ALTER TABLE memberships ADD COLUMN IF NOT EXISTS gocardless_mandate_id VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_clients_gocardless_customer ON clients(tenant_id) WHERE gocardless_customer_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_memberships_gocardless_mandate ON memberships(tenant_id) WHERE gocardless_mandate_id IS NOT NULL;
