ALTER TABLE inbound_webhook_events
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'received',
    ADD COLUMN IF NOT EXISTS retry_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_error TEXT,
    ADD COLUMN IF NOT EXISTS last_error_at TIMESTAMPTZ;

UPDATE inbound_webhook_events
SET status = CASE
    WHEN processed_at IS NOT NULL THEN 'processed'
    ELSE 'received'
END
WHERE status IS NULL OR status = '';

CREATE INDEX IF NOT EXISTS idx_inbound_webhook_events_status
    ON inbound_webhook_events (provider, status, received_at DESC);
