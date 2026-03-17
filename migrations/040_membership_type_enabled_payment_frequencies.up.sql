ALTER TABLE membership_types
    ADD COLUMN IF NOT EXISTS enabled_payment_frequencies JSONB;
