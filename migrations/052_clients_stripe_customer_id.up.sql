ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS stripe_customer_id TEXT;

CREATE INDEX IF NOT EXISTS idx_clients_stripe_customer_id
    ON clients (tenant_id, stripe_customer_id)
    WHERE stripe_customer_id IS NOT NULL;

COMMENT ON COLUMN clients.stripe_customer_id IS 'Stripe customer id used for saved payment methods and setup sessions';
