-- Multi-currency support for tenant settings, memberships, and payments.

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS enabled_currencies JSONB NOT NULL DEFAULT '["AUD"]'::jsonb;

ALTER TABLE membership_types
    ADD COLUMN IF NOT EXISTS currency_code VARCHAR(3) NOT NULL DEFAULT 'AUD';

ALTER TABLE memberships
    ADD COLUMN IF NOT EXISTS currency_code VARCHAR(3) NOT NULL DEFAULT 'AUD';

ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS currency_code VARCHAR(3) NOT NULL DEFAULT 'AUD';

UPDATE membership_types
SET currency_code = 'AUD'
WHERE currency_code IS NULL OR currency_code = '';

UPDATE memberships
SET currency_code = 'AUD'
WHERE currency_code IS NULL OR currency_code = '';

UPDATE payments
SET currency_code = 'AUD'
WHERE currency_code IS NULL OR currency_code = '';

UPDATE tenants
SET enabled_currencies = '["AUD"]'::jsonb
WHERE enabled_currencies IS NULL OR jsonb_typeof(enabled_currencies) <> 'array' OR jsonb_array_length(enabled_currencies) = 0;

ALTER TABLE membership_types
    ADD CONSTRAINT chk_membership_types_currency_code
    CHECK (currency_code ~ '^[A-Z]{3}$');

ALTER TABLE memberships
    ADD CONSTRAINT chk_memberships_currency_code
    CHECK (currency_code ~ '^[A-Z]{3}$');

ALTER TABLE payments
    ADD CONSTRAINT chk_payments_currency_code
    CHECK (currency_code ~ '^[A-Z]{3}$');
