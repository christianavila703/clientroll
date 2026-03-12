DROP INDEX IF EXISTS idx_membership_types_one_most_popular_per_group;

CREATE UNIQUE INDEX IF NOT EXISTS idx_membership_types_one_most_popular
    ON membership_types (tenant_id)
    WHERE is_most_popular = true AND deleted_at IS NULL;
