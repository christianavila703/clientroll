-- Per-tenant operational email template settings (enabled + optional overrides).
CREATE TABLE IF NOT EXISTS tenant_email_templates (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    template_key TEXT NOT NULL,
    category TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    subject_override TEXT,
    body_override TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, template_key),
    CONSTRAINT chk_tenant_email_templates_category
        CHECK (category IN ('client', 'payments', 'internal', 'reminders'))
);

CREATE INDEX IF NOT EXISTS idx_tenant_email_templates_tenant_category
    ON tenant_email_templates (tenant_id, category);
