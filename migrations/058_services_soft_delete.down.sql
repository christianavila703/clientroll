DROP INDEX IF EXISTS idx_services_tenant_deleted_at;

ALTER TABLE services
DROP COLUMN IF EXISTS deleted_at;
