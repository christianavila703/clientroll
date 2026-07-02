-- Rich Stripe context for admin UI (BECS/card subscription invoices, etc.)
ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS stripe_details JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN payments.stripe_details IS 'Merged Stripe snapshot: invoice/PI/charge/mandate IDs, method summary, timeline timestamps';
