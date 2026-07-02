-- Link webhook events to the payment/membership they affected for audit trail.
ALTER TABLE inbound_webhook_events
  ADD COLUMN IF NOT EXISTS payment_id UUID REFERENCES payments(id),
  ADD COLUMN IF NOT EXISTS membership_id UUID REFERENCES memberships(id),
  ADD COLUMN IF NOT EXISTS expected_outcome VARCHAR(30);

CREATE INDEX IF NOT EXISTS idx_webhook_events_payment ON inbound_webhook_events(payment_id) WHERE payment_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_webhook_events_membership ON inbound_webhook_events(membership_id) WHERE membership_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_webhook_events_stuck ON inbound_webhook_events(status, received_at) WHERE status = 'received';
