ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS enabled_payment_frequencies JSONB NOT NULL
    DEFAULT '["upfront","weekly","fortnightly","monthly","quarterly","custom_schedule"]'::jsonb;
