-- One-off: Update user email ben@businessnet.com.au → ben+sf@businessnet.com.au
-- Tenant slug (cnektp) stays unchanged.

UPDATE users
SET email = 'ben+sf@businessnet.com.au'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp')
  AND email = 'ben@businessnet.com.au';

UPDATE staff_invitations
SET email = 'ben+sf@businessnet.com.au'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp')
  AND email = 'ben@businessnet.com.au';

UPDATE user_email_verifications
SET email = 'ben+sf@businessnet.com.au'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp')
  AND email = 'ben@businessnet.com.au';
