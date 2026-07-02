-- Revert: ben+sf@businessnet.com.au → ben@businessnet.com.au

UPDATE users
SET email = 'ben@businessnet.com.au'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp')
  AND email = 'ben+sf@businessnet.com.au';

UPDATE staff_invitations
SET email = 'ben@businessnet.com.au'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp')
  AND email = 'ben+sf@businessnet.com.au';

UPDATE user_email_verifications
SET email = 'ben@businessnet.com.au'
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp')
  AND email = 'ben+sf@businessnet.com.au';
