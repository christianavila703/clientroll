-- Revert: owner+alt@example.com → owner@example.com

UPDATE users
SET email = 'owner@example.com'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'sample-tenant')
  AND email = 'owner+alt@example.com';

UPDATE staff_invitations
SET email = 'owner@example.com'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'sample-tenant')
  AND email = 'owner+alt@example.com';

UPDATE user_email_verifications
SET email = 'owner@example.com'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'sample-tenant')
  AND email = 'owner+alt@example.com';
