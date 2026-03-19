ALTER TABLE service_charges
ADD COLUMN IF NOT EXISTS processing_fee_amount NUMERIC(10,2) NOT NULL DEFAULT 0;

UPDATE service_charges
SET processing_fee_amount = 0
WHERE processing_fee_amount IS NULL;
