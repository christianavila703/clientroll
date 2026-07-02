-- Add GoCardless subscription ID to memberships
ALTER TABLE memberships ADD COLUMN IF NOT EXISTS gocardless_subscription_id VARCHAR(255);

-- Bank details table (stores last 4 digits only, never full account number)
CREATE TABLE IF NOT EXISTS bank_details (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    client_id UUID NOT NULL REFERENCES clients(id),
    account_holder_name VARCHAR(255),
    bsb VARCHAR(20),
    account_number_last4 VARCHAR(4),
    institution_name VARCHAR(255),
    gocardless_bank_account_id VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bank_details_client ON bank_details(client_id, tenant_id);

-- Redirect flow sessions table (maps session_token -> client + tenant for completion)
CREATE TABLE IF NOT EXISTS redirect_flow_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    client_id UUID NOT NULL REFERENCES clients(id),
    session_token VARCHAR(255) NOT NULL UNIQUE,
    flow_id VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_redirect_flow_sessions_token ON redirect_flow_sessions(session_token);
