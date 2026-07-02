-- Platform billing additional fields needed for Stripe automation and enforcement.

-- platform_billing_config: store Stripe product/price IDs (non-secret).
ALTER TABLE platform_billing_config
    ADD COLUMN IF NOT EXISTS stripe_product_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS stripe_price_id VARCHAR(255);

-- tenant_subscriptions: track subscription item and enforcement state.
ALTER TABLE tenant_subscriptions
    ADD COLUMN IF NOT EXISTS stripe_subscription_item_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS past_due_since TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS billing_grace_until TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS has_valid_payment_method BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS card_brand VARCHAR(50),
    ADD COLUMN IF NOT EXISTS card_last4 VARCHAR(4),
    ADD COLUMN IF NOT EXISTS card_exp_month INT,
    ADD COLUMN IF NOT EXISTS card_exp_year INT;

-- tenant_invoices: store Stripe URLs for admin visibility.
ALTER TABLE tenant_invoices
    ADD COLUMN IF NOT EXISTS hosted_invoice_url TEXT,
    ADD COLUMN IF NOT EXISTS invoice_pdf_url TEXT;

