-- Add currency support to services catalog items.
ALTER TABLE services
    ADD COLUMN IF NOT EXISTS currency_code VARCHAR(3) NOT NULL DEFAULT 'AUD';

UPDATE services
SET currency_code = 'AUD'
WHERE currency_code IS NULL OR currency_code = '';

ALTER TABLE services
    ADD CONSTRAINT chk_services_currency_code
    CHECK (currency_code ~ '^[A-Z]{3}$');
