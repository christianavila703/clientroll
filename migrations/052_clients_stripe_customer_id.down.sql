DROP INDEX IF EXISTS idx_clients_stripe_customer_id;

ALTER TABLE clients
    DROP COLUMN IF EXISTS stripe_customer_id;
