DROP POLICY IF EXISTS tenant_isolation ON xero_connections;
ALTER TABLE xero_connections DISABLE ROW LEVEL SECURITY;

DROP TABLE IF EXISTS xero_connections;
