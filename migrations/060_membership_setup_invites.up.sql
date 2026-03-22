CREATE TABLE IF NOT EXISTS membership_setup_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    token UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    membership_type_id UUID NOT NULL REFERENCES membership_types(id) ON DELETE CASCADE,
    term_option_id UUID NOT NULL REFERENCES term_options(id) ON DELETE CASCADE,
    start_date DATE NOT NULL,
    payment_frequency VARCHAR(32) NOT NULL,
    payment_method VARCHAR(32),
    payment_method_strategy VARCHAR(32) NOT NULL,
    payment_method_id TEXT,
    gocardless_mandate_id TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_membership_setup_invites_tenant_client
    ON membership_setup_invites(tenant_id, client_id);

CREATE INDEX IF NOT EXISTS idx_membership_setup_invites_token
    ON membership_setup_invites(token);

CREATE INDEX IF NOT EXISTS idx_membership_setup_invites_expires
    ON membership_setup_invites(expires_at);

