-- Separate company street line from unit (parity with personal unit_number + address).
ALTER TABLE clients ADD COLUMN IF NOT EXISTS company_unit_number TEXT;
