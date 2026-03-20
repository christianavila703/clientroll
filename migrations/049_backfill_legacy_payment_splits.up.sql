-- Backfill legacy payment split fields (base/tax/processing fee).
-- Safe strategy:
-- 1) If amount already exceeds base+tax, treat the delta as processing fee.
-- 2) Otherwise, if membership payment_amount exists and amount exceeds it,
--    treat (amount - membership payment_amount) as fee and recompute base/tax
--    from membership payment_amount using current GST rules.

-- Pass 1: derive fee from existing amount vs base+tax delta.
UPDATE payments p
SET processing_fee_amount = GREATEST(0, p.amount - COALESCE(p.base_amount, 0) - COALESCE(p.tax_amount, 0))
WHERE COALESCE(p.processing_fee_amount, 0) <= 0.0001
  AND p.amount > (COALESCE(p.base_amount, 0) + COALESCE(p.tax_amount, 0) + 0.009);

-- Pass 2: derive fee/base/tax from membership payment amount and tenant tax settings.
UPDATE payments p
SET
  processing_fee_amount = GREATEST(0, p.amount - m.payment_amount),
  tax_amount = CASE
    WHEN COALESCE(mt.apply_sales_tax, TRUE) = TRUE AND COALESCE(t.sales_tax_percent, 0) > 0
      THEN ROUND((m.payment_amount * (LEAST(GREATEST(t.sales_tax_percent, 0), 100) / (100 + LEAST(GREATEST(t.sales_tax_percent, 0), 100))))::numeric, 2)
    ELSE 0
  END,
  base_amount = GREATEST(
    0,
    m.payment_amount - CASE
      WHEN COALESCE(mt.apply_sales_tax, TRUE) = TRUE AND COALESCE(t.sales_tax_percent, 0) > 0
        THEN ROUND((m.payment_amount * (LEAST(GREATEST(t.sales_tax_percent, 0), 100) / (100 + LEAST(GREATEST(t.sales_tax_percent, 0), 100))))::numeric, 2)
      ELSE 0
    END
  )
FROM memberships m
JOIN membership_types mt ON mt.id = m.membership_type_id
JOIN tenants t ON t.id = m.tenant_id
WHERE p.membership_id = m.id
  AND p.tenant_id = m.tenant_id
  AND m.payment_amount IS NOT NULL
  AND m.payment_amount > 0
  AND COALESCE(p.processing_fee_amount, 0) <= 0.0001
  AND p.amount > m.payment_amount + 0.009;
