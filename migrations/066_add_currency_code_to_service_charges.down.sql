ALTER TABLE service_charges
    DROP CONSTRAINT IF EXISTS chk_service_charges_currency_code;

ALTER TABLE service_charges
    DROP COLUMN IF EXISTS currency_code;
