-- One-off: Move tenant owner@example → owner+alt@example
-- Run with: psql $DATABASE_URL -f scripts/move-tenant-owner-to-owner-alt.sql
--
-- If your slug uses hyphens (e.g. owner-example), use these instead:
--   OLD: owner-example  →  NEW: owner-alt-example
--   (Edit the script to replace owner@example / owner+alt@example accordingly)
--
-- Note: Slug "owner+alt@example" contains @ - Settings UI won't allow slug edits.
-- Use "owner-alt-example" if you want to edit the slug in the app later.

BEGIN;

-- 1. Update tenant slug
UPDATE tenants
SET slug = 'owner+alt@example', updated_at = NOW()
WHERE slug = 'owner@example';

-- 2. Update admin/staff user email in that tenant
UPDATE users
SET email = 'owner+alt@example'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'owner+alt@example')
  AND email = 'owner@example';

-- 3. Update any pending staff invitations
UPDATE staff_invitations
SET email = 'owner+alt@example'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'owner+alt@example')
  AND email = 'owner@example';

-- 4. Update email verification records
UPDATE user_email_verifications
SET email = 'owner+alt@example'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'owner+alt@example')
  AND email = 'owner@example';

COMMIT;
