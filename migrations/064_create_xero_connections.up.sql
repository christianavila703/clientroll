CREATE TABLE IF NOT EXISTS xero_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL UNIQUE REFERENCES tenants(id),
    access_token_encrypted TEXT NOT NULL,
    refresh_token_encrypted TEXT NOT NULL,
    token_expiry TIMESTAMPTZ NOT NULL,
    xero_tenant_id TEXT NOT NULL,
    xero_tenant_name TEXT,
    default_revenue_account_code TEXT NOT NULL DEFAULT '200',
    default_payment_account_code TEXT NOT NULL DEFAULT '090',
    is_active BOOLEAN NOT NULL DEFAULT true,
    needs_reauth BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_xero_connections_tenant
    ON xero_connections(tenant_id);

ALTER TABLE xero_connections ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON xero_connections
    USING (tenant_id = current_tenant_id());
