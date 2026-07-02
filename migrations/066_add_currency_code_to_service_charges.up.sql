ALTER TABLE service_charges
    ADD COLUMN IF NOT EXISTS currency_code VARCHAR(3) NOT NULL DEFAULT 'AUD';

UPDATE service_charges sc
SET currency_code = UPPER(COALESCE(s.currency_code, 'AUD'))
FROM services s
WHERE sc.service_id = s.id
  AND (sc.currency_code IS NULL OR sc.currency_code = '' OR sc.currency_code = 'AUD');

UPDATE service_charges
SET currency_code = 'AUD'
WHERE currency_code IS NULL OR currency_code = '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_service_charges_currency_code'
    ) THEN
        ALTER TABLE service_charges
            ADD CONSTRAINT chk_service_charges_currency_code
            CHECK (currency_code ~ '^[A-Z]{3}$');
    END IF;
END $$;
