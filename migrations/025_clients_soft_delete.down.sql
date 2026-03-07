DROP INDEX IF EXISTS idx_clients_tenant_deleted_at;

ALTER TABLE clients
    DROP COLUMN IF EXISTS deleted_at;

