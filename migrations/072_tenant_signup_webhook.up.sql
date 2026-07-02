ALTER TABLE tenants
ADD COLUMN IF NOT EXISTS signup_webhook_url TEXT,
ADD COLUMN IF NOT EXISTS signup_webhook_secret_encrypted TEXT,
ADD COLUMN IF NOT EXISTS signup_webhook_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE memberships
ADD COLUMN IF NOT EXISTS signup_webhook_sent_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS signup_webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    membership_id UUID NOT NULL REFERENCES memberships(id) ON DELETE CASCADE,
    payload JSONB NOT NULL,
    http_status INT,
    response_body TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    attempt_count INT NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_signup_webhook_delivery_status CHECK (
        status IN ('pending', 'delivered', 'failed', 'retrying')
    )
);

CREATE INDEX IF NOT EXISTS idx_signup_webhook_deliveries_tenant
    ON signup_webhook_deliveries(tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_signup_webhook_deliveries_membership
    ON signup_webhook_deliveries(membership_id);
