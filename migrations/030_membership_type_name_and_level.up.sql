ALTER TABLE membership_types
    ADD COLUMN IF NOT EXISTS membership_name TEXT,
    ADD COLUMN IF NOT EXISTS membership_level TEXT;

-- Backfill from legacy title where possible.
UPDATE membership_types
SET
    membership_name = COALESCE(
        NULLIF(membership_name, ''),
        NULLIF(split_part(title, ' - ', 1), '')
    ),
    membership_level = COALESCE(
        NULLIF(membership_level, ''),
        CASE
            WHEN position(' - ' IN title) > 0 THEN NULLIF(split_part(title, ' - ', 2), '')
            ELSE NULL
        END
    );
