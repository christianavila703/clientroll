DROP POLICY IF EXISTS tenant_isolation ON xero_sync_events;
ALTER TABLE xero_sync_events DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS xero_sync_events;

DROP POLICY IF EXISTS tenant_isolation ON xero_sync_mappings;
ALTER TABLE xero_sync_mappings DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS xero_sync_mappings;
