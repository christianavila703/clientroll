-- One-off: Update user email owner@example.com → owner+alt@example.com
-- Tenant slug (sample-tenant) stays unchanged.

UPDATE users
SET email = 'owner+alt@example.com'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'sample-tenant')
  AND email = 'owner@example.com';

UPDATE staff_invitations
SET email = 'owner+alt@example.com'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'sample-tenant')
  AND email = 'owner@example.com';

UPDATE user_email_verifications
SET email = 'owner+alt@example.com'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'sample-tenant')
  AND email = 'owner@example.com';
