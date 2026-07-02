-- Migration 076: membership_reacceptance_required
--
-- Tracks which active memberships still need to re-accept a T&C document
-- because the admin flagged the latest update as a "major" revision. When
-- the admin saves a T&C with requires_reacceptance=true, one row is inserted
-- per membership that previously accepted the prior version of that document;
-- the member then sees a re-acceptance prompt on their next sign-in / next
-- payment attempt until they confirm.

CREATE TABLE membership_reacceptance_required (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    membership_id UUID NOT NULL REFERENCES memberships(id) ON DELETE CASCADE,
    terms_id UUID NOT NULL REFERENCES terms_and_conditions(id) ON DELETE CASCADE,
    -- Live version that needs accepting. When the member re-accepts, this
    -- row is removed (or acknowledged_at is set, depending on retention).
    required_version INT NOT NULL,
    triggered_by_user_id UUID,
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ,
    PRIMARY KEY (membership_id, terms_id)
);

CREATE INDEX idx_reacceptance_tenant ON membership_reacceptance_required(tenant_id);
CREATE INDEX idx_reacceptance_pending
    ON membership_reacceptance_required(tenant_id, acknowledged_at)
    WHERE acknowledged_at IS NULL;

ALTER TABLE membership_reacceptance_required ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'public' AND tablename = 'membership_reacceptance_required'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON membership_reacceptance_required
            USING (tenant_id::text = current_setting('app.current_tenant_id', true));
    END IF;
END $$;
