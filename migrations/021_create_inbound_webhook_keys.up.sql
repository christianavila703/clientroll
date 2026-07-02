-- Per-tenant inbound webhook keys used to authenticate partner webhooks.
-- Keys are looked up by key_id on public routes; secrets are stored encrypted.

CREATE TABLE inbound_webhook_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    key_id VARCHAR(80) NOT NULL UNIQUE,
    secret_encrypted TEXT NOT NULL,
    label VARCHAR(255),
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_inbound_webhook_keys_tenant
    ON inbound_webhook_keys(tenant_id);

CREATE INDEX idx_inbound_webhook_keys_tenant_active
    ON inbound_webhook_keys(tenant_id, is_active);
