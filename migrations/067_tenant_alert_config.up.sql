-- Tenant alert configuration for owner payment monitoring emails.
CREATE TABLE tenant_alert_config (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    alert_email VARCHAR(255) NOT NULL,
    immediate_payment_failed BOOLEAN NOT NULL DEFAULT TRUE,
    immediate_dispute BOOLEAN NOT NULL DEFAULT TRUE,
    immediate_mandate_cancelled BOOLEAN NOT NULL DEFAULT TRUE,
    immediate_pipeline_break BOOLEAN NOT NULL DEFAULT TRUE,
    daily_digest_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    daily_digest_hour INT NOT NULL DEFAULT 6,
    daily_digest_always BOOLEAN NOT NULL DEFAULT TRUE,
    grace_days_card INT NOT NULL DEFAULT 1,
    grace_days_direct_debit INT NOT NULL DEFAULT 3,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id)
);

CREATE INDEX idx_tenant_alert_config_tenant ON tenant_alert_config(tenant_id);

-- Reconciliation run history for audit trail.
CREATE TABLE reconciliation_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    run_type VARCHAR(30) NOT NULL DEFAULT 'daily',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    total_checked INT NOT NULL DEFAULT 0,
    total_matched INT NOT NULL DEFAULT 0,
    total_mismatched INT NOT NULL DEFAULT 0,
    total_auto_fixed INT NOT NULL DEFAULT 0,
    details JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reconciliation_runs_tenant ON reconciliation_runs(tenant_id);
CREATE INDEX idx_reconciliation_runs_created ON reconciliation_runs(tenant_id, created_at);
