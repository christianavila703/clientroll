ALTER TABLE tenants DROP CONSTRAINT IF EXISTS chk_tenants_signup_price_display_mode;

ALTER TABLE tenants
    ADD CONSTRAINT chk_tenants_signup_price_display_mode
    CHECK (signup_price_display_mode IN ('total', 'weekly', 'monthly'));
