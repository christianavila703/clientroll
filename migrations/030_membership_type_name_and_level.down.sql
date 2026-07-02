ALTER TABLE membership_types
    DROP COLUMN IF EXISTS membership_level,
    DROP COLUMN IF EXISTS membership_name;
