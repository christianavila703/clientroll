-- Outbound webhook endpoints and delivery log

-- Webhook endpoints (tenant-configured)
CREATE TABLE webhook_endpoints (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    secret_encrypted TEXT NOT NULL,
    event_types TEXT[] NOT NULL DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_endpoints_tenant ON webhook_endpoints(tenant_id);
CREATE INDEX idx_webhook_endpoints_active ON webhook_endpoints(tenant_id, is_active);

-- Webhook delivery log
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    webhook_endpoint_id UUID NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    http_status INT,
    response_body TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    attempt_count INT NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_delivery_status CHECK (
        status IN ('pending', 'delivered', 'failed', 'retrying')
    )
);

CREATE INDEX idx_deliveries_tenant ON webhook_deliveries(tenant_id);
CREATE INDEX idx_deliveries_endpoint ON webhook_deliveries(webhook_endpoint_id);
CREATE INDEX idx_deliveries_status ON webhook_deliveries(status);
CREATE INDEX idx_deliveries_retry ON webhook_deliveries(next_retry_at) WHERE status = 'retrying';
