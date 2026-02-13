DROP FUNCTION IF EXISTS get_tenant_id_by_provider_organisation(text, text);
DROP INDEX IF EXISTS idx_tenant_credentials_org;
ALTER TABLE tenant_credentials DROP COLUMN IF EXISTS provider_organisation_id;
