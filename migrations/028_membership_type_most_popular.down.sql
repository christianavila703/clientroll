DROP INDEX IF EXISTS idx_membership_types_one_most_popular;

ALTER TABLE membership_types
    DROP COLUMN IF EXISTS is_most_popular;
