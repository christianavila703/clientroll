-- Enable RLS on tenant-scoped tables added after migration 009.

ALTER TABLE membership_agreement_snapshots ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON membership_agreement_snapshots
    USING (tenant_id = current_tenant_id());

ALTER TABLE membership_accepted_terms ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON membership_accepted_terms
    USING (tenant_id = current_tenant_id());

ALTER TABLE membership_setup_invites ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON membership_setup_invites
    USING (tenant_id = current_tenant_id());
