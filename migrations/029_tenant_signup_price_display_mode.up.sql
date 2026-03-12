ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS signup_price_display_mode VARCHAR(20) NOT NULL DEFAULT 'weekly';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_tenants_signup_price_display_mode'
    ) THEN
        ALTER TABLE tenants
            ADD CONSTRAINT chk_tenants_signup_price_display_mode
            CHECK (signup_price_display_mode IN ('total', 'weekly'));
    END IF;
END $$;
