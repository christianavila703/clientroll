DROP POLICY IF EXISTS tenant_isolation ON membership_setup_invites;
ALTER TABLE membership_setup_invites DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON membership_accepted_terms;
ALTER TABLE membership_accepted_terms DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON membership_agreement_snapshots;
ALTER TABLE membership_agreement_snapshots DISABLE ROW LEVEL SECURITY;
