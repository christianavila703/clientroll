-- Migration 075: terms_and_conditions_versions
--
-- Captures an immutable audit trail of every T&C document revision. The live
-- row in terms_and_conditions always holds the latest version; each save
-- snapshots the previous (or, for the first version, the new) content here so
-- that:
--   * Admins can review what changed and when.
--   * Future re-acceptance prompts know what the member previously agreed to.
--   * Compliance can prove which exact text was in force on a given date.
--
-- Backfill: every existing terms_and_conditions row is seeded with one
-- history entry whose effective_from matches its created_at.

CREATE TABLE terms_and_conditions_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    terms_id UUID NOT NULL REFERENCES terms_and_conditions(id) ON DELETE CASCADE,
    version INT NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    is_active BOOLEAN NOT NULL,
    -- Free-form change note the admin can supply when saving. Optional.
    change_note TEXT,
    -- When this version became the live row. Mirrors terms_and_conditions.updated_at
    -- at the time the version was captured.
    effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (terms_id, version)
);

CREATE INDEX idx_terms_versions_tenant ON terms_and_conditions_versions(tenant_id);
CREATE INDEX idx_terms_versions_terms ON terms_and_conditions_versions(terms_id, version DESC);

ALTER TABLE terms_and_conditions_versions ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE schemaname = 'public' AND tablename = 'terms_and_conditions_versions'
          AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON terms_and_conditions_versions
            USING (tenant_id::text = current_setting('app.current_tenant_id', true));
    END IF;
END $$;

-- Backfill: snapshot every current document as its initial version.
INSERT INTO terms_and_conditions_versions (
    tenant_id, terms_id, version, title, content, is_active, effective_from, created_at
)
SELECT
    tenant_id,
    id,
    version,
    title,
    content,
    is_active,
    created_at,
    created_at
FROM terms_and_conditions
ON CONFLICT (terms_id, version) DO NOTHING;
