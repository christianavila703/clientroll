-- Optional breakdown of each membership payment (base / tax / processing fee) for transparency.
ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS base_amount NUMERIC(10, 2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount NUMERIC(10, 2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS processing_fee_amount NUMERIC(10, 2) NOT NULL DEFAULT 0;

-- Legacy rows: amount was always stored as the total collected; treat it as base with no split.
UPDATE payments
SET base_amount = amount,
    tax_amount = 0,
    processing_fee_amount = 0
WHERE base_amount = 0
  AND tax_amount = 0
  AND processing_fee_amount = 0;
