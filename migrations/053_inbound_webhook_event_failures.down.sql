DROP INDEX IF EXISTS idx_inbound_webhook_events_status;

ALTER TABLE inbound_webhook_events
    DROP COLUMN IF EXISTS last_error_at,
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS retry_count,
    DROP COLUMN IF EXISTS status;
