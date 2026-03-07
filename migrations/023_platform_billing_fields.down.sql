ALTER TABLE tenant_invoices
    DROP COLUMN IF EXISTS invoice_pdf_url,
    DROP COLUMN IF EXISTS hosted_invoice_url;

ALTER TABLE tenant_subscriptions
    DROP COLUMN IF EXISTS card_exp_year,
    DROP COLUMN IF EXISTS card_exp_month,
    DROP COLUMN IF EXISTS card_last4,
    DROP COLUMN IF EXISTS card_brand,
    DROP COLUMN IF EXISTS has_valid_payment_method,
    DROP COLUMN IF EXISTS billing_grace_until,
    DROP COLUMN IF EXISTS past_due_since,
    DROP COLUMN IF EXISTS stripe_subscription_item_id;

ALTER TABLE platform_billing_config
    DROP COLUMN IF EXISTS stripe_price_id,
    DROP COLUMN IF EXISTS stripe_product_id;

