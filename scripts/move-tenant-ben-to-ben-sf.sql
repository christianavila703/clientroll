-- One-off: Move tenant ben@businessnet → ben+sf@businessnet
-- Run with: psql $DATABASE_URL -f scripts/move-tenant-ben-to-ben-sf.sql
--
-- If your slug uses hyphens (e.g. ben-businessnet), use these instead:
--   OLD: ben-businessnet  →  NEW: ben-sf-businessnet
--   (Edit the script to replace ben@businessnet / ben+sf@businessnet accordingly)
--
-- Note: Slug "ben+sf@businessnet" contains @ - Settings UI won't allow slug edits.
-- Use "ben-sf-businessnet" if you want to edit the slug in the app later.

BEGIN;

-- 1. Update tenant slug
UPDATE tenants
SET slug = 'ben+sf@businessnet', updated_at = NOW()
WHERE slug = 'ben@businessnet';

-- 2. Update admin/staff user email in that tenant
UPDATE users
SET email = 'ben+sf@businessnet'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'ben+sf@businessnet')
  AND email = 'ben@businessnet';

-- 3. Update any pending staff invitations
UPDATE staff_invitations
SET email = 'ben+sf@businessnet'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'ben+sf@businessnet')
  AND email = 'ben@businessnet';

-- 4. Update email verification records
UPDATE user_email_verifications
SET email = 'ben+sf@businessnet'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'ben+sf@businessnet')
  AND email = 'ben@businessnet';

COMMIT;
