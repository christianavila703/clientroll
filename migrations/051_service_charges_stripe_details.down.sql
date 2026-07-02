DROP INDEX IF EXISTS idx_service_charges_provider_reference;

ALTER TABLE service_charges
    DROP COLUMN IF EXISTS stripe_details,
    DROP COLUMN IF EXISTS payment_method,
    DROP COLUMN IF EXISTS provider_reference;
