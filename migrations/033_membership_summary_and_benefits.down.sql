ALTER TABLE membership_types
    DROP COLUMN IF EXISTS summary,
    DROP COLUMN IF EXISTS benefits;
