ALTER TABLE membership_types
    ADD COLUMN IF NOT EXISTS is_most_popular BOOLEAN NOT NULL DEFAULT false;

-- Allow at most one "most popular" membership type per tenant (active records only).
CREATE UNIQUE INDEX IF NOT EXISTS idx_membership_types_one_most_popular
    ON membership_types (tenant_id)
    WHERE is_most_popular = true AND deleted_at IS NULL;
