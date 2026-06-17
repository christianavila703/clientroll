DROP TABLE IF EXISTS signup_webhook_deliveries;

ALTER TABLE memberships
DROP COLUMN IF EXISTS signup_webhook_sent_at;

ALTER TABLE tenants
DROP COLUMN IF EXISTS signup_webhook_enabled,
DROP COLUMN IF EXISTS signup_webhook_secret_encrypted,
DROP COLUMN IF EXISTS signup_webhook_url;
