-- Payments, failures, and disputes

-- Payments
CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    membership_id UUID NOT NULL REFERENCES memberships(id) ON DELETE CASCADE,
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    amount DECIMAL(10, 2) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    provider_reference VARCHAR(255),
    due_date DATE NOT NULL,
    paid_date DATE,
    failure_reason TEXT,
    failure_code VARCHAR(50),
    retry_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_payment_status CHECK (
        status IN ('pending', 'succeeded', 'failed', 'disputed', 'refunded', 'cancelled')
    )
);

CREATE INDEX idx_payments_tenant ON payments(tenant_id);
CREATE INDEX idx_payments_membership ON payments(membership_id);
CREATE INDEX idx_payments_client ON payments(client_id);
CREATE INDEX idx_payments_status ON payments(tenant_id, status);
CREATE INDEX idx_payments_due_date ON payments(tenant_id, due_date);

-- Payment disputes
CREATE TABLE payment_disputes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    provider_dispute_id VARCHAR(255),
    reason TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'open',
    amount DECIMAL(10, 2) NOT NULL,
    notes TEXT,
    resolved_by VARCHAR(255),
    opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_dispute_status CHECK (
        status IN ('open', 'under_review', 'won', 'lost', 'resolved')
    )
);

CREATE INDEX idx_disputes_tenant ON payment_disputes(tenant_id);
CREATE INDEX idx_disputes_payment ON payment_disputes(payment_id);
CREATE INDEX idx_disputes_status ON payment_disputes(tenant_id, status);

-- Payment failure log (detailed history of each failure)
CREATE TABLE payment_failure_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    failure_code VARCHAR(50),
    failure_reason TEXT,
    provider_detail TEXT,
    failed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_failure_log_tenant ON payment_failure_log(tenant_id);
CREATE INDEX idx_failure_log_payment ON payment_failure_log(payment_id);
