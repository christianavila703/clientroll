-- Platform-level tables (not tenant-scoped)

-- Tenants
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    timezone VARCHAR(100) NOT NULL DEFAULT 'Australia/Brisbane',
    is_active BOOLEAN NOT NULL DEFAULT true,
    logo_url TEXT,
    primary_colour VARCHAR(7) DEFAULT '#6366f1',
    default_thankyou_heading TEXT DEFAULT 'Thank You!',
    default_thankyou_body TEXT DEFAULT 'Your membership has been successfully created.',
    default_thankyou_image_url TEXT,
    default_thankyou_redirect_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tenants_slug ON tenants(slug);
CREATE INDEX idx_tenants_is_active ON tenants(is_active);

-- Platform admins (above tenants)
CREATE TABLE platform_admins (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Platform billing configuration (Stripe)
CREATE TABLE platform_billing_config (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    per_member_price_usd DECIMAL(10, 2) NOT NULL DEFAULT 2.00,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    stripe_publishable_key_encrypted TEXT,
    stripe_secret_key_encrypted TEXT,
    stripe_webhook_secret_encrypted TEXT,
    is_active BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
