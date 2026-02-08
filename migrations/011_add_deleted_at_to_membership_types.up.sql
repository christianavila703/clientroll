-- Soft delete support for membership types.
-- deleted_at != NULL means the type is hidden from listings but
-- existing client memberships referencing it remain intact.
ALTER TABLE membership_types ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ DEFAULT NULL;

-- Drop the unique slug constraint so a new type with the same name can be created
-- after the old one is soft-deleted.
DROP INDEX IF EXISTS idx_membership_types_slug_tenant;
CREATE UNIQUE INDEX idx_membership_types_slug_tenant_active
    ON membership_types (tenant_id, slug)
    WHERE deleted_at IS NULL;
