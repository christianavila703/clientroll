ALTER TABLE services
    DROP CONSTRAINT IF EXISTS chk_services_currency_code;

ALTER TABLE services
    DROP COLUMN IF EXISTS currency_code;
