-- Tenant-scoped core tables

-- Tenant credentials (GoCardless, MailerSend API keys etc.)
CREATE TABLE tenant_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    provider_name VARCHAR(50) NOT NULL,
    api_key_encrypted TEXT,
    api_secret_encrypted TEXT,
    environment VARCHAR(20) DEFAULT 'sandbox',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tenant_credentials_tenant ON tenant_credentials(tenant_id);
CREATE UNIQUE INDEX idx_tenant_credentials_provider ON tenant_credentials(tenant_id, provider_name);

-- Users (tenant staff/admins)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'tenant_staff',
    is_active BOOLEAN NOT NULL DEFAULT true,
    failed_login_attempts INT NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_user_role CHECK (role IN ('tenant_admin', 'tenant_staff'))
);

CREATE INDEX idx_users_tenant ON users(tenant_id);
CREATE UNIQUE INDEX idx_users_email_tenant ON users(tenant_id, email);
CREATE INDEX idx_users_email ON users(email);

-- Agreement settings (Section A - persistent company details per tenant)
CREATE TABLE agreement_settings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE UNIQUE,
    company_name VARCHAR(255),
    business_name VARCHAR(255),
    abn_acn VARCHAR(50),
    trading_name VARCHAR(255),
    address TEXT,
    state VARCHAR(50),
    postcode VARCHAR(10),
    email VARCHAR(255),
    phone VARCHAR(50),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_agreement_settings_tenant ON agreement_settings(tenant_id);

-- Tenant subscriptions (platform billing via Stripe)
CREATE TABLE tenant_subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE UNIQUE,
    stripe_customer_id VARCHAR(255),
    stripe_subscription_id VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    active_member_count INT NOT NULL DEFAULT 0,
    current_monthly_amount_usd DECIMAL(10, 2) NOT NULL DEFAULT 0,
    current_period_start DATE,
    current_period_end DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_tenant_subscriptions_tenant ON tenant_subscriptions(tenant_id);

-- Tenant usage snapshots (daily active member counts for billing)
CREATE TABLE tenant_usage_snapshots (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    active_member_count INT NOT NULL DEFAULT 0,
    amount_usd DECIMAL(10, 2) NOT NULL DEFAULT 0,
    snapshot_date DATE NOT NULL,
    invoiced BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tenant_usage_tenant ON tenant_usage_snapshots(tenant_id);
CREATE UNIQUE INDEX idx_tenant_usage_date ON tenant_usage_snapshots(tenant_id, snapshot_date);

-- Tenant invoices (from Stripe)
CREATE TABLE tenant_invoices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    stripe_invoice_id VARCHAR(255),
    amount_usd DECIMAL(10, 2) NOT NULL,
    member_count INT NOT NULL DEFAULT 0,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'draft',
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tenant_invoices_tenant ON tenant_invoices(tenant_id);
