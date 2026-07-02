ALTER TABLE payments
    DROP CONSTRAINT IF EXISTS chk_payments_currency_code;

ALTER TABLE memberships
    DROP CONSTRAINT IF EXISTS chk_memberships_currency_code;

ALTER TABLE membership_types
    DROP CONSTRAINT IF EXISTS chk_membership_types_currency_code;

ALTER TABLE payments
    DROP COLUMN IF EXISTS currency_code;

ALTER TABLE memberships
    DROP COLUMN IF EXISTS currency_code;

ALTER TABLE membership_types
    DROP COLUMN IF EXISTS currency_code;

ALTER TABLE tenants
    DROP COLUMN IF EXISTS enabled_currencies;
