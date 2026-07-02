-- Migration 077: terms_reacceptance_invites
--
-- Tokenised, single-use links a member follows to re-accept an updated T&C.
-- One row is issued per (membership_id, terms_id) when the admin flags a
-- T&C as requiring re-acceptance. The token is uniquely indexed so a staff
-- member can email/share the link without further auth — the URL itself
-- proves the recipient is the right member.
--
-- Re-flagging the same T&C rotates the token and clears completed_at, so
-- the member is forced to sign the latest version (not whatever they may
-- have accepted previously).

CREATE TABLE terms_reacceptance_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    token UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    membership_id UUID NOT NULL REFERENCES memberships(id) ON DELETE CASCADE,
    terms_id UUID NOT NULL REFERENCES terms_and_conditions(id) ON DELETE CASCADE,
    terms_version INT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    signature_typed TEXT,
    signature_image_data TEXT,
    signature_ip TEXT,
    signature_user_agent TEXT,
    triggered_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (membership_id, terms_id)
);

CREATE INDEX idx_reaccept_invites_tenant ON terms_reacceptance_invites(tenant_id);
CREATE INDEX idx_reaccept_invites_token ON terms_reacceptance_invites(token);
CREATE INDEX idx_reaccept_invites_pending
    ON terms_reacceptance_invites(tenant_id, expires_at)
    WHERE completed_at IS NULL;

ALTER TABLE terms_reacceptance_invites ENABLE ROW LEVEL SECURITY;

-- Tenant isolation policy (matches existing pattern). Public-facing endpoints
-- bypass RLS by resolving the tenant from the URL slug + token first, then
-- setting app.current_tenant_id before touching this table.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'public' AND tablename = 'terms_reacceptance_invites'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON terms_reacceptance_invites
            USING (tenant_id::text = current_setting('app.current_tenant_id', true));
    END IF;
END $$;
