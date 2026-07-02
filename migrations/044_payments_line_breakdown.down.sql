ALTER TABLE payments
    DROP COLUMN IF EXISTS base_amount,
    DROP COLUMN IF EXISTS tax_amount,
    DROP COLUMN IF EXISTS processing_fee_amount;
