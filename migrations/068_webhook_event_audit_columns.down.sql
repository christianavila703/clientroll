DROP INDEX IF EXISTS idx_webhook_events_stuck;
DROP INDEX IF EXISTS idx_webhook_events_membership;
DROP INDEX IF EXISTS idx_webhook_events_payment;

ALTER TABLE inbound_webhook_events
  DROP COLUMN IF EXISTS expected_outcome,
  DROP COLUMN IF EXISTS membership_id,
  DROP COLUMN IF EXISTS payment_id;
