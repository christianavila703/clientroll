-- Membership types, terms, bonuses, and T&C tables

-- Membership types
CREATE TABLE membership_types (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    description TEXT,
    inclusions TEXT,
    top_image_url TEXT,
    is_active BOOLEAN NOT NULL DEFAULT false,
    total_value DECIMAL(10, 2) NOT NULL DEFAULT 0,
    sort_order INT NOT NULL DEFAULT 0,
    use_custom_thankyou BOOLEAN NOT NULL DEFAULT false,
    thankyou_heading TEXT,
    thankyou_body TEXT,
    thankyou_image_url TEXT,
    thankyou_redirect_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_membership_types_tenant ON membership_types(tenant_id);
CREATE UNIQUE INDEX idx_membership_types_slug ON membership_types(tenant_id, slug);
CREATE INDEX idx_membership_types_active ON membership_types(tenant_id, is_active);

-- Membership term options (e.g. 6 months, 12 months)
CREATE TABLE membership_term_options (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    membership_type_id UUID NOT NULL REFERENCES membership_types(id) ON DELETE CASCADE,
    duration_months INT NOT NULL,
    fee_total DECIMAL(10, 2) NOT NULL DEFAULT 0,
    fee_weekly DECIMAL(10, 2),
    fee_fortnightly DECIMAL(10, 2),
    fee_monthly DECIMAL(10, 2),
    allow_upfront BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_term_options_tenant ON membership_term_options(tenant_id);
CREATE INDEX idx_term_options_type ON membership_term_options(membership_type_id);

-- Terms and conditions (versioned)
CREATE TABLE terms_and_conditions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_terms_tenant ON terms_and_conditions(tenant_id);
CREATE INDEX idx_terms_active ON terms_and_conditions(tenant_id, is_active);

-- Junction: membership types <-> terms and conditions
CREATE TABLE membership_type_terms (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    membership_type_id UUID NOT NULL REFERENCES membership_types(id) ON DELETE CASCADE,
    terms_and_conditions_id UUID NOT NULL REFERENCES terms_and_conditions(id) ON DELETE CASCADE,
    PRIMARY KEY (membership_type_id, terms_and_conditions_id)
);

CREATE INDEX idx_type_terms_tenant ON membership_type_terms(tenant_id);

-- Bonuses (standalone, tenant-level)
CREATE TABLE bonuses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    image_url TEXT,
    value DECIMAL(10, 2) NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bonuses_tenant ON bonuses(tenant_id);
CREATE INDEX idx_bonuses_active ON bonuses(tenant_id, is_active);

-- Junction: term options <-> bonuses (many-to-many)
CREATE TABLE term_option_bonuses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    term_option_id UUID NOT NULL REFERENCES membership_term_options(id) ON DELETE CASCADE,
    bonus_id UUID NOT NULL REFERENCES bonuses(id) ON DELETE CASCADE,
    UNIQUE (term_option_id, bonus_id)
);

CREATE INDEX idx_term_bonuses_tenant ON term_option_bonuses(tenant_id);
CREATE INDEX idx_term_bonuses_option ON term_option_bonuses(term_option_id);
CREATE INDEX idx_term_bonuses_bonus ON term_option_bonuses(bonus_id);
