DROP INDEX IF EXISTS idx_membership_types_one_most_popular;

CREATE UNIQUE INDEX IF NOT EXISTS idx_membership_types_one_most_popular_per_group
    ON membership_types (tenant_id, COALESCE(NULLIF(membership_name, ''), title))
    WHERE is_most_popular = true AND deleted_at IS NULL;
