DROP INDEX IF EXISTS idx_membership_types_slug_tenant_active;
ALTER TABLE membership_types DROP COLUMN IF EXISTS deleted_at;
