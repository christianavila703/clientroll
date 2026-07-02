-- Email verification tracking (new table per schema change policy)
CREATE TABLE user_email_verifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    token_hash VARCHAR(64) NOT NULL,
    verified BOOLEAN NOT NULL DEFAULT false,
    verified_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_email_verif_tenant ON user_email_verifications(tenant_id);
CREATE INDEX idx_email_verif_user ON user_email_verifications(user_id);
CREATE INDEX idx_email_verif_token ON user_email_verifications(token_hash);

-- RLS
ALTER TABLE user_email_verifications ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON user_email_verifications
    USING (tenant_id = current_tenant_id());
