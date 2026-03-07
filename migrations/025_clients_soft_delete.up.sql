-- Soft delete for clients (retain history, allow re-adding)

ALTER TABLE clients
    ADD COLUMN deleted_at TIMESTAMPTZ;

CREATE INDEX idx_clients_tenant_deleted_at ON clients(tenant_id, deleted_at);

