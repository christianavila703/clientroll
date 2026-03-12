-- Split legacy combined ABN/ACN field into dedicated columns.
ALTER TABLE agreement_settings
    ADD COLUMN IF NOT EXISTS abn VARCHAR(50),
    ADD COLUMN IF NOT EXISTS acn VARCHAR(50);

-- Backfill ABN from legacy combined field where possible.
UPDATE agreement_settings
SET abn = abn_acn
WHERE abn IS NULL
  AND abn_acn IS NOT NULL
  AND trim(abn_acn) <> '';
