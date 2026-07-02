ALTER TABLE tenants
    DROP CONSTRAINT IF EXISTS chk_tenants_signup_price_display_mode;

ALTER TABLE tenants
    DROP COLUMN IF EXISTS signup_price_display_mode;
