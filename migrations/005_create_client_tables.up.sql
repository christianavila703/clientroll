-- Clients and memberships

-- Clients
CREATE TABLE clients (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    address TEXT,
    suburb VARCHAR(100),
    state VARCHAR(50),
    postcode VARCHAR(10),
    email VARCHAR(255),
    phone VARCHAR(50),
    birth_date DATE,
    gender VARCHAR(10),
    emergency_contact_name VARCHAR(255),
    emergency_contact_phone VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_clients_tenant ON clients(tenant_id);
CREATE INDEX idx_clients_email ON clients(tenant_id, email);
CREATE INDEX idx_clients_name ON clients(tenant_id, name);

-- Memberships (agreements)
CREATE TABLE memberships (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    membership_type_id UUID NOT NULL REFERENCES membership_types(id),
    term_option_id UUID NOT NULL REFERENCES membership_term_options(id),
    start_date DATE NOT NULL,
    min_term_end_date DATE NOT NULL,
    payment_method VARCHAR(50),
    payment_frequency VARCHAR(20),
    payment_amount DECIMAL(10, 2),
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    member_signature_data TEXT,
    member_signed_at TIMESTAMPTZ,
    staff_user_id UUID REFERENCES users(id),
    staff_signature_data TEXT,
    staff_signed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_membership_status CHECK (
        status IN ('pending', 'active', 'cancelled', 'expired', 'suspended')
    ),
    CONSTRAINT chk_payment_frequency CHECK (
        payment_frequency IN ('weekly', 'fortnightly', 'monthly', 'upfront')
    )
);

CREATE INDEX idx_memberships_tenant ON memberships(tenant_id);
CREATE INDEX idx_memberships_client ON memberships(client_id);
CREATE INDEX idx_memberships_status ON memberships(tenant_id, status);
CREATE INDEX idx_memberships_type ON memberships(membership_type_id);

-- Membership addons
CREATE TABLE membership_addons (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    membership_id UUID NOT NULL REFERENCES memberships(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    value DECIMAL(10, 2) NOT NULL DEFAULT 0
);

CREATE INDEX idx_membership_addons_tenant ON membership_addons(tenant_id);
CREATE INDEX idx_membership_addons_membership ON membership_addons(membership_id);

-- Bank details (encrypted)
CREATE TABLE bank_details (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    membership_id UUID NOT NULL REFERENCES memberships(id) ON DELETE CASCADE UNIQUE,
    institution_name VARCHAR(255),
    branch_location VARCHAR(255),
    account_holder_name VARCHAR(255),
    account_number_encrypted BYTEA,
    bsb_encrypted BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bank_details_tenant ON bank_details(tenant_id);
CREATE UNIQUE INDEX idx_bank_details_membership ON bank_details(membership_id);
