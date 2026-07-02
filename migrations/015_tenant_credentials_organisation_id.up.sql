-- Store GoCardless organisation_id per tenant for webhook routing
ALTER TABLE tenant_credentials ADD COLUMN IF NOT EXISTS provider_organisation_id VARCHAR(100);
CREATE INDEX IF NOT EXISTS idx_tenant_credentials_org ON tenant_credentials(provider_name, provider_organisation_id);

-- Lookup tenant by provider org id (used by webhooks; bypasses RLS)
CREATE OR REPLACE FUNCTION get_tenant_id_by_provider_organisation(p_provider text, p_org_id text)
RETURNS uuid LANGUAGE sql SECURITY DEFINER SET search_path = public
AS $$
  SELECT tenant_id FROM tenant_credentials
  WHERE provider_name = p_provider AND provider_organisation_id = p_org_id AND is_active = true
  LIMIT 1;
$$;
