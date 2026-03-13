-- Stores a frozen snapshot of the full agreement at the moment of signing.
-- This is the legal record — immutable once created.
CREATE TABLE IF NOT EXISTS membership_agreement_snapshots (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    membership_id UUID NOT NULL REFERENCES memberships(id) ON DELETE CASCADE,
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,

    -- Frozen client details at time of signing
    client_name TEXT NOT NULL,
    client_email TEXT,
    client_phone TEXT,
    client_address TEXT,
    client_date_of_birth DATE,
    client_gender TEXT,
    client_emergency_contact_name TEXT,
    client_emergency_contact_phone TEXT,

    -- Frozen membership details
    membership_type_title TEXT NOT NULL,
    membership_name TEXT,
    membership_level TEXT,
    membership_description TEXT,
    membership_inclusions TEXT,
    membership_benefits TEXT,
    term_months INTEGER NOT NULL,
    term_label TEXT,
    total_fee NUMERIC(10,2) NOT NULL,
    payment_method TEXT,
    payment_frequency TEXT,
    payment_amount NUMERIC(10,2),
    payment_interval_unit TEXT,
    payment_interval_count INTEGER DEFAULT 1,
    num_payments INTEGER,
    start_date DATE NOT NULL,
    end_date DATE,

    -- Fee breakdown (frozen from term option)
    fee_weekly NUMERIC(10,2),
    fee_fortnightly NUMERIC(10,2),
    fee_monthly NUMERIC(10,2),
    fee_quarterly NUMERIC(10,2),
    allow_upfront BOOLEAN DEFAULT FALSE,

    -- Custom schedule (if applicable)
    custom_schedule_label TEXT,
    custom_schedule_amount NUMERIC(10,2),
    custom_schedule_interval_unit TEXT,
    custom_schedule_interval_count INTEGER,
    custom_schedule_num_payments INTEGER,

    -- Frozen organisation details
    org_company_name TEXT,
    org_trading_name TEXT,
    org_abn TEXT,
    org_acn TEXT,
    org_address TEXT,
    org_email TEXT,
    org_phone TEXT,

    -- Signatures (base64 image data)
    member_signature_data TEXT,
    member_signed_at TIMESTAMPTZ,
    staff_signature_data TEXT,
    staff_signed_at TIMESTAMPTZ,

    -- T&C acceptance records (JSON array of {id, title, version, content_hash, accepted_at})
    accepted_terms JSONB DEFAULT '[]'::jsonb,

    -- Full rendered HTML of the agreement (for PDF generation)
    agreement_html TEXT,

    -- Metadata
    ip_address TEXT,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agreement_snapshots_membership ON membership_agreement_snapshots(membership_id);
CREATE INDEX IF NOT EXISTS idx_agreement_snapshots_client ON membership_agreement_snapshots(client_id);
CREATE INDEX IF NOT EXISTS idx_agreement_snapshots_tenant ON membership_agreement_snapshots(tenant_id);

-- Stores individual T&C acceptance records with content snapshot
CREATE TABLE IF NOT EXISTS membership_accepted_terms (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    membership_id UUID NOT NULL REFERENCES memberships(id) ON DELETE CASCADE,
    terms_id UUID NOT NULL REFERENCES terms_and_conditions(id),
    snapshot_id UUID REFERENCES membership_agreement_snapshots(id) ON DELETE CASCADE,

    -- Frozen T&C content at acceptance time
    terms_title TEXT NOT NULL,
    terms_version INTEGER NOT NULL DEFAULT 1,
    terms_content_html TEXT NOT NULL,
    content_sha256 TEXT NOT NULL,

    accepted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_accepted_terms_membership ON membership_accepted_terms(membership_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_accepted_terms_unique ON membership_accepted_terms(membership_id, terms_id);
