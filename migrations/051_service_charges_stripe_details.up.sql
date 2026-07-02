ALTER TABLE service_charges
    ADD COLUMN IF NOT EXISTS provider_reference TEXT,
    ADD COLUMN IF NOT EXISTS payment_method TEXT,
    ADD COLUMN IF NOT EXISTS stripe_details JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_service_charges_provider_reference
    ON service_charges (tenant_id, provider_reference)
    WHERE provider_reference IS NOT NULL;

COMMENT ON COLUMN service_charges.provider_reference IS 'External provider reference (Stripe checkout session / PI / charge id)';
COMMENT ON COLUMN service_charges.payment_method IS 'Payment method hint (e.g. card, direct_debit)';
COMMENT ON COLUMN service_charges.stripe_details IS 'Merged Stripe snapshot for service charges: checkout / PI / charge / customer details';
