-- Inbound webhook event dedupe and payment provider uniqueness

CREATE TABLE inbound_webhook_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    event_id VARCHAR(255) NOT NULL,
    resource_type VARCHAR(100),
    resource_id VARCHAR(255),
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX uq_inbound_webhook_events_provider_event
    ON inbound_webhook_events(tenant_id, provider, event_id);

CREATE INDEX idx_inbound_webhook_events_resource
    ON inbound_webhook_events(tenant_id, provider, resource_type, resource_id);

CREATE UNIQUE INDEX uq_payments_provider_reference
    ON payments(tenant_id, provider_reference)
    WHERE provider_reference IS NOT NULL;
