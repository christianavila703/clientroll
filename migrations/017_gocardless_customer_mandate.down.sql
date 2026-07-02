DROP INDEX IF EXISTS idx_memberships_gocardless_mandate;
DROP INDEX IF EXISTS idx_clients_gocardless_customer;
ALTER TABLE memberships DROP COLUMN IF EXISTS gocardless_mandate_id;
ALTER TABLE clients DROP COLUMN IF EXISTS gocardless_customer_id;
