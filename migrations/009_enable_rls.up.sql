-- Enable Row-Level Security on all tenant-scoped tables.
-- This is the database-level safety net: even if application code
-- has a bug and omits WHERE tenant_id = $1, PostgreSQL itself
-- blocks cross-tenant reads/writes.

-- The app sets: SET LOCAL app.current_tenant_id = '<uuid>' per request.

-- Helper function to get current tenant from session
CREATE OR REPLACE FUNCTION current_tenant_id() RETURNS UUID AS $$
BEGIN
    RETURN NULLIF(current_setting('app.current_tenant_id', true), '')::UUID;
EXCEPTION
    WHEN OTHERS THEN RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;

-- Enable RLS and create policies for each tenant-scoped table
-- Pattern: enable RLS, create policy that filters by tenant_id

-- tenant_credentials
ALTER TABLE tenant_credentials ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenant_credentials
    USING (tenant_id = current_tenant_id());

-- users
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON users
    USING (tenant_id = current_tenant_id());

-- agreement_settings
ALTER TABLE agreement_settings ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON agreement_settings
    USING (tenant_id = current_tenant_id());

-- tenant_subscriptions
ALTER TABLE tenant_subscriptions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenant_subscriptions
    USING (tenant_id = current_tenant_id());

-- tenant_usage_snapshots
ALTER TABLE tenant_usage_snapshots ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenant_usage_snapshots
    USING (tenant_id = current_tenant_id());

-- tenant_invoices
ALTER TABLE tenant_invoices ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenant_invoices
    USING (tenant_id = current_tenant_id());

-- membership_types
ALTER TABLE membership_types ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON membership_types
    USING (tenant_id = current_tenant_id());

-- membership_term_options
ALTER TABLE membership_term_options ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON membership_term_options
    USING (tenant_id = current_tenant_id());

-- terms_and_conditions
ALTER TABLE terms_and_conditions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON terms_and_conditions
    USING (tenant_id = current_tenant_id());

-- membership_type_terms
ALTER TABLE membership_type_terms ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON membership_type_terms
    USING (tenant_id = current_tenant_id());

-- bonuses
ALTER TABLE bonuses ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON bonuses
    USING (tenant_id = current_tenant_id());

-- term_option_bonuses
ALTER TABLE term_option_bonuses ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON term_option_bonuses
    USING (tenant_id = current_tenant_id());

-- clients
ALTER TABLE clients ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON clients
    USING (tenant_id = current_tenant_id());

-- memberships
ALTER TABLE memberships ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON memberships
    USING (tenant_id = current_tenant_id());

-- membership_addons
ALTER TABLE membership_addons ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON membership_addons
    USING (tenant_id = current_tenant_id());

-- bank_details
ALTER TABLE bank_details ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON bank_details
    USING (tenant_id = current_tenant_id());

-- payments
ALTER TABLE payments ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON payments
    USING (tenant_id = current_tenant_id());

-- payment_disputes
ALTER TABLE payment_disputes ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON payment_disputes
    USING (tenant_id = current_tenant_id());

-- payment_failure_log
ALTER TABLE payment_failure_log ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON payment_failure_log
    USING (tenant_id = current_tenant_id());

-- webhook_endpoints
ALTER TABLE webhook_endpoints ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON webhook_endpoints
    USING (tenant_id = current_tenant_id());

-- webhook_deliveries
ALTER TABLE webhook_deliveries ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON webhook_deliveries
    USING (tenant_id = current_tenant_id());

-- staff_invitations
ALTER TABLE staff_invitations ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON staff_invitations
    USING (tenant_id = current_tenant_id());

-- password_reset_tokens
ALTER TABLE password_reset_tokens ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON password_reset_tokens
    USING (tenant_id = current_tenant_id());

-- uploaded_files
ALTER TABLE uploaded_files ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON uploaded_files
    USING (tenant_id = current_tenant_id());

-- email_log
ALTER TABLE email_log ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON email_log
    USING (tenant_id = current_tenant_id());

-- audit_log
ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON audit_log
    USING (tenant_id = current_tenant_id());
