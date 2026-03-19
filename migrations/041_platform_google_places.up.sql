-- Store a single platform-wide Google Places API key (encrypted) for all tenants.
ALTER TABLE platform_billing_config
    ADD COLUMN IF NOT EXISTS google_places_api_key_encrypted TEXT,
    ADD COLUMN IF NOT EXISTS google_places_is_active BOOLEAN NOT NULL DEFAULT FALSE;
