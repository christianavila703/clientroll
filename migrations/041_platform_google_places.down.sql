ALTER TABLE platform_billing_config
    DROP COLUMN IF EXISTS google_places_is_active,
    DROP COLUMN IF EXISTS google_places_api_key_encrypted;
