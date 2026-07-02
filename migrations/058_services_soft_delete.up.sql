ALTER TABLE services
ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_services_tenant_deleted_at ON services(tenant_id, deleted_at);
