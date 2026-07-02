CREATE TABLE IF NOT EXISTS xero_sync_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    entity_type TEXT NOT NULL CHECK (entity_type IN ('contact', 'membership_payment', 'service_charge', 'credit_note')),
    entity_id UUID NOT NULL,
    xero_object_id TEXT NOT NULL,
    xero_object_ref TEXT,
    sync_state TEXT NOT NULL DEFAULT 'synced' CHECK (sync_state IN ('synced', 'invoice_created', 'payment_applied', 'credit_note_applied', 'failed')),
    paid_date_at_sync TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_xero_sync_entity
    ON xero_sync_mappings(tenant_id, entity_type, entity_id);

CREATE INDEX IF NOT EXISTS idx_xero_sync_object
    ON xero_sync_mappings(tenant_id, xero_object_id);

ALTER TABLE xero_sync_mappings ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON xero_sync_mappings
    USING (tenant_id = current_tenant_id());

CREATE TABLE IF NOT EXISTS xero_sync_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    entity_type TEXT NOT NULL,
    entity_id UUID NOT NULL,
    action TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('success', 'failed')),
    error_code TEXT,
    error_body TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_xero_sync_events_tenant_status
    ON xero_sync_events(tenant_id, status, created_at DESC);

ALTER TABLE xero_sync_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON xero_sync_events
    USING (tenant_id = current_tenant_id());
